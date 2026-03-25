// Package specrun provides top-level convenience functions for parsing,
// validating, generating, and verifying specs. It bridges the public types
// in pkg/spec with the internal implementation packages.
package specrun

import (
	"fmt"

	"github.com/bamsammich/speclang/v2/internal/generator"
	"github.com/bamsammich/speclang/v2/internal/parser"
	"github.com/bamsammich/speclang/v2/internal/runner"
	"github.com/bamsammich/speclang/v2/internal/validator"
	"github.com/bamsammich/speclang/v2/pkg/spec"
)

// Options configures verification behavior.
type Options struct {
	Seed       uint64
	Iterations int
}

// Parse parses a spec from source text.
func Parse(source string) (*spec.Spec, error) {
	return parser.Parse(source)
}

// ParseFile parses a spec from a file, resolving includes and imports.
func ParseFile(path string, imports spec.ImportRegistry) (*spec.Spec, error) {
	return parser.ParseFileWithImports(path, imports)
}

// Validate checks a spec for semantic errors.
func Validate(s *spec.Spec) []error {
	return validator.Validate(s)
}

// FormatErrors formats validation errors into a human-readable string.
func FormatErrors(errs []error) string {
	return validator.FormatErrors(errs)
}

// Verify runs the full verification pipeline.
// If registry is nil, DefaultRegistry() is used.
func Verify(s *spec.Spec, registry *spec.Registry, opts Options) (*spec.Result, error) {
	if registry == nil {
		registry = DefaultRegistry()
	}

	adapters, err := buildAdapterMap(s, registry)
	if err != nil {
		return nil, err
	}
	defer closeAdapters(adapters)

	r := runner.New(s, adapters, opts.Seed)
	if opts.Iterations > 0 {
		r.SetN(opts.Iterations)
	}
	return r.Verify()
}

// Generate produces one random input for a named scope.
func Generate(s *spec.Spec, scopeName string, seed uint64) (map[string]any, error) {
	for _, scope := range s.Scopes {
		if scope.Name == scopeName {
			if scope.Contract == nil {
				return nil, fmt.Errorf("scope %q has no contract", scopeName)
			}
			g := generator.New(scope.Contract, s.Models, seed)
			return g.GenerateInput()
		}
	}
	return nil, fmt.Errorf("scope %q not found", scopeName)
}

func buildAdapterMap(s *spec.Spec, reg *spec.Registry) (map[string]spec.Adapter, error) {
	adapters := make(map[string]spec.Adapter)
	for _, scope := range s.Scopes {
		if scope.Use == "" {
			continue
		}
		if _, exists := adapters[scope.Use]; exists {
			continue
		}
		adp, err := reg.Adapter(scope.Use)
		if err != nil {
			closeAdapters(adapters)
			return nil, fmt.Errorf("adapter %q: %w", scope.Use, err)
		}
		adapters[scope.Use] = adp
	}
	return adapters, nil
}

func closeAdapters(adapters map[string]spec.Adapter) {
	for _, adp := range adapters {
		adp.Close() //nolint:errcheck // best-effort cleanup
	}
}
