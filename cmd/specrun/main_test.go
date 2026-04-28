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

	// v4: no spec name — verify expected structure keys are present
	if _, ok := result["scopes"]; !ok {
		t.Errorf("expected 'scopes' key in parse output, got keys: %v", result)
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

	if result["spec"] != "transfer.spec" {
		t.Errorf("expected spec=transfer.spec, got %v", result["spec"])
	}
	if result["scenarios_run"] != float64(3) {
		t.Errorf("expected scenarios_run=3, got %v", result["scenarios_run"])
	}
}

func TestVerify_ProcessAdapter(t *testing.T) {
	bin := specrunBin(t)

	specContent := `process {
  command: "echo"
}

model EchoResult {
  exit_code: int
}

contract EchoContract -> EchoResult {
  action {
    let result = process.exec("{\"hello\":\"world\"}")
    return result
  }

  invariant always_succeeds {
    output.exit_code == 0
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

// ─── Glob verify tests ────────────────────────────────────────────────────────

// echoSpecContent returns a minimal process-adapter spec with one scenario.
// The spec verifies against the `echo` binary, which is always available,
// printing its arguments. We only care about exit_code.
const _echoSpec = `process {
  command: "echo"
}

model EchoOut {
  exit_code: int
}

contract EchoContract -> EchoOut {
  action {
    let result = process.exec("hello")
    return result
  }

  invariant always_ok {
    output.exit_code == 0
  }
}
`

// noContractSpec is a valid spec with only a model — no contracts.
const _noContractSpec = `model EmptyModel {
  value: string
}
`

func TestVerify_GlobMatchesMultiple(t *testing.T) {
	t.Parallel()
	bin := specrunBin(t)

	dir := t.TempDir()

	// Two specs with contracts, one without.
	writeFile(t, filepath.Join(dir, "a.spec"), _echoSpec)
	writeFile(t, filepath.Join(dir, "b.spec"), _echoSpec)
	writeFile(t, filepath.Join(dir, "empty.spec"), _noContractSpec)

	pattern := filepath.Join(dir, "*.spec")

	cmd := exec.Command(bin, "verify", "--json", "--iterations", "1", pattern)
	out, err := cmd.CombinedOutput()
	t.Logf("output:\n%s", out)
	if err != nil {
		t.Fatalf("verify glob failed: %v\n%s", err, out)
	}

	// JSON-lines output: two result objects (a.spec and b.spec).
	// empty.spec should produce only a stderr skip line, not a JSON result.
	// CombinedOutput merges stderr into the buffer, so we count only JSON lines.
	combined := string(out)
	jsonLines := jsonOnlyLines(combined)
	if len(jsonLines) != 2 {
		t.Fatalf("expected 2 JSON result lines (a.spec + b.spec), got %d:\n%s", len(jsonLines), combined)
	}

	for _, line := range jsonLines {
		var result map[string]any
		if err := json.Unmarshal([]byte(line), &result); err != nil {
			t.Fatalf("line not valid JSON: %v\nline: %s", err, line)
		}
	}
}

func TestVerify_GlobZeroMatches(t *testing.T) {
	t.Parallel()
	bin := specrunBin(t)

	dir := t.TempDir()
	pattern := filepath.Join(dir, "*.spec") // dir is empty

	cmd := exec.Command(bin, "verify", pattern)
	err := cmd.Run()
	if err == nil {
		t.Fatal("expected non-zero exit for zero-match glob, got exit 0")
	}
	exitErr, ok := err.(*exec.ExitError)
	if !ok || exitErr.ExitCode() != 1 {
		t.Errorf("expected exit code 1, got: %v", err)
	}
}

func TestVerify_GlobRecursive(t *testing.T) {
	t.Parallel()
	bin := specrunBin(t)

	dir := t.TempDir()
	subDir := filepath.Join(dir, "sub")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	writeFile(t, filepath.Join(dir, "root.spec"), _echoSpec)
	writeFile(t, filepath.Join(subDir, "nested.spec"), _echoSpec)

	pattern := filepath.Join(dir, "**", "*.spec")

	cmd := exec.Command(bin, "verify", "--json", "--iterations", "1", pattern)
	out, err := cmd.CombinedOutput()
	t.Logf("output:\n%s", out)
	if err != nil {
		t.Fatalf("recursive glob verify failed: %v\n%s", err, out)
	}

	lines := jsonOnlyLines(string(out))
	if len(lines) != 2 {
		t.Fatalf("expected 2 JSON result lines (root.spec + nested.spec), got %d:\n%s", len(lines), string(out))
	}
}

func TestVerify_NoContractSkip(t *testing.T) {
	t.Parallel()
	bin := specrunBin(t)

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "only_model.spec"), _noContractSpec)

	// Single file, no contracts — should exit 0 with stderr skip message.
	cmd := exec.Command(bin, "verify", filepath.Join(dir, "only_model.spec"))
	out, err := cmd.CombinedOutput()
	t.Logf("output:\n%s", out)
	if err != nil {
		t.Fatalf("expected exit 0 for no-contract spec, got: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "no contracts, skipping") {
		t.Errorf("expected skip message in output, got:\n%s", out)
	}
}

func TestVerify_SingleFilePreserved(t *testing.T) {
	t.Parallel()
	bin := specrunBin(t)

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "single.spec"), _echoSpec)

	cmd := exec.Command(bin, "verify", "--json", "--iterations", "1", filepath.Join(dir, "single.spec"))
	out, err := cmd.CombinedOutput()
	t.Logf("output:\n%s", out)
	if err != nil {
		t.Fatalf("single-file verify failed: %v\n%s", err, out)
	}

	var result map[string]any
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("output not valid JSON: %v\n%s", err, out)
	}
}

// writeFile is a test helper that writes content to path, fatally failing on error.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// jsonOnlyLines returns lines from s that start with '{' (JSON objects).
// This filters out stderr messages (e.g. skip notices) from CombinedOutput.
func jsonOnlyLines(s string) []string {
	var out []string
	for _, line := range strings.Split(strings.TrimSpace(s), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "{") {
			out = append(out, trimmed)
		}
	}
	return out
}

// ─── Migrate command safety tests ─────────────────────────────────────────────

// validV3Spec is a minimal v3 spec that migrates to valid v4.
const _validV3Spec = `spec Minimal {
  scope ping {
    action ping(msg: string) {
      let result = http.post("/ping", { msg: msg })
      return result
    }

    contract {
      input {
        msg: string
      }
      output {
        ok: bool
      }
      action: ping
    }

    scenario basic {
      given {
        msg: "hello"
      }
      then {
        ok == true
      }
    }
  }
}
`

// TestMigrate_V4_DryRunPrintsOutput verifies migrate without --write prints to stdout.
func TestMigrate_V4_DryRunPrintsOutput(t *testing.T) {
	t.Parallel()
	bin := specrunBin(t)

	specFile := filepath.Join(t.TempDir(), "minimal.spec")
	writeFile(t, specFile, _validV3Spec)

	cmd := exec.Command(bin, "migrate", "--to", "v4", specFile)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("migrate dry-run failed: %v\n%s", err, out)
	}
	// Output should contain v4 contract keyword
	if !strings.Contains(string(out), "contract") {
		t.Errorf("expected 'contract' in output, got:\n%s", out)
	}
	// Original file must be unchanged
	content, err := os.ReadFile(specFile)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if string(content) != _validV3Spec {
		t.Errorf("original file was modified without --write:\ngot:\n%s\nwant:\n%s", content, _validV3Spec)
	}
}

// TestMigrate_V4_WriteProducesValidV4 verifies --write overwrites the file with parseable v4.
func TestMigrate_V4_WriteProducesValidV4(t *testing.T) {
	t.Parallel()
	bin := specrunBin(t)

	specFile := filepath.Join(t.TempDir(), "to_migrate.spec")
	writeFile(t, specFile, _validV3Spec)

	cmd := exec.Command(bin, "migrate", "--to", "v4", "--write", specFile)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("migrate --write failed: %v\n%s", err, out)
	}

	// File must now exist and contain v4 syntax
	content, err := os.ReadFile(specFile)
	if err != nil {
		t.Fatalf("read migrated file: %v", err)
	}
	migrated := string(content)
	if !strings.Contains(migrated, "contract") {
		t.Errorf("migrated file missing 'contract' keyword:\n%s", migrated)
	}
	// Must not contain v3 scope wrapper (which was the top-level construct)
	if strings.Contains(migrated, "spec Minimal") {
		t.Errorf("migrated file still has v3 'spec Minimal' wrapper:\n%s", migrated)
	}
	// Verify that specrun can parse the migrated file
	parseCmd := exec.Command(bin, "parse", specFile)
	parseOut, parseErr := parseCmd.CombinedOutput()
	if parseErr != nil {
		t.Errorf("migrated file does not parse as v4: %v\n%s", parseErr, parseOut)
	}
}

// TestMigrate_V4_BackupCreated verifies --backup creates a .v3.bak file.
func TestMigrate_V4_BackupCreated(t *testing.T) {
	t.Parallel()
	bin := specrunBin(t)

	dir := t.TempDir()
	specFile := filepath.Join(dir, "backup_test.spec")
	writeFile(t, specFile, _validV3Spec)

	cmd := exec.Command(bin, "migrate", "--to", "v4", "--backup", specFile)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("migrate --backup failed: %v\n%s", err, out)
	}

	bakPath := specFile + ".v3.bak"
	bak, err := os.ReadFile(bakPath)
	if err != nil {
		t.Fatalf(".v3.bak not created: %v", err)
	}
	if string(bak) != _validV3Spec {
		t.Errorf(".v3.bak content mismatch:\ngot:\n%s\nwant:\n%s", bak, _validV3Spec)
	}
	// The spec file itself must have been migrated
	content, err := os.ReadFile(specFile)
	if err != nil {
		t.Fatalf("read migrated file: %v", err)
	}
	if !strings.Contains(string(content), "contract") {
		t.Errorf("migrated file missing 'contract' keyword:\n%s", string(content))
	}
}

// TestMigrate_V4_NoBackupByDefault verifies --backup is NOT created by default.
func TestMigrate_V4_NoBackupByDefault(t *testing.T) {
	t.Parallel()
	bin := specrunBin(t)

	dir := t.TempDir()
	specFile := filepath.Join(dir, "no_bak.spec")
	writeFile(t, specFile, _validV3Spec)

	cmd := exec.Command(bin, "migrate", "--to", "v4", "--write", specFile)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("migrate --write failed: %v\n%s", err, out)
	}

	bakPath := specFile + ".v3.bak"
	if _, err := os.Stat(bakPath); err == nil {
		t.Errorf(".v3.bak was created without --backup flag")
	}
}

// TestMigrate_V4_StringLiteralPreserved verifies && inside a string literal survives migration.
func TestMigrate_V4_StringLiteralPreserved(t *testing.T) {
	t.Parallel()
	bin := specrunBin(t)

	src := `spec API {
  scope check {
    action check(url: string) {
      let result = http.post(url, {})
      return result
    }

    contract {
      input {
        url: string
      }
      output {
        ok: bool
      }
      action: check
    }

    scenario url_with_ampersand {
      given {
        url: "http://example.com?a=1&&b=2"
      }
      then {
        ok == true
      }
    }
  }
}
`
	specFile := filepath.Join(t.TempDir(), "ampersand.spec")
	writeFile(t, specFile, src)

	cmd := exec.Command(bin, "migrate", "--to", "v4", specFile)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("migrate failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), `"http://example.com?a=1&&b=2"`) {
		t.Errorf("string literal with && was corrupted; output:\n%s", out)
	}
}
