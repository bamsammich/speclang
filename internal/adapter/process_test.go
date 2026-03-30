package adapter

import (
	"encoding/json"
	"testing"
)

func TestProcessAdapter_ExecEcho(t *testing.T) {
	t.Parallel()
	a := NewProcessAdapter()
	if err := a.Init(map[string]string{"command": "echo"}); err != nil {
		t.Fatal(err)
	}

	args := mustMarshal(t, []any{"hello", "world"})
	resp, err := a.Call("exec", args)
	if err != nil {
		t.Fatal(err)
	}
	if !resp.OK {
		t.Fatalf("exec failed: %s", resp.Error)
	}

	exitResp, err := a.Call("exit_code", nil)
	if err != nil {
		t.Fatal(err)
	}
	if string(exitResp.Actual) != "0" {
		t.Fatalf("expected exit_code=0, got %s", string(exitResp.Actual))
	}
}

func TestProcessAdapter_ExecWithBaseArgs(t *testing.T) {
	t.Parallel()
	a := NewProcessAdapter()
	if err := a.Init(map[string]string{
		"command": "echo",
		"args":    "base arg",
	}); err != nil {
		t.Fatal(err)
	}

	args := mustMarshal(t, []any{"extra"})
	resp, err := a.Call("exec", args)
	if err != nil {
		t.Fatal(err)
	}
	if !resp.OK {
		t.Fatal("exec failed")
	}

	stdoutResp, err := a.Call("stdout", nil)
	if err != nil {
		t.Fatal(err)
	}
	if string(stdoutResp.Actual) != `"base arg extra\n"` {
		t.Fatalf("stdout: expected %q, got %s", "base arg extra\\n", string(stdoutResp.Actual))
	}
}

func TestProcessAdapter_ExitCodeNonZero(t *testing.T) {
	t.Parallel()
	a := NewProcessAdapter()
	if err := a.Init(map[string]string{"command": "false"}); err != nil {
		t.Fatal(err)
	}

	args := mustMarshal(t, []any{})
	_, err := a.Call("exec", args)
	if err != nil {
		t.Fatal(err)
	}

	exitResp, err := a.Call("exit_code", nil)
	if err != nil {
		t.Fatal(err)
	}
	if string(exitResp.Actual) != "1" {
		t.Fatalf("expected exit_code=1, got: %s", string(exitResp.Actual))
	}
}

func TestProcessAdapter_JSONStdout(t *testing.T) {
	t.Parallel()
	a := NewProcessAdapter()
	if err := a.Init(map[string]string{"command": "echo"}); err != nil {
		t.Fatal(err)
	}

	args := mustMarshal(t, []any{`{"name":"test","value":42}`})
	if _, err := a.Call("exec", args); err != nil {
		t.Fatal(err)
	}

	resp, err := a.Call("stdout.name", nil)
	if err != nil {
		t.Fatal(err)
	}
	if string(resp.Actual) != `"test"` {
		t.Fatalf("stdout.name: expected %q, got %s", "test", string(resp.Actual))
	}

	resp, err = a.Call("stdout.value", nil)
	if err != nil {
		t.Fatal(err)
	}
	if string(resp.Actual) != "42" {
		t.Fatalf("stdout.value: expected 42, got %s", string(resp.Actual))
	}
}

func TestProcessAdapter_Stderr(t *testing.T) {
	t.Parallel()
	a := NewProcessAdapter()
	if err := a.Init(map[string]string{"command": "sh"}); err != nil {
		t.Fatal(err)
	}

	// "sh -c" writes to stderr
	args := mustMarshal(t, []any{"-c", "echo error-output >&2"})
	if _, err := a.Call("exec", args); err != nil {
		t.Fatal(err)
	}

	resp, err := a.Call("stderr", nil)
	if err != nil {
		t.Fatal(err)
	}
	if string(resp.Actual) != `"error-output\n"` {
		t.Fatalf("stderr: expected %q, got %s", "error-output\\n", string(resp.Actual))
	}
}

func TestProcessAdapter_QueryBeforeExec(t *testing.T) {
	t.Parallel()
	a := NewProcessAdapter()
	a.Command = "echo"

	_, err := a.Call("exit_code", nil)
	if err == nil {
		t.Fatal("expected error when querying before exec")
	}
}

func TestProcessAdapter_UnknownExecOnly(t *testing.T) {
	t.Parallel()
	a := NewProcessAdapter()
	a.Command = "echo"

	// Unknown methods that aren't "exec" and there's no prior exec result
	_, err := a.Call("unknown", json.RawMessage(`[]`))
	if err == nil {
		t.Fatal("expected error for unknown method with no prior exec")
	}
}
