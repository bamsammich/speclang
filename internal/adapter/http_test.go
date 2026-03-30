package adapter

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// writeJSON encodes v as JSON to w, falling back to a 500 error if encoding fails.
func writeJSON(w http.ResponseWriter, v any) {
	if err := json.NewEncoder(w).Encode(v); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

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
	a, err := NewHTTPAdapter()
	if err != nil {
		t.Fatal(err)
	}
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

// assertQuery calls Call with a query method and asserts the actual value matches expected.
func assertQuery(t *testing.T, a *HTTPAdapter, method string, expected string) {
	t.Helper()
	resp, err := a.Call(method, nil)
	if err != nil {
		t.Fatalf("query %q: %v", method, err)
	}
	if !resp.OK {
		t.Fatalf("query %q not OK: %s", method, resp.Error)
	}
	// Normalize through JSON for consistent comparison.
	var actualNorm, expectedNorm any
	if err := json.Unmarshal(resp.Actual, &actualNorm); err != nil {
		t.Fatalf("normalizing actual: %v", err)
	}
	if err := json.Unmarshal([]byte(expected), &expectedNorm); err != nil {
		t.Fatalf("normalizing expected: %v", err)
	}
	actualJSON, _ := json.Marshal(actualNorm)
	expectedJSON, _ := json.Marshal(expectedNorm)
	if string(actualJSON) != string(expectedJSON) {
		t.Fatalf("query %q: expected %s, got %s", method, expected, string(resp.Actual))
	}
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
	if _, err := a.Call("post", args); err != nil {
		t.Fatal(err)
	}
}

// --- extractPath unit tests ---

func TestExtractPath_TopLevel(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
	obj := map[string]any{"name": "alice"}
	_, err := extractPath(obj, "missing")
	if err == nil {
		t.Fatal("expected error for missing key")
	}
}

func TestExtractPath_NonMapIntermediate(t *testing.T) {
	t.Parallel()
	obj := map[string]any{"name": "alice"}
	_, err := extractPath(obj, "name.sub")
	if err == nil {
		t.Fatal("expected error for non-map intermediate")
	}
}

func TestExtractPath_NullValue(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
	obj := map[string]any{
		"items": []any{"alpha"},
	}
	_, err := extractPath(obj, "items.5")
	if err == nil {
		t.Fatal("expected error for out-of-range index")
	}
}

func TestExtractPath_ArrayNegativeIndex(t *testing.T) {
	t.Parallel()
	obj := map[string]any{
		"items": []any{"alpha"},
	}
	_, err := extractPath(obj, "items.-1")
	if err == nil {
		t.Fatal("expected error for negative index")
	}
}

func TestExtractPath_ObjectInArray(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
	a, srv := newTestAdapter(t)
	defer srv.Close()

	doTransfer(t, a, 100, 50, 30)

	assertQuery(t, a, "from.balance", `70`)
	assertQuery(t, a, "to.balance", `80`)
}

func TestAction_PostInsufficientFunds(t *testing.T) {
	t.Parallel()
	a, srv := newTestAdapter(t)
	defer srv.Close()

	doTransfer(t, a, 10, 50, 30)

	assertQuery(t, a, "error", `"insufficient_funds"`)
}

func TestCall_Status(t *testing.T) {
	t.Parallel()
	a, srv := newTestAdapter(t)
	defer srv.Close()

	doTransfer(t, a, 100, 50, 30)
	assertQuery(t, a, "status", `200`)
}

func TestCall_NestedBodyField(t *testing.T) {
	t.Parallel()
	a, srv := newTestAdapter(t)
	defer srv.Close()

	doTransfer(t, a, 100, 50, 30)
	assertQuery(t, a, "from.balance", `70`)
}

func TestCall_ErrorNull(t *testing.T) {
	t.Parallel()
	a, srv := newTestAdapter(t)
	defer srv.Close()

	doTransfer(t, a, 100, 50, 30)
	assertQuery(t, a, "error", `null`)
}

func TestCall_ErrorString(t *testing.T) {
	t.Parallel()
	a, srv := newTestAdapter(t)
	defer srv.Close()

	doTransfer(t, a, 10, 50, 30)
	assertQuery(t, a, "error", `"insufficient_funds"`)
}

func TestCall_BeforeRequest(t *testing.T) {
	t.Parallel()
	a, err := NewHTTPAdapter()
	if err != nil {
		t.Fatal(err)
	}
	a.BaseURL = "http://unused"

	_, err = a.Call("status", nil)
	if err == nil {
		t.Fatal("expected error when querying before any request")
	}
}

func TestAction_Header(t *testing.T) {
	t.Parallel()
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

	a, err := NewHTTPAdapter()
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Init(map[string]string{"base_url": srv.URL}); err != nil {
		t.Fatal(err)
	}

	headerArgs := mustMarshal(t, []any{"Authorization", "Bearer token123"})
	if _, err := a.Call("header", headerArgs); err != nil {
		t.Fatal(err)
	}

	getArgs := mustMarshal(t, []any{"/"})
	if _, err := a.Call("get", getArgs); err != nil {
		t.Fatal(err)
	}

	if receivedAuth != "Bearer token123" {
		t.Fatalf("expected Authorization header, got %q", receivedAuth)
	}
}

func TestCall_UnknownMethod(t *testing.T) {
	t.Parallel()
	a, err := NewHTTPAdapter()
	if err != nil {
		t.Fatal(err)
	}
	a.BaseURL = "http://unused"

	// "patch" is now a valid HTTP method, so test a truly unknown method.
	// But first we need a request to have been made for non-action methods.
	// For an unknown dot-path with no prior request, we get "no request has been made yet"
	_, err = a.Call("nonexistent.field", nil)
	if err == nil {
		t.Fatal("expected error for unknown method with no prior request")
	}
}

func TestCall_Header(t *testing.T) {
	t.Parallel()
	a, srv := newTestAdapter(t)
	defer srv.Close()

	doTransfer(t, a, 100, 50, 30)
	assertQuery(t, a, "header.X-Request-Id", `"test-123"`)
}

func TestCall_ValueMismatch(t *testing.T) {
	t.Parallel()
	a, srv := newTestAdapter(t)
	defer srv.Close()

	doTransfer(t, a, 100, 50, 30)

	// Call returns the actual value; comparison is the runner's job now.
	resp, err := a.Call("from.balance", nil)
	if err != nil {
		t.Fatal(err)
	}
	if string(resp.Actual) == "999" {
		t.Fatal("expected actual value to differ from 999")
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
		writeJSON(w, created)
	})

	mux.HandleFunc("GET /api/resources/1", func(w http.ResponseWriter, r *http.Request) {
		if created == nil {
			http.Error(w, `{"error":"not_found"}`, http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		writeJSON(w, created)
	})

	mux.HandleFunc("GET /api/headers", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		writeJSON(w, map[string]any{
			"auth":   r.Header.Get("Authorization"),
			"custom": r.Header.Get("X-Custom"),
		})
	})

	// Cookie endpoints
	mux.HandleFunc("POST /api/login", func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "session", Value: "abc123", Path: "/"})
		w.Header().Set("Content-Type", "application/json")
		writeJSON(w, map[string]any{"logged_in": true})
	})

	mux.HandleFunc("GET /api/me", func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("session")
		if err != nil || cookie.Value != "abc123" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			writeJSON(w, map[string]any{"error": "unauthorized"})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		writeJSON(w, map[string]any{"user": "alice", "session": cookie.Value})
	})

	return mux
}

