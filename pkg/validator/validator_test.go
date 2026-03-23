package validator

import (
	"fmt"
	"testing"

	"github.com/bamsammich/speclang/v2/pkg/parser"
)

func TestValidate_UnknownTypeInContract(t *testing.T) {
	spec := &parser.Spec{
		Scopes: []*parser.Scope{
			{
				Name: "test",
				Use:  "http",
				Contract: &parser.Contract{
					Input: []*parser.Field{
						{Name: "item", Type: parser.TypeExpr{Name: "Widget"}},
					},
				},
			},
		},
	}

	errs := Validate(spec)
	if len(errs) == 0 {
		t.Fatal("expected validation error for unknown type Widget")
	}
	found := false
	for _, e := range errs {
		if contains(e.Error(), "Widget") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected error mentioning Widget, got: %v", errs)
	}
}

func TestValidate_KnownModelPasses(t *testing.T) {
	spec := &parser.Spec{
		Models: []*parser.Model{
			{Name: "Widget", Fields: []*parser.Field{
				{Name: "name", Type: parser.TypeExpr{Name: "string"}},
			}},
		},
		Scopes: []*parser.Scope{
			{
				Name: "test",
				Use:  "http",
				Contract: &parser.Contract{
					Input: []*parser.Field{
						{Name: "item", Type: parser.TypeExpr{Name: "Widget"}},
					},
				},
			},
		},
	}

	errs := Validate(spec)
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got: %v", errs)
	}
}

func TestValidate_UnknownArrayElementType(t *testing.T) {
	spec := &parser.Spec{
		Scopes: []*parser.Scope{
			{
				Name: "test",
				Use:  "http",
				Contract: &parser.Contract{
					Input: []*parser.Field{
						{Name: "items", Type: parser.TypeExpr{
							Name:     "array",
							ElemType: &parser.TypeExpr{Name: "Widget"},
						}},
					},
				},
			},
		},
	}

	errs := Validate(spec)
	if len(errs) == 0 {
		t.Fatal("expected validation error for unknown array element type Widget")
	}
}

func TestValidate_GivenLiteralTypeMismatch(t *testing.T) {
	spec := &parser.Spec{
		Scopes: []*parser.Scope{
			{
				Name: "test",
				Use:  "http",
				Contract: &parser.Contract{
					Input: []*parser.Field{
						{Name: "count", Type: parser.TypeExpr{Name: "int"}},
					},
				},
				Scenarios: []*parser.Scenario{
					{
						Name: "smoke",
						Given: &parser.Block{
							Steps: []parser.GivenStep{
								&parser.Assignment{Path: "count", Value: parser.LiteralString{Value: "not_an_int"}},
							},
						},
					},
				},
			},
		},
	}

	errs := Validate(spec)
	if len(errs) == 0 {
		t.Fatal("expected validation error for type mismatch")
	}
}

func TestValidate_GivenLiteralCorrectType(t *testing.T) {
	spec := &parser.Spec{
		Scopes: []*parser.Scope{
			{
				Name: "test",
				Use:  "http",
				Contract: &parser.Contract{
					Input: []*parser.Field{
						{Name: "count", Type: parser.TypeExpr{Name: "int"}},
						{Name: "name", Type: parser.TypeExpr{Name: "string"}},
						{Name: "active", Type: parser.TypeExpr{Name: "bool"}},
					},
				},
				Scenarios: []*parser.Scenario{
					{
						Name: "smoke",
						Given: &parser.Block{
							Steps: []parser.GivenStep{
								&parser.Assignment{Path: "count", Value: parser.LiteralInt{Value: 42}},
								&parser.Assignment{Path: "name", Value: parser.LiteralString{Value: "foo"}},
								&parser.Assignment{Path: "active", Value: parser.LiteralBool{Value: true}},
							},
						},
					},
				},
			},
		},
	}

	errs := Validate(spec)
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got: %v", errs)
	}
}

