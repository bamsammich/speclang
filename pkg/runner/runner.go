package runner

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/bamsammich/speclang/pkg/adapter"
	"github.com/bamsammich/speclang/pkg/generator"
	"github.com/bamsammich/speclang/pkg/parser"
)

// Result captures the outcome of a verification run.
type Result struct {
	Spec              string    `json:"spec"`
	Failures          []Failure `json:"failures"`
	ScenariosRun      int       `json:"scenarios_run"`
	ScenariosPassed   int       `json:"scenarios_passed"`
	InvariantsChecked int       `json:"invariants_checked"`
	InvariantsPassed  int       `json:"invariants_passed"`
}

type Failure struct {
	Name        string `json:"name"`
	Scope       string `json:"scope"`
	Input       any    `json:"input,omitempty"`
	Expected    any    `json:"expected,omitempty"`
	Actual      any    `json:"actual,omitempty"`
	Description string `json:"description"`
	Shrunk      bool   `json:"shrunk"`
}

// Runner orchestrates spec verification.
type Runner struct {
	spec    *parser.Spec
	adapter adapter.Adapter
	seed    uint64
	n       int
}

// New creates a runner for the given spec.
func New(spec *parser.Spec, adp adapter.Adapter, seed uint64) *Runner {
	return &Runner{
		spec:    spec,
		adapter: adp,
		seed:    seed,
		n:       100,
	}
}

// SetN configures how many inputs to generate per when-scenario and invariant.
func (r *Runner) SetN(n int) {
	r.n = n
}

// scopeRunner holds per-scope state for running scenarios and invariants.
type scopeRunner struct {
	runner    *Runner
	generator *generator.Generator
	scopeDef  *parser.Scope
	scope     string
	path      string
	method    string
}

func (sr *scopeRunner) scenarios() []*parser.Scenario {
	return sr.scopeDef.Scenarios
}

func (sr *scopeRunner) invariants() []*parser.Invariant {
	return sr.scopeDef.Invariants
}

// Verify runs all scopes' scenarios and invariants, returning results.
func (r *Runner) Verify() (*Result, error) {
	res := &Result{Spec: r.spec.Name}

	for _, scope := range r.spec.Scopes {
		sr := r.newScopeRunner(scope)
		if err := sr.run(res); err != nil {
			return nil, err
		}
	}

	return res, nil
}

func (r *Runner) newScopeRunner(scope *parser.Scope) *scopeRunner {
	gen := generator.New(scope.Contract, r.spec.Models, r.seed)
	method := strings.ToUpper(resolveConfigString(scope.Config, "method"))
	if method == "" {
		method = "POST"
	}
	return &scopeRunner{
		runner:    r,
		generator: gen,
		scopeDef:  scope,
		scope:     scope.Name,
		path:      resolveConfigString(scope.Config, "path"),
		method:    method,
	}
}

func (sr *scopeRunner) run(res *Result) error {
	for _, sc := range sr.scenarios() {
		if sc.Given != nil {
			res.ScenariosRun++
			if err := sr.runGivenScenario(sc, res); err != nil {
				return fmt.Errorf(
					"scope %q scenario %q: %w",
					sr.scope, sc.Name, err,
				)
			}
		} else if sc.When != nil {
			res.ScenariosRun++
			if err := sr.runWhenScenario(sc, res); err != nil {
				return fmt.Errorf(
					"scope %q scenario %q: %w",
					sr.scope, sc.Name, err,
				)
			}
		}
	}

	for _, inv := range sr.invariants() {
		res.InvariantsChecked++
		if err := sr.runInvariant(inv, res); err != nil {
			return fmt.Errorf(
				"scope %q invariant %q: %w",
				sr.scope, inv.Name, err,
			)
		}
	}

	return nil
}

// resolveConfigString extracts a string value from a scope's config map.
func resolveConfigString(config map[string]parser.Expr, key string) string {
	if config == nil {
		return ""
	}
	expr, ok := config[key]
	if !ok {
		return ""
	}
	switch e := expr.(type) {
	case parser.LiteralString:
		return e.Value
	default:
		return fmt.Sprintf("%v", e)
	}
}

