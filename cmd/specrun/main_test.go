package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// specrunBin builds the specrun binary to a temp dir and returns its path.
func specrunBin(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "specrun")
	cmd := exec.Command("go", "build", "-o", bin, ".")
	cmd.Dir, _ = filepath.Abs(".")
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

	specContent := `use process

spec EchoTest {
  target {
    command: "echo"
  }

  scope echo {
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
			resp["from"] = map[string]any{"id": req.From.ID, "balance": req.From.Balance - req.Amount}
			resp["to"] = map[string]any{"id": req.To.ID, "balance": req.To.Balance + req.Amount}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})
	return httptest.NewServer(mux)
}
