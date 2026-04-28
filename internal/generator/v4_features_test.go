package generator

import (
	"testing"

	"github.com/bamsammich/speclang/v4/internal/parser"
)

// --- Eval: and, or, not, in, implies ---

func TestEval_And(t *testing.T) {
	t.Parallel()
	tests := []struct {
		l, r bool
		want bool
	}{
		{true, true, true},
		{true, false, false},
		{false, true, false},
		{false, false, false},
	}
	for _, tt := range tests {
		expr := parser.BinaryOp{
			Left: parser.LiteralBool{Value: tt.l}, Op: "and",
			Right: parser.LiteralBool{Value: tt.r},
		}
		val, ok := Eval(expr, nil)
		if !ok {
			t.Fatalf("and(%v, %v) failed", tt.l, tt.r)
		}
		if val != tt.want {
			t.Errorf("and(%v, %v) = %v, want %v", tt.l, tt.r, val, tt.want)
		}
	}
}

func TestEval_Or(t *testing.T) {
	t.Parallel()
	tests := []struct {
		l, r bool
		want bool
	}{
		{true, true, true},
		{true, false, true},
		{false, true, true},
		{false, false, false},
	}
	for _, tt := range tests {
		expr := parser.BinaryOp{
			Left: parser.LiteralBool{Value: tt.l}, Op: "or",
			Right: parser.LiteralBool{Value: tt.r},
		}
		val, ok := Eval(expr, nil)
		if !ok {
			t.Fatalf("or(%v, %v) failed", tt.l, tt.r)
		}
		if val != tt.want {
			t.Errorf("or(%v, %v) = %v, want %v", tt.l, tt.r, val, tt.want)
		}
	}
}

func TestEval_Not(t *testing.T) {
	t.Parallel()
	for _, b := range []bool{true, false} {
		expr := parser.UnaryOp{Op: "not", Operand: parser.LiteralBool{Value: b}}
		val, ok := Eval(expr, nil)
		if !ok {
			t.Fatalf("not(%v) failed", b)
		}
		if val != !b {
			t.Errorf("not(%v) = %v, want %v", b, val, !b)
		}
	}
}

func TestEval_In(t *testing.T) {
	t.Parallel()

	list := parser.ArrayLiteral{Elements: []parser.Expr{
		parser.LiteralString{Value: "a"},
		parser.LiteralString{Value: "b"},
		parser.LiteralString{Value: "c"},
	}}

	// "b" in ["a","b","c"] → true
	val, ok := Eval(parser.BinaryOp{
		Left: parser.LiteralString{Value: "b"}, Op: "in", Right: list,
	}, nil)
	if !ok {
		t.Fatal("eval failed")
	}
	if val != true {
		t.Error("expected true for 'b' in list")
	}

	// "z" in ["a","b","c"] → false
	val, ok = Eval(parser.BinaryOp{
		Left: parser.LiteralString{Value: "z"}, Op: "in", Right: list,
	}, nil)
	if !ok {
		t.Fatal("eval failed")
	}
	if val != false {
		t.Error("expected false for 'z' in list")
	}
}

func TestEval_In_Ints(t *testing.T) {
	t.Parallel()

	list := parser.ArrayLiteral{Elements: []parser.Expr{
		parser.LiteralInt{Value: 1},
		parser.LiteralInt{Value: 2},
		parser.LiteralInt{Value: 3},
	}}

	val, ok := Eval(parser.BinaryOp{
		Left: parser.LiteralInt{Value: 2}, Op: "in", Right: list,
	}, nil)
	if !ok {
		t.Fatal("eval failed")
	}
	if val != true {
		t.Error("expected true for 2 in [1,2,3]")
	}
}

func TestEval_Implies(t *testing.T) {
	t.Parallel()
	tests := []struct {
		l, r bool
		want bool
	}{
		{true, true, true},
		{true, false, false},
		{false, true, true},
		{false, false, true},
	}
	for _, tt := range tests {
		expr := parser.BinaryOp{
			Left: parser.LiteralBool{Value: tt.l}, Op: "implies",
			Right: parser.LiteralBool{Value: tt.r},
		}
		val, ok := Eval(expr, nil)
		if !ok {
			t.Fatalf("implies(%v, %v) failed", tt.l, tt.r)
		}
		if val != tt.want {
			t.Errorf("implies(%v, %v) = %v, want %v", tt.l, tt.r, val, tt.want)
		}
	}
}

