package runner_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/bamsammich/speclang/v3/internal/adapter"
	"github.com/bamsammich/speclang/v3/internal/runner"
	"github.com/bamsammich/speclang/v3/pkg/spec"
)

// givenCallsAdapter handles two kinds of calls:
//   - "setup" returns {"id": "g123"} (simulates creating a resource)
//   - "get" returns {"error": null, "name": <group_id>} (simulates the contract action)
//   - "fail" returns {ok: false} to simulate action failure
type givenCallsAdapter struct {
	failAction bool
}

func (a *givenCallsAdapter) Init(_ context.Context, _ map[string]string) error { return nil }
func (a *givenCallsAdapter) Reset() error                                      { return nil }
func (a *givenCallsAdapter) Close(_ context.Context) error                     { return nil }

func (a *givenCallsAdapter) Call(_ context.Context, method string, args json.RawMessage) (*spec.Response, error) {
	switch method {
	case "setup":
		return &spec.Response{OK: true, Actual: json.RawMessage(`{"id":"g123"}`)}, nil
	case "get":
		var rawArgs []json.RawMessage
		if err := json.Unmarshal(args, &rawArgs); err == nil && len(rawArgs) > 0 {
			var body map[string]any
			if err := json.Unmarshal(rawArgs[0], &body); err == nil {
				result := map[string]any{"error": nil}
				if gid, ok := body["group_id"]; ok {
					result["name"] = gid
				}
				b, _ := json.Marshal(result)
				return &spec.Response{OK: true, Actual: b}, nil
			}
		}
		return &spec.Response{OK: true, Actual: json.RawMessage(`{"error":null}`)}, nil
	case "fail":
		if a.failAction {
			return &spec.Response{OK: false, Error: "server_error"}, nil
		}
		return &spec.Response{OK: true, Actual: json.RawMessage(`{"error":"expected_fail"}`)}, nil
	default:
		return &spec.Response{OK: true, Actual: json.RawMessage(`{}`)}, nil
	}
}

// TestGivenWithLetThenContractAction verifies that when a given block has
// let bindings (adapter calls), the contract action still executes and
// assertions evaluate against the action output.
func TestGivenWithLetThenContractAction(t *testing.T) {
	t.Parallel()

	s := &spec.Spec{
		Name: "GivenCallsTest",
		Scopes: []*spec.Scope{{
			Name: "test",
			Contract: &spec.Contract{
				Input:  []*spec.Field{{Name: "group_id", Type: spec.TypeExpr{Name: "string"}}},
				Output: []*spec.Field{{Name: "name", Type: spec.TypeExpr{Name: "string"}}},
				Action: "run",
			},
			Actions: []*spec.ActionDef{{
				Name:   "run",
				Params: []*spec.Param{{Name: "group_id", Type: spec.TypeExpr{Name: "string"}}},
				Body: []spec.GivenStep{
					&spec.LetBinding{
						Name: "result",
						Value: spec.AdapterCall{
							Adapter: "http",
							Method:  "get",
							Args: []spec.Expr{spec.ObjectLiteral{
								Fields: []*spec.ObjField{{Key: "group_id", Value: spec.FieldRef{Path: "group_id"}}},
							}},
						},
					},
					&spec.ReturnStmt{Value: spec.FieldRef{Path: "result"}},
				},
			}},
			Scenarios: []*spec.Scenario{{
				Name: "dynamic_input",
				Given: &spec.Block{
					Steps: []spec.GivenStep{
						// Setup call: create a resource, get its ID.
						&spec.LetBinding{
							Name: "r0",
							Value: spec.AdapterCall{
								Adapter: "http",
								Method:  "setup",
								Args:    []spec.Expr{},
							},
						},
						// Use the returned ID as contract input.
						&spec.Assignment{
							Path:  "group_id",
							Value: spec.FieldRef{Path: "r0.id"},
						},
					},
				},
				Then: &spec.Block{
					Assertions: []*spec.Assertion{{
						Expr: spec.BinaryOp{
							Left:  spec.FieldRef{Path: "name"},
							Op:    "==",
							Right: spec.LiteralString{Value: "g123"},
						},
					}},
				},
			}},
		}},
	}

	adp := &givenCallsAdapter{}
	r := runner.New(s, map[string]adapter.Adapter{"http": adp}, 42)
	res, err := r.Verify(context.Background())
	if err != nil {
		t.Fatalf("verify error: %v", err)
	}
	if len(res.Failures) > 0 {
		t.Errorf("expected pass, got failure: %s", res.Failures[0].Description)
	}
}

