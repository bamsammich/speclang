package runner

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/bamsammich/speclang/v2/pkg/adapter"
	"github.com/bamsammich/speclang/v2/pkg/generator"
	"github.com/bamsammich/speclang/v2/pkg/parser"
	"github.com/bamsammich/speclang/v2/pkg/spec"
)

// errorPseudoField is the name of the pseudo-field used to assert against action errors.
const errorPseudoField = "error"

// Result type aliases — all types are defined in pkg/spec and re-exported here
// for backward compatibility.

type Result = spec.Result
type Failure = spec.Failure
type ScopeResult = spec.ScopeResult
type CheckResult = spec.CheckResult

// Runner orchestrates spec verification.
type Runner struct {
	spec     *parser.Spec
	adapters map[string]adapter.Adapter
	seed     uint64
	n        int
}

// New creates a runner for the given spec.
func New(spec *parser.Spec, adapters map[string]adapter.Adapter, seed uint64) *Runner {
	return &Runner{
		spec:     spec,
		adapters: adapters,
		seed:     seed,
		n:        100,
	}
}

// SetN configures how many inputs to generate per when-scenario and invariant.
func (r *Runner) SetN(n int) {
	r.n = n
}

// scopeRunner holds per-scope state for running scenarios and invariants.
type scopeRunner struct {
	runner          *Runner
	adapter         adapter.Adapter // resolved from scope.Use
	generator       *generator.Generator
	scopeDef        *parser.Scope
	scope           string
	path            string
	method          string
	lastActionError string // captured when an action returns {ok: false}
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
		sr, err := r.newScopeRunner(scope)
		if err != nil {
			return nil, err
		}
		if err := sr.run(res); err != nil {
			return nil, err
		}
	}

	return res, nil
}

func (r *Runner) newScopeRunner(scope *parser.Scope) (*scopeRunner, error) {
	adp, ok := r.adapters[scope.Use]
	if !ok {
		return nil, fmt.Errorf("no adapter for plugin %q in scope %q", scope.Use, scope.Name)
	}
	gen := generator.New(scope.Contract, r.spec.Models, r.seed)
	method := strings.ToUpper(resolveConfigString(scope.Config, "method"))
	return &scopeRunner{
		runner:    r,
		adapter:   adp,
		generator: gen,
		scopeDef:  scope,
		scope:     scope.Name,
		path:      resolveConfigString(scope.Config, "path"),
		method:    method,
	}, nil
}