// executeInput sends an input map to the adapter and returns the parsed response.
func (sr *scopeRunner) executeInput(input map[string]any) (map[string]any, error) {
	inputJSON, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("marshaling input: %w", err)
	}

	args, err := json.Marshal([]json.RawMessage{
		json.RawMessage(fmt.Sprintf("%q", sr.path)),
		json.RawMessage(inputJSON),
	})
	if err != nil {
		return nil, fmt.Errorf("marshaling args: %w", err)
	}

	resp, err := sr.runner.adapter.Action(strings.ToLower(sr.method), args)
	if err != nil {
		return nil, fmt.Errorf("executing %s %s: %w", sr.method, sr.path, err)
	}
	if !resp.OK {
		return nil, fmt.Errorf("%s %s failed: %s", sr.method, sr.path, resp.Error)
	}

	var output map[string]any
	if err := json.Unmarshal(resp.Actual, &output); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}
	return output, nil
}

func (sr *scopeRunner) runGivenScenario(sc *parser.Scenario, res *Result) error {
	input := assignmentsToMap(sc.Given.Assignments)

	if _, err := sr.executeInput(input); err != nil {
		return err
	}

	if sc.Then != nil {
		if f, err := sr.checkThenAssertions(sc.Name, input, sc.Then); err != nil {
			return err
		} else if f != nil {
			res.Failures = append(res.Failures, *f)
			return nil
		}
	}

	res.ScenariosPassed++
	return nil
}

func (sr *scopeRunner) runWhenScenario(sc *parser.Scenario, res *Result) error {
	predicate := buildPredicate(sc.When.Predicates)

	for range sr.runner.n {
		input, err := sr.generator.GenerateMatching(predicate)
		if err != nil {
			return err
		}

		if _, err := sr.executeInput(input); err != nil {
			return err
		}

		if sc.Then == nil {
			continue
		}

		if f, err := sr.checkThenAssertions(sc.Name, input, sc.Then); err != nil {
			return err
		} else if f != nil {
			f = sr.shrinkFailure(f, sc.Then)
			res.Failures = append(res.Failures, *f)
			return nil
		}
	}

	res.ScenariosPassed++
	return nil
}

// buildPredicate creates a filter function from when-block predicates.
func buildPredicate(predicates []parser.Expr) func(map[string]any) bool {
	return func(input map[string]any) bool {
		for _, pred := range predicates {
			val, ok := generator.Eval(pred, input)
			if !ok {
				return false
			}
			b, isBool := val.(bool)
			if !isBool || !b {
				return false
			}
		}
		return true
	}
}

// checkThenAssertions checks all then-block assertions via the adapter.
// Returns a Failure on the first failing assertion, or nil if all pass.
func (sr *scopeRunner) checkThenAssertions(
	name string,
	input map[string]any,
	then *parser.Block,
) (*Failure, error) {
	for _, a := range then.Assertions {
		expected, err := exprToJSON(a.Expected)
		if err != nil {
			return nil, fmt.Errorf("marshaling expected for %q: %w", a.Target, err)
		}

		resp, err := sr.runner.adapter.Assert(a.Target, "", expected)
		if err != nil {
			return nil, fmt.Errorf("asserting %q: %w", a.Target, err)
		}
		if !resp.OK {
			return &Failure{
				Name:        name,
				Scope:       sr.scope,
				Input:       input,
				Expected:    string(expected),
				Actual:      string(resp.Actual),
				Description: fmt.Sprintf("assertion %q failed: %s", a.Target, resp.Error),
			}, nil
		}
	}
	return nil, nil
}

func (sr *scopeRunner) runInvariant(inv *parser.Invariant, res *Result) error {
	for range sr.runner.n {
		input, err := sr.generator.GenerateInput()
		if err != nil {
			return err
		}

		output, err := sr.executeInput(input)
		if err != nil {
			return err
		}

		ctx := buildInvariantContext(input, output)

		if !evalGuard(inv.When, ctx) {
			continue
		}

		if f := checkInvariantAssertions(inv.Name, sr.scope, input, inv.Assertions, ctx); f != nil {
			f = sr.shrinkInvariantFailure(f, inv)
			res.Failures = append(res.Failures, *f)
			return nil
		}
	}

	res.InvariantsPassed++
	return nil
}

// shrinkFailure attempts to shrink a when-scenario failure to a minimal counterexample.
func (sr *scopeRunner) shrinkFailure(f *Failure, then *parser.Block) *Failure {
	input, ok := f.Input.(map[string]any)
	if !ok {
		return f
	}

	models := make(map[string]*parser.Model, len(sr.runner.spec.Models))
	for _, m := range sr.runner.spec.Models {
		models[m.Name] = m
	}

	fields := sr.generator.ContractInput()
	shrunk := generator.Shrink(
		input, fields, models,
		func(candidate map[string]any) bool {
			if _, err := sr.executeInput(candidate); err != nil {
				return false
			}
			fail, err := sr.checkThenAssertions(f.Name, candidate, then)
			return err == nil && fail != nil
		},
	)

	if fmt.Sprintf("%v", shrunk) != fmt.Sprintf("%v", input) {
		f.Input = shrunk
		f.Shrunk = true
	}
	return f
}

