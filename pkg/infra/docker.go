package infra

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
)

const (
	defaultDockerfile  = "Dockerfile"
	healthPollInterval = 500 * time.Millisecond
	healthTimeout      = 30 * time.Second
	stopTimeout        = 10 // seconds
)

// DockerManager manages containers via the Docker API.
type DockerManager struct {
	client     *client.Client
	cfg        Config
	runID      string
	containers []string // container IDs created during Start
}

// NewDockerManager creates a Docker-based ServiceManager.
func NewDockerManager(cfg Config) (*DockerManager, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("creating docker client: %w", err)
	}

	runID, err := generateRunID()
	if err != nil {
		return nil, fmt.Errorf("generating run ID: %w", err)
	}

	return &DockerManager{
		client: cli,
		cfg:    cfg,
		runID:  runID,
	}, nil
}

// Start builds/pulls images, creates and starts containers, waits for health.
func (dm *DockerManager) Start(ctx context.Context) ([]RunningService, error) {
	var services []RunningService

	for _, svc := range dm.cfg.Services {
		rs, err := dm.startService(ctx, svc)
		if err != nil {
			// Best-effort cleanup of already-started containers.
			_ = dm.Stop(ctx) //nolint:errcheck // intentional best-effort cleanup
			return nil, fmt.Errorf("starting service %s: %w", svc.Name, err)
		}
		services = append(services, rs)
	}

	return services, nil
}

// Stop stops and removes all containers created during Start.
func (dm *DockerManager) Stop(ctx context.Context) error {
	var errs []error

	timeout := stopTimeout
	for _, id := range dm.containers {
		stopErr := dm.client.ContainerStop(ctx, id, container.StopOptions{Timeout: &timeout})
		if stopErr != nil {
			errs = append(errs, fmt.Errorf("stopping container %s: %w", id, stopErr))
		}
		rmErr := dm.client.ContainerRemove(ctx, id, container.RemoveOptions{Force: true})
		if rmErr != nil {
			errs = append(errs, fmt.Errorf("removing container %s: %w", id, rmErr))
		}
	}

	dm.containers = nil

	return errors.Join(errs...)
}

// Cleanup finds and removes all containers labeled for this spec.
func (dm *DockerManager) Cleanup(ctx context.Context) error {
	label := fmt.Sprintf("specrun.spec=%s", dm.cfg.SpecName)
	containers, err := dm.client.ContainerList(ctx, container.ListOptions{
		All:     true,
		Filters: newFilterArgs("label", label),
	})
	if err != nil {
		return fmt.Errorf("listing containers: %w", err)
	}

	var errs []error

	timeout := stopTimeout
	for _, c := range containers {
		stopOpts := container.StopOptions{Timeout: &timeout}
		if stopErr := dm.client.ContainerStop(ctx, c.ID, stopOpts); stopErr != nil {
			errs = append(errs, fmt.Errorf("stopping container %s: %w", c.ID, stopErr))
		}
		rmErr := dm.client.ContainerRemove(ctx, c.ID, container.RemoveOptions{Force: true})
		if rmErr != nil {
			errs = append(errs, fmt.Errorf("removing container %s: %w", c.ID, rmErr))
		}
	}

	return errors.Join(errs...)
}

func (dm *DockerManager) startService(ctx context.Context, svc ServiceDef) (RunningService, error) {
	if err := dm.ensureImage(ctx, svc); err != nil {
		return RunningService{}, err
	}

	imgName := dm.imageName(svc)
	containerName := dm.containerName(svc)

	containerPort, portBindings, exposedPorts := buildPortConfig(svc)

	envSlice := buildEnvSlice(svc.Env)

	binds := buildBinds(svc.Volumes)

	resp, err := dm.client.ContainerCreate(ctx,
		&container.Config{
			Image:        imgName,
			ExposedPorts: exposedPorts,
			Env:          envSlice,
			Labels: map[string]string{
				"specrun.spec":   dm.cfg.SpecName,
				"specrun.run-id": dm.runID,
			},
		},
		&container.HostConfig{
			PortBindings: portBindings,
			Binds:        binds,
		},
		nil, nil, containerName,
	)
	if err != nil {
		return RunningService{}, fmt.Errorf("creating container: %w", err)
	}

	dm.containers = append(dm.containers, resp.ID)

	if err := dm.client.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return RunningService{}, fmt.Errorf("starting container: %w", err)
	}

	port, err := dm.resolveHostPort(ctx, resp.ID, containerPort)
	if err != nil {
		return RunningService{}, fmt.Errorf("resolving host port: %w", err)
	}

	if err := waitForReady(ctx, port, svc.Health); err != nil {
		return RunningService{}, fmt.Errorf("health check failed for %s: %w", svc.Name, err)
	}

	return RunningService{
		Name: svc.Name,
		URL:  fmt.Sprintf("http://localhost:%d", port),
		Port: port,
	}, nil
}

