package migrate

import (
	"fmt"
	"sort"
	"strings"

	"github.com/bamsammich/speclang/v3/pkg/spec"
)

// hasSynthesizableConfig returns true if the scope has v2 config that can be
// converted into a synthesized action (method/path for HTTP, command for process).
func hasSynthesizableConfig(sc *spec.Scope) bool {
	if sc.Config == nil {
		return false
	}
	_, hasMethod := sc.Config["method"]
	_, hasPath := sc.Config["path"]
	_, hasCommand := sc.Config["command"]
	return hasMethod || hasPath || hasCommand
}

// synthesizedAction holds the pieces of an action generated from v2 use+config.
type synthesizedAction struct {
	name        string
	params      []*spec.Param
	headerCalls []string // e.g., http.header("X-Custom", "value")
	callExpr    string   // e.g., http.post("/path", { field: field })
}

// synthesizeAction builds a v3 action definition from v2 scope use+config+contract.
func synthesizeAction(sc *spec.Scope) *synthesizedAction {
	adapter := sc.Use
	if adapter == "" {
		adapter = "http"
	}

	sa := &synthesizedAction{
		name: sc.Name,
	}

	// Extract method and path from config
	method := "post"
	path := "/"
	if sc.Config != nil {
		if m, ok := sc.Config["method"]; ok {
			if ls, ok := m.(spec.LiteralString); ok {
				method = strings.ToLower(ls.Value)
			}
		}
		if p, ok := sc.Config["path"]; ok {
			if ls, ok := p.(spec.LiteralString); ok {
				path = ls.Value
			} else {
				path = spec.FormatExpr(p)
			}
		}

		// Extract header configs (header_X_Name pattern)
		for k, v := range sc.Config {
			if strings.HasPrefix(k, "header_") {
				headerName := strings.ReplaceAll(strings.TrimPrefix(k, "header_"), "_", "-")
				sa.headerCalls = append(sa.headerCalls, fmt.Sprintf("%s.header(%q, %s)", adapter, headerName, spec.FormatExpr(v)))
			}
		}
		sort.Strings(sa.headerCalls)
	}

	// Build params from contract input fields
	if sc.Contract != nil {
		for _, f := range sc.Contract.Input {
			sa.params = append(sa.params, &spec.Param{
				Name: f.Name,
				Type: f.Type,
			})
		}
	}

	// Build the call expression
	switch adapter {
	case "process":
		sa.callExpr = buildProcessCallExpr(sc, sa.params)
	default:
		sa.callExpr = buildHTTPCallExpr(adapter, method, path, sa.params)
	}

	return sa
}

// buildHTTPCallExpr builds: http.post("/path", { field1: field1, field2: field2 })
func buildHTTPCallExpr(adapter, method, path string, params []*spec.Param) string {
	// GET/DELETE typically don't have a body
	needsBody := method == "post" || method == "put" || method == "patch"

	if !needsBody || len(params) == 0 {
		return fmt.Sprintf("%s.%s(%q)", adapter, method, path)
	}

	// Build object literal body from params
	fields := make([]string, len(params))
	for i, p := range params {
		fields[i] = p.Name + ": " + p.Name
	}
	body := "{ " + strings.Join(fields, ", ") + " }"
	return fmt.Sprintf("%s.%s(%q, %s)", adapter, method, path, body)
}

// buildProcessCallExpr builds: process.exec(field1, field2)
func buildProcessCallExpr(sc *spec.Scope, params []*spec.Param) string {
	args := make([]string, len(params))
	for i, p := range params {
		args[i] = p.Name
	}
	return "process.exec(" + strings.Join(args, ", ") + ")"
}

