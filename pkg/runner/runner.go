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
	Spec              string        `json:"spec"`
	Scopes            []ScopeResult `json:"scopes"`
	Failures          []Failure     `json:"failures"`
	ScenariosRun      int           `json:"scenarios_run"`
	ScenariosPassed   int           `json:"scenarios_passed"`
	InvariantsChecked int           `json:"invariants_checked"`
	InvariantsPassed  int           `json:"invariants_passed"`
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

// ScopeResult captures per-scope verification results.
type ScopeResult struct {
	Name   string        `json:"name"`
	Checks []CheckResult `json:"checks"`
}

// CheckResult captures the outcome of a single scenario or invariant.
type CheckResult struct {
	Name      string   `json:"name"`
	Kind      string   `json:"kind"`               // "scenario" or "invariant"
	Passed    bool     `json:"passed"`
	InputsRun int      `json:"inputs_run"`          // 1 for given-scenarios, N for when/invariants
	FailedAt  int      `json:"failed_at,omitempty"` // which input number failed (0 if passed)
	Failure   *Failure `json:"failure,omitempty"`
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
	scopeRes := ScopeResult{Name: sr.scope}

	for _, sc := range sr.scenarios() {
		var check CheckResult
		var err error

		if sc.Given != nil {
			check, err = sr.runGivenScenario(sc)
		} else if sc.When != nil {
			check, err = sr.runWhenScenario(sc)
		} else {
			continue
		}

		if err != nil {
			return fmt.Errorf("scope %q scenario %q: %w", sr.scope, sc.Name, err)
		}

		scopeRes.Checks = append(scopeRes.Checks, check)
		res.ScenariosRun++
		if check.Passed {
			res.ScenariosPassed++
		} else if check.Failure != nil {
			res.Failures = append(res.Failures, *check.Failure)
		}
	}

	for _, inv := range sr.invariants() {
		check, err := sr.runInvariant(inv)
		if err != nil {
			return fmt.Errorf("scope %q invariant %q: %w", sr.scope, inv.Name, err)
		}

		scopeRes.Checks = append(scopeRes.Checks, check)
		res.InvariantsChecked++
		if check.Passed {
			res.InvariantsPassed++
		} else if check.Failure != nil {
			res.Failures = append(res.Failures, *check.Failure)
		}
	}

	res.Scopes = append(res.Scopes, scopeRes)
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

	actionName, args, err := sr.buildAction(inputJSON)
	if err != nil {
		return nil, err
	}

	resp, err := sr.runner.adapter.Action(actionName, args)
	if err != nil {
		return nil, fmt.Errorf("executing action %q: %w", actionName, err)
	}
	if !resp.OK {
		return nil, fmt.Errorf("action %q failed: %s", actionName, resp.Error)
	}

	var output map[string]any
	if err := json.Unmarshal(resp.Actual, &output); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}
	return output, nil
}

// buildAction constructs the adapter action call based on scope config.
func (sr *scopeRunner) buildAction(inputJSON json.RawMessage) (string, json.RawMessage, error) {
	if sr.method != "" {
		// HTTP-style: action is the method, args are [path, body]
		args, err := json.Marshal([]json.RawMessage{
			json.RawMessage(fmt.Sprintf("%q", sr.path)),
			inputJSON,
		})
		if err != nil {
			return "", nil, fmt.Errorf("marshaling HTTP args: %w", err)
		}
		return strings.ToLower(sr.method), args, nil
	}

	// Process-style: action is "exec", args are scope config args + input fields as CLI args
	var inputMap map[string]any
	if err := json.Unmarshal(inputJSON, &inputMap); err != nil {
		return "", nil, err
	}

	var execArgs []any
	// Prepend scope config args (e.g., "parse" or "generate --scope transfer --seed")
	if configArgs := resolveConfigString(sr.scopeDef.Config, "args"); configArgs != "" {
		for _, a := range strings.Fields(configArgs) {
			execArgs = append(execArgs, a)
		}
	}
	if sr.scopeDef.Contract != nil {
		for _, field := range sr.scopeDef.Contract.Input {
			if val, ok := inputMap[field.Name]; ok {
				switch v := val.(type) {
				case string:
					execArgs = append(execArgs, v)
				default:
					b, err := json.Marshal(v)
					if err != nil {
						return "", nil, fmt.Errorf("marshaling input field %q: %w", field.Name, err)
					}
					execArgs = append(execArgs, string(b))
				}
			}
		}
	}

	args, err := json.Marshal(execArgs)
	if err != nil {
		return "", nil, fmt.Errorf("marshaling exec args: %w", err)
	}
	return "exec", args, nil
}

func (sr *scopeRunner) runGivenScenario(sc *parser.Scenario) (CheckResult, error) {
	var input map[string]any

	if hasCalls(sc.Given.Steps) {
		// Step-by-step execution: calls go to the adapter in order,
		// assignments accumulate into the input context.
		var err error
		input, err = sr.executeGivenSteps(sc.Given.Steps)
		if err != nil {
			return CheckResult{}, err
		}
	} else {
		// Request/response execution: collect all assignments, dispatch as one action.
		input = stepsToMap(sc.Given.Steps)
		if _, err := sr.executeInput(input); err != nil {
			return CheckResult{}, err
		}
	}

	check := CheckResult{
		Name:      sc.Name,
		Kind:      "scenario",
		InputsRun: 1,
		Passed:    true,
	}

	if sc.Then != nil {
		if f, err := sr.checkThenAssertions(sc.Name, input, sc.Then); err != nil {
			return CheckResult{}, err
		} else if f != nil {
			check.Passed = false
			check.FailedAt = 1
			check.Failure = f
		}
	}

	return check, nil
}

