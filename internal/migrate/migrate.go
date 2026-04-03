package migrate

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/bamsammich/speclang/v3/internal/parser"
	"github.com/bamsammich/speclang/v3/internal/v2parser"
	"github.com/bamsammich/speclang/v3/pkg/spec"
)

// MigratedFile represents a single migrated file with its path and v3 output.
type MigratedFile struct {
	Path    string // original file path
	Output  string // v3 text
	Warning string // non-empty if output did not parse as valid v3
}

// MigrateFile reads a v2 spec file, resolves its include tree, and returns
// migrated v3 text for every file in the tree (root + all included files).
func MigrateFile(path string) ([]MigratedFile, error) {
	absRoot, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolving path: %w", err)
	}

	// Parse the resolved AST to get the locator map across all files.
	resolved, err := v2parser.ParseFile(absRoot)
	if err != nil {
		return nil, fmt.Errorf("parsing v2 spec: %w", err)
	}
	locators := resolved.Locators

	// Collect all files in the include tree.
	files, err := collectIncludes(absRoot)
	if err != nil {
		return nil, fmt.Errorf("collecting includes: %w", err)
	}

	// Migrate each file. The root file is parsed as a full spec; included files
	// are fragments (models, scopes, actions without a spec wrapper).
	var results []MigratedFile
	for i, f := range files {
		src, err := os.ReadFile(f)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", f, err)
		}

		var output string
		var hasIncludes bool
		if i == 0 {
			// Root file: strip include directives before parsing (v2parser.Parse
			// doesn't resolve includes), then re-add them to the output.
			stripped, inc := stripIncludes(string(src))
			hasIncludes = len(inc) > 0
			s, err := v2parser.Parse(stripped)
			if err != nil {
				return nil, fmt.Errorf("parsing %s: %w", f, err)
			}
			if s.Locators == nil && locators != nil {
				s.Locators = locators
			}
			output, err = MigrateSpec(s)
			if err != nil {
				return nil, fmt.Errorf("migrating %s: %w", f, err)
			}
			if hasIncludes {
				output = insertIncludes(output, inc)
			}
		} else {
			// Included file: fragment (no spec wrapper). Migrate as fragment.
			output, err = migrateFragment(string(src), locators)
			if err != nil {
				return nil, fmt.Errorf("migrating %s: %w", f, err)
			}
		}

		mf := MigratedFile{Path: f, Output: output}

		// Validate output parses as v3.
		// Skip for files with includes (parser.Parse doesn't resolve them)
		// and for fragments (no spec wrapper).
		if i == 0 && !hasIncludes {
			if _, err := parser.Parse(output); err != nil {
				mf.Warning = err.Error()
			}
		}

		results = append(results, mf)
	}

	return results, nil
}

// MigrateSpec transforms a v2 AST into formatted v3 spec text.
func MigrateSpec(s *spec.Spec) (string, error) {
	w := &v3Writer{}
	w.emitSpec(s)
	return w.String(), nil
}

// stripIncludes removes `include "..."` lines from source, returning
// the stripped source and the list of include paths.
func stripIncludes(src string) (string, []string) {
	var includes []string
	var lines []string
	for _, line := range strings.Split(src, "\n") {
		if m := includeRe.FindStringSubmatch(line); m != nil {
			includes = append(includes, m[1])
		} else {
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, "\n"), includes
}

// insertIncludes adds include directives back into migrated output,
// just before the closing brace of the spec block.
func insertIncludes(output string, includes []string) string {
	lines := strings.Split(output, "\n")
	// Find the last "}" which closes the spec block.
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.TrimSpace(lines[i]) == "}" {
			// Insert includes before this line.
			var insertLines []string
			insertLines = append(insertLines, "")
			for _, inc := range includes {
				insertLines = append(insertLines, fmt.Sprintf("  include %q", inc))
			}
			after := append(insertLines, lines[i:]...)
			lines = append(lines[:i], after...)
			break
		}
	}
	return strings.Join(lines, "\n")
}

// migrateFragment migrates an included file (no spec wrapper).
// It wraps the content in a temporary spec, parses, migrates, then strips the wrapper.
func migrateFragment(src string, locators map[string]string) (string, error) {
	// Wrap in a temporary spec so the v2 parser can handle it.
	wrapped := "spec _Fragment {\n" + src + "\n}\n"
	s, err := v2parser.Parse(wrapped)
	if err != nil {
		return "", fmt.Errorf("parsing fragment: %w", err)
	}

	if locators != nil {
		s.Locators = locators
	}

	output, err := MigrateSpec(s)
	if err != nil {
		return "", err
	}

	// Strip the spec wrapper: remove first line ("spec _Fragment {") and last line ("}")
	// to return just the inner content.
	return stripSpecWrapper(output), nil
}

// stripSpecWrapper removes the `spec _Fragment { ... }` wrapper added for fragment parsing,
// returning the inner content with one level of indentation removed.
func stripSpecWrapper(output string) string {
	lines := strings.Split(output, "\n")
	if len(lines) < 2 {
		return output
	}

	// Skip first line (spec _Fragment {) and last non-empty line (})
	inner := lines[1:]
	for len(inner) > 0 && strings.TrimSpace(inner[len(inner)-1]) == "" {
		inner = inner[:len(inner)-1]
	}
	if len(inner) > 0 && strings.TrimSpace(inner[len(inner)-1]) == "}" {
		inner = inner[:len(inner)-1]
	}

	// Remove one level of indentation (2 spaces).
	var result []string
	for _, line := range inner {
		if strings.HasPrefix(line, "  ") {
			result = append(result, line[2:])
		} else {
			result = append(result, line)
		}
	}

	return strings.Join(result, "\n") + "\n"
}

// includeRe matches `include "path"` directives in spec source text.
var includeRe = regexp.MustCompile(`(?m)^\s*include\s+"([^"]+)"`)

// collectIncludes returns all files in the include tree rooted at path,
// in depth-first order with the root first.
func collectIncludes(rootPath string) ([]string, error) {
	seen := make(map[string]bool)
	var files []string

	var walk func(absPath string) error
	walk = func(absPath string) error {
		if seen[absPath] {
			return nil
		}
		seen[absPath] = true
		files = append(files, absPath)

		src, err := os.ReadFile(absPath)
		if err != nil {
			return err
		}

		dir := filepath.Dir(absPath)
		for _, m := range includeRe.FindAllStringSubmatch(string(src), -1) {
			relPath := m[1]
			incAbs, err := filepath.Abs(filepath.Join(dir, relPath))
			if err != nil {
				return fmt.Errorf("resolving include %q: %w", relPath, err)
			}
			if err := walk(incAbs); err != nil {
				return err
			}
		}
		return nil
	}

	if err := walk(rootPath); err != nil {
		return nil, err
	}
	return files, nil
}
