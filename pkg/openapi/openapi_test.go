package openapi

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/bamsammich/speclang/v2/pkg/parser"
)

func testdataPath(name string) string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "testdata", "openapi", name)
}

func TestResolve_Petstore(t *testing.T) {
	t.Parallel()
	r := &Resolver{}
	models, scopes, err := r.Resolve(testdataPath("petstore.yaml"))
	if err != nil {
		t.Fatal(err)
	}

	if len(models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(models))
	}

	// Models are sorted alphabetically
	assertModelName(t, models, 0, "Owner")
	assertModelName(t, models, 1, "Pet")

	if len(scopes) != 2 {
		t.Fatalf("expected 2 scopes, got %d", len(scopes))
	}
}

func TestConvertSchemas_Petstore(t *testing.T) {
	t.Parallel()
	doc, err := loadDocument(testdataPath("petstore.yaml"))
	if err != nil {
		t.Fatal(err)
	}

	models := convertSchemas(doc)
	if len(models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(models))
	}

	// Find Pet model
	var pet *parser.Model
	for _, m := range models {
		if m.Name == "Pet" {
			pet = m
		}
	}
	if pet == nil {
		t.Fatal("Pet model not found")
	}

	if len(pet.Fields) != 3 {
		t.Fatalf("expected 3 fields in Pet, got %d", len(pet.Fields))
	}

	// Fields are sorted: id, name, tag
	assertField(t, pet.Fields[0], "id", "int", false)
	assertField(t, pet.Fields[1], "name", "string", false)
	assertField(t, pet.Fields[2], "tag", "string", true)
}

func TestConvertSchemas_Constraints(t *testing.T) {
	t.Parallel()
	doc, err := loadDocument(testdataPath("constraints.yaml"))
	if err != nil {
		t.Fatal(err)
	}

	models := convertSchemas(doc)
	if len(models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(models))
	}

	m := models[0]
	if m.Name != "BoundedItem" {
		t.Fatalf("expected model name BoundedItem, got %q", m.Name)
	}

	// Fields: price, quantity, rating (sorted)
	if len(m.Fields) != 3 {
		t.Fatalf("expected 3 fields, got %d", len(m.Fields))
	}

	// price: exclusiveMinimum: 0, exclusiveMaximum: 10000
	price := m.Fields[0]
	if price.Name != "price" {
		t.Fatalf("expected field 'price', got %q", price.Name)
	}
	if price.Constraint == nil {
		t.Fatal("expected constraint on price")
	}

	// quantity: minimum: 1, maximum: 100
	quantity := m.Fields[1]
	if quantity.Name != "quantity" {
		t.Fatalf("expected field 'quantity', got %q", quantity.Name)
	}
	if quantity.Constraint == nil {
		t.Fatal("expected constraint on quantity")
	}

	// rating: minimum: 0 (only lower bound)
	rating := m.Fields[2]
	if rating.Name != "rating" {
		t.Fatalf("expected field 'rating', got %q", rating.Name)
	}
	if rating.Constraint == nil {
		t.Fatal("expected constraint on rating")
	}
}

func TestConvertSchemas_Refs(t *testing.T) {
	t.Parallel()
	doc, err := loadDocument(testdataPath("refs.yaml"))
	if err != nil {
		t.Fatal(err)
	}

	models := convertSchemas(doc)

	var order *parser.Model
	for _, m := range models {
		if m.Name == "Order" {
			order = m
		}
	}
	if order == nil {
		t.Fatal("Order model not found")
	}

	// Find customer field — should be a model reference
	var customerField *parser.Field
	for _, f := range order.Fields {
		if f.Name == "customer" {
			customerField = f
		}
	}
	if customerField == nil {
		t.Fatal("customer field not found")
	}
	if customerField.Type.Name != "Customer" {
		t.Errorf("expected type 'Customer', got %q", customerField.Type.Name)
	}
	if customerField.Type.Optional {
		t.Error("customer should be required (not optional)")
	}
}

func TestConvertSchemas_Empty(t *testing.T) {
	t.Parallel()
	doc, err := loadDocument(testdataPath("empty.yaml"))
	if err != nil {
		t.Fatal(err)
	}

	models := convertSchemas(doc)
	if len(models) != 0 {
		t.Errorf("expected 0 models, got %d", len(models))
	}
}