// hasCalls returns true if any given step is a Call (not just assignments).
// When calls are present, steps execute in order rather than being batched.
func hasCalls(steps []parser.GivenStep) bool {
	for _, s := range steps {
		if _, ok := s.(*parser.Call); ok {
			return true
		}
	}
	return false
}

// executeGivenSteps walks given block steps in order: calls go to the adapter,
// assignments accumulate into the input context for assertion evaluation.
func (sr *scopeRunner) executeGivenSteps(steps []parser.GivenStep) (map[string]any, error) {
	input := make(map[string]any)
	for _, step := range steps {
		switch s := step.(type) {
		case *parser.Assignment:
			setPath(input, s.Path, exprToValue(s.Value))
		case *parser.Call:
			args, err := sr.marshalCallArgs(s)
			if err != nil {
				return nil, fmt.Errorf("marshaling args for %s.%s: %w", s.Namespace, s.Method, err)
			}
			resp, err := sr.runner.adapter.Action(s.Method, args)
			if err != nil {
				return nil, fmt.Errorf("executing %s.%s: %w", s.Namespace, s.Method, err)
			}
			if !resp.OK {
				return nil, fmt.Errorf("action %s.%s failed: %s", s.Namespace, s.Method, resp.Error)
			}
		}
	}
	return input, nil
}

// marshalCallArgs converts Call expression arguments to JSON for the adapter.
// FieldRef args are resolved as locator names from the spec's locators map.
func (sr *scopeRunner) marshalCallArgs(call *parser.Call) (json.RawMessage, error) {
	var resolved []any
	for _, arg := range call.Args {
		switch a := arg.(type) {
		case parser.FieldRef:
			// Resolve locator name to CSS selector
			if selector, ok := sr.runner.spec.Locators[a.Path]; ok {
				resolved = append(resolved, selector)
			} else {
				// Not a locator — pass the name as-is (could be a variable)
				resolved = append(resolved, a.Path)
			}
		default:
			resolved = append(resolved, exprToValue(arg))
		}
	}
	return json.Marshal(resolved)
}

func (sr *scopeRunner) runWhenScenario(sc *parser.Scenario) (CheckResult, error) {
	predicate := buildPredicate(sc.When.Predicates)

	check := CheckResult{
		Name:   sc.Name,
		Kind:   "scenario",
		Passed: true,
	}

	for i := range sr.runner.n {
		input, err := sr.generator.GenerateMatching(predicate)
		if err != nil {
			return CheckResult{}, err
		}

		if _, err := sr.executeInput(input); err != nil {
			return CheckResult{}, err
		}

		check.InputsRun = i + 1

		if sc.Then == nil {
			continue
		}

		if f, err := sr.checkThenAssertions(sc.Name, input, sc.Then); err != nil {
			return CheckResult{}, err
		} else if f != nil {
			f = sr.shrinkFailure(f, sc.Then)
			check.Passed = false
			check.FailedAt = i + 1
			check.Failure = f
			return check, nil
		}
	}

	return check, nil
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
		val, ok := generator.Eval(a.Expected, input)
		if !ok {
			return nil, fmt.Errorf("evaluating expected expression for %q", a.Target)
		}
		expected, err := json.Marshal(val)
		if err != nil {
			return nil, fmt.Errorf("marshaling expected for %q: %w", a.Target, err)
		}

		property := a.Target
		locator := ""
		if a.Plugin != "" {
			selector, ok := sr.runner.spec.Locators[a.Target]
			if !ok {
				return nil, fmt.Errorf("locator %q not defined in locators block", a.Target)
			}
			locator = selector
			property = a.Property
		}
		resp, err := sr.runner.adapter.Assert(property, locator, expected)
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

func (sr *scopeRunner) runInvariant(inv *parser.Invariant) (CheckResult, error) {
	check := CheckResult{
		Name:   inv.Name,
		Kind:   "invariant",
		Passed: true,
	}

	for i := range sr.runner.n {
		input, err := sr.generator.GenerateInput()
		if err != nil {
			return CheckResult{}, err
		}

		output, err := sr.executeInput(input)
		if err != nil {
			return CheckResult{}, err
		}

		check.InputsRun = i + 1

		ctx := buildInvariantContext(input, output)

		if !evalGuard(inv.When, ctx) {
			continue
		}

		if f := checkInvariantAssertions(inv.Name, sr.scope, input, inv.Assertions, ctx); f != nil {
			f = sr.shrinkInvariantFailure(f, inv)
			check.Passed = false
			check.FailedAt = i + 1
			check.Failure = f
			return check, nil
		}
	}

	return check, nil
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

// stepsToMap extracts assignments from given steps into a nested map.
func stepsToMap(steps []parser.GivenStep) map[string]any {
	result := make(map[string]any)
	for _, s := range steps {
		if a, ok := s.(*parser.Assignment); ok {
			setPath(result, a.Path, exprToValue(a.Value))
		}
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
	case parser.LiteralFloat:
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
