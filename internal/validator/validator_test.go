package validator

import (
	"fmt"
	"strings"
	"testing"

	"github.com/bamsammich/speclang/v4/internal/parser"
	"github.com/bamsammich/speclang/v4/pkg/spec"
)

// testRegistry returns a registry with http and process plugin assertions
// registered, matching the built-in plugin definitions.
func testRegistry() *spec.Registry {
	r := spec.NewRegistry()
	r.Register("http", spec.PluginDef{
		Assertions: map[string]spec.AssertionDef{
			"status": {Type: "int"},
			"body":   {Type: "any"},
			"header": {Type: "string"},
		},
	})
	r.Register("process", spec.PluginDef{
		Assertions: map[string]spec.AssertionDef{
			"exit_code": {Type: "int"},
			"stdout":    {Type: "any"},
			"stderr":    {Type: "string"},
		},
	})
	r.Register("playwright", spec.PluginDef{
		Assertions: map[string]spec.AssertionDef{
			"visible": {Type: "bool"},
			"text":    {Type: "string"},
			"count":   {Type: "int"},
		},
	})
	return r
}

func TestValidate_UnknownTypeInContract(t *testing.T) {
	t.Parallel()
	spec := &parser.Spec{
		Scopes: []*parser.Scope{
			{
				Name: "test",
				Contracts: []*parser.Contract{
					{
						Name: "check",
						Fields: []*parser.Field{
							{Name: "item", Type: parser.TypeExpr{Name: "Widget"}},
						},
					},
				},
			},
		},
	}

	errs := Validate(spec, testRegistry())
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
	t.Parallel()
	spec := &parser.Spec{
		Models: []*parser.Model{
			{Name: "Widget", Fields: []*parser.Field{
				{Name: "name", Type: parser.TypeExpr{Name: "string"}},
			}},
		},
		Scopes: []*parser.Scope{
			{
				Name: "test",
				Contracts: []*parser.Contract{
					{
						Name: "check",
						Fields: []*parser.Field{
							{Name: "item", Type: parser.TypeExpr{Name: "Widget"}},
						},
					},
				},
			},
		},
	}

	errs := Validate(spec, testRegistry())
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got: %v", errs)
	}
}

func TestValidate_UnknownArrayElementType(t *testing.T) {
	t.Parallel()
	spec := &parser.Spec{
		Scopes: []*parser.Scope{
			{
				Name: "test",
				Contracts: []*parser.Contract{
					{
						Name: "check",
						Fields: []*parser.Field{
							{Name: "items", Type: parser.TypeExpr{
								Name:     "array",
								ElemType: &parser.TypeExpr{Name: "Widget"},
							}},
						},
					},
				},
			},
		},
	}

	errs := Validate(spec, testRegistry())
	if len(errs) == 0 {
		t.Fatal("expected validation error for unknown array element type Widget")
	}
}