// transformAssertion converts a v2 assertion AST node into v3 assertion text.
func transformAssertion(a *spec.Assertion, locators map[string]string) string {
	// Expression assertion (invariants)
	if a.Expr != nil {
		return spec.FormatExpr(a.Expr)
	}

	// Plugin assertion: target@plugin.property
	if a.Plugin != "" {
		selector := resolveLocator(a.Target, locators)
		op := a.Operator
		if op == "" || op == ":" {
			op = "=="
		}
		return fmt.Sprintf("%s.%s(%s) %s %s", a.Plugin, a.Property, selector, op, spec.FormatExpr(a.Expected))
	}

	// Path assertion: field op value
	op := a.Operator
	if op == "" || op == ":" {
		op = "=="
	}

	// For then-block assertions referencing output fields, convert
	// v2 "from.balance" to v3 "output.from.balance" style if needed.
	// However, v2 used bare paths that the runner resolved against the output,
	// and v3 also supports bare paths in then blocks. Keep as-is.
	return fmt.Sprintf("%s %s %s", a.Target, op, spec.FormatExpr(a.Expected))
}

// resolveLocator looks up a locator name and returns a formatted selector string.
// If the name is not found in the map, returns it quoted as a best-effort.
func resolveLocator(name string, locators map[string]string) string {
	if locators != nil {
		if sel, ok := locators[name]; ok {
			return formatSelector(sel)
		}
	}
	// Not a locator — return as a quoted string (might be a literal selector already)
	return fmt.Sprintf("%q", name)
}

// formatSelector wraps a CSS selector in single quotes, adding brackets if it
// looks like an attribute selector without them (e.g., data-testid="email").
func formatSelector(sel string) string {
	// v2 parser strips brackets from locator selectors — add them back
	// if the selector doesn't already start with a CSS combinator.
	if sel != "" && !strings.HasPrefix(sel, "[") && !strings.HasPrefix(sel, ".") &&
		!strings.HasPrefix(sel, "#") && !strings.HasPrefix(sel, ">") {
		sel = "[" + sel + "]"
	}
	if strings.Contains(sel, `"`) {
		return "'" + sel + "'"
	}
	return fmt.Sprintf("'%s'", sel)
}

// inferAdapterConfigs extracts adapter config blocks from a v2 target.
// Returns map[adapter_name] -> map[key] -> formatted_value_string.
func inferAdapterConfigs(s *spec.Spec) map[string]map[string]string {
	configs := make(map[string]map[string]string)

	if s.Target == nil || len(s.Target.Fields) == 0 {
		// Also handle v3 AdapterConfigs already present
		if s.AdapterConfigs != nil {
			for name, cfg := range s.AdapterConfigs {
				configs[name] = make(map[string]string)
				for k, v := range cfg {
					configs[name][k] = spec.FormatExpr(v)
				}
			}
		}
		return configs
	}

	// Determine which adapters are used across all scopes
	adaptersUsed := make(map[string]bool)
	for _, sc := range s.Scopes {
		if sc.Use != "" {
			adaptersUsed[sc.Use] = true
		}
	}

	// Playwright-specific fields
	playwrightFields := map[string]bool{
		"headless": true,
		"timeout":  true,
	}

	for key, val := range s.Target.Fields {
		valStr := spec.FormatExpr(val)

		// Route field to the appropriate adapter config
		if playwrightFields[key] && adaptersUsed["playwright"] {
			if configs["playwright"] == nil {
				configs["playwright"] = make(map[string]string)
			}
			configs["playwright"][key] = valStr
		}

		// base_url goes to all adapters that use it
		if key == "base_url" {
			for a := range adaptersUsed {
				if configs[a] == nil {
					configs[a] = make(map[string]string)
				}
				configs[a][key] = valStr
			}
			// If no scopes found, default to http
			if len(adaptersUsed) == 0 {
				if configs["http"] == nil {
					configs["http"] = make(map[string]string)
				}
				configs["http"][key] = valStr
			}
			continue
		}

		// Non-playwright fields default to http
		if !playwrightFields[key] {
			if configs["http"] == nil {
				configs["http"] = make(map[string]string)
			}
			configs["http"][key] = valStr
		}
	}

	return configs
}

