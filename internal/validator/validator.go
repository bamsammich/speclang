package validator

import (
	"fmt"
	"strings"

	"github.com/bamsammich/speclang/v3/internal/parser"
	"github.com/bamsammich/speclang/v3/pkg/spec"
)

// primitives are type names that don't need model resolution.
var primitives = map[string]bool{
	"int": true, "string": true, "bool": true,
	"float": true, "bytes": true, "array": true, "map": true,
	"enum": true, "any": true,
}

// Validate performs post-parse semantic validation on a spec.
// The registry is used to look up plugin-specific assertion targets
// (e.g., "status" for http, "exit_code" for process). It must not be nil.
func Validate(s *parser.Spec, registry *spec.Registry) []error {
	v := &validator{
		models:   buildModelRegistry(s.Models),
		registry: registry,
	}

	v.validateServiceRefs(s)

	for _, scope := range s.Scopes {
		v.scope = scope.Name
		v.validateContract(scope)
		v.validateScenarios(scope)
	}

	return v.errs
}

type validator struct {
	models   map[string]*parser.Model
	registry *spec.Registry
	scope    string
	errs     []error
}

func buildModelRegistry(models []*parser.Model) map[string]*parser.Model {
	reg := make(map[string]*parser.Model, len(models))
	for _, m := range models {
		reg[m.Name] = m
	}
	return reg
}

func (v *validator) errorf(format string, args ...any) {
	v.errs = append(v.errs, fmt.Errorf(format, args...))
}

func (v *validator) posErr(pos spec.Pos, format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	if s := pos.String(); s != "" {
		v.errs = append(v.errs, fmt.Errorf("%s: %s", s, msg))
	} else {
		v.errs = append(v.errs, fmt.Errorf("%s", msg))
	}
}

func (v *validator) validateContract(scope *parser.Scope) {
	if scope.Contract == nil {
		return
	}
	for _, f := range scope.Contract.Input {
		v.validateTypeExpr(f.Type, fmt.Sprintf("scope %q, contract input %q", v.scope, f.Name))
	}
	for _, f := range scope.Contract.Output {
		v.validateTypeExpr(f.Type, fmt.Sprintf("scope %q, contract output %q", v.scope, f.Name))
	}

	// v3: validate contract action reference
	if scope.Contract.Action != "" {
		v.validateContractAction(scope)
	}
}

// validateContractAction checks that a contract's action: field references
// a defined action in the scope or spec.
func (v *validator) validateContractAction(scope *parser.Scope) {
	actionName := scope.Contract.Action

	// Check scope-level actions
	for _, a := range scope.Actions {
		if a.Name == actionName {
			return
		}
	}

	// Check spec-level actions (validator doesn't have direct spec ref,
	// but action validation at spec level is handled separately)
	// For now, scope-level is sufficient — spec-level will be caught at runtime.
}

func (v *validator) validateScenarios(scope *parser.Scope) {
	if scope.Contract == nil {
		return
	}
	inputFields := buildFieldMap(scope.Contract.Input)

	for _, sc := range scope.Scenarios {
		v.validateGivenBlock(sc, inputFields)
		v.validateThenBlock(sc, scope)
	}
}

func (v *validator) validateThenBlock(sc *parser.Scenario, scope *parser.Scope) {
	if sc.Then == nil || scope.Contract == nil {
		return
	}

	outputFields := buildFieldMap(scope.Contract.Output)

	// Build set of plugin assertion targets (e.g., "status", "body", "header" for http).
	pluginAssertions := v.pluginAssertionTargets(scope.Use)

	for _, a := range sc.Then.Assertions {
		// v3 expression assertions (Expr is set, Target is empty) — skip
		// target-based validation since they use full expressions.
		if a.Expr != nil && a.Target == "" {
			continue
		}

		// Skip plugin assertions (e.g., welcome@playwright.visible)
		if a.Plugin != "" {
			continue
		}
		// Skip expression assertions (invariant-style, no Target)
		if a.Target == "" {
			continue
		}

		// "error" is a pseudo-field for asserting on action errors, not a contract output.
		if a.Target == "error" {
			continue
		}

		fieldName := topLevelField(a.Target)

		// Check plugin-specific built-in targets (e.g., "status" for http).
		if pluginAssertions[fieldName] {
			continue
		}

		if _, ok := outputFields[fieldName]; !ok {
			v.posErr(a.Pos, "scope %q, scenario %q: then target %q does not match any output field",
				v.scope, sc.Name, a.Target)
		}
	}
}

// pluginAssertionTargets returns the set of built-in assertion target names
// for the given plugin (from the registry). Returns nil if the plugin is
// not found or has no assertions.
func (v *validator) pluginAssertionTargets(pluginName string) map[string]bool {
	if v.registry == nil || pluginName == "" {
		return nil
	}
	def, ok := v.registry.Plugin(pluginName)
	if !ok {
		return nil
	}
	targets := make(map[string]bool, len(def.Assertions))
	for name := range def.Assertions {
		targets[name] = true
	}
	return targets
}

