package runner_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bamsammich/speclang/pkg/adapter"
	"github.com/bamsammich/speclang/pkg/parser"
	"github.com/bamsammich/speclang/pkg/runner"
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

	r := runner.New(spec, adp, 42)
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
		Uses: []string{"http"},
		Scopes: []*parser.Scope{{
			Name: "math",
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

	r := runner.New(spec, adp, 1)
	res, err := r.Verify()
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if len(res.Failures) > 0 {
		t.Errorf("expected no failures, got %v", res.Failures[0].Description)
	}
}

// mockAdapter records Assert calls to verify locator resolution.
type mockAdapter struct {
	assertCalls []assertCall
}

type assertCall struct {
	Property string
	Locator  string
	Expected json.RawMessage
}

func (m *mockAdapter) Init(config map[string]string) error { return nil }
func (m *mockAdapter) Action(name string, args json.RawMessage) (*adapter.Response, error) {
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
		Uses: []string{"playwright"},
		Locators: map[string]string{
			"welcome": "[data-testid=welcome]",
		},
		Scopes: []*parser.Scope{{
			Name: "ui",
			Config: map[string]parser.Expr{
				"path":   parser.LiteralString{Value: "/home"},
				"method": parser.LiteralString{Value: "POST"},
			},
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
	r := runner.New(spec, mock, 1)
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
		Uses: []string{"playwright"},
		// No locators defined
		Scopes: []*parser.Scope{{
			Name:   "ui",
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
	r := runner.New(spec, mock, 1)
	_, err := r.Verify()
	if err == nil {
		t.Fatal("expected error for missing locator, got nil")
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

	r := runner.New(spec, adp, 42)
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