func TestValidate_NullOnlyForOptional(t *testing.T) {
	spec := &parser.Spec{
		Scopes: []*parser.Scope{
			{
				Name: "test",
				Use:  "http",
				Contract: &parser.Contract{
					Input: []*parser.Field{
						{Name: "required_field", Type: parser.TypeExpr{Name: "string"}},
						{Name: "optional_field", Type: parser.TypeExpr{Name: "string", Optional: true}},
					},
				},
				Scenarios: []*parser.Scenario{
					{
						Name: "smoke",
						Given: &parser.Block{
							Steps: []parser.GivenStep{
								&parser.Assignment{Path: "required_field", Value: parser.LiteralNull{}},
								&parser.Assignment{Path: "optional_field", Value: parser.LiteralNull{}},
							},
						},
					},
				},
			},
		},
	}

	errs := Validate(spec)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error (null on required), got %d: %v", len(errs), errs)
	}
}

func TestValidate_ArrayElementTypeMismatch(t *testing.T) {
	spec := &parser.Spec{
		Scopes: []*parser.Scope{
			{
				Name: "test",
				Use:  "http",
				Contract: &parser.Contract{
					Input: []*parser.Field{
						{Name: "tags", Type: parser.TypeExpr{
							Name:     "array",
							ElemType: &parser.TypeExpr{Name: "int"},
						}},
					},
				},
				Scenarios: []*parser.Scenario{
					{
						Name: "smoke",
						Given: &parser.Block{
							Steps: []parser.GivenStep{
								&parser.Assignment{
									Path: "tags",
									Value: parser.ArrayLiteral{
										Elements: []parser.Expr{
											parser.LiteralInt{Value: 1},
											parser.LiteralString{Value: "oops"},
											parser.LiteralInt{Value: 3},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	errs := Validate(spec)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error for string in []int, got %d: %v", len(errs), errs)
	}
}

func TestValidate_ArrayOfObjectsFieldCheck(t *testing.T) {
	spec := &parser.Spec{
		Models: []*parser.Model{
			{Name: "Item", Fields: []*parser.Field{
				{Name: "name", Type: parser.TypeExpr{Name: "string"}},
				{Name: "price", Type: parser.TypeExpr{Name: "int"}},
			}},
		},
		Scopes: []*parser.Scope{
			{
				Name: "test",
				Use:  "http",
				Contract: &parser.Contract{
					Input: []*parser.Field{
						{Name: "items", Type: parser.TypeExpr{
							Name:     "array",
							ElemType: &parser.TypeExpr{Name: "Item"},
						}},
					},
				},
				Scenarios: []*parser.Scenario{
					{
						Name: "smoke",
						Given: &parser.Block{
							Steps: []parser.GivenStep{
								&parser.Assignment{
									Path: "items",
									Value: parser.ArrayLiteral{
										Elements: []parser.Expr{
											parser.ObjectLiteral{Fields: []*parser.ObjField{
												{Key: "name", Value: parser.LiteralString{Value: "widget"}},
												{Key: "price", Value: parser.LiteralInt{Value: 100}},
											}},
											parser.ObjectLiteral{Fields: []*parser.ObjField{
												{Key: "name", Value: parser.LiteralString{Value: "gadget"}},
												{Key: "colour", Value: parser.LiteralString{Value: "red"}},
											}},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	errs := Validate(spec)
	if len(errs) == 0 {
		t.Fatal("expected errors for unknown field 'colour'")
	}
	foundColour := false
	for _, e := range errs {
		if contains(e.Error(), "colour") {
			foundColour = true
		}
	}
	if !foundColour {
		t.Errorf("expected error about unknown field 'colour', got: %v", errs)
	}
}

func TestValidate_NestedArrays(t *testing.T) {
	spec := &parser.Spec{
		Scopes: []*parser.Scope{
			{
				Name: "test",
				Use:  "http",
				Contract: &parser.Contract{
					Input: []*parser.Field{
						{Name: "matrix", Type: parser.TypeExpr{
							Name: "array",
							ElemType: &parser.TypeExpr{
								Name:     "array",
								ElemType: &parser.TypeExpr{Name: "int"},
							},
						}},
					},
				},
				Scenarios: []*parser.Scenario{
					{
						Name: "smoke",
						Given: &parser.Block{
							Steps: []parser.GivenStep{
								&parser.Assignment{
									Path: "matrix",
									Value: parser.ArrayLiteral{
										Elements: []parser.Expr{
											parser.ArrayLiteral{Elements: []parser.Expr{
												parser.LiteralInt{Value: 1},
											}},
											parser.ArrayLiteral{Elements: []parser.Expr{
												parser.LiteralString{Value: "bad"},
											}},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	errs := Validate(spec)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error for string in nested [][]int, got %d: %v", len(errs), errs)
	}
}

func TestValidate_ObjectLiteralUnknownField(t *testing.T) {
	spec := &parser.Spec{
		Models: []*parser.Model{
			{Name: "Account", Fields: []*parser.Field{
				{Name: "id", Type: parser.TypeExpr{Name: "string"}},
				{Name: "balance", Type: parser.TypeExpr{Name: "int"}},
			}},
		},
		Scopes: []*parser.Scope{
			{
				Name: "test",
				Use:  "http",
				Contract: &parser.Contract{
					Input: []*parser.Field{
						{Name: "from", Type: parser.TypeExpr{Name: "Account"}},
					},
				},
				Scenarios: []*parser.Scenario{
					{
						Name: "smoke",
						Given: &parser.Block{
							Steps: []parser.GivenStep{
								&parser.Assignment{
									Path: "from",
									Value: parser.ObjectLiteral{Fields: []*parser.ObjField{
										{Key: "id", Value: parser.LiteralString{Value: "alice"}},
										{Key: "balance", Value: parser.LiteralInt{Value: 100}},
										{Key: "email", Value: parser.LiteralString{Value: "alice@test.com"}},
									}},
								},
							},
						},
					},
				},
			},
		},
	}

	errs := Validate(spec)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error for unknown field email, got %d: %v", len(errs), errs)
	}
	if !contains(errs[0].Error(), "email") {
		t.Errorf("expected error about 'email', got: %v", errs[0])
	}
}

func TestValidate_ObjectLiteralFieldTypeMismatch(t *testing.T) {
	spec := &parser.Spec{
		Models: []*parser.Model{
			{Name: "Account", Fields: []*parser.Field{
				{Name: "id", Type: parser.TypeExpr{Name: "string"}},
				{Name: "balance", Type: parser.TypeExpr{Name: "int"}},
			}},
		},
		Scopes: []*parser.Scope{
			{
				Name: "test",
				Use:  "http",
				Contract: &parser.Contract{
					Input: []*parser.Field{
						{Name: "from", Type: parser.TypeExpr{Name: "Account"}},
					},
				},
				Scenarios: []*parser.Scenario{
					{
						Name: "smoke",
						Given: &parser.Block{
							Steps: []parser.GivenStep{
								&parser.Assignment{
									Path: "from",
									Value: parser.ObjectLiteral{Fields: []*parser.ObjField{
										{Key: "id", Value: parser.LiteralString{Value: "alice"}},
										{Key: "balance", Value: parser.LiteralString{Value: "not_an_int"}},
									}},
								},
							},
						},
					},
				},
			},
		},
	}

	errs := Validate(spec)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error for balance type mismatch, got %d: %v", len(errs), errs)
	}
}

func TestValidate_ObjectLiteralValidPasses(t *testing.T) {
	spec := &parser.Spec{
		Models: []*parser.Model{
			{Name: "Account", Fields: []*parser.Field{
				{Name: "id", Type: parser.TypeExpr{Name: "string"}},
				{Name: "balance", Type: parser.TypeExpr{Name: "int"}},
			}},
		},
		Scopes: []*parser.Scope{
			{
				Name: "test",
				Use:  "http",
				Contract: &parser.Contract{
					Input: []*parser.Field{
						{Name: "from", Type: parser.TypeExpr{Name: "Account"}},
					},
				},
				Scenarios: []*parser.Scenario{
					{
						Name: "smoke",
						Given: &parser.Block{
							Steps: []parser.GivenStep{
								&parser.Assignment{
									Path: "from",
									Value: parser.ObjectLiteral{Fields: []*parser.ObjField{
										{Key: "id", Value: parser.LiteralString{Value: "alice"}},
										{Key: "balance", Value: parser.LiteralInt{Value: 100}},
									}},
								},
							},
						},
					},
				},
			},
		},
	}

	errs := Validate(spec)
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got: %v", errs)
	}
}

func TestValidate_GivenMissingRequiredField(t *testing.T) {
	spec := &parser.Spec{
		Scopes: []*parser.Scope{
			{
				Name: "test",
				Use:  "http",
				Contract: &parser.Contract{
					Input: []*parser.Field{
						{Name: "from", Type: parser.TypeExpr{Name: "string"}},
						{Name: "to", Type: parser.TypeExpr{Name: "string"}},
						{Name: "note", Type: parser.TypeExpr{Name: "string", Optional: true}},
					},
				},
				Scenarios: []*parser.Scenario{
					{
						Name: "smoke",
						Given: &parser.Block{
							Steps: []parser.GivenStep{
								&parser.Assignment{Path: "from", Value: parser.LiteralString{Value: "alice"}},
								// "to" is missing and required
								// "note" is missing but optional — should not error
							},
						},
					},
				},
			},
		},
	}

	errs := Validate(spec)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error for missing 'to', got %d: %v", len(errs), errs)
	}
	if !contains(errs[0].Error(), "to") {
		t.Errorf("expected error about 'to', got: %v", errs[0])
	}
}

func TestValidate_GivenWithCallsSkipsCompleteness(t *testing.T) {
	spec := &parser.Spec{
		Scopes: []*parser.Scope{
			{
				Name: "test",
				Use:  "playwright",
				Contract: &parser.Contract{
					Input: []*parser.Field{
						{Name: "username", Type: parser.TypeExpr{Name: "string"}},
					},
				},
				Scenarios: []*parser.Scenario{
					{
						Name: "ui_flow",
						Given: &parser.Block{
							Steps: []parser.GivenStep{
								&parser.Call{Namespace: "playwright", Method: "fill"},
							},
						},
					},
				},
			},
		},
	}

	errs := Validate(spec)
	if len(errs) != 0 {
		t.Fatalf("expected no errors (given with calls skips completeness), got: %v", errs)
	}
}

func TestValidate_WhenScenarioSkipsCompleteness(t *testing.T) {
	spec := &parser.Spec{
		Scopes: []*parser.Scope{
			{
				Name: "test",
				Use:  "http",
				Contract: &parser.Contract{
					Input: []*parser.Field{
						{Name: "amount", Type: parser.TypeExpr{Name: "int"}},
					},
				},
				Scenarios: []*parser.Scenario{
					{
						Name: "generative",
						When: &parser.Block{
							Predicates: []parser.Expr{
								parser.BinaryOp{Left: parser.FieldRef{Path: "amount"}, Op: ">", Right: parser.LiteralInt{Value: 0}},
							},
						},
					},
				},
			},
		},
	}

	errs := Validate(spec)
	if len(errs) != 0 {
		t.Fatalf("expected no errors (when scenario skips completeness), got: %v", errs)
	}
}

func TestValidate_ThenUnknownField(t *testing.T) {
	spec := &parser.Spec{
		Scopes: []*parser.Scope{
			{
				Name: "test",
				Use:  "http",
				Contract: &parser.Contract{
					Input: []*parser.Field{
						{Name: "x", Type: parser.TypeExpr{Name: "int"}},
					},
					Output: []*parser.Field{
						{Name: "result", Type: parser.TypeExpr{Name: "int"}},
						{Name: "error", Type: parser.TypeExpr{Name: "string", Optional: true}},
					},
				},
				Scenarios: []*parser.Scenario{
					{
						Name: "smoke",
						Given: &parser.Block{
							Steps: []parser.GivenStep{
								&parser.Assignment{Path: "x", Value: parser.LiteralInt{Value: 1}},
							},
						},
						Then: &parser.Block{
							Assertions: []*parser.Assertion{
								{Target: "result", Expected: parser.LiteralInt{Value: 42}},
								{Target: "typo_field", Expected: parser.LiteralInt{Value: 0}},
							},
						},
					},
				},
			},
		},
	}

	errs := Validate(spec)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error for unknown then target, got %d: %v", len(errs), errs)
	}
	if !contains(errs[0].Error(), "typo_field") {
		t.Errorf("expected error about 'typo_field', got: %v", errs[0])
	}
}

func TestValidate_ThenDotPathValid(t *testing.T) {
	spec := &parser.Spec{
		Models: []*parser.Model{
			{Name: "Account", Fields: []*parser.Field{
				{Name: "id", Type: parser.TypeExpr{Name: "string"}},
				{Name: "balance", Type: parser.TypeExpr{Name: "int"}},
			}},
		},
		Scopes: []*parser.Scope{
			{
				Name: "test",
				Use:  "http",
				Contract: &parser.Contract{
					Input: []*parser.Field{
						{Name: "from", Type: parser.TypeExpr{Name: "Account"}},
					},
					Output: []*parser.Field{
						{Name: "from", Type: parser.TypeExpr{Name: "Account"}},
						{Name: "error", Type: parser.TypeExpr{Name: "string", Optional: true}},
					},
				},
				Scenarios: []*parser.Scenario{
					{
						Name: "smoke",
						Given: &parser.Block{
							Steps: []parser.GivenStep{
								&parser.Assignment{
									Path: "from",
									Value: parser.ObjectLiteral{Fields: []*parser.ObjField{
										{Key: "id", Value: parser.LiteralString{Value: "alice"}},
										{Key: "balance", Value: parser.LiteralInt{Value: 100}},
									}},
								},
							},
						},
						Then: &parser.Block{
							Assertions: []*parser.Assertion{
								{Target: "from.balance", Expected: parser.LiteralInt{Value: 70}},
								{Target: "error", Expected: parser.LiteralNull{}},
							},
						},
					},
				},
			},
		},
	}

	errs := Validate(spec)
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got: %v", errs)
	}
}

func TestValidate_ThenPluginAssertionSkipped(t *testing.T) {
	spec := &parser.Spec{
		Scopes: []*parser.Scope{
			{
				Name: "test",
				Use:  "playwright",
				Contract: &parser.Contract{
					Input: []*parser.Field{
						{Name: "x", Type: parser.TypeExpr{Name: "int"}},
					},
				},
				Scenarios: []*parser.Scenario{
					{
						Name: "ui",
						Given: &parser.Block{
							Steps: []parser.GivenStep{
								&parser.Assignment{Path: "x", Value: parser.LiteralInt{Value: 1}},
							},
						},
						Then: &parser.Block{
							Assertions: []*parser.Assertion{
								{Target: "welcome", Plugin: "playwright", Property: "visible", Expected: parser.LiteralBool{Value: true}},
							},
						},
					},
				},
			},
		},
	}

	errs := Validate(spec)
	if len(errs) != 0 {
		t.Fatalf("expected no errors (plugin assertions skipped), got: %v", errs)
	}
}

func TestValidate_MultipleErrors(t *testing.T) {
	spec := &parser.Spec{
		Scopes: []*parser.Scope{
			{
				Name: "test",
				Use:  "http",
				Contract: &parser.Contract{
					Input: []*parser.Field{
						{Name: "count", Type: parser.TypeExpr{Name: "int"}},
						{Name: "name", Type: parser.TypeExpr{Name: "string"}},
					},
					Output: []*parser.Field{
						{Name: "result", Type: parser.TypeExpr{Name: "int"}},
					},
				},
				Scenarios: []*parser.Scenario{
					{
						Name: "bad",
						Given: &parser.Block{
							Steps: []parser.GivenStep{
								&parser.Assignment{Path: "count", Value: parser.LiteralString{Value: "wrong"}},
								// name is missing
							},
						},
						Then: &parser.Block{
							Assertions: []*parser.Assertion{
								{Target: "typo", Expected: parser.LiteralInt{Value: 0}},
							},
						},
					},
				},
			},
		},
	}

	errs := Validate(spec)
	if len(errs) < 3 {
		t.Fatalf("expected at least 3 errors (type mismatch + missing field + bad then target), got %d: %v", len(errs), errs)
	}
}

func TestFormatErrors(t *testing.T) {
	errs := []error{
		fmt.Errorf("scope %q, contract input %q: unknown type %q", "orders", "items", "Itme"),
		fmt.Errorf("scope %q, scenario %q, field %q: expected int, got string literal", "orders", "smoke", "count"),
		fmt.Errorf("scope %q, scenario %q: missing required field %q", "orders", "smoke", "name"),
		fmt.Errorf("scope %q, scenario %q: then target %q does not match any output field", "transfer", "basic", "balnce"),
	}

	output := FormatErrors(errs)
	if output == "" {
		t.Fatal("expected non-empty output")
	}
	if !contains(output, "orders") || !contains(output, "transfer") {
		t.Error("expected hierarchical output grouped by scope")
	}
	if !contains(output, "validation errors:") {
		t.Error("expected header line")
	}
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
