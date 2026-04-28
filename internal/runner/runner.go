package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"

	"github.com/bamsammich/speclang/v4/internal/adapter"
	"github.com/bamsammich/speclang/v4/internal/generator"
	"github.com/bamsammich/speclang/v4/internal/parser"
	"github.com/bamsammich/speclang/v4/pkg/spec"
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

// scopeRunner holds per-contract state for running scenarios and invariants.
type scopeRunner struct {
	ctx             context.Context
	runner          *Runner
	generator       *generator.Generator
	contractDef     *parser.Contract
	scopeDef        *parser.Scope // nil for top-level contracts
	scope           string
	lastActionError string         // captured when an action returns {ok: false}
	lastOutput      map[string]any // captured output from the last action execution
}

func (sr *scopeRunner) scenarios() []*parser.Scenario {
	return sr.contractDef.Scenarios
}

func (sr *scopeRunner) invariants() []*parser.Invariant {
	return sr.contractDef.Invariants
}

// Verify runs all contracts' scenarios and invariants, returning results.
func (r *Runner) Verify(ctx context.Context) (*Result, error) {
	res := &Result{}

	// Top-level contracts
	for _, contract := range r.spec.Contracts {
		sr := r.newContractRunner(ctx, contract, nil)
		if err := sr.run(res); err != nil {
			return nil, err
		}
	}

	// Scoped contracts (inherit scope's before/after hooks)
	for _, scope := range r.spec.Scopes {
		for _, contract := range scope.Contracts {
			sr := r.newContractRunner(ctx, contract, scope)
			if err := sr.run(res); err != nil {
				return nil, err
			}
		}
	}

	return res, nil
}

func (r *Runner) newContractRunner(ctx context.Context, contract *parser.Contract, scope *parser.Scope) *scopeRunner {
	gen := generator.New(contract, r.spec.Models, r.seed)
	if len(r.spec.Enums) > 0 {
		gen.SetEnums(r.spec.Enums)
	}
	if len(r.spec.Config) > 0 {
		gen.SetConfig(r.spec.Config)
	}

	scopeName := contract.Name
	if scope != nil {
		scopeName = scope.Name
	}

	return &scopeRunner{
		ctx:         ctx,
		runner:      r,
		generator:   gen,
		contractDef: contract,
		scopeDef:    scope,
		scope:       scopeName,
	}
}

