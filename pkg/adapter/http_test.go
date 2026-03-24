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

func TestExtractPath_ArrayIndex(t *testing.T) {
	obj := map[string]any{
		"items": []any{"alpha", "beta", "gamma"},
	}
	val, err := extractPath(obj, "items.0")
	if err != nil {
		t.Fatal(err)
	}
	if val != "alpha" {
		t.Fatalf("got %v, want alpha", val)
	}
}

func TestExtractPath_ArrayIndexNested(t *testing.T) {
	obj := map[string]any{
		"scopes": []any{
			map[string]any{"name": "transfer", "passed": true},
			map[string]any{"name": "validate", "passed": false},
		},
	}
	val, err := extractPath(obj, "scopes.1.name")
	if err != nil {
		t.Fatal(err)
	}
	if val != "validate" {
		t.Fatalf("got %v, want validate", val)
	}
}

func TestExtractPath_ArrayIndexOutOfRange(t *testing.T) {
	obj := map[string]any{
		"items": []any{"alpha"},
	}
	_, err := extractPath(obj, "items.5")
	if err == nil {
		t.Fatal("expected error for out-of-range index")
	}
}

func TestExtractPath_ArrayNegativeIndex(t *testing.T) {
	obj := map[string]any{
		"items": []any{"alpha"},
	}
	_, err := extractPath(obj, "items.-1")
	if err == nil {
		t.Fatal("expected error for negative index")
	}
}

func TestExtractPath_ObjectInArray(t *testing.T) {
	obj := map[string]any{
		"failures": []any{
			map[string]any{
				"name":   "conservation",
				"input":  map[string]any{"amount": float64(5)},
				"shrunk": true,
			},
		},
	}
	val, err := extractPath(obj, "failures.0.input.amount")
	if err != nil {
		t.Fatal(err)
	}
	if val != float64(5) {
		t.Fatalf("got %v, want 5", val)
	}
}

func TestExtractPath_ArrayInArray(t *testing.T) {
	obj := map[string]any{
		"matrix": []any{
			[]any{1, 2},
			[]any{3, 4},
		},
	}
	val, err := extractPath(obj, "matrix.1.0")
	if err != nil {
		t.Fatal(err)
	}
	if val != 3 {
		t.Fatalf("got %v, want 3", val)
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

// --- Multi-step workflow tests ---

// multiStepHandler provides endpoints for multi-step HTTP workflow tests.
// POST /api/resources — creates a resource, returns {"id": 1, "name": <name>}
// GET  /api/resources/1 — returns the last created resource (or 404)
// GET  /api/headers — echoes request headers
func multiStepMux() *http.ServeMux {
	mux := http.NewServeMux()
	var created map[string]any

	mux.HandleFunc("POST /api/resources", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		created = map[string]any{"id": float64(1), "name": body["name"]}
		if r.Header.Get("Authorization") != "" {
			created["auth"] = r.Header.Get("Authorization")
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(created)
	})

	mux.HandleFunc("GET /api/resources/1", func(w http.ResponseWriter, r *http.Request) {
		if created == nil {
			http.Error(w, `{"error":"not_found"}`, http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(created)
	})

	mux.HandleFunc("GET /api/headers", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"auth":   r.Header.Get("Authorization"),
			"custom": r.Header.Get("X-Custom"),
		})
	})

	// Cookie endpoints
	mux.HandleFunc("POST /api/login", func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "session", Value: "abc123", Path: "/"})
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"logged_in": true})
	})

	mux.HandleFunc("GET /api/me", func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("session")
		if err != nil || cookie.Value != "abc123" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]any{"error": "unauthorized"})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"user": "alice", "session": cookie.Value})
	})

	return mux
}

