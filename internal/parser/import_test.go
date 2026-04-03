package parser

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// mockResolver implements ImportResolver for testing.
type mockResolver struct {
	err    error
	models []*Model
	scopes []*Scope
}

func (m *mockResolver) Resolve(absPath string) ([]*Model, []*Scope, error) {
	return m.models, m.scopes, m.err
}

// writeSpecFile is a test helper that writes spec content to a file and
// fails the test on error.
func writeSpecFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestLexImportKeyword(t *testing.T) {
	t.Parallel()
	tokens, err := Lex(`import openapi("schema.yaml")`)
	if err != nil {
		t.Fatal(err)
	}
	if tokens[0].Type != TokenImport {
		t.Errorf("expected TokenImport, got %s", tokens[0].Type)
	}
	if tokens[0].Value != "import" {
		t.Errorf("expected value 'import', got %q", tokens[0].Value)
	}
}

func TestParseImport_Basic(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	specFile := filepath.Join(dir, "test.spec")
	writeSpecFile(t, specFile, `
spec Test {
  import test("schema.yaml")
}
`)

	resolver := &mockResolver{
		models: []*Model{
			{Name: "Pet", Fields: []*Field{
				{Name: "id", Type: TypeExpr{Name: "int"}},
				{Name: "name", Type: TypeExpr{Name: "string"}},
			}},
		},
		scopes: []*Scope{
			{
				Name: "list_pets",
				Use:  "http",
				Config: map[string]Expr{
					"path":   LiteralString{Value: "/pets"},
					"method": LiteralString{Value: "GET"},
				},
			},
		},
	}

	registry := ImportRegistry{"test": resolver}
	spec, err := ParseFileWithImports(specFile, registry)
	if err != nil {
		t.Fatal(err)
	}

	if len(spec.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(spec.Models))
	}
	if spec.Models[0].Name != "Pet" {
		t.Errorf("expected model name 'Pet', got %q", spec.Models[0].Name)
	}
	if len(spec.Models[0].Fields) != 2 {
		t.Errorf("expected 2 fields, got %d", len(spec.Models[0].Fields))
	}

	if len(spec.Scopes) != 1 {
		t.Fatalf("expected 1 scope, got %d", len(spec.Scopes))
	}
	if spec.Scopes[0].Name != "list_pets" {
		t.Errorf("expected scope name 'list_pets', got %q", spec.Scopes[0].Name)
	}
}

func TestParseImport_MergesWithHandWritten(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	specFile := filepath.Join(dir, "test.spec")
	writeSpecFile(t, specFile, `
spec Test {
  model Local {
    id: int
  }
  import test("schema.yaml")
  scope local_scope {
    contract {
      input { x: int }
      output { y: int }
    }
  }
}
`)

	resolver := &mockResolver{
		models: []*Model{
			{Name: "Imported", Fields: []*Field{{Name: "a", Type: TypeExpr{Name: "string"}}}},
		},
		scopes: []*Scope{
			{Name: "imported_scope"},
		},
	}

	registry := ImportRegistry{"test": resolver}
	spec, err := ParseFileWithImports(specFile, registry)
	if err != nil {
		t.Fatal(err)
	}

	if len(spec.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(spec.Models))
	}
	if len(spec.Scopes) != 2 {
		t.Fatalf("expected 2 scopes, got %d", len(spec.Scopes))
	}

	modelNames := map[string]bool{}
	for _, m := range spec.Models {
		modelNames[m.Name] = true
	}
	if !modelNames["Local"] || !modelNames["Imported"] {
		t.Errorf("expected models Local and Imported, got %v", modelNames)
	}

	scopeNames := map[string]bool{}
	for _, s := range spec.Scopes {
		scopeNames[s.Name] = true
	}
	if !scopeNames["local_scope"] || !scopeNames["imported_scope"] {
		t.Errorf("expected scopes local_scope and imported_scope, got %v", scopeNames)
	}
}

