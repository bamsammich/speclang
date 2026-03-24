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
// seen tracks files currently in the include chain (ancestors only).
func resolveIncludes(
	tokens []Token,
	dir string,
	filePath string,
	seen map[string]bool,
) ([]Token, error) {
	if seen == nil {
		seen = make(map[string]bool)
	}
	seen[filePath] = true
	defer delete(seen, filePath)

	var result []Token
	for i := 0; i < len(tokens); i++ {
		if tokens[i].Type != TokenInclude {
			if tokens[i].Type == TokenEOF {
				continue // drop intermediate EOFs
			}
			result = append(result, tokens[i])
			continue
		}

		// Consume include + string path
		includeTok := tokens[i]
		i++
		if i >= len(tokens) || tokens[i].Type != TokenString {
			return nil, fmt.Errorf("%s:%d:%d: include requires a string path",
				includeTok.File, includeTok.Line, includeTok.Col)
		}
		relPath := tokens[i].Value

		// Resolve relative to the including file's directory
		absInclude, err := filepath.Abs(filepath.Join(dir, relPath))
		if err != nil {
			return nil, fmt.Errorf("%s:%d:%d: resolving include path %q: %w",
				includeTok.File, includeTok.Line, includeTok.Col, relPath, err)
		}

		// Circular detection
		if seen[absInclude] {
			return nil, fmt.Errorf("%s:%d:%d: circular include detected: %s",
				includeTok.File, includeTok.Line, includeTok.Col, absInclude)
		}

		// Lex the included file
		included, err := lexFile(absInclude)
		if err != nil {
			return nil, fmt.Errorf("%s:%d:%d: %w",
				includeTok.File, includeTok.Line, includeTok.Col, err)
		}

		// Recursively resolve includes in the included file
		resolved, err := resolveIncludes(included, filepath.Dir(absInclude), absInclude, seen)
		if err != nil {
			return nil, err
		}

		// Strip the trailing EOF from the included file's resolved tokens
		// (each resolveIncludes call appends its own EOF; we only want one at the end)
		if len(resolved) > 0 && resolved[len(resolved)-1].Type == TokenEOF {
			resolved = resolved[:len(resolved)-1]
		}

		result = append(result, resolved...)
	}

	// Add EOF at the end
	result = append(result, Token{Type: TokenEOF, File: filePath})
	return result, nil
}

// validateNoDuplicates checks that model names and scope names are unique.
func validateNoDuplicates(spec *Spec) error {
	models := make(map[string]bool)
	for _, m := range spec.Models {
		if models[m.Name] {
			return fmt.Errorf("duplicate model %q", m.Name)
		}
		models[m.Name] = true
	}

	scopes := make(map[string]bool)
	for _, s := range spec.Scopes {
		if scopes[s.Name] {
			return fmt.Errorf("duplicate scope %q", s.Name)
		}
		scopes[s.Name] = true
	}

	return nil
}