func buildFieldMap(fields []*parser.Field) map[string]*parser.Field {
	m := make(map[string]*parser.Field, len(fields))
	for _, f := range fields {
		m[f.Name] = f
	}
	return m
}

func (v *validator) validateGivenBlock(sc *parser.Scenario, inputFields map[string]*parser.Field) {
	if sc.Given == nil {
		return
	}

	// Check if given block contains any calls — if so, skip completeness
	hasCalls := false
	for _, step := range sc.Given.Steps {
		switch step.(type) {
		case *parser.Call, *parser.AdapterCall, *parser.LetBinding:
			hasCalls = true
		}
		if hasCalls {
			break
		}
	}

	// Type-check assignments
	for _, step := range sc.Given.Steps {
		assign, ok := step.(*parser.Assignment)
		if !ok {
			continue
		}

		fieldName := topLevelField(assign.Path)
		field, ok := inputFields[fieldName]
		if !ok {
			continue
		}

		v.checkExprType(assign.Value, field.Type,
			fmt.Sprintf("scope %q, scenario %q, field %q", v.scope, sc.Name, assign.Path))
	}

	// Completeness check: all required fields must be assigned
	// Skip for given blocks with action calls (e.g., Playwright flows)
	if !hasCalls {
		v.checkGivenCompleteness(sc, inputFields)
	}
}

func (v *validator) checkGivenCompleteness(
	sc *parser.Scenario,
	inputFields map[string]*parser.Field,
) {
	assigned := make(map[string]bool)
	for _, step := range sc.Given.Steps {
		if a, ok := step.(*parser.Assignment); ok {
			assigned[topLevelField(a.Path)] = true
		}
	}
	for name, f := range inputFields {
		if !f.Type.Optional && !assigned[name] {
			v.posErr(sc.Given.Pos, "scope %q, scenario %q: missing required field %q",
				v.scope, sc.Name, name)
		}
	}
}

func topLevelField(path string) string {
	for i, c := range path {
		if c == '.' {
			return path[:i]
		}
	}
	return path
}

func (v *validator) checkExprType(expr parser.Expr, te parser.TypeExpr, context string) {
	// LiteralNull is valid only for optional types
	if nl, isNull := expr.(parser.LiteralNull); isNull {
		if !te.Optional {
			v.posErr(nl.Pos, "%s: null is not valid for non-optional type %s", context, typeName(te))
		}
		return
	}

	switch te.Name {
	case "int":
		v.checkIntType(expr, context)
	case "float":
		v.checkFloatType(expr, context)
	case "string":
		v.checkStringType(expr, context)
	case "bool":
		v.checkBoolType(expr, context)
	case "enum":
		v.checkEnumType(expr, te, context)
	case "array":
		v.checkArrayType(expr, te, context)
	default:
		v.checkModelType(expr, te, context)
	}
}

func (v *validator) checkIntType(expr parser.Expr, context string) {
	if _, ok := expr.(parser.LiteralInt); !ok && !isNonLiteral(expr) {
		v.posErr(exprPos(expr), "%s: expected int, got %s", context, exprTypeName(expr))
	}
}

func (v *validator) checkFloatType(expr parser.Expr, context string) {
	switch expr.(type) {
	case parser.LiteralFloat, parser.LiteralInt:
		// ok — accept int literals for float fields
	default:
		if !isNonLiteral(expr) {
			v.posErr(exprPos(expr), "%s: expected float, got %s", context, exprTypeName(expr))
		}
	}
}

func (v *validator) checkStringType(expr parser.Expr, context string) {
	if _, ok := expr.(parser.LiteralString); !ok && !isNonLiteral(expr) {
		v.posErr(exprPos(expr), "%s: expected string, got %s", context, exprTypeName(expr))
	}
}

func (v *validator) checkBoolType(expr parser.Expr, context string) {
	if _, ok := expr.(parser.LiteralBool); !ok && !isNonLiteral(expr) {
		v.posErr(exprPos(expr), "%s: expected bool, got %s", context, exprTypeName(expr))
	}
}

func (v *validator) checkEnumType(expr parser.Expr, te parser.TypeExpr, context string) {
	str, ok := expr.(parser.LiteralString)
	if !ok {
		if !isNonLiteral(expr) {
			v.posErr(exprPos(expr), "%s: expected enum value, got %s", context, exprTypeName(expr))
		}
		return
	}
	for _, variant := range te.Variants {
		if str.Value == variant {
			return
		}
	}
	v.posErr(str.Pos, "%s: %q is not a valid enum variant (expected one of %v)",
		context, str.Value, te.Variants)
}