// TestGivenWithLetErrorPropagation verifies that when the contract action
// fails after a calls-path given block, the error is propagated.
func TestGivenWithLetErrorPropagation(t *testing.T) {
	t.Parallel()

	s := &spec.Spec{
		Name: "GivenCallsErrorTest",
		Scopes: []*spec.Scope{{
			Name: "test",
			Contract: &spec.Contract{
				Input:  []*spec.Field{{Name: "group_id", Type: spec.TypeExpr{Name: "string"}}},
				Output: []*spec.Field{{Name: "error", Type: spec.TypeExpr{Name: "string", Optional: true}}},
				Action: "run",
			},
			Actions: []*spec.ActionDef{{
				Name:   "run",
				Params: []*spec.Param{{Name: "group_id", Type: spec.TypeExpr{Name: "string"}}},
				Body: []spec.GivenStep{
					&spec.LetBinding{
						Name: "result",
						Value: spec.AdapterCall{
							Adapter: "http",
							Method:  "fail",
							Args:    []spec.Expr{},
						},
					},
					&spec.ReturnStmt{Value: spec.FieldRef{Path: "result"}},
				},
			}},
			Scenarios: []*spec.Scenario{{
				Name: "action_fails",
				Given: &spec.Block{
					Steps: []spec.GivenStep{
						&spec.LetBinding{
							Name:  "r0",
							Value: spec.AdapterCall{Adapter: "http", Method: "setup"},
						},
						&spec.Assignment{Path: "group_id", Value: spec.FieldRef{Path: "r0.id"}},
					},
				},
				Then: &spec.Block{
					Assertions: []*spec.Assertion{{
						Expr: spec.BinaryOp{
							Left:  spec.FieldRef{Path: "error"},
							Op:    "==",
							Right: spec.LiteralNull{},
						},
					}},
				},
			}},
		}},
	}

	adp := &givenCallsAdapter{failAction: true}
	r := runner.New(s, map[string]adapter.Adapter{"http": adp}, 42)
	_, err := r.Verify(context.Background())
	if err == nil {
		t.Fatal("expected error from failed action, got nil")
	}
	if !strings.Contains(err.Error(), "action failed") {
		t.Errorf("expected 'action failed' error, got: %v", err)
	}
}

// TestGivenWithLetExpectedError verifies that when a contract action fails
// with an expected error, the error pseudo-field assertion passes.
func TestGivenWithLetExpectedError(t *testing.T) {
	t.Parallel()

	s := &spec.Spec{
		Name: "GivenCallsExpectedErrorTest",
		Scopes: []*spec.Scope{{
			Name: "test",
			Contract: &spec.Contract{
				Input:  []*spec.Field{{Name: "group_id", Type: spec.TypeExpr{Name: "string"}}},
				Output: []*spec.Field{{Name: "error", Type: spec.TypeExpr{Name: "string", Optional: true}}},
				Action: "run",
			},
			Actions: []*spec.ActionDef{{
				Name:   "run",
				Params: []*spec.Param{{Name: "group_id", Type: spec.TypeExpr{Name: "string"}}},
				Body: []spec.GivenStep{
					&spec.LetBinding{
						Name: "result",
						Value: spec.AdapterCall{
							Adapter: "http",
							Method:  "fail",
							Args:    []spec.Expr{},
						},
					},
					&spec.ReturnStmt{Value: spec.FieldRef{Path: "result"}},
				},
			}},
			Scenarios: []*spec.Scenario{{
				Name: "expected_error",
				Given: &spec.Block{
					Steps: []spec.GivenStep{
						&spec.LetBinding{
							Name:  "r0",
							Value: spec.AdapterCall{Adapter: "http", Method: "setup"},
						},
						&spec.Assignment{Path: "group_id", Value: spec.FieldRef{Path: "r0.id"}},
					},
				},
				Then: &spec.Block{
					Assertions: []*spec.Assertion{{
						Expr: spec.BinaryOp{
							Left:  spec.FieldRef{Path: "error"},
							Op:    "==",
							Right: spec.LiteralString{Value: "expected_fail"},
						},
					}},
				},
			}},
		}},
	}

	adp := &givenCallsAdapter{}
	r := runner.New(s, map[string]adapter.Adapter{"http": adp}, 42)
	res, err := r.Verify(context.Background())
	if err != nil {
		t.Fatalf("verify error: %v", err)
	}
	if len(res.Failures) > 0 {
		t.Errorf("expected pass, got failure: %s", res.Failures[0].Description)
	}
}
