package validator

import (
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

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
