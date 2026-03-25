package validator

import (
	"testing"

	"github.com/bamsammich/speclang/v2/internal/parser"
)

func TestValidate_EnumValidVariant(t *testing.T) {
	t.Parallel()
	spec := &parser.Spec{
		Scopes: []*parser.Scope{
			{
				Name: "test",
				Use:  "http",
				Contract: &parser.Contract{
					Input: []*parser.Field{
						{
							Name: "status",
							Type: parser.TypeExpr{
								Name:     "enum",
								Variants: []string{"active", "inactive"},
							},
						},
					},
				},
				Scenarios: []*parser.Scenario{
					{
						Name: "smoke",
						Given: &parser.Block{
							Steps: []parser.GivenStep{
								&parser.Assignment{
									Path:  "status",
									Value: parser.LiteralString{Value: "active"},
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

func TestValidate_EnumInvalidVariant(t *testing.T) {
	t.Parallel()
	spec := &parser.Spec{
		Scopes: []*parser.Scope{
			{
				Name: "test",
				Use:  "http",
				Contract: &parser.Contract{
					Input: []*parser.Field{
						{
							Name: "status",
							Type: parser.TypeExpr{
								Name:     "enum",
								Variants: []string{"active", "inactive"},
							},
						},
					},
				},
				Scenarios: []*parser.Scenario{
					{
						Name: "smoke",
						Given: &parser.Block{
							Steps: []parser.GivenStep{
								&parser.Assignment{
									Path:  "status",
									Value: parser.LiteralString{Value: "deleted"},
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
		t.Fatalf("expected 1 error for invalid variant, got %d: %v", len(errs), errs)
	}
	if !contains(errs[0].Error(), "deleted") {
		t.Errorf("expected error about 'deleted', got: %v", errs[0])
	}
}

func TestValidate_EnumWrongLiteralType(t *testing.T) {
	t.Parallel()
	spec := &parser.Spec{
		Scopes: []*parser.Scope{
			{
				Name: "test",
				Use:  "http",
				Contract: &parser.Contract{
					Input: []*parser.Field{
						{
							Name: "status",
							Type: parser.TypeExpr{
								Name:     "enum",
								Variants: []string{"active", "inactive"},
							},
						},
					},
				},
				Scenarios: []*parser.Scenario{
					{
						Name: "smoke",
						Given: &parser.Block{
							Steps: []parser.GivenStep{
								&parser.Assignment{
									Path:  "status",
									Value: parser.LiteralInt{Value: 42},
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
		t.Fatalf("expected 1 error for non-string assigned to enum, got %d: %v", len(errs), errs)
	}
}

func TestValidate_EnumInContractPasses(t *testing.T) {
	t.Parallel()
	spec := &parser.Spec{
		Scopes: []*parser.Scope{
			{
				Name: "test",
				Use:  "http",
				Contract: &parser.Contract{
					Input: []*parser.Field{
						{
							Name: "status",
							Type: parser.TypeExpr{Name: "enum", Variants: []string{"a", "b"}},
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

func TestValidate_EnumNullForOptional(t *testing.T) {
	t.Parallel()
	spec := &parser.Spec{
		Scopes: []*parser.Scope{
			{
				Name: "test",
				Use:  "http",
				Contract: &parser.Contract{
					Input: []*parser.Field{
						{
							Name: "role",
							Type: parser.TypeExpr{
								Name:     "enum",
								Variants: []string{"admin", "user"},
								Optional: true,
							},
						},
					},
				},
				Scenarios: []*parser.Scenario{
					{
						Name: "smoke",
						Given: &parser.Block{
							Steps: []parser.GivenStep{
								&parser.Assignment{Path: "role", Value: parser.LiteralNull{}},
							},
						},
					},
				},
			},
		},
	}

	errs := Validate(spec)
	if len(errs) != 0 {
		t.Fatalf("expected no errors for null on optional enum, got: %v", errs)
	}
}
