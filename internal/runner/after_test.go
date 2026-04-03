package runner_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/bamsammich/speclang/v3/internal/adapter"
	"github.com/bamsammich/speclang/v3/internal/runner"
	"github.com/bamsammich/speclang/v3/pkg/spec"
)

// TestAfterStepsExecuteAfterScenario verifies that after block actions run
// after the given block's main action for a given-scenario.
func TestAfterStepsExecuteAfterScenario(t *testing.T) {
	adp := newRecordingAdapter()

	afterBlock := &spec.Block{
		Steps: []spec.GivenStep{
			&spec.Call{Namespace: "http", Method: "cleanup"},
		},
	}

	s := minimalSpec([]*spec.Scope{
		{
			Name:     "test_scope",
			Use:      "http",
			Config:   map[string]spec.Expr{"method": spec.LiteralString{Value: "POST"}, "path": spec.LiteralString{Value: "/api/test"}},
			After:    afterBlock,
			Contract: minimalContract(),
			Scenarios: []*spec.Scenario{
				givenScenario("basic", 42),
			},
		},
	})

	r := runner.New(s, map[string]adapter.Adapter{"http": adp}, 1)
	r.SetN(1)
	_, err := r.Verify(context.Background())
	if err != nil {
		t.Fatalf("verify failed: %v", err)
	}

	calls := adp.calls()
	// Expect: reset, action:post (main), action:result (assertion query), action:cleanup (after)
	if len(calls) < 4 {
		t.Fatalf("expected at least 4 calls, got %d: %v", len(calls), calls)
	}
	if calls[0] != "reset" {
		t.Errorf("expected first call to be 'reset', got %q", calls[0])
	}
	if calls[1] != "action:post" {
		t.Errorf("expected second call to be 'action:post', got %q", calls[1])
	}
	// The last call should be cleanup (after block)
	last := calls[len(calls)-1]
	if last != "action:cleanup" {
		t.Errorf("expected last call to be 'action:cleanup', got %q; all calls: %v", last, calls)
	}
}

// TestAfterExecutesBetweenScenarios verifies that after runs between scenarios.
func TestAfterExecutesBetweenScenarios(t *testing.T) {
	adp := newRecordingAdapter()

	s := minimalSpec([]*spec.Scope{
		{
			Name:   "test_scope",
			Use:    "http",
			Config: map[string]spec.Expr{"method": spec.LiteralString{Value: "POST"}, "path": spec.LiteralString{Value: "/api/test"}},
			After: &spec.Block{
				Steps: []spec.GivenStep{
					&spec.Call{Namespace: "http", Method: "cleanup"},
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
	_, err := r.Verify(context.Background())
	if err != nil {
		t.Fatalf("verify failed: %v", err)
	}

	calls := adp.calls()
	// Count cleanup calls — should be at least 2 (one per scenario).
	cleanupCount := 0
	for _, c := range calls {
		if c == "action:cleanup" {
			cleanupCount++
		}
	}
	if cleanupCount < 2 {
		t.Errorf("expected at least 2 cleanup calls (one per scenario), got %d; calls: %v", cleanupCount, calls)
	}
}

// TestAfterExecutesOnFailure verifies that after runs even when the scenario fails.
func TestAfterExecutesOnFailure(t *testing.T) {
	adp := newRecordingAdapter()
	// Make the main action fail
	adp.fail["post"] = "server error"

	s := minimalSpec([]*spec.Scope{
		{
			Name:   "test_scope",
			Use:    "http",
			Config: map[string]spec.Expr{"method": spec.LiteralString{Value: "POST"}, "path": spec.LiteralString{Value: "/api/test"}},
			After: &spec.Block{
				Steps: []spec.GivenStep{
					&spec.Call{Namespace: "http", Method: "cleanup"},
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
	_, _ = r.Verify(context.Background()) // may return error, that's fine

	calls := adp.calls()
	// The cleanup should still have been called despite the main action failing.
	found := false
	for _, c := range calls {
		if c == "action:cleanup" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected cleanup to run even on failure, calls: %v", calls)
	}
}

// TestAfterErrorDoesNotAffectResult verifies that after block errors don't mask
// a passing test result.
func TestAfterErrorDoesNotAffectResult(t *testing.T) {
	adp := &afterPassingAdapter{
		failCleanup: true,
	}

	s := minimalSpec([]*spec.Scope{
		{
			Name:   "test_scope",
			Use:    "http",
			Config: map[string]spec.Expr{"method": spec.LiteralString{Value: "POST"}, "path": spec.LiteralString{Value: "/api/test"}},
			After: &spec.Block{
				Steps: []spec.GivenStep{
					&spec.Call{Namespace: "http", Method: "cleanup"},
				},
			},
			Contract: minimalContract(),
			Scenarios: []*spec.Scenario{
				givenScenario("basic", 42),
			},
		},
	})

	r := runner.New(s, map[string]adapter.Adapter{"http": adp}, 1)
	r.SetN(1)
	result, err := r.Verify(context.Background())
	if err != nil {
		t.Fatalf("expected verify to succeed despite after block error, got: %v", err)
	}

	// The test should still pass despite the after block error.
	if len(result.Failures) > 0 {
		t.Errorf("expected no failures, got %d", len(result.Failures))
	}
}

// afterPassingAdapter returns correct values so assertions pass, but fails cleanup.
type afterPassingAdapter struct {
	failCleanup bool
}

func (a *afterPassingAdapter) Init(_ context.Context, _ map[string]string) error { return nil }
func (a *afterPassingAdapter) Reset() error                                      { return nil }
func (a *afterPassingAdapter) Close(_ context.Context) error                     { return nil }

func (a *afterPassingAdapter) Call(_ context.Context, method string, args json.RawMessage) (*spec.Response, error) {
	switch method {
	case "cleanup":
		if a.failCleanup {
			return &spec.Response{OK: false, Error: "cleanup failed: db locked"}, nil
		}
		return &spec.Response{OK: true, Actual: json.RawMessage(`{}`)}, nil
	case "result":
		// Return the expected value for the assertion
		return &spec.Response{OK: true, Actual: json.RawMessage(`42`)}, nil
	default:
		return &spec.Response{OK: true, Actual: json.RawMessage(`{}`)}, nil
	}
}
