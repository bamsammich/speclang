package generator

import (
	"testing"

	"github.com/bamsammich/speclang/v2/pkg/parser"
)

func TestGenerateEnum(t *testing.T) {
	variants := []string{"http", "process", "playwright"}
	g := New(
		&parser.Contract{
			Input: []*parser.Field{
				{Name: "plugin", Type: parser.TypeExpr{Name: "enum", Variants: variants}},
			},
		},
		nil, 42,
	)

	seen := make(map[string]bool)
	for range 100 {
		input, err := g.GenerateInput()
		if err != nil {
			t.Fatal(err)
		}
		val, ok := input["plugin"].(string)
		if !ok {
			t.Fatalf("expected string for plugin, got %T", input["plugin"])
		}
		seen[val] = true
		// Verify it's a valid variant
		found := false
		for _, v := range variants {
			if val == v {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("generated %q which is not a valid variant", val)
		}
	}

	// With 100 draws from 3 variants, we should see all of them
	for _, v := range variants {
		if !seen[v] {
			t.Errorf("variant %q was never generated in 100 draws", v)
		}
	}
}

func TestGenerateEnum_SingleVariant(t *testing.T) {
	g := New(
		&parser.Contract{
			Input: []*parser.Field{
				{Name: "mode", Type: parser.TypeExpr{Name: "enum", Variants: []string{"default"}}},
			},
		},
		nil, 42,
	)

	input, err := g.GenerateInput()
	if err != nil {
		t.Fatal(err)
	}
	val, ok := input["mode"].(string)
	if !ok {
		t.Fatalf("expected string, got %T", input["mode"])
	}
	if val != "default" {
		t.Errorf("got %q, want 'default'", val)
	}
}

func TestGenerateEnum_Optional(t *testing.T) {
	g := New(
		&parser.Contract{
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
		nil, 42,
	)

	sawNil := false
	sawValue := false
	for range 100 {
		input, err := g.GenerateInput()
		if err != nil {
			t.Fatal(err)
		}
		if input["role"] == nil {
			sawNil = true
		} else {
			sawValue = true
			val, ok := input["role"].(string)
			if !ok {
				t.Fatalf("expected string or nil, got %T", input["role"])
			}
			if val != "admin" && val != "user" {
				t.Errorf("got %q, want 'admin' or 'user'", val)
			}
		}
	}
	if !sawNil {
		t.Error("optional enum never produced nil in 100 draws")
	}
	if !sawValue {
		t.Error("optional enum never produced a value in 100 draws")
	}
}

func TestShrinkEnum(t *testing.T) {
	variants := []string{"alpha", "beta", "gamma"}
	input := map[string]any{"plugin": "gamma"}
	fields := []*parser.Field{
		{Name: "plugin", Type: parser.TypeExpr{Name: "enum", Variants: variants}},
	}

	// stillFails returns true for any variant (so shrinking picks the first)
	result := Shrink(input, fields, nil, func(m map[string]any) bool {
		return true
	})

	if result["plugin"] != "alpha" {
		t.Errorf("shrunk to %q, want 'alpha' (first variant)", result["plugin"])
	}
}

func TestShrinkEnum_SpecificVariant(t *testing.T) {
	variants := []string{"alpha", "beta", "gamma"}
	input := map[string]any{"plugin": "gamma"}
	fields := []*parser.Field{
		{Name: "plugin", Type: parser.TypeExpr{Name: "enum", Variants: variants}},
	}

	// Only fails for "beta" and "gamma"
	result := Shrink(input, fields, nil, func(m map[string]any) bool {
		v := m["plugin"].(string)
		return v == "beta" || v == "gamma"
	})

	if result["plugin"] != "beta" {
		t.Errorf("shrunk to %q, want 'beta' (earliest failing variant)", result["plugin"])
	}
}
