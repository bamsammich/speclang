package infra

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
)

func skipIfNoDocker(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker not available")
	}
	// Also verify the daemon is reachable.
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		t.Skipf("docker client init failed: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := cli.Ping(ctx); err != nil {
		t.Skipf("docker daemon not reachable: %v", err)
	}
}

func TestDockerManager_StartStop_Image(t *testing.T) {
	skipIfNoDocker(t)

	cfg := Config{
		SpecName: "infratest",
		Services: []ServiceDef{
			{
				Name:  "web",
				Image: "nginx:alpine",
				Port:  0, // dynamic
			},
		},
	}

	dm, err := NewDockerManager(cfg)
	if err != nil {
		t.Fatalf("NewDockerManager: %v", err)
	}

	ctx := context.Background()
	services, err := dm.Start(ctx)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() {
		if stopErr := dm.Stop(ctx); stopErr != nil {
			t.Errorf("Stop: %v", stopErr)
		}
	}()

	if len(services) != 1 {
		t.Fatalf("expected 1 service, got %d", len(services))
	}

	svc := services[0]
	if svc.Name != "web" {
		t.Errorf("expected name 'web', got %q", svc.Name)
	}
	if svc.Port == 0 {
		t.Error("expected non-zero dynamic port")
	}
	if svc.URL == "" {
		t.Error("expected non-empty URL")
	}
}

func TestDockerManager_StartStop_Build(t *testing.T) {
	skipIfNoDocker(t)

	// Create a temporary Dockerfile.
	dir := t.TempDir()
	dockerfile := filepath.Join(dir, "Dockerfile")
	err := os.WriteFile(dockerfile, []byte(`FROM nginx:alpine
EXPOSE 80
`), 0o644)
	if err != nil {
		t.Fatalf("writing Dockerfile: %v", err)
	}

	cfg := Config{
		SpecName: "buildtest",
		Services: []ServiceDef{
			{
				Name:  "built",
				Build: dir,
				Port:  0,
			},
		},
	}

	dm, err := NewDockerManager(cfg)
	if err != nil {
		t.Fatalf("NewDockerManager: %v", err)
	}

	ctx := context.Background()
	services, err := dm.Start(ctx)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() {
		if stopErr := dm.Stop(ctx); stopErr != nil {
			t.Errorf("Stop: %v", stopErr)
		}
	}()

	if len(services) != 1 {
		t.Fatalf("expected 1 service, got %d", len(services))
	}

	if services[0].Port == 0 {
		t.Error("expected non-zero port for built service")
	}
}

func TestDockerManager_Labels(t *testing.T) {
	skipIfNoDocker(t)

	cfg := Config{
		SpecName: "labeltest",
		Services: []ServiceDef{
			{
				Name:  "labeled",
				Image: "nginx:alpine",
			},
		},
	}

	dm, err := NewDockerManager(cfg)
	if err != nil {
		t.Fatalf("NewDockerManager: %v", err)
	}

	ctx := context.Background()
	if _, err := dm.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() {
		if stopErr := dm.Stop(ctx); stopErr != nil {
			t.Errorf("Stop: %v", stopErr)
		}
	}()

	// Verify labels via Docker API.
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		t.Fatalf("creating client: %v", err)
	}

	args := filters.NewArgs()
	args.Add("label", "specrun.spec=labeltest")
	containers, err := cli.ContainerList(ctx, container.ListOptions{Filters: args})
	if err != nil {
		t.Fatalf("listing containers: %v", err)
	}

	if len(containers) != 1 {
		t.Fatalf("expected 1 labeled container, got %d", len(containers))
	}

	labels := containers[0].Labels
	if labels["specrun.spec"] != "labeltest" {
		t.Errorf("expected label specrun.spec=labeltest, got %q", labels["specrun.spec"])
	}
	if labels["specrun.run-id"] == "" {
		t.Error("expected non-empty specrun.run-id label")
	}
}