func TestMultiStep_CreateThenVerify(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(multiStepMux())
	defer srv.Close()

	a, err := NewHTTPAdapter()
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Init(map[string]string{"base_url": srv.URL}); err != nil {
		t.Fatal(err)
	}

	// Step 1: POST to create a resource
	postArgs := mustMarshal(t, []any{"/api/resources", map[string]any{"name": "widget"}})
	resp, err := a.Call("post", postArgs)
	if err != nil {
		t.Fatalf("POST failed: %v", err)
	}
	if !resp.OK {
		t.Fatalf("POST not OK: %s", resp.Error)
	}

	// Step 2: GET to verify the resource exists
	getArgs := mustMarshal(t, []any{"/api/resources/1"})
	resp, err = a.Call("get", getArgs)
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	if !resp.OK {
		t.Fatalf("GET not OK: %s", resp.Error)
	}

	// Assertions apply to the last response (the GET)
	assertQuery(t, a, "name", `"widget"`)
	assertQuery(t, a, "id", `1`)
}

func TestMultiStep_HeaderPersistence(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(multiStepMux())
	defer srv.Close()

	a, err := NewHTTPAdapter()
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Init(map[string]string{"base_url": srv.URL}); err != nil {
		t.Fatal(err)
	}

	// Set a header
	headerArgs := mustMarshal(t, []any{"Authorization", "Bearer my-token"})
	if _, err := a.Call("header", headerArgs); err != nil {
		t.Fatal(err)
	}

	// First request: POST — header should be present
	postArgs := mustMarshal(t, []any{"/api/resources", map[string]any{"name": "test"}})
	if _, err := a.Call("post", postArgs); err != nil {
		t.Fatal(err)
	}
	assertQuery(t, a, "auth", `"Bearer my-token"`)

	// Second request: GET — header should still be present
	getArgs := mustMarshal(t, []any{"/api/headers"})
	if _, err := a.Call("get", getArgs); err != nil {
		t.Fatal(err)
	}
	assertQuery(t, a, "auth", `"Bearer my-token"`)
}

func TestMultiStep_CookiePersistence(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(multiStepMux())
	defer srv.Close()

	a, err := NewHTTPAdapter()
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Init(map[string]string{"base_url": srv.URL}); err != nil {
		t.Fatal(err)
	}

	// Step 1: POST /api/login — server sets a session cookie
	loginArgs := mustMarshal(t, []any{"/api/login", map[string]any{}})
	resp, err := a.Call("post", loginArgs)
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}
	if !resp.OK {
		t.Fatalf("login not OK: %s", resp.Error)
	}

	// Step 2: GET /api/me — cookie should be sent automatically
	meArgs := mustMarshal(t, []any{"/api/me"})
	resp, err = a.Call("get", meArgs)
	if err != nil {
		t.Fatalf("me failed: %v", err)
	}
	if !resp.OK {
		t.Fatalf("me not OK: %s", resp.Error)
	}

	assertQuery(t, a, "user", `"alice"`)
}

func TestMultiStep_ErrorInMiddle(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(multiStepMux())
	defer srv.Close()

	a, err := NewHTTPAdapter()
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Init(map[string]string{"base_url": srv.URL}); err != nil {
		t.Fatal(err)
	}

	// GET a resource that doesn't exist yet — should get 404
	getArgs := mustMarshal(t, []any{"/api/resources/1"})
	resp, err := a.Call("get", getArgs)
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	// The action still returns OK (the request succeeded); status is captured
	if !resp.OK {
		t.Fatalf("GET not OK: %s", resp.Error)
	}

	// Assert the 404 status
	assertQuery(t, a, "status", `404`)
}
