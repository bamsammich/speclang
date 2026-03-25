package infra

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// ComposeManager manages containers via docker compose.
type ComposeManager struct {
	projectName string
	cfg         Config
}

// NewComposeManager creates a compose-based ServiceManager.
// Validates that the compose file exists and docker compose is available.
func NewComposeManager(cfg Config) (*ComposeManager, error) {
	if _, err := os.Stat(cfg.ComposePath); err != nil {
		return nil, fmt.Errorf("compose file %s: %w", cfg.ComposePath, err)
	}

	if err := checkComposeAvailable(); err != nil {
		return nil, err
	}

	return &ComposeManager{
		cfg:         cfg,
		projectName: sanitizeProjectName(cfg.SpecName),
	}, nil
}

// Start runs docker compose up and resolves service ports.
func (cm *ComposeManager) Start(ctx context.Context) ([]RunningService, error) {
	args := cm.baseArgs()
	args = append(args, "up", "-d", "--wait", "--build")

	if err := runCompose(ctx, args); err != nil {
		return nil, fmt.Errorf("compose up: %w", err)
	}

	return cm.resolveServices(ctx)
}

// Stop runs docker compose down to stop and remove containers.
func (cm *ComposeManager) Stop(ctx context.Context) error {
	args := cm.baseArgs()
	args = append(args, "down", "-v")

	if err := runCompose(ctx, args); err != nil {
		return fmt.Errorf("compose down: %w", err)
	}
	return nil
}

// Cleanup stops and removes all compose resources (same as Stop for compose).
func (cm *ComposeManager) Cleanup(ctx context.Context) error {
	return cm.Stop(ctx)
}

func (cm *ComposeManager) baseArgs() []string {
	return []string{"-f", cm.cfg.ComposePath, "-p", cm.projectName}
}

// resolveServices discovers running services and their mapped ports.
func (cm *ComposeManager) resolveServices(ctx context.Context) ([]RunningService, error) {
	names, err := cm.listServiceNames(ctx)
	if err != nil {
		return nil, err
	}

	var services []RunningService
	for _, name := range names {
		port, err := cm.resolveServicePort(ctx, name)
		if err != nil {
			// Service may not expose ports; skip it.
			continue
		}
		services = append(services, RunningService{
			Name: name,
			URL:  fmt.Sprintf("http://localhost:%d", port),
			Port: port,
		})
	}

	return services, nil
}

// listServiceNames runs docker compose config --services to get service names.
func (cm *ComposeManager) listServiceNames(ctx context.Context) ([]string, error) {
	args := cm.baseArgs()
	args = append(args, "config", "--services")

	out, err := runComposeOutput(ctx, args)
	if err != nil {
		return nil, fmt.Errorf("listing compose services: %w", err)
	}

	var names []string
	scanner := bufio.NewScanner(strings.NewReader(out))
	for scanner.Scan() {
		name := strings.TrimSpace(scanner.Text())
		if name != "" {
			names = append(names, name)
		}
	}
	return names, scanner.Err()
}

// resolveServicePort runs docker compose port to get the host port for a service.
func (cm *ComposeManager) resolveServicePort(ctx context.Context, serviceName string) (int, error) {
	// Try common ports: 80, 8080, 3000, 5000
	for _, containerPort := range []int{80, 8080, 3000, 5000} {
		args := cm.baseArgs()
		args = append(args, "port", serviceName, strconv.Itoa(containerPort))

		out, err := runComposeOutput(ctx, args)
		if err != nil {
			continue
		}

		port, err := parseComposePort(out)
		if err != nil {
			continue
		}
		return port, nil
	}

	return 0, fmt.Errorf("no exposed port found for service %s", serviceName)
}

// parseComposePort parses "0.0.0.0:12345" output from docker compose port.
func parseComposePort(output string) (int, error) {
	output = strings.TrimSpace(output)
	if output == "" {
		return 0, errors.New("empty port output")
	}

	parts := strings.SplitN(output, ":", 2)
	if len(parts) != 2 {
		return 0, fmt.Errorf("unexpected port format: %s", output)
	}

	port, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, fmt.Errorf("parsing port number: %w", err)
	}

	return port, nil
}

func checkComposeAvailable() error {
	cmd := exec.Command("docker", "compose", "version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker compose not available: %w", err)
	}
	return nil
}

func runCompose(ctx context.Context, args []string) error {
	fullArgs := append([]string{"compose"}, args...)
	cmd := exec.CommandContext(ctx, "docker", fullArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func runComposeOutput(ctx context.Context, args []string) (string, error) {
	fullArgs := append([]string{"compose"}, args...)
	cmd := exec.CommandContext(ctx, "docker", fullArgs...)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func sanitizeProjectName(name string) string {
	return "specrun-" + strings.ToLower(strings.ReplaceAll(name, " ", "-"))
}
