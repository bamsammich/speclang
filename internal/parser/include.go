package parser

import (
	"fmt"
	"os"
	"path/filepath"
)

// lexFile reads and lexes a file, tagging each token with the file path.
func lexFile(path string) ([]Token, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	tokens, err := Lex(string(data))
	if err != nil {
		return nil, fmt.Errorf("lexing %s: %w", path, err)
	}
	for i := range tokens {
		tokens[i].File = path
	}
	return tokens, nil
}

// resolveIncludes recursively resolves include directives in a token stream.
// dir is the directory of the file being processed (for relative path resolution).
// filePath is the absolute path of the current file (for circular detection).
// seen tracks files currently in the include chain (ancestors only) for cycle detection.
// spliced tracks every file already incorporated into the output (across all chains);
// a file in spliced is silently skipped on a second visit — this is how diamond
// includes (A→B→X and A→C→X) are made safe without a duplicate-declaration error.
//
// Security note: include paths are NOT sandboxed to a root directory. A spec may
// include any file the invoking user can read (absolute or relative, inside or
// outside the spec directory). This is intentional — specs are already executable
// code (process adapter, docker volumes, arbitrary HTTP), so a path-containment
// policy on include would be security theater. The trust boundary is the spec
// file itself; see SECURITY.md.
func resolveIncludes(
	tokens []Token,
	dir string,
	filePath string,
	seen map[string]bool,
) ([]Token, error) {
	return resolveIncludesInner(tokens, dir, filePath, seen, make(map[string]bool))
}

func resolveIncludesInner(
	tokens []Token,
	dir string,
	filePath string,
	seen map[string]bool,
	spliced map[string]bool,
) ([]Token, error) {
	if seen == nil {
		seen = make(map[string]bool)
	}
	seen[filePath] = true
	defer delete(seen, filePath)

	spliced[filePath] = true

	var result []Token
	for i := 0; i < len(tokens); i++ {
		if tokens[i].Type != TokenInclude {
			if tokens[i].Type == TokenEOF {
				continue // drop intermediate EOFs
			}
			result = append(result, tokens[i])
			continue
		}

		resolved, newIdx, err := processInclude(tokens, i, dir, seen, spliced)
		if err != nil {
			return nil, err
		}
		i = newIdx
		result = append(result, resolved...)
	}

	// Add EOF at the end
	result = append(result, Token{Type: TokenEOF, File: filePath})
	return result, nil
}

// processInclude handles a single include directive: validates the path token,
// resolves the file, lexes it, and recursively resolves nested includes.
// Returns the resolved tokens (with trailing EOF stripped) and the updated
// token index pointing at the path token.
func processInclude(tokens []Token, i int, dir string, seen map[string]bool, spliced map[string]bool) ([]Token, int, error) {
	includeTok := tokens[i]
	i++
	if i >= len(tokens) ||
		tokens[i].Type != TokenString { //nolint:gosec // bounds check on left side of || guards the access
		return nil, i, fmt.Errorf("%s:%d:%d: include requires a string path",
			includeTok.File, includeTok.Line, includeTok.Col)
	}
	relPath := tokens[i].Value //nolint:gosec // i is bounds-checked on line above

	absInclude, err := filepath.Abs(filepath.Join(dir, relPath))
	if err != nil {
		return nil, i, fmt.Errorf("%s:%d:%d: resolving include path %q: %w",
			includeTok.File, includeTok.Line, includeTok.Col, relPath, err)
	}

	if seen[absInclude] {
		return nil, i, fmt.Errorf("%s:%d:%d: circular include detected: %s",
			includeTok.File, includeTok.Line, includeTok.Col, absInclude)
	}

	// Diamond-include dedup: if this file has already been spliced into the
	// output stream (via a different include chain), skip it silently.
	if spliced[absInclude] {
		return nil, i, nil
	}

	included, err := lexFile(absInclude)
	if err != nil {
		return nil, i, fmt.Errorf("%s:%d:%d: %w",
			includeTok.File, includeTok.Line, includeTok.Col, err)
	}

	resolved, err := resolveIncludesInner(included, filepath.Dir(absInclude), absInclude, seen, spliced)
	if err != nil {
		return nil, i, err
	}

	// Strip the trailing EOF (each resolveIncludesInner call appends its own)
	if len(resolved) > 0 && resolved[len(resolved)-1].Type == TokenEOF {
		resolved = resolved[:len(resolved)-1]
	}

	return resolved, i, nil
}

// duplicateHint is the guidance appended after the machine-readable first line
// of a duplicate-declaration error.
const duplicateHint = `
If both specs need this declaration, factor it into a shared file (e.g. shared/models.spec) and ` + "`include`" + ` that file from both.
Avoid using ` + "`include`" + ` to compose whole runnable specs into a super-spec; use ` + "`specrun verify <glob>`" + ` instead to run them independently.`

// dupDecl formats a "duplicate declaration" error.
// kind is the declaration kind (e.g. "model", "scope").
// name is the declared name.
// firstFile and secondFile are the source files; either may be empty.
func dupDecl(kind, name, firstFile, secondFile string) error {
	var loc string
	switch {
	case firstFile != "" && secondFile != "" && firstFile != secondFile:
		loc = fmt.Sprintf(" declared in both %q and %q", firstFile, secondFile)
	case firstFile != "":
		loc = fmt.Sprintf(" in %q", firstFile)
	case secondFile != "":
		loc = fmt.Sprintf(" in %q", secondFile)
	}
	return fmt.Errorf("duplicate declaration: %s %q%s%s", kind, name, loc, duplicateHint)
}

// validateNoDuplicates checks that model, enum, action, scope, and top-level
// contract names are unique across the fully resolved token stream.
func validateNoDuplicates(spec *Spec) error {
	models := make(map[string]string) // name → file
	for _, m := range spec.Models {
		if prev, dup := models[m.Name]; dup {
			return dupDecl("model", m.Name, prev, m.Pos.File)
		}
		models[m.Name] = m.Pos.File
	}

	enums := make(map[string]string)
	for _, e := range spec.Enums {
		if prev, dup := enums[e.Name]; dup {
			return dupDecl("enum", e.Name, prev, e.Pos.File)
		}
		enums[e.Name] = e.Pos.File
	}

	actions := make(map[string]string)
	for _, a := range spec.Actions {
		if prev, dup := actions[a.Name]; dup {
			return dupDecl("action", a.Name, prev, a.Pos.File)
		}
		actions[a.Name] = a.Pos.File
	}

	scopes := make(map[string]string)
	for _, s := range spec.Scopes {
		if prev, dup := scopes[s.Name]; dup {
			return dupDecl("scope", s.Name, prev, s.Pos.File)
		}
		scopes[s.Name] = s.Pos.File
	}

	contracts := make(map[string]string)
	for _, c := range spec.Contracts {
		if prev, dup := contracts[c.Name]; dup {
			return dupDecl("contract", c.Name, prev, c.Pos.File)
		}
		contracts[c.Name] = c.Pos.File
	}

	services := make(map[string]string)
	for _, svc := range spec.Services {
		if prev, dup := services[svc.Name]; dup {
			return dupDecl("service", svc.Name, prev, svc.Pos.File)
		}
		services[svc.Name] = svc.Pos.File
	}

	return nil
}