func TestDockerManager_Cleanup(t *testing.T) {
	skipIfNoDocker(t)

	cfg := Config{
		SpecName: "cleanuptest",
		Services: []ServiceDef{
			{
				Name:  "orphan",
				Image: "nginx:alpine",
			},
		},
	}

	dm, err := NewDockerManager(cfg)
	if err != nil {
		t.Fatalf("NewDockerManager: %v", err)
	}

	ctx := context.Background()
	if _, err := dm.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Don't call Stop — simulate orphaned container.
	// Cleanup should find and remove it.
	if err := dm.Cleanup(ctx); err != nil {
		t.Fatalf("Cleanup: %v", err)
	}

	// Verify container is gone.
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		t.Fatalf("creating client: %v", err)
	}

	args := filters.NewArgs()
	args.Add("label", "specrun.spec=cleanuptest")
	containers, err := cli.ContainerList(ctx, container.ListOptions{All: true, Filters: args})
	if err != nil {
		t.Fatalf("listing containers: %v", err)
	}

	if len(containers) != 0 {
		t.Errorf("expected 0 containers after cleanup, got %d", len(containers))
	}
}

func TestDockerManager_EnvVars(t *testing.T) {
	skipIfNoDocker(t)

	cfg := Config{
		SpecName: "envtest",
		Services: []ServiceDef{
			{
				Name:  "withenv",
				Image: "nginx:alpine",
				Env: map[string]string{
					"FOO": "bar",
					"BAZ": "qux",
				},
			},
		},
	}

	dm, err := NewDockerManager(cfg)
	if err != nil {
		t.Fatalf("NewDockerManager: %v", err)
	}

	ctx := context.Background()
	services, err := dm.Start(ctx)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() {
		if stopErr := dm.Stop(ctx); stopErr != nil {
			t.Errorf("Stop: %v", stopErr)
		}
	}()

	if len(services) != 1 {
		t.Fatalf("expected 1 service, got %d", len(services))
	}

	// Verify env vars are set via container inspect.
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		t.Fatalf("creating client: %v", err)
	}

	info, err := cli.ContainerInspect(ctx, dm.containers[0])
	if err != nil {
		t.Fatalf("inspecting container: %v", err)
	}

	envMap := make(map[string]string)
	for _, e := range info.Config.Env {
		parts := splitEnvVar(e)
		if len(parts) == 2 {
			envMap[parts[0]] = parts[1]
		}
	}

	if envMap["FOO"] != "bar" {
		t.Errorf("expected FOO=bar, got FOO=%q", envMap["FOO"])
	}
	if envMap["BAZ"] != "qux" {
		t.Errorf("expected BAZ=qux, got BAZ=%q", envMap["BAZ"])
	}
}

// splitEnvVar splits "KEY=VALUE" into ["KEY", "VALUE"].
func splitEnvVar(s string) []string {
	idx := 0
	for i, c := range s {
		if c == '=' {
			idx = i
			break
		}
	}
	if idx == 0 {
		return []string{s}
	}
	return []string{s[:idx], s[idx+1:]}
}

func TestBuildPortConfig_Dynamic(t *testing.T) {
	svc := ServiceDef{Port: 0}
	containerPort, portBindings, exposedPorts := buildPortConfig(svc)

	if string(containerPort) != "80/tcp" {
		t.Errorf("expected container port 80/tcp, got %s", containerPort)
	}

	bindings := portBindings[containerPort]
	if len(bindings) != 1 {
		t.Fatalf("expected 1 binding, got %d", len(bindings))
	}
	if bindings[0].HostPort != "" {
		t.Errorf("expected empty host port for dynamic, got %q", bindings[0].HostPort)
	}

	if _, ok := exposedPorts[containerPort]; !ok {
		t.Error("expected container port in exposed ports")
	}
}

func TestBuildPortConfig_Static(t *testing.T) {
	svc := ServiceDef{Port: 8080}
	containerPort, portBindings, _ := buildPortConfig(svc)

	if string(containerPort) != "8080/tcp" {
		t.Errorf("expected container port 8080/tcp, got %s", containerPort)
	}

	bindings := portBindings[containerPort]
	if len(bindings) != 1 {
		t.Fatalf("expected 1 binding, got %d", len(bindings))
	}
	if bindings[0].HostPort != "8080" {
		t.Errorf("expected host port 8080, got %q", bindings[0].HostPort)
	}
}

