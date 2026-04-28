package migrate

import (
	"strings"
	"testing"

	"github.com/bamsammich/speclang/v4/internal/parser"
	"github.com/bamsammich/speclang/v4/internal/v3parser"
	"github.com/bamsammich/speclang/v4/pkg/spec"
)

// --- Operator normalization tests (AST-level) ---

// TestNormalizeOperatorsInExpr_And verifies && BinaryOp is rewritten to "and".
func TestNormalizeOperatorsInExpr_And(t *testing.T) {
	t.Parallel()

	e := spec.BinaryOp{
		Left:  spec.FieldRef{Path: "a"},
		Op:    "&&",
		Right: spec.FieldRef{Path: "b"},
	}
	got := v4FormatExpr(e)
	if !strings.Contains(got, "and") {
		t.Errorf("expected 'and' in output, got %q", got)
	}
	if strings.Contains(got, "&&") {
		t.Errorf("expected && to be removed, got %q", got)
	}
}

// TestNormalizeOperatorsInExpr_Or verifies || BinaryOp is rewritten to "or".
func TestNormalizeOperatorsInExpr_Or(t *testing.T) {
	t.Parallel()

	e := spec.BinaryOp{
		Left:  spec.FieldRef{Path: "a"},
		Op:    "||",
		Right: spec.FieldRef{Path: "b"},
	}
	got := v4FormatExpr(e)
	if !strings.Contains(got, "or") {
		t.Errorf("expected 'or' in output, got %q", got)
	}
	if strings.Contains(got, "||") {
		t.Errorf("expected || to be removed, got %q", got)
	}
}

// TestNormalizeOperatorsInExpr_Not verifies ! UnaryOp is rewritten to "not".
func TestNormalizeOperatorsInExpr_Not(t *testing.T) {
	t.Parallel()

	e := spec.UnaryOp{
		Op:      "!",
		Operand: spec.FieldRef{Path: "cond"},
	}
	got := v4FormatExpr(e)
	if !strings.Contains(got, "not") {
		t.Errorf("expected 'not' in output, got %q", got)
	}
	if strings.Contains(got, "!") {
		t.Errorf("expected ! to be removed, got %q", got)
	}
}

// TestNormalizeOperatorsInExpr_NotEqualPreserved verifies != BinaryOp is not touched.
func TestNormalizeOperatorsInExpr_NotEqualPreserved(t *testing.T) {
	t.Parallel()

	e := spec.BinaryOp{
		Left:  spec.FieldRef{Path: "x"},
		Op:    "!=",
		Right: spec.LiteralNull{},
	}
	got := v4FormatExpr(e)
	if !strings.Contains(got, "!=") {
		t.Errorf("!= should be preserved, got %q", got)
	}
	if strings.Contains(got, "not") {
		t.Errorf("!= should not become 'not', got %q", got)
	}
}

// TestMigrateV3File_StringLiteralWithAnd verifies && inside a string literal is not rewritten.
// Regression test: prior text-pass would corrupt URLs containing && query params.
func TestMigrateV3File_StringLiteralWithAnd(t *testing.T) {
	t.Parallel()

	// The given block has a string literal with && in it (e.g. a URL query string).
	src := `spec API {
  scope check {
    action check(url: string) {
      let result = http.post(url, {})
      return result
    }

    contract {
      input {
        url: string
      }
      output {
        ok: bool
      }
      action: check
    }

    scenario url_with_ampersand {
      given {
        url: "http://example.com?a=1&&b=2"
      }
      then {
        ok == true
      }
    }
  }
}`
	got, err := MigrateV3File(src)
	if err != nil {
		t.Fatalf("MigrateV3File: %v", err)
	}
	// The literal must be preserved verbatim — && must not become " and "
	if !strings.Contains(got, `"http://example.com?a=1&&b=2"`) {
		t.Errorf("string literal with && was corrupted; got:\n%s", got)
	}
}

// TestMigrateV3File_StringLiteralWithBang verifies ! inside a string literal is not rewritten.
func TestMigrateV3File_StringLiteralWithBang(t *testing.T) {
	t.Parallel()

	src := `spec API {
  scope ping {
    action ping(msg: string) {
      let result = http.post("/ping", { msg: msg })
      return result
    }

    contract {
      input {
        msg: string
      }
      output {
        ok: bool
      }
      action: ping
    }

    scenario bang_in_string {
      given {
        msg: "bang!"
      }
      then {
        ok == true
      }
    }
  }
}`
	got, err := MigrateV3File(src)
	if err != nil {
		t.Fatalf("MigrateV3File: %v", err)
	}
	if !strings.Contains(got, `"bang!"`) {
		t.Errorf("string literal with ! was corrupted; got:\n%s", got)
	}
}

