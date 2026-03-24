package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// specrunBin builds the specrun binary to a temp dir and returns its path.
func specrunBin(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "specrun")
	cmd := exec.Command("go", "build", "-o", bin, ".")
	absDir, err := filepath.Abs(".")
	if err != nil {
		t.Fatalf("abs path: %v", err)
	}
	cmd.Dir = absDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to build specrun: %v\n%s", err, out)
	}
	return bin
}

func TestParse_ValidSpec(t *testing.T) {
	bin := specrunBin(t)

	specFile, err := filepath.Abs("../../examples/transfer.spec")
	if err != nil {
		t.Fatalf("abs path: %v", err)
	}

	cmd := exec.Command(bin, "parse", specFile)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("specrun parse failed: %v\n%s", err, out)
	}

	var result map[string]any
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, out)
	}

	name, ok := result["name"].(string)
	if !ok || name != "AccountAPI" {
		t.Errorf("expected name=AccountAPI, got %v", result["name"])
	}
}

func TestParse_InvalidSpec(t *testing.T) {
	bin := specrunBin(t)

	specFile, err := filepath.Abs("../../testdata/include/circular/a.spec")
	if err != nil {
		t.Fatalf("abs path: %v", err)
	}

	cmd := exec.Command(bin, "parse", specFile)
	err = cmd.Run()
	if err == nil {
		t.Fatal("expected non-zero exit for circular include, got exit 0")
	}
	exitErr, ok := err.(*exec.ExitError)
	if !ok || exitErr.ExitCode() != 1 {
		t.Errorf("expected exit code 1, got: %v", err)
	}
}

func TestParse_MissingFile(t *testing.T) {
	bin := specrunBin(t)

	cmd := exec.Command(bin, "parse", "/nonexistent/path/file.spec")
	err := cmd.Run()
	if err == nil {
		t.Fatal("expected non-zero exit for missing file, got exit 0")
	}
	exitErr, ok := err.(*exec.ExitError)
	if !ok || exitErr.ExitCode() == 0 {
		t.Errorf("expected non-zero exit code, got: %v", err)
	}
}

func TestGenerate_ValidScope(t *testing.T) {
	bin := specrunBin(t)

	specFile, err := filepath.Abs("../../examples/transfer.spec")
	if err != nil {
		t.Fatalf("abs path: %v", err)
	}

	cmd := exec.Command(bin, "generate", "--scope", "transfer", specFile)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("specrun generate failed: %v\n%s", err, out)
	}

	var result map[string]any
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, out)
	}

	for _, field := range []string{"from", "to", "amount"} {
		if _, ok := result[field]; !ok {
			t.Errorf("expected field %q in output, got keys: %v", field, result)
		}
	}
}

func TestGenerate_UnknownScope(t *testing.T) {
	bin := specrunBin(t)

	specFile, err := filepath.Abs("../../examples/transfer.spec")
	if err != nil {
		t.Fatalf("abs path: %v", err)
	}

	cmd := exec.Command(bin, "generate", "--scope", "nonexistent", specFile)
	err = cmd.Run()
	if err == nil {
		t.Fatal("expected non-zero exit for unknown scope, got exit 0")
	}
	exitErr, ok := err.(*exec.ExitError)
	if !ok || exitErr.ExitCode() == 0 {
		t.Errorf("expected non-zero exit code, got: %v", err)
	}
}

func TestGenerate_Reproducible(t *testing.T) {
	bin := specrunBin(t)

	specFile, err := filepath.Abs("../../examples/transfer.spec")
	if err != nil {
		t.Fatalf("abs path: %v", err)
	}

	run := func() []byte {
		cmd := exec.Command(bin, "generate", "--scope", "transfer", "--seed", "99", specFile)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("specrun generate failed: %v\n%s", err, out)
		}
		return out
	}

	first := run()
	second := run()

	if string(first) != string(second) {
		t.Errorf("expected same output with same seed\nfirst:  %s\nsecond: %s", first, second)
	}
}

func TestVerify_JSON(t *testing.T) {
	bin := specrunBin(t)

	srv := startTransferServer(t)
	defer srv.Close()

	specFile, err := filepath.Abs("../../examples/transfer.spec")
	if err != nil {
		t.Fatalf("abs path: %v", err)
	}

	cmd := exec.Command(bin, "verify", "--json", "--seed", "42", "--iterations", "10", specFile)
	cmd.Env = append(os.Environ(), "APP_URL="+srv.URL)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("specrun verify --json failed: %v\n%s", err, out)
	}

	var result map[string]any
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out)
	}

	if result["spec"] != "AccountAPI" {
		t.Errorf("expected spec=AccountAPI, got %v", result["spec"])
	}
	if result["scenarios_run"] != float64(3) {
		t.Errorf("expected scenarios_run=3, got %v", result["scenarios_run"])
	}
}

func TestVerify_ProcessAdapter(t *testing.T) {
	bin := specrunBin(t)

	specContent := `spec EchoTest {
  target {
    command: "echo"
  }

  scope echo {
    use process
    config {
      args: "{\"hello\":\"world\"}"
    }

    contract {
      input {}
      output {
        exit_code: int
      }
    }

    invariant always_succeeds {
      exit_code == 0
    }
  }
}`
	specFile := filepath.Join(t.TempDir(), "echo.spec")
	if err := os.WriteFile(specFile, []byte(specContent), 0644); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(bin, "verify", "--json", "--iterations", "1", specFile)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("specrun verify failed: %v\n%s", err, out)
	}

	var result map[string]any
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("output not valid JSON: %v\n%s", err, out)
	}
}

