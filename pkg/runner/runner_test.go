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
