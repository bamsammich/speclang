package parser_test

import (
	"strings"
	"testing"

	"github.com/bamsammich/speclang/v3/internal/parser"
)

// --- v3 spec-level structure ---

func TestParseV3_AdapterConfigBlocks(t *testing.T) {
	t.Parallel()
	spec, err := parser.Parse(`
spec App {
  http {
    base_url: env(APP_URL, "http://localhost:8080")
  }
  playwright {
    base_url: env(APP_URL, "http://localhost:8080")
    headless: true
  }
}`)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	if len(spec.AdapterConfigs) != 2 {
		t.Fatalf("expected 2 adapter configs, got %d", len(spec.AdapterConfigs))
	}

	httpConf, ok := spec.AdapterConfigs["http"]
	if !ok {
		t.Fatal("expected http adapter config")
	}
	if _, ok := httpConf["base_url"]; !ok {
		t.Error("expected base_url in http config")
	}

	pwConf, ok := spec.AdapterConfigs["playwright"]
	if !ok {
		t.Fatal("expected playwright adapter config")
	}
	if _, ok := pwConf["base_url"]; !ok {
		t.Error("expected base_url in playwright config")
	}
	headless, ok := pwConf["headless"]
	if !ok {
		t.Fatal("expected headless in playwright config")
	}
	if b, ok := headless.(parser.LiteralBool); !ok || !b.Value {
		t.Errorf("expected headless=true, got %v", headless)
	}
}

func TestParseV3_SpecLevelServices(t *testing.T) {
	t.Parallel()
	spec, err := parser.Parse(`
spec App {
  services {
    app {
      build: "./server"
      port: 8080
    }
  }
}`)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if len(spec.Services) != 1 {
		t.Fatalf("expected 1 service, got %d", len(spec.Services))
	}
	svc := spec.Services[0]
	if svc.Name != "app" {
		t.Errorf("expected service name app, got %q", svc.Name)
	}
	if svc.Build != "./server" {
		t.Errorf("expected build ./server, got %q", svc.Build)
	}
	if svc.Port != 8080 {
		t.Errorf("expected port 8080, got %d", svc.Port)
	}
}

func TestParseV3_ActionDef(t *testing.T) {
	t.Parallel()
	spec, err := parser.Parse(`
spec App {
  http {
    base_url: "http://localhost:8080"
  }
  action login(username: string, password: string) {
    let result = http.post("/api/auth/login", { username: username, password: password })
    http.header("Authorization", "Bearer " + result.body.access_token)
    return result.body
  }
}`)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if len(spec.Actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(spec.Actions))
	}
	a := spec.Actions[0]
	if a.Name != "login" {
		t.Errorf("expected action name login, got %q", a.Name)
	}
	if len(a.Params) != 2 {
		t.Fatalf("expected 2 params, got %d", len(a.Params))
	}
	if a.Params[0].Name != "username" || a.Params[0].Type.Name != "string" {
		t.Errorf("expected param username:string, got %s:%s", a.Params[0].Name, a.Params[0].Type.Name)
	}
	if len(a.Body) != 3 {
		t.Fatalf("expected 3 body steps, got %d", len(a.Body))
	}

	// Step 0: let binding
	letStep, ok := a.Body[0].(*parser.LetBinding)
	if !ok {
		t.Fatalf("expected LetBinding, got %T", a.Body[0])
	}
	if letStep.Name != "result" {
		t.Errorf("expected let name result, got %q", letStep.Name)
	}

	// Step 1: adapter call
	callStep, ok := a.Body[1].(*parser.AdapterCall)
	if !ok {
		t.Fatalf("expected AdapterCall, got %T", a.Body[1])
	}
	if callStep.Adapter != "http" || callStep.Method != "header" {
		t.Errorf("expected http.header, got %s.%s", callStep.Adapter, callStep.Method)
	}

	// Step 2: return
	retStep, ok := a.Body[2].(*parser.ReturnStmt)
	if !ok {
		t.Fatalf("expected ReturnStmt, got %T", a.Body[2])
	}
	if retStep.Value == nil {
		t.Error("expected return value")
	}
}

func TestParseV3_ScopeLevelAction(t *testing.T) {
	t.Parallel()
	spec, err := parser.Parse(`
spec App {
  scope transfer {
    action transfer(from: string, to: string, amount: int) {
      let result = http.post("/api/transfer", { from: from, to: to, amount: amount })
      return result.body
    }
    contract {
      input { from: string, to: string, amount: int }
      output { error: string? }
      action: transfer
    }
  }
}`)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	scope := spec.Scopes[0]
	if len(scope.Actions) != 1 {
		t.Fatalf("expected 1 scope action, got %d", len(scope.Actions))
	}
	if scope.Actions[0].Name != "transfer" {
		t.Errorf("expected action name transfer, got %q", scope.Actions[0].Name)
	}
}

