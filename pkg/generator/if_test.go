package generator

import (
	"testing"

	"github.com/bamsammich/speclang/v2/pkg/parser"
)

func TestEvalIfExpr_TrueBranch(t *testing.T) {
	t.Parallel()

	expr := parser.IfExpr{
		Condition: parser.LiteralBool{Value: true},
		Then:      parser.LiteralInt{Value: 42},
		Else:      parser.LiteralInt{Value: 0},
	}
	vars := map[string]any{}
	val, ok := Eval(expr, vars)
	if !ok {
		t.Fatal("eval failed")
	}
	if val != 42 {
		t.Errorf("expected 42, got %v", val)
	}
}

func TestEvalIfExpr_FalseBranch(t *testing.T) {
	t.Parallel()

	expr := parser.IfExpr{
		Condition: parser.LiteralBool{Value: false},
		Then:      parser.LiteralInt{Value: 42},
		Else:      parser.LiteralInt{Value: 0},
	}
	vars := map[string]any{}
	val, ok := Eval(expr, vars)
	if !ok {
		t.Fatal("eval failed")
	}
	if val != 0 {
		t.Errorf("expected 0, got %v", val)
	}
}

func TestEvalIfExpr_ConditionFromExpression(t *testing.T) {
	t.Parallel()

	// if x > 10 then x - 10 else 0
	expr := parser.IfExpr{
		Condition: parser.BinaryOp{
			Left: parser.FieldRef{Path: "x"}, Op: ">", Right: parser.LiteralInt{Value: 10},
		},
		Then: parser.BinaryOp{
			Left: parser.FieldRef{Path: "x"}, Op: "-", Right: parser.LiteralInt{Value: 10},
		},
		Else: parser.LiteralInt{Value: 0},
	}

	// x = 25 → should take then branch: 25 - 10 = 15
	val, ok := Eval(expr, map[string]any{"x": 25})
	if !ok {
		t.Fatal("eval failed")
	}
	if val != 15 {
		t.Errorf("expected 15, got %v", val)
	}

	// x = 5 → should take else branch: 0
	val, ok = Eval(expr, map[string]any{"x": 5})
	if !ok {
		t.Fatal("eval failed")
	}
	if val != 0 {
		t.Errorf("expected 0, got %v", val)
	}
}

func TestEvalIfExpr_Nested(t *testing.T) {
	t.Parallel()

	// if a then (if b then 1 else 2) else 3
	expr := parser.IfExpr{
		Condition: parser.FieldRef{Path: "a"},
		Then: parser.IfExpr{
			Condition: parser.FieldRef{Path: "b"},
			Then:      parser.LiteralInt{Value: 1},
			Else:      parser.LiteralInt{Value: 2},
		},
		Else: parser.LiteralInt{Value: 3},
	}

	tests := []struct {
		a, b bool
		want int
	}{
		{true, true, 1},
		{true, false, 2},
		{false, true, 3},
		{false, false, 3},
	}

	for _, tt := range tests {
		val, ok := Eval(expr, map[string]any{"a": tt.a, "b": tt.b})
		if !ok {
			t.Fatalf("eval failed for a=%v, b=%v", tt.a, tt.b)
		}
		if val != tt.want {
			t.Errorf("a=%v, b=%v: expected %d, got %v", tt.a, tt.b, tt.want, val)
		}
	}
}

func TestEvalIfExpr_NonBoolCondition(t *testing.T) {
	t.Parallel()

	expr := parser.IfExpr{
		Condition: parser.LiteralInt{Value: 1},
		Then:      parser.LiteralInt{Value: 42},
		Else:      parser.LiteralInt{Value: 0},
	}
	_, ok := Eval(expr, map[string]any{})
	if ok {
		t.Error("expected eval to fail for non-bool condition")
	}
}

func TestEvalIfExpr_StringBranches(t *testing.T) {
	t.Parallel()

	// if flag then "yes" else "no"
	expr := parser.IfExpr{
		Condition: parser.FieldRef{Path: "flag"},
		Then:      parser.LiteralString{Value: "yes"},
		Else:      parser.LiteralString{Value: "no"},
	}

	val, ok := Eval(expr, map[string]any{"flag": true})
	if !ok {
		t.Fatal("eval failed")
	}
	if val != "yes" {
		t.Errorf("expected yes, got %v", val)
	}

	val, ok = Eval(expr, map[string]any{"flag": false})
	if !ok {
		t.Fatal("eval failed")
	}
	if val != "no" {
		t.Errorf("expected no, got %v", val)
	}
}
