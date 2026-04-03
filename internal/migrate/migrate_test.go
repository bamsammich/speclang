package migrate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bamsammich/speclang/v3/internal/parser"
	"github.com/bamsammich/speclang/v3/internal/v2parser"
	"github.com/bamsammich/speclang/v3/pkg/spec"
)

// TestGoldenFiles verifies that migrating v2 specs produces the expected v3 output.
func TestGoldenFiles(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		v2File string
		v3File string
	}{
		{"http_basic", "testdata/http_basic.v2.spec", "testdata/http_basic.v3.spec"},
		{"playwright", "testdata/playwright.v2.spec", "testdata/playwright.v3.spec"},
		{"process", "testdata/process.v2.spec", "testdata/process.v3.spec"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			v2Src, err := os.ReadFile(tt.v2File)
			if err != nil {
				t.Fatalf("reading v2 fixture: %v", err)
			}

			s, err := v2parser.Parse(string(v2Src))
			if err != nil {
				t.Fatalf("parsing v2 spec: %v", err)
			}

			got, err := MigrateSpec(s)
			if err != nil {
				t.Fatalf("migrating spec: %v", err)
			}

			want, err := os.ReadFile(tt.v3File)
			if err != nil {
				t.Fatalf("reading golden file: %v", err)
			}

			if got != string(want) {
				t.Errorf("migration output mismatch.\n--- got ---\n%s\n--- want ---\n%s", got, string(want))
			}
		})
	}
}

// TestRoundTrip verifies that migrated output parses as valid v3.
func TestRoundTrip(t *testing.T) {
	t.Parallel()

	v2Files, err := filepath.Glob("testdata/*.v2.spec")
	if err != nil {
		t.Fatal(err)
	}

	for _, f := range v2Files {
		t.Run(filepath.Base(f), func(t *testing.T) {
			t.Parallel()

			src, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}

			s, err := v2parser.Parse(string(src))
			if err != nil {
				t.Fatalf("v2 parse: %v", err)
			}

			output, err := MigrateSpec(s)
			if err != nil {
				t.Fatalf("migrate: %v", err)
			}

			if _, err := parser.Parse(output); err != nil {
				t.Errorf("v3 round-trip parse failed:\n%s\nerror: %v", output, err)
			}
		})
	}
}

// TestTransformAssertion_ColonToEquals verifies colon assertions become ==.
func TestTransformAssertion_ColonToEquals(t *testing.T) {
	t.Parallel()

	a := &spec.Assertion{
		Target:   "error",
		Operator: ":",
		Expected: spec.LiteralNull{},
	}
	got := transformAssertion(a, nil)
	if got != "error == null" {
		t.Errorf("got %q, want %q", got, "error == null")
	}
}

// TestTransformAssertion_EmptyOperatorToEquals verifies default operator becomes ==.
func TestTransformAssertion_EmptyOperatorToEquals(t *testing.T) {
	t.Parallel()

	a := &spec.Assertion{
		Target:   "status",
		Operator: "",
		Expected: spec.LiteralInt{Value: 200},
	}
	got := transformAssertion(a, nil)
	if got != "status == 200" {
		t.Errorf("got %q, want %q", got, "status == 200")
	}
}