func (sr *scopeRunner) run(res *Result) error {
	scopeRes := ScopeResult{Name: sr.scope}

	for _, sc := range sr.scenarios() {
		var check CheckResult
		var err error

		switch {
		case sc.Given != nil:
			check, err = sr.runGivenScenario(sc)
		case sc.When != nil:
			check, err = sr.runWhenScenario(sc)
		default:
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
// When expectError is true, an action returning {ok: false} is captured as an
// assertable error instead of failing the test. The captured error string is
// stored in sr.lastActionError.
func (sr *scopeRunner) executeInput(input map[string]any) (map[string]any, error) {
	sr.lastActionError = ""

	inputJSON, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("marshaling input: %w", err)
	}

	actionName, args, err := sr.buildAction(inputJSON)
	if err != nil {
		return nil, err
	}

	resp, err := sr.adapter.Action(actionName, args)
	if err != nil {
		return nil, fmt.Errorf("executing action %q: %w", actionName, err)
	}
	if !resp.OK {
		sr.lastActionError = resp.Error
		return nil, nil
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
		return sr.buildHTTPAction(inputJSON)
	}
	return sr.buildExecAction(inputJSON)
}

func (sr *scopeRunner) buildHTTPAction(inputJSON json.RawMessage) (string, json.RawMessage, error) {
	args, err := json.Marshal([]json.RawMessage{
		json.RawMessage(fmt.Sprintf("%q", sr.path)),
		inputJSON,
	})
	if err != nil {
		return "", nil, fmt.Errorf("marshaling HTTP args: %w", err)
	}
	return strings.ToLower(sr.method), args, nil
}

func (sr *scopeRunner) buildExecAction(inputJSON json.RawMessage) (string, json.RawMessage, error) {
	var inputMap map[string]any
	if err := json.Unmarshal(inputJSON, &inputMap); err != nil {
		return "", nil, err
	}

	execArgs := sr.collectExecArgs(inputMap)
	args, err := json.Marshal(execArgs)
	if err != nil {
		return "", nil, fmt.Errorf("marshaling exec args: %w", err)
	}
	return "exec", args, nil
}

func (sr *scopeRunner) collectExecArgs(inputMap map[string]any) []any {
	var execArgs []any
	if configArgs := resolveConfigString(sr.scopeDef.Config, "args"); configArgs != "" {
		for _, a := range strings.Fields(configArgs) {
			execArgs = append(execArgs, a)
		}
	}
	if sr.scopeDef.Contract != nil {
		for _, field := range sr.scopeDef.Contract.Input {
			if val, ok := inputMap[field.Name]; ok {
				execArgs = append(execArgs, fieldToString(val))
			}
		}
	}
	return execArgs
}

func fieldToString(val any) string {
	if s, ok := val.(string); ok {
		return s
	}
	b, err := json.Marshal(val)
	if err != nil {
		return fmt.Sprintf("%v", val)
	}
	return string(b)
}

func (sr *scopeRunner) runGivenScenario(sc *parser.Scenario) (CheckResult, error) {
	input, err := sr.executeGivenInput(sc)
	if err != nil {
		return CheckResult{}, err
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

// executeGivenInput executes the given block, either step-by-step (when calls
// are present) or as a batched request/response.
func (sr *scopeRunner) executeGivenInput(sc *parser.Scenario) (map[string]any, error) {
	expectsError := hasErrorPseudoAssertion(sc.Then, sr.scopeDef.Contract)

	if hasCalls(sc.Given.Steps) {
		input, err := sr.executeGivenSteps(sc.Given.Steps)
		if err != nil {
			if !expectsError || sr.lastActionError == "" {
				return nil, err
			}
		}
		return input, nil
	}

	input := stepsToMap(sc.Given.Steps)
	if _, err := sr.executeInput(input); err != nil {
		return nil, err
	}
	if sr.lastActionError != "" && !expectsError {
		return nil, fmt.Errorf("action failed: %s", sr.lastActionError)
	}
	return input, nil
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
// When an action returns {ok: false}, the error is captured in sr.lastActionError
// and the remaining steps are skipped. The caller decides whether this is an
// expected error (via hasErrorPseudoAssertion) or a test failure.
func (sr *scopeRunner) executeGivenSteps(steps []parser.GivenStep) (map[string]any, error) {
	sr.lastActionError = ""
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
			resp, err := sr.adapter.Action(s.Method, args)
			if err != nil {
				return nil, fmt.Errorf("executing %s.%s: %w", s.Namespace, s.Method, err)
			}
			if !resp.OK {
				sr.lastActionError = resp.Error
				return input, fmt.Errorf(
					"action %s.%s failed: %s",
					s.Namespace,
					s.Method,
					resp.Error,
				)
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
	needsPageIsolation := sc.Then != nil && hasPluginAssertions(sc.Then.Assertions)
	expectsError := hasErrorPseudoAssertion(sc.Then, sr.scopeDef.Contract)

	check := CheckResult{
		Name:   sc.Name,
		Kind:   "scenario",
		Passed: true,
	}

	for i := range sr.runner.n {
		f, err := sr.runWhenIteration(sc, predicate, needsPageIsolation, expectsError, i)
		if err != nil {
			return CheckResult{}, err
		}
		check.InputsRun = i + 1
		if f != nil {
			check.Passed = false
			check.FailedAt = i + 1
			check.Failure = f
			return check, nil
		}
	}

	return check, nil
}

// runWhenIteration runs a single iteration of a when-scenario. It returns a
// failure (if assertions fail) or an error (if execution fails). A nil failure
// with nil error means the iteration passed.
func (sr *scopeRunner) runWhenIteration(
	sc *parser.Scenario,
	predicate func(map[string]any) bool,
	needsPageIsolation bool,
	expectsError bool,
	i int,
) (*Failure, error) {
	input, err := sr.generator.GenerateMatching(predicate)
	if err != nil {
		return nil, err
	}

	if needsPageIsolation {
		if err := sr.newPageWithNavigation(); err != nil {
			return nil, fmt.Errorf("iteration %d: %w", i+1, err)
		}
		defer func() {
			//nolint:errcheck // best-effort page cleanup, test result takes priority
			sr.closePage()
		}()
	}

	if _, err := sr.executeInput(input); err != nil {
		return nil, err
	}

	if sr.lastActionError != "" && !expectsError {
		return nil, fmt.Errorf("action failed: %s", sr.lastActionError)
	}

	if sc.Then == nil {
		return nil, nil
	}

	f, err := sr.checkThenAssertions(sc.Name, input, sc.Then)
	if err != nil {
		return nil, err
	}
	if f != nil {
		return sr.shrinkFailure(f, sc.Then), nil
	}
	return nil, nil
}

// hasPluginAssertions returns true if any assertion uses @plugin.property syntax.
func hasPluginAssertions(assertions []*parser.Assertion) bool {
	for _, a := range assertions {
		if a.Plugin != "" {
			return true
		}
	}
	return false
}

// newPageWithNavigation creates a fresh page and navigates to the scope's configured URL.
func (sr *scopeRunner) newPageWithNavigation() error {
	resp, err := sr.adapter.Action("new_page", nil)
	if err != nil {
		return fmt.Errorf("creating new page: %w", err)
	}
	if !resp.OK {
		return fmt.Errorf("creating new page: %s", resp.Error)
	}

	url := resolveConfigString(sr.scopeDef.Config, "url")
	if url != "" {
		args, err := json.Marshal([]string{url})
		if err != nil {
			return fmt.Errorf("marshaling goto args: %w", err)
		}
		resp, err := sr.adapter.Action("goto", args)
		if err != nil {
			return fmt.Errorf("navigating to %q: %w", url, err)
		}
		if !resp.OK {
			return fmt.Errorf("navigating to %q: %s", url, resp.Error)
		}
	}
	return nil
}

// closePage closes the current page via the adapter.
func (sr *scopeRunner) closePage() error {
	resp, err := sr.adapter.Action("close_page", nil)
	if err != nil {
		return err
	}
	if !resp.OK {
		return fmt.Errorf("closing page: %s", resp.Error)
	}
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

// hasErrorPseudoAssertion returns true if the then block asserts on the "error"
// pseudo-field (i.e., "error" is NOT a contract output field).
func hasErrorPseudoAssertion(then *parser.Block, contract *parser.Contract) bool {
	if then == nil {
		return false
	}
	// If "error" is declared in the contract output, it's a real field, not a pseudo-field.
	if contract != nil {
		for _, f := range contract.Output {
			if f.Name == errorPseudoField {
				return false
			}
		}
	}
	for _, a := range then.Assertions {
		if a.Target == errorPseudoField && a.Plugin == "" {
			return true
		}
	}
	return false
}

// checkThenAssertions checks all then-block assertions via the adapter.
// Returns a Failure on the first failing assertion, or nil if all pass.
// The "error" pseudo-field is handled specially: it asserts against the last
// action error captured from an adapter {ok: false} response.
func (sr *scopeRunner) checkThenAssertions(
	name string,
	input map[string]any,
	then *parser.Block,
) (*Failure, error) {
	for _, a := range then.Assertions {
		f, err := sr.checkSingleAssertion(name, input, a)
		if err != nil {
			return nil, err
		}
		if f != nil {
			return f, nil
		}
	}
	return nil, nil
}

func (sr *scopeRunner) checkSingleAssertion(
	name string,
	input map[string]any,
	a *parser.Assertion,
) (*Failure, error) {
	val, ok := generator.Eval(a.Expected, input)
	if !ok {
		return nil, fmt.Errorf("evaluating expected expression for %q", a.Target)
	}
	expected, err := json.Marshal(val)
	if err != nil {
		return nil, fmt.Errorf("marshaling expected for %q: %w", a.Target, err)
	}

	// Handle the "error" pseudo-field: assert against the last action error.
	if a.Target == errorPseudoField && a.Plugin == "" && !sr.hasOutputField(errorPseudoField) {
		if f := sr.checkErrorAssertion(name, input, val, expected); f != nil {
			return f, nil
		}
		return nil, nil
	}

	property, locator, err := sr.resolveAssertionTarget(a)
	if err != nil {
		return nil, err
	}
	resp, err := sr.adapter.Assert(property, locator, expected)
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
	return nil, nil
}

func (sr *scopeRunner) resolveAssertionTarget(a *parser.Assertion) (string, string, error) {
	if a.Plugin == "" {
		return a.Target, "", nil
	}
	selector, ok := sr.runner.spec.Locators[a.Target]
	if !ok {
		return "", "", fmt.Errorf("locator %q not defined in locators block", a.Target)
	}
	return a.Property, selector, nil
}

// hasOutputField returns true if the scope's contract declares a field with the given name.
func (sr *scopeRunner) hasOutputField(name string) bool {
	if sr.scopeDef.Contract == nil {
		return false
	}
	for _, f := range sr.scopeDef.Contract.Output {
		if f.Name == name {
			return true
		}
	}
	return false
}

// checkErrorAssertion checks the "error" pseudo-field against the last captured action error.
// If the expected value is null/nil, the assertion passes when no error occurred.
// If the expected value is a string, the assertion passes when the error matches exactly.
func (sr *scopeRunner) checkErrorAssertion(
	name string,
	input map[string]any,
	expectedVal any,
	expectedJSON json.RawMessage,
) *Failure {
	if expectedVal == nil {
		// Asserting error: null — expect no error.
		if sr.lastActionError == "" {
			return nil // pass
		}
		return &Failure{
			Name:     name,
			Scope:    sr.scope,
			Input:    input,
			Expected: "null",
			Actual:   fmt.Sprintf("%q", sr.lastActionError),
			Description: fmt.Sprintf(
				"assertion \"error\" failed: expected no error, got %q",
				sr.lastActionError,
			),
		}
	}

	// Asserting error: "some string" — expect that specific error.
	//nolint:errcheck // json.Marshal on a string value cannot fail
	actualJSON, _ := json.Marshal(sr.lastActionError)

	if sr.lastActionError == "" {
		return &Failure{
			Name:     name,
			Scope:    sr.scope,
			Input:    input,
			Expected: string(expectedJSON),
			Actual:   `""`,
			Description: fmt.Sprintf(
				"assertion \"error\" failed: expected error %s, but no error occurred",
				string(expectedJSON),
			),
		}
	}

	if string(actualJSON) == string(expectedJSON) {
		return nil // pass
	}

	return &Failure{
		Name:     name,
		Scope:    sr.scope,
		Input:    input,
		Expected: string(expectedJSON),
		Actual:   string(actualJSON),
		Description: fmt.Sprintf(
			"assertion \"error\" failed: expected %s, got %s",
			string(expectedJSON),
			string(actualJSON),
		),
	}
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
		if sr.lastActionError != "" {
			return CheckResult{}, fmt.Errorf("action failed: %s", sr.lastActionError)
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
	expectsError := hasErrorPseudoAssertion(then, sr.scopeDef.Contract)
	shrunk := generator.Shrink(
		input, fields, models,
		func(candidate map[string]any) bool {
			if _, err := sr.executeInput(candidate); err != nil {
				return false
			}
			if sr.lastActionError != "" && !expectsError {
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
			if err != nil || sr.lastActionError != "" {
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
	case parser.ArrayLiteral:
		result := make([]any, len(e.Elements))
		for i, elem := range e.Elements {
			result[i] = exprToValue(elem)
		}
		return result
	default:
		return nil
	}
}
