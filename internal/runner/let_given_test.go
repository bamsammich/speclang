package runner_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/bamsammich/speclang/v3/internal/adapter"
	"github.com/bamsammich/speclang/v3/internal/runner"
	"github.com/bamsammich/speclang/v3/pkg/spec"
)

// letGivenAdapter returns a JSON response so let bindings can extract fields.
type letGivenAdapter struct{}

func (a *letGivenAdapter) Init(_ context.Context, _ map[string]string) error { return nil }
func (a *letGivenAdapter) Reset() error                                      { return nil }
func (a *letGivenAdapter) Close(_ context.Context) error                     { return nil }

func (a *letGivenAdapter) Call(_ context.Context, method string, _ json.RawMessage) (*spec.Response, error) {
	switch method {
	case "post":
		return &spec.Response{
			OK:     true,
			Actual: json.RawMessage(`{"group":{"id":"g123","name":"Test"}}`),
		}, nil
	default:
		return &spec.Response{OK: true, Actual: json.RawMessage(`{"group_id":"g123"}`)}, nil
	}
}

// TestLetBindingInGivenBlock verifies that let bindings in given blocks
// are evaluated and available for subsequent assignments.
func TestLetBindingInGivenBlock(t *testing.T) {
	t.Parallel()

	s := &spec.Spec{
		Name: "LetGivenTest",
		Scopes: []*spec.Scope{{
			Name: "test",
			Contract: &spec.Contract{
				Input:  []*spec.Field{{Name: "group_id", Type: spec.TypeExpr{Name: "string"}}},
				Output: []*spec.Field{{Name: "group_id", Type: spec.TypeExpr{Name: "string"}}},
				Action: "run",
			},
			Actions: []*spec.ActionDef{{
				Name:   "run",
				Params: []*spec.Param{{Name: "group_id", Type: spec.TypeExpr{Name: "string"}}},
				Body: []spec.GivenStep{
					&spec.ReturnStmt{Value: spec.ObjectLiteral{
						Fields: []*spec.ObjField{{Key: "group_id", Value: spec.FieldRef{Path: "group_id"}}},
					}},
				},
			}},
			Scenarios: []*spec.Scenario{{
				Name: "let_in_given",
				Given: &spec.Block{
					Steps: []spec.GivenStep{
						&spec.LetBinding{
							Name: "r0",
							Value: spec.AdapterCall{
								Adapter: "http",
								Method:  "post",
								Args:    []spec.Expr{spec.LiteralString{Value: "/api/groups"}},
							},
						},
						&spec.Assignment{
							Path:  "group_id",
							Value: spec.FieldRef{Path: "r0.group.id"},
						},
					},
				},
				Then: &spec.Block{
					Assertions: []*spec.Assertion{{
						Expr: spec.BinaryOp{
							Left:  spec.FieldRef{Path: "group_id"},
							Op:    "==",
							Right: spec.LiteralString{Value: "g123"},
						},
					}},
				},
			}},
		}},
	}

	adp := &letGivenAdapter{}
	r := runner.New(s, map[string]adapter.Adapter{"http": adp}, 42)
	res, err := r.Verify(context.Background())
	if err != nil {
		t.Fatalf("verify error: %v", err)
	}

	if len(res.Failures) > 0 {
		t.Errorf("expected pass, got failure: %s", res.Failures[0].Description)
		for _, f := range res.Failures {
			inputJSON, _ := json.Marshal(f.Input)
		t.Logf("  failure: %s (input: %s)", f.Description, string(inputJSON))
		}
	}
}
