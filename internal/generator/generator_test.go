package generator

import (
	"testing"

	"github.com/bamsammich/speclang/v3/internal/parser"
)

func TestEval_EnvRef_SetVar(t *testing.T) {
	t.Setenv("SPECTEST_EVAL_SET", "hello")
	val, ok := Eval(parser.EnvRef{Var: "SPECTEST_EVAL_SET"}, nil)
	if !ok {
		t.Fatal("Eval returned ok=false for EnvRef")
	}
	if val != "hello" {
		t.Errorf("got %q, want %q", val, "hello")
	}
}

func TestEval_EnvRef_UnsetWithDefault(t *testing.T) {
	t.Parallel()

	val, ok := Eval(parser.EnvRef{Var: "SPECTEST_EVAL_UNSET_12345", Default: "fallback"}, nil)
	if !ok {
		t.Fatal("Eval returned ok=false for EnvRef with default")
	}
	if val != "fallback" {
		t.Errorf("got %q, want %q", val, "fallback")
	}
}

func TestEval_EnvRef_UnsetNoDefault(t *testing.T) {
	t.Parallel()

	val, ok := Eval(parser.EnvRef{Var: "SPECTEST_EVAL_UNSET_12345"}, nil)
	if !ok {
		t.Fatal("Eval returned ok=false for EnvRef without default")
	}
	if val != "" {
		t.Errorf("got %q, want empty string", val)
	}
}

func TestEval_StringConcat(t *testing.T) {
	t.Parallel()
	tests := []struct {
		expr parser.Expr
		want any
		name string
	}{
		{
			name: "string + string",
			expr: parser.BinaryOp{
				Left: parser.LiteralString{Value: "hello"}, Op: "+",
				Right: parser.LiteralString{Value: " world"},
			},
			want: "hello world",
		},
		{
			name: "string + int auto-coerce",
			expr: parser.BinaryOp{
				Left: parser.LiteralString{Value: "count: "}, Op: "+",
				Right: parser.LiteralInt{Value: 42},
			},
			want: "count: 42",
		},
		{
			name: "int + string auto-coerce",
			expr: parser.BinaryOp{
				Left: parser.LiteralInt{Value: 42}, Op: "+",
				Right: parser.LiteralString{Value: " items"},
			},
			want: "42 items",
		},
		{
			name: "string + bool auto-coerce",
			expr: parser.BinaryOp{
				Left: parser.LiteralString{Value: "flag: "}, Op: "+",
				Right: parser.LiteralBool{Value: true},
			},
			want: "flag: true",
		},
		{
			name: "chained concat",
			expr: parser.BinaryOp{
				Left: parser.BinaryOp{
					Left: parser.LiteralString{Value: "a"}, Op: "+",
					Right: parser.LiteralString{Value: "b"},
				},
				Op:    "+",
				Right: parser.LiteralString{Value: "c"},
			},
			want: "abc",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result, ok := Eval(tt.expr, nil)
			if !ok {
				t.Fatal("Eval returned not-ok")
			}
			if result != tt.want {
				t.Fatalf("got %v (%T), want %v (%T)", result, result, tt.want, tt.want)
			}
		})
	}
}

func transferContract() (*parser.Contract, []*parser.Model) {
	models := []*parser.Model{
		{
			Name: "Account",
			Fields: []*parser.Field{
				{Name: "id", Type: parser.TypeExpr{Name: "string"}},
				{Name: "balance", Type: parser.TypeExpr{Name: "int"}},
			},
		},
	}
	contract := &parser.Contract{
		Input: []*parser.Field{
			{Name: "from", Type: parser.TypeExpr{Name: "Account"}},
			{Name: "to", Type: parser.TypeExpr{Name: "Account"}},
			{
				Name: "amount",
				Type: parser.TypeExpr{Name: "int"},
				// 0 < amount <= from.balance
				Constraint: parser.BinaryOp{
					Op: "<=",
					Left: parser.BinaryOp{
						Op:    "<",
						Left:  parser.LiteralInt{Value: 0},
						Right: parser.FieldRef{Path: "amount"},
					},
					Right: parser.FieldRef{Path: "from.balance"},
				},
			},
		},
	}
	return contract, models
}

