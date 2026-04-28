package generator

import (
	"testing"

	"github.com/bamsammich/speclang/v4/internal/parser"
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
		Fields: []*parser.Field{
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
		Fields: []*parser.Field{
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

// --- GAP A: config.key resolution in generator (end-to-end via parse) ---

// TestGeneratorConfigResolution_EnumConstraint verifies that config.key expressions
// in field constraints are resolved when SetConfig is called, specifically testing
// with a non-trivial constraint involving config.max_transfer.
func TestGeneratorConfigResolution_EnumConstraint(t *testing.T) {
	t.Parallel()
	config := map[string]parser.Expr{
		"max_transfer": parser.LiteralInt{Value: 500},
	}

	contract := &parser.Contract{
		Name: "Transfer",
		Fields: []*parser.Field{
			{
				Name:       "amount",
				Type:       parser.TypeExpr{Name: "int"},
				Constraint: parser.BinaryOp{Left: parser.FieldRef{Path: "amount"}, Op: "<=", Right: parser.FieldRef{Path: "config.max_transfer"}},
			},
		},
		ReturnType: parser.TypeExpr{Name: "string"},
	}

	g := New(contract, nil, 77)
	g.SetConfig(config)

	for range 50 {
		input, err := g.GenerateInput()
		if err != nil {
			t.Fatalf("GenerateInput: %v", err)
		}
		amount, ok := input["amount"].(int)
		if !ok {
			t.Fatalf("amount is not int: %T", input["amount"])
		}
		if amount > 500 {
			t.Errorf("amount %d exceeds config.max_transfer (500)", amount)
		}
	}
}

// TestGenerateInput_CircularWhenReturnsError verifies that a contract with
// circular when dependencies (a when b>0, b when a>0) causes GenerateInput
// to return an error naming the unresolved fields rather than silently
// producing incomplete inputs.
func TestGenerateInput_CircularWhenReturnsError(t *testing.T) {
	t.Parallel()
	contract := &parser.Contract{
		Name: "circular",
		Fields: []*parser.Field{
			{
				Name: "a",
				Type: parser.TypeExpr{Name: "int", Optional: true},
				When: parser.BinaryOp{
					Left:  parser.FieldRef{Path: "b"},
					Op:    ">",
					Right: parser.LiteralInt{Value: 0},
				},
			},
			{
				Name: "b",
				Type: parser.TypeExpr{Name: "int", Optional: true},
				When: parser.BinaryOp{
					Left:  parser.FieldRef{Path: "a"},
					Op:    ">",
					Right: parser.LiteralInt{Value: 0},
				},
			},
		},
	}
	g := New(contract, nil, 1)
	_, err := g.GenerateInput()
	if err == nil {
		t.Fatal("expected error for circular when dependency, got nil")
	}
	// Error must name at least one of the involved fields.
	if !contains(err.Error(), "a") && !contains(err.Error(), "b") {
		t.Errorf("expected error to name involved fields, got: %v", err)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 ||
		func() bool {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
			return false
		}())
}

// TestGenerateInput_NonCircularWhenStillWorks verifies that a valid when chain
// (tracking present only when status=="shipped") generates correctly even after
// the circular-when detection was added.
func TestGenerateInput_NonCircularWhenStillWorks(t *testing.T) {
	t.Parallel()
	contract := &parser.Contract{
		Name: "noncircular",
		Fields: []*parser.Field{
			{Name: "status", Type: parser.TypeExpr{Name: "enum", Variants: []string{"shipped", "pending"}}},
			{
				Name: "tracking",
				Type: parser.TypeExpr{Name: "string"},
				When: parser.BinaryOp{
					Left:  parser.FieldRef{Path: "status"},
					Op:    "==",
					Right: parser.LiteralString{Value: "shipped"},
				},
			},
		},
	}
	g := New(contract, nil, 7)
	for range 30 {
		input, err := g.GenerateInput()
		if err != nil {
			t.Fatalf("GenerateInput returned error for non-circular when: %v", err)
		}
		_ = input
	}
}

// TestGenerateFields_NonCyclicThreeFieldChain verifies that a three-field non-cyclic
// dependency chain (a: plain, b: when a > 0, c: when b == 0) generates correctly
// across multiple seeds without error.
func TestGenerateFields_NonCyclicThreeFieldChain(t *testing.T) {
	t.Parallel()
	// a has no When — always generated.
	// b depends on a (no cycle).
	// c depends on b (no cycle).
	contract := &parser.Contract{
		Name: "chain",
		Fields: []*parser.Field{
			{
				Name: "a",
				Type: parser.TypeExpr{Name: "int"},
				// No When.
			},
			{
				Name: "b",
				Type: parser.TypeExpr{Name: "int", Optional: true},
				When: parser.BinaryOp{
					Left:  parser.FieldRef{Path: "a"},
					Op:    ">",
					Right: parser.LiteralInt{Value: 0},
				},
			},
			{
				Name: "c",
				Type: parser.TypeExpr{Name: "int", Optional: true},
				When: parser.BinaryOp{
					Left:  parser.FieldRef{Path: "b"},
					Op:    "==",
					Right: parser.LiteralInt{Value: 0},
				},
			},
		},
		ReturnType: parser.TypeExpr{Name: "string"},
	}

	seeds := []uint64{1, 2, 3, 42, 99, 100, 777, 12345}
	for _, seed := range seeds {
		g := New(contract, nil, seed)
		for range 10 {
			input, err := g.GenerateInput()
			if err != nil {
				t.Errorf("seed=%d: GenerateInput returned error for non-cyclic 3-field chain: %v", seed, err)
				continue
			}
			// "a" must always be present.
			if _, ok := input["a"]; !ok {
				t.Errorf("seed=%d: 'a' (unconditional) must always be present, got input=%v", seed, input)
			}
		}
	}
}

// --- GAP B: State-dependent fields via real status-equality When condition ---

// TestGenerateFields_WhenStatusShipped verifies tracking is present exactly when
// status=="shipped". Uses enum-constrained variants to force deterministic outcomes.
func TestGenerateFields_WhenStatusShipped(t *testing.T) {
	t.Parallel()
	whenExpr := parser.BinaryOp{
		Left:  parser.FieldRef{Path: "status"},
		Op:    "==",
		Right: parser.LiteralString{Value: "shipped"},
	}

	t.Run("shipped always has tracking", func(t *testing.T) {
		t.Parallel()
		contract := &parser.Contract{
			Name: "Shipment",
			Fields: []*parser.Field{
				{Name: "status", Type: parser.TypeExpr{Name: "enum", Variants: []string{"shipped"}}},
				{Name: "tracking", Type: parser.TypeExpr{Name: "string"}, When: whenExpr},
			},
			ReturnType: parser.TypeExpr{Name: "string"},
		}
		g := New(contract, nil, 99)
		for range 20 {
			input, err := g.GenerateInput()
			if err != nil {
				t.Fatalf("GenerateInput: %v", err)
			}
			if _, ok := input["tracking"]; !ok {
				t.Errorf("tracking must be present when status=shipped, got input=%v", input)
			}
		}
	})

	t.Run("pending never has tracking", func(t *testing.T) {
		t.Parallel()
		contract := &parser.Contract{
			Name: "Shipment",
			Fields: []*parser.Field{
				{Name: "status", Type: parser.TypeExpr{Name: "enum", Variants: []string{"pending"}}},
				{Name: "tracking", Type: parser.TypeExpr{Name: "string"}, When: whenExpr},
			},
			ReturnType: parser.TypeExpr{Name: "string"},
		}
		g := New(contract, nil, 42)
		for range 20 {
			input, err := g.GenerateInput()
			if err != nil {
				t.Fatalf("GenerateInput: %v", err)
			}
			if _, ok := input["tracking"]; ok {
				t.Errorf("tracking must be absent when status=pending, got input=%v", input)
			}
		}
	})
}
