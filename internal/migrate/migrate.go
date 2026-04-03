package migrate

import (
	"fmt"
	"os"

	"github.com/bamsammich/speclang/v3/internal/parser"
	"github.com/bamsammich/speclang/v3/internal/v2parser"
	"github.com/bamsammich/speclang/v3/pkg/spec"
)

// MigrateFile reads a v2 spec file, transforms it, and returns v3 text.
func MigrateFile(path string) (string, error) {
	// Parse with v2 parser (resolves includes for locator context)
	s, err := v2parser.ParseFile(path)
	if err != nil {
		return "", fmt.Errorf("parsing v2 spec: %w", err)
	}

	output, err := MigrateSpec(s)
	if err != nil {
		return "", err
	}

	// Validate output parses as v3
	if _, err := parser.Parse(output); err != nil {
		// Prepend warning but still return the output
		warning := "# MIGRATE-WARNING: output did not parse as valid v3: " + err.Error() + "\n"
		return warning + output, nil
	}

	return output, nil
}

// MigrateFileRaw reads a v2 spec file by path and returns v3 text.
// Unlike MigrateFile, it reads the raw source for per-file migration
// while using the resolved AST for locator context.
func MigrateFileRaw(path string) (string, error) {
	// Read raw source
	src, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("reading file: %w", err)
	}

	// Parse raw source (no include resolution — preserves include directives)
	s, err := v2parser.Parse(string(src))
	if err != nil {
		return "", fmt.Errorf("parsing v2 spec: %w", err)
	}

	return MigrateSpec(s)
}

// MigrateSpec transforms a v2 AST into formatted v3 spec text.
func MigrateSpec(s *spec.Spec) (string, error) {
	w := &v3Writer{}
	w.emitSpec(s)
	return w.String(), nil
}