func TestParseV3_ContractAction(t *testing.T) {
	t.Parallel()
	spec, err := parser.Parse(`
spec App {
  scope test {
    contract {
      input { x: int }
      output { y: int }
      action: do_something
    }
  }
}`)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	c := spec.Scopes[0].Contract
	if c.Action != "do_something" {
		t.Errorf("expected contract action do_something, got %q", c.Action)
	}
}

func TestParseV3_AssertionSyntax(t *testing.T) {
	t.Parallel()
	spec, err := parser.Parse(`
spec App {
  scope test {
    contract {
      input { x: int }
      output { y: int, error: string? }
    }
    scenario smoke {
      given { x: 1 }
      then {
        y == 42
        error == null
        y > 0
        y != 0
        y >= 1
        y <= 100
      }
    }
  }
}`)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	assertions := spec.Scopes[0].Scenarios[0].Then.Assertions
	if len(assertions) != 6 {
		t.Fatalf("expected 6 assertions, got %d", len(assertions))
	}

	// All assertions should be expression-based (BinaryOp)
	ops := []string{"==", "==", ">", "!=", ">=", "<="}
	for i, expectedOp := range ops {
		a := assertions[i]
		if a.Expr == nil {
			t.Fatalf("assertion[%d]: expected Expr, got nil", i)
		}
		binOp, ok := a.Expr.(parser.BinaryOp)
		if !ok {
			t.Fatalf("assertion[%d]: expected BinaryOp, got %T", i, a.Expr)
		}
		if binOp.Op != expectedOp {
			t.Errorf("assertion[%d]: expected op %q, got %q", i, expectedOp, binOp.Op)
		}
	}
}

func TestParseV3_AdapterCallInAssertion(t *testing.T) {
	t.Parallel()
	spec, err := parser.Parse(`
spec App {
  playwright {
    base_url: "http://localhost:8080"
  }
  scope test {
    contract {
      input { x: int }
      output { ok: bool }
    }
    scenario check_ui {
      given { x: 1 }
      then {
        playwright.visible('[data-testid="welcome"]') == true
        playwright.text('[data-testid="msg"]') == "hello"
      }
    }
  }
}`)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	assertions := spec.Scopes[0].Scenarios[0].Then.Assertions
	if len(assertions) != 2 {
		t.Fatalf("expected 2 assertions, got %d", len(assertions))
	}

	// First assertion: playwright.visible(...) == true
	a0 := assertions[0]
	binOp, ok := a0.Expr.(parser.BinaryOp)
	if !ok {
		t.Fatalf("expected BinaryOp, got %T", a0.Expr)
	}
	if binOp.Op != "==" {
		t.Errorf("expected ==, got %q", binOp.Op)
	}
	call, ok := binOp.Left.(parser.AdapterCall)
	if !ok {
		t.Fatalf("expected AdapterCall on LHS, got %T", binOp.Left)
	}
	if call.Adapter != "playwright" || call.Method != "visible" {
		t.Errorf("expected playwright.visible, got %s.%s", call.Adapter, call.Method)
	}
}

func TestParseV3_LetInBefore(t *testing.T) {
	t.Parallel()
	spec, err := parser.Parse(`
spec App {
  scope test {
    before {
      let session = login("admin", "test")
      http.header("X-Session", session.token)
    }
    contract {
      input { x: int }
      output { y: int }
    }
  }
}`)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	before := spec.Scopes[0].Before
	if before == nil {
		t.Fatal("expected before block")
	}
	if len(before.Steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(before.Steps))
	}
	letStep, ok := before.Steps[0].(*parser.LetBinding)
	if !ok {
		t.Fatalf("expected LetBinding, got %T", before.Steps[0])
	}
	if letStep.Name != "session" {
		t.Errorf("expected let name session, got %q", letStep.Name)
	}
}

func TestParseV3_LetInGiven(t *testing.T) {
	t.Parallel()
	spec, err := parser.Parse(`
spec App {
  scope test {
    contract {
      input { x: int }
      output { y: int }
    }
    scenario smoke {
      given {
        let setup = http.post("/setup", {})
        x: 1
      }
      then {
        y == 2
      }
    }
  }
}`)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	given := spec.Scopes[0].Scenarios[0].Given
	if len(given.Steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(given.Steps))
	}
	_, ok := given.Steps[0].(*parser.LetBinding)
	if !ok {
		t.Fatalf("expected LetBinding, got %T", given.Steps[0])
	}
	_, ok = given.Steps[1].(*parser.Assignment)
	if !ok {
		t.Fatalf("expected Assignment, got %T", given.Steps[1])
	}
}

