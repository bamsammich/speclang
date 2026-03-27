package generator

import (
	"testing"

	"github.com/bamsammich/speclang/v2/internal/parser"
)

func TestResolveRef_ArrayIndex(t *testing.T) {
	t.Parallel()

	ctx := &evalCtx{
		input: map[string]any{
			"items": []any{"first", "second", "third"},
		},
	}

	val, ok := ctx.resolveRef("items.0")
	if !ok {
		t.Fatal("resolveRef returned ok=false for items.0")
	}
	if val != "first" {
		t.Errorf("items.0 = %v, want %q", val, "first")
	}

	val, ok = ctx.resolveRef("items.2")
	if !ok {
		t.Fatal("resolveRef returned ok=false for items.2")
	}
	if val != "third" {
		t.Errorf("items.2 = %v, want %q", val, "third")
	}
}

func TestResolveRef_NestedArrayIndex(t *testing.T) {
	t.Parallel()

	ctx := &evalCtx{
		input: map[string]any{
			"results": []any{
				map[string]any{"name": "alice"},
				map[string]any{"name": "bob"},
			},
		},
	}

	val, ok := ctx.resolveRef("results.0.name")
	if !ok {
		t.Fatal("resolveRef returned ok=false for results.0.name")
	}
	if val != "alice" {
		t.Errorf("results.0.name = %v, want %q", val, "alice")
	}

	val, ok = ctx.resolveRef("results.1.name")
	if !ok {
		t.Fatal("resolveRef returned ok=false for results.1.name")
	}
	if val != "bob" {
		t.Errorf("results.1.name = %v, want %q", val, "bob")
	}
}

func TestResolveRef_ArrayIndexOutOfRange(t *testing.T) {
	t.Parallel()

	ctx := &evalCtx{
		input: map[string]any{
			"items": []any{"only"},
		},
	}

	_, ok := ctx.resolveRef("items.5")
	if ok {
		t.Error("expected ok=false for out-of-range index")
	}

	_, ok = ctx.resolveRef("items.-1")
	if ok {
		t.Error("expected ok=false for negative index")
	}
}

func TestEval_FieldRef_ArrayIndex(t *testing.T) {
	t.Parallel()

	val, ok := Eval(parser.FieldRef{Path: "output.items.0"}, map[string]any{
		"output": map[string]any{
			"items": []any{"alpha", "beta"},
		},
	})
	if !ok {
		t.Fatal("Eval returned ok=false for output.items.0")
	}
	if val != "alpha" {
		t.Errorf("output.items.0 = %v, want %q", val, "alpha")
	}
}