// resolveAdapter looks up an adapter by name from the runner's adapter map.
func (sr *scopeRunner) resolveAdapter(name string) (adapter.Adapter, error) {
	adp, ok := sr.runner.adapters[name]
	if !ok {
		return nil, fmt.Errorf("no adapter for plugin %q in scope %q", name, sr.scope)
	}
	return adp, nil
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


// executeInput executes the contract's action block with the given input and returns the output.
// The captured error string is stored in sr.lastActionError.
func (sr *scopeRunner) executeInput(input map[string]any) (map[string]any, error) {
	sr.lastActionError = ""
	sr.lastOutput = nil

	output, err := sr.executeContractAction(input)
	sr.lastOutput = output
	return output, err
}

// executeContractAction executes the contract's action block body.
// The input map fields are bound as the execution context.
func (sr *scopeRunner) executeContractAction(input map[string]any) (map[string]any, error) {
	if sr.contractDef == nil || sr.contractDef.Action == nil {
		return nil, nil
	}

	// Build parameter context from input map.
	ctx := make(map[string]any, len(input))
	for k, v := range input {
		ctx[k] = v
	}

	// Fill missing optional fields with nil; error on missing required fields.
	for _, field := range sr.contractDef.Fields {
		if _, exists := ctx[field.Name]; !exists {
			if field.Type.Optional {
				ctx[field.Name] = nil
			} else {
				return nil, fmt.Errorf("contract %q: missing required field %q", sr.contractDef.Name, field.Name)
			}
		}
	}

	return sr.executeBlockBody(sr.contractDef.Name, sr.contractDef.Action.Body, ctx)
}

// findAction looks up an action by name at spec level.
func (sr *scopeRunner) findAction(name string) *parser.ActionDef {
	for _, a := range sr.runner.spec.Actions {
		if a.Name == name {
			return a
		}
	}
	return nil
}

// executeLocalActionCall resolves arguments, builds a child context, and executes
// a local (spec-level or scope-level) action, returning its result.
func (sr *scopeRunner) executeLocalActionCall(action *parser.ActionDef, args []parser.Expr, parentCtx map[string]any) (any, error) {
	childCtx := make(map[string]any)
	for i, param := range action.Params {
		if i < len(args) {
			val, _ := generator.Eval(args[i], parentCtx)
			childCtx[param.Name] = val
		}
	}
	result, err := sr.executeActionBody(action, childCtx)
	if err != nil {
		return nil, err
	}
	// Return the result as a generic value (map or nil).
	if result == nil {
		return nil, nil
	}
	return any(result), nil
}

// executeActionBody executes a named action's body steps and returns the output.
func (sr *scopeRunner) executeActionBody(action *parser.ActionDef, ctx map[string]any) (map[string]any, error) {
	return sr.executeBlockBody(action.Name, action.Body, ctx)
}

// executeBlockBody executes a slice of GivenStep and returns the output.
// Handles LetBinding, AdapterCall, ReturnStmt, and Call steps.
func (sr *scopeRunner) executeBlockBody(name string, steps []parser.GivenStep, ctx map[string]any) (map[string]any, error) {
	var returnVal any

	for _, step := range steps {
		switch s := step.(type) {
		case *parser.LetBinding:
			val, err := sr.evalActionExpr(s.Value, ctx)
			if err != nil {
				return nil, fmt.Errorf("action %q, let %q: %w", name, s.Name, err)
			}
			ctx[s.Name] = val

		case *parser.AdapterCall:
			// Empty adapter namespace means this could be a local action call.
			if s.Adapter == "" {
				if calledAction := sr.findAction(s.Method); calledAction != nil {
					result, err := sr.executeLocalActionCall(calledAction, s.Args, ctx)
					if err != nil {
						return nil, fmt.Errorf("action %q calling %q: %w", name, s.Method, err)
					}
					if m, ok := result.(map[string]any); ok {
						ctx["body"] = m
					}
					break
				}
			}
			resp, err := sr.executeAdapterCall(s, ctx)
			if err != nil {
				return nil, fmt.Errorf("action %q, %s.%s: %w", name, s.Adapter, s.Method, err)
			}
			if !resp.OK {
				sr.lastActionError = resp.Error
				return nil, nil
			}
			// Store response as "body" for subsequent steps
			if len(resp.Actual) > 0 {
				var parsed any
				if err := json.Unmarshal(resp.Actual, &parsed); err == nil {
					ctx["body"] = parsed
				}
			}

		case *parser.ReturnStmt:
			val, err := sr.evalActionExpr(s.Value, ctx)
			if err != nil {
				return nil, fmt.Errorf("action %q return: %w", name, err)
			}
			returnVal = val

		case *parser.Call:
			adp, err := sr.resolveAdapterForCall(s.Namespace)
			if err != nil {
				return nil, err
			}
			args, err := sr.marshalCallArgs(s, ctx)
			if err != nil {
				return nil, fmt.Errorf("action %q, %s.%s: %w", name, s.Namespace, s.Method, err)
			}
			resp, err := adp.Call(sr.ctx, s.Method, args)
			if err != nil {
				return nil, fmt.Errorf("action %q, %s.%s: %w", name, s.Namespace, s.Method, err)
			}
			if !resp.OK {
				sr.lastActionError = resp.Error
				return nil, nil
			}
			if len(resp.Actual) > 0 {
				var parsed any
				if err := json.Unmarshal(resp.Actual, &parsed); err == nil {
					ctx["body"] = parsed
				}
			}
		}
	}

	// Convert return value or body to output map
	if returnVal != nil {
		if m, ok := returnVal.(map[string]any); ok {
			return m, nil
		}
		// Wrap non-map return in a body key
		return map[string]any{"body": returnVal}, nil
	}
	if body, ok := ctx["body"]; ok {
		if m, ok := body.(map[string]any); ok {
			return m, nil
		}
	}
	return nil, nil
}

// evalActionExpr evaluates an expression in an action context.
// Handles AdapterCall expressions (right side of let) and standard expressions.
func (sr *scopeRunner) evalActionExpr(expr parser.Expr, ctx map[string]any) (any, error) {
	switch e := expr.(type) {
	case parser.AdapterCall:
		// Empty adapter namespace means this could be a local action call.
		if e.Adapter == "" {
			if action := sr.findAction(e.Method); action != nil {
				return sr.executeLocalActionCall(action, e.Args, ctx)
			}
		}
		resp, err := sr.executeAdapterCallVal(e, ctx)
		if err != nil {
			return nil, err
		}
		if !resp.OK {
			sr.lastActionError = resp.Error
			return nil, nil
		}
		var parsed any
		if err := json.Unmarshal(resp.Actual, &parsed); err != nil {
			return nil, fmt.Errorf("parsing response: %w", err)
		}
		return parsed, nil
	case *parser.AdapterCall:
		// Empty adapter namespace means this could be a local action call.
		if e.Adapter == "" {
			if action := sr.findAction(e.Method); action != nil {
				return sr.executeLocalActionCall(action, e.Args, ctx)
			}
		}
		resp, err := sr.executeAdapterCall(e, ctx)
		if err != nil {
			return nil, err
		}
		if !resp.OK {
			sr.lastActionError = resp.Error
			return nil, nil
		}
		var parsed any
		if err := json.Unmarshal(resp.Actual, &parsed); err != nil {
			return nil, fmt.Errorf("parsing response: %w", err)
		}
		return parsed, nil
	default:
		val, ok := generator.Eval(expr, ctx)
		if !ok {
			return nil, fmt.Errorf("could not evaluate expression")
		}
		return val, nil
	}
}

// executeAdapterCallVal is the value-type variant of executeAdapterCall.
func (sr *scopeRunner) executeAdapterCallVal(call parser.AdapterCall, ctx map[string]any) (*spec.Response, error) {
	adp, err := sr.resolveAdapterForCall(call.Adapter)
	if err != nil {
		return nil, err
	}

	var resolved []any
	for _, arg := range call.Args {
		val, _ := generator.Eval(arg, ctx)
		resolved = append(resolved, val)
	}
	args, err := json.Marshal(resolved)
	if err != nil {
		return nil, fmt.Errorf("marshaling args: %w", err)
	}

	return adp.Call(sr.ctx, call.Method, args)
}

// executeAdapterCall executes an AdapterCall step, resolving the adapter by name.
func (sr *scopeRunner) executeAdapterCall(call *parser.AdapterCall, ctx map[string]any) (*spec.Response, error) {
	adp, err := sr.resolveAdapterForCall(call.Adapter)
	if err != nil {
		return nil, err
	}

	var resolved []any
	for _, arg := range call.Args {
		val, _ := generator.Eval(arg, ctx)
		resolved = append(resolved, val)
	}
	args, err := json.Marshal(resolved)
	if err != nil {
		return nil, fmt.Errorf("marshaling args: %w", err)
	}

	return adp.Call(sr.ctx, call.Method, args)
}

// resolveAdapterForCall resolves an adapter by namespace from the runner's adapter map.
func (sr *scopeRunner) resolveAdapterForCall(namespace string) (adapter.Adapter, error) {
	return sr.resolveAdapter(namespace)
}

// executeBefore resets the adapter to clean state, then executes the scope's
// before block steps. Returns the input context from before assignments.
func (sr *scopeRunner) executeBefore() (map[string]any, error) {
	if err := sr.resetAdapters(); err != nil {
		return nil, fmt.Errorf("resetting adapter: %w", err)
	}
	sr.lastActionError = ""

	if sr.scopeDef == nil || sr.scopeDef.Before == nil {
		return nil, nil
	}

	return sr.executeGivenSteps(sr.scopeDef.Before.Steps)
}

// executeAfter runs the scope's after block steps. Errors are logged to stderr
// but never propagated — cleanup must not affect test results.
func (sr *scopeRunner) executeAfter() {
	if sr.scopeDef == nil || sr.scopeDef.After == nil {
		return
	}
	if _, err := sr.executeGivenSteps(sr.scopeDef.After.Steps); err != nil {
		fmt.Fprintf(os.Stderr, "warning: after block in scope %q: %v\n", sr.scope, err)
	}
}

// resetAdapters resets all adapter states.
func (sr *scopeRunner) resetAdapters() error {
	for name, adp := range sr.runner.adapters {
		if err := adp.Reset(); err != nil {
			return fmt.Errorf("adapter %q: %w", name, err)
		}
	}
	return nil
}

func (sr *scopeRunner) runGivenScenario(sc *parser.Scenario) (CheckResult, error) {
	if _, err := sr.executeBefore(); err != nil {
		return CheckResult{}, fmt.Errorf("before block: %w", err)
	}
	defer sr.executeAfter()

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
	expectsError := hasErrorPseudoAssertion(sc.Then, sr.contractDef)

	if hasCalls(sc.Given.Steps) {
		input, err := sr.executeGivenSteps(sc.Given.Steps)
		if err != nil {
			if !expectsError || sr.lastActionError == "" {
				return nil, err
			}
		}
		// Run the contract action with the accumulated input.
		if sr.contractDef != nil && sr.contractDef.Action != nil {
			if _, err := sr.executeInput(input); err != nil {
				return nil, err
			}
			if sr.lastActionError != "" && !expectsError {
				return nil, fmt.Errorf("action failed: %s", sr.lastActionError)
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

// hasCalls returns true if any given step requires sequential execution
// (adapter calls or let bindings, not just static assignments).
func hasCalls(steps []parser.GivenStep) bool {
	for _, s := range steps {
		switch s.(type) {
		case *parser.Call, *parser.AdapterCall, *parser.LetBinding:
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
			val, _ := generator.Eval(s.Value, input)
			setPath(input, s.Path, val)
		case *parser.Call:
			// Empty namespace may be a local action call.
			if s.Namespace == "" {
				if action := sr.findAction(s.Method); action != nil {
					result, err := sr.executeLocalActionCall(action, s.Args, input)
					if err != nil {
						return nil, fmt.Errorf("calling action %q: %w", s.Method, err)
					}
					if m, ok := result.(map[string]any); ok {
						input["body"] = m
					}
					break
				}
			}
			adp, err := sr.resolveAdapterForCall(s.Namespace)
			if err != nil {
				return nil, fmt.Errorf("resolving adapter for %s.%s: %w", s.Namespace, s.Method, err)
			}
			args, err := sr.marshalCallArgs(s, input)
			if err != nil {
				return nil, fmt.Errorf("marshaling args for %s.%s: %w", s.Namespace, s.Method, err)
			}
			resp, err := adp.Call(sr.ctx, s.Method, args)
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
			// Make the action response available as "body" for subsequent steps.
			if len(resp.Actual) > 0 {
				var parsed any
				if err := json.Unmarshal(resp.Actual, &parsed); err == nil {
					input["body"] = parsed
				}
			}
		case *parser.AdapterCall:
			// Empty adapter namespace may be a local action call.
			if s.Adapter == "" {
				if action := sr.findAction(s.Method); action != nil {
					result, err := sr.executeLocalActionCall(action, s.Args, input)
					if err != nil {
						return nil, fmt.Errorf("calling action %q: %w", s.Method, err)
					}
					if m, ok := result.(map[string]any); ok {
						input["body"] = m
					}
					break
				}
			}
			resp, err := sr.executeAdapterCall(s, input)
			if err != nil {
				return nil, fmt.Errorf("executing %s.%s: %w", s.Adapter, s.Method, err)
			}
			if !resp.OK {
				sr.lastActionError = resp.Error
				return input, fmt.Errorf(
					"action %s.%s failed: %s",
					s.Adapter,
					s.Method,
					resp.Error,
				)
			}
			if len(resp.Actual) > 0 {
				var parsed any
				if err := json.Unmarshal(resp.Actual, &parsed); err == nil {
					input["body"] = parsed
				}
			}
		case *parser.LetBinding:
			val, err := sr.evalActionExpr(s.Value, input)
			if err != nil {
				return nil, fmt.Errorf("evaluating let %q: %w", s.Name, err)
			}
			input[s.Name] = val
		}
	}
	return input, nil
}

// marshalCallArgs converts Call expression arguments to JSON for the adapter.
// FieldRef args are resolved as locator names from the spec's locators map.
// The ctx provides the accumulated step context for expression evaluation
// (e.g., "body" from a previous action response).
func (sr *scopeRunner) marshalCallArgs(call *parser.Call, ctx map[string]any) (json.RawMessage, error) {
	var resolved []any
	for _, arg := range call.Args {
		switch a := arg.(type) {
		case parser.FieldRef:
			// Resolve locator name to CSS selector
			if selector, ok := sr.runner.spec.Locators[a.Path]; ok {
				resolved = append(resolved, selector)
			} else {
				// Try resolving as a context reference (e.g., body.access_token)
				if val, ok := generator.Eval(a, ctx); ok {
					resolved = append(resolved, val)
				} else {
					resolved = append(resolved, a.Path)
				}
			}
		default:
			val, _ := generator.Eval(arg, ctx)
			resolved = append(resolved, val)
		}
	}
	return json.Marshal(resolved)
}

func (sr *scopeRunner) runWhenScenario(sc *parser.Scenario) (CheckResult, error) {
	predicate := buildPredicate(sc.When.Predicates)
	needsPageIsolation := sc.Then != nil && hasPluginAssertions(sc.Then.Assertions)
	expectsError := hasErrorPseudoAssertion(sc.Then, sr.contractDef)

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
	if _, err := sr.executeBefore(); err != nil {
		return nil, fmt.Errorf("before block: %w", err)
	}
	defer sr.executeAfter()

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

// newPageWithNavigation creates a fresh Playwright page.
func (sr *scopeRunner) newPageWithNavigation() error {
	adp, err := sr.resolveAdapter("playwright")
	if err != nil {
		return fmt.Errorf("resolving playwright adapter: %w", err)
	}
	resp, err := adp.Call(sr.ctx, "new_page", nil)
	if err != nil {
		return fmt.Errorf("creating new page: %w", err)
	}
	if !resp.OK {
		return fmt.Errorf("creating new page: %s", resp.Error)
	}
	return nil
}

// closePage closes the current Playwright page.
func (sr *scopeRunner) closePage() error {
	adp, err := sr.resolveAdapter("playwright")
	if err != nil {
		return fmt.Errorf("resolving playwright adapter: %w", err)
	}
	resp, err := adp.Call(sr.ctx, "close_page", nil)
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
// pseudo-field (i.e., action error, not a return type field named "error").
// In v4, "error" in then-block assertions is always treated as the action error pseudo-field.
func hasErrorPseudoAssertion(then *parser.Block, _ *parser.Contract) bool {
	if then == nil {
		return false
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

// checkExprAssertion evaluates a v4 expression assertion (where Expr is a BinaryOp
// like `output.from.balance == from.balance - amount`).
//
// Field resolution rules (v4 plan §3):
//   - Bare names resolve ONLY against contract input fields.
//   - output.<field> resolves via the nested "output" map.
//   - input.<field> resolves via the nested "input" map.
//
// Output fields are NOT spread into the top-level namespace — that caused bare
// output refs to silently override input refs, making relational assertions
// non-falsifiable when input and output fields share names.
func (sr *scopeRunner) checkExprAssertion(
	name string,
	input map[string]any,
	a *parser.Assertion,
) (*Failure, error) {
	// Build assertion context: bare names resolve to input fields.
	// output.X resolves via the nested "output" map.
	ctx := make(map[string]any, len(input)+2)
	for k, v := range input {
		ctx[k] = v
	}
	ctx["input"] = input
	ctx["output"] = sr.lastOutput

	// Handle error pseudo-field in expression assertions
	if sr.lastActionError != "" {
		ctx["error"] = sr.lastActionError
	} else if _, hasError := ctx["error"]; !hasError {
		// If no error occurred and no error in output, set to nil
		ctx["error"] = nil
	}

	exprStr := spec.FormatExpr(a.Expr)
	val, ok := generator.Eval(a.Expr, ctx)
	if !ok {
		return &Failure{
			Name:        name,
			Scope:       sr.scope,
			Input:       input,
			Description: fmt.Sprintf("then assertion could not be evaluated: %s", exprStr),
		}, nil
	}
	b, isBool := val.(bool)
	if !isBool || !b {
		return &Failure{
			Name:        name,
			Scope:       sr.scope,
			Input:       input,
			Expected:    true,
			Actual:      val,
			Description: fmt.Sprintf("then assertion failed: %s", exprStr),
		}, nil
	}
	return nil, nil
}

func (sr *scopeRunner) checkSingleAssertion(
	name string,
	input map[string]any,
	a *parser.Assertion,
) (*Failure, error) {
	// v3 expression assertion (Expr field set, Target empty)
	if a.Expr != nil && a.Target == "" {
		return sr.checkExprAssertion(name, input, a)
	}

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

	property, callArgs, err := sr.buildAssertionCall(a)
	if err != nil {
		return nil, err
	}
	pluginName := a.Plugin
	if pluginName == "" {
		pluginName = "playwright"
	}
	adp, err := sr.resolveAdapter(pluginName)
	if err != nil {
		return nil, fmt.Errorf("resolving adapter %q for assertion: %w", pluginName, err)
	}
	resp, err := adp.Call(sr.ctx, property, callArgs)
	if err != nil {
		return nil, fmt.Errorf("querying %q: %w", a.Target, err)
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

	op := a.Operator
	if op == "" {
		op = "=="
	}

	if op == "==" {
		// Equality: compare actual (from adapter) vs expected in the runner.
		ok, cmpErr := compareEquality(resp.Actual, expected)
		if cmpErr != nil {
			return nil, fmt.Errorf("comparing %q: %w", a.Target, cmpErr)
		}
		if !ok {
			return &Failure{
				Name:        name,
				Scope:       sr.scope,
				Input:       input,
				Expected:    string(expected),
				Actual:      string(resp.Actual),
				Description: fmt.Sprintf("assertion %q failed: expected %s, got %s", a.Target, string(expected), string(resp.Actual)),
			}, nil
		}
		return nil, nil
	}

	// Non-equality operator: compare actual vs expected in the runner.
	ok, cmpErr := compareAssertion(op, resp.Actual, expected)
	if cmpErr != nil {
		return nil, fmt.Errorf("comparing %q with %s: %w", a.Target, op, cmpErr)
	}
	if !ok {
		return &Failure{
			Name:        name,
			Scope:       sr.scope,
			Input:       input,
			Expected:    fmt.Sprintf("%s %s", op, string(expected)),
			Actual:      string(resp.Actual),
			Description: fmt.Sprintf("assertion %q failed: got %s, expected %s %s", a.Target, string(resp.Actual), op, string(expected)),
		}, nil
	}
	return nil, nil
}

// compareAssertion evaluates a non-equality comparison between actual and expected
// JSON values. Supports !=, >, >=, <, <=. Relational operators require numeric values.
func compareAssertion(op string, actual, expected json.RawMessage) (bool, error) {
	if op == "!=" {
		var a, e any
		if err := json.Unmarshal(actual, &a); err != nil {
			return false, fmt.Errorf("unmarshaling actual: %w", err)
		}
		if err := json.Unmarshal(expected, &e); err != nil {
			return false, fmt.Errorf("unmarshaling expected: %w", err)
		}
		return fmt.Sprintf("%v", a) != fmt.Sprintf("%v", e), nil
	}

	// Relational operators require numeric values.
	var a, e float64
	if err := json.Unmarshal(actual, &a); err != nil {
		return false, fmt.Errorf("operator %s requires numeric actual, got %s", op, string(actual))
	}
	if err := json.Unmarshal(expected, &e); err != nil {
		return false, fmt.Errorf("operator %s requires numeric expected, got %s", op, string(expected))
	}

	switch op {
	case ">":
		return a > e, nil
	case ">=":
		return a >= e, nil
	case "<":
		return a < e, nil
	case "<=":
		return a <= e, nil
	default:
		return false, fmt.Errorf("unsupported operator %q", op)
	}
}

// buildAssertionCall constructs the Call method name and args for an assertion.
// For non-plugin assertions (e.g., body field paths), it returns (target, nil, nil).
// For plugin assertions (e.g., playwright.visible on a locator), it returns
// (property, JSON array with selector, nil).
func (sr *scopeRunner) buildAssertionCall(a *parser.Assertion) (string, json.RawMessage, error) {
	if a.Plugin == "" {
		return a.Target, nil, nil
	}
	selector, ok := sr.runner.spec.Locators[a.Target]
	if !ok {
		return "", nil, fmt.Errorf("locator %q not defined in locators block", a.Target)
	}
	args, err := json.Marshal([]string{selector})
	if err != nil {
		return "", nil, fmt.Errorf("marshaling assertion args: %w", err)
	}
	return a.Property, args, nil
}

// compareEquality checks if two JSON values are deeply equal after normalization.
func compareEquality(actual, expected json.RawMessage) (bool, error) {
	var actualNorm, expectedNorm any
	if err := json.Unmarshal(actual, &actualNorm); err != nil {
		return false, fmt.Errorf("normalizing actual: %w", err)
	}
	if err := json.Unmarshal(expected, &expectedNorm); err != nil {
		return false, fmt.Errorf("normalizing expected: %w", err)
	}
	return reflect.DeepEqual(actualNorm, expectedNorm), nil
}

// hasOutputField returns true if the contract's return type model declares a field with the given name.
func (sr *scopeRunner) hasOutputField(name string) bool {
	if sr.contractDef == nil || sr.contractDef.ReturnType.Name == "" {
		return false
	}
	typeName := sr.contractDef.ReturnType.Name
	for _, m := range sr.runner.spec.Models {
		if m.Name == typeName {
			for _, f := range m.Fields {
				if f.Name == name {
					return true
				}
			}
			return false
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
		if _, err := sr.executeBefore(); err != nil {
			return CheckResult{}, fmt.Errorf("before block: %w", err)
		}

		input, err := sr.generator.GenerateInput()
		if err != nil {
			sr.executeAfter()
			return CheckResult{}, err
		}

		output, err := sr.executeInput(input)
		if err != nil {
			sr.executeAfter()
			return CheckResult{}, err
		}
		if sr.lastActionError != "" {
			sr.executeAfter()
			return CheckResult{}, fmt.Errorf("action failed: %s", sr.lastActionError)
		}

		check.InputsRun = i + 1

		ctx := buildInvariantContext(input, output)

		if !evalGuard(inv.When, ctx) {
			sr.executeAfter()
			continue
		}

		if f := checkInvariantAssertions(inv.Name, sr.scope, input, inv.Assertions, ctx); f != nil {
			sr.executeAfter()
			f = sr.shrinkInvariantFailure(f, inv)
			check.Passed = false
			check.FailedAt = i + 1
			check.Failure = f
			return check, nil
		}
		sr.executeAfter()
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
	expectsError := hasErrorPseudoAssertion(then, sr.contractDef)
	shrunk := generator.Shrink(
		input, fields, models,
		func(candidate map[string]any) bool {
			if _, err := sr.executeBefore(); err != nil {
				return false
			}
			defer sr.executeAfter()
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
			if _, err := sr.executeBefore(); err != nil {
				return false
			}
			defer sr.executeAfter()
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
		exprStr := spec.FormatExpr(a.Expr)
		val, ok := generator.Eval(a.Expr, ctx)
		if !ok {
			return &Failure{
				Name:        name,
				Scope:       scope,
				Input:       input,
				Description: fmt.Sprintf("invariant could not be evaluated: %s", exprStr),
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
				Description: fmt.Sprintf("invariant failed: %s", exprStr),
			}
		}
	}
	return nil
}

// buildInvariantContext builds the eval context for invariant assertions.
//
// Field resolution rules (v4 plan §3):
//   - Bare names resolve ONLY against contract input fields.
//   - output.<field> resolves via the nested "output" map.
//   - input.<field> resolves via the nested "input" map (redundant but explicit).
//   - config.<key> resolves via the generator's config (handled separately).
//
// Output fields are NOT spread into the top-level namespace; spreading caused
// bare refs to output fields to silently shadow input fields, making invariants
// like "output.a + output.b == a + b" vacuously true when a,b resolved to output.
func buildInvariantContext(input, output map[string]any) map[string]any {
	ctx := make(map[string]any, len(input)+2)
	for k, v := range input {
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
			val, _ := generator.Eval(a.Value, nil)
			setPath(result, a.Path, val)
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
