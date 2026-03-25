package spec

// ImportResolver converts an external schema file into speclang AST nodes.
type ImportResolver interface {
	Resolve(absPath string) ([]*Model, []*Scope, error)
}

// ImportRegistry maps adapter names (e.g., "openapi") to their resolvers.
type ImportRegistry map[string]ImportResolver