// --- Named enum generation ---

func TestGenerateNamedEnum(t *testing.T) {
	t.Parallel()

	enums := []*parser.NamedEnum{
		{Name: "Status", Variants: []string{"pending", "active", "closed"}},
	}
	contract := &parser.Contract{
		Fields: []*parser.Field{
			{Name: "status", Type: parser.TypeExpr{Name: "Status"}},
		},
	}

	seen := make(map[string]bool)
	for seed := range uint64(100) {
		g := New(contract, nil, seed)
		g.SetEnums(enums)
		input, err := g.GenerateInput()
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		val, ok := input["status"].(string)
		if !ok {
			t.Fatalf("seed %d: status is %T, want string", seed, input["status"])
		}
		seen[val] = true
		// Must be a valid variant
		valid := false
		for _, v := range enums[0].Variants {
			if val == v {
				valid = true
				break
			}
		}
		if !valid {
			t.Errorf("seed %d: generated %q which is not a valid variant", seed, val)
		}
	}

	// With 100 seeds and 3 variants, we should see all of them.
	for _, v := range enums[0].Variants {
		if !seen[v] {
			t.Errorf("variant %q was never generated in 100 draws", v)
		}
	}
}

func TestGenerateNamedEnum_DoesNotConflictWithModel(t *testing.T) {
	t.Parallel()

	// A model and an enum with different names — field typed as enum resolves to enum.
	models := []*parser.Model{
		{Name: "Account", Fields: []*parser.Field{
			{Name: "id", Type: parser.TypeExpr{Name: "int"}},
		}},
	}
	enums := []*parser.NamedEnum{
		{Name: "Role", Variants: []string{"admin", "user"}},
	}
	contract := &parser.Contract{
		Fields: []*parser.Field{
			{Name: "account", Type: parser.TypeExpr{Name: "Account"}},
			{Name: "role", Type: parser.TypeExpr{Name: "Role"}},
		},
	}

	g := New(contract, models, 42)
	g.SetEnums(enums)
	input, err := g.GenerateInput()
	if err != nil {
		t.Fatal(err)
	}

	// account should be a map (model)
	if _, ok := input["account"].(map[string]any); !ok {
		t.Fatalf("account is %T, want map", input["account"])
	}
	// role should be a string (enum variant)
	role, ok := input["role"].(string)
	if !ok {
		t.Fatalf("role is %T, want string", input["role"])
	}
	if role != "admin" && role != "user" {
		t.Errorf("role is %q, want admin or user", role)
	}
}

// --- State-dependent fields (When) ---

func TestGenerateFields_WhenTrue(t *testing.T) {
	t.Parallel()

	contract := &parser.Contract{
		Fields: []*parser.Field{
			{Name: "active", Type: parser.TypeExpr{Name: "bool"}},
			{
				Name: "details",
				Type: parser.TypeExpr{Name: "string"},
				When: parser.LiteralBool{Value: true},
			},
		},
	}

	g := New(contract, nil, 42)
	input, err := g.GenerateInput()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := input["details"]; !ok {
		t.Error("field with When=true should be present")
	}
}

func TestGenerateFields_WhenFalse(t *testing.T) {
	t.Parallel()

	contract := &parser.Contract{
		Fields: []*parser.Field{
			{Name: "active", Type: parser.TypeExpr{Name: "bool"}},
			{
				Name: "details",
				Type: parser.TypeExpr{Name: "string"},
				When: parser.LiteralBool{Value: false},
			},
		},
	}

	g := New(contract, nil, 42)
	input, err := g.GenerateInput()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := input["details"]; ok {
		t.Error("field with When=false should be absent")
	}
}

