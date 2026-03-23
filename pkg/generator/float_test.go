package generator

import (
	"math"
	"math/rand/v2"
	"testing"

	"github.com/bamsammich/speclang/v2/pkg/parser"
)

func TestGenerateFloat(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 0))
	for range 100 {
		v := generateFloat(rng)
		if math.IsNaN(v) {
			t.Fatal("generateFloat produced NaN")
		}
		if math.IsInf(v, 0) {
			t.Fatal("generateFloat produced Inf")
		}
	}
}

func TestGenerateValue_Float(t *testing.T) {
	g := New(
		&parser.Contract{
			Input: []*parser.Field{
				{Name: "price", Type: parser.TypeExpr{Name: "float"}},
			},
		},
		nil, 42,
	)
	input, err := g.GenerateInput()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := input["price"].(float64); !ok {
		t.Errorf("expected float64 for price, got %T", input["price"])
	}
}

func TestFloatConstraint(t *testing.T) {
	// amount > 0.5
	constraint := parser.BinaryOp{
		Left:  parser.FieldRef{Path: "amount"},
		Op:    ">",
		Right: parser.LiteralFloat{Value: 0.5},
	}
	g := New(
		&parser.Contract{
			Input: []*parser.Field{
				{Name: "amount", Type: parser.TypeExpr{Name: "float"}, Constraint: constraint},
			},
		},
		nil, 42,
	)
	input, err := g.GenerateInput()
	if err != nil {
		t.Fatal(err)
	}
	v, ok := input["amount"].(float64)
	if !ok {
		t.Fatalf("expected float64, got %T", input["amount"])
	}
	if v <= 0.5 {
		t.Errorf("constraint amount > 0.5 violated: got %v", v)
	}
}

func TestToFloat(t *testing.T) {
	tests := []struct {
		input any
		want  float64
		ok    bool
	}{
		{float64(3.14), 3.14, true},
		{int(42), 42.0, true},
		{float64(0), 0.0, true},
		{"hello", 0, false},
		{true, 0, false},
	}
	for _, tt := range tests {
		got, ok := toFloat(tt.input)
		if ok != tt.ok {
			t.Errorf("toFloat(%v): ok = %v, want %v", tt.input, ok, tt.ok)
		}
		if ok && got != tt.want {
			t.Errorf("toFloat(%v) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestToInt_NoTruncation(t *testing.T) {
	// toInt should not truncate 3.7 to 3
	_, ok := toInt(float64(3.7))
	if ok {
		t.Error("toInt(3.7) should return false, not truncate")
	}

	// But exact integers as float64 should work (JSON round-trip)
	v, ok := toInt(float64(42.0))
	if !ok {
		t.Error("toInt(42.0) should succeed")
	}
	if v != 42 {
		t.Errorf("toInt(42.0) = %d, want 42", v)
	}
}

func TestMixedIntFloatArithmetic(t *testing.T) {
	// 3.0 + 0.5 should produce 3.5, not truncate
	result, ok := evalBinaryValues("+", float64(3.0), float64(0.5))
	if !ok {
		t.Fatal("float addition failed")
	}
	if result != float64(3.5) {
		t.Errorf("3.0 + 0.5 = %v, want 3.5", result)
	}

	// int + float64 should use float path
	result, ok = evalBinaryValues("+", int(3), float64(0.5))
	if !ok {
		t.Fatal("mixed int+float addition failed")
	}
	if result != float64(3.5) {
		t.Errorf("3 + 0.5 = %v, want 3.5", result)
	}

	// int + int should stay int
	result, ok = evalBinaryValues("+", int(3), int(4))
	if !ok {
		t.Fatal("int addition failed")
	}
	if result != int(7) {
		t.Errorf("3 + 4 = %v, want 7", result)
	}
}

func TestFloatComparison(t *testing.T) {
	result, ok := evalBinaryValues("<", float64(0.5), float64(1.0))
	if !ok || result != true {
		t.Errorf("0.5 < 1.0 = %v, want true", result)
	}

	result, ok = evalBinaryValues(">=", float64(1.0), float64(1.0))
	if !ok || result != true {
		t.Errorf("1.0 >= 1.0 = %v, want true", result)
	}
}

func TestEvalLiteralFloat(t *testing.T) {
	ctx := &evalCtx{input: map[string]any{}}
	val, ok := ctx.eval(parser.LiteralFloat{Value: 3.14})
	if !ok {
		t.Fatal("eval LiteralFloat failed")
	}
	if val != float64(3.14) {
		t.Errorf("eval LiteralFloat = %v, want 3.14", val)
	}
}

func TestEvalUnaryNegativeFloat(t *testing.T) {
	ctx := &evalCtx{input: map[string]any{}}
	val, ok := ctx.eval(parser.UnaryOp{Op: "-", Operand: parser.LiteralFloat{Value: 3.14}})
	if !ok {
		t.Fatal("eval unary -float failed")
	}
	if val != float64(-3.14) {
		t.Errorf("eval -3.14 = %v, want -3.14", val)
	}
}
