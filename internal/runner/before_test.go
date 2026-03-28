package runner_test

import (
	"encoding/json"
	"fmt"
	"sync"
	"testing"

	"github.com/bamsammich/speclang/v2/internal/adapter"
	"github.com/bamsammich/speclang/v2/internal/runner"
	"github.com/bamsammich/speclang/v2/pkg/spec"
)

// recordingAdapter is a test adapter that records all calls in order.
type recordingAdapter struct {
	mu   sync.Mutex
	log  []string
	fail map[string]string // action name -> error message (to simulate failures)
}

func newRecordingAdapter() *recordingAdapter {
	return &recordingAdapter{
		fail: make(map[string]string),
	}
}

func (a *recordingAdapter) Init(map[string]string) error { return nil }

func (a *recordingAdapter) Action(name string, args json.RawMessage) (*spec.Response, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.log = append(a.log, "action:"+name)

	if errMsg, ok := a.fail[name]; ok {
		return &spec.Response{OK: false, Error: errMsg}, nil
	}

	// Return a minimal valid JSON response.
	return &spec.Response{OK: true, Actual: json.RawMessage(`{}`)}, nil
}

func (a *recordingAdapter) Assert(property, locator string, expected json.RawMessage) (*spec.Response, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.log = append(a.log, "assert:"+property)
	return &spec.Response{OK: true, Actual: expected}, nil
}

func (a *recordingAdapter) Reset() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.log = append(a.log, "reset")
	return nil
}

func (a *recordingAdapter) Close() error { return nil }

func (a *recordingAdapter) calls() []string {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := make([]string, len(a.log))
	copy(out, a.log)
	return out
}

// failingResetAdapter always returns an error from Reset.
type failingResetAdapter struct {
	recordingAdapter
}

func (a *failingResetAdapter) Reset() error {
	return fmt.Errorf("reset failed: simulated error")
}

// --- helpers to build minimal specs ---

func minimalSpec(scopes []*spec.Scope) *spec.Spec {
	return &spec.Spec{
		Name:   "test",
		Scopes: scopes,
	}
}

func minimalContract() *spec.Contract {
	return &spec.Contract{
		Input: []*spec.Field{
			{Name: "x", Type: spec.TypeExpr{Name: "int"}},
		},
		Output: []*spec.Field{
			{Name: "result", Type: spec.TypeExpr{Name: "int"}},
		},
	}
}

func givenScenario(name string, value int) *spec.Scenario {
	return &spec.Scenario{
		Name: name,
		Given: &spec.Block{
			Steps: []spec.GivenStep{
				&spec.Assignment{Path: "x", Value: spec.LiteralInt{Value: value}},
			},
		},
		Then: &spec.Block{
			Assertions: []*spec.Assertion{
				{Target: "result", Expected: spec.LiteralInt{Value: value}},
			},
		},
	}
}

// TestBeforeStepsExecuteBeforeGiven verifies that before block actions run
// before the given block's main action for a given-scenario.
func TestBeforeStepsExecuteBeforeGiven(t *testing.T) {
	adp := newRecordingAdapter()

	beforeBlock := &spec.Block{
		Steps: []spec.GivenStep{
			&spec.Call{Namespace: "http", Method: "setup"},
		},
	}

	s := minimalSpec([]*spec.Scope{
		{
			Name:     "test_scope",
			Use:      "http",
			Config:   map[string]spec.Expr{"method": spec.LiteralString{Value: "POST"}, "path": spec.LiteralString{Value: "/api/test"}},
			Before:   beforeBlock,
			Contract: minimalContract(),
			Scenarios: []*spec.Scenario{
				givenScenario("basic", 42),
			},
		},
	})

	r := runner.New(s, map[string]adapter.Adapter{"http": adp}, 1)
	r.SetN(1)
	_, err := r.Verify()
	if err != nil {
		t.Fatalf("verify failed: %v", err)
	}

	calls := adp.calls()
	// Expect: reset, action:setup (before), then the main action (post)
	if len(calls) < 3 {
		t.Fatalf("expected at least 3 calls, got %d: %v", len(calls), calls)
	}
	if calls[0] != "reset" {
		t.Errorf("expected first call to be 'reset', got %q", calls[0])
	}
	if calls[1] != "action:setup" {
		t.Errorf("expected second call to be 'action:setup', got %q", calls[1])
	}
	if calls[2] != "action:post" {
		t.Errorf("expected third call to be 'action:post', got %q", calls[2])
	}
}