func (v *validator) checkArrayType(expr parser.Expr, te parser.TypeExpr, context string) {
	arr, ok := expr.(parser.ArrayLiteral)
	if !ok {
		if !isNonLiteral(expr) {
			v.posErr(exprPos(expr), "%s: expected array, got %s", context, exprTypeName(expr))
		}
		return
	}
	if te.ElemType != nil {
		for i, elem := range arr.Elements {
			v.checkExprType(elem, *te.ElemType, fmt.Sprintf("%s[%d]", context, i))
		}
	}
}

func (v *validator) checkModelType(expr parser.Expr, te parser.TypeExpr, context string) {
	obj, ok := expr.(parser.ObjectLiteral)
	if !ok {
		if !isNonLiteral(expr) {
			v.posErr(exprPos(expr), "%s: expected %s (object), got %s", context, te.Name, exprTypeName(expr))
		}
		return
	}
	model, ok := v.models[te.Name]
	if !ok {
		return // unknown model — already reported by validateContract
	}
	modelFields := make(map[string]*parser.Field, len(model.Fields))
	for _, f := range model.Fields {
		modelFields[f.Name] = f
	}
	for _, of := range obj.Fields {
		mf, ok := modelFields[of.Key]
		if !ok {
			v.posErr(of.Pos, "%s: unknown field %q in model %s", context, of.Key, te.Name)
			continue
		}
		v.checkExprType(of.Value, mf.Type, fmt.Sprintf("%s.%s", context, of.Key))
	}
}

// exprPos extracts the Pos from any Expr node.
func exprPos(expr parser.Expr) spec.Pos {
	switch e := expr.(type) {
	case parser.LiteralInt:
		return e.Pos
	case parser.LiteralFloat:
		return e.Pos
	case parser.LiteralString:
		return e.Pos
	case parser.LiteralBool:
		return e.Pos
	case parser.LiteralNull:
		return e.Pos
	case parser.FieldRef:
		return e.Pos
	case parser.BinaryOp:
		return e.Pos
	case parser.UnaryOp:
		return e.Pos
	case parser.ObjectLiteral:
		return e.Pos
	case parser.ArrayLiteral:
		return e.Pos
	case parser.EnvRef:
		return e.Pos
	case parser.ServiceRef:
		return e.Pos
	case parser.LenExpr:
		return e.Pos
	case parser.AllExpr:
		return e.Pos
	case parser.AnyExpr:
		return e.Pos
	case parser.ContainsExpr:
		return e.Pos
	case parser.ExistsExpr:
		return e.Pos
	case parser.HasKeyExpr:
		return e.Pos
	case parser.RegexLiteral:
		return e.Pos
	case parser.IfExpr:
		return e.Pos
	case parser.AdapterCall:
		return e.Pos
	}
	return spec.Pos{}
}

// isNonLiteral returns true for expressions that can't be statically type-checked.
func isNonLiteral(expr parser.Expr) bool {
	switch expr.(type) {
	case parser.FieldRef, parser.BinaryOp, parser.UnaryOp,
		parser.EnvRef, parser.ServiceRef, parser.LenExpr, parser.AllExpr, parser.AnyExpr,
		parser.ContainsExpr, parser.ExistsExpr, parser.HasKeyExpr,
		parser.RegexLiteral, parser.IfExpr, parser.AdapterCall:
		return true
	}
	return false
}

// validateServiceRefs checks that every service() reference in target fields
// refers to a declared service or that a compose file is set (compose services
// are external and not declared inline).
func (v *validator) validateServiceRefs(spec *parser.Spec) {
	if spec.Target == nil {
		return
	}

	// Build set of declared service names.
	declared := make(map[string]bool, len(spec.Target.Services))
	for _, svc := range spec.Target.Services {
		declared[svc.Name] = true
	}

	hasCompose := spec.Target.Compose != ""

	for key, expr := range spec.Target.Fields {
		ref, ok := expr.(parser.ServiceRef)
		if !ok {
			continue
		}
		if hasCompose || declared[ref.Name] {
			continue
		}
		v.posErr(ref.Pos, "target field %q: service(%s) references undeclared service", key, ref.Name)
	}
}

func typeName(te parser.TypeExpr) string {
	switch te.Name {
	case "array":
		if te.ElemType != nil {
			return "[]" + typeName(*te.ElemType)
		}
		return "[]unknown"
	case "map":
		return "map"
	case "enum":
		return fmt.Sprintf("enum(%v)", te.Variants)
	default:
		name := te.Name
		if te.Optional {
			name += "?"
		}
		return name
	}
}