// TestMigrateV3File_OperatorInExpressionRewritten verifies a real && operator in an expression
// is rewritten to "and" while a && inside a string literal in the same spec is preserved.
func TestMigrateV3File_MixedOperatorAndStringLiteral(t *testing.T) {
	t.Parallel()

	src := `spec API {
  scope multi {
    action multi(a: bool, b: bool) {
      let result = http.post("/multi", { a: a, b: b })
      return result
    }

    contract {
      input {
        a: bool
        b: bool
      }
      output {
        ok: bool
      }
      action: multi
    }

    scenario mixed {
      given {
        a: true
        b: true
      }
      then {
        ok == true
      }
    }

    invariant both_true {
      a && b
    }
  }
}`
	got, err := MigrateV3File(src)
	if err != nil {
		t.Fatalf("MigrateV3File: %v", err)
	}
	// The && operator in the invariant expression must become "and"
	if !strings.Contains(got, "a and b") {
		t.Errorf("expected '&& → and' in invariant expression; got:\n%s", got)
	}
	// No raw && should remain in the output
	if strings.Contains(got, "&&") {
		t.Errorf("raw && found in output (should have been rewritten); got:\n%s", got)
	}
}

// --- Wrapper strip test ---

// TestMigrateV3File_StripSpecWrapper verifies the spec Name { } wrapper is removed.
func TestMigrateV3File_StripSpecWrapper(t *testing.T) {
	t.Parallel()

	src := `spec Minimal {
  model Foo {
    x: int
  }
}`
	got, err := MigrateV3File(src)
	if err != nil {
		t.Fatalf("MigrateV3File: %v", err)
	}
	if strings.Contains(got, "spec Minimal") {
		t.Errorf("output should not contain 'spec Minimal', got:\n%s", got)
	}
	if !strings.Contains(got, "model Foo") {
		t.Errorf("output should contain 'model Foo', got:\n%s", got)
	}
}

// TestMigrateV3File_DropDescription verifies description field is removed.
func TestMigrateV3File_DropDescription(t *testing.T) {
	t.Parallel()

	src := `spec Foo {
  description: "some description"

  model Bar {
    x: int
  }
}`
	got, err := MigrateV3File(src)
	if err != nil {
		t.Fatalf("MigrateV3File: %v", err)
	}
	if strings.Contains(got, "description") {
		t.Errorf("output should not contain 'description', got:\n%s", got)
	}
}

// --- Scope → contract structural transform ---

// TestMigrateV3File_ScopeToContract verifies v3 scope becomes a v4 contract.
func TestMigrateV3File_ScopeToContract(t *testing.T) {
	t.Parallel()

	src := `spec API {
  scope do_thing {
    action do_thing(x: int) {
      let result = http.post("/do", { x: x })
      return result
    }

    contract {
      input {
        x: int
      }
      output {
        ok: bool
      }
      action: do_thing
    }

    scenario pass {
      given {
        x: 1
      }
      then {
        ok == true
      }
    }
  }
}`
	got, err := MigrateV3File(src)
	if err != nil {
		t.Fatalf("MigrateV3File: %v", err)
	}
	// Should contain a v4 contract declaration
	if !strings.Contains(got, "contract DoThing") {
		t.Errorf("expected 'contract DoThing', got:\n%s", got)
	}
	// Should have return type
	if !strings.Contains(got, "->") {
		t.Errorf("expected '->' in contract declaration, got:\n%s", got)
	}
	// Should NOT have the scope wrapper (no before/after)
	if strings.Contains(got, "scope do_thing") {
		t.Errorf("scope wrapper should be absent (no before/after), got:\n%s", got)
	}
	// Action body should be inlined
	if !strings.Contains(got, "action {") {
		t.Errorf("expected inlined 'action {' block, got:\n%s", got)
	}
}