// TestTransformAssertion_PluginAssertion verifies @plugin.property → plugin.method(selector).
func TestTransformAssertion_PluginAssertion(t *testing.T) {
	t.Parallel()

	locators := map[string]string{
		"welcome_msg": `data-testid=welcome`,
	}
	a := &spec.Assertion{
		Target:   "welcome_msg",
		Plugin:   "playwright",
		Property: "visible",
		Operator: ":",
		Expected: spec.LiteralBool{Value: true},
	}
	got := transformAssertion(a, locators)
	want := "playwright.visible('[data-testid=welcome]') == true"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestTransformAssertion_RelationalOperator verifies non-equality operators pass through.
func TestTransformAssertion_RelationalOperator(t *testing.T) {
	t.Parallel()

	a := &spec.Assertion{
		Target:   "count",
		Operator: ">=",
		Expected: spec.LiteralInt{Value: 1},
	}
	got := transformAssertion(a, nil)
	if got != "count >= 1" {
		t.Errorf("got %q, want %q", got, "count >= 1")
	}
}

// TestTransformAssertion_ExpressionAssertion verifies invariant expressions pass through.
func TestTransformAssertion_ExpressionAssertion(t *testing.T) {
	t.Parallel()

	a := &spec.Assertion{
		Expr: spec.BinaryOp{
			Left:  spec.FieldRef{Path: "output.balance"},
			Op:    ">=",
			Right: spec.LiteralInt{Value: 0},
		},
	}
	got := transformAssertion(a, nil)
	if got != "output.balance >= 0" {
		t.Errorf("got %q, want %q", got, "output.balance >= 0")
	}
}

// TestSynthesizeAction_HTTP_POST verifies action synthesis for HTTP POST.
func TestSynthesizeAction_HTTP_POST(t *testing.T) {
	t.Parallel()

	sc := &spec.Scope{
		Name: "transfer",
		Use:  "http",
		Config: map[string]spec.Expr{
			"method": spec.LiteralString{Value: "POST"},
			"path":   spec.LiteralString{Value: "/api/transfer"},
		},
		Contract: &spec.Contract{
			Input: []*spec.Field{
				{Name: "from", Type: spec.TypeExpr{Name: "string"}},
				{Name: "amount", Type: spec.TypeExpr{Name: "int"}},
			},
		},
	}

	sa := synthesizeAction(sc)
	if sa.name != "transfer" {
		t.Errorf("name = %q, want %q", sa.name, "transfer")
	}
	if len(sa.params) != 2 {
		t.Fatalf("params len = %d, want 2", len(sa.params))
	}
	if !strings.Contains(sa.callExpr, "http.post") {
		t.Errorf("callExpr = %q, want to contain %q", sa.callExpr, "http.post")
	}
	if !strings.Contains(sa.callExpr, "/api/transfer") {
		t.Errorf("callExpr = %q, want to contain %q", sa.callExpr, "/api/transfer")
	}
}

// TestSynthesizeAction_HTTP_GET verifies GET actions have no body.
func TestSynthesizeAction_HTTP_GET(t *testing.T) {
	t.Parallel()

	sc := &spec.Scope{
		Name: "fetch",
		Use:  "http",
		Config: map[string]spec.Expr{
			"method": spec.LiteralString{Value: "GET"},
			"path":   spec.LiteralString{Value: "/api/status"},
		},
		Contract: &spec.Contract{
			Input: []*spec.Field{
				{Name: "id", Type: spec.TypeExpr{Name: "string"}},
			},
		},
	}

	sa := synthesizeAction(sc)
	// GET should not have a body object
	if strings.Contains(sa.callExpr, "{") {
		t.Errorf("GET callExpr should not contain body object: %q", sa.callExpr)
	}
}

// TestSynthesizeAction_Process verifies process adapter action synthesis.
func TestSynthesizeAction_Process(t *testing.T) {
	t.Parallel()

	sc := &spec.Scope{
		Name: "cli_test",
		Use:  "process",
		Config: map[string]spec.Expr{
			"command": spec.LiteralString{Value: "./specrun"},
			"args":    spec.LiteralString{Value: "parse"},
		},
		Contract: &spec.Contract{
			Input: []*spec.Field{
				{Name: "file", Type: spec.TypeExpr{Name: "string"}},
			},
		},
	}

	sa := synthesizeAction(sc)
	if !strings.Contains(sa.callExpr, "process.exec") {
		t.Errorf("callExpr = %q, want to contain %q", sa.callExpr, "process.exec")
	}
}

// TestResolveLocator_Found verifies locator lookup and formatting.
func TestResolveLocator_Found(t *testing.T) {
	t.Parallel()

	locators := map[string]string{
		"submit_btn": `data-testid=submit`,
	}
	got := resolveLocator("submit_btn", locators)
	if got != "'[data-testid=submit]'" {
		t.Errorf("got %q, want %q", got, "'[data-testid=submit]'")
	}
}

// TestResolveLocator_Missing verifies unknown locator names are quoted as-is.
func TestResolveLocator_Missing(t *testing.T) {
	t.Parallel()

	got := resolveLocator("unknown", nil)
	if got != `"unknown"` {
		t.Errorf("got %q, want %q", got, `"unknown"`)
	}
}

// TestFormatTypeExpr verifies type formatting for various types.
func TestFormatTypeExpr(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		typ  spec.TypeExpr
		want string
	}{
		{"int", spec.TypeExpr{Name: "int"}, "int"},
		{"string?", spec.TypeExpr{Name: "string", Optional: true}, "string?"},
		{"array", spec.TypeExpr{Name: "array", ElemType: &spec.TypeExpr{Name: "int"}}, "[]int"},
		{"map", spec.TypeExpr{Name: "map", KeyType: &spec.TypeExpr{Name: "string"}, ValType: &spec.TypeExpr{Name: "int"}}, "map[string,int]"},
		{"enum", spec.TypeExpr{Name: "enum", Variants: []string{"a", "b"}}, `enum("a", "b")`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := formatTypeExpr(tt.typ)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// TestHasSynthesizableConfig verifies config detection.
func TestHasSynthesizableConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		config map[string]spec.Expr
		want   bool
	}{
		{"nil config", nil, false},
		{"empty config", map[string]spec.Expr{}, false},
		{"with method", map[string]spec.Expr{"method": spec.LiteralString{Value: "POST"}}, true},
		{"with path", map[string]spec.Expr{"path": spec.LiteralString{Value: "/api"}}, true},
		{"with command", map[string]spec.Expr{"command": spec.LiteralString{Value: "./bin"}}, true},
		{"unrelated", map[string]spec.Expr{"base_url": spec.LiteralString{Value: "http://x"}}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			sc := &spec.Scope{Config: tt.config}
			got := hasSynthesizableConfig(sc)
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}