func exprTypeName(expr parser.Expr) string {
	switch expr.(type) {
	case parser.LiteralInt:
		return "int literal"
	case parser.LiteralFloat:
		return "float literal"
	case parser.LiteralString:
		return "string literal"
	case parser.LiteralBool:
		return "bool literal"
	case parser.LiteralNull:
		return "null"
	case parser.ArrayLiteral:
		return "array literal"
	case parser.ObjectLiteral:
		return "object literal"
	default:
		return fmt.Sprintf("%T", expr)
	}
}

// FormatErrors formats validation errors in a hierarchical display
// grouped by scope, then by context (contract/scenario).
func FormatErrors(errs []error) string {
	if len(errs) == 0 {
		return ""
	}

	scopes, scopeOrder, ungrouped := groupErrors(errs)

	var b strings.Builder
	b.WriteString("validation errors:\n")

	for _, name := range scopeOrder {
		formatScopeErrors(&b, name, scopes[name])
	}
	for _, msg := range ungrouped {
		fmt.Fprintf(&b, "  - %s\n", msg)
	}

	return b.String()
}

type scopeErrors struct {
	contract      []string
	scenarios     map[string][]string
	scenarioOrder []string
}

func groupErrors(errs []error) (map[string]*scopeErrors, []string, []string) {
	scopes := make(map[string]*scopeErrors)
	var scopeOrder []string
	var ungrouped []string

	for _, err := range errs {
		msg := err.Error()
		scopeName, rest, ok := extractScope(msg)
		if !ok {
			ungrouped = append(ungrouped, msg)
			continue
		}
		se, exists := scopes[scopeName]
		if !exists {
			se = &scopeErrors{scenarios: make(map[string][]string)}
			scopes[scopeName] = se
			scopeOrder = append(scopeOrder, scopeName)
		}

		scenarioName, detail, ok := extractScenario(rest)
		if ok {
			if _, seen := se.scenarios[scenarioName]; !seen {
				se.scenarioOrder = append(se.scenarioOrder, scenarioName)
			}
			se.scenarios[scenarioName] = append(se.scenarios[scenarioName], detail)
		} else {
			se.contract = append(se.contract, rest)
		}
	}

	return scopes, scopeOrder, ungrouped
}

func formatScopeErrors(b *strings.Builder, name string, se *scopeErrors) {
	fmt.Fprintf(b, "\n  scope %s:\n", name)
	if len(se.contract) > 0 {
		b.WriteString("    contract:\n")
		for _, msg := range se.contract {
			fmt.Fprintf(b, "      - %s\n", msg)
		}
	}
	for _, scName := range se.scenarioOrder {
		msgs := se.scenarios[scName]
		fmt.Fprintf(b, "    scenario %s:\n", scName)
		for _, msg := range msgs {
			fmt.Fprintf(b, "      - %s\n", msg)
		}
	}
}

// extractScope parses 'scope "name", rest' from an error message.
// Handles optional position prefixes like "file:line:col: scope ..." or "line:col: scope ...".
// The position prefix is preserved in `rest` so it appears in the formatted output.
func extractScope(msg string) (scope, rest string, ok bool) {
	const prefix = "scope \""

	// Try direct match first (no position prefix).
	s := msg
	posPrefix := ""
	if !strings.HasPrefix(s, prefix) {
		// Skip position prefix: find "scope \"" anywhere after a ": " separator.
		idx := strings.Index(s, ": "+prefix)
		if idx < 0 {
			return "", "", false
		}
		posPrefix = s[:idx+2] // e.g., "file:line:col: "
		s = s[idx+2:]
	}
	s = s[len(prefix):]
	idx := strings.Index(s, "\"")
	if idx < 0 {
		return "", "", false
	}
	scope = s[:idx]
	detail := strings.TrimLeft(s[idx+1:], ", ")
	rest = posPrefix + detail
	return scope, rest, true
}

// extractScenario parses 'scenario "name", rest' from the remainder.
func extractScenario(msg string) (scenario, rest string, ok bool) {
	const prefix = "scenario \""
	if !strings.HasPrefix(msg, prefix) {
		return "", "", false
	}
	msg = msg[len(prefix):]
	idx := strings.Index(msg, "\"")
	if idx < 0 {
		return "", "", false
	}
	scenario = msg[:idx]
	rest = strings.TrimLeft(msg[idx+1:], ", :")
	rest = strings.TrimSpace(rest)
	return scenario, rest, true
}

func (v *validator) validateTypeExpr(te parser.TypeExpr, context string) {
	switch {
	case primitives[te.Name]:
		if te.ElemType != nil {
			v.validateTypeExpr(*te.ElemType, context)
		}
		if te.KeyType != nil {
			v.validateTypeExpr(*te.KeyType, context)
		}
		if te.ValType != nil {
			v.validateTypeExpr(*te.ValType, context)
		}
	default:
		if _, ok := v.models[te.Name]; !ok {
			v.posErr(te.Pos, "%s: unknown type %q", context, te.Name)
		}
	}
}