// TestMigrateV3File_BeforeAfterPreserved verifies scope wrapper is kept when before/after exists.
func TestMigrateV3File_BeforeAfterPreserved(t *testing.T) {
	t.Parallel()

	src := `spec WithLifecycle {
  http {
    base_url: "http://localhost:8080"
  }

  scope authenticated {
    before {
      http.post("/login", {})
    }

    after {
      http.post("/logout", {})
    }

    action do_thing(name: string) {
      let result = http.post("/thing", { name: name })
      return result
    }

    contract {
      input {
        name: string
      }
      output {
        id: string
      }
      action: do_thing
    }

    scenario create {
      given {
        name: "widget"
      }
      then {
        id == "abc"
      }
    }
  }
}`
	got, err := MigrateV3File(src)
	if err != nil {
		t.Fatalf("MigrateV3File: %v", err)
	}
	// Scope wrapper should be preserved
	if !strings.Contains(got, "scope authenticated {") {
		t.Errorf("expected 'scope authenticated {', got:\n%s", got)
	}
	// before block preserved
	if !strings.Contains(got, "before {") {
		t.Errorf("expected 'before {', got:\n%s", got)
	}
	// after block preserved
	if !strings.Contains(got, "after {") {
		t.Errorf("expected 'after {', got:\n%s", got)
	}
}

// --- output. prefix transform ---

// TestMigrateV3File_OutputPrefix verifies bare output field refs get "output." prefix.
func TestMigrateV3File_OutputPrefix(t *testing.T) {
	t.Parallel()

	src := `spec API {
  scope transfer {
    action transfer(from: string, amount: int) {
      let result = http.post("/transfer", { from: from, amount: amount })
      return result
    }

    contract {
      input {
        from: string
        amount: int
      }
      output {
        error: string?
        balance: int
      }
      action: transfer
    }

    scenario ok {
      given {
        from: "alice"
        amount: 10
      }
      then {
        error == null
        balance == 90
      }
    }
  }
}`
	got, err := MigrateV3File(src)
	if err != nil {
		t.Fatalf("MigrateV3File: %v", err)
	}
	// "error" and "balance" are output fields, should be prefixed
	if !strings.Contains(got, "output.error") {
		t.Errorf("expected 'output.error' in output, got:\n%s", got)
	}
	if !strings.Contains(got, "output.balance") {
		t.Errorf("expected 'output.balance' in output, got:\n%s", got)
	}
}

// TestMigrateV3File_OutputPrefix_AlreadyPrefixed verifies existing output. refs are not double-prefixed.
func TestMigrateV3File_OutputPrefix_AlreadyPrefixed(t *testing.T) {
	t.Parallel()

	src := `spec API {
  scope fetch {
    action fetch(id: string) {
      let result = http.get("/fetch/" + id)
      return result
    }

    contract {
      input {
        id: string
      }
      output {
        name: string
      }
      action: fetch
    }

    invariant has_name {
      output.name != null
    }
  }
}`
	got, err := MigrateV3File(src)
	if err != nil {
		t.Fatalf("MigrateV3File: %v", err)
	}
	// Should not double-prefix: "output.output.name" should not appear
	if strings.Contains(got, "output.output.") {
		t.Errorf("double-prefixing detected, got:\n%s", got)
	}
	if !strings.Contains(got, "output.name") {
		t.Errorf("expected 'output.name', got:\n%s", got)
	}
}

// --- Include preservation ---

// TestExtractIncludes verifies include paths are extracted from v3 source.
func TestExtractIncludes(t *testing.T) {
	t.Parallel()

	src := `spec Foo {
  include "models/account.spec"
  include "scopes/transfer.spec"

  model Bar {
    x: int
  }
}`
	includes := extractIncludes(src)
	if len(includes) != 2 {
		t.Fatalf("expected 2 includes, got %d: %v", len(includes), includes)
	}
	if includes[0] != "models/account.spec" {
		t.Errorf("includes[0] = %q, want %q", includes[0], "models/account.spec")
	}
	if includes[1] != "scopes/transfer.spec" {
		t.Errorf("includes[1] = %q, want %q", includes[1], "scopes/transfer.spec")
	}
}

