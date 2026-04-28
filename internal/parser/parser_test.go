package parser_test

import (
	"strings"
	"testing"

	"github.com/bamsammich/speclang/v4/internal/parser"
)

// --- v4 spec-level structure ---

func TestParseV4_AdapterConfigBlocks(t *testing.T) {
	t.Parallel()
	spec, err := parser.Parse(`
http {
  base_url: env(APP_URL, "http://localhost:8080")
}
playwright {
  base_url: env(APP_URL, "http://localhost:8080")
  headless: true
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

func TestParseV4_SpecLevelServices(t *testing.T) {
	t.Parallel()
	spec, err := parser.Parse(`
services {
  app {
    build: "./server"
    port: 8080
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

func TestParseV4_ActionDef(t *testing.T) {
	t.Parallel()
	spec, err := parser.Parse(`
http {
  base_url: "http://localhost:8080"
}
action login(username: string, password: string) {
  let result = http.post("/api/auth/login", { username: username, password: password })
  http.header("Authorization", "Bearer " + result.body.access_token)
  return result.body
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

func TestParseV4_ContractWithActionBlock(t *testing.T) {
	t.Parallel()
	spec, err := parser.Parse(`
scope test {
  contract DoSomething -> int {
    x: int
    action {
      return http.post("/do", { x: x })
    }
  }
}`)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	c := spec.Scopes[0].Contracts[0]
	if c.Action == nil {
		t.Fatal("expected contract action block")
	}
	if len(c.Action.Body) != 1 {
		t.Fatalf("expected 1 action step, got %d", len(c.Action.Body))
	}
}

func TestParseV4_AssertionSyntax(t *testing.T) {
	t.Parallel()
	spec, err := parser.Parse(`
scope test {
  contract AssertTest -> int {
    x: int
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

	assertions := spec.Scopes[0].Contracts[0].Scenarios[0].Then.Assertions
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

func TestParseV4_AdapterCallInAssertion(t *testing.T) {
	t.Parallel()
	spec, err := parser.Parse(`
playwright {
  base_url: "http://localhost:8080"
}
scope test {
  contract UiCheck -> bool {
    x: int
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
	assertions := spec.Scopes[0].Contracts[0].Scenarios[0].Then.Assertions
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

func TestParseV4_LetInBefore(t *testing.T) {
	t.Parallel()
	spec, err := parser.Parse(`
scope test {
  before {
    let session = login("admin", "test")
    http.header("X-Session", session.token)
  }
  contract SomeContract -> int {
    x: int
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

func TestParseV4_LetInGiven(t *testing.T) {
	t.Parallel()
	spec, err := parser.Parse(`
scope test {
  contract LetGiven -> int {
    x: int
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
	given := spec.Scopes[0].Contracts[0].Scenarios[0].Given
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

func TestParseV4_SingleQuotedStrings(t *testing.T) {
	t.Parallel()
	spec, err := parser.Parse(`
scope test {
  contract SingleQuote -> bool {
    x: int
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
	assertions := spec.Scopes[0].Contracts[0].Scenarios[0].Then.Assertions
	if len(assertions) != 1 {
		t.Fatalf("expected 1 assertion, got %d", len(assertions))
	}
}

func TestParseV4_CompleteSpec(t *testing.T) {
	t.Parallel()
	spec, err := parser.Parse(`
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
  before {
    let session = login("admin", "test")
  }

  contract Transfer -> Account {
    from: Account
    to: Account
    amount: int { 0 < amount <= from.balance }
    action {
      return http.post("/api/v1/accounts/transfer", { from: from, to: to, amount: amount })
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
	if scope.Before == nil {
		t.Fatal("expected before block")
	}
	if len(scope.Contracts) != 1 {
		t.Fatalf("expected 1 contract, got %d", len(scope.Contracts))
	}
	c := scope.Contracts[0]
	if c.Action == nil {
		t.Fatal("expected contract action block")
	}
	if len(c.Invariants) != 2 {
		t.Fatalf("expected 2 invariants, got %d", len(c.Invariants))
	}
	if len(c.Scenarios) != 2 {
		t.Fatalf("expected 2 scenarios, got %d", len(c.Scenarios))
	}
}

func TestParseV4_NoUseRequired(t *testing.T) {
	t.Parallel()
	// Scopes no longer require 'use' directive
	_, err := parser.Parse(`
scope test {
  contract SomeContract -> int {
    x: int
  }
}`)
	if err != nil {
		t.Fatalf("expected no error without 'use', got: %v", err)
	}
}

// --- Expression precedence tests ---

func TestParseExprPrecedence(t *testing.T) {
	t.Parallel()

	tests := []struct {
		checkFn func(*testing.T, parser.Expr)
		name    string
		input   string
	}{
		{
			name:  "addition before equality",
			input: `scope s { contract C -> int { invariant i { a + b == c } } }`,
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
			input: `scope s { contract C -> bool { invariant i { a or b and c } } }`,
			checkFn: func(t *testing.T, expr parser.Expr) {
				t.Helper()
				or, ok := expr.(parser.BinaryOp)
				if !ok || or.Op != "or" {
					t.Fatalf("expected or, got %v", expr)
				}
				and, ok := or.Right.(parser.BinaryOp)
				if !ok || and.Op != "and" {
					t.Errorf("expected and on right of or, got %v", or.Right)
				}
			},
		},
		{
			name:  "division at multiplicative precedence",
			input: `scope s { contract C -> int { invariant i { a + b / c } } }`,
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
			input: `scope s { contract C -> int { invariant i { a + b % c } } }`,
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
			input: `scope s { contract C -> bool { invariant i { not a } } }`,
			checkFn: func(t *testing.T, expr parser.Expr) {
				t.Helper()
				unary, ok := expr.(parser.UnaryOp)
				if !ok || unary.Op != "not" {
					t.Fatalf("expected UnaryOp{not}, got %v", expr)
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
				len(spec.Scopes[0].Contracts) != 1 ||
				len(spec.Scopes[0].Contracts[0].Invariants) != 1 ||
				len(spec.Scopes[0].Contracts[0].Invariants[0].Assertions) != 1 {
				t.Fatal("expected 1 scope with 1 contract with 1 invariant with 1 assertion")
			}
			tc.checkFn(t, spec.Scopes[0].Contracts[0].Invariants[0].Assertions[0].Expr)
		})
	}
}

func TestParseErrorCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
	}{
		{"unexpected token at top level", "123"},
		{"unterminated scope", "scope T {"},
		{"scope missing name", "scope {"},
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

func TestParseV4_DuplicateBeforeRejected(t *testing.T) {
	t.Parallel()
	_, err := parser.Parse(`
scope api {
  before {
    http.post("/setup", {})
  }
  before {
    http.post("/setup2", {})
  }
  contract SomeContract -> int {
    x: int
  }
}`)
	if err == nil {
		t.Fatal("expected error for duplicate before blocks")
	}
	if !strings.Contains(err.Error(), "multiple 'before' blocks") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseV4_UnknownTokenInScope(t *testing.T) {
	t.Parallel()
	_, err := parser.Parse(`
scope api {
  use http
}`)
	if err == nil {
		t.Fatal("expected error for 'use' in v4 scope")
	}
}

func TestParseV4_AdapterCallAsExpr(t *testing.T) {
	t.Parallel()
	spec, err := parser.Parse(`
scope test {
  contract StatusCheck -> int {
    invariant visible_check {
      http.status() == 200
    }
  }
}`)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	inv := spec.Scopes[0].Contracts[0].Invariants[0]
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

func TestParseV4_SpecWrapperRejected(t *testing.T) {
	t.Parallel()
	_, err := parser.Parse(`spec App { }`)
	if err == nil {
		t.Fatal("expected error for v3 spec wrapper in v4")
	}
	if !strings.Contains(err.Error(), "spec Name { }") && !strings.Contains(err.Error(), "wrapper") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- GAP A: Top-level config block ---

// TestParseV4_ConfigBlock verifies that a top-level config { } block parses
// correctly into Spec.Config. "config" lexes as TokenConfig (a keyword), which
// previously caused "unexpected token Config at top level".
func TestParseV4_ConfigBlock(t *testing.T) {
	t.Parallel()
	spec, err := parser.Parse(`
config {
  max_transfer: 1000000
  api_version: "v2"
}`)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if len(spec.Config) != 2 {
		t.Fatalf("expected 2 config keys, got %d: %v", len(spec.Config), spec.Config)
	}
	if _, ok := spec.Config["max_transfer"]; !ok {
		t.Error("expected config key max_transfer")
	}
	if _, ok := spec.Config["api_version"]; !ok {
		t.Error("expected config key api_version")
	}
	// Verify the values are the correct expression types.
	if mt, ok := spec.Config["max_transfer"]; ok {
		if li, ok := mt.(parser.LiteralInt); !ok || li.Value != 1000000 {
			t.Errorf("max_transfer: expected LiteralInt(1000000), got %T(%v)", mt, mt)
		}
	}
	if av, ok := spec.Config["api_version"]; ok {
		if ls, ok := av.(parser.LiteralString); !ok || ls.Value != "v2" {
			t.Errorf("api_version: expected LiteralString(v2), got %T(%v)", av, av)
		}
	}
}

// TestParseV4_ConfigBlockWithOtherDecls verifies config block coexists with
// models and contracts at the top level.
func TestParseV4_ConfigBlockWithOtherDecls(t *testing.T) {
	t.Parallel()
	spec, err := parser.Parse(`
config {
  limit: 500
}

model Item {
  name: string
}

contract GetItem -> Item {
  id: string
}`)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if len(spec.Config) != 1 {
		t.Fatalf("expected 1 config key, got %d", len(spec.Config))
	}
	if len(spec.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(spec.Models))
	}
	if len(spec.Contracts) != 1 {
		t.Fatalf("expected 1 contract, got %d", len(spec.Contracts))
	}
}

// TestParseV4_ConfigRefInConstraint verifies config.key references parse inside
// constraint expressions.
func TestParseV4_ConfigRefInConstraint(t *testing.T) {
	t.Parallel()
	spec, err := parser.Parse(`
config {
  max_transfer: 1000000
}

contract Transfer -> string {
  amount: int { amount <= config.max_transfer }
}`)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if len(spec.Contracts) != 1 {
		t.Fatalf("expected 1 contract, got %d", len(spec.Contracts))
	}
	c := spec.Contracts[0]
	if len(c.Fields) != 1 {
		t.Fatalf("expected 1 field, got %d", len(c.Fields))
	}
	f := c.Fields[0]
	if f.Constraint == nil {
		t.Fatal("expected constraint on amount field")
	}
	// constraint should be a BinaryOp
	_, ok := f.Constraint.(parser.BinaryOp)
	if !ok {
		t.Fatalf("expected BinaryOp constraint, got %T", f.Constraint)
	}
}

// --- GAP B: State-dependent field presence ---

// TestParseV4_FieldWhen verifies that "field: type when expr" parses the When
// expression onto Field.When, which was previously not consumed by parseField.
func TestParseV4_FieldWhen(t *testing.T) {
	t.Parallel()
	spec, err := parser.Parse(`
model Shipment {
  status: string
  tracking: string when status == "shipped"
}`)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if len(spec.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(spec.Models))
	}
	m := spec.Models[0]
	if len(m.Fields) != 2 {
		t.Fatalf("expected 2 fields, got %d", len(m.Fields))
	}
	statusField := m.Fields[0]
	if statusField.Name != "status" {
		t.Errorf("expected field 0 to be status, got %q", statusField.Name)
	}
	if statusField.When != nil {
		t.Errorf("status field should not have When, got %v", statusField.When)
	}
	trackingField := m.Fields[1]
	if trackingField.Name != "tracking" {
		t.Errorf("expected field 1 to be tracking, got %q", trackingField.Name)
	}
	if trackingField.When == nil {
		t.Fatal("tracking field should have a When expression")
	}
	// The When expression should be a BinaryOp (status == "shipped")
	binOp, ok := trackingField.When.(parser.BinaryOp)
	if !ok {
		t.Fatalf("expected BinaryOp When expression, got %T", trackingField.When)
	}
	if binOp.Op != "==" {
		t.Errorf("expected == operator in When, got %q", binOp.Op)
	}
}

// TestParseV4_FieldWhenInContract verifies that state-dependent fields work
// inside contract input field declarations.
func TestParseV4_FieldWhenInContract(t *testing.T) {
	t.Parallel()
	spec, err := parser.Parse(`
contract Order -> string {
  status: string
  tracking: string when status == "shipped"
  notes: string when status == "cancelled"
}`)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if len(spec.Contracts) != 1 {
		t.Fatalf("expected 1 contract, got %d", len(spec.Contracts))
	}
	c := spec.Contracts[0]
	if len(c.Fields) != 3 {
		t.Fatalf("expected 3 fields, got %d", len(c.Fields))
	}
	// status: no When
	if c.Fields[0].When != nil {
		t.Errorf("status should not have When")
	}
	// tracking: has When
	if c.Fields[1].When == nil {
		t.Errorf("tracking should have When")
	}
	// notes: has When
	if c.Fields[2].When == nil {
		t.Errorf("notes should have When")
	}
}

// TestParseV4_FieldWhenWithConstraint verifies that a field can have both a
// constraint block and a when expression.
func TestParseV4_FieldWhenWithConstraint(t *testing.T) {
	t.Parallel()
	spec, err := parser.Parse(`
model Account {
  balance: int { balance >= 0 }
  bonus: int { bonus > 0 } when balance > 1000
}`)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	m := spec.Models[0]
	bonus := m.Fields[1]
	if bonus.Name != "bonus" {
		t.Fatalf("expected bonus field, got %q", bonus.Name)
	}
	if bonus.Constraint == nil {
		t.Error("bonus should have Constraint")
	}
	if bonus.When == nil {
		t.Error("bonus should have When expression")
	}
}

// --- in-operator RHS forms ---

// inOperatorInvariant is a helper that parses a spec with a single invariant
// whose body is `field in <rhs>` and returns the parsed BinaryOp.
func inOperatorInvariant(t *testing.T, rhs string) parser.BinaryOp {
	t.Helper()
	src := `scope s { contract C -> int { invariant i { status in ` + rhs + ` } } }`
	spec, err := parser.Parse(src)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	inv := spec.Scopes[0].Contracts[0].Invariants[0]
	if len(inv.Assertions) != 1 {
		t.Fatalf("expected 1 assertion, got %d", len(inv.Assertions))
	}
	op, ok := inv.Assertions[0].Expr.(parser.BinaryOp)
	if !ok {
		t.Fatalf("expected BinaryOp, got %T", inv.Assertions[0].Expr)
	}
	if op.Op != "in" {
		t.Fatalf("expected op 'in', got %q", op.Op)
	}
	return op
}

// TestParseIn_BracketForm verifies the existing bracket form `x in [a, b, c]`.
func TestParseIn_BracketForm(t *testing.T) {
	t.Parallel()
	op := inOperatorInvariant(t, `["pending", "active"]`)
	arr, ok := op.Right.(parser.ArrayLiteral)
	if !ok {
		t.Fatalf("expected ArrayLiteral RHS, got %T", op.Right)
	}
	if len(arr.Elements) != 2 {
		t.Fatalf("expected 2 elements, got %d", len(arr.Elements))
	}
}

// TestParseIn_ParenForm verifies the new paren form `x in (a, b, c)`.
func TestParseIn_ParenForm(t *testing.T) {
	t.Parallel()
	op := inOperatorInvariant(t, `("pending", "active")`)
	arr, ok := op.Right.(parser.ArrayLiteral)
	if !ok {
		t.Fatalf("expected ArrayLiteral RHS from paren form, got %T", op.Right)
	}
	if len(arr.Elements) != 2 {
		t.Fatalf("expected 2 elements, got %d", len(arr.Elements))
	}
	// Verify element values
	for i, want := range []string{"pending", "active"} {
		lit, ok := arr.Elements[i].(parser.LiteralString)
		if !ok {
			t.Fatalf("element %d: expected LiteralString, got %T", i, arr.Elements[i])
		}
		if lit.Value != want {
			t.Errorf("element %d: expected %q, got %q", i, want, lit.Value)
		}
	}
}

// TestParseIn_ParenFormSingleElement verifies `x in ("only")` — single element.
func TestParseIn_ParenFormSingleElement(t *testing.T) {
	t.Parallel()
	op := inOperatorInvariant(t, `("only")`)
	arr, ok := op.Right.(parser.ArrayLiteral)
	if !ok {
		t.Fatalf("expected ArrayLiteral RHS, got %T", op.Right)
	}
	if len(arr.Elements) != 1 {
		t.Fatalf("expected 1 element, got %d", len(arr.Elements))
	}
}

// TestParseIn_ParenFormTrailingComma verifies `x in (a, b,)` trailing comma is accepted.
func TestParseIn_ParenFormTrailingComma(t *testing.T) {
	t.Parallel()
	op := inOperatorInvariant(t, `("a", "b",)`)
	arr, ok := op.Right.(parser.ArrayLiteral)
	if !ok {
		t.Fatalf("expected ArrayLiteral RHS, got %T", op.Right)
	}
	if len(arr.Elements) != 2 {
		t.Fatalf("expected 2 elements, got %d", len(arr.Elements))
	}
}

// TestParseIn_ParenFormIntegers verifies `x in (1, 2, 3)` with int literals.
func TestParseIn_ParenFormIntegers(t *testing.T) {
	t.Parallel()
	op := inOperatorInvariant(t, `(1, 2, 3)`)
	arr, ok := op.Right.(parser.ArrayLiteral)
	if !ok {
		t.Fatalf("expected ArrayLiteral RHS, got %T", op.Right)
	}
	if len(arr.Elements) != 3 {
		t.Fatalf("expected 3 elements, got %d", len(arr.Elements))
	}
	for i, want := range []int{1, 2, 3} {
		lit, ok := arr.Elements[i].(parser.LiteralInt)
		if !ok {
			t.Fatalf("element %d: expected LiteralInt, got %T", i, arr.Elements[i])
		}
		if lit.Value != want {
			t.Errorf("element %d: expected %d, got %d", i, want, lit.Value)
		}
	}
}

// TestParseIn_EmptyParenFormErrors verifies `x in ()` produces a parse error.
func TestParseIn_EmptyParenFormErrors(t *testing.T) {
	t.Parallel()
	src := `scope s { contract C -> int { invariant i { status in () } } }`
	_, err := parser.Parse(src)
	if err == nil {
		t.Fatal("expected parse error for empty in () list")
	}
	if !strings.Contains(err.Error(), "at least one element") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

// TestParseIn_BracketAndParenEquivalent verifies that bracket and paren forms
// produce structurally identical ArrayLiteral RHS nodes.
func TestParseIn_BracketAndParenEquivalent(t *testing.T) {
	t.Parallel()
	bracket := inOperatorInvariant(t, `[1, 2, 3]`)
	paren := inOperatorInvariant(t, `(1, 2, 3)`)

	bArr, ok := bracket.Right.(parser.ArrayLiteral)
	if !ok {
		t.Fatalf("bracket: expected ArrayLiteral, got %T", bracket.Right)
	}
	pArr, ok := paren.Right.(parser.ArrayLiteral)
	if !ok {
		t.Fatalf("paren: expected ArrayLiteral, got %T", paren.Right)
	}

	if len(bArr.Elements) != len(pArr.Elements) {
		t.Fatalf("element count mismatch: bracket=%d paren=%d",
			len(bArr.Elements), len(pArr.Elements))
	}
	for i := range bArr.Elements {
		bLit, bOk := bArr.Elements[i].(parser.LiteralInt)
		pLit, pOk := pArr.Elements[i].(parser.LiteralInt)
		if !bOk || !pOk {
			t.Fatalf("element %d: types differ — bracket=%T paren=%T",
				i, bArr.Elements[i], pArr.Elements[i])
		}
		if bLit.Value != pLit.Value {
			t.Errorf("element %d: values differ — bracket=%d paren=%d",
				i, bLit.Value, pLit.Value)
		}
	}
}
