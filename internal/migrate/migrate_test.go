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
		{"given_body_ref", "testdata/given_body_ref.v2.spec", "testdata/given_body_ref.v3.spec"},
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

// TestMigrateFile_FollowsIncludes verifies that MigrateFile walks the include tree
// and returns migrated output for all files.
func TestMigrateFile_FollowsIncludes(t *testing.T) {
	t.Parallel()

	results, err := MigrateFile("testdata/with_includes/root.v2.spec")
	if err != nil {
		t.Fatalf("MigrateFile: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 migrated files, got %d", len(results))
	}

	// Root should be first.
	if !strings.HasSuffix(results[0].Path, "root.v2.spec") {
		t.Errorf("first file should be root, got %s", results[0].Path)
	}
	if !strings.HasSuffix(results[1].Path, "scopes.v2.spec") {
		t.Errorf("second file should be scopes, got %s", results[1].Path)
	}

	// Root should have adapter config.
	if !strings.Contains(results[0].Output, "http {") {
		t.Error("root output missing http adapter config")
	}

	// Scopes file should have synthesized action with == assertions.
	if !strings.Contains(results[1].Output, "error == null") {
		t.Error("scopes output should have error == null, got:\n" + results[1].Output)
	}

	// Both should parse as valid v3.
	for _, mf := range results {
		if mf.Warning != "" {
			t.Errorf("%s: v3 parse warning: %s", mf.Path, mf.Warning)
		}
	}
}

// TestCollectIncludes verifies include tree walking.
func TestCollectIncludes(t *testing.T) {
	t.Parallel()

	abs, err := filepath.Abs("testdata/with_includes/root.v2.spec")
	if err != nil {
		t.Fatal(err)
	}

	files, err := collectIncludes(abs)
	if err != nil {
		t.Fatalf("collectIncludes: %v", err)
	}

	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d: %v", len(files), files)
	}

	if !strings.HasSuffix(files[0], "root.v2.spec") {
		t.Errorf("first file should be root, got %s", files[0])
	}
	if !strings.HasSuffix(files[1], "scopes.v2.spec") {
		t.Errorf("second file should be scopes, got %s", files[1])
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

// TestTransformBodyRefs_GivenBlock verifies body refs in given blocks are converted.
func TestTransformBodyRefs_GivenBlock(t *testing.T) {
	t.Parallel()

	steps := []spec.GivenStep{
		&spec.Call{Namespace: "http", Method: "post", Args: []spec.Expr{
			spec.LiteralString{Value: "/api/groups"},
			spec.ObjectLiteral{Fields: []*spec.ObjField{{Key: "name", Value: spec.LiteralString{Value: "Test"}}}},
		}},
		&spec.Call{Namespace: "http", Method: "post", Args: []spec.Expr{
			spec.BinaryOp{
				Left:  spec.LiteralString{Value: "/api/groups/"},
				Op:    "+",
				Right: spec.FieldRef{Path: "body.group.id"},
			},
		}},
	}

	result := transformBodyRefs(steps, "http")
	if len(result) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(result))
	}

	// First step should be wrapped in a let binding.
	if _, ok := result[0].(*spec.LetBinding); !ok {
		t.Errorf("expected first step to be LetBinding, got %T", result[0])
	}
}

// TestTransformBodyRefs_Assignment verifies body refs in assignment values are converted.
func TestTransformBodyRefs_Assignment(t *testing.T) {
	t.Parallel()

	steps := []spec.GivenStep{
		&spec.Call{Namespace: "http", Method: "post", Args: []spec.Expr{
			spec.LiteralString{Value: "/api/groups"},
		}},
		&spec.Assignment{Path: "group_id", Value: spec.FieldRef{Path: "body.group.id"}},
	}

	result := transformBodyRefs(steps, "http")

	// First step should be let binding.
	lb, ok := result[0].(*spec.LetBinding)
	if !ok {
		t.Fatalf("expected LetBinding, got %T", result[0])
	}

	// Assignment should reference the let variable, not body.
	assign, ok := result[1].(*spec.Assignment)
	if !ok {
		t.Fatalf("expected Assignment, got %T", result[1])
	}
	ref, ok := assign.Value.(spec.FieldRef)
	if !ok {
		t.Fatalf("expected FieldRef value, got %T", assign.Value)
	}
	if !strings.HasPrefix(ref.Path, lb.Name+".") {
		t.Errorf("assignment value %q should reference %q", ref.Path, lb.Name)
	}
}

// TestTransformBodyRefs_NonAdjacent verifies body ref conversion works with intervening steps.
func TestTransformBodyRefs_NonAdjacent(t *testing.T) {
	t.Parallel()

	steps := []spec.GivenStep{
		&spec.Call{Namespace: "http", Method: "post", Args: []spec.Expr{
			spec.LiteralString{Value: "/api/init"},
		}},
		&spec.Assignment{Path: "name", Value: spec.LiteralString{Value: "test"}},
		// body ref is 2 steps after the call
		&spec.Call{Namespace: "http", Method: "post", Args: []spec.Expr{
			spec.FieldRef{Path: "body.token"},
		}},
	}

	result := transformBodyRefs(steps, "http")

	// First step should be wrapped in let binding.
	if _, ok := result[0].(*spec.LetBinding); !ok {
		t.Errorf("expected first step to be LetBinding, got %T", result[0])
	}
	// Assignment should be unchanged.
	if _, ok := result[1].(*spec.Assignment); !ok {
		t.Errorf("expected second step to be Assignment, got %T", result[1])
	}
}

// TestTransformBodyRefs_MultipleRefs verifies unique variable names for multiple calls.
func TestTransformBodyRefs_MultipleRefs(t *testing.T) {
	t.Parallel()

	steps := []spec.GivenStep{
		&spec.Call{Namespace: "http", Method: "post", Args: []spec.Expr{
			spec.LiteralString{Value: "/api/first"},
		}},
		&spec.Assignment{Path: "first_id", Value: spec.FieldRef{Path: "body.id"}},
		&spec.Call{Namespace: "http", Method: "post", Args: []spec.Expr{
			spec.LiteralString{Value: "/api/second"},
		}},
		&spec.Assignment{Path: "second_id", Value: spec.FieldRef{Path: "body.id"}},
	}

	result := transformBodyRefs(steps, "http")

	// Both calls should be wrapped with different variable names.
	lb0, ok0 := result[0].(*spec.LetBinding)
	lb2, ok2 := result[2].(*spec.LetBinding)
	if !ok0 || !ok2 {
		t.Fatalf("expected both calls wrapped, got types: %T, %T", result[0], result[2])
	}
	if lb0.Name == lb2.Name {
		t.Errorf("variable names should differ, both are %q", lb0.Name)
	}
}

// TestBuildPathExpr verifies URL template to string concatenation conversion.
func TestBuildPathExpr(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		path string
		want string
	}{
		{"plain path", "/api/users", `"/api/users"`},
		{"trailing param", "/api/groups/:id", `"/api/groups/" + id`},
		{"middle param", "/api/groups/:id/leave", `"/api/groups/" + id + "/leave"`},
		{"multiple params", "/api/:org/groups/:id", `"/api/" + org + "/groups/" + id`},
		{"leading param", ":version/api", `version + "/api"`},
		{"no slashes", ":id", `id`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := buildPathExpr(tt.path)
			if got != tt.want {
				t.Errorf("buildPathExpr(%q) = %q, want %q", tt.path, got, tt.want)
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
