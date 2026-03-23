package runner_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bamsammich/speclang/v2/pkg/adapter"
	"github.com/bamsammich/speclang/v2/pkg/parser"
	"github.com/bamsammich/speclang/v2/pkg/runner"
)

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

	adp := adapter.NewHTTPAdapter()
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
								Left: parser.FieldRef{Path: "a"},
								Op:   "+",
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
		json.NewDecoder(r.Body).Decode(&req)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]int{"sum": req["a"] + req["b"]})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	adp := adapter.NewHTTPAdapter()
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

// mockAdapter records Action and Assert calls for testing.
type mockAdapter struct {
	actionCalls []actionCall
	assertCalls []assertCall
}

type actionCall struct {
	Name string
	Args json.RawMessage
}

type assertCall struct {
	Property string
	Locator  string
	Expected json.RawMessage
}

func (m *mockAdapter) Init(config map[string]string) error { return nil }
func (m *mockAdapter) Action(name string, args json.RawMessage) (*adapter.Response, error) {
	m.actionCalls = append(m.actionCalls, actionCall{Name: name, Args: args})
	return &adapter.Response{OK: true, Actual: json.RawMessage(`{}`)}, nil
}
func (m *mockAdapter) Assert(property string, locator string, expected json.RawMessage) (*adapter.Response, error) {
	m.assertCalls = append(m.assertCalls, assertCall{
		Property: property,
		Locator:  locator,
		Expected: expected,
	})
	return &adapter.Response{OK: true, Actual: expected}, nil
}
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

	if len(mock.assertCalls) != 1 {
		t.Fatalf("expected 1 assert call, got %d", len(mock.assertCalls))
	}

	call := mock.assertCalls[0]
	if call.Property != "visible" {
		t.Errorf("property = %q, want 'visible'", call.Property)
	}
	if call.Locator != "[data-testid=welcome]" {
		t.Errorf("locator = %q, want '[data-testid=welcome]'", call.Locator)
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
						&parser.Assignment{Path: "user", Value: parser.LiteralString{Value: "alice"}},
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

	// Verify action calls executed in order
	if len(mock.actionCalls) != 2 {
		t.Fatalf("expected 2 action calls, got %d", len(mock.actionCalls))
	}

	// First call: fill(username_selector, "alice")
	if mock.actionCalls[0].Name != "fill" {
		t.Errorf("action 0: name = %q, want 'fill'", mock.actionCalls[0].Name)
	}
	var fillArgs []any
	json.Unmarshal(mock.actionCalls[0].Args, &fillArgs)
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
	if mock.actionCalls[1].Name != "click" {
		t.Errorf("action 1: name = %q, want 'click'", mock.actionCalls[1].Name)
	}
	var clickArgs []any
	json.Unmarshal(mock.actionCalls[1].Args, &clickArgs)
	if len(clickArgs) != 1 || clickArgs[0] != "[data-testid=submit]" {
		t.Errorf("click args = %v, want [[data-testid=submit]]", clickArgs)
	}

	// Verify assertion
	if len(mock.assertCalls) != 1 {
		t.Fatalf("expected 1 assert call, got %d", len(mock.assertCalls))
	}
	if mock.assertCalls[0].Property != "visible" {
		t.Errorf("assert property = %q, want 'visible'", mock.assertCalls[0].Property)
	}
	if mock.assertCalls[0].Locator != "[data-testid=welcome]" {
		t.Errorf("assert locator = %q, want '[data-testid=welcome]'", mock.assertCalls[0].Locator)
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

	adp := adapter.NewHTTPAdapter()
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
