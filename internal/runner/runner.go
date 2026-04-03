package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"

	"github.com/bamsammich/speclang/v3/internal/adapter"
	"github.com/bamsammich/speclang/v3/internal/generator"
	"github.com/bamsammich/speclang/v3/internal/parser"
	"github.com/bamsammich/speclang/v3/pkg/spec"
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
	ctx             context.Context
	runner          *Runner
	adapter         adapter.Adapter // resolved from scope.Use (v2 only, nil in v3)
	generator       *generator.Generator
	scopeDef        *parser.Scope
	scope           string
	path            string
	method          string
	lastActionError string         // captured when an action returns {ok: false}
	lastOutput      map[string]any // captured output from the last action execution
}

func (sr *scopeRunner) scenarios() []*parser.Scenario {
	return sr.scopeDef.Scenarios
}

func (sr *scopeRunner) invariants() []*parser.Invariant {
	return sr.scopeDef.Invariants
}

// Verify runs all scopes' scenarios and invariants, returning results.
func (r *Runner) Verify(ctx context.Context) (*Result, error) {
	res := &Result{Spec: r.spec.Name}

	for _, scope := range r.spec.Scopes {
		sr, err := r.newScopeRunner(ctx, scope)
		if err != nil {
			return nil, err
		}
		if err := sr.run(res); err != nil {
			return nil, err
		}
	}

	return res, nil
}

func (r *Runner) newScopeRunner(ctx context.Context, scope *parser.Scope) (*scopeRunner, error) {
	gen := generator.New(scope.Contract, r.spec.Models, r.seed)

	sr := &scopeRunner{
		ctx:       ctx,
		runner:    r,
		generator: gen,
		scopeDef:  scope,
		scope:     scope.Name,
	}

	// v3 path: no Use directive, adapters resolved per-call
	if scope.Use == "" {
		return sr, nil
	}

	// v2 compat path: single adapter per scope via Use directive
	adp, ok := r.adapters[scope.Use]
	if !ok {
		return nil, fmt.Errorf("no adapter for plugin %q in scope %q", scope.Use, scope.Name)
	}
	sr.adapter = adp
	sr.path = evalConfigString(scope.Config, "path")
	sr.method = strings.ToUpper(evalConfigString(scope.Config, "method"))
	return sr, nil
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

// evalConfigString evaluates a config key to a string via generator.Eval.
func evalConfigString(config map[string]parser.Expr, key string) string {
	if config == nil {
		return ""
	}
	expr, ok := config[key]
	if !ok {
		return ""
	}
	val, ok := generator.Eval(expr, nil)
	if !ok {
		return ""
	}
	if s, isStr := val.(string); isStr {
		return s
	}
	return fmt.Sprintf("%v", val)
}

// executeInput sends an input map to the adapter and returns the parsed response.
// In v3 mode (contract has an action), it dispatches through the named action.
// In v2 mode (scope has Use/Config), it builds the action from config.
// The captured error string is stored in sr.lastActionError.
func (sr *scopeRunner) executeInput(input map[string]any) (map[string]any, error) {
	sr.lastActionError = ""
	sr.lastOutput = nil

	// v3 path: contract references a named action
	if sr.scopeDef.Contract != nil && sr.scopeDef.Contract.Action != "" {
		output, err := sr.executeContractAction(input)
		sr.lastOutput = output
		return output, err
	}

	// v2 compat path: build action from scope config
	inputJSON, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("marshaling input: %w", err)
	}

	actionName, args, err := sr.buildAction(inputJSON)
	if err != nil {
		return nil, err
	}

	resp, err := sr.adapter.Call(sr.ctx, actionName, args)
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
	sr.lastOutput = output
	return output, nil
}

// executeContractAction looks up the contract's named action and executes its body.
// The input map fields are bound as action parameters.
func (sr *scopeRunner) executeContractAction(input map[string]any) (map[string]any, error) {
	actionName := sr.scopeDef.Contract.Action
	actionDef := sr.findAction(actionName)
	if actionDef == nil {
		return nil, fmt.Errorf("scope %q: contract references undefined action %q", sr.scope, actionName)
	}

	// Build parameter context from input map.
	ctx := make(map[string]any, len(input))
	for k, v := range input {
		ctx[k] = v
	}

	// Fill missing optional fields with nil; error on missing required fields.
	for _, field := range sr.scopeDef.Contract.Input {
		if _, exists := ctx[field.Name]; !exists {
			if field.Type.Optional {
				ctx[field.Name] = nil
			} else {
				return nil, fmt.Errorf("scope %q: missing required input field %q", sr.scope, field.Name)
			}
		}
	}

	return sr.executeActionBody(actionDef, ctx)
}

