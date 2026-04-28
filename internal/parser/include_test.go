package parser

import (
	"os"
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
	root := filepath.Join("..", "..", "testdata", "include", "basic_v4", "root.spec")
	spec, err := ParseFile(root)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
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
	root := filepath.Join("..", "..", "testdata", "include", "nested_v4", "root.spec")
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
	root := filepath.Join("..", "..", "testdata", "include", "duplicate_v4", "root.spec")
	_, err := ParseFile(root)
	if err == nil {
		t.Fatal("expected error for duplicate model")
	}
	msg := err.Error()
	// Machine-readable first line
	if !strings.HasPrefix(msg, `duplicate declaration: model "Account"`) {
		t.Fatalf("expected error to start with machine-readable shape, got: %v", err)
	}
	// Must name both files
	if !strings.Contains(msg, "models_a.spec") || !strings.Contains(msg, "models_b.spec") {
		t.Fatalf("expected error to cite both include files, got: %v", err)
	}
	// Must include actionable hint
	if !strings.Contains(msg, "specrun verify") {
		t.Fatalf("expected error to include glob hint, got: %v", err)
	}
}

func TestParseFile_DuplicateScopeError(t *testing.T) {
	t.Parallel()
	root := filepath.Join("..", "..", "testdata", "include", "duplicate_scope_v4", "root.spec")
	_, err := ParseFile(root)
	if err == nil {
		t.Fatal("expected error for duplicate scope")
	}
	msg := err.Error()
	// Machine-readable first line
	if !strings.HasPrefix(msg, `duplicate declaration: scope "transfer"`) {
		t.Fatalf("expected error to start with machine-readable shape, got: %v", err)
	}
	// Must name both files
	if !strings.Contains(msg, "scope_a.spec") || !strings.Contains(msg, "scope_b.spec") {
		t.Fatalf("expected error to cite both include files, got: %v", err)
	}
	// Must include actionable hint
	if !strings.Contains(msg, "specrun verify") {
		t.Fatalf("expected error to include glob hint, got: %v", err)
	}
}

// TestParseFile_DuplicateEnumError verifies that duplicate named enum declarations
// produce the rich error message.
func TestParseFile_DuplicateEnumError(t *testing.T) {
	t.Parallel()
	// Inline spec with duplicate enum declarations (no include fixture needed).
	src := `
enum Status { active, inactive }
enum Status { pending }
`
	_, err := Parse(src)
	if err != nil {
		t.Fatalf("Parse should succeed (validateNoDuplicates runs in ParseFile, not Parse): %v", err)
	}

	// Use ParseFile path to exercise validateNoDuplicates.
	dir := t.TempDir()
	specFile := filepath.Join(dir, "dup_enum.spec")
	if writeErr := os.WriteFile(specFile, []byte(src), 0o600); writeErr != nil {
		t.Fatalf("writing temp spec: %v", writeErr)
	}
	_, err = ParseFile(specFile)
	if err == nil {
		t.Fatal("expected error for duplicate enum")
	}
	msg := err.Error()
	if !strings.HasPrefix(msg, `duplicate declaration: enum "Status"`) {
		t.Fatalf("expected machine-readable shape, got: %v", err)
	}
	if !strings.Contains(msg, "specrun verify") {
		t.Fatalf("expected hint, got: %v", err)
	}
}

// TestParseFile_DuplicateActionError verifies that duplicate top-level action
// declarations produce the rich error message.
func TestParseFile_DuplicateActionError(t *testing.T) {
	t.Parallel()
	src := `
action setup() { }
action setup() { }
`
	dir := t.TempDir()
	specFile := filepath.Join(dir, "dup_action.spec")
	if writeErr := os.WriteFile(specFile, []byte(src), 0o600); writeErr != nil {
		t.Fatalf("writing temp spec: %v", writeErr)
	}
	_, err := ParseFile(specFile)
	if err == nil {
		t.Fatal("expected error for duplicate action")
	}
	msg := err.Error()
	if !strings.HasPrefix(msg, `duplicate declaration: action "setup"`) {
		t.Fatalf("expected machine-readable shape, got: %v", err)
	}
	if !strings.Contains(msg, "specrun verify") {
		t.Fatalf("expected hint, got: %v", err)
	}
}

// TestParseFile_DiamondInclude verifies that a file reached via multiple
// include chains (A→B→X, A→C→X, A→X directly) is spliced exactly once.
// Before the include-once fix this produced: duplicate declaration: model "Shared".
func TestParseFile_DiamondInclude(t *testing.T) {
	t.Parallel()
	root := filepath.Join("..", "..", "testdata", "include", "diamond", "root.spec")
	spec, err := ParseFile(root)
	if err != nil {
		t.Fatalf("ParseFile diamond: unexpected error: %v", err)
	}
	// shared.spec declares Shared and Result — each must appear exactly once.
	counts := make(map[string]int)
	for _, m := range spec.Models {
		counts[m.Name]++
	}
	for _, name := range []string{"Shared", "Result"} {
		if counts[name] != 1 {
			t.Errorf("expected model %q exactly once, got %d", name, counts[name])
		}
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

// TestInclude_SameDir verifies that a same-directory include works.
func TestInclude_SameDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// Write a shared fragment.
	shared := filepath.Join(dir, "shared.spec")
	if err := os.WriteFile(shared, []byte("model Shared { id: string }\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	// Write root spec that includes it.
	root := filepath.Join(dir, "root.spec")
	if err := os.WriteFile(root, []byte(`include "shared.spec"`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	spec, err := ParseFile(root)
	if err != nil {
		t.Fatalf("expected same-dir include to succeed: %v", err)
	}
	if len(spec.Models) != 1 || spec.Models[0].Name != "Shared" {
		t.Errorf("expected model Shared, got %v", spec.Models)
	}
}

// TestInclude_Subdir verifies that an include of a file in a subdirectory works.
func TestInclude_Subdir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	sub := filepath.Join(dir, "models")
	if err := os.MkdirAll(sub, 0o750); err != nil {
		t.Fatal(err)
	}
	shared := filepath.Join(sub, "account.spec")
	if err := os.WriteFile(shared, []byte("model Account { id: string }\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(dir, "root.spec")
	if err := os.WriteFile(root, []byte(`include "models/account.spec"`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	spec, err := ParseFile(root)
	if err != nil {
		t.Fatalf("expected sub-dir include to succeed: %v", err)
	}
	if len(spec.Models) != 1 || spec.Models[0].Name != "Account" {
		t.Errorf("expected model Account, got %v", spec.Models)
	}
}

