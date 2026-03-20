package adapter

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func transferHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

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
		resp["from"].(map[string]any)["balance"] = req.From.Balance - req.Amount
		resp["to"].(map[string]any)["balance"] = req.To.Balance + req.Amount
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Request-Id", "test-123")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func newTestAdapter(t *testing.T) (*HTTPAdapter, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(transferHandler))
	a := NewHTTPAdapter()
	if err := a.Init(map[string]string{"base_url": srv.URL}); err != nil {
		t.Fatal(err)
	}
	return a, srv
}

func mustMarshal(t *testing.T, v any) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func doTransfer(t *testing.T, a *HTTPAdapter, fromBalance, toBalance, amount int) {
	t.Helper()
	args := mustMarshal(t, []any{
		"/transfer",
		map[string]any{
			"from":   map[string]any{"id": "acct1", "balance": fromBalance},
			"to":     map[string]any{"id": "acct2", "balance": toBalance},
			"amount": amount,
		},
	})
	if _, err := a.Action("post", args); err != nil {
		t.Fatal(err)
	}
}

// --- extractPath unit tests ---

func TestExtractPath_TopLevel(t *testing.T) {
	obj := map[string]any{"name": "alice"}
	val, err := extractPath(obj, "name")
	if err != nil {
		t.Fatal(err)
	}
	if val != "alice" {
		t.Fatalf("got %v, want alice", val)
	}
}

func TestExtractPath_Nested(t *testing.T) {
	obj := map[string]any{
		"from": map[string]any{"balance": float64(70)},
	}
	val, err := extractPath(obj, "from.balance")
	if err != nil {
		t.Fatal(err)
	}
	if val != float64(70) {
		t.Fatalf("got %v, want 70", val)
	}
}

func TestExtractPath_MissingKey(t *testing.T) {
	obj := map[string]any{"name": "alice"}
	_, err := extractPath(obj, "missing")
	if err == nil {
		t.Fatal("expected error for missing key")
	}
}

func TestExtractPath_NonMapIntermediate(t *testing.T) {
	obj := map[string]any{"name": "alice"}
	_, err := extractPath(obj, "name.sub")
	if err == nil {
		t.Fatal("expected error for non-map intermediate")
	}
}

func TestExtractPath_NullValue(t *testing.T) {
	obj := map[string]any{"error": nil}
	val, err := extractPath(obj, "error")
	if err != nil {
		t.Fatal(err)
	}
	if val != nil {
		t.Fatalf("got %v, want nil", val)
	}
}

// --- Integration tests ---

func TestAction_PostSuccess(t *testing.T) {
	a, srv := newTestAdapter(t)
	defer srv.Close()

	doTransfer(t, a, 100, 50, 30)

	assertResp, err := a.Assert("from.balance", "", json.RawMessage(`70`))
	if err != nil {
		t.Fatal(err)
	}
	if !assertResp.OK {
		t.Fatalf("assertion failed: %s", assertResp.Error)
	}

	assertResp, err = a.Assert("to.balance", "", json.RawMessage(`80`))
	if err != nil {
		t.Fatal(err)
	}
	if !assertResp.OK {
		t.Fatalf("assertion failed: %s", assertResp.Error)
	}
}

func TestAction_PostInsufficientFunds(t *testing.T) {
	a, srv := newTestAdapter(t)
	defer srv.Close()

	doTransfer(t, a, 10, 50, 30)

	assertResp, err := a.Assert("error", "", json.RawMessage(`"insufficient_funds"`))
	if err != nil {
		t.Fatal(err)
	}
	if !assertResp.OK {
		t.Fatalf("assertion failed: %s", assertResp.Error)
	}
}

func TestAssert_Status(t *testing.T) {
	a, srv := newTestAdapter(t)
	defer srv.Close()

	doTransfer(t, a, 100, 50, 30)

	resp, err := a.Assert("status", "", json.RawMessage(`200`))
	if err != nil {
		t.Fatal(err)
	}
	if !resp.OK {
		t.Fatalf("assertion failed: %s", resp.Error)
	}
}

func TestAssert_NestedBodyField(t *testing.T) {
	a, srv := newTestAdapter(t)
	defer srv.Close()

	doTransfer(t, a, 100, 50, 30)

	resp, err := a.Assert("from.balance", "", json.RawMessage(`70`))
	if err != nil {
		t.Fatal(err)
	}
	if !resp.OK {
		t.Fatalf("assertion failed: %s", resp.Error)
	}
}

func TestAssert_ErrorNull(t *testing.T) {
	a, srv := newTestAdapter(t)
	defer srv.Close()

	doTransfer(t, a, 100, 50, 30)

	resp, err := a.Assert("error", "", json.RawMessage(`null`))
	if err != nil {
		t.Fatal(err)
	}
	if !resp.OK {
		t.Fatalf("expected null error, got: %s", resp.Error)
	}
}

func TestAssert_ErrorString(t *testing.T) {
	a, srv := newTestAdapter(t)
	defer srv.Close()

	doTransfer(t, a, 10, 50, 30)

	resp, err := a.Assert("error", "", json.RawMessage(`"insufficient_funds"`))
	if err != nil {
		t.Fatal(err)
	}
	if !resp.OK {
		t.Fatalf("assertion failed: %s", resp.Error)
	}
}

func TestAssert_BeforeRequest(t *testing.T) {
	a := NewHTTPAdapter()
	a.BaseURL = "http://unused"

	_, err := a.Assert("status", "", json.RawMessage(`200`))
	if err == nil {
		t.Fatal("expected error when asserting before any request")
	}
}

func TestAction_Header(t *testing.T) {
	var receivedAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write([]byte(`{"ok": true}`)); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}))
	defer srv.Close()

	a := NewHTTPAdapter()
	if err := a.Init(map[string]string{"base_url": srv.URL}); err != nil {
		t.Fatal(err)
	}

	headerArgs := mustMarshal(t, []any{"Authorization", "Bearer token123"})
	if _, err := a.Action("header", headerArgs); err != nil {
		t.Fatal(err)
	}

	getArgs := mustMarshal(t, []any{"/"})
	if _, err := a.Action("get", getArgs); err != nil {
		t.Fatal(err)
	}

	if receivedAuth != "Bearer token123" {
		t.Fatalf("expected Authorization header, got %q", receivedAuth)
	}
}

func TestAction_Unknown(t *testing.T) {
	a := NewHTTPAdapter()
	a.BaseURL = "http://unused"

	_, err := a.Action("patch", json.RawMessage(`["/foo"]`))
	if err == nil {
		t.Fatal("expected error for unknown action")
	}
}

func TestAssert_Header(t *testing.T) {
	a, srv := newTestAdapter(t)
	defer srv.Close()

	doTransfer(t, a, 100, 50, 30)

	resp, err := a.Assert("header.X-Request-Id", "", json.RawMessage(`"test-123"`))
	if err != nil {
		t.Fatal(err)
	}
	if !resp.OK {
		t.Fatalf("assertion failed: %s", resp.Error)
	}
}

func TestAssert_FailureMismatch(t *testing.T) {
	a, srv := newTestAdapter(t)
	defer srv.Close()

	doTransfer(t, a, 100, 50, 30)

	resp, err := a.Assert("from.balance", "", json.RawMessage(`999`))
	if err != nil {
		t.Fatal(err)
	}
	if resp.OK {
		t.Fatal("expected assertion to fail for mismatched value")
	}
}