// transformBodyRefs detects implicit body.X references in before/after block steps
// and wraps preceding adapter calls with let bindings.
func transformBodyRefs(steps []spec.GivenStep, scopeAdapter string) []spec.GivenStep {
	if len(steps) == 0 {
		return steps
	}

	result := make([]spec.GivenStep, 0, len(steps))
	var lastCallIdx int = -1

	for i, step := range steps {
		// Check if this step contains body.X references
		hasBodyRef := stepHasBodyRef(step)

		if hasBodyRef && lastCallIdx >= 0 && lastCallIdx == i-1 {
			// Wrap the previous call in a let binding
			prevStep := result[len(result)-1]
			result[len(result)-1] = wrapCallInLet(prevStep, "result")

			// Rewrite body.X → result.X in current step
			step = rewriteBodyRefs(step, "result")
		}

		// Track calls for body ref resolution
		switch step.(type) {
		case *spec.Call, *spec.AdapterCall:
			lastCallIdx = i
		}

		result = append(result, step)
	}

	return result
}

// stepHasBodyRef checks if a step contains FieldRef paths starting with "body."
func stepHasBodyRef(step spec.GivenStep) bool {
	switch s := step.(type) {
	case *spec.Call:
		for _, arg := range s.Args {
			if exprHasBodyRef(arg) {
				return true
			}
		}
	case *spec.AdapterCall:
		for _, arg := range s.Args {
			if exprHasBodyRef(arg) {
				return true
			}
		}
	}
	return false
}

// exprHasBodyRef checks if an expression tree contains body.X references.
func exprHasBodyRef(e spec.Expr) bool {
	if e == nil {
		return false
	}
	switch v := e.(type) {
	case spec.FieldRef:
		return strings.HasPrefix(v.Path, "body.")
	case spec.BinaryOp:
		return exprHasBodyRef(v.Left) || exprHasBodyRef(v.Right)
	case spec.UnaryOp:
		return exprHasBodyRef(v.Operand)
	}
	return false
}

// wrapCallInLet wraps a Call or AdapterCall step in a LetBinding.
func wrapCallInLet(step spec.GivenStep, varName string) spec.GivenStep {
	switch s := step.(type) {
	case *spec.Call:
		return &spec.LetBinding{
			Name: varName,
			Value: spec.AdapterCall{
				Adapter: s.Namespace,
				Method:  s.Method,
				Args:    s.Args,
			},
		}
	case *spec.AdapterCall:
		return &spec.LetBinding{
			Name:  varName,
			Value: *s,
		}
	}
	return step
}

// rewriteBodyRefs replaces body.X with varName.X in a step's expressions.
func rewriteBodyRefs(step spec.GivenStep, varName string) spec.GivenStep {
	switch s := step.(type) {
	case *spec.Call:
		newArgs := make([]spec.Expr, len(s.Args))
		for i, arg := range s.Args {
			newArgs[i] = rewriteBodyRefsInExpr(arg, varName)
		}
		return &spec.Call{
			Namespace: s.Namespace,
			Method:    s.Method,
			Args:      newArgs,
		}
	case *spec.AdapterCall:
		newArgs := make([]spec.Expr, len(s.Args))
		for i, arg := range s.Args {
			newArgs[i] = rewriteBodyRefsInExpr(arg, varName)
		}
		return &spec.AdapterCall{
			Adapter: s.Adapter,
			Method:  s.Method,
			Args:    newArgs,
		}
	}
	return step
}

// rewriteBodyRefsInExpr replaces body.X with varName.X in an expression tree.
func rewriteBodyRefsInExpr(e spec.Expr, varName string) spec.Expr {
	if e == nil {
		return nil
	}
	switch v := e.(type) {
	case spec.FieldRef:
		if strings.HasPrefix(v.Path, "body.") {
			return spec.FieldRef{Path: varName + "." + strings.TrimPrefix(v.Path, "body.")}
		}
		if v.Path == "body" {
			return spec.FieldRef{Path: varName}
		}
		return v
	case spec.BinaryOp:
		return spec.BinaryOp{
			Left:  rewriteBodyRefsInExpr(v.Left, varName),
			Right: rewriteBodyRefsInExpr(v.Right, varName),
			Op:    v.Op,
		}
	case spec.UnaryOp:
		return spec.UnaryOp{
			Operand: rewriteBodyRefsInExpr(v.Operand, varName),
			Op:      v.Op,
		}
	default:
		return e
	}
}

// sortedKeys returns the keys of a map[string]map[string]string in sorted order.
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// sortedStringMapKeys returns sorted keys from a map[string]string.
func sortedStringMapKeys(m map[string]string) []string {
	return sortedKeys(m)
}
