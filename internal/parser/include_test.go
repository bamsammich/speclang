package parser

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveIncludes_Basic(t *testing.T) {
	t.Parallel()
	root := filepath.Join("..", "..", "testdata", "include", "v3basic", "root.spec")
	tokens, err := lexFile(root)
	if err != nil {
		t.Fatalf("lexing root: %v", err)
	}

	absRoot, err := filepath.Abs(root)
	if err != nil {
		t.Fatal(err)
	}

	resolved, err := resolveIncludes(tokens, filepath.Dir(absRoot), absRoot, nil)
	if err != nil {
		t.Fatalf("resolveIncludes: %v", err)
	}

	// Should have no TokenInclude remaining
	for _, tok := range resolved {
		if tok.Type == TokenInclude {
			t.Fatal("resolved tokens still contain TokenInclude")
		}
	}

	// Should end with exactly one EOF
	if resolved[len(resolved)-1].Type != TokenEOF {
		t.Fatal("expected EOF as last token")
	}

	// Count EOFs — should be exactly one
	eofCount := 0
	for _, tok := range resolved {
		if tok.Type == TokenEOF {
			eofCount++
		}
	}
	if eofCount != 1 {
		t.Fatalf("expected exactly 1 EOF, got %d", eofCount)
	}

	// Should contain tokens from included files
	hasModel := false
	hasScope := false
	for _, tok := range resolved {
		if tok.Type == TokenModel {
			hasModel = true
		}
		if tok.Type == TokenScope {
			hasScope = true
		}
	}
	if !hasModel {
		t.Error("expected model token from included models.spec")
	}
	if !hasScope {
		t.Error("expected scope token from included scopes.spec")
	}
}

func TestResolveIncludes_Nested(t *testing.T) {
	t.Parallel()
	root := filepath.Join("..", "..", "testdata", "include", "nested", "root.spec")
	tokens, err := lexFile(root)
	if err != nil {
		t.Fatal(err)
	}

	absRoot, err := filepath.Abs(root)
	if err != nil {
		t.Fatal(err)
	}

	resolved, err := resolveIncludes(tokens, filepath.Dir(absRoot), absRoot, nil)
	if err != nil {
		t.Fatalf("resolveIncludes: %v", err)
	}

	// Should contain tokens from leaf.spec (included via mid.spec)
	modelCount := 0
	for _, tok := range resolved {
		if tok.Type == TokenModel {
			modelCount++
		}
	}
	if modelCount != 2 {
		t.Fatalf(
			"expected 2 model tokens (Item from leaf + Container from mid), got %d",
			modelCount,
		)
	}
}

func TestResolveIncludes_Circular(t *testing.T) {
	t.Parallel()
	root := filepath.Join("..", "..", "testdata", "include", "circular", "a.spec")
	tokens, err := lexFile(root)
	if err != nil {
		t.Fatal(err)
	}

	absRoot, err := filepath.Abs(root)
	if err != nil {
		t.Fatal(err)
	}

	_, err = resolveIncludes(tokens, filepath.Dir(absRoot), absRoot, nil)
	if err == nil {
		t.Fatal("expected circular include error")
	}

	if !strings.Contains(err.Error(), "circular") {
		t.Fatalf("expected error to mention 'circular', got: %v", err)
	}
}

func TestParseFile_WithIncludes(t *testing.T) {
	t.Parallel()
	root := filepath.Join("..", "..", "testdata", "include", "v3basic", "root.spec")
	spec, err := ParseFile(root)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}

	if spec.Name != "TestAPI" {
		t.Errorf("expected spec name TestAPI, got %q", spec.Name)
	}
	if len(spec.Models) != 1 || spec.Models[0].Name != "Account" {
		t.Errorf("expected 1 model Account, got %v", spec.Models)
	}
	if len(spec.Scopes) != 1 || spec.Scopes[0].Name != "transfer" {
		t.Errorf("expected 1 scope transfer, got %v", spec.Scopes)
	}
}

func TestParseFile_NestedIncludes(t *testing.T) {
	t.Parallel()
	root := filepath.Join("..", "..", "testdata", "include", "nested", "root.spec")
	spec, err := ParseFile(root)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}

	if len(spec.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(spec.Models))
	}
	names := map[string]bool{}
	for _, m := range spec.Models {
		names[m.Name] = true
	}
	if !names["Item"] || !names["Container"] {
		t.Errorf("expected models Item and Container, got %v", names)
	}
}

func TestParseFile_CircularIncludeError(t *testing.T) {
	t.Parallel()
	root := filepath.Join("..", "..", "testdata", "include", "circular", "a.spec")
	_, err := ParseFile(root)
	if err == nil {
		t.Fatal("expected error for circular include")
	}
}

func TestParseFile_DuplicateModelError(t *testing.T) {
	t.Parallel()
	root := filepath.Join("..", "..", "testdata", "include", "duplicate", "root.spec")
	_, err := ParseFile(root)
	if err == nil {
		t.Fatal("expected error for duplicate model")
	}
	if !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("expected error to mention 'duplicate', got: %v", err)
	}
	if !strings.Contains(err.Error(), "Account") {
		t.Fatalf("expected error to mention 'Account', got: %v", err)
	}
}

func TestParseFile_DuplicateScopeError(t *testing.T) {
	t.Parallel()
	root := filepath.Join("..", "..", "testdata", "include", "v3duplicate_scope", "root.spec")
	_, err := ParseFile(root)
	if err == nil {
		t.Fatal("expected error for duplicate scope")
	}
	if !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("expected error to mention 'duplicate', got: %v", err)
	}
	if !strings.Contains(err.Error(), "transfer") {
		t.Fatalf("expected error to mention 'transfer', got: %v", err)
	}
}

func TestResolveIncludes_MissingFile(t *testing.T) {
	t.Parallel()
	tokens := []Token{
		{Type: TokenInclude, Value: "include", File: "test.spec", Line: 1, Col: 1},
		{Type: TokenString, Value: "nonexistent.spec", File: "test.spec", Line: 1, Col: 9},
		{Type: TokenEOF, File: "test.spec"},
	}

	_, err := resolveIncludes(tokens, t.TempDir(), "/fake/test.spec", nil)
	if err == nil {
		t.Fatal("expected error for missing include file")
	}
}

func TestResolveIncludes_NonStringAfterInclude(t *testing.T) {
	t.Parallel()
	tokens := []Token{
		{Type: TokenInclude, Value: "include", File: "test.spec", Line: 1, Col: 1},
		{Type: TokenIdent, Value: "foo", File: "test.spec", Line: 1, Col: 9},
		{Type: TokenEOF, File: "test.spec"},
	}

	_, err := resolveIncludes(tokens, t.TempDir(), "/fake/test.spec", nil)
	if err == nil {
		t.Fatal("expected error when include is followed by non-string")
	}
	if !strings.Contains(err.Error(), "string path") {
		t.Fatalf("expected error about string path, got: %v", err)
	}
}