func (dm *DockerManager) ensureImage(ctx context.Context, svc ServiceDef) error {
	if svc.Build != "" {
		return dm.buildImage(ctx, svc)
	}
	return dm.pullImage(ctx, svc.Image)
}

func (dm *DockerManager) buildImage(ctx context.Context, svc ServiceDef) error {
	// Use the Go module root (if found) as the build context so that
	// Dockerfiles can COPY go.mod and reference any package in the module.
	// The Dockerfile path is set relative to the context directory.
	contextDir, dockerfilePath := resolveBuildContext(svc.Build)

	tarBuf, err := createBuildContext(contextDir)
	if err != nil {
		return fmt.Errorf("creating build context: %w", err)
	}

	resp, err := dm.client.ImageBuild(ctx, tarBuf, types.ImageBuildOptions{
		Tags:       []string{dm.imageName(svc)},
		Remove:     true,
		Dockerfile: dockerfilePath,
	})
	if err != nil {
		return fmt.Errorf("building image: %w", err)
	}
	defer resp.Body.Close()

	// Must drain the response body for the build to complete.
	if _, err := io.Copy(io.Discard, resp.Body); err != nil {
		return fmt.Errorf("reading build output: %w", err)
	}

	return nil
}

func (dm *DockerManager) pullImage(ctx context.Context, imgRef string) error {
	reader, err := dm.client.ImagePull(ctx, imgRef, image.PullOptions{})
	if err != nil {
		return fmt.Errorf("pulling image %s: %w", imgRef, err)
	}
	defer reader.Close()

	// Must drain the response body for the pull to complete.
	if _, err := io.Copy(io.Discard, reader); err != nil {
		return fmt.Errorf("reading pull output: %w", err)
	}

	return nil
}

func (dm *DockerManager) resolveHostPort(
	ctx context.Context,
	containerID string,
	containerPort nat.Port,
) (int, error) {
	info, err := dm.client.ContainerInspect(ctx, containerID)
	if err != nil {
		return 0, fmt.Errorf("inspecting container: %w", err)
	}

	bindings, ok := info.NetworkSettings.Ports[containerPort]
	if !ok || len(bindings) == 0 {
		return 0, fmt.Errorf("no port binding found for %s", containerPort)
	}

	port, err := strconv.Atoi(bindings[0].HostPort)
	if err != nil {
		return 0, fmt.Errorf("parsing host port: %w", err)
	}

	return port, nil
}

func (dm *DockerManager) imageName(svc ServiceDef) string {
	if svc.Image != "" {
		return svc.Image
	}
	return fmt.Sprintf("specrun-%s-%s", strings.ToLower(dm.cfg.SpecName), strings.ToLower(svc.Name))
}

func (dm *DockerManager) containerName(svc ServiceDef) string {
	return fmt.Sprintf("specrun-%s-%s", strings.ToLower(dm.cfg.SpecName), strings.ToLower(svc.Name))
}

// buildPortConfig creates Docker port config from a ServiceDef.
// If Port > 0, binds to that specific host port. If Port == 0, lets Docker assign.
func buildPortConfig(svc ServiceDef) (nat.Port, nat.PortMap, nat.PortSet) {
	// Default container port to 80 if not specified.
	containerPortNum := svc.Port
	if containerPortNum == 0 {
		containerPortNum = 80
	}

	containerPort := nat.Port(fmt.Sprintf("%d/tcp", containerPortNum))

	hostPort := ""
	if svc.Port > 0 {
		hostPort = strconv.Itoa(svc.Port)
	}

	portBindings := nat.PortMap{
		containerPort: []nat.PortBinding{
			{HostIP: "0.0.0.0", HostPort: hostPort},
		},
	}

	exposedPorts := nat.PortSet{
		containerPort: struct{}{},
	}

	return containerPort, portBindings, exposedPorts
}

