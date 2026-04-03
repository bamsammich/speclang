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

// optionalFieldAdapter echoes back input fields so assertions can check them.
type optionalFieldAdapter struct{}

func (a *optionalFieldAdapter) Init(_ context.Context, _ map[string]string) error { return nil }
func (a *optionalFieldAdapter) Reset() error                                      { return nil }
func (a *optionalFieldAdapter) Close(_ context.Context) error                     { return nil }

func (a *optionalFieldAdapter) Call(_ context.Context, _ string, args json.RawMessage) (*spec.Response, error) {
	// Echo args back as the response — the action body returns them as output.
	var rawArgs []json.RawMessage
	if err := json.Unmarshal(args, &rawArgs); err != nil || len(rawArgs) == 0 {
		return &spec.Response{OK: true, Actual: json.RawMessage(`{}`)}, nil
	}
	return &spec.Response{OK: true, Actual: rawArgs[0]}, nil
}

// buildOptionalFieldSpec creates a spec with one required field and one optional field.
// The action posts the input as JSON so we can assert on it.
func buildOptionalFieldSpec(givenSteps []spec.GivenStep, thenAssertions []*spec.Assertion) *spec.Spec {
	return &spec.Spec{
		Name: "OptionalFieldTest",
		Scopes: []*spec.Scope{{
			Name: "test",
			Contract: &spec.Contract{
				Input: []*spec.Field{
					{Name: "name", Type: spec.TypeExpr{Name: "string"}},
					{Name: "description", Type: spec.TypeExpr{Name: "string", Optional: true}},
				},
				Output: []*spec.Field{
					{Name: "name", Type: spec.TypeExpr{Name: "string"}},
					{Name: "description", Type: spec.TypeExpr{Name: "string", Optional: true}},
				},
				Action: "run",
			},
			Actions: []*spec.ActionDef{{
				Name: "run",
				Params: []*spec.Param{
					{Name: "name", Type: spec.TypeExpr{Name: "string"}},
					{Name: "description", Type: spec.TypeExpr{Name: "string", Optional: true}},
				},
				Body: []spec.GivenStep{
					&spec.ReturnStmt{Value: spec.ObjectLiteral{
						Fields: []*spec.ObjField{
							{Key: "name", Value: spec.FieldRef{Path: "name"}},
							{Key: "description", Value: spec.FieldRef{Path: "description"}},
						},
					}},
				},
			}},
			Scenarios: []*spec.Scenario{{
				Name: "test_scenario",
				Given: &spec.Block{
					Steps: givenSteps,
				},
				Then: &spec.Block{
					Assertions: thenAssertions,
				},
			}},
		}},
	}
}

// TestOptionalFieldDefaultsToNull verifies that omitting an optional field
// from a given block causes it to default to null in the action context.
func TestOptionalFieldDefaultsToNull(t *testing.T) {
	t.Parallel()

	s := buildOptionalFieldSpec(
		// given: only provide "name", omit "description"
		[]spec.GivenStep{
			&spec.Assignment{Path: "name", Value: spec.LiteralString{Value: "test"}},
		},
		// then: description should be null
		[]*spec.Assertion{{
			Expr: spec.BinaryOp{
				Left:  spec.FieldRef{Path: "description"},
				Op:    "==",
				Right: spec.LiteralNull{},
			},
		}},
	)

	adp := &optionalFieldAdapter{}
	r := runner.New(s, map[string]adapter.Adapter{"http": adp}, 42)
	res, err := r.Verify(context.Background())
	if err != nil {
		t.Fatalf("verify error: %v", err)
	}
	if len(res.Failures) > 0 {
		t.Errorf("expected pass, got failure: %s", res.Failures[0].Description)
	}
}

// TestExplicitNullOptionalFieldPassesThrough verifies that explicitly setting
// an optional field to null has the same behavior as omitting it.
func TestExplicitNullOptionalFieldPassesThrough(t *testing.T) {
	t.Parallel()

	s := buildOptionalFieldSpec(
		// given: explicitly set description to null
		[]spec.GivenStep{
			&spec.Assignment{Path: "name", Value: spec.LiteralString{Value: "test"}},
			&spec.Assignment{Path: "description", Value: spec.LiteralNull{}},
		},
		// then: description should be null
		[]*spec.Assertion{{
			Expr: spec.BinaryOp{
				Left:  spec.FieldRef{Path: "description"},
				Op:    "==",
				Right: spec.LiteralNull{},
			},
		}},
	)

	adp := &optionalFieldAdapter{}
	r := runner.New(s, map[string]adapter.Adapter{"http": adp}, 42)
	res, err := r.Verify(context.Background())
	if err != nil {
		t.Fatalf("verify error: %v", err)
	}
	if len(res.Failures) > 0 {
		t.Errorf("expected pass, got failure: %s", res.Failures[0].Description)
	}
}

// TestMissingRequiredFieldReturnsError verifies that omitting a required field
// from a given block produces a clear runtime error.
func TestMissingRequiredFieldReturnsError(t *testing.T) {
	t.Parallel()

	s := buildOptionalFieldSpec(
		// given: omit required "name" field
		[]spec.GivenStep{
			&spec.Assignment{Path: "description", Value: spec.LiteralString{Value: "a description"}},
		},
		[]*spec.Assertion{{
			Expr: spec.BinaryOp{
				Left:  spec.FieldRef{Path: "name"},
				Op:    "==",
				Right: spec.LiteralString{Value: "test"},
			},
		}},
	)

	adp := &optionalFieldAdapter{}
	r := runner.New(s, map[string]adapter.Adapter{"http": adp}, 42)
	_, err := r.Verify(context.Background())
	if err == nil {
		t.Fatal("expected error for missing required field, got nil")
	}
	if !strings.Contains(err.Error(), "missing required input field") {
		t.Errorf("expected error about missing required field, got: %v", err)
	}
	if !strings.Contains(err.Error(), "name") {
		t.Errorf("expected error to mention field name, got: %v", err)
	}
}
