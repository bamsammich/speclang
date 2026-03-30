package v2parser_test

import (
	"testing"

	"github.com/bamsammich/speclang/v3/internal/v2parser"
)

func TestParseTransferSpec(t *testing.T) {
	t.Parallel()

	spec, err := v2parser.ParseFile("../../examples/transfer.spec")
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	t.Run("top-level structure", func(t *testing.T) {
		t.Parallel()

		if spec.Name != "AccountAPI" {
			t.Errorf("expected Name=AccountAPI, got %q", spec.Name)
		}
		if spec.Target == nil {
			t.Fatal("expected Target to be set")
		}
		if len(spec.Models) != 1 {
			t.Fatalf("expected 1 model, got %d", len(spec.Models))
		}
		if len(spec.Scopes) != 1 {
			t.Fatalf("expected 1 scope, got %d", len(spec.Scopes))
		}
	})

	t.Run("scope structure", func(t *testing.T) {
		t.Parallel()

		scope := spec.Scopes[0]
		if scope.Name != "transfer" {
			t.Errorf("expected scope name transfer, got %q", scope.Name)
		}
		if scope.Contract == nil {
			t.Fatal("expected Contract to be set")
		}
		if len(scope.Invariants) != 3 {
			t.Fatalf("expected 3 invariants, got %d", len(scope.Invariants))
		}
		if len(scope.Scenarios) != 3 {
			t.Fatalf("expected 3 scenarios, got %d", len(scope.Scenarios))
		}
	})

	t.Run("scope config", func(t *testing.T) {
		t.Parallel()

		scope := spec.Scopes[0]
		if scope.Config == nil {
			t.Fatal("expected config to be set")
		}
		pathExpr, ok := scope.Config["path"]
		if !ok {
			t.Fatal("expected path in config")
		}
		pathStr, ok := pathExpr.(v2parser.LiteralString)
		if !ok || pathStr.Value != "/api/v1/accounts/transfer" {
			t.Errorf("expected path=/api/v1/accounts/transfer, got %v", pathExpr)
		}
		methodExpr, ok := scope.Config["method"]
		if !ok {
			t.Fatal("expected method in config")
		}
		methodStr, ok := methodExpr.(v2parser.LiteralString)
		if !ok || methodStr.Value != "POST" {
			t.Errorf("expected method=POST, got %v", methodExpr)
		}
	})

	t.Run("target", func(t *testing.T) {
		t.Parallel()

		val, ok := spec.Target.Fields["base_url"]
		if !ok {
			t.Fatal("expected base_url in target")
		}
		svcRef, ok := val.(v2parser.ServiceRef)
		if !ok {
			t.Fatalf("expected ServiceRef, got %T", val)
		}
		if svcRef.Name != "app" {
			t.Errorf("expected Name=app, got %q", svcRef.Name)
		}

		if len(spec.Target.Services) != 1 {
			t.Fatalf("expected 1 service, got %d", len(spec.Target.Services))
		}
		svc := spec.Target.Services[0]
		if svc.Name != "app" {
			t.Errorf("expected service name=app, got %q", svc.Name)
		}
		if svc.Build != "./server" {
			t.Errorf("expected build=./server, got %q", svc.Build)
		}
		if svc.Port != 8080 {
			t.Errorf("expected port=8080, got %d", svc.Port)
		}
	})

	t.Run("model Account", func(t *testing.T) {
		t.Parallel()

		m := spec.Models[0]
		if m.Name != "Account" {
			t.Errorf("expected model name Account, got %q", m.Name)
		}
		if len(m.Fields) != 2 {
			t.Fatalf("expected 2 fields, got %d", len(m.Fields))
		}
		if m.Fields[0].Name != "id" || m.Fields[0].Type.Name != "string" {
			t.Errorf("expected field id:string, got %s:%s", m.Fields[0].Name, m.Fields[0].Type.Name)
		}
		if m.Fields[1].Name != "balance" || m.Fields[1].Type.Name != "int" {
			t.Errorf(
				"expected field balance:int, got %s:%s",
				m.Fields[1].Name,
				m.Fields[1].Type.Name,
			)
		}
	})

	t.Run("contract", func(t *testing.T) {
		t.Parallel()

		c := spec.Scopes[0].Contract
		if len(c.Input) != 3 {
			t.Fatalf("expected 3 input fields, got %d", len(c.Input))
		}
		if len(c.Output) != 3 {
			t.Fatalf("expected 3 output fields, got %d", len(c.Output))
		}

		// Check input field types.
		if c.Input[0].Name != "from" || c.Input[0].Type.Name != "Account" {
			t.Errorf("expected from:Account, got %s:%s", c.Input[0].Name, c.Input[0].Type.Name)
		}
		if c.Input[1].Name != "to" || c.Input[1].Type.Name != "Account" {
			t.Errorf("expected to:Account, got %s:%s", c.Input[1].Name, c.Input[1].Type.Name)
		}
		if c.Input[2].Name != "amount" || c.Input[2].Type.Name != "int" {
			t.Errorf("expected amount:int, got %s:%s", c.Input[2].Name, c.Input[2].Type.Name)
		}

		// amount has a constraint.
		if c.Input[2].Constraint == nil {
			t.Fatal("expected constraint on amount field")
		}

		// output error field is optional.
		errField := c.Output[2]
		if errField.Name != "error" {
			t.Errorf("expected field name 'error', got %q", errField.Name)
		}
		if !errField.Type.Optional {
			t.Error("expected error field to be optional (string?)")
		}
		if errField.Type.Name != "string" {
			t.Errorf("expected error type string, got %q", errField.Type.Name)
		}
	})

	t.Run("contract amount constraint", func(t *testing.T) {
		t.Parallel()

		// The constraint is: 0 < amount <= from.balance
		// Parsed as: BinaryOp{BinaryOp{0, "<", amount}, "<=", from.balance}
		c := spec.Scopes[0].Contract.Input[2].Constraint
		outer, ok := c.(v2parser.BinaryOp)
		if !ok {
			t.Fatalf("expected BinaryOp, got %T", c)
		}
		if outer.Op != "<=" {
			t.Errorf("expected outer op <=, got %q", outer.Op)
		}

		inner, ok := outer.Left.(v2parser.BinaryOp)
		if !ok {
			t.Fatalf("expected inner BinaryOp, got %T", outer.Left)
		}
		if inner.Op != "<" {
			t.Errorf("expected inner op <, got %q", inner.Op)
		}

		zero, ok := inner.Left.(v2parser.LiteralInt)
		if !ok || zero.Value != 0 {
			t.Errorf("expected LiteralInt{0}, got %v", inner.Left)
		}

		amountRef, ok := inner.Right.(v2parser.FieldRef)
		if !ok || amountRef.Path != "amount" {
			t.Errorf("expected FieldRef{amount}, got %v", inner.Right)
		}

		balRef, ok := outer.Right.(v2parser.FieldRef)
		if !ok || balRef.Path != "from.balance" {
			t.Errorf("expected FieldRef{from.balance}, got %v", outer.Right)
		}
	})

	t.Run("invariant conservation", func(t *testing.T) {
		t.Parallel()

		inv := spec.Scopes[0].Invariants[0]
		if inv.Name != "conservation" {
			t.Errorf("expected name conservation, got %q", inv.Name)
		}

		// Has a "when" guard: error == null
		if inv.When == nil {
			t.Fatal("expected when guard")
		}
		guard, ok := inv.When.(v2parser.BinaryOp)
		if !ok {
			t.Fatalf("expected BinaryOp guard, got %T", inv.When)
		}
		if guard.Op != "==" {
			t.Errorf("expected == guard, got %q", guard.Op)
		}
		ref, ok := guard.Left.(v2parser.FieldRef)
		if !ok || ref.Path != "error" {
			t.Errorf("expected FieldRef{error}, got %v", guard.Left)
		}
		if _, ok := guard.Right.(v2parser.LiteralNull); !ok {
			t.Errorf("expected LiteralNull, got %T", guard.Right)
		}

		// Body: one assertion expression
		// output.from.balance + output.to.balance == input.from.balance + input.to.balance
		if len(inv.Assertions) != 1 {
			t.Fatalf("expected 1 assertion, got %d", len(inv.Assertions))
		}
		a := inv.Assertions[0]
		if a.Expr == nil {
			t.Fatal("expected Expr assertion")
		}
		eq, ok := a.Expr.(v2parser.BinaryOp)
		if !ok {
			t.Fatalf("expected BinaryOp, got %T", a.Expr)
		}
		if eq.Op != "==" {
			t.Errorf("expected == op, got %q", eq.Op)
		}
		// Left: output.from.balance + output.to.balance
		lhs, ok := eq.Left.(v2parser.BinaryOp)
		if !ok || lhs.Op != "+" {
			t.Errorf("expected + on lhs, got %v", eq.Left)
		}
	})

	t.Run("invariant non_negative", func(t *testing.T) {
		t.Parallel()

		inv := spec.Scopes[0].Invariants[1]
		if inv.Name != "non_negative" {
			t.Errorf("expected name non_negative, got %q", inv.Name)
		}
		if inv.When != nil {
			t.Errorf("expected no when guard, got %v", inv.When)
		}
		if len(inv.Assertions) != 2 {
			t.Fatalf("expected 2 assertions, got %d", len(inv.Assertions))
		}

		// First: output.from.balance >= 0
		a0, ok := inv.Assertions[0].Expr.(v2parser.BinaryOp)
		if !ok {
			t.Fatalf("expected BinaryOp, got %T", inv.Assertions[0].Expr)
		}
		if a0.Op != ">=" {
			t.Errorf("expected >=, got %q", a0.Op)
		}
		lhs, ok := a0.Left.(v2parser.FieldRef)
		if !ok || lhs.Path != "output.from.balance" {
			t.Errorf("expected output.from.balance, got %v", a0.Left)
		}
	})

	t.Run("invariant no_mutation_on_error", func(t *testing.T) {
		t.Parallel()

		inv := spec.Scopes[0].Invariants[2]
		if inv.Name != "no_mutation_on_error" {
			t.Errorf("expected name no_mutation_on_error, got %q", inv.Name)
		}
		if inv.When == nil {
			t.Fatal("expected when guard")
		}
		if len(inv.Assertions) != 2 {
			t.Fatalf("expected 2 assertions, got %d", len(inv.Assertions))
		}
	})

	t.Run("scenario success (given/then)", func(t *testing.T) {
		t.Parallel()

		sc := spec.Scopes[0].Scenarios[0]
		if sc.Name != "success" {
			t.Errorf("expected name success, got %q", sc.Name)
		}
		if sc.Given == nil {
			t.Fatal("expected given block")
		}
		if sc.When != nil {
			t.Error("expected no when block in success scenario")
		}
		if sc.Then == nil {
			t.Fatal("expected then block")
		}

		// Given block: 3 steps (all assignments)
		if len(sc.Given.Steps) != 3 {
			t.Fatalf("expected 3 given steps, got %d", len(sc.Given.Steps))
		}

		// from: { id: "alice", balance: 100 }
		fromAssign := sc.Given.Steps[0].(*v2parser.Assignment)
		if fromAssign.Path != "from" {
			t.Errorf("expected path 'from', got %q", fromAssign.Path)
		}
		fromObj, ok := fromAssign.Value.(v2parser.ObjectLiteral)
		if !ok {
			t.Fatalf("expected ObjectLiteral, got %T", fromAssign.Value)
		}
		if len(fromObj.Fields) != 2 {
			t.Fatalf("expected 2 fields in from object, got %d", len(fromObj.Fields))
		}
		if fromObj.Fields[0].Key != "id" {
			t.Errorf("expected key 'id', got %q", fromObj.Fields[0].Key)
		}
		idVal, ok := fromObj.Fields[0].Value.(v2parser.LiteralString)
		if !ok || idVal.Value != "alice" {
			t.Errorf("expected LiteralString{alice}, got %v", fromObj.Fields[0].Value)
		}

		// amount: 30
		amountAssign := sc.Given.Steps[2].(*v2parser.Assignment)
		if amountAssign.Path != "amount" {
			t.Errorf("expected path 'amount', got %q", amountAssign.Path)
		}
		amountVal, ok := amountAssign.Value.(v2parser.LiteralInt)
		if !ok || amountVal.Value != 30 {
			t.Errorf("expected LiteralInt{30}, got %v", amountAssign.Value)
		}

		// Then block: 3 assertions
		if len(sc.Then.Assertions) != 3 {
			t.Fatalf("expected 3 then assertions, got %d", len(sc.Then.Assertions))
		}
		if sc.Then.Assertions[0].Target != "from.balance" {
			t.Errorf("expected target 'from.balance', got %q", sc.Then.Assertions[0].Target)
		}
		binOp, ok := sc.Then.Assertions[0].Expected.(v2parser.BinaryOp)
		if !ok || binOp.Op != "-" {
			t.Errorf("expected BinaryOp{-}, got %v", sc.Then.Assertions[0].Expected)
		}
		// error: null
		if sc.Then.Assertions[2].Target != "error" {
			t.Errorf("expected target 'error', got %q", sc.Then.Assertions[2].Target)
		}
		if _, ok := sc.Then.Assertions[2].Expected.(v2parser.LiteralNull); !ok {
			t.Errorf("expected LiteralNull, got %T", sc.Then.Assertions[2].Expected)
		}
	})

	t.Run("scenario overdraft (when/then)", func(t *testing.T) {
		t.Parallel()

		sc := spec.Scopes[0].Scenarios[1]
		if sc.Name != "overdraft" {
			t.Errorf("expected name overdraft, got %q", sc.Name)
		}
		if sc.Given != nil {
			t.Error("expected no given block")
		}
		if sc.When == nil {
			t.Fatal("expected when block")
		}
		if sc.Then == nil {
			t.Fatal("expected then block")
		}

		// when { amount > from.balance }
		if len(sc.When.Predicates) != 1 {
			t.Fatalf("expected 1 predicate, got %d", len(sc.When.Predicates))
		}
		pred, ok := sc.When.Predicates[0].(v2parser.BinaryOp)
		if !ok {
			t.Fatalf("expected BinaryOp predicate, got %T", sc.When.Predicates[0])
		}
		if pred.Op != ">" {
			t.Errorf("expected > op, got %q", pred.Op)
		}

		// then { error: "insufficient_funds" }
		if len(sc.Then.Assertions) != 1 {
			t.Fatalf("expected 1 assertion, got %d", len(sc.Then.Assertions))
		}
		a := sc.Then.Assertions[0]
		if a.Target != "error" {
			t.Errorf("expected target 'error', got %q", a.Target)
		}
		strVal, ok := a.Expected.(v2parser.LiteralString)
		if !ok || strVal.Value != "insufficient_funds" {
			t.Errorf("expected LiteralString{insufficient_funds}, got %v", a.Expected)
		}
	})

	t.Run("scenario zero_transfer", func(t *testing.T) {
		t.Parallel()

		sc := spec.Scopes[0].Scenarios[2]
		if sc.Name != "zero_transfer" {
			t.Errorf("expected name zero_transfer, got %q", sc.Name)
		}

		// when { amount == 0 }
		if len(sc.When.Predicates) != 1 {
			t.Fatalf("expected 1 predicate, got %d", len(sc.When.Predicates))
		}
		pred, ok := sc.When.Predicates[0].(v2parser.BinaryOp)
		if !ok || pred.Op != "==" {
			t.Errorf("expected == predicate, got %v", sc.When.Predicates[0])
		}

		// then { error: "invalid_amount" }
		if len(sc.Then.Assertions) != 1 {
			t.Fatalf("expected 1 assertion, got %d", len(sc.Then.Assertions))
		}
		strVal, ok := sc.Then.Assertions[0].Expected.(v2parser.LiteralString)
		if !ok || strVal.Value != "invalid_amount" {
			t.Errorf(
				"expected LiteralString{invalid_amount}, got %v",
				sc.Then.Assertions[0].Expected,
			)
		}
	})
}

