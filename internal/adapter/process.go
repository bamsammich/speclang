package adapter

import (
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

type lastExecResult struct {
	stdout    map[string]any
	rawStdout string
	rawStderr string
	exitCode  int
}

// ProcessAdapter is the built-in adapter for testing subprocesses.
type ProcessAdapter struct {
	last     *lastExecResult
	Command  string
	baseArgs []string
}

func NewProcessAdapter() *ProcessAdapter {
	return &ProcessAdapter{}
}

func (a *ProcessAdapter) Init(config map[string]string) error {
	cmd, ok := config["command"]
	if !ok {
		return errors.New("process adapter requires 'command' in config")
	}
	a.Command = cmd
	if argsStr, ok := config["args"]; ok && argsStr != "" {
		a.baseArgs = strings.Fields(argsStr)
	}
	return nil
}

func (a *ProcessAdapter) Call(method string, args json.RawMessage) (*Response, error) {
	// Action: exec
	if method == "exec" {
		return a.doExec(args)
	}

	// Queries — return current value in Actual
	if a.last == nil {
		return nil, errors.New("no command has been executed yet")
	}

	var actual any

	switch {
	case method == "exit_code":
		actual = float64(a.last.exitCode)
	case method == "stderr":
		actual = a.last.rawStderr
	case method == "stdout" && a.last.stdout == nil:
		actual = a.last.rawStdout
	case method == "stdout":
		actual = a.last.stdout
	default:
		path := strings.TrimPrefix(method, "stdout.")
		if a.last.stdout == nil {
			return nil, fmt.Errorf("stdout is not JSON, cannot extract path %q", path)
		}
		val, err := extractPath(a.last.stdout, path)
		if err != nil {
			return nil, err
		}
		actual = val
	}

	actualJSON, err := json.Marshal(actual)
	if err != nil {
		return nil, fmt.Errorf("marshaling actual value: %w", err)
	}
	return &Response{OK: true, Actual: actualJSON}, nil
}

func (a *ProcessAdapter) doExec(args json.RawMessage) (*Response, error) {
	var rawArgs []json.RawMessage
	if len(args) > 0 {
		if err := json.Unmarshal(args, &rawArgs); err != nil {
			return nil, fmt.Errorf("parsing action args: %w", err)
		}
	}

	cmdArgs := make([]string, len(a.baseArgs))
	copy(cmdArgs, a.baseArgs)
	for _, raw := range rawArgs {
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			s = strings.TrimSpace(string(raw))
		}
		cmdArgs = append(cmdArgs, s)
	}

	//nolint:gosec // process adapter intentionally executes user-specified commands from spec config
	cmd := exec.Command(a.Command, cmdArgs...)
	var stdoutBuf, stderrBuf strings.Builder
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	err := cmd.Run()

	exitCode := 0
	if err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			return nil, fmt.Errorf("executing %s: %w", a.Command, err)
		}
		exitCode = exitErr.ExitCode()
	}

	stdout := stdoutBuf.String()
	stderr := stderrBuf.String()

	var parsed map[string]any
	//nolint:errcheck // best-effort JSON parse; stdout may not be JSON
	_ = json.Unmarshal([]byte(strings.TrimSpace(stdout)), &parsed)

	// Build the result that gets returned as Actual (for the runner's executeInput)
	result := map[string]any{
		"exit_code": exitCode,
	}
	for k, v := range parsed {
		result[k] = v
	}
	resultJSON, _ := json.Marshal(result) //nolint:errcheck // result is always marshallable

	a.last = &lastExecResult{
		exitCode:  exitCode,
		rawStdout: stdout,
		rawStderr: stderr,
		stdout:    parsed,
	}

	return &Response{OK: true, Actual: json.RawMessage(resultJSON)}, nil
}

func (a *ProcessAdapter) Reset() error {
	a.last = nil
	return nil
}

func (*ProcessAdapter) Close() error {
	return nil
}