func TestParseV3_SingleQuotedStrings(t *testing.T) {
	t.Parallel()
	spec, err := parser.Parse(`
spec App {
  scope test {
    contract {
      input { x: int }
      output { ok: bool }
    }
    scenario check {
      given { x: 1 }
      then {
        playwright.visible('[data-testid="email"]') == true
      }
    }
  }
}`)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	assertions := spec.Scopes[0].Scenarios[0].Then.Assertions
	if len(assertions) != 1 {
		t.Fatalf("expected 1 assertion, got %d", len(assertions))
	}
}

func TestParseV3_CompleteSpec(t *testing.T) {
	t.Parallel()
	spec, err := parser.Parse(`
spec AccountAPI {
  description: "Account transfer API"

  http {
    base_url: env(APP_URL, "http://localhost:8080")
  }

  services {
    app {
      build: "./server"
      port: 8080
    }
  }

  model Account { id: string, balance: int }

  action login(username: string, password: string) {
    let result = http.post("/api/auth/login", { username: username, password: password })
    http.header("Authorization", "Bearer " + result.body.token)
    return result.body
  }

  scope transfer {
    action transfer(from: Account, to: Account, amount: int) {
      let result = http.post("/api/v1/accounts/transfer", {
        from: from, to: to, amount: amount
      })
      return result.body
    }

    before {
      let session = login("admin", "test")
    }

    contract {
      input {
        from: Account
        to: Account
        amount: int { 0 < amount <= from.balance }
      }
      output {
        from: Account
        to: Account
        error: string?
      }
      action: transfer
    }

    invariant conservation {
      when error == null:
        output.from.balance + output.to.balance == input.from.balance + input.to.balance
    }

    invariant non_negative {
      output.from.balance >= 0
      output.to.balance >= 0
    }

    scenario success {
      given {
        from: { id: "alice", balance: 100 }
        to: { id: "bob", balance: 50 }
        amount: 30
      }
      then {
        from.balance == from.balance - amount
        to.balance == to.balance + amount
        error == null
      }
    }

    scenario overdraft {
      when {
        amount > from.balance
      }
      then {
        error == "insufficient_funds"
      }
    }
  }
}`)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	if spec.Name != "AccountAPI" {
		t.Errorf("expected Name=AccountAPI, got %q", spec.Name)
	}
	if spec.Description != "Account transfer API" {
		t.Errorf("expected description, got %q", spec.Description)
	}

	// Adapter configs
	if len(spec.AdapterConfigs) != 1 {
		t.Fatalf("expected 1 adapter config, got %d", len(spec.AdapterConfigs))
	}

	// Services
	if len(spec.Services) != 1 {
		t.Fatalf("expected 1 service, got %d", len(spec.Services))
	}

	// Model
	if len(spec.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(spec.Models))
	}
	if spec.Models[0].Name != "Account" {
		t.Errorf("expected model name Account, got %q", spec.Models[0].Name)
	}

	// Spec-level actions
	if len(spec.Actions) != 1 {
		t.Fatalf("expected 1 spec action, got %d", len(spec.Actions))
	}
	if spec.Actions[0].Name != "login" {
		t.Errorf("expected action name login, got %q", spec.Actions[0].Name)
	}

	// Scope
	if len(spec.Scopes) != 1 {
		t.Fatalf("expected 1 scope, got %d", len(spec.Scopes))
	}
	scope := spec.Scopes[0]
	if scope.Name != "transfer" {
		t.Errorf("expected scope name transfer, got %q", scope.Name)
	}
	if len(scope.Actions) != 1 {
		t.Fatalf("expected 1 scope action, got %d", len(scope.Actions))
	}
	if scope.Before == nil {
		t.Fatal("expected before block")
	}
	if scope.Contract == nil {
		t.Fatal("expected contract")
	}
	if scope.Contract.Action != "transfer" {
		t.Errorf("expected contract action transfer, got %q", scope.Contract.Action)
	}
	if len(scope.Invariants) != 2 {
		t.Fatalf("expected 2 invariants, got %d", len(scope.Invariants))
	}
	if len(scope.Scenarios) != 2 {
		t.Fatalf("expected 2 scenarios, got %d", len(scope.Scenarios))
	}
}

func TestParseV3_NoUseRequired(t *testing.T) {
	t.Parallel()
	// Scopes no longer require 'use' directive
	_, err := parser.Parse(`
spec App {
  scope test {
    contract {
      input { x: int }
      output { y: int }
    }
  }
}`)
	if err != nil {
		t.Fatalf("expected no error without 'use', got: %v", err)
	}
}

// --- Expression precedence tests (unchanged from v2, but without 'use') ---