func TestParseExprPrecedence(t *testing.T) {
	t.Parallel()

	tests := []struct {
		checkFn func(*testing.T, v2parser.Expr)
		name    string
		input   string
	}{
		{
			name:  "addition before equality",
			input: "spec T { scope s { use http invariant i { a + b == c } } }",
			checkFn: func(t *testing.T, expr v2parser.Expr) {
				t.Helper()
				eq, ok := expr.(v2parser.BinaryOp)
				if !ok || eq.Op != "==" {
					t.Fatalf("expected ==, got %v", expr)
				}
				plus, ok := eq.Left.(v2parser.BinaryOp)
				if !ok || plus.Op != "+" {
					t.Errorf("expected + on left of ==, got %v", eq.Left)
				}
			},
		},
		{
			name:  "and before or",
			input: "spec T { scope s { use http invariant i { a || b && c } } }",
			checkFn: func(t *testing.T, expr v2parser.Expr) {
				t.Helper()
				or, ok := expr.(v2parser.BinaryOp)
				if !ok || or.Op != "||" {
					t.Fatalf("expected ||, got %v", expr)
				}
				and, ok := or.Right.(v2parser.BinaryOp)
				if !ok || and.Op != "&&" {
					t.Errorf("expected && on right of ||, got %v", or.Right)
				}
			},
		},
		{
			name:  "division at multiplicative precedence",
			input: "spec T { scope s { use http invariant i { a + b / c } } }",
			checkFn: func(t *testing.T, expr v2parser.Expr) {
				t.Helper()
				plus, ok := expr.(v2parser.BinaryOp)
				if !ok || plus.Op != "+" {
					t.Fatalf("expected +, got %v", expr)
				}
				div, ok := plus.Right.(v2parser.BinaryOp)
				if !ok || div.Op != "/" {
					t.Errorf("expected / on right of +, got %v", plus.Right)
				}
			},
		},
		{
			name:  "modulo at multiplicative precedence",
			input: "spec T { scope s { use http invariant i { a + b % c } } }",
			checkFn: func(t *testing.T, expr v2parser.Expr) {
				t.Helper()
				plus, ok := expr.(v2parser.BinaryOp)
				if !ok || plus.Op != "+" {
					t.Fatalf("expected +, got %v", expr)
				}
				mod, ok := plus.Right.(v2parser.BinaryOp)
				if !ok || mod.Op != "%" {
					t.Errorf("expected %% on right of +, got %v", plus.Right)
				}
			},
		},
		{
			name:  "unary negation",
			input: "spec T { scope s { use http invariant i { !a } } }",
			checkFn: func(t *testing.T, expr v2parser.Expr) {
				t.Helper()
				unary, ok := expr.(v2parser.UnaryOp)
				if !ok || unary.Op != "!" {
					t.Fatalf("expected UnaryOp{!}, got %v", expr)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			spec, err := v2parser.Parse(tc.input)
			if err != nil {
				t.Fatalf("parse error: %v", err)
			}
			if len(spec.Scopes) != 1 ||
				len(spec.Scopes[0].Invariants) != 1 ||
				len(spec.Scopes[0].Invariants[0].Assertions) != 1 {
				t.Fatal("expected 1 scope with 1 invariant with 1 assertion")
			}
			tc.checkFn(t, spec.Scopes[0].Invariants[0].Assertions[0].Expr)
		})
	}
}

func TestParse_Description(t *testing.T) {
	t.Parallel()

	spec, err := v2parser.Parse(`spec Foo {
  description: "A test specification"
}`)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if spec.Description != "A test specification" {
		t.Errorf("expected description %q, got %q", "A test specification", spec.Description)
	}
}

func TestParse_DescriptionOptional(t *testing.T) {
	t.Parallel()

	spec, err := v2parser.Parse(`spec Foo {}`)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if spec.Description != "" {
		t.Errorf("expected empty description, got %q", spec.Description)
	}
}

func TestParseErrorCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
	}{
		{"missing spec name", "spec {"},
		{"missing opening brace", "spec T model {}"},
		{"unexpected token in spec body", "spec T { 123 }"},
		{"unterminated spec", "spec T {"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := v2parser.Parse(tc.input)
			if err == nil {
				t.Error("expected parse error, got nil")
			}
		})
	}
}