func TestMultiStep_CreateThenVerify(t *testing.T) {
	srv := httptest.NewServer(multiStepMux())
	defer srv.Close()

	a := NewHTTPAdapter()
	if err := a.Init(map[string]string{"base_url": srv.URL}); err != nil {
		t.Fatal(err)
	}

	// Step 1: POST to create a resource
	postArgs := mustMarshal(t, []any{"/api/resources", map[string]any{"name": "widget"}})
	resp, err := a.Action("post", postArgs)
	if err != nil {
		t.Fatalf("POST failed: %v", err)
	}
	if !resp.OK {
		t.Fatalf("POST not OK: %s", resp.Error)
	}

	// Step 2: GET to verify the resource exists
	getArgs := mustMarshal(t, []any{"/api/resources/1"})
	resp, err = a.Action("get", getArgs)
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	if !resp.OK {
		t.Fatalf("GET not OK: %s", resp.Error)
	}

	// Assertions apply to the last response (the GET)
	assertResp, err := a.Assert("name", "", json.RawMessage(`"widget"`))
	if err != nil {
		t.Fatal(err)
	}
	if !assertResp.OK {
		t.Fatalf("assertion failed: %s", assertResp.Error)
	}

	assertResp, err = a.Assert("id", "", json.RawMessage(`1`))
	if err != nil {
		t.Fatal(err)
	}
	if !assertResp.OK {
		t.Fatalf("assertion failed: %s", assertResp.Error)
	}
}

func TestMultiStep_HeaderPersistence(t *testing.T) {
	srv := httptest.NewServer(multiStepMux())
	defer srv.Close()

	a := NewHTTPAdapter()
	if err := a.Init(map[string]string{"base_url": srv.URL}); err != nil {
		t.Fatal(err)
	}

	// Set a header
	headerArgs := mustMarshal(t, []any{"Authorization", "Bearer my-token"})
	if _, err := a.Action("header", headerArgs); err != nil {
		t.Fatal(err)
	}

	// First request: POST — header should be present
	postArgs := mustMarshal(t, []any{"/api/resources", map[string]any{"name": "test"}})
	if _, err := a.Action("post", postArgs); err != nil {
		t.Fatal(err)
	}
	assertResp, err := a.Assert("auth", "", json.RawMessage(`"Bearer my-token"`))
	if err != nil {
		t.Fatal(err)
	}
	if !assertResp.OK {
		t.Fatalf("POST auth assertion failed: %s", assertResp.Error)
	}

	// Second request: GET — header should still be present
	getArgs := mustMarshal(t, []any{"/api/headers"})
	if _, err := a.Action("get", getArgs); err != nil {
		t.Fatal(err)
	}
	assertResp, err = a.Assert("auth", "", json.RawMessage(`"Bearer my-token"`))
	if err != nil {
		t.Fatal(err)
	}
	if !assertResp.OK {
		t.Fatalf("GET auth assertion failed: %s", assertResp.Error)
	}
}

func TestMultiStep_CookiePersistence(t *testing.T) {
	srv := httptest.NewServer(multiStepMux())
	defer srv.Close()

	a := NewHTTPAdapter()
	if err := a.Init(map[string]string{"base_url": srv.URL}); err != nil {
		t.Fatal(err)
	}

	// Step 1: POST /api/login — server sets a session cookie
	loginArgs := mustMarshal(t, []any{"/api/login", map[string]any{}})
	resp, err := a.Action("post", loginArgs)
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}
	if !resp.OK {
		t.Fatalf("login not OK: %s", resp.Error)
	}

	// Step 2: GET /api/me — cookie should be sent automatically
	meArgs := mustMarshal(t, []any{"/api/me"})
	resp, err = a.Action("get", meArgs)
	if err != nil {
		t.Fatalf("me failed: %v", err)
	}
	if !resp.OK {
		t.Fatalf("me not OK: %s", resp.Error)
	}

	assertResp, err := a.Assert("user", "", json.RawMessage(`"alice"`))
	if err != nil {
		t.Fatal(err)
	}
	if !assertResp.OK {
		t.Fatalf("assertion failed: %s", assertResp.Error)
	}
}

func TestMultiStep_ErrorInMiddle(t *testing.T) {
	srv := httptest.NewServer(multiStepMux())
	defer srv.Close()

	a := NewHTTPAdapter()
	if err := a.Init(map[string]string{"base_url": srv.URL}); err != nil {
		t.Fatal(err)
	}

	// GET a resource that doesn't exist yet — should get 404
	getArgs := mustMarshal(t, []any{"/api/resources/1"})
	resp, err := a.Action("get", getArgs)
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	// The action still returns OK (the request succeeded); status is captured
	if !resp.OK {
		t.Fatalf("GET not OK: %s", resp.Error)
	}

	// Assert the 404 status
	assertResp, err := a.Assert("status", "", json.RawMessage(`404`))
	if err != nil {
		t.Fatal(err)
	}
	if !assertResp.OK {
		t.Fatalf("status assertion failed: %s", assertResp.Error)
	}
}