// shrinkInvariantFailure attempts to shrink an invariant failure to a minimal counterexample.
func (sr *scopeRunner) shrinkInvariantFailure(f *Failure, inv *parser.Invariant) *Failure {
	input, ok := f.Input.(map[string]any)
	if !ok {
		return f
	}

	models := make(map[string]*parser.Model, len(sr.runner.spec.Models))
	for _, m := range sr.runner.spec.Models {
		models[m.Name] = m
	}

	fields := sr.generator.ContractInput()
	shrunk := generator.Shrink(
		input, fields, models,
		func(candidate map[string]any) bool {
			output, err := sr.executeInput(candidate)
			if err != nil {
				return false
			}
			ctx := buildInvariantContext(candidate, output)
			if !evalGuard(inv.When, ctx) {
				return false
			}
			return checkInvariantAssertions(
				inv.Name, sr.scope, candidate, inv.Assertions, ctx,
			) != nil
		},
	)

	if fmt.Sprintf("%v", shrunk) != fmt.Sprintf("%v", input) {
		f.Input = shrunk
		f.Shrunk = true
	}
	return f
}

// evalGuard evaluates an optional when-guard. Returns true if guard is nil or evaluates to true.
func evalGuard(guard parser.Expr, ctx map[string]any) bool {
	if guard == nil {
		return true
	}
	val, ok := generator.Eval(guard, ctx)
	if !ok {
		return false
	}
	b, isBool := val.(bool)
	return isBool && b
}

// checkInvariantAssertions evaluates invariant assertion expressions.
// Returns a Failure on the first failing assertion, or nil if all pass.
func checkInvariantAssertions(
	name string,
	scope string,
	input map[string]any,
	assertions []*parser.Assertion,
	ctx map[string]any,
) *Failure {
	for _, a := range assertions {
		val, ok := generator.Eval(a.Expr, ctx)
		if !ok {
			return &Failure{
				Name:        name,
				Scope:       scope,
				Input:       input,
				Description: "invariant expression could not be evaluated",
			}
		}
		b, isBool := val.(bool)
		if !isBool || !b {
			return &Failure{
				Name:        name,
				Scope:       scope,
				Input:       input,
				Expected:    true,
				Actual:      val,
				Description: fmt.Sprintf("invariant assertion evaluated to %v", val),
			}
		}
	}
	return nil
}

// buildInvariantContext merges input and output into a single eval context.
// Result: {"input": inputMap, "output": outputMap, <top-level output fields>}
func buildInvariantContext(input, output map[string]any) map[string]any {
	ctx := make(map[string]any, len(output)+2)
	for k, v := range output {
		ctx[k] = v
	}
	ctx["input"] = input
	ctx["output"] = output
	return ctx
}

// assignmentsToMap converts a list of assignments to a nested map.
func assignmentsToMap(assignments []*parser.Assignment) map[string]any {
	result := make(map[string]any)
	for _, a := range assignments {
		setPath(result, a.Path, exprToValue(a.Value))
	}
	return result
}

// setPath sets a dotted path in a nested map.
func setPath(m map[string]any, path string, value any) {
	parts := strings.Split(path, ".")
	current := m
	for _, part := range parts[:len(parts)-1] {
		next, ok := current[part].(map[string]any)
		if !ok {
			next = make(map[string]any)
			current[part] = next
		}
		current = next
	}
	current[parts[len(parts)-1]] = value
}

// exprToValue converts an AST expression to a Go value.
func exprToValue(expr parser.Expr) any {
	switch e := expr.(type) {
	case parser.LiteralInt:
		return e.Value
	case parser.LiteralString:
		return e.Value
	case parser.LiteralBool:
		return e.Value
	case parser.LiteralNull:
		return nil
	case parser.ObjectLiteral:
		m := make(map[string]any, len(e.Fields))
		for _, f := range e.Fields {
			m[f.Key] = exprToValue(f.Value)
		}
		return m
	default:
		return nil
	}
}

// exprToJSON marshals an AST expression to JSON.
func exprToJSON(expr parser.Expr) (json.RawMessage, error) {
	v := exprToValue(expr)
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(b), nil
}