// TestMigrateV3File_IncludesAtTopLevel verifies include directives are emitted at top level.
func TestMigrateV3File_IncludesAtTopLevel(t *testing.T) {
	t.Parallel()

	// v3 specs with includes — the parser resolves them but we re-emit them
	src := `spec Foo {
  model Bar {
    x: int
  }
}`
	// Inject the original source with includes to test extraction
	s, err := v3parser.Parse(src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	// Simulate having includes in original source
	srcWithIncludes := strings.Replace(src, `spec Foo {`, "spec Foo {\n  include \"shared/models.spec\"", 1)
	got, err := MigrateV3Spec(s, srcWithIncludes)
	if err != nil {
		t.Fatalf("MigrateV3Spec: %v", err)
	}
	// Include should be at top level (first line)
	lines := strings.Split(strings.TrimSpace(got), "\n")
	if lines[0] != `include "shared/models.spec"` {
		t.Errorf("first line should be include, got %q (full output:\n%s)", lines[0], got)
	}
}

// --- Synthesized output model ---

// TestOutputTypeName_SynthesizedModel verifies multi-field output becomes a named model.
func TestOutputTypeName_SynthesizedModel(t *testing.T) {
	t.Parallel()

	sc := &v3parser.Scope{
		Name: "my_action",
		Contract: &v3parser.Contract{
			Output: []*spec.Field{
				{Name: "status", Type: spec.TypeExpr{Name: "string"}},
				{Name: "error", Type: spec.TypeExpr{Name: "string", Optional: true}},
			},
		},
	}

	synthesized := collectSynthesizedModels([]*v3parser.Scope{sc})
	if len(synthesized) != 1 {
		t.Fatalf("expected 1 synthesized model, got %d", len(synthesized))
	}
	if synthesized[0].Name != "MyActionResult" {
		t.Errorf("model name = %q, want %q", synthesized[0].Name, "MyActionResult")
	}
}

// TestOutputTypeName_SingleModelRef verifies single model-ref output uses that model directly.
func TestOutputTypeName_SingleModelRef(t *testing.T) {
	t.Parallel()

	sc := &v3parser.Scope{
		Name: "get_account",
		Contract: &v3parser.Contract{
			Output: []*spec.Field{
				{Name: "account", Type: spec.TypeExpr{Name: "Account"}},
			},
		},
	}

	synthesized := collectSynthesizedModels([]*v3parser.Scope{sc})
	if len(synthesized) != 0 {
		t.Errorf("expected 0 synthesized models for single model-ref output, got %d", len(synthesized))
	}

	retType := outputTypeName(sc, nil)
	if retType != "Account" {
		t.Errorf("return type = %q, want %q", retType, "Account")
	}
}

// TestContractName verifies scope names are CamelCased for contract names.
func TestContractName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  string
	}{
		{"transfer", "Transfer"},
		{"parse_valid", "ParseValid"},
		{"my_long_scope_name", "MyLongScopeName"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			got := contractName(tt.input)
			if got != tt.want {
				t.Errorf("contractName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// TestContractOutputModelName verifies output model naming.
func TestContractOutputModelName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  string
	}{
		{"transfer", "TransferResult"},
		{"parse_valid", "ParseValidResult"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			got := contractOutputModelName(tt.input)
			if got != tt.want {
				t.Errorf("contractOutputModelName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// --- Round-trip test ---

// TestV3ToV4RoundTrip verifies that migrated output parses as valid v4.
func TestV3ToV4RoundTrip(t *testing.T) {
	t.Parallel()

	fixtures := []struct {
		name string
		src  string
	}{
		{
			"operator_swap",
			`spec OperatorSwap {
  scope check {
    action check(x: int, y: int) {
      let result = http.post("/check", { x: x, y: y })
      return result
    }

    contract {
      input {
        x: int
        y: int
      }
      output {
        ok: bool
      }
      action: check
    }

    invariant always_true {
      ok == true
    }
  }
}`,
		},
		{
			"minimal_with_model",
			`spec Minimal {
  model Foo {
    id: string
    count: int
  }

  scope get_foo {
    action get_foo(id: string) {
      let result = http.get("/foo/" + id)
      return result
    }

    contract {
      input {
        id: string
      }
      output {
        count: int
      }
      action: get_foo
    }
  }
}`,
		},
		{
			"scope_contract_with_before_after",
			`spec WithLifecycle {
  http {
    base_url: "http://localhost:8080"
  }

  scope auth_scope {
    before {
      http.post("/login", {})
    }

    after {
      http.post("/logout", {})
    }

    action do_thing(name: string) {
      let result = http.post("/thing", { name: name })
      return result
    }

    contract {
      input {
        name: string
      }
      output {
        status: string
      }
      action: do_thing
    }
  }
}`,
		},
	}

	for _, tt := range fixtures {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			v4src, err := MigrateV3File(tt.src)
			if err != nil {
				t.Fatalf("MigrateV3File: %v", err)
			}

			if _, err := parser.Parse(v4src); err != nil {
				t.Errorf("v4 round-trip parse failed:\n%s\nerror: %v", v4src, err)
			}
		})
	}
}