func TestConvertPaths_Petstore(t *testing.T) {
	t.Parallel()
	doc, err := loadDocument(testdataPath("petstore.yaml"))
	if err != nil {
		t.Fatal(err)
	}

	scopes := convertPaths(doc)
	if len(scopes) != 2 {
		t.Fatalf("expected 2 scopes, got %d", len(scopes))
	}

	// Scopes sorted by name: create_pet, list_pets
	assertScopeName(t, scopes, 0, "create_pet")
	assertScopeName(t, scopes, 1, "list_pets")

	// create_pet should have config with path and method
	createPet := scopes[0]
	assertConfigValue(t, createPet, "path", "/pets")
	assertConfigValue(t, createPet, "method", "POST")

	// create_pet should have a contract with input (from request body)
	if createPet.Contract == nil {
		t.Fatal("create_pet should have a contract")
	}
	if len(createPet.Contract.Input) == 0 {
		t.Error("create_pet should have contract input fields")
	}
	if len(createPet.Contract.Output) == 0 {
		t.Error("create_pet should have contract output fields")
	}

	// list_pets: GET with no request body
	listPets := scopes[1]
	assertConfigValue(t, listPets, "path", "/pets")
	assertConfigValue(t, listPets, "method", "GET")
}

func TestConvertPaths_Empty(t *testing.T) {
	t.Parallel()
	doc, err := loadDocument(testdataPath("empty.yaml"))
	if err != nil {
		t.Fatal(err)
	}

	scopes := convertPaths(doc)
	if len(scopes) != 0 {
		t.Errorf("expected 0 scopes, got %d", len(scopes))
	}
}

func TestSanitizeScopeName(t *testing.T) {
	t.Parallel()
	tests := []struct {
		method, path string
		want         string
	}{
		{"GET", "/pets", "get_pets"},
		{"POST", "/api/v1/accounts/transfer", "post_api_v1_accounts_transfer"},
		{"GET", "/pets/{petId}", "get_pets_petId"},
		{"DELETE", "/api/v1/users/{id}/roles", "delete_api_v1_users_id_roles"},
	}
	for _, tt := range tests {
		t.Run(tt.method+"_"+tt.path, func(t *testing.T) {
			t.Parallel()
			got := sanitizeScopeName(tt.method, tt.path)
			if got != tt.want {
				t.Errorf(
					"sanitizeScopeName(%q, %q) = %q, want %q",
					tt.method, tt.path, got, tt.want,
				)
			}
		})
	}
}

func TestLoadDocument_InvalidFile(t *testing.T) {
	t.Parallel()
	_, err := loadDocument("/nonexistent/file.yaml")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

// --- helpers ---

func assertModelName(t *testing.T, models []*parser.Model, idx int, name string) {
	t.Helper()
	if models[idx].Name != name {
		t.Errorf("models[%d].Name = %q, want %q", idx, models[idx].Name, name)
	}
}

func assertScopeName(t *testing.T, scopes []*parser.Scope, idx int, name string) {
	t.Helper()
	if scopes[idx].Name != name {
		t.Errorf("scopes[%d].Name = %q, want %q", idx, scopes[idx].Name, name)
	}
}

func assertField(t *testing.T, f *parser.Field, name, typeName string, optional bool) {
	t.Helper()
	if f.Name != name {
		t.Errorf("field.Name = %q, want %q", f.Name, name)
	}
	if f.Type.Name != typeName {
		t.Errorf("field %q: Type.Name = %q, want %q", name, f.Type.Name, typeName)
	}
	if f.Type.Optional != optional {
		t.Errorf("field %q: Optional = %v, want %v", name, f.Type.Optional, optional)
	}
}

func assertConfigValue(t *testing.T, scope *parser.Scope, key, expected string) {
	t.Helper()
	expr, ok := scope.Config[key]
	if !ok {
		t.Errorf("scope %q: config key %q not found", scope.Name, key)
		return
	}
	lit, ok := expr.(parser.LiteralString)
	if !ok {
		t.Errorf("scope %q: config %q is not a LiteralString", scope.Name, key)
		return
	}
	if lit.Value != expected {
		t.Errorf("scope %q: config %q = %q, want %q", scope.Name, key, lit.Value, expected)
	}
}
