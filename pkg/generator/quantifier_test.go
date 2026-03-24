package generator

import (
	"testing"

	"github.com/bamsammich/speclang/v2/pkg/parser"
)

func TestEvalAll_AllTrue(t *testing.T) {
	t.Parallel()

	vars := map[string]any{
		"items": []any{1, 2, 3, 4, 5},
	}
	// all(items, x => x > 0)
	expr := parser.AllExpr{
		Array:    parser.FieldRef{Path: "items"},
		BoundVar: "x",
		Predicate: parser.BinaryOp{
			Left: parser.FieldRef{Path: "x"}, Op: ">", Right: parser.LiteralInt{Value: 0},
		},
	}

	val, ok := Eval(expr, vars)
	if !ok {
		t.Fatal("eval failed")
	}
	if val != true {
		t.Errorf("expected true, got %v", val)
	}
}

func TestEvalAll_OneFalse(t *testing.T) {
	t.Parallel()

	vars := map[string]any{
		"items": []any{1, 2, -1, 4, 5},
	}
	expr := parser.AllExpr{
		Array:    parser.FieldRef{Path: "items"},
		BoundVar: "x",
		Predicate: parser.BinaryOp{
			Left: parser.FieldRef{Path: "x"}, Op: ">", Right: parser.LiteralInt{Value: 0},
		},
	}

	val, ok := Eval(expr, vars)
	if !ok {
		t.Fatal("eval failed")
	}
	if val != false {
		t.Errorf("expected false, got %v", val)
	}
}

func TestEvalAll_EmptyArray(t *testing.T) {
	t.Parallel()

	vars := map[string]any{
		"items": []any{},
	}
	expr := parser.AllExpr{
		Array:    parser.FieldRef{Path: "items"},
		BoundVar: "x",
		Predicate: parser.BinaryOp{
			Left: parser.FieldRef{Path: "x"}, Op: ">", Right: parser.LiteralInt{Value: 0},
		},
	}

	val, ok := Eval(expr, vars)
	if !ok {
		t.Fatal("eval failed")
	}
	if val != true {
		t.Errorf("expected true for vacuous all on empty array, got %v", val)
	}
}

func TestEvalAny_OneTrue(t *testing.T) {
	t.Parallel()

	vars := map[string]any{
		"items": []any{-1, -2, 3, -4},
	}
	expr := parser.AnyExpr{
		Array:    parser.FieldRef{Path: "items"},
		BoundVar: "x",
		Predicate: parser.BinaryOp{
			Left: parser.FieldRef{Path: "x"}, Op: ">", Right: parser.LiteralInt{Value: 0},
		},
	}

	val, ok := Eval(expr, vars)
	if !ok {
		t.Fatal("eval failed")
	}
	if val != true {
		t.Errorf("expected true, got %v", val)
	}
}

func TestEvalAny_AllFalse(t *testing.T) {
	t.Parallel()

	vars := map[string]any{
		"items": []any{-1, -2, -3},
	}
	expr := parser.AnyExpr{
		Array:    parser.FieldRef{Path: "items"},
		BoundVar: "x",
		Predicate: parser.BinaryOp{
			Left: parser.FieldRef{Path: "x"}, Op: ">", Right: parser.LiteralInt{Value: 0},
		},
	}

	val, ok := Eval(expr, vars)
	if !ok {
		t.Fatal("eval failed")
	}
	if val != false {
		t.Errorf("expected false, got %v", val)
	}
}

func TestEvalAny_EmptyArray(t *testing.T) {
	t.Parallel()

	vars := map[string]any{
		"items": []any{},
	}
	expr := parser.AnyExpr{
		Array:    parser.FieldRef{Path: "items"},
		BoundVar: "x",
		Predicate: parser.BinaryOp{
			Left: parser.FieldRef{Path: "x"}, Op: ">", Right: parser.LiteralInt{Value: 0},
		},
	}

	val, ok := Eval(expr, vars)
	if !ok {
		t.Fatal("eval failed")
	}
	if val != false {
		t.Errorf("expected false for any on empty array, got %v", val)
	}
}

func TestEvalNestedQuantifiers(t *testing.T) {
	t.Parallel()

	vars := map[string]any{
		"matrix": []any{
			[]any{1, 2, 3},
			[]any{-1, 5},
			[]any{0, 10},
		},
	}
	// all(matrix, row => any(row, x => x > 0))
	expr := parser.AllExpr{
		Array:    parser.FieldRef{Path: "matrix"},
		BoundVar: "row",
		Predicate: parser.AnyExpr{
			Array:    parser.FieldRef{Path: "row"},
			BoundVar: "x",
			Predicate: parser.BinaryOp{
				Left: parser.FieldRef{Path: "x"}, Op: ">", Right: parser.LiteralInt{Value: 0},
			},
		},
	}

	val, ok := Eval(expr, vars)
	if !ok {
		t.Fatal("eval failed")
	}
	if val != true {
		t.Errorf("expected true (every row has at least one positive), got %v", val)
	}
}

func TestEvalQuantifier_BoundVarDoesNotLeak(t *testing.T) {
	t.Parallel()

	vars := map[string]any{
		"items": []any{1, 2, 3},
	}
	// Evaluate all() — bound var "x" should not exist after
	expr := parser.AllExpr{
		Array:    parser.FieldRef{Path: "items"},
		BoundVar: "x",
		Predicate: parser.BinaryOp{
			Left: parser.FieldRef{Path: "x"}, Op: ">", Right: parser.LiteralInt{Value: 0},
		},
	}

	Eval(expr, vars)

	// Verify "x" is not in vars
	if _, exists := vars["x"]; exists {
		t.Error("bound variable 'x' leaked into outer scope")
	}
}

func TestEvalAll_WithObjectElements(t *testing.T) {
	t.Parallel()

	vars := map[string]any{
		"scopes": []any{
			map[string]any{"name": "transfer", "passed": true},
			map[string]any{"name": "deposit", "passed": true},
		},
	}
	// all(scopes, s => s.passed == true)
	expr := parser.AllExpr{
		Array:    parser.FieldRef{Path: "scopes"},
		BoundVar: "s",
		Predicate: parser.BinaryOp{
			Left: parser.FieldRef{Path: "s.passed"}, Op: "==", Right: parser.LiteralBool{Value: true},
		},
	}

	val, ok := Eval(expr, vars)
	if !ok {
		t.Fatal("eval failed")
	}
	if val != true {
		t.Errorf("expected true, got %v", val)
	}
}

func TestEvalAny_WithStringPredicate(t *testing.T) {
	t.Parallel()

	vars := map[string]any{
		"scopes": []any{
			map[string]any{"name": "transfer"},
			map[string]any{"name": "deposit"},
		},
	}
	// any(scopes, s => s.name == "transfer")
	expr := parser.AnyExpr{
		Array:    parser.FieldRef{Path: "scopes"},
		BoundVar: "s",
		Predicate: parser.BinaryOp{
			Left: parser.FieldRef{Path: "s.name"}, Op: "==", Right: parser.LiteralString{Value: "transfer"},
		},
	}

	val, ok := Eval(expr, vars)
	if !ok {
		t.Fatal("eval failed")
	}
	if val != true {
		t.Errorf("expected true, got %v", val)
	}
}