func TestGenerateFields_WhenDependsOnOtherField(t *testing.T) {
	t.Parallel()

	// "premium" field only present when type == "premium" (simulated via bool)
	contract := &parser.Contract{
		Fields: []*parser.Field{
			{Name: "is_premium", Type: parser.TypeExpr{Name: "bool"}},
			{
				Name: "discount",
				Type: parser.TypeExpr{Name: "int"},
				When: parser.FieldRef{Path: "is_premium"},
			},
		},
	}

	sawPresent := false
	sawAbsent := false
	for seed := range uint64(100) {
		g := New(contract, nil, seed)
		input, err := g.GenerateInput()
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		isPremium := input["is_premium"].(bool)
		_, hasDiscount := input["discount"]
		if isPremium && hasDiscount {
			sawPresent = true
		}
		if !isPremium && !hasDiscount {
			sawAbsent = true
		}
		if !isPremium && hasDiscount {
			t.Fatalf("seed %d: discount present when is_premium=false", seed)
		}
	}
	if !sawPresent {
		t.Error("never saw discount present with is_premium=true in 100 seeds")
	}
	if !sawAbsent {
		t.Error("never saw discount absent with is_premium=false in 100 seeds")
	}
}

// --- config.key resolution ---

func TestEvalWithConfig(t *testing.T) {
	t.Parallel()

	config := map[string]parser.Expr{
		"max_amount": parser.LiteralInt{Value: 500},
		"label":      parser.LiteralString{Value: "test"},
	}

	// config.max_amount == 500
	val, ok := EvalWithConfig(
		parser.FieldRef{Path: "config.max_amount"},
		nil,
		config,
	)
	if !ok {
		t.Fatal("eval failed for config.max_amount")
	}
	if val != 500 {
		t.Errorf("config.max_amount = %v, want 500", val)
	}

	// config.label == "test"
	val, ok = EvalWithConfig(
		parser.FieldRef{Path: "config.label"},
		nil,
		config,
	)
	if !ok {
		t.Fatal("eval failed for config.label")
	}
	if val != "test" {
		t.Errorf("config.label = %v, want %q", val, "test")
	}
}

func TestEvalWithConfig_UnknownKey(t *testing.T) {
	t.Parallel()

	config := map[string]parser.Expr{
		"known": parser.LiteralInt{Value: 1},
	}

	_, ok := EvalWithConfig(
		parser.FieldRef{Path: "config.unknown"},
		nil,
		config,
	)
	if ok {
		t.Error("expected eval to fail for unknown config key")
	}
}

func TestEvalWithConfig_InConstraint(t *testing.T) {
	t.Parallel()

	// Constraint: amount <= config.max_amount
	config := map[string]parser.Expr{
		"max_amount": parser.LiteralInt{Value: 100},
	}
	expr := parser.BinaryOp{
		Left: parser.FieldRef{Path: "amount"}, Op: "<=",
		Right: parser.FieldRef{Path: "config.max_amount"},
	}

	// amount=50 <= 100 → true
	val, ok := EvalWithConfig(expr, map[string]any{"amount": 50}, config)
	if !ok {
		t.Fatal("eval failed")
	}
	if val != true {
		t.Error("expected true for 50 <= 100")
	}

	// amount=150 <= 100 → false
	val, ok = EvalWithConfig(expr, map[string]any{"amount": 150}, config)
	if !ok {
		t.Fatal("eval failed")
	}
	if val != false {
		t.Error("expected false for 150 <= 100")
	}
}

func TestGenerateInput_ConfigInConstraint(t *testing.T) {
	t.Parallel()

	config := map[string]parser.Expr{
		"max_val": parser.LiteralInt{Value: 10},
	}
	contract := &parser.Contract{
		Fields: []*parser.Field{
			{
				Name: "x",
				Type: parser.TypeExpr{Name: "int"},
				// x <= config.max_val (i.e., x <= 10)
				Constraint: parser.BinaryOp{
					Left: parser.FieldRef{Path: "x"}, Op: "<=",
					Right: parser.FieldRef{Path: "config.max_val"},
				},
			},
		},
	}

	for seed := range uint64(100) {
		g := New(contract, nil, seed)
		g.SetConfig(config)
		input, err := g.GenerateInput()
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		x := input["x"].(int)
		if x > 10 {
			t.Fatalf("seed %d: x=%d exceeds config.max_val=10", seed, x)
		}
	}
}
