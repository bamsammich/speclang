package main

import (
	"encoding/json"
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

func TestHelp_RootCommand(t *testing.T) {
	t.Parallel()
	bin := specrunBin(t)
	cmd := exec.Command(bin, "--help")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("--help failed: %v\n%s", err, out)
	}
	output := string(out)
	for _, want := range []string{"parse", "generate", "verify", "install"} {
		if !strings.Contains(output, want) {
			t.Errorf("--help missing %q:\n%s", want, output)
		}
	}
}

func TestHelp_VerifyCommand(t *testing.T) {
	t.Parallel()
	bin := specrunBin(t)
	cmd := exec.Command(bin, "verify", "--help")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("verify --help failed: %v\n%s", err, out)
	}
	output := string(out)
	for _, want := range []string{"--seed", "--iterations", "--json", "--keep-services"} {
		if !strings.Contains(output, want) {
			t.Errorf("verify --help missing %q:\n%s", want, output)
		}
	}
	// --no-services was removed; should not appear
	if strings.Contains(output, "--no-services") {
		t.Errorf("verify --help should not contain --no-services:\n%s", output)
	}
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
	skipIfNoDocker(t)

	bin := specrunBin(t)

	specFile, err := filepath.Abs("../../examples/transfer.spec")
	if err != nil {
		t.Fatalf("abs path: %v", err)
	}

	cmd := exec.Command(bin, "verify", "--json", "--seed", "42", "--iterations", "10", specFile)
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
  process {
    command: "echo"
  }

  scope echo {
    action run_echo() {
      let result = process.exec("{\"hello\":\"world\"}")
      return result
    }

    contract {
      input {}
      output {
        exit_code: int
      }
      action: run_echo
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
	skipIfNoDocker(t)

	bin := specrunBin(t)

	specFile, err := filepath.Abs("../../specs/speclang.spec")
	if err != nil {
		t.Fatalf("abs path: %v", err)
	}

	projectRoot, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("abs project root: %v", err)
	}

	echoToolBin := buildEchoTool(t)

	cmd := exec.Command(bin, "verify", "--json", "--iterations", "10", specFile)
	cmd.Env = append(os.Environ(),
		"SPECRUN_BIN="+bin,
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
	skipIfNoDocker(t)

	bin := specrunBin(t)

	specFile, err := filepath.Abs("../../examples/transfer.spec")
	if err != nil {
		t.Fatalf("abs path: %v", err)
	}

	cmd := exec.Command(bin, "verify", "--seed", "42", "--iterations", "10", specFile)
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

// skipIfNoDocker skips the test if Docker is not available.
func skipIfNoDocker(t *testing.T) {
	t.Helper()
	_, err := exec.LookPath("docker")
	if err != nil {
		t.Skip("skipping: docker not found on PATH")
	}
	cmd := exec.Command("docker", "info")
	if err := cmd.Run(); err != nil {
		t.Skip("skipping: docker daemon not running")
	}
}