func TestParseExprPrecedence(t *testing.T) {
	t.Parallel()

	tests := []struct {
		checkFn func(*testing.T, parser.Expr)
		name    string
		input   string
	}{
		{
			name:  "addition before equality",
			input: "spec T { scope s { invariant i { a + b == c } } }",
			checkFn: func(t *testing.T, expr parser.Expr) {
				t.Helper()
				eq, ok := expr.(parser.BinaryOp)
				if !ok || eq.Op != "==" {
					t.Fatalf("expected ==, got %v", expr)
				}
				plus, ok := eq.Left.(parser.BinaryOp)
				if !ok || plus.Op != "+" {
					t.Errorf("expected + on left of ==, got %v", eq.Left)
				}
			},
		},
		{
			name:  "and before or",
			input: "spec T { scope s { invariant i { a || b && c } } }",
			checkFn: func(t *testing.T, expr parser.Expr) {
				t.Helper()
				or, ok := expr.(parser.BinaryOp)
				if !ok || or.Op != "||" {
					t.Fatalf("expected ||, got %v", expr)
				}
				and, ok := or.Right.(parser.BinaryOp)
				if !ok || and.Op != "&&" {
					t.Errorf("expected && on right of ||, got %v", or.Right)
				}
			},
		},
		{
			name:  "division at multiplicative precedence",
			input: "spec T { scope s { invariant i { a + b / c } } }",
			checkFn: func(t *testing.T, expr parser.Expr) {
				t.Helper()
				plus, ok := expr.(parser.BinaryOp)
				if !ok || plus.Op != "+" {
					t.Fatalf("expected +, got %v", expr)
				}
				div, ok := plus.Right.(parser.BinaryOp)
				if !ok || div.Op != "/" {
					t.Errorf("expected / on right of +, got %v", plus.Right)
				}
			},
		},
		{
			name:  "modulo at multiplicative precedence",
			input: "spec T { scope s { invariant i { a + b % c } } }",
			checkFn: func(t *testing.T, expr parser.Expr) {
				t.Helper()
				plus, ok := expr.(parser.BinaryOp)
				if !ok || plus.Op != "+" {
					t.Fatalf("expected +, got %v", expr)
				}
				mod, ok := plus.Right.(parser.BinaryOp)
				if !ok || mod.Op != "%" {
					t.Errorf("expected %% on right of +, got %v", plus.Right)
				}
			},
		},
		{
			name:  "unary negation",
			input: "spec T { scope s { invariant i { !a } } }",
			checkFn: func(t *testing.T, expr parser.Expr) {
				t.Helper()
				unary, ok := expr.(parser.UnaryOp)
				if !ok || unary.Op != "!" {
					t.Fatalf("expected UnaryOp{!}, got %v", expr)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			spec, err := parser.Parse(tc.input)
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

	spec, err := parser.Parse(`spec Foo {
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

	spec, err := parser.Parse(`spec Foo {}`)
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
			_, err := parser.Parse(tc.input)
			if err == nil {
				t.Error("expected parse error, got nil")
			}
		})
	}
}

func TestParseV3_DuplicateBeforeRejected(t *testing.T) {
	t.Parallel()
	_, err := parser.Parse(`
spec Test {
  scope api {
    before {
      http.post("/setup", {})
    }
    before {
      http.post("/setup2", {})
    }
    contract {
      input { x: int }
      output { y: int }
    }
  }
}`)
	if err == nil {
		t.Fatal("expected error for duplicate before blocks")
	}
	if !strings.Contains(err.Error(), "multiple 'before' blocks") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseV3_UnknownTokenInScope(t *testing.T) {
	t.Parallel()
	_, err := parser.Parse(`
spec Test {
  scope api {
    use http
  }
}`)
	if err == nil {
		t.Fatal("expected error for 'use' in v3 scope")
	}
}

func TestParseV3_AdapterCallAsExpr(t *testing.T) {
	t.Parallel()
	spec, err := parser.Parse(`
spec App {
  scope test {
    invariant visible_check {
      http.status() == 200
    }
  }
}`)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	inv := spec.Scopes[0].Invariants[0]
	if len(inv.Assertions) != 1 {
		t.Fatalf("expected 1 assertion, got %d", len(inv.Assertions))
	}
	binOp, ok := inv.Assertions[0].Expr.(parser.BinaryOp)
	if !ok {
		t.Fatalf("expected BinaryOp, got %T", inv.Assertions[0].Expr)
	}
	call, ok := binOp.Left.(parser.AdapterCall)
	if !ok {
		t.Fatalf("expected AdapterCall on LHS, got %T", binOp.Left)
	}
	if call.Adapter != "http" || call.Method != "status" {
		t.Errorf("expected http.status, got %s.%s", call.Adapter, call.Method)
	}
}
