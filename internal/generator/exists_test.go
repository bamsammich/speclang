package generator

import (
	"testing"

	"github.com/bamsammich/speclang/v2/internal/parser"
)

func TestEvalExists_Present(t *testing.T) {
	vars := map[string]any{
		"output": map[string]any{
			"name": "alice",
		},
	}
	expr := parser.ExistsExpr{Arg: parser.FieldRef{Path: "output.name"}}
	val, ok := Eval(expr, vars)
	if !ok {
		t.Fatal("Eval returned not-ok")
	}
	if val != true {
		t.Fatalf("got %v, want true", val)
	}
}

func TestEvalExists_Missing(t *testing.T) {
	vars := map[string]any{
		"output": map[string]any{
			"name": "alice",
		},
	}
	expr := parser.ExistsExpr{Arg: parser.FieldRef{Path: "output.missing"}}
	val, ok := Eval(expr, vars)
	if !ok {
		t.Fatal("Eval returned not-ok")
	}
	if val != false {
		t.Fatalf("got %v, want false", val)
	}
}

func TestEvalExists_NilValue(t *testing.T) {
	// Key exists but value is nil — exists should return true
	vars := map[string]any{
		"output": map[string]any{
			"error": nil,
		},
	}
	expr := parser.ExistsExpr{Arg: parser.FieldRef{Path: "output.error"}}
	val, ok := Eval(expr, vars)
	if !ok {
		t.Fatal("Eval returned not-ok")
	}
	if val != true {
		t.Fatalf("got %v, want true (key exists with nil value)", val)
	}
}

func TestEvalExists_NestedPath(t *testing.T) {
	vars := map[string]any{
		"output": map[string]any{
			"data": map[string]any{
				"items": []any{1, 2, 3},
			},
		},
	}
	expr := parser.ExistsExpr{Arg: parser.FieldRef{Path: "output.data.items"}}
	val, ok := Eval(expr, vars)
	if !ok {
		t.Fatal("Eval returned not-ok")
	}
	if val != true {
		t.Fatalf("got %v, want true", val)
	}
}

func TestEvalExists_MissingIntermediate(t *testing.T) {
	vars := map[string]any{
		"output": map[string]any{
			"name": "alice",
		},
	}
	expr := parser.ExistsExpr{Arg: parser.FieldRef{Path: "output.data.items"}}
	val, ok := Eval(expr, vars)
	if !ok {
		t.Fatal("Eval returned not-ok")
	}
	if val != false {
		t.Fatalf("got %v, want false", val)
	}
}

func TestEvalHasKey_Present(t *testing.T) {
	vars := map[string]any{
		"data": map[string]any{
			"name":  "alice",
			"score": 42,
		},
	}
	expr := parser.HasKeyExpr{
		Arg: parser.FieldRef{Path: "data"},
		Key: parser.LiteralString{Value: "name"},
	}
	val, ok := Eval(expr, vars)
	if !ok {
		t.Fatal("Eval returned not-ok")
	}
	if val != true {
		t.Fatalf("got %v, want true", val)
	}
}

func TestEvalHasKey_Missing(t *testing.T) {
	vars := map[string]any{
		"data": map[string]any{
			"name": "alice",
		},
	}
	expr := parser.HasKeyExpr{
		Arg: parser.FieldRef{Path: "data"},
		Key: parser.LiteralString{Value: "missing"},
	}
	val, ok := Eval(expr, vars)
	if !ok {
		t.Fatal("Eval returned not-ok")
	}
	if val != false {
		t.Fatalf("got %v, want false", val)
	}
}

func TestEvalHasKey_NotAMap(t *testing.T) {
	vars := map[string]any{
		"data": "not a map",
	}
	expr := parser.HasKeyExpr{
		Arg: parser.FieldRef{Path: "data"},
		Key: parser.LiteralString{Value: "key"},
	}
	val, ok := Eval(expr, vars)
	if !ok {
		t.Fatal("Eval returned not-ok")
	}
	if val != false {
		t.Fatalf("got %v, want false (not a map)", val)
	}
}

func TestEvalHasKey_MissingObj(t *testing.T) {
	vars := map[string]any{}
	expr := parser.HasKeyExpr{
		Arg: parser.FieldRef{Path: "missing"},
		Key: parser.LiteralString{Value: "key"},
	}
	val, ok := Eval(expr, vars)
	if !ok {
		t.Fatal("Eval returned not-ok")
	}
	if val != false {
		t.Fatalf("got %v, want false (missing object)", val)
	}
}

func TestEvalExists_WithNegation(t *testing.T) {
	// !exists(output.missing) should evaluate to true
	vars := map[string]any{
		"output": map[string]any{},
	}
	expr := parser.UnaryOp{
		Op:      "!",
		Operand: parser.ExistsExpr{Arg: parser.FieldRef{Path: "output.missing"}},
	}
	val, ok := Eval(expr, vars)
	if !ok {
		t.Fatal("Eval returned not-ok")
	}
	if val != true {
		t.Fatalf("got %v, want true (!exists on missing path)", val)
	}
}