// findAction looks up an action by name, first in the scope, then at spec level.
func (sr *scopeRunner) findAction(name string) *parser.ActionDef {
	for _, a := range sr.scopeDef.Actions {
		if a.Name == name {
			return a
		}
	}
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

// executeActionBody executes an action's body steps and returns the output.
// Handles LetBinding, AdapterCall, ReturnStmt, and legacy Call steps.
func (sr *scopeRunner) executeActionBody(action *parser.ActionDef, ctx map[string]any) (map[string]any, error) {
	var returnVal any

	for _, step := range action.Body {
		switch s := step.(type) {
		case *parser.LetBinding:
			val, err := sr.evalActionExpr(s.Value, ctx)
			if err != nil {
				return nil, fmt.Errorf("action %q, let %q: %w", action.Name, s.Name, err)
			}
			ctx[s.Name] = val

		case *parser.AdapterCall:
			// Empty adapter namespace means this could be a local action call.
			if s.Adapter == "" {
				if calledAction := sr.findAction(s.Method); calledAction != nil {
					result, err := sr.executeLocalActionCall(calledAction, s.Args, ctx)
					if err != nil {
						return nil, fmt.Errorf("action %q calling %q: %w", action.Name, s.Method, err)
					}
					if m, ok := result.(map[string]any); ok {
						ctx["body"] = m
					}
					break
				}
			}
			resp, err := sr.executeAdapterCall(s, ctx)
			if err != nil {
				return nil, fmt.Errorf("action %q, %s.%s: %w", action.Name, s.Adapter, s.Method, err)
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
				return nil, fmt.Errorf("action %q return: %w", action.Name, err)
			}
			returnVal = val

		case *parser.Call:
			adp, err := sr.resolveAdapterForCall(s.Namespace)
			if err != nil {
				return nil, err
			}
			args, err := sr.marshalCallArgs(s, ctx)
			if err != nil {
				return nil, fmt.Errorf("action %q, %s.%s: %w", action.Name, s.Namespace, s.Method, err)
			}
			resp, err := adp.Call(sr.ctx, s.Method, args)
			if err != nil {
				return nil, fmt.Errorf("action %q, %s.%s: %w", action.Name, s.Namespace, s.Method, err)
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

// resolveAdapterForCall resolves an adapter by namespace. In v2 mode (single
// adapter), returns sr.adapter. In v3 mode, looks up by name.
func (sr *scopeRunner) resolveAdapterForCall(namespace string) (adapter.Adapter, error) {
	if sr.adapter != nil && (namespace == "" || namespace == sr.scopeDef.Use) {
		return sr.adapter, nil
	}
	return sr.resolveAdapter(namespace)
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

// evalConfigArgs evaluates the "args" config expression into exec arguments.
// Array form preserves each element as a separate argument; string form splits on whitespace.
func evalConfigArgs(argsExpr parser.Expr, inputMap map[string]any) []any {
	switch e := argsExpr.(type) {
	case parser.ArrayLiteral:
		var args []any
		for _, elem := range e.Elements {
			if val, ok := generator.Eval(elem, inputMap); ok {
				args = append(args, val)
			}
		}
		return args
	default:
		val, ok := generator.Eval(e, inputMap)
		if !ok {
			return nil
		}
		s, isStr := val.(string)
		if !isStr {
			return nil
		}
		var args []any
		for _, a := range strings.Fields(s) {
			args = append(args, a)
		}
		return args
	}
}

func (sr *scopeRunner) collectExecArgs(inputMap map[string]any) []any {
	var execArgs []any
	if argsExpr, ok := sr.scopeDef.Config["args"]; ok {
		execArgs = append(execArgs, evalConfigArgs(argsExpr, inputMap)...)
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

// executeBefore resets the adapter to clean state, then executes the scope's
// before block steps. Returns the input context from before assignments.
func (sr *scopeRunner) executeBefore() (map[string]any, error) {
	if err := sr.resetAdapters(); err != nil {
		return nil, fmt.Errorf("resetting adapter: %w", err)
	}
	sr.lastActionError = ""

	if sr.scopeDef.Before == nil {
		return nil, nil
	}

	return sr.executeGivenSteps(sr.scopeDef.Before.Steps)
}

// executeAfter runs the scope's after block steps. Errors are logged to stderr
// but never propagated — cleanup must not affect test results.
func (sr *scopeRunner) executeAfter() {
	if sr.scopeDef.After == nil {
		return
	}
	if _, err := sr.executeGivenSteps(sr.scopeDef.After.Steps); err != nil {
		fmt.Fprintf(os.Stderr, "warning: after block in scope %q: %v\n", sr.scope, err)
	}
}

// resetAdapters resets the adapter state. In v2 mode (single adapter), resets
// that adapter. In v3 mode, resets all adapters in the runner.
func (sr *scopeRunner) resetAdapters() error {
	if sr.adapter != nil {
		return sr.adapter.Reset()
	}
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

// newPageWithNavigation creates a fresh page and navigates to the scope's configured URL.
func (sr *scopeRunner) newPageWithNavigation() error {
	resp, err := sr.adapter.Call(sr.ctx, "new_page", nil)
	if err != nil {
		return fmt.Errorf("creating new page: %w", err)
	}
	if !resp.OK {
		return fmt.Errorf("creating new page: %s", resp.Error)
	}

	url := evalConfigString(sr.scopeDef.Config, "url")
	if url != "" {
		args, err := json.Marshal([]string{url})
		if err != nil {
			return fmt.Errorf("marshaling goto args: %w", err)
		}
		resp, err := sr.adapter.Call(sr.ctx, "goto", args)
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
	resp, err := sr.adapter.Call(sr.ctx, "close_page", nil)
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

// checkExprAssertion evaluates a v3 expression assertion (where Expr is a BinaryOp
// like `from.balance == from.balance - amount`). The context includes both the given
// input and the action output, allowing field references to resolve against output.
func (sr *scopeRunner) checkExprAssertion(
	name string,
	input map[string]any,
	a *parser.Assertion,
) (*Failure, error) {
	// Build assertion context: output fields are directly accessible,
	// input fields accessible as-is (for given references like amount).
	ctx := make(map[string]any)
	// Input values first (from given block)
	for k, v := range input {
		ctx[k] = v
	}
	// Output values overlay (from action response)
	if sr.lastOutput != nil {
		for k, v := range sr.lastOutput {
			ctx[k] = v
		}
	}
	// Also provide namespaced access
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
	resp, err := sr.adapter.Call(sr.ctx, property, callArgs)
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
	expectsError := hasErrorPseudoAssertion(then, sr.scopeDef.Contract)
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