func TestBuildEnvSlice(t *testing.T) {
	env := map[string]string{"A": "1", "B": "2"}
	result := buildEnvSlice(env)
	if len(result) != 2 {
		t.Fatalf("expected 2 env vars, got %d", len(result))
	}

	// nil map returns nil.
	if buildEnvSlice(nil) != nil {
		t.Error("expected nil for nil map")
	}
}

func TestBuildBinds(t *testing.T) {
	vols := map[string]string{"/host/path": "/container/path"}
	result := buildBinds(vols)
	if len(result) != 1 {
		t.Fatalf("expected 1 bind, got %d", len(result))
	}
	if result[0] != "/host/path:/container/path" {
		t.Errorf("unexpected bind: %s", result[0])
	}

	if buildBinds(nil) != nil {
		t.Error("expected nil for nil map")
	}
}

func TestGenerateRunID(t *testing.T) {
	id, err := generateRunID()
	if err != nil {
		t.Fatalf("generateRunID: %v", err)
	}
	if len(id) != 32 { // 16 bytes = 32 hex chars
		t.Errorf("expected 32-char hex string, got %d chars: %s", len(id), id)
	}

	// Two calls should produce different IDs.
	id2, err := generateRunID()
	if err != nil {
		t.Fatalf("generateRunID: %v", err)
	}
	if id == id2 {
		t.Error("expected different run IDs")
	}
}

func TestFindModuleRoot(t *testing.T) {
	// Create a temp tree: root/go.mod, root/sub/dir/
	root := t.TempDir()
	gomod := filepath.Join(root, "go.mod")
	if err := os.WriteFile(gomod, []byte("module test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	subDir := filepath.Join(root, "sub", "dir")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatal(err)
	}

	got := findModuleRoot(subDir)
	if got != root {
		t.Errorf("findModuleRoot(%q) = %q, want %q", subDir, got, root)
	}

	// No go.mod anywhere → empty string.
	nomod := t.TempDir()
	if got := findModuleRoot(nomod); got != "" {
		t.Errorf("findModuleRoot(%q) = %q, want empty", nomod, got)
	}
}

func TestResolveBuildContext_WithGoMod(t *testing.T) {
	root := t.TempDir()
	gomod := filepath.Join(root, "go.mod")
	if err := os.WriteFile(gomod, []byte("module test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	buildDir := filepath.Join(root, "services", "web")
	if err := os.MkdirAll(buildDir, 0o755); err != nil {
		t.Fatal(err)
	}

	ctxDir, dfPath := resolveBuildContext(buildDir)
	if ctxDir != root {
		t.Errorf("contextDir = %q, want %q", ctxDir, root)
	}
	if dfPath != "services/web/Dockerfile" {
		t.Errorf("dockerfilePath = %q, want %q", dfPath, "services/web/Dockerfile")
	}
}

func TestResolveBuildContext_WithoutGoMod(t *testing.T) {
	dir := t.TempDir()
	ctxDir, dfPath := resolveBuildContext(dir)
	if ctxDir != dir {
		t.Errorf("contextDir = %q, want %q", ctxDir, dir)
	}
	if dfPath != "Dockerfile" {
		t.Errorf("dockerfilePath = %q, want %q", dfPath, "Dockerfile")
	}
}

func TestCreateBuildContext(t *testing.T) {
	dir := t.TempDir()
	dockerfilePath := filepath.Join(dir, "Dockerfile")
	if err := os.WriteFile(dockerfilePath, []byte("FROM scratch\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "app.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	buf, err := createBuildContext(dir)
	if err != nil {
		t.Fatalf("createBuildContext: %v", err)
	}

	if buf.Len() == 0 {
		t.Error("expected non-empty tar buffer")
	}
}