// TestBeforeResetBetweenScenarios verifies that Reset is called before each scenario,
// so state does not leak between them.
func TestBeforeResetBetweenScenarios(t *testing.T) {
	adp := newRecordingAdapter()

	s := minimalSpec([]*spec.Scope{
		{
			Name:   "test_scope",
			Use:    "http",
			Config: map[string]spec.Expr{"method": spec.LiteralString{Value: "POST"}, "path": spec.LiteralString{Value: "/api/test"}},
			Before: &spec.Block{
				Steps: []spec.GivenStep{
					&spec.Call{Namespace: "http", Method: "seed_data"},
				},
			},
			Contract: minimalContract(),
			Scenarios: []*spec.Scenario{
				givenScenario("first", 1),
				givenScenario("second", 2),
			},
		},
	})

	r := runner.New(s, map[string]adapter.Adapter{"http": adp}, 1)
	r.SetN(1)
	_, err := r.Verify()
	if err != nil {
		t.Fatalf("verify failed: %v", err)
	}

	calls := adp.calls()
	// Count resets — should be at least 2 (one per scenario).
	resetCount := 0
	for _, c := range calls {
		if c == "reset" {
			resetCount++
		}
	}
	if resetCount < 2 {
		t.Errorf("expected at least 2 resets (one per scenario), got %d; calls: %v", resetCount, calls)
	}
}

// TestBeforeFailureReturnsError verifies that when a before block action fails,
// the scope fails with a before-related error.
func TestBeforeFailureReturnsError(t *testing.T) {
	adp := newRecordingAdapter()
	adp.fail["setup"] = "setup failed: db unavailable"

	s := minimalSpec([]*spec.Scope{
		{
			Name:   "test_scope",
			Use:    "http",
			Config: map[string]spec.Expr{"method": spec.LiteralString{Value: "POST"}, "path": spec.LiteralString{Value: "/api/test"}},
			Before: &spec.Block{
				Steps: []spec.GivenStep{
					&spec.Call{Namespace: "http", Method: "setup"},
				},
			},
			Contract: minimalContract(),
			Scenarios: []*spec.Scenario{
				givenScenario("basic", 1),
			},
		},
	})

	r := runner.New(s, map[string]adapter.Adapter{"http": adp}, 1)
	r.SetN(1)
	_, err := r.Verify()
	if err == nil {
		t.Fatal("expected verify to return an error when before block fails, but got nil")
	}

	// The error should mention "before block".
	if got := err.Error(); !contains(got, "before") {
		t.Errorf("expected error to mention 'before', got: %s", got)
	}
}

// TestBeforeBodyRefResolution verifies that body.field references in before/given
// steps resolve from the previous action's response (#97).
func TestBeforeBodyRefResolution(t *testing.T) {
	adp := &bodyRefAdapter{}

	// before block: post to login (returns {"access_token":"tok123"}),
	// then call header with "Bearer " + body.access_token
	beforeBlock := &spec.Block{
		Steps: []spec.GivenStep{
			&spec.Call{Namespace: "http", Method: "post", Args: []spec.Expr{
				spec.LiteralString{Value: "/auth/login"},
				spec.ObjectLiteral{},
			}},
			&spec.Call{Namespace: "http", Method: "header", Args: []spec.Expr{
				spec.LiteralString{Value: "Authorization"},
				spec.BinaryOp{
					Left:  spec.LiteralString{Value: "Bearer "},
					Op:    "+",
					Right: spec.FieldRef{Path: "body.access_token"},
				},
			}},
		},
	}

	s := minimalSpec([]*spec.Scope{
		{
			Name:      "test_scope",
			Use:       "http",
			Config:    map[string]spec.Expr{"method": spec.LiteralString{Value: "POST"}, "path": spec.LiteralString{Value: "/api/test"}},
			Before:    beforeBlock,
			Contract:  minimalContract(),
			Scenarios: []*spec.Scenario{givenScenario("basic", 42)},
		},
	})

	r := runner.New(s, map[string]adapter.Adapter{"http": adp}, 1)
	r.SetN(1)
	_, err := r.Verify()
	if err != nil {
		t.Fatalf("verify failed: %v", err)
	}

	// The header call should have received "Bearer tok123" as the second arg.
	if adp.lastHeaderValue != "Bearer tok123" {
		t.Errorf("expected header value 'Bearer tok123', got %q", adp.lastHeaderValue)
	}
}

// bodyRefAdapter returns a JSON body from "post" and records "header" args.
type bodyRefAdapter struct {
	lastHeaderValue string
}

func (a *bodyRefAdapter) Init(map[string]string) error { return nil }
func (a *bodyRefAdapter) Reset() error                 { return nil }
func (a *bodyRefAdapter) Close() error                 { return nil }

func (a *bodyRefAdapter) Action(name string, args json.RawMessage) (*spec.Response, error) {
	switch name {
	case "post":
		return &spec.Response{
			OK:     true,
			Actual: json.RawMessage(`{"access_token":"tok123"}`),
		}, nil
	case "header":
		var rawArgs []json.RawMessage
		if err := json.Unmarshal(args, &rawArgs); err == nil && len(rawArgs) >= 2 {
			var val string
			json.Unmarshal(rawArgs[1], &val)
			a.lastHeaderValue = val
		}
		return &spec.Response{OK: true, Actual: json.RawMessage(`{}`)}, nil
	default:
		return &spec.Response{OK: true, Actual: json.RawMessage(`{}`)}, nil
	}
}

func (a *bodyRefAdapter) Assert(property, locator string, expected json.RawMessage) (*spec.Response, error) {
	return &spec.Response{OK: true, Actual: expected}, nil
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
