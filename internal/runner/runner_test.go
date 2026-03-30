package runner_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bamsammich/speclang/v3/internal/adapter"
	"github.com/bamsammich/speclang/v3/internal/parser"
	"github.com/bamsammich/speclang/v3/internal/runner"
)

// writeJSON encodes v as JSON to w, falling back to a 500 error if encoding fails.
func writeJSON(w http.ResponseWriter, v any) {
	if err := json.NewEncoder(w).Encode(v); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func transferHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		From struct {
			ID      string `json:"id"`
			Balance int    `json:"balance"`
		} `json:"from"`
		To struct {
			ID      string `json:"id"`
			Balance int    `json:"balance"`
		} `json:"to"`
		Amount int `json:"amount"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	resp := map[string]any{
		"from":  map[string]any{"id": req.From.ID, "balance": req.From.Balance},
		"to":    map[string]any{"id": req.To.ID, "balance": req.To.Balance},
		"error": nil,
	}

	switch {
	case req.Amount <= 0:
		resp["error"] = "invalid_amount"
	case req.Amount > req.From.Balance:
		resp["error"] = "insufficient_funds"
	default:
		resp["from"] = map[string]any{"id": req.From.ID, "balance": req.From.Balance - req.Amount}
		resp["to"] = map[string]any{"id": req.To.ID, "balance": req.To.Balance + req.Amount}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func TestVerify_ScopeResults(t *testing.T) {
	spec, err := parser.ParseFile("../../examples/transfer.spec")
	if err != nil {
		t.Fatalf("parsing spec: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/accounts/transfer", transferHandler)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	adp, err := adapter.NewHTTPAdapter()
	if err != nil {
		t.Fatal(err)
	}
	if err := adp.Init(map[string]string{"base_url": srv.URL}); err != nil {
		t.Fatal(err)
	}

	r := runner.New(spec, map[string]adapter.Adapter{"http": adp}, 42)
	r.SetN(10)
	res, err := r.Verify()
	if err != nil {
		t.Fatalf("verify: %v", err)
	}

	if len(res.Scopes) != 1 {
		t.Fatalf("expected 1 scope, got %d", len(res.Scopes))
	}

	scope := res.Scopes[0]
	if scope.Name != "transfer" {
		t.Errorf("expected scope name 'transfer', got %q", scope.Name)
	}

	// 3 scenarios + 3 invariants = 6 checks
	if len(scope.Checks) != 6 {
		t.Fatalf("expected 6 checks, got %d", len(scope.Checks))
	}

	for _, check := range scope.Checks {
		if !check.Passed {
			t.Errorf("check %q (%s) failed", check.Name, check.Kind)
		}
		if check.InputsRun < 1 {
			t.Errorf("check %q has InputsRun=%d, expected >= 1", check.Name, check.InputsRun)
		}
	}

	// Verify the first check is a scenario
	if scope.Checks[0].Kind != "scenario" {
		t.Errorf("expected first check to be scenario, got %q", scope.Checks[0].Kind)
	}
}

func TestRelationalAssertions(t *testing.T) {
	t.Parallel()

	// Build a spec with relational then-assertions programmatically.
	spec := &parser.Spec{
		Name: "RelTest",
		Scopes: []*parser.Scope{{
			Name: "math",
			Use:  "http",
			Config: map[string]parser.Expr{
				"path":   parser.LiteralString{Value: "/add"},
				"method": parser.LiteralString{Value: "POST"},
			},
			Scenarios: []*parser.Scenario{{
				Name: "relational",
				Given: &parser.Block{
					Steps: []parser.GivenStep{
						&parser.Assignment{Path: "a", Value: parser.LiteralInt{Value: 7}},
						&parser.Assignment{Path: "b", Value: parser.LiteralInt{Value: 3}},
					},
				},
				Then: &parser.Block{
					Assertions: []*parser.Assertion{
						{
							Target: "sum",
							Expected: parser.BinaryOp{
								Left:  parser.FieldRef{Path: "a"},
								Op:    "+",
								Right: parser.FieldRef{Path: "b"},
							},
						},
					},
				},
			}},
		}},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /add", func(w http.ResponseWriter, r *http.Request) {
		var req map[string]int
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).
			Encode(map[string]int{"sum": req["a"] + req["b"]}); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	adp, err := adapter.NewHTTPAdapter()
	if err != nil {
		t.Fatal(err)
	}
	if err := adp.Init(map[string]string{"base_url": srv.URL}); err != nil {
		t.Fatal(err)
	}

	r := runner.New(spec, map[string]adapter.Adapter{"http": adp}, 1)
	res, err := r.Verify()
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if len(res.Failures) > 0 {
		t.Errorf("expected no failures, got %v", res.Failures[0].Description)
	}
}

func TestEnvRefInGivenBlock(t *testing.T) {
	t.Setenv("SPECTEST_RUNNER_VAL", "resolved_value")

	sp := &parser.Spec{
		Name: "EnvRefTest",
		Scopes: []*parser.Scope{{
			Name:   "env_scope",
			Use:    "test",
			Config: map[string]parser.Expr{},
			Contract: &parser.Contract{
				Input:  []*parser.Field{{Name: "file", Type: parser.TypeExpr{Name: "string"}}},
				Output: []*parser.Field{{Name: "result", Type: parser.TypeExpr{Name: "string"}}},
			},
			Scenarios: []*parser.Scenario{{
				Name: "env_given",
				Given: &parser.Block{
					Steps: []parser.GivenStep{
						&parser.Assignment{
							Path:  "file",
							Value: parser.EnvRef{Var: "SPECTEST_RUNNER_VAL", Default: "fallback"},
						},
					},
				},
				Then: &parser.Block{
					Assertions: []*parser.Assertion{
						{Target: "result", Expected: parser.LiteralString{Value: "ok"}},
					},
				},
			}},
		}},
	}

	mock := &mockAdapter{}
	r := runner.New(sp, map[string]adapter.Adapter{"test": mock}, 1)
	_, err := r.Verify()
	if err != nil {
		t.Fatalf("verify: %v", err)
	}

	// Find the exec call (first call). The assertion query is a second call.
	if len(mock.calls) < 1 {
		t.Fatalf("expected at least 1 action call, got %d", len(mock.calls))
	}

	// The exec action should contain "resolved_value" from the env var.
	execCall := mock.calls[0]
	args := string(execCall.Args)
	if !json.Valid(execCall.Args) {
		t.Fatalf("invalid JSON in action args: %s", args)
	}
	// The process adapter receives exec args as a JSON array.
	// The input field "file" should have the resolved env value.
	var execArgs []any
	if err := json.Unmarshal(execCall.Args, &execArgs); err != nil {
		t.Fatalf("unmarshal args: %v", err)
	}
	found := false
	for _, a := range execArgs {
		if s, ok := a.(string); ok && s == "resolved_value" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected env-resolved value 'resolved_value' in args, got %s", args)
	}
}

func TestEnvRefInConfigBlock(t *testing.T) {
	t.Setenv("SPECTEST_CONFIG_ARGS", "parse")

	sp := &parser.Spec{
		Name: "EnvConfigTest",
		Scopes: []*parser.Scope{{
			Name: "env_config",
			Use:  "test",
			Config: map[string]parser.Expr{
				"args": parser.EnvRef{Var: "SPECTEST_CONFIG_ARGS", Default: "help"},
			},
			Contract: &parser.Contract{
				Input:  []*parser.Field{{Name: "file", Type: parser.TypeExpr{Name: "string"}}},
				Output: []*parser.Field{{Name: "result", Type: parser.TypeExpr{Name: "string"}}},
			},
			Scenarios: []*parser.Scenario{{
				Name: "env_config_scenario",
				Given: &parser.Block{
					Steps: []parser.GivenStep{
						&parser.Assignment{
							Path:  "file",
							Value: parser.LiteralString{Value: "test.spec"},
						},
					},
				},
				Then: &parser.Block{
					Assertions: []*parser.Assertion{
						{Target: "result", Expected: parser.LiteralString{Value: "ok"}},
					},
				},
			}},
		}},
	}

	mock := &mockAdapter{}
	r := runner.New(sp, map[string]adapter.Adapter{"test": mock}, 1)
	_, err := r.Verify()
	if err != nil {
		t.Fatalf("verify: %v", err)
	}

	// Find the exec call (first call). The assertion query is a second call.
	if len(mock.calls) < 1 {
		t.Fatalf("expected at least 1 action call, got %d", len(mock.calls))
	}

	// The exec args should start with "parse" from the env-resolved config.
	var execArgs []any
	if err := json.Unmarshal(mock.calls[0].Args, &execArgs); err != nil {
		t.Fatalf("unmarshal args: %v", err)
	}
	if len(execArgs) < 1 {
		t.Fatal("expected at least 1 exec arg")
	}
	if execArgs[0] != "parse" {
		t.Errorf("first exec arg = %v, want 'parse'", execArgs[0])
	}
}

func TestCollectExecArgs_ArrayConfig(t *testing.T) {
	t.Parallel()

	sp := &parser.Spec{
		Name: "ArrayArgsTest",
		Scopes: []*parser.Scope{{
			Name: "arr_scope",
			Use:  "test",
			Config: map[string]parser.Expr{
				"args": parser.ArrayLiteral{
					Elements: []parser.Expr{
						parser.LiteralString{Value: "verify"},
						parser.LiteralString{Value: "--json"},
						parser.LiteralString{Value: "path with spaces/file.spec"},
					},
				},
			},
			Contract: &parser.Contract{
				Input:  []*parser.Field{{Name: "x", Type: parser.TypeExpr{Name: "int"}}},
				Output: []*parser.Field{{Name: "result", Type: parser.TypeExpr{Name: "string"}}},
			},
			Scenarios: []*parser.Scenario{{
				Name: "array_args_scenario",
				Given: &parser.Block{
					Steps: []parser.GivenStep{
						&parser.Assignment{
							Path:  "x",
							Value: parser.LiteralInt{Value: 1},
						},
					},
				},
				Then: &parser.Block{
					Assertions: []*parser.Assertion{
						{Target: "result", Expected: parser.LiteralString{Value: "ok"}},
					},
				},
			}},
		}},
	}

	mock := &mockAdapter{}
	r := runner.New(sp, map[string]adapter.Adapter{"test": mock}, 1)
	_, err := r.Verify()
	if err != nil {
		t.Fatalf("verify: %v", err)
	}

	// Find the exec call (first call). The assertion query is a second call.
	if len(mock.calls) < 1 {
		t.Fatalf("expected at least 1 action call, got %d", len(mock.calls))
	}

	var execArgs []any
	if err := json.Unmarshal(mock.calls[0].Args, &execArgs); err != nil {
		t.Fatalf("unmarshal args: %v", err)
	}

	// Array form: 3 config args + 1 input field = 4 total
	if len(execArgs) != 4 {
		t.Fatalf("expected 4 exec args, got %d: %v", len(execArgs), execArgs)
	}
	if execArgs[0] != "verify" {
		t.Errorf("arg 0 = %v, want 'verify'", execArgs[0])
	}
	if execArgs[1] != "--json" {
		t.Errorf("arg 1 = %v, want '--json'", execArgs[1])
	}
	// The space-containing path must be preserved as a single argument
	if execArgs[2] != "path with spaces/file.spec" {
		t.Errorf("arg 2 = %v, want 'path with spaces/file.spec'", execArgs[2])
	}
}

// mockAdapter records Call invocations for testing.
type mockAdapter struct {
	calls []mockCall
}

type mockCall struct {
	Method string
	Args   json.RawMessage
}

func (m *mockAdapter) Init(config map[string]string) error { return nil }
func (m *mockAdapter) Call(method string, args json.RawMessage) (*adapter.Response, error) {
	m.calls = append(m.calls, mockCall{Method: method, Args: args})
	return &adapter.Response{OK: true, Actual: json.RawMessage(`{}`)}, nil
}
func (m *mockAdapter) Reset() error { return nil }
func (m *mockAdapter) Close() error { return nil }


func TestLocatorResolution(t *testing.T) {
	t.Parallel()

	spec := &parser.Spec{
		Name: "LocatorTest",
		Locators: map[string]string{
			"welcome": "[data-testid=welcome]",
		},
		Scopes: []*parser.Scope{{
			Name:   "ui",
			Use:    "playwright",
			Config: map[string]parser.Expr{},
			Scenarios: []*parser.Scenario{{
				Name: "check_visible",
				Given: &parser.Block{
					Steps: []parser.GivenStep{
						&parser.Assignment{Path: "x", Value: parser.LiteralInt{Value: 1}},
					},
				},
				Then: &parser.Block{
					Assertions: []*parser.Assertion{
						{
							Target:   "welcome",
							Plugin:   "playwright",
							Property: "visible",
							Expected: parser.LiteralBool{Value: true},
						},
					},
				},
			}},
		}},
	}

	mock := &mockAdapter{}
	r := runner.New(spec, map[string]adapter.Adapter{"playwright": mock}, 1)
	_, err := r.Verify()
	if err != nil {
		t.Fatalf("verify: %v", err)
	}

	// The assertion goes through Call("visible", args) where args contains the selector.
	// Find the "visible" call in mock.calls.
	var visibleCall *mockCall
	for i := range mock.calls {
		if mock.calls[i].Method == "visible" {
			visibleCall = &mock.calls[i]
			break
		}
	}
	if visibleCall == nil {
		t.Fatal("expected a 'visible' call, found none")
	}

	var callArgs []string
	if err := json.Unmarshal(visibleCall.Args, &callArgs); err != nil {
		t.Fatalf("unmarshal visible args: %v", err)
	}
	if len(callArgs) != 1 || callArgs[0] != "[data-testid=welcome]" {
		t.Errorf("visible args = %v, want [[data-testid=welcome]]", callArgs)
	}
}

func TestLocatorResolution_MissingLocator(t *testing.T) {
	t.Parallel()

	spec := &parser.Spec{
		Name: "LocatorTest",
		// No locators defined
		Scopes: []*parser.Scope{{
			Name:   "ui",
			Use:    "playwright",
			Config: map[string]parser.Expr{},
			Scenarios: []*parser.Scenario{{
				Name: "missing",
				Given: &parser.Block{
					Steps: []parser.GivenStep{
						&parser.Assignment{Path: "x", Value: parser.LiteralInt{Value: 1}},
					},
				},
				Then: &parser.Block{
					Assertions: []*parser.Assertion{
						{
							Target:   "nonexistent",
							Plugin:   "playwright",
							Property: "visible",
							Expected: parser.LiteralBool{Value: true},
						},
					},
				},
			}},
		}},
	}

	mock := &mockAdapter{}
	r := runner.New(spec, map[string]adapter.Adapter{"playwright": mock}, 1)
	_, err := r.Verify()
	if err == nil {
		t.Fatal("expected error for missing locator, got nil")
	}
}

func TestGivenStepExecution(t *testing.T) {
	t.Parallel()

	// Spec with mixed given steps: calls and assignments, executed in order.
	spec := &parser.Spec{
		Name: "StepTest",
		Locators: map[string]string{
			"username": "[data-testid=username]",
			"submit":   "[data-testid=submit]",
			"welcome":  "[data-testid=welcome]",
		},
		Scopes: []*parser.Scope{{
			Name:   "login",
			Use:    "playwright",
			Config: map[string]parser.Expr{},
			Scenarios: []*parser.Scenario{{
				Name: "login_flow",
				Given: &parser.Block{
					Steps: []parser.GivenStep{
						// playwright.fill(username, "alice")
						&parser.Call{
							Namespace: "playwright",
							Method:    "fill",
							Args: []parser.Expr{
								parser.FieldRef{Path: "username"},
								parser.LiteralString{Value: "alice"},
							},
						},
						// user: "alice"
						&parser.Assignment{
							Path:  "user",
							Value: parser.LiteralString{Value: "alice"},
						},
						// playwright.click(submit)
						&parser.Call{
							Namespace: "playwright",
							Method:    "click",
							Args: []parser.Expr{
								parser.FieldRef{Path: "submit"},
							},
						},
					},
				},
				Then: &parser.Block{
					Assertions: []*parser.Assertion{
						{
							Target:   "welcome",
							Plugin:   "playwright",
							Property: "visible",
							Expected: parser.LiteralBool{Value: true},
						},
					},
				},
			}},
		}},
	}

	mock := &mockAdapter{}
	r := runner.New(spec, map[string]adapter.Adapter{"playwright": mock}, 1)
	_, err := r.Verify()
	if err != nil {
		t.Fatalf("verify: %v", err)
	}

	// Verify calls executed in order: fill, click, then visible (assertion)
	if len(mock.calls) < 3 {
		t.Fatalf("expected at least 3 calls, got %d", len(mock.calls))
	}

	// First call: fill(username_selector, "alice")
	if mock.calls[0].Method != "fill" {
		t.Errorf("call 0: method = %q, want 'fill'", mock.calls[0].Method)
	}
	var fillArgs []any
	if err := json.Unmarshal(mock.calls[0].Args, &fillArgs); err != nil {
		t.Fatalf("unmarshal fill args: %v", err)
	}
	if len(fillArgs) != 2 {
		t.Fatalf("fill: expected 2 args, got %d", len(fillArgs))
	}
	if fillArgs[0] != "[data-testid=username]" {
		t.Errorf("fill arg 0 = %q, want '[data-testid=username]'", fillArgs[0])
	}
	if fillArgs[1] != "alice" {
		t.Errorf("fill arg 1 = %q, want 'alice'", fillArgs[1])
	}

	// Second call: click(submit_selector)
	if mock.calls[1].Method != "click" {
		t.Errorf("action 1: name = %q, want 'click'", mock.calls[1].Method)
	}
	var clickArgs []any
	if err := json.Unmarshal(mock.calls[1].Args, &clickArgs); err != nil {
		t.Fatalf("unmarshal click args: %v", err)
	}
	if len(clickArgs) != 1 || clickArgs[0] != "[data-testid=submit]" {
		t.Errorf("click args = %v, want [[data-testid=submit]]", clickArgs)
	}

	// Verify assertion — the "visible" query goes through Call too.
	var visibleCall *mockCall
	for i := range mock.calls {
		if mock.calls[i].Method == "visible" {
			visibleCall = &mock.calls[i]
			break
		}
	}
	if visibleCall == nil {
		t.Fatal("expected a 'visible' call, found none")
	}
	var assertArgs []string
	if err := json.Unmarshal(visibleCall.Args, &assertArgs); err != nil {
		t.Fatalf("unmarshal visible args: %v", err)
	}
	if len(assertArgs) != 1 || assertArgs[0] != "[data-testid=welcome]" {
		t.Errorf("visible args = %v, want [[data-testid=welcome]]", assertArgs)
	}
}

func TestMultiStepHTTPGivenBlock(t *testing.T) {
	t.Parallel()

	// Spec with multi-step HTTP given block: POST to create, then GET to verify.
	spec := &parser.Spec{
		Name: "MultiStepHTTP",
		Scopes: []*parser.Scope{{
			Name:   "workflow",
			Use:    "http",
			Config: map[string]parser.Expr{},
			Scenarios: []*parser.Scenario{{
				Name: "create_then_verify",
				Given: &parser.Block{
					Steps: []parser.GivenStep{
						// http.post("/api/resources", { name: "widget" })
						&parser.Call{
							Namespace: "http",
							Method:    "post",
							Args: []parser.Expr{
								parser.LiteralString{Value: "/api/resources"},
								parser.ObjectLiteral{Fields: []*parser.ObjField{
									{Key: "name", Value: parser.LiteralString{Value: "widget"}},
								}},
							},
						},
						// http.get("/api/resources/1")
						&parser.Call{
							Namespace: "http",
							Method:    "get",
							Args: []parser.Expr{
								parser.LiteralString{Value: "/api/resources/1"},
							},
						},
						// name: "widget" (for assertion evaluation)
						&parser.Assignment{
							Path:  "name",
							Value: parser.LiteralString{Value: "widget"},
						},
					},
				},
				Then: &parser.Block{
					Assertions: []*parser.Assertion{
						{Target: "name", Expected: parser.FieldRef{Path: "name"}},
						{Target: "id", Expected: parser.LiteralInt{Value: 1}},
					},
				},
			}},
		}},
	}

	// Server with state: POST creates, GET retrieves
	var created map[string]any
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/resources", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		created = map[string]any{"id": 1, "name": body["name"]}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(201)
		if err := json.NewEncoder(w).Encode(created); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})
	mux.HandleFunc("GET /api/resources/1", func(w http.ResponseWriter, _ *http.Request) {
		if created == nil {
			w.WriteHeader(404)
			if _, err := w.Write([]byte(`{"error":"not_found"}`)); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(created); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	adp, err := adapter.NewHTTPAdapter()
	if err != nil {
		t.Fatal(err)
	}
	if err := adp.Init(map[string]string{"base_url": srv.URL}); err != nil {
		t.Fatal(err)
	}

	r := runner.New(spec, map[string]adapter.Adapter{"http": adp}, 1)
	res, err := r.Verify()
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if len(res.Failures) > 0 {
		t.Errorf("expected no failures, got: %s", res.Failures[0].Description)
	}
	if res.ScenariosRun != 1 || res.ScenariosPassed != 1 {
		t.Errorf("expected 1/1 scenarios passed, got %d/%d", res.ScenariosPassed, res.ScenariosRun)
	}
}

func TestMultiStepHTTPHeaderPersistence(t *testing.T) {
	t.Parallel()

	spec := &parser.Spec{
		Name: "HeaderPersist",
		Scopes: []*parser.Scope{{
			Name:   "auth_flow",
			Use:    "http",
			Config: map[string]parser.Expr{},
			Scenarios: []*parser.Scenario{{
				Name: "headers_persist",
				Given: &parser.Block{
					Steps: []parser.GivenStep{
						// http.header("Authorization", "Bearer tok")
						&parser.Call{
							Namespace: "http",
							Method:    "header",
							Args: []parser.Expr{
								parser.LiteralString{Value: "Authorization"},
								parser.LiteralString{Value: "Bearer tok"},
							},
						},
						// http.get("/api/echo-headers")
						&parser.Call{
							Namespace: "http",
							Method:    "get",
							Args: []parser.Expr{
								parser.LiteralString{Value: "/api/echo-headers"},
							},
						},
					},
				},
				Then: &parser.Block{
					Assertions: []*parser.Assertion{
						{Target: "auth", Expected: parser.LiteralString{Value: "Bearer tok"}},
					},
				},
			}},
		}},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/echo-headers", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		writeJSON(w, map[string]any{
			"auth": r.Header.Get("Authorization"),
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	adp, err := adapter.NewHTTPAdapter()
	if err != nil {
		t.Fatal(err)
	}
	if err := adp.Init(map[string]string{"base_url": srv.URL}); err != nil {
		t.Fatal(err)
	}

	r := runner.New(spec, map[string]adapter.Adapter{"http": adp}, 1)
	res, err := r.Verify()
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if len(res.Failures) > 0 {
		t.Errorf("expected no failures, got: %s", res.Failures[0].Description)
	}
}

func TestVerifyTransferSpec(t *testing.T) {
	spec, err := parser.ParseFile("../../examples/transfer.spec")
	if err != nil {
		t.Fatalf("parsing spec: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/accounts/transfer", transferHandler)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	adp, err := adapter.NewHTTPAdapter()
	if err != nil {
		t.Fatalf("create adapter: %v", err)
	}
	if err := adp.Init(map[string]string{"base_url": srv.URL}); err != nil {
		t.Fatalf("init adapter: %v", err)
	}

	r := runner.New(spec, map[string]adapter.Adapter{"http": adp}, 42)
	r.SetN(100)

	res, err := r.Verify()
	if err != nil {
		t.Fatalf("verify: %v", err)
	}

	if len(res.Failures) > 0 {
		for _, f := range res.Failures {
			t.Errorf("failure in %q (scope %q): %s (input=%v, expected=%v, actual=%v)",
				f.Name, f.Scope, f.Description, f.Input, f.Expected, f.Actual)
		}
	}

	if res.Spec != "AccountAPI" {
		t.Errorf("expected spec name AccountAPI, got %q", res.Spec)
	}
	if res.ScenariosRun != 3 {
		t.Errorf("expected 3 scenarios run, got %d", res.ScenariosRun)
	}
	if res.ScenariosPassed != 3 {
		t.Errorf("expected 3 scenarios passed, got %d", res.ScenariosPassed)
	}
	if res.InvariantsChecked != 3 {
		t.Errorf("expected 3 invariants checked, got %d", res.InvariantsChecked)
	}
	if res.InvariantsPassed != 3 {
		t.Errorf("expected 3 invariants passed, got %d", res.InvariantsPassed)
	}
}

// failingAdapter returns {ok: false} on Call invocations with a configurable error message.
type failingAdapter struct {
	errorMsg string
}

func (f *failingAdapter) Init(config map[string]string) error { return nil }
func (f *failingAdapter) Call(method string, args json.RawMessage) (*adapter.Response, error) {
	return &adapter.Response{OK: false, Error: f.errorMsg}, nil
}
func (f *failingAdapter) Reset() error { return nil }
func (f *failingAdapter) Close() error { return nil }

func TestErrorPseudoField_GivenScenario_ExpectedError(t *testing.T) {
	t.Parallel()

	// Scenario expects an error and the adapter returns one — should pass.
	spec := &parser.Spec{
		Name: "ErrorTest",
		Scopes: []*parser.Scope{{
			Name: "fail_scope",
			Use:  "test",
			Config: map[string]parser.Expr{
				"path":   parser.LiteralString{Value: "/fail"},
				"method": parser.LiteralString{Value: "POST"},
			},
			Contract: &parser.Contract{
				Input: []*parser.Field{
					{Name: "x", Type: parser.TypeExpr{Name: "int"}},
				},
				// No "error" in output — triggers pseudo-field behavior.
				Output: []*parser.Field{
					{Name: "result", Type: parser.TypeExpr{Name: "string"}},
				},
			},
			Scenarios: []*parser.Scenario{{
				Name: "expect_failure",
				Given: &parser.Block{
					Steps: []parser.GivenStep{
						&parser.Assignment{Path: "x", Value: parser.LiteralInt{Value: 1}},
					},
				},
				Then: &parser.Block{
					Assertions: []*parser.Assertion{
						{
							Target:   "error",
							Expected: parser.LiteralString{Value: "something went wrong"},
						},
					},
				},
			}},
		}},
	}

	adp := &failingAdapter{errorMsg: "something went wrong"}
	r := runner.New(spec, map[string]adapter.Adapter{"test": adp}, 1)
	res, err := r.Verify()
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if len(res.Failures) > 0 {
		t.Errorf("expected no failures, got: %s", res.Failures[0].Description)
	}
	if res.ScenariosPassed != 1 {
		t.Errorf("expected 1 scenario passed, got %d", res.ScenariosPassed)
	}
}

func TestErrorPseudoField_GivenScenario_ExpectedNull(t *testing.T) {
	t.Parallel()

	// Scenario asserts error: null but no error occurs — should pass.
	spec := &parser.Spec{
		Name: "ErrorNullTest",
		Scopes: []*parser.Scope{{
			Name: "ok_scope",
			Use:  "test",
			Config: map[string]parser.Expr{
				"path":   parser.LiteralString{Value: "/ok"},
				"method": parser.LiteralString{Value: "POST"},
			},
			Contract: &parser.Contract{
				Input:  []*parser.Field{{Name: "x", Type: parser.TypeExpr{Name: "int"}}},
				Output: []*parser.Field{{Name: "result", Type: parser.TypeExpr{Name: "string"}}},
			},
			Scenarios: []*parser.Scenario{{
				Name: "no_error",
				Given: &parser.Block{
					Steps: []parser.GivenStep{
						&parser.Assignment{Path: "x", Value: parser.LiteralInt{Value: 1}},
					},
				},
				Then: &parser.Block{
					Assertions: []*parser.Assertion{
						{Target: "error", Expected: parser.LiteralNull{}},
					},
				},
			}},
		}},
	}

	mock := &mockAdapter{}
	r := runner.New(spec, map[string]adapter.Adapter{"test": mock}, 1)
	res, err := r.Verify()
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if len(res.Failures) > 0 {
		t.Errorf("expected no failures, got: %s", res.Failures[0].Description)
	}
}

func TestErrorPseudoField_GivenScenario_UnexpectedError(t *testing.T) {
	t.Parallel()

	// Scenario asserts error: null but an error occurs — should fail the assertion.
	spec := &parser.Spec{
		Name: "ErrorUnexpectedTest",
		Scopes: []*parser.Scope{{
			Name: "err_scope",
			Use:  "test",
			Config: map[string]parser.Expr{
				"path":   parser.LiteralString{Value: "/err"},
				"method": parser.LiteralString{Value: "POST"},
			},
			Contract: &parser.Contract{
				Input:  []*parser.Field{{Name: "x", Type: parser.TypeExpr{Name: "int"}}},
				Output: []*parser.Field{{Name: "result", Type: parser.TypeExpr{Name: "string"}}},
			},
			Scenarios: []*parser.Scenario{{
				Name: "unexpected_error",
				Given: &parser.Block{
					Steps: []parser.GivenStep{
						&parser.Assignment{Path: "x", Value: parser.LiteralInt{Value: 1}},
					},
				},
				Then: &parser.Block{
					Assertions: []*parser.Assertion{
						{Target: "error", Expected: parser.LiteralNull{}},
					},
				},
			}},
		}},
	}

	adp := &failingAdapter{errorMsg: "oops"}
	r := runner.New(spec, map[string]adapter.Adapter{"test": adp}, 1)
	res, err := r.Verify()
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if len(res.Failures) != 1 {
		t.Fatalf("expected 1 failure, got %d", len(res.Failures))
	}
	if res.Failures[0].Expected != "null" {
		t.Errorf("expected null, got %q", res.Failures[0].Expected)
	}
}

func TestErrorPseudoField_NoAssertion_ActionFails(t *testing.T) {
	t.Parallel()

	// Action fails but there's no error assertion — should be a test error.
	spec := &parser.Spec{
		Name: "ErrorNoAssertTest",
		Scopes: []*parser.Scope{{
			Name: "err_scope",
			Use:  "test",
			Config: map[string]parser.Expr{
				"path":   parser.LiteralString{Value: "/err"},
				"method": parser.LiteralString{Value: "POST"},
			},
			Contract: &parser.Contract{
				Input:  []*parser.Field{{Name: "x", Type: parser.TypeExpr{Name: "int"}}},
				Output: []*parser.Field{{Name: "result", Type: parser.TypeExpr{Name: "string"}}},
			},
			Scenarios: []*parser.Scenario{{
				Name: "no_error_assertion",
				Given: &parser.Block{
					Steps: []parser.GivenStep{
						&parser.Assignment{Path: "x", Value: parser.LiteralInt{Value: 1}},
					},
				},
				// No then block with error assertion.
			}},
		}},
	}

	adp := &failingAdapter{errorMsg: "oops"}
	r := runner.New(spec, map[string]adapter.Adapter{"test": adp}, 1)
	_, err := r.Verify()
	if err == nil {
		t.Fatal("expected error when action fails without error assertion, got nil")
	}
}

func TestErrorPseudoField_WrongMessage(t *testing.T) {
	t.Parallel()

	// Scenario expects error "foo" but gets "bar" — should fail assertion.
	spec := &parser.Spec{
		Name: "ErrorMismatchTest",
		Scopes: []*parser.Scope{{
			Name: "mismatch_scope",
			Use:  "test",
			Config: map[string]parser.Expr{
				"path":   parser.LiteralString{Value: "/fail"},
				"method": parser.LiteralString{Value: "POST"},
			},
			Contract: &parser.Contract{
				Input:  []*parser.Field{{Name: "x", Type: parser.TypeExpr{Name: "int"}}},
				Output: []*parser.Field{{Name: "result", Type: parser.TypeExpr{Name: "string"}}},
			},
			Scenarios: []*parser.Scenario{{
				Name: "wrong_error",
				Given: &parser.Block{
					Steps: []parser.GivenStep{
						&parser.Assignment{Path: "x", Value: parser.LiteralInt{Value: 1}},
					},
				},
				Then: &parser.Block{
					Assertions: []*parser.Assertion{
						{Target: "error", Expected: parser.LiteralString{Value: "expected_error"}},
					},
				},
			}},
		}},
	}

	adp := &failingAdapter{errorMsg: "different_error"}
	r := runner.New(spec, map[string]adapter.Adapter{"test": adp}, 1)
	res, err := r.Verify()
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if len(res.Failures) != 1 {
		t.Fatalf("expected 1 failure, got %d", len(res.Failures))
	}
	if res.Failures[0].Expected != `"expected_error"` {
		t.Errorf("expected %q, got %q", `"expected_error"`, res.Failures[0].Expected)
	}
	if res.Failures[0].Actual != `"different_error"` {
		t.Errorf("actual %q, got %q", `"different_error"`, res.Failures[0].Actual)
	}
}

func TestErrorPseudoField_ExpectedErrorButNoneOccurred(t *testing.T) {
	t.Parallel()

	// Scenario asserts error: "foo" but no error occurs — should fail.
	spec := &parser.Spec{
		Name: "ErrorExpectedButNone",
		Scopes: []*parser.Scope{{
			Name: "no_err_scope",
			Use:  "test",
			Config: map[string]parser.Expr{
				"path":   parser.LiteralString{Value: "/ok"},
				"method": parser.LiteralString{Value: "POST"},
			},
			Contract: &parser.Contract{
				Input:  []*parser.Field{{Name: "x", Type: parser.TypeExpr{Name: "int"}}},
				Output: []*parser.Field{{Name: "result", Type: parser.TypeExpr{Name: "string"}}},
			},
			Scenarios: []*parser.Scenario{{
				Name: "expected_error_missing",
				Given: &parser.Block{
					Steps: []parser.GivenStep{
						&parser.Assignment{Path: "x", Value: parser.LiteralInt{Value: 1}},
					},
				},
				Then: &parser.Block{
					Assertions: []*parser.Assertion{
						{Target: "error", Expected: parser.LiteralString{Value: "should_fail"}},
					},
				},
			}},
		}},
	}

	mock := &mockAdapter{}
	r := runner.New(spec, map[string]adapter.Adapter{"test": mock}, 1)
	res, err := r.Verify()
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if len(res.Failures) != 1 {
		t.Fatalf("expected 1 failure, got %d", len(res.Failures))
	}
}

func TestErrorPseudoField_WithGivenCalls(t *testing.T) {
	t.Parallel()

	// Test error assertion with mixed given steps (calls + assignments).
	spec := &parser.Spec{
		Name: "ErrorCallTest",
		Locators: map[string]string{
			"submit": "[data-testid=submit]",
		},
		Scopes: []*parser.Scope{{
			Name:   "call_scope",
			Use:    "test",
			Config: map[string]parser.Expr{},
			Scenarios: []*parser.Scenario{{
				Name: "call_fails",
				Given: &parser.Block{
					Steps: []parser.GivenStep{
						&parser.Assignment{Path: "x", Value: parser.LiteralInt{Value: 1}},
						&parser.Call{
							Namespace: "test",
							Method:    "click",
							Args:      []parser.Expr{parser.FieldRef{Path: "submit"}},
						},
					},
				},
				Then: &parser.Block{
					Assertions: []*parser.Assertion{
						{Target: "error", Expected: parser.LiteralString{Value: "click failed"}},
					},
				},
			}},
		}},
	}

	adp := &failingAdapter{errorMsg: "click failed"}
	r := runner.New(spec, map[string]adapter.Adapter{"test": adp}, 1)
	res, err := r.Verify()
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if len(res.Failures) > 0 {
		t.Errorf("expected no failures, got: %s", res.Failures[0].Description)
	}
}

func TestErrorPseudoField_ContractErrorField_NotIntercepted(t *testing.T) {
	t.Parallel()

	// When "error" is a contract output field, it should go through the adapter's
	// Assert method, not the pseudo-field handler. This test verifies the transfer
	// spec pattern still works.
	spec := &parser.Spec{
		Name: "ContractErrorTest",
		Scopes: []*parser.Scope{{
			Name: "transfer",
			Use:  "http",
			Config: map[string]parser.Expr{
				"path":   parser.LiteralString{Value: "/transfer"},
				"method": parser.LiteralString{Value: "POST"},
			},
			Contract: &parser.Contract{
				Input: []*parser.Field{
					{Name: "amount", Type: parser.TypeExpr{Name: "int"}},
				},
				Output: []*parser.Field{
					{Name: "error", Type: parser.TypeExpr{Name: "string", Optional: true}},
				},
			},
			Scenarios: []*parser.Scenario{{
				Name: "check_error_field",
				Given: &parser.Block{
					Steps: []parser.GivenStep{
						&parser.Assignment{Path: "amount", Value: parser.LiteralInt{Value: -1}},
					},
				},
				Then: &parser.Block{
					Assertions: []*parser.Assertion{
						{Target: "error", Expected: parser.LiteralString{Value: "invalid_amount"}},
					},
				},
			}},
		}},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /transfer", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		writeJSON(w, map[string]any{"error": "invalid_amount"})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	adp, err := adapter.NewHTTPAdapter()
	if err != nil {
		t.Fatal(err)
	}
	if err := adp.Init(map[string]string{"base_url": srv.URL}); err != nil {
		t.Fatal(err)
	}

	r := runner.New(spec, map[string]adapter.Adapter{"http": adp}, 1)
	res, err := r.Verify()
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if len(res.Failures) > 0 {
		t.Errorf("expected no failures, got: %s", res.Failures[0].Description)
	}
}
