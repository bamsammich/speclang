package adapter

import (
	"encoding/json"
	"testing"
)

func TestProcessAdapter_ExecEcho(t *testing.T) {
	a := NewProcessAdapter()
	if err := a.Init(map[string]string{"command": "echo"}); err != nil {
		t.Fatal(err)
	}

	args := mustMarshal(t, []any{"hello", "world"})
	resp, err := a.Action("exec", args)
	if err != nil {
		t.Fatal(err)
	}
	if !resp.OK {
		t.Fatalf("exec failed: %s", resp.Error)
	}

	exitResp, err := a.Assert("exit_code", "", json.RawMessage(`0`))
	if err != nil {
		t.Fatal(err)
	}
	if !exitResp.OK {
		t.Fatalf("exit_code assertion failed: %s", exitResp.Error)
	}
}

func TestProcessAdapter_ExecWithBaseArgs(t *testing.T) {
	a := NewProcessAdapter()
	if err := a.Init(map[string]string{
		"command": "echo",
		"args":    "base arg",
	}); err != nil {
		t.Fatal(err)
	}

	args := mustMarshal(t, []any{"extra"})
	resp, err := a.Action("exec", args)
	if err != nil {
		t.Fatal(err)
	}
	if !resp.OK {
		t.Fatal("exec failed")
	}

	stdoutResp, err := a.Assert("stdout", "", json.RawMessage(`"base arg extra\n"`))
	if err != nil {
		t.Fatal(err)
	}
	if !stdoutResp.OK {
		t.Fatalf("stdout assertion failed: %s (actual: %s)", stdoutResp.Error, string(stdoutResp.Actual))
	}
}

func TestProcessAdapter_ExitCodeNonZero(t *testing.T) {
	a := NewProcessAdapter()
	if err := a.Init(map[string]string{"command": "false"}); err != nil {
		t.Fatal(err)
	}

	args := mustMarshal(t, []any{})
	_, err := a.Action("exec", args)
	if err != nil {
		t.Fatal(err)
	}

	exitResp, err := a.Assert("exit_code", "", json.RawMessage(`1`))
	if err != nil {
		t.Fatal(err)
	}
	if !exitResp.OK {
		t.Fatalf("expected exit_code=1, got: %s", string(exitResp.Actual))
	}
}

func TestProcessAdapter_JSONStdout(t *testing.T) {
	a := NewProcessAdapter()
	if err := a.Init(map[string]string{"command": "echo"}); err != nil {
		t.Fatal(err)
	}

	args := mustMarshal(t, []any{`{"name":"test","value":42}`})
	if _, err := a.Action("exec", args); err != nil {
		t.Fatal(err)
	}

	resp, err := a.Assert("stdout.name", "", json.RawMessage(`"test"`))
	if err != nil {
		t.Fatal(err)
	}
	if !resp.OK {
		t.Fatalf("stdout.name assertion failed: %s", resp.Error)
	}

	resp, err = a.Assert("stdout.value", "", json.RawMessage(`42`))
	if err != nil {
		t.Fatal(err)
	}
	if !resp.OK {
		t.Fatalf("stdout.value assertion failed: %s", resp.Error)
	}
}

func TestProcessAdapter_AssertBeforeExec(t *testing.T) {
	a := NewProcessAdapter()
	a.Command = "echo"

	_, err := a.Assert("exit_code", "", json.RawMessage(`0`))
	if err == nil {
		t.Fatal("expected error when asserting before exec")
	}
}

func TestProcessAdapter_UnknownAction(t *testing.T) {
	a := NewProcessAdapter()
	a.Command = "echo"

	_, err := a.Action("unknown", json.RawMessage(`[]`))
	if err == nil {
		t.Fatal("expected error for unknown action")
	}
}
