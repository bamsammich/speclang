package infra

import "context"

// RunningService represents a started container with a resolved URL.
type RunningService struct {
	Name string
	URL  string // e.g., "http://localhost:8080"
	Port int    // actual mapped host port
}

// ServiceManager handles container lifecycle.
type ServiceManager interface {
	Start(ctx context.Context) ([]RunningService, error)
	Stop(ctx context.Context) error
	Cleanup(ctx context.Context) error
}

// Config holds service declarations from a parsed spec target.
type Config struct {
	ComposePath string       // docker-compose file path
	SpecName    string       // for container labeling
	SpecDir     string       // base directory for resolving relative paths
	Services    []ServiceDef // inline service definitions
}

// ServiceDef mirrors parser.Service with resolved paths.
type ServiceDef struct {
	Env     map[string]string
	Volumes map[string]string // host:container (absolute paths)
	Name    string
	Build   string // Dockerfile directory (absolute path)
	Image   string // pre-built image
	Health  string // HTTP health path
	Port    int    // static port (0 = dynamic)
}