func TestSelfVerification_Parse(t *testing.T) {
	bin := specrunBin(t)

	specFile, err := filepath.Abs("../../specs/speclang.spec")
	if err != nil {
		t.Fatalf("abs path: %v", err)
	}

	projectRoot, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("abs project root: %v", err)
	}

	srv := startTransferServer(t)
	defer srv.Close()

	brokenSrv := startBrokenTransferServer(t)
	defer brokenSrv.Close()

	httpTestSrv := startHTTPTestServer(t)
	defer httpTestSrv.Close()

	echoToolBin := buildEchoTool(t)

	cmd := exec.Command(bin, "verify", "--json", "--iterations", "10", specFile)
	cmd.Env = append(os.Environ(),
		"SPECRUN_BIN="+bin,
		"APP_URL="+srv.URL,
		"BROKEN_APP_URL="+brokenSrv.URL,
		"HTTP_TEST_URL="+httpTestSrv.URL,
		"ECHO_TOOL_BIN="+echoToolBin,
	)
	// Set working dir to project root so relative paths in specs resolve correctly.
	cmd.Dir = projectRoot
	out, err := cmd.CombinedOutput()
	t.Logf("output:\n%s", out)
	if err != nil {
		t.Fatalf("self-verification failed: %v\n%s", err, out)
	}

	var result map[string]any
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("output not JSON: %v\n%s", err, out)
	}

	failures, _ := result["failures"].([]any)
	if len(failures) > 0 {
		t.Fatalf("self-verification had failures:\n%s", out)
	}
}

func TestVerify_HumanOutput(t *testing.T) {
	bin := specrunBin(t)

	srv := startTransferServer(t)
	defer srv.Close()

	specFile, err := filepath.Abs("../../examples/transfer.spec")
	if err != nil {
		t.Fatalf("abs path: %v", err)
	}

	cmd := exec.Command(bin, "verify", "--seed", "42", "--iterations", "10", specFile)
	cmd.Env = append(os.Environ(), "APP_URL="+srv.URL)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("verify failed: %v\n%s", err, out)
	}

	output := string(out)

	// Check per-scope structure
	if !strings.Contains(output, "scope transfer:") {
		t.Errorf("missing scope header in output:\n%s", output)
	}

	// Check per-item markers
	if !strings.Contains(output, "✓ scenario success") {
		t.Errorf("missing scenario success line:\n%s", output)
	}
	if !strings.Contains(output, "✓ invariant conservation") {
		t.Errorf("missing invariant conservation line:\n%s", output)
	}

	// Check summary
	if !strings.Contains(output, "Scenarios:  3/3 passed") {
		t.Errorf("missing scenario summary:\n%s", output)
	}
}

// startBrokenTransferServer returns a test server that credits the to-account
// but never debits the from-account (conservation invariant violation).
func startBrokenTransferServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/accounts/transfer", func(w http.ResponseWriter, r *http.Request) {
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
			// BUG: only credits to-account, does NOT debit from-account
			resp["to"] = map[string]any{
				"id": req.To.ID, "balance": req.To.Balance + req.Amount,
			}
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})
	return httptest.NewServer(mux)
}

// buildEchoTool builds the echo_tool binary for process adapter tests.
func buildEchoTool(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "echo_tool")
	absDir, err := filepath.Abs("../../testdata/self/echo_tool")
	if err != nil {
		t.Fatalf("abs path: %v", err)
	}
	cmd := exec.Command("go", "build", "-o", bin, ".")
	cmd.Dir = absDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to build echo_tool: %v\n%s", err, out)
	}
	return bin
}

// startHTTPTestServer starts the HTTP adapter test server using httptest.
func startHTTPTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/items", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Request-Id", "test-123")
		w.Header().Set("Requestid", "test-123")
		json.NewEncoder(w).Encode(map[string]any{
			"items": []map[string]any{
				{"id": 1, "name": "alpha"},
				{"id": 2, "name": "beta"},
			},
			"count": 2,
		})
	})

	mux.HandleFunc("GET /api/items/1", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"id":   1,
			"name": "alpha",
			"tags": []string{"first", "primary"},
		})
	})

	mux.HandleFunc("POST /api/items", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		body["id"] = 42
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(body)
	})

	mux.HandleFunc("PUT /api/items/1", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		body["id"] = 1
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(body)
	})

	mux.HandleFunc("DELETE /api/items/1", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"deleted": true})
	})

	mux.HandleFunc("GET /api/headers", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"auth":         r.Header.Get("Authorization"),
			"custom":       r.Header.Get("X-Custom"),
			"content_type": r.Header.Get("Content-Type"),
		})
	})

	// Multi-step workflow endpoints
	var createdResource map[string]any

	mux.HandleFunc("POST /api/resources", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		createdResource = map[string]any{"id": 1, "name": body["name"]}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(createdResource)
	})

	mux.HandleFunc("GET /api/resources/1", func(w http.ResponseWriter, _ *http.Request) {
		if createdResource == nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]any{"error": "not_found"})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(createdResource)
	})

	return httptest.NewServer(mux)
}

func startTransferServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/accounts/transfer", func(w http.ResponseWriter, r *http.Request) {
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
			resp["from"] = map[string]any{
				"id": req.From.ID, "balance": req.From.Balance - req.Amount,
			}
			resp["to"] = map[string]any{
				"id": req.To.ID, "balance": req.To.Balance + req.Amount,
			}
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})
	return httptest.NewServer(mux)
}
