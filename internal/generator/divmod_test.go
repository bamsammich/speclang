package generator

import (
	"testing"

	"github.com/bamsammich/speclang/v3/internal/parser"
)

func TestIntDivision(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		l, r int
		want int
	}{
		{"10/3", 10, 3, 3},
		{"9/3", 9, 3, 3},
		{"7/2", 7, 2, 3},
		{"0/5", 0, 5, 0},
		{"100/10", 100, 10, 10},
		{"-7/2", -7, 2, -3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result, ok := evalBinaryValues("/", tt.l, tt.r)
			if !ok {
				t.Fatal("int division failed")
			}
			if result != tt.want {
				t.Errorf("%d / %d = %v, want %d", tt.l, tt.r, result, tt.want)
			}
		})
	}
}

func TestFloatDivision(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		l, r float64
		want float64
	}{
		{"10.0/3.0", 10.0, 3.0, 10.0 / 3.0},
		{"7.5/2.5", 7.5, 2.5, 3.0},
		{"1.0/4.0", 1.0, 4.0, 0.25},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result, ok := evalBinaryValues("/", tt.l, tt.r)
			if !ok {
				t.Fatal("float division failed")
			}
			if result != tt.want {
				t.Errorf("%v / %v = %v, want %v", tt.l, tt.r, result, tt.want)
			}
		})
	}
}

func TestMixedIntFloatDivision(t *testing.T) {
	t.Parallel()

	// int / float64 should use float path
	result, ok := evalBinaryValues("/", int(7), float64(2.0))
	if !ok {
		t.Fatal("mixed int/float division failed")
	}
	if result != float64(3.5) {
		t.Errorf("7 / 2.0 = %v, want 3.5", result)
	}

	// float64 / int should use float path
	result, ok = evalBinaryValues("/", float64(7.0), int(2))
	if !ok {
		t.Fatal("mixed float/int division failed")
	}
	if result != float64(3.5) {
		t.Errorf("7.0 / 2 = %v, want 3.5", result)
	}
}

func TestIntModulo(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		l, r int
		want int
	}{
		{"10%3", 10, 3, 1},
		{"9%3", 9, 3, 0},
		{"7%2", 7, 2, 1},
		{"0%5", 0, 5, 0},
		{"100%7", 100, 7, 2},
		{"-7%2", -7, 2, -1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result, ok := evalBinaryValues("%", tt.l, tt.r)
			if !ok {
				t.Fatal("int modulo failed")
			}
			if result != tt.want {
				t.Errorf("%d %% %d = %v, want %d", tt.l, tt.r, result, tt.want)
			}
		})
	}
}

func TestDivisionByZero_Int(t *testing.T) {
	t.Parallel()

	_, ok := evalBinaryValues("/", int(10), int(0))
	if ok {
		t.Error("int division by zero should fail")
	}
}

func TestDivisionByZero_Float(t *testing.T) {
	t.Parallel()

	_, ok := evalBinaryValues("/", float64(10.0), float64(0.0))
	if ok {
		t.Error("float division by zero should fail")
	}
}

func TestModuloByZero(t *testing.T) {
	t.Parallel()

	_, ok := evalBinaryValues("%", int(10), int(0))
	if ok {
		t.Error("modulo by zero should fail")
	}
}

func TestModuloFloat_Unsupported(t *testing.T) {
	t.Parallel()

	// Modulo is only defined for integers
	_, ok := evalBinaryValues("%", float64(10.0), float64(3.0))
	if ok {
		t.Error("float modulo should fail")
	}

	// Mixed int/float modulo should also fail
	_, ok = evalBinaryValues("%", int(10), float64(3.0))
	if ok {
		t.Error("mixed int/float modulo should fail")
	}
}

func TestEvalDivModExpressions(t *testing.T) {
	t.Parallel()

	// Test via the full Eval path (expression tree evaluation)
	vars := map[string]any{"a": 10, "b": 3}

	// a / b == 3
	result, ok := Eval(parser.BinaryOp{
		Left: parser.BinaryOp{
			Left:  parser.FieldRef{Path: "a"},
			Op:    "/",
			Right: parser.FieldRef{Path: "b"},
		},
		Op:    "==",
		Right: parser.LiteralInt{Value: 3},
	}, vars)
	if !ok {
		t.Fatal("eval a/b==3 failed")
	}
	if result != true {
		t.Errorf("10/3 == 3 should be true, got %v", result)
	}

	// a % b == 1
	result, ok = Eval(parser.BinaryOp{
		Left: parser.BinaryOp{
			Left:  parser.FieldRef{Path: "a"},
			Op:    "%",
			Right: parser.FieldRef{Path: "b"},
		},
		Op:    "==",
		Right: parser.LiteralInt{Value: 1},
	}, vars)
	if !ok {
		t.Fatal("eval a mod b == 1 failed")
	}
	if result != true {
		t.Errorf("10%%3 == 1 should be true, got %v", result)
	}
}