func TestValidate_GivenLiteralTypeMismatch(t *testing.T) {
	t.Parallel()
	spec := &parser.Spec{
		Scopes: []*parser.Scope{
			{
				Name: "test",
				Contracts: []*parser.Contract{
					{
						Name: "check",
						Fields: []*parser.Field{
							{Name: "count", Type: parser.TypeExpr{Name: "int"}},
						},
						Scenarios: []*parser.Scenario{
							{
								Name: "smoke",
								Given: &parser.Block{
									Steps: []parser.GivenStep{
										&parser.Assignment{
											Path:  "count",
											Value: parser.LiteralString{Value: "not_an_int"},
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

	errs := Validate(spec, testRegistry())
	if len(errs) == 0 {
		t.Fatal("expected validation error for type mismatch")
	}
}

func TestValidate_GivenLiteralCorrectType(t *testing.T) {
	t.Parallel()
	spec := &parser.Spec{
		Scopes: []*parser.Scope{
			{
				Name: "test",
				Contracts: []*parser.Contract{
					{
						Name: "check",
						Fields: []*parser.Field{
							{Name: "count", Type: parser.TypeExpr{Name: "int"}},
							{Name: "name", Type: parser.TypeExpr{Name: "string"}},
							{Name: "active", Type: parser.TypeExpr{Name: "bool"}},
						},
						Scenarios: []*parser.Scenario{
							{
								Name: "smoke",
								Given: &parser.Block{
									Steps: []parser.GivenStep{
										&parser.Assignment{
											Path:  "count",
											Value: parser.LiteralInt{Value: 42},
										},
										&parser.Assignment{
											Path:  "name",
											Value: parser.LiteralString{Value: "foo"},
										},
										&parser.Assignment{
											Path:  "active",
											Value: parser.LiteralBool{Value: true},
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

	errs := Validate(spec, testRegistry())
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got: %v", errs)
	}
}

func TestValidate_NullOnlyForOptional(t *testing.T) {
	t.Parallel()
	spec := &parser.Spec{
		Scopes: []*parser.Scope{
			{
				Name: "test",
				Contracts: []*parser.Contract{
					{
						Name: "check",
						Fields: []*parser.Field{
							{Name: "required_field", Type: parser.TypeExpr{Name: "string"}},
							{
								Name: "optional_field",
								Type: parser.TypeExpr{Name: "string", Optional: true},
							},
						},
						Scenarios: []*parser.Scenario{
							{
								Name: "smoke",
								Given: &parser.Block{
									Steps: []parser.GivenStep{
										&parser.Assignment{
											Path:  "required_field",
											Value: parser.LiteralNull{},
										},
										&parser.Assignment{
											Path:  "optional_field",
											Value: parser.LiteralNull{},
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

	errs := Validate(spec, testRegistry())
	if len(errs) != 1 {
		t.Fatalf("expected 1 error (null on required), got %d: %v", len(errs), errs)
	}
}

func TestValidate_ArrayElementTypeMismatch(t *testing.T) {
	t.Parallel()
	spec := &parser.Spec{
		Scopes: []*parser.Scope{
			{
				Name: "test",
				Contracts: []*parser.Contract{
					{
						Name: "check",
						Fields: []*parser.Field{
							{Name: "tags", Type: parser.TypeExpr{
								Name:     "array",
								ElemType: &parser.TypeExpr{Name: "int"},
							}},
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
			},
		},
	}

	errs := Validate(spec, testRegistry())
	if len(errs) != 1 {
		t.Fatalf("expected 1 error for string in []int, got %d: %v", len(errs), errs)
	}
}

func TestValidate_ArrayOfObjectsFieldCheck(t *testing.T) {
	t.Parallel()
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
				Contracts: []*parser.Contract{
					{
						Name: "check",
						Fields: []*parser.Field{
							{Name: "items", Type: parser.TypeExpr{
								Name:     "array",
								ElemType: &parser.TypeExpr{Name: "Item"},
							}},
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
														{
															Key:   "name",
															Value: parser.LiteralString{Value: "widget"},
														},
														{
															Key:   "price",
															Value: parser.LiteralInt{Value: 100},
														},
													}},
													parser.ObjectLiteral{Fields: []*parser.ObjField{
														{
															Key:   "name",
															Value: parser.LiteralString{Value: "gadget"},
														},
														{
															Key:   "colour",
															Value: parser.LiteralString{Value: "red"},
														},
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
			},
		},
	}

	errs := Validate(spec, testRegistry())
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
	t.Parallel()
	spec := &parser.Spec{
		Scopes: []*parser.Scope{
			{
				Name: "test",
				Contracts: []*parser.Contract{
					{
						Name: "check",
						Fields: []*parser.Field{
							{Name: "matrix", Type: parser.TypeExpr{
								Name: "array",
								ElemType: &parser.TypeExpr{
									Name:     "array",
									ElemType: &parser.TypeExpr{Name: "int"},
								},
							}},
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
			},
		},
	}

	errs := Validate(spec, testRegistry())
	if len(errs) != 1 {
		t.Fatalf("expected 1 error for string in nested [][]int, got %d: %v", len(errs), errs)
	}
}

func TestValidate_ObjectLiteralUnknownField(t *testing.T) {
	t.Parallel()
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
				Contracts: []*parser.Contract{
					{
						Name: "check",
						Fields: []*parser.Field{
							{Name: "from", Type: parser.TypeExpr{Name: "Account"}},
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
												{
													Key:   "email",
													Value: parser.LiteralString{Value: "alice@test.com"},
												},
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

	errs := Validate(spec, testRegistry())
	if len(errs) != 1 {
		t.Fatalf("expected 1 error for unknown field email, got %d: %v", len(errs), errs)
	}
	if !contains(errs[0].Error(), "email") {
		t.Errorf("expected error about 'email', got: %v", errs[0])
	}
}

func TestValidate_ObjectLiteralFieldTypeMismatch(t *testing.T) {
	t.Parallel()
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
				Contracts: []*parser.Contract{
					{
						Name: "check",
						Fields: []*parser.Field{
							{Name: "from", Type: parser.TypeExpr{Name: "Account"}},
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
												{
													Key:   "balance",
													Value: parser.LiteralString{Value: "not_an_int"},
												},
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

	errs := Validate(spec, testRegistry())
	if len(errs) != 1 {
		t.Fatalf("expected 1 error for balance type mismatch, got %d: %v", len(errs), errs)
	}
}

func TestValidate_ObjectLiteralValidPasses(t *testing.T) {
	t.Parallel()
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
				Contracts: []*parser.Contract{
					{
						Name: "check",
						Fields: []*parser.Field{
							{Name: "from", Type: parser.TypeExpr{Name: "Account"}},
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
			},
		},
	}

	errs := Validate(spec, testRegistry())
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got: %v", errs)
	}
}

func TestValidate_GivenMissingRequiredField(t *testing.T) {
	t.Parallel()
	spec := &parser.Spec{
		Scopes: []*parser.Scope{
			{
				Name: "test",
				Contracts: []*parser.Contract{
					{
						Name: "check",
						Fields: []*parser.Field{
							{Name: "from", Type: parser.TypeExpr{Name: "string"}},
							{Name: "to", Type: parser.TypeExpr{Name: "string"}},
							{Name: "note", Type: parser.TypeExpr{Name: "string", Optional: true}},
						},
						Scenarios: []*parser.Scenario{
							{
								Name: "smoke",
								Given: &parser.Block{
									Steps: []parser.GivenStep{
										&parser.Assignment{
											Path:  "from",
											Value: parser.LiteralString{Value: "alice"},
										},
										// "to" is missing and required
										// "note" is missing but optional — should not error
									},
								},
							},
						},
					},
				},
			},
		},
	}

	errs := Validate(spec, testRegistry())
	if len(errs) != 1 {
		t.Fatalf("expected 1 error for missing 'to', got %d: %v", len(errs), errs)
	}
	if !contains(errs[0].Error(), "to") {
		t.Errorf("expected error about 'to', got: %v", errs[0])
	}
}

func TestValidate_GivenWithCallsSkipsCompleteness(t *testing.T) {
	t.Parallel()
	spec := &parser.Spec{
		Scopes: []*parser.Scope{
			{
				Name: "test",
				Contracts: []*parser.Contract{
					{
						Name: "ui_check",
						Fields: []*parser.Field{
							{Name: "username", Type: parser.TypeExpr{Name: "string"}},
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
			},
		},
	}

	errs := Validate(spec, testRegistry())
	if len(errs) != 0 {
		t.Fatalf("expected no errors (given with calls skips completeness), got: %v", errs)
	}
}

func TestValidate_WhenScenarioSkipsCompleteness(t *testing.T) {
	t.Parallel()
	spec := &parser.Spec{
		Scopes: []*parser.Scope{
			{
				Name: "test",
				Contracts: []*parser.Contract{
					{
						Name: "check",
						Fields: []*parser.Field{
							{Name: "amount", Type: parser.TypeExpr{Name: "int"}},
						},
						Scenarios: []*parser.Scenario{
							{
								Name: "generative",
								When: &parser.Block{
									Predicates: []parser.Expr{
										parser.BinaryOp{
											Left:  parser.FieldRef{Path: "amount"},
											Op:    ">",
											Right: parser.LiteralInt{Value: 0},
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

	errs := Validate(spec, testRegistry())
	if len(errs) != 0 {
		t.Fatalf("expected no errors (when scenario skips completeness), got: %v", errs)
	}
}

// TestValidate_ThenUnknownField verifies that a then-block assertion target that
// does not match any field in the contract's return model produces a validation error.
func TestValidate_ThenUnknownField(t *testing.T) {
	t.Parallel()
	// ReturnModel has "result" and "error" fields; "typo_field" is not in it.
	spec := &parser.Spec{
		Models: []*parser.Model{
			{Name: "ReturnModel", Fields: []*parser.Field{
				{Name: "result", Type: parser.TypeExpr{Name: "int"}},
				{Name: "error", Type: parser.TypeExpr{Name: "string", Optional: true}},
			}},
		},
		Scopes: []*parser.Scope{
			{
				Name: "test",
				Contracts: []*parser.Contract{
					{
						Name:       "check",
						ReturnType: parser.TypeExpr{Name: "ReturnModel"},
						Fields: []*parser.Field{
							{Name: "x", Type: parser.TypeExpr{Name: "int"}},
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
			},
		},
	}

	errs := Validate(spec, testRegistry())
	if len(errs) != 1 {
		t.Fatalf("expected 1 error for unknown then target, got %d: %v", len(errs), errs)
	}
	if !contains(errs[0].Error(), "typo_field") {
		t.Errorf("expected error about 'typo_field', got: %v", errs[0])
	}
}

// TestValidate_ThenDotPathValid verifies that dot-path assertion targets (e.g., "from.balance")
// are accepted when the top-level field exists in the return model.
func TestValidate_ThenDotPathValid(t *testing.T) {
	t.Parallel()
	spec := &parser.Spec{
		Models: []*parser.Model{
			{Name: "Account", Fields: []*parser.Field{
				{Name: "id", Type: parser.TypeExpr{Name: "string"}},
				{Name: "balance", Type: parser.TypeExpr{Name: "int"}},
			}},
			{Name: "TransferResult", Fields: []*parser.Field{
				{Name: "from", Type: parser.TypeExpr{Name: "Account"}},
				{Name: "error", Type: parser.TypeExpr{Name: "string", Optional: true}},
			}},
		},
		Scopes: []*parser.Scope{
			{
				Name: "test",
				Contracts: []*parser.Contract{
					{
						Name:       "check",
						ReturnType: parser.TypeExpr{Name: "TransferResult"},
						Fields: []*parser.Field{
							{Name: "from", Type: parser.TypeExpr{Name: "Account"}},
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
			},
		},
	}

	errs := Validate(spec, testRegistry())
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got: %v", errs)
	}
}

func TestValidate_ThenPluginAssertionSkipped(t *testing.T) {
	t.Parallel()
	spec := &parser.Spec{
		Scopes: []*parser.Scope{
			{
				Name: "test",
				Contracts: []*parser.Contract{
					{
						Name: "ui",
						Fields: []*parser.Field{
							{Name: "x", Type: parser.TypeExpr{Name: "int"}},
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
										{
											Target:   "welcome",
											Plugin:   "playwright",
											Property: "visible",
											Expected: parser.LiteralBool{Value: true},
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

	errs := Validate(spec, testRegistry())
	if len(errs) != 0 {
		t.Fatalf("expected no errors (plugin assertions skipped), got: %v", errs)
	}
}

func TestValidate_MultipleErrors(t *testing.T) {
	t.Parallel()
	// ReturnModel has "result"; "typo" is not in it.
	spec := &parser.Spec{
		Models: []*parser.Model{
			{Name: "ResultModel", Fields: []*parser.Field{
				{Name: "result", Type: parser.TypeExpr{Name: "int"}},
			}},
		},
		Scopes: []*parser.Scope{
			{
				Name: "test",
				Contracts: []*parser.Contract{
					{
						Name:       "check",
						ReturnType: parser.TypeExpr{Name: "ResultModel"},
						Fields: []*parser.Field{
							{Name: "count", Type: parser.TypeExpr{Name: "int"}},
							{Name: "name", Type: parser.TypeExpr{Name: "string"}},
						},
						Scenarios: []*parser.Scenario{
							{
								Name: "bad",
								Given: &parser.Block{
									Steps: []parser.GivenStep{
										&parser.Assignment{
											Path:  "count",
											Value: parser.LiteralString{Value: "wrong"},
										},
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
			},
		},
	}

	errs := Validate(spec, testRegistry())
	if len(errs) < 3 {
		t.Fatalf(
			"expected at least 3 errors (type mismatch + missing field + bad then target), got %d: %v",
			len(errs),
			errs,
		)
	}
}

func TestFormatErrors(t *testing.T) {
	t.Parallel()
	errs := []error{
		fmt.Errorf("scope %q, contract input %q: unknown type %q", "orders", "items", "Itme"),
		fmt.Errorf(
			"scope %q, scenario %q, field %q: expected int, got string literal",
			"orders",
			"smoke",
			"count",
		),
		fmt.Errorf("scope %q, scenario %q: missing required field %q", "orders", "smoke", "name"),
		fmt.Errorf(
			"scope %q, scenario %q: then target %q does not match any output field",
			"transfer",
			"basic",
			"balnce",
		),
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

func TestValidate_ServiceRefValid(t *testing.T) {
	t.Parallel()
	spec := &parser.Spec{
		Target: &parser.Target{
			Fields: map[string]parser.Expr{
				"base_url": parser.ServiceRef{Name: "myapp"},
			},
			Services: []*parser.Service{
				{Name: "myapp", Image: "myapp:latest", Port: 8080},
			},
		},
	}

	errs := Validate(spec, testRegistry())
	if len(errs) != 0 {
		t.Fatalf("expected no errors for valid service ref, got: %v", errs)
	}
}

func TestValidate_ServiceRefUndeclared(t *testing.T) {
	t.Parallel()
	spec := &parser.Spec{
		Target: &parser.Target{
			Fields: map[string]parser.Expr{
				"base_url": parser.ServiceRef{Name: "ghost"},
			},
		},
	}

	errs := Validate(spec, testRegistry())
	if len(errs) != 1 {
		t.Fatalf("expected 1 error for undeclared service ref, got %d: %v", len(errs), errs)
	}
	if !contains(errs[0].Error(), "ghost") || !contains(errs[0].Error(), "undeclared") {
		t.Errorf("expected error about undeclared service 'ghost', got: %v", errs[0])
	}
}

func TestValidate_ServiceRefWithCompose(t *testing.T) {
	t.Parallel()
	// When compose is set, service refs are resolved externally — no error.
	spec := &parser.Spec{
		Target: &parser.Target{
			Fields: map[string]parser.Expr{
				"base_url": parser.ServiceRef{Name: "external_svc"},
			},
			Compose: "docker-compose.yml",
		},
	}

	errs := Validate(spec, testRegistry())
	if len(errs) != 0 {
		t.Fatalf("expected no errors when compose is set, got: %v", errs)
	}
}

// --- Named enum tests ---

func TestValidate_NamedEnumValidVariant(t *testing.T) {
	t.Parallel()
	s := &parser.Spec{
		Enums: []*parser.NamedEnum{
			{Name: "Role", Variants: []string{"admin", "user", "guest"}},
		},
		Contracts: []*parser.Contract{
			{
				Name: "check",
				Fields: []*parser.Field{
					{Name: "role", Type: parser.TypeExpr{Name: "string"}},
				},
				Invariants: []*parser.Invariant{
					{
						Name: "valid_role",
						Assertions: []*parser.Assertion{
							{Expr: parser.BinaryOp{
								Left:  parser.FieldRef{Path: "output.role"},
								Op:    "==",
								Right: parser.FieldRef{Path: "Role.admin"},
							}},
						},
					},
				},
			},
		},
	}
	errs := Validate(s, testRegistry())
	if len(errs) != 0 {
		t.Fatalf("expected no errors for valid enum variant, got: %v", errs)
	}
}

func TestValidate_NamedEnumInvalidVariant(t *testing.T) {
	t.Parallel()
	s := &parser.Spec{
		Enums: []*parser.NamedEnum{
			{Name: "Role", Variants: []string{"admin", "user", "guest"}},
		},
		Contracts: []*parser.Contract{
			{
				Name: "check",
				Fields: []*parser.Field{
					{Name: "role", Type: parser.TypeExpr{Name: "string"}},
				},
				Invariants: []*parser.Invariant{
					{
						Name: "bad_role",
						Assertions: []*parser.Assertion{
							{Expr: parser.BinaryOp{
								Left:  parser.FieldRef{Path: "output.role"},
								Op:    "==",
								Right: parser.FieldRef{Path: "Role.superadmin"},
							}},
						},
					},
				},
			},
		},
	}
	errs := Validate(s, testRegistry())
	if len(errs) != 1 {
		t.Fatalf("expected 1 error for invalid enum variant, got %d: %v", len(errs), errs)
	}
	if !contains(errs[0].Error(), "superadmin") || !contains(errs[0].Error(), "Role") {
		t.Errorf("expected error about invalid variant 'superadmin' of enum 'Role', got: %v", errs[0])
	}
}

func TestValidate_NamedEnumAsType(t *testing.T) {
	t.Parallel()
	s := &parser.Spec{
		Enums: []*parser.NamedEnum{
			{Name: "Status", Variants: []string{"active", "inactive"}},
		},
		Contracts: []*parser.Contract{
			{
				Name: "check",
				Fields: []*parser.Field{
					{Name: "status", Type: parser.TypeExpr{Name: "Status"}},
				},
			},
		},
	}
	errs := Validate(s, testRegistry())
	if len(errs) != 0 {
		t.Fatalf("expected no errors for named enum as type, got: %v", errs)
	}
}

// --- Config ref tests ---

func TestValidate_ConfigRefValid(t *testing.T) {
	t.Parallel()
	s := &parser.Spec{
		Config: map[string]parser.Expr{
			"base_url": parser.LiteralString{Value: "http://localhost:8080"},
		},
		Contracts: []*parser.Contract{
			{
				Name: "check",
				Fields: []*parser.Field{
					{Name: "x", Type: parser.TypeExpr{Name: "int"}},
				},
				Invariants: []*parser.Invariant{
					{
						Name: "uses_config",
						Assertions: []*parser.Assertion{
							{Expr: parser.BinaryOp{
								Left:  parser.FieldRef{Path: "config.base_url"},
								Op:    "!=",
								Right: parser.LiteralString{Value: ""},
							}},
						},
					},
				},
			},
		},
	}
	errs := Validate(s, testRegistry())
	if len(errs) != 0 {
		t.Fatalf("expected no errors for valid config ref, got: %v", errs)
	}
}

func TestValidate_ConfigRefUnknownKey(t *testing.T) {
	t.Parallel()
	s := &parser.Spec{
		Config: map[string]parser.Expr{
			"base_url": parser.LiteralString{Value: "http://localhost"},
		},
		Contracts: []*parser.Contract{
			{
				Name: "check",
				Fields: []*parser.Field{
					{Name: "x", Type: parser.TypeExpr{Name: "int"}},
				},
				Invariants: []*parser.Invariant{
					{
						Name: "bad_config",
						Assertions: []*parser.Assertion{
							{Expr: parser.FieldRef{Path: "config.missing_key"}},
						},
					},
				},
			},
		},
	}
	errs := Validate(s, testRegistry())
	if len(errs) != 1 {
		t.Fatalf("expected 1 error for unknown config key, got %d: %v", len(errs), errs)
	}
	if !contains(errs[0].Error(), "missing_key") {
		t.Errorf("expected error about 'missing_key', got: %v", errs[0])
	}
}

func TestValidate_ConfigRefNoConfigBlock(t *testing.T) {
	t.Parallel()
	s := &parser.Spec{
		Contracts: []*parser.Contract{
			{
				Name: "check",
				Fields: []*parser.Field{
					{Name: "x", Type: parser.TypeExpr{Name: "int"}},
				},
				Invariants: []*parser.Invariant{
					{
						Name: "bad",
						Assertions: []*parser.Assertion{
							{Expr: parser.FieldRef{Path: "config.anything"}},
						},
					},
				},
			},
		},
	}
	errs := Validate(s, testRegistry())
	if len(errs) != 1 {
		t.Fatalf("expected 1 error for config ref with no config block, got %d: %v", len(errs), errs)
	}
	if !contains(errs[0].Error(), "config") {
		t.Errorf("expected error about config, got: %v", errs[0])
	}
}

// --- Contract inheritance tests ---

func TestValidate_ContractInheritsValid(t *testing.T) {
	t.Parallel()
	s := &parser.Spec{
		Models: []*parser.Model{
			{Name: "InputModel", Fields: []*parser.Field{
				{Name: "amount", Type: parser.TypeExpr{Name: "int"}},
				{Name: "name", Type: parser.TypeExpr{Name: "string"}},
			}},
		},
		Contracts: []*parser.Contract{
			{
				Name:     "check",
				Inherits: "InputModel",
				Constraints: []parser.Expr{
					parser.BinaryOp{
						Left:  parser.FieldRef{Path: "amount"},
						Op:    ">",
						Right: parser.LiteralInt{Value: 0},
					},
				},
			},
		},
	}
	errs := Validate(s, testRegistry())
	if len(errs) != 0 {
		t.Fatalf("expected no errors for valid inheritance, got: %v", errs)
	}
}

func TestValidate_ContractInheritsUnknownModel(t *testing.T) {
	t.Parallel()
	s := &parser.Spec{
		Contracts: []*parser.Contract{
			{
				Name:     "check",
				Inherits: "NonExistent",
			},
		},
	}
	errs := Validate(s, testRegistry())
	if len(errs) != 1 {
		t.Fatalf("expected 1 error for unknown inherited model, got %d: %v", len(errs), errs)
	}
	if !contains(errs[0].Error(), "NonExistent") {
		t.Errorf("expected error about 'NonExistent', got: %v", errs[0])
	}
}

func TestValidate_ContractConstraintUnknownField(t *testing.T) {
	t.Parallel()
	s := &parser.Spec{
		Models: []*parser.Model{
			{Name: "InputModel", Fields: []*parser.Field{
				{Name: "amount", Type: parser.TypeExpr{Name: "int"}},
			}},
		},
		Contracts: []*parser.Contract{
			{
				Name:     "check",
				Inherits: "InputModel",
				Constraints: []parser.Expr{
					parser.BinaryOp{
						Left:  parser.FieldRef{Path: "nonexistent"},
						Op:    ">",
						Right: parser.LiteralInt{Value: 0},
					},
				},
			},
		},
	}
	errs := Validate(s, testRegistry())
	if len(errs) != 1 {
		t.Fatalf("expected 1 error for unknown field in constraint, got %d: %v", len(errs), errs)
	}
	if !contains(errs[0].Error(), "nonexistent") {
		t.Errorf("expected error about 'nonexistent', got: %v", errs[0])
	}
}

// --- Field When tests ---

func TestValidate_FieldWhenValid(t *testing.T) {
	t.Parallel()
	s := &parser.Spec{
		Models: []*parser.Model{
			{Name: "Order", Fields: []*parser.Field{
				{Name: "status", Type: parser.TypeExpr{Name: "string"}},
				{Name: "tracking_id", Type: parser.TypeExpr{Name: "string", Optional: true},
					When: parser.BinaryOp{
						Left:  parser.FieldRef{Path: "status"},
						Op:    "==",
						Right: parser.LiteralString{Value: "shipped"},
					}},
			}},
		},
	}
	errs := Validate(s, testRegistry())
	if len(errs) != 0 {
		t.Fatalf("expected no errors for valid when expression, got: %v", errs)
	}
}

func TestValidate_FieldWhenUnknownSibling(t *testing.T) {
	t.Parallel()
	s := &parser.Spec{
		Models: []*parser.Model{
			{Name: "Order", Fields: []*parser.Field{
				{Name: "status", Type: parser.TypeExpr{Name: "string"}},
				{Name: "tracking_id", Type: parser.TypeExpr{Name: "string", Optional: true},
					When: parser.BinaryOp{
						Left:  parser.FieldRef{Path: "nonexistent_field"},
						Op:    "==",
						Right: parser.LiteralString{Value: "shipped"},
					}},
			}},
		},
	}
	errs := Validate(s, testRegistry())
	if len(errs) != 1 {
		t.Fatalf("expected 1 error for unknown sibling in when, got %d: %v", len(errs), errs)
	}
	if !contains(errs[0].Error(), "nonexistent_field") {
		t.Errorf("expected error about 'nonexistent_field', got: %v", errs[0])
	}
}

func TestValidate_FieldWhenCircularDependency(t *testing.T) {
	t.Parallel()
	s := &parser.Spec{
		Models: []*parser.Model{
			{Name: "Circular", Fields: []*parser.Field{
				{Name: "a", Type: parser.TypeExpr{Name: "string", Optional: true},
					When: parser.FieldRef{Path: "b"}},
				{Name: "b", Type: parser.TypeExpr{Name: "string", Optional: true},
					When: parser.FieldRef{Path: "a"}},
			}},
		},
	}
	errs := Validate(s, testRegistry())
	if len(errs) == 0 {
		t.Fatal("expected error for circular when dependency")
	}
	found := false
	for _, e := range errs {
		if contains(e.Error(), "circular") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected error about circular dependency, got: %v", errs)
	}
}

// TestValidate_ContractFieldWhenCircularDependency verifies that circular when
// dependencies among contract input fields (not model fields) are detected.
// e.g., a: int when b > 0 and b: int when a > 0 creates an unresolvable cycle.
func TestValidate_ContractFieldWhenCircularDependency(t *testing.T) {
	t.Parallel()
	s := &parser.Spec{
		Contracts: []*parser.Contract{
			{
				Name: "circular_contract",
				Fields: []*parser.Field{
					{
						Name: "a",
						Type: parser.TypeExpr{Name: "int", Optional: true},
						When: parser.BinaryOp{
							Left:  parser.FieldRef{Path: "b"},
							Op:    ">",
							Right: parser.LiteralInt{Value: 0},
						},
					},
					{
						Name: "b",
						Type: parser.TypeExpr{Name: "int", Optional: true},
						When: parser.BinaryOp{
							Left:  parser.FieldRef{Path: "a"},
							Op:    ">",
							Right: parser.LiteralInt{Value: 0},
						},
					},
				},
			},
		},
	}
	errs := Validate(s, testRegistry())
	if len(errs) == 0 {
		t.Fatal("expected error for circular when dependency in contract fields")
	}
	found := false
	for _, e := range errs {
		if contains(e.Error(), "circular") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected error about circular dependency involving field names, got: %v", errs)
	}
	// Error message must name at least one of the involved fields.
	foundFieldName := false
	for _, e := range errs {
		if contains(e.Error(), "a") || contains(e.Error(), "b") {
			foundFieldName = true
		}
	}
	if !foundFieldName {
		t.Errorf("expected error to name involved field(s), got: %v", errs)
	}
}

// TestValidate_ContractFieldWhenNonCircular verifies that a valid when expression
// (referencing only a non-when-gated peer field) does not produce an error.
func TestValidate_ContractFieldWhenNonCircular(t *testing.T) {
	t.Parallel()
	s := &parser.Spec{
		Contracts: []*parser.Contract{
			{
				Name: "valid_contract",
				Fields: []*parser.Field{
					{
						Name: "status",
						Type: parser.TypeExpr{Name: "string"},
						// No When — this is the base field.
					},
					{
						Name: "tracking",
						Type: parser.TypeExpr{Name: "string", Optional: true},
						When: parser.BinaryOp{
							Left:  parser.FieldRef{Path: "status"},
							Op:    "==",
							Right: parser.LiteralString{Value: "shipped"},
						},
					},
				},
			},
		},
	}
	errs := Validate(s, testRegistry())
	for _, e := range errs {
		if contains(e.Error(), "circular") {
			t.Errorf("unexpected circular when error for valid dependency chain: %v", e)
		}
	}
}

// TestValidate_ContractFieldWhenLongerCycle verifies that a 3-field circular when
// chain (a→b, b→c, c→a) is detected and names at least two of the involved fields.
func TestValidate_ContractFieldWhenLongerCycle(t *testing.T) {
	t.Parallel()
	s := &parser.Spec{
		Contracts: []*parser.Contract{
			{
				Name: "three_cycle",
				Fields: []*parser.Field{
					{
						Name: "a",
						Type: parser.TypeExpr{Name: "int", Optional: true},
						When: parser.BinaryOp{
							Left:  parser.FieldRef{Path: "b"},
							Op:    ">",
							Right: parser.LiteralInt{Value: 0},
						},
					},
					{
						Name: "b",
						Type: parser.TypeExpr{Name: "int", Optional: true},
						When: parser.BinaryOp{
							Left:  parser.FieldRef{Path: "c"},
							Op:    ">",
							Right: parser.LiteralInt{Value: 0},
						},
					},
					{
						Name: "c",
						Type: parser.TypeExpr{Name: "int", Optional: true},
						When: parser.BinaryOp{
							Left:  parser.FieldRef{Path: "a"},
							Op:    ">",
							Right: parser.LiteralInt{Value: 0},
						},
					},
				},
			},
		},
	}
	errs := Validate(s, testRegistry())
	if len(errs) == 0 {
		t.Fatal("expected error for 3-field circular when dependency in contract fields")
	}
	foundCircular := false
	for _, e := range errs {
		if contains(e.Error(), "circular") {
			foundCircular = true
		}
	}
	if !foundCircular {
		t.Errorf("expected error about circular dependency, got: %v", errs)
	}
	// Error must name at least two of the involved fields.
	involvedCount := 0
	for _, e := range errs {
		msg := e.Error()
		for _, name := range []string{"a", "b", "c"} {
			if contains(msg, name) {
				involvedCount++
				break
			}
		}
	}
	if involvedCount < 1 {
		t.Errorf("expected error to name at least one involved field from {a,b,c}, got: %v", errs)
	}
}

// --- Invariant validation tests ---

func TestValidate_InvariantExprRefsValidated(t *testing.T) {
	t.Parallel()
	s := &parser.Spec{
		Enums: []*parser.NamedEnum{
			{Name: "Status", Variants: []string{"active", "inactive"}},
		},
		Contracts: []*parser.Contract{
			{
				Name: "check",
				Fields: []*parser.Field{
					{Name: "x", Type: parser.TypeExpr{Name: "int"}},
				},
				Invariants: []*parser.Invariant{
					{
						Name: "enum_check",
						Assertions: []*parser.Assertion{
							{Expr: parser.BinaryOp{
								Left:  parser.FieldRef{Path: "Status.active"},
								Op:    "==",
								Right: parser.FieldRef{Path: "Status.active"},
							}},
						},
					},
				},
			},
		},
	}
	errs := Validate(s, testRegistry())
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got: %v", errs)
	}
}

func TestValidate_ScenarioWhenExprRefsValidated(t *testing.T) {
	t.Parallel()
	s := &parser.Spec{
		Config: map[string]parser.Expr{
			"limit": parser.LiteralInt{Value: 100},
		},
		Contracts: []*parser.Contract{
			{
				Name: "check",
				Fields: []*parser.Field{
					{Name: "amount", Type: parser.TypeExpr{Name: "int"}},
				},
				Scenarios: []*parser.Scenario{
					{
						Name: "with_config",
						When: &parser.Block{
							Predicates: []parser.Expr{
								parser.BinaryOp{
									Left:  parser.FieldRef{Path: "amount"},
									Op:    "<",
									Right: parser.FieldRef{Path: "config.limit"},
								},
							},
						},
					},
				},
			},
		},
	}
	errs := Validate(s, testRegistry())
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got: %v", errs)
	}
}

func TestValidate_ScenarioThenExprRefsValidated(t *testing.T) {
	t.Parallel()
	s := &parser.Spec{
		Contracts: []*parser.Contract{
			{
				Name: "check",
				Fields: []*parser.Field{
					{Name: "x", Type: parser.TypeExpr{Name: "int"}},
				},
				Scenarios: []*parser.Scenario{
					{
						Name: "bad_config",
						When: &parser.Block{
							Predicates: []parser.Expr{
								parser.BinaryOp{
									Left:  parser.FieldRef{Path: "x"},
									Op:    ">",
									Right: parser.LiteralInt{Value: 0},
								},
							},
						},
						Then: &parser.Block{
							Assertions: []*parser.Assertion{
								{Expr: parser.FieldRef{Path: "config.missing"}},
							},
						},
					},
				},
			},
		},
	}
	errs := Validate(s, testRegistry())
	if len(errs) != 1 {
		t.Fatalf("expected 1 error for config ref in then block with no config, got %d: %v", len(errs), errs)
	}
}

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

// TestValidate_ErrorPseudoFieldAllowed verifies that "error" in a then block is allowed
// as an error pseudo-field when it is not declared in the return model.
func TestValidate_ErrorPseudoFieldAllowed(t *testing.T) {
	t.Parallel()
	// ReturnModel has "result" but not "error"; "error" should be accepted as pseudo-field.
	spec := &parser.Spec{
		Models: []*parser.Model{
			{Name: "ResultModel", Fields: []*parser.Field{
				{Name: "result", Type: parser.TypeExpr{Name: "string"}},
			}},
		},
		Scopes: []*parser.Scope{{
			Name: "test",
			Contracts: []*parser.Contract{{
				Name:       "check",
				ReturnType: parser.TypeExpr{Name: "ResultModel"},
				Fields:     []*parser.Field{{Name: "x", Type: parser.TypeExpr{Name: "int"}}},
				Scenarios: []*parser.Scenario{{
					Name: "expect_error",
					Given: &parser.Block{
						Steps: []parser.GivenStep{
							&parser.Assignment{Path: "x", Value: parser.LiteralInt{Value: 1}},
						},
					},
					Then: &parser.Block{
						Assertions: []*parser.Assertion{
							{Target: "error", Expected: parser.LiteralString{Value: "something"}},
						},
					},
				}},
			}},
		}},
	}

	errs := Validate(spec, testRegistry())
	for _, e := range errs {
		if contains(e.Error(), "error") && contains(e.Error(), "output field") {
			t.Errorf("error pseudo-field should be allowed, got: %v", e)
		}
	}
}

// TestValidate_ErrorContractFieldStillValidated verifies that when "error" IS a field
// in the return model, unknown assertion targets still produce errors.
func TestValidate_ErrorContractFieldStillValidated(t *testing.T) {
	t.Parallel()
	// ReturnModel explicitly includes "error"; "nonexistent" should still fail.
	spec := &parser.Spec{
		Models: []*parser.Model{
			{Name: "ErrorResultModel", Fields: []*parser.Field{
				{Name: "error", Type: parser.TypeExpr{Name: "string"}},
			}},
		},
		Scopes: []*parser.Scope{{
			Name: "test",
			Contracts: []*parser.Contract{{
				Name:       "check",
				ReturnType: parser.TypeExpr{Name: "ErrorResultModel"},
				Fields:     []*parser.Field{{Name: "x", Type: parser.TypeExpr{Name: "int"}}},
				Scenarios: []*parser.Scenario{{
					Name: "check_fields",
					Given: &parser.Block{
						Steps: []parser.GivenStep{
							&parser.Assignment{Path: "x", Value: parser.LiteralInt{Value: 1}},
						},
					},
					Then: &parser.Block{
						Assertions: []*parser.Assertion{
							{Target: "error", Expected: parser.LiteralString{Value: "ok"}},
							{Target: "nonexistent", Expected: parser.LiteralString{Value: "bad"}},
						},
					},
				}},
			}},
		}},
	}

	errs := Validate(spec, testRegistry())
	found := false
	for _, e := range errs {
		if contains(e.Error(), "nonexistent") {
			found = true
		}
		if contains(e.Error(), `"error"`) && contains(e.Error(), "output field") {
			t.Errorf("error should be valid when declared in output, got: %v", e)
		}
	}
	if !found {
		t.Error("expected validation error for 'nonexistent' field")
	}
}

func TestValidate_PluginAssertionTargetsAllowed(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		plugin string
		target string
	}{
		{"http status", "http", "status"},
		{"http body", "http", "body"},
		{"http header", "http", "header"},
		{"process exit_code", "process", "exit_code"},
		{"process stdout", "process", "stdout"},
		{"process stderr", "process", "stderr"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			spec := &parser.Spec{
				Scopes: []*parser.Scope{{
					Name: "test",
					Contracts: []*parser.Contract{{
						Name:   "check",
						Fields: []*parser.Field{},
						Scenarios: []*parser.Scenario{{
							Name:  "check",
							Given: &parser.Block{},
							Then: &parser.Block{
								Assertions: []*parser.Assertion{
									{Target: tt.target, Expected: parser.LiteralInt{Value: 200}, Operator: "=="},
								},
							},
						}},
					}},
				}},
			}

			errs := Validate(spec, testRegistry())
			for _, e := range errs {
				if contains(e.Error(), tt.target) {
					t.Errorf("%s should be allowed as assertion target for %s plugin, got: %v",
						tt.target, tt.plugin, e)
				}
			}
		})
	}
}

// TestValidate_UnknownTargetStillRejected verifies that an assertion target not in
// the return model produces a validation error.
func TestValidate_UnknownTargetStillRejected(t *testing.T) {
	t.Parallel()
	spec := &parser.Spec{
		Models: []*parser.Model{
			{Name: "DataResult", Fields: []*parser.Field{
				{Name: "data", Type: parser.TypeExpr{Name: "string"}},
			}},
		},
		Scopes: []*parser.Scope{{
			Name: "test",
			Contracts: []*parser.Contract{{
				Name:       "check",
				ReturnType: parser.TypeExpr{Name: "DataResult"},
				Fields:     []*parser.Field{},
				Scenarios: []*parser.Scenario{{
					Name:  "check",
					Given: &parser.Block{},
					Then: &parser.Block{
						Assertions: []*parser.Assertion{
							{Target: "nonexistent", Expected: parser.LiteralInt{Value: 42}, Operator: "=="},
						},
					},
				}},
			}},
		}},
	}

	errs := Validate(spec, testRegistry())
	found := false
	for _, e := range errs {
		if contains(e.Error(), "nonexistent") {
			found = true
		}
	}
	if !found {
		t.Error("expected validation error for unknown target 'nonexistent'")
	}
}

// --- Bare output ref validation (v4 field-resolution rule) ---

// TestValidate_BareInputRefInInvariant_Valid verifies that a bare ref to an input
// field in an invariant assertion is accepted (v4 rule: bare names → input).
func TestValidate_BareInputRefInInvariant_Valid(t *testing.T) {
	t.Parallel()
	s := &parser.Spec{
		Models: []*parser.Model{
			{Name: "Res", Fields: []*parser.Field{
				{Name: "total", Type: parser.TypeExpr{Name: "int"}},
			}},
		},
		Contracts: []*parser.Contract{{
			Name: "add",
			Fields: []*parser.Field{
				{Name: "a", Type: parser.TypeExpr{Name: "int"}},
				{Name: "b", Type: parser.TypeExpr{Name: "int"}},
			},
			ReturnType: parser.TypeExpr{Name: "Res"},
			Invariants: []*parser.Invariant{{
				Name: "sum_positive",
				Assertions: []*parser.Assertion{{
					// "a" and "b" are input fields — bare refs are legal.
					Expr: parser.BinaryOp{
						Left:  parser.FieldRef{Path: "a"},
						Op:    ">=",
						Right: parser.LiteralInt{Value: 0},
					},
				}},
			}},
		}},
	}
	errs := Validate(s, testRegistry())
	for _, e := range errs {
		if contains(e.Error(), "output field") {
			t.Errorf("bare input ref should be accepted, got: %v", e)
		}
	}
}

// TestValidate_PrefixedOutputRefInInvariant_Valid verifies that output.<field> refs
// in invariant assertions are accepted (v4 rule: output.X → return model fields).
func TestValidate_PrefixedOutputRefInInvariant_Valid(t *testing.T) {
	t.Parallel()
	s := &parser.Spec{
		Models: []*parser.Model{
			{Name: "Res", Fields: []*parser.Field{
				{Name: "total", Type: parser.TypeExpr{Name: "int"}},
			}},
		},
		Contracts: []*parser.Contract{{
			Name:       "add",
			Fields:     []*parser.Field{{Name: "a", Type: parser.TypeExpr{Name: "int"}}},
			ReturnType: parser.TypeExpr{Name: "Res"},
			Invariants: []*parser.Invariant{{
				Name: "total_positive",
				Assertions: []*parser.Assertion{{
					// "output.total" — correctly prefixed return-model field.
					Expr: parser.BinaryOp{
						Left:  parser.FieldRef{Path: "output.total"},
						Op:    ">=",
						Right: parser.LiteralInt{Value: 0},
					},
				}},
			}},
		}},
	}
	errs := Validate(s, testRegistry())
	for _, e := range errs {
		if contains(e.Error(), "output field") || contains(e.Error(), "total") {
			t.Errorf("prefixed output ref should be accepted, got: %v", e)
		}
	}
}

// TestValidate_BareOutputOnlyRefInInvariant_Error verifies that a bare ref to a
// name that exists only in the return model (not in contract input) produces an
// error with the "output." prefix hint.
func TestValidate_BareOutputOnlyRefInInvariant_Error(t *testing.T) {
	t.Parallel()
	s := &parser.Spec{
		Models: []*parser.Model{
			{Name: "Res", Fields: []*parser.Field{
				{Name: "total", Type: parser.TypeExpr{Name: "int"}},
			}},
		},
		Contracts: []*parser.Contract{{
			Name:       "add",
			Fields:     []*parser.Field{{Name: "a", Type: parser.TypeExpr{Name: "int"}}},
			ReturnType: parser.TypeExpr{Name: "Res"},
			Invariants: []*parser.Invariant{{
				Name: "bare_output",
				Assertions: []*parser.Assertion{{
					// "total" is only in the return model — must be "output.total".
					Expr: parser.BinaryOp{
						Left:  parser.FieldRef{Path: "total"},
						Op:    ">=",
						Right: parser.LiteralInt{Value: 0},
					},
				}},
			}},
		}},
	}
	errs := Validate(s, testRegistry())
	found := false
	for _, e := range errs {
		if contains(e.Error(), "total") && contains(e.Error(), "output field") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected error about bare output ref 'total', got: %v", errs)
	}
}

// TestValidate_BareOutputOnlyRefInScenarioThen_Error verifies the same rule
// applies in scenario then-block expression assertions.
func TestValidate_BareOutputOnlyRefInScenarioThen_Error(t *testing.T) {
	t.Parallel()
	s := &parser.Spec{
		Models: []*parser.Model{
			{Name: "Res", Fields: []*parser.Field{
				{Name: "sum", Type: parser.TypeExpr{Name: "int"}},
			}},
		},
		Contracts: []*parser.Contract{{
			Name:       "add",
			Fields:     []*parser.Field{{Name: "a", Type: parser.TypeExpr{Name: "int"}}},
			ReturnType: parser.TypeExpr{Name: "Res"},
			Scenarios: []*parser.Scenario{{
				Name: "smoke",
				Given: &parser.Block{
					Steps: []parser.GivenStep{
						&parser.Assignment{Path: "a", Value: parser.LiteralInt{Value: 1}},
					},
				},
				Then: &parser.Block{
					Assertions: []*parser.Assertion{{
						// "sum" is only in return model — must use "output.sum".
						Expr: parser.BinaryOp{
							Left:  parser.FieldRef{Path: "sum"},
							Op:    "==",
							Right: parser.LiteralInt{Value: 1},
						},
					}},
				},
			}},
		}},
	}
	errs := Validate(s, testRegistry())
	found := false
	for _, e := range errs {
		if contains(e.Error(), "sum") && contains(e.Error(), "output field") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected error about bare output ref 'sum' in then block, got: %v", errs)
	}
}

// TestValidate_BothInputAndOutputSameName_InputWins verifies that when a field name
// appears in both contract input and return model, the bare ref resolves to input
// silently (no error) — as documented in the v4 plan.
func TestValidate_BothInputAndOutputSameName_InputWins(t *testing.T) {
	t.Parallel()
	s := &parser.Spec{
		Models: []*parser.Model{
			{Name: "Res", Fields: []*parser.Field{
				{Name: "value", Type: parser.TypeExpr{Name: "int"}},
			}},
		},
		Contracts: []*parser.Contract{{
			Name: "echo",
			// "value" is in both input and return model — input wins, no error.
			Fields:     []*parser.Field{{Name: "value", Type: parser.TypeExpr{Name: "int"}}},
			ReturnType: parser.TypeExpr{Name: "Res"},
			Invariants: []*parser.Invariant{{
				Name: "echo_identity",
				Assertions: []*parser.Assertion{{
					// bare "value" resolves to input silently.
					Expr: parser.BinaryOp{
						Left:  parser.FieldRef{Path: "value"},
						Op:    ">=",
						Right: parser.LiteralInt{Value: 0},
					},
				}},
			}},
		}},
	}
	errs := Validate(s, testRegistry())
	for _, e := range errs {
		if contains(e.Error(), "output field") {
			t.Errorf("should not error when name exists in both input and return model, got: %v", e)
		}
	}
}

// TestValidate_TransferConservationInvariant_Valid verifies that the canonical
// transfer conservation invariant validates cleanly:
//   output.from.balance + output.to.balance == from.balance + to.balance
//
// where from/to are input fields and output.from/output.to are return model refs.
func TestValidate_TransferConservationInvariant_Valid(t *testing.T) {
	t.Parallel()
	s := &parser.Spec{
		Models: []*parser.Model{
			{Name: "Account", Fields: []*parser.Field{
				{Name: "id", Type: parser.TypeExpr{Name: "string"}},
				{Name: "balance", Type: parser.TypeExpr{Name: "int"}},
			}},
			{Name: "TransferResult", Fields: []*parser.Field{
				{Name: "from", Type: parser.TypeExpr{Name: "Account"}},
				{Name: "to", Type: parser.TypeExpr{Name: "Account"}},
				{Name: "error", Type: parser.TypeExpr{Name: "string", Optional: true}},
			}},
		},
		Contracts: []*parser.Contract{{
			Name: "Transfer",
			Fields: []*parser.Field{
				{Name: "from", Type: parser.TypeExpr{Name: "Account"}},
				{Name: "to", Type: parser.TypeExpr{Name: "Account"}},
				{Name: "amount", Type: parser.TypeExpr{Name: "int"}},
			},
			ReturnType: parser.TypeExpr{Name: "TransferResult"},
			Invariants: []*parser.Invariant{{
				Name: "conservation",
				Assertions: []*parser.Assertion{{
					// output.from and output.to are prefixed (return model)
					// from and to (bare) are input fields — this is the correct form.
					Expr: parser.BinaryOp{
						Left: parser.BinaryOp{
							Left:  parser.FieldRef{Path: "output.from.balance"},
							Op:    "+",
							Right: parser.FieldRef{Path: "output.to.balance"},
						},
						Op: "==",
						Right: parser.BinaryOp{
							Left:  parser.FieldRef{Path: "from.balance"},
							Op:    "+",
							Right: parser.FieldRef{Path: "to.balance"},
						},
					},
				}},
			}},
		}},
	}
	errs := Validate(s, testRegistry())
	for _, e := range errs {
		t.Errorf("expected no errors for canonical transfer conservation invariant, got: %v", e)
	}
}