func TestGenerateInput_SatisfiesConstraints(t *testing.T) {
	t.Parallel()

	contract, models := transferContract()

	for seed := range uint64(1000) {
		g := New(contract, models, seed)
		input, err := g.GenerateInput()
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}

		amount, ok := input["amount"].(int)
		if !ok {
			t.Fatalf("seed %d: amount is %T, want int", seed, input["amount"])
		}

		from, ok := input["from"].(map[string]any)
		if !ok {
			t.Fatalf("seed %d: from is %T, want map", seed, input["from"])
		}

		fromBalance, ok := from["balance"].(int)
		if !ok {
			t.Fatalf("seed %d: from.balance is %T, want int", seed, from["balance"])
		}

		if amount <= 0 {
			t.Fatalf("seed %d: amount=%d violates 0 < amount", seed, amount)
		}
		if amount > fromBalance {
			t.Fatalf("seed %d: amount=%d > from.balance=%d", seed, amount, fromBalance)
		}
	}
}

func TestGenerateInput_Reproducible(t *testing.T) {
	t.Parallel()

	contract, models := transferContract()

	g1 := New(contract, models, 42)
	g2 := New(contract, models, 42)

	input1, err := g1.GenerateInput()
	if err != nil {
		t.Fatal(err)
	}
	input2, err := g2.GenerateInput()
	if err != nil {
		t.Fatal(err)
	}

	// Same seed must produce identical amount.
	if input1["amount"] != input2["amount"] {
		t.Fatalf("same seed produced different amounts: %v vs %v",
			input1["amount"], input2["amount"])
	}
}

func TestGenerateInput_DifferentSeeds(t *testing.T) {
	t.Parallel()

	contract, models := transferContract()

	// With enough seeds, we should see variation.
	amounts := make(map[int]bool)
	for seed := range uint64(100) {
		g := New(contract, models, seed)
		input, err := g.GenerateInput()
		if err != nil {
			t.Fatal(err)
		}
		amounts[input["amount"].(int)] = true
	}

	if len(amounts) < 5 {
		t.Fatalf("expected diverse amounts across 100 seeds, got %d distinct values", len(amounts))
	}
}

func TestGenerateN(t *testing.T) {
	t.Parallel()

	contract, models := transferContract()
	g := New(contract, models, 99)

	inputs, err := g.GenerateN(50)
	if err != nil {
		t.Fatal(err)
	}
	if len(inputs) != 50 {
		t.Fatalf("got %d inputs, want 50", len(inputs))
	}
}

func TestGenerateInput_NoContract(t *testing.T) {
	t.Parallel()

	g := New(nil, nil, 0)
	_, err := g.GenerateInput()
	if err == nil {
		t.Fatal("expected error for nil contract")
	}
}

func TestGenerateInput_NoConstraints(t *testing.T) {
	t.Parallel()

	contract := &parser.Contract{
		Input: []*parser.Field{
			{Name: "name", Type: parser.TypeExpr{Name: "string"}},
			{Name: "count", Type: parser.TypeExpr{Name: "int"}},
			{Name: "flag", Type: parser.TypeExpr{Name: "bool"}},
		},
	}

	g := New(contract, nil, 7)
	input, err := g.GenerateInput()
	if err != nil {
		t.Fatal(err)
	}

	if _, ok := input["name"].(string); !ok {
		t.Fatalf("name is %T, want string", input["name"])
	}
	if _, ok := input["count"].(int); !ok {
		t.Fatalf("count is %T, want int", input["count"])
	}
	// bool could be either value, just check type
	if _, ok := input["flag"].(bool); !ok {
		t.Fatalf("flag is %T, want bool", input["flag"])
	}
}

func TestGenerateInput_BalancesNonNegative(t *testing.T) {
	t.Parallel()

	contract, models := transferContract()

	for seed := range uint64(1000) {
		g := New(contract, models, seed)
		input, err := g.GenerateInput()
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}

		from := input["from"].(map[string]any)
		to := input["to"].(map[string]any)
		fromBal := from["balance"].(int)
		toBal := to["balance"].(int)

		if fromBal < 0 {
			t.Fatalf("seed %d: from.balance=%d is negative", seed, fromBal)
		}
		if toBal < 0 {
			t.Fatalf("seed %d: to.balance=%d is negative", seed, toBal)
		}
	}
}

func TestGenerateInput_FieldsPresent(t *testing.T) {
	t.Parallel()

	contract, models := transferContract()
	g := New(contract, models, 1)

	input, err := g.GenerateInput()
	if err != nil {
		t.Fatal(err)
	}

	for _, key := range []string{"from", "to", "amount"} {
		if _, exists := input[key]; !exists {
			t.Fatalf("missing field %q in generated input", key)
		}
	}

	from := input["from"].(map[string]any)
	for _, key := range []string{"id", "balance"} {
		if _, exists := from[key]; !exists {
			t.Fatalf("missing field %q in from account", key)
		}
	}
}