func TestParseImport_UnknownAdapter(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	specFile := filepath.Join(dir, "test.spec")
	writeSpecFile(t, specFile, `
spec Test {
  import unknown("schema.yaml")
}
`)

	registry := ImportRegistry{"openapi": &mockResolver{}}
	_, err := ParseFileWithImports(specFile, registry)
	if err == nil {
		t.Fatal("expected error for unknown adapter")
	}
	if !strings.Contains(err.Error(), "unknown import adapter") {
		t.Errorf("expected 'unknown import adapter' in error, got: %v", err)
	}
}

func TestParseImport_NilRegistry(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	specFile := filepath.Join(dir, "test.spec")
	writeSpecFile(t, specFile, `
spec Test {
  import openapi("schema.yaml")
}
`)

	_, err := ParseFileWithImports(specFile, nil)
	if err == nil {
		t.Fatal("expected error for nil registry")
	}
	if !strings.Contains(err.Error(), "no import resolvers registered") {
		t.Errorf("expected 'no import resolvers registered' in error, got: %v", err)
	}
}

func TestParseImport_ResolverError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	specFile := filepath.Join(dir, "test.spec")
	writeSpecFile(t, specFile, `
spec Test {
  import test("bad.yaml")
}
`)

	resolver := &mockResolver{err: fmt.Errorf("file not found")}
	registry := ImportRegistry{"test": resolver}
	_, err := ParseFileWithImports(specFile, registry)
	if err == nil {
		t.Fatal("expected error from resolver")
	}
	if !strings.Contains(err.Error(), "file not found") {
		t.Errorf("expected 'file not found' in error, got: %v", err)
	}
}

func TestParseImport_DuplicateModelName(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	specFile := filepath.Join(dir, "test.spec")
	writeSpecFile(t, specFile, `
spec Test {
  model Pet {
    id: int
  }
  import test("schema.yaml")
}
`)

	resolver := &mockResolver{
		models: []*Model{
			{Name: "Pet", Fields: []*Field{{Name: "name", Type: TypeExpr{Name: "string"}}}},
		},
	}

	registry := ImportRegistry{"test": resolver}
	_, err := ParseFileWithImports(specFile, registry)
	if err == nil {
		t.Fatal("expected error for duplicate model name")
	}
	if !strings.Contains(err.Error(), "duplicate") && !strings.Contains(err.Error(), "Pet") {
		t.Errorf("expected duplicate error mentioning 'Pet', got: %v", err)
	}
}

func TestParseImport_DuplicateScopeName(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	specFile := filepath.Join(dir, "test.spec")
	writeSpecFile(t, specFile, `
spec Test {
  scope my_scope {
    contract {
      input { x: int }
      output { y: int }
    }
  }
  import test("schema.yaml")
}
`)

	resolver := &mockResolver{
		scopes: []*Scope{
			{Name: "my_scope"},
		},
	}

	registry := ImportRegistry{"test": resolver}
	_, err := ParseFileWithImports(specFile, registry)
	if err == nil {
		t.Fatal("expected error for duplicate scope name")
	}
	if !strings.Contains(err.Error(), "duplicate") && !strings.Contains(err.Error(), "my_scope") {
		t.Errorf("expected duplicate error mentioning 'my_scope', got: %v", err)
	}
}

func TestParseImport_EmptyResult(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	specFile := filepath.Join(dir, "test.spec")
	writeSpecFile(t, specFile, `
spec Test {
  import test("empty.yaml")
}
`)

	resolver := &mockResolver{models: nil, scopes: nil}
	registry := ImportRegistry{"test": resolver}
	spec, err := ParseFileWithImports(specFile, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(spec.Models) != 0 {
		t.Errorf("expected 0 models, got %d", len(spec.Models))
	}
	if len(spec.Scopes) != 0 {
		t.Errorf("expected 0 scopes, got %d", len(spec.Scopes))
	}
}

func TestParseImport_SyntaxError_MissingParen(t *testing.T) {
	t.Parallel()
	src := `
spec Test {
  import openapi "schema.yaml"
}
`
	_, err := Parse(src)
	if err == nil {
		t.Fatal("expected parse error for missing parens")
	}
}

func TestParseImport_SyntaxError_MissingPath(t *testing.T) {
	t.Parallel()
	src := `
spec Test {
  import openapi()
}
`
	_, err := Parse(src)
	if err == nil {
		t.Fatal("expected parse error for missing path")
	}
}