func buildEnvSlice(env map[string]string) []string {
	if len(env) == 0 {
		return nil
	}
	result := make([]string, 0, len(env))
	for k, v := range env {
		result = append(result, fmt.Sprintf("%s=%s", k, v))
	}
	return result
}

func buildBinds(volumes map[string]string) []string {
	if len(volumes) == 0 {
		return nil
	}
	result := make([]string, 0, len(volumes))
	for host, container := range volumes {
		result = append(result, fmt.Sprintf("%s:%s", host, container))
	}
	return result
}

// waitForReady polls until the service is reachable.
// If healthPath is set, performs HTTP GET; otherwise TCP dial.
func waitForReady(ctx context.Context, port int, healthPath string) error {
	deadline := time.Now().Add(healthTimeout)

	for time.Now().Before(deadline) {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		if healthPath != "" {
			if httpHealthCheck(port, healthPath) {
				return nil
			}
		} else {
			if tcpHealthCheck(port) {
				return nil
			}
		}

		time.Sleep(healthPollInterval)
	}

	return fmt.Errorf("service on port %d not ready after %s", port, healthTimeout)
}

func httpHealthCheck(port int, path string) bool {
	httpClient := &http.Client{Timeout: 2 * time.Second}
	resp, err := httpClient.Get(fmt.Sprintf("http://localhost:%d%s", port, path))
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusInternalServerError
}

func tcpHealthCheck(port int) bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%d", port), 2*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

func generateRunID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("reading random bytes: %w", err)
	}
	return fmt.Sprintf("%x", b), nil
}

// resolveBuildContext determines the Docker build context directory and the
// relative Dockerfile path. If a Go module root (go.mod) is found as an
// ancestor of buildDir, it is used as the context so that the Dockerfile can
// access the full module tree. Otherwise buildDir itself is the context.
func resolveBuildContext(buildDir string) (contextDir, dockerfilePath string) {
	moduleRoot := findModuleRoot(buildDir)
	if moduleRoot == "" {
		return buildDir, defaultDockerfile
	}

	rel, err := filepath.Rel(moduleRoot, buildDir)
	if err != nil {
		return buildDir, defaultDockerfile
	}

	return moduleRoot, filepath.ToSlash(filepath.Join(rel, defaultDockerfile))
}

// findModuleRoot walks up from dir looking for a directory containing go.mod.
// Returns the path to that directory, or "" if none is found.
func findModuleRoot(dir string) string {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return ""
	}

	for {
		if _, err := os.Stat(filepath.Join(abs, "go.mod")); err == nil {
			return abs
		}
		parent := filepath.Dir(abs)
		if parent == abs {
			return ""
		}
		abs = parent
	}
}

// createBuildContext creates a tar archive of the directory for Docker image build.
func createBuildContext(dir string) (*bytes.Buffer, error) {
	buf := &bytes.Buffer{}
	tw := tar.NewWriter(buf)

	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		return addToTar(tw, dir, path, d)
	})
	if err != nil {
		return nil, err
	}

	if err := tw.Close(); err != nil {
		return nil, fmt.Errorf("closing tar writer: %w", err)
	}

	return buf, nil
}

// addToTar adds a single file or directory entry to the tar writer.
func addToTar(tw *tar.Writer, baseDir, path string, d os.DirEntry) error {
	rel, err := filepath.Rel(baseDir, path)
	if err != nil {
		return fmt.Errorf("computing relative path: %w", err)
	}

	info, err := d.Info()
	if err != nil {
		return fmt.Errorf("getting file info: %w", err)
	}

	header, err := tar.FileInfoHeader(info, "")
	if err != nil {
		return fmt.Errorf("creating tar header: %w", err)
	}
	header.Name = filepath.ToSlash(rel)

	if err := tw.WriteHeader(header); err != nil {
		return fmt.Errorf("writing tar header: %w", err)
	}

	if d.IsDir() {
		return nil
	}

	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("opening file: %w", err)
	}
	defer f.Close()

	if _, err := io.Copy(tw, f); err != nil {
		return fmt.Errorf("writing file to tar: %w", err)
	}

	return nil
}

func newFilterArgs(key, value string) filters.Args {
	args := filters.NewArgs()
	args.Add(key, value)
	return args
}
