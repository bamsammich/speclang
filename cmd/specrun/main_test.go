package main

import (
	"encoding/json"
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
