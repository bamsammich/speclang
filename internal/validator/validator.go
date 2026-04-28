package validator

import (
	"fmt"
	"strings"

	"github.com/bamsammich/speclang/v4/internal/parser"
	"github.com/bamsammich/speclang/v4/pkg/spec"
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
		enums:    buildEnumRegistry(s.Enums),
		config:   s.Config,
		registry: registry,
	}

	v.validateServiceRefs(s)
	v.validateModels(s.Models)

	// Validate top-level contracts
	for _, c := range s.Contracts {
		v.validateContractV4(c)
	}

	// Validate scoped contracts
	for _, scope := range s.Scopes {
		for _, c := range scope.Contracts {
			v.scope = scope.Name
			v.validateContractV4(c)
		}
	}

	return v.errs
}

type validator struct {
	models   map[string]*parser.Model
	enums    map[string]*parser.NamedEnum
	config   map[string]parser.Expr
	registry *spec.Registry
	scope    string
	errs     []error
}

func buildEnumRegistry(enums []*parser.NamedEnum) map[string]*parser.NamedEnum {
	reg := make(map[string]*parser.NamedEnum, len(enums))
	for _, e := range enums {
		reg[e.Name] = e
	}
	return reg
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

// validateModels validates model field When expressions.
func (v *validator) validateModels(models []*parser.Model) {
	for _, m := range models {
		siblings := buildFieldMap(m.Fields)
		for _, f := range m.Fields {
			if f.When != nil {
				v.validateWhenExpr(f.When, siblings, fmt.Sprintf("model %q, field %q", m.Name, f.Name))
			}
		}
		v.checkCircularWhen(m)
	}
}

// checkCircularWhen detects circular when dependencies among fields in a model.
// Field A depends on Field B if A's When expression references B, and B also has
// a When expression that references A (directly or transitively).
func (v *validator) checkCircularWhen(m *parser.Model) {
	// Build dependency graph: field name -> set of field names referenced in When expr
	deps := make(map[string]map[string]bool)
	for _, f := range m.Fields {
		if f.When == nil {
			continue
		}
		refs := collectFieldRefs(f.When)
		deps[f.Name] = refs
	}

	// Detect cycles via DFS
	visited := make(map[string]bool)
	inStack := make(map[string]bool)

	var visit func(name string) bool
	visit = func(name string) bool {
		if inStack[name] {
			return true // cycle
		}
		if visited[name] {
			return false
		}
		visited[name] = true
		inStack[name] = true
		for dep := range deps[name] {
			if deps[dep] != nil && visit(dep) {
				return true
			}
		}
		inStack[name] = false
		return false
	}

	for name := range deps {
		if visit(name) {
			v.posErr(spec.Pos{}, "model %q: circular when dependency involving field %q", m.Name, name)
			return // report once per model
		}
		// Reset for next root
		visited = make(map[string]bool)
		inStack = make(map[string]bool)
	}
}

// checkCircularWhenContract detects circular when dependencies among contract input fields.
// It mirrors checkCircularWhen for models, applied to contract.Fields.
func (v *validator) checkCircularWhenContract(c *parser.Contract, contractCtx string) {
	deps := make(map[string]map[string]bool)
	for _, f := range c.Fields {
		if f.When == nil {
			continue
		}
		refs := collectFieldRefs(f.When)
		deps[f.Name] = refs
	}

	visited := make(map[string]bool)
	inStack := make(map[string]bool)

	var visit func(name string) bool
	visit = func(name string) bool {
		if inStack[name] {
			return true
		}
		if visited[name] {
			return false
		}
		visited[name] = true
		inStack[name] = true
		for dep := range deps[name] {
			if deps[dep] != nil && visit(dep) {
				return true
			}
		}
		inStack[name] = false
		return false
	}

	for name := range deps {
		if visit(name) {
			v.posErr(c.Pos, "%s: circular when dependency involving field %q", contractCtx, name)
			return // report once per contract
		}
		visited = make(map[string]bool)
		inStack = make(map[string]bool)
	}
}

// collectFieldRefs extracts bare field names (top-level segment) from an expression.
func collectFieldRefs(expr parser.Expr) map[string]bool {
	refs := make(map[string]bool)
	var walk func(e parser.Expr)
	walk = func(e parser.Expr) {
		if e == nil {
			return
		}
		switch v := e.(type) {
		case parser.FieldRef:
			refs[topLevelField(v.Path)] = true
		case parser.BinaryOp:
			walk(v.Left)
			walk(v.Right)
		case parser.UnaryOp:
			walk(v.Operand)
		case parser.LenExpr:
			walk(v.Arg)
		case parser.ContainsExpr:
			walk(v.Haystack)
			walk(v.Needle)
		case parser.ExistsExpr:
			walk(v.Arg)
		case parser.HasKeyExpr:
			walk(v.Arg)
			walk(v.Key)
		case parser.AllExpr:
			walk(v.Array)
			walk(v.Predicate)
		case parser.AnyExpr:
			walk(v.Array)
			walk(v.Predicate)
		case parser.IfExpr:
			walk(v.Condition)
			walk(v.Then)
			walk(v.Else)
		}
	}
	walk(expr)
	return refs
}

// validateWhenExpr validates that a field's When expression only references sibling fields.
func (v *validator) validateWhenExpr(expr parser.Expr, siblings map[string]*parser.Field, context string) {
	refs := collectFieldRefs(expr)
	for name := range refs {
		if _, ok := siblings[name]; !ok {
			v.posErr(exprPos(expr), "%s: when expression references unknown field %q", context, name)
		}
	}
}

// validateContractV4 validates a v4 contract: its fields, return type, scenarios, and invariants.
func (v *validator) validateContractV4(c *parser.Contract) {
	contractCtx := fmt.Sprintf("contract %q", c.Name)
	if v.scope != "" {
		contractCtx = fmt.Sprintf("scope %q, contract %q", v.scope, c.Name)
	}

	// Validate inheritance
	if c.Inherits != "" {
		inheritedModel, ok := v.models[c.Inherits]
		if !ok {
			v.posErr(c.Pos, "%s: inherits unknown model %q", contractCtx, c.Inherits)
		} else {
			// Validate constraint expressions reference inherited model fields
			inheritedFields := buildFieldMap(inheritedModel.Fields)
			for _, expr := range c.Constraints {
				v.validateExprFieldRefs(expr, inheritedFields, fmt.Sprintf("%s, constrain block", contractCtx))
			}
		}
	}

	// Validate input fields
	for _, f := range c.Fields {
		v.validateTypeExpr(f.Type, fmt.Sprintf("%s, field %q", contractCtx, f.Name))
		if f.When != nil {
			siblings := buildFieldMap(c.Fields)
			v.validateWhenExpr(f.When, siblings, fmt.Sprintf("%s, field %q", contractCtx, f.Name))
		}
	}
	// Detect circular when dependencies among contract input fields.
	v.checkCircularWhenContract(c, contractCtx)


	// Validate return type (if named)
	if c.ReturnType.Name != "" && c.ReturnType.Name != "any" {
		v.validateTypeExpr(c.ReturnType, fmt.Sprintf("%s, return type", contractCtx))
	}

	// Validate constraint expressions for enum/config refs
	for _, expr := range c.Constraints {
		v.validateExprRefs(expr, fmt.Sprintf("%s, constrain block", contractCtx))
	}

	// Build input and return-model field maps for field-resolution validation.
	inputFields := buildFieldMap(c.Fields)
	var returnFields map[string]*parser.Field
	if c.ReturnType.Name != "" && !primitives[c.ReturnType.Name] {
		if model, ok := v.models[c.ReturnType.Name]; ok {
			returnFields = buildFieldMap(model.Fields)
		}
	}

	// Validate invariants
	for _, inv := range c.Invariants {
		invCtx := fmt.Sprintf("%s, invariant %q", contractCtx, inv.Name)
		if inv.When != nil {
			v.validateExprRefs(inv.When, invCtx)
			v.validateBareOutputRefs(inv.When, inputFields, returnFields, invCtx)
		}
		for _, a := range inv.Assertions {
			if a.Expr != nil {
				v.validateExprRefs(a.Expr, invCtx)
				v.validateBareOutputRefs(a.Expr, inputFields, returnFields, invCtx)
			}
		}
	}

	// Validate scenarios
	for _, sc := range c.Scenarios {
		v.validateGivenBlock(sc, inputFields)
		v.validateScenarioThenBlock(sc, c)

		// Validate expression refs in when/then blocks
		scCtx := fmt.Sprintf("%s, scenario %q", contractCtx, sc.Name)
		if sc.When != nil {
			for _, pred := range sc.When.Predicates {
				v.validateExprRefs(pred, scCtx)
				v.validateBareOutputRefs(pred, inputFields, returnFields, scCtx)
			}
		}
		if sc.Then != nil {
			for _, a := range sc.Then.Assertions {
				if a.Expr != nil {
					v.validateExprRefs(a.Expr, scCtx)
					v.validateBareOutputRefs(a.Expr, inputFields, returnFields, scCtx)
				}
			}
		}
	}
}

// validateExprFieldRefs validates that FieldRef nodes in an expression reference
// known fields from the given field map (ignoring prefixed refs like output., input., config.).
func (v *validator) validateExprFieldRefs(expr parser.Expr, fields map[string]*parser.Field, context string) {
	if expr == nil {
		return
	}
	switch e := expr.(type) {
	case parser.FieldRef:
		top := topLevelField(e.Path)
		// Skip prefixed refs and enum refs
		if top == "output" || top == "input" || top == "config" || top == "error" {
			return
		}
		if _, ok := v.enums[top]; ok {
			return
		}
		if _, ok := fields[top]; !ok {
			v.posErr(e.Pos, "%s: references unknown field %q", context, top)
		}
	case parser.BinaryOp:
		v.validateExprFieldRefs(e.Left, fields, context)
		v.validateExprFieldRefs(e.Right, fields, context)
	case parser.UnaryOp:
		v.validateExprFieldRefs(e.Operand, fields, context)
	case parser.LenExpr:
		v.validateExprFieldRefs(e.Arg, fields, context)
	case parser.ContainsExpr:
		v.validateExprFieldRefs(e.Haystack, fields, context)
		v.validateExprFieldRefs(e.Needle, fields, context)
	case parser.AllExpr:
		v.validateExprFieldRefs(e.Array, fields, context)
		v.validateExprFieldRefs(e.Predicate, fields, context)
	case parser.AnyExpr:
		v.validateExprFieldRefs(e.Array, fields, context)
		v.validateExprFieldRefs(e.Predicate, fields, context)
	case parser.IfExpr:
		v.validateExprFieldRefs(e.Condition, fields, context)
		v.validateExprFieldRefs(e.Then, fields, context)
		v.validateExprFieldRefs(e.Else, fields, context)
	case parser.ExistsExpr:
		v.validateExprFieldRefs(e.Arg, fields, context)
	case parser.HasKeyExpr:
		v.validateExprFieldRefs(e.Arg, fields, context)
		v.validateExprFieldRefs(e.Key, fields, context)
	}
}

func (v *validator) validateScenarioThenBlock(sc *parser.Scenario, c *parser.Contract) {
	if sc.Then == nil {
		return
	}

	// Build output field map from return type model (if the return type is a named model).
	var outputFields map[string]*parser.Field
	if c.ReturnType.Name != "" && !primitives[c.ReturnType.Name] {
		if model, ok := v.models[c.ReturnType.Name]; ok {
			outputFields = buildFieldMap(model.Fields)
		}
	}

	for _, a := range sc.Then.Assertions {
		// Expression assertions (Expr is set, Target is empty) — validated by expression
		if a.Expr != nil && a.Target == "" {
			continue
		}
		// Skip plugin assertions
		if a.Plugin != "" {
			continue
		}
		// Skip expression assertions (no Target)
		if a.Target == "" {
			continue
		}
		// Skip error pseudo-field when not declared as an output field
		if a.Target == "error" && (outputFields == nil || outputFields["error"] == nil) {
			continue
		}
		// Validate target against return model fields (if available)
		if outputFields != nil {
			topField := topLevelField(a.Target)
			if _, ok := outputFields[topField]; !ok {
				v.posErr(spec.Pos{}, "scope %q, scenario %q: then target %q does not match any output field",
					v.scope, sc.Name, a.Target)
			}
		}
	}
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
		// Fields with a When condition are conditionally present — they are never
		// required in a given block regardless of their type's optional flag.
		if !f.Type.Optional && f.When == nil && !assigned[name] {
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

// validateServiceRefs validates service declarations and service() references.
// For v4: checks compose/build exclusion and validates service() refs in adapter configs.
// For v2 compat: also checks spec.Target fields.
func (v *validator) validateServiceRefs(spec *parser.Spec) {
	// Build set of declared v4 service names; validate compose/build exclusion.
	declared := make(map[string]bool, len(spec.Services))
	hasCompose := false
	for _, svc := range spec.Services {
		declared[svc.Name] = true
		if svc.Compose != "" && (svc.Build != "" || svc.Image != "") {
			v.errorf("service %q: compose is mutually exclusive with build/image", svc.Name)
		}
		if svc.Compose != "" {
			hasCompose = true
		}
	}

	// Validate service() refs in v4 adapter config values.
	for adapterName, fields := range spec.AdapterConfigs {
		for fieldName, expr := range fields {
			ref, ok := expr.(parser.ServiceRef)
			if !ok {
				continue
			}
			if hasCompose || declared[ref.Name] {
				continue
			}
			v.errorf("adapter %q field %q: service(%s) references undeclared service", adapterName, fieldName, ref.Name)
		}
	}

	// v2 compat: validate service() refs in spec.Target fields.
	if spec.Target == nil {
		return
	}
	declaredTarget := make(map[string]bool, len(spec.Target.Services))
	for _, svc := range spec.Target.Services {
		declaredTarget[svc.Name] = true
	}
	hasComposeTarget := spec.Target.Compose != ""
	for key, expr := range spec.Target.Fields {
		ref, ok := expr.(parser.ServiceRef)
		if !ok {
			continue
		}
		if hasComposeTarget || declaredTarget[ref.Name] {
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

// validateEnumRef checks that a FieldRef like "EnumName.variant" references a valid
// named enum and a valid variant within that enum.
func (v *validator) validateEnumRef(ref parser.FieldRef, context string) {
	dotIdx := strings.Index(ref.Path, ".")
	if dotIdx < 0 {
		return
	}
	enumName := ref.Path[:dotIdx]
	ne, ok := v.enums[enumName]
	if !ok {
		return // not an enum ref
	}
	variant := ref.Path[dotIdx+1:]
	for _, ev := range ne.Variants {
		if ev == variant {
			return
		}
	}
	v.posErr(ref.Pos, "%s: %q is not a variant of enum %s (valid: %v)", context, variant, enumName, ne.Variants)
}

// validateConfigRef checks that a FieldRef like "config.key" references a known config key.
func (v *validator) validateConfigRef(ref parser.FieldRef, context string) {
	dotIdx := strings.Index(ref.Path, ".")
	if dotIdx < 0 {
		return
	}
	prefix := ref.Path[:dotIdx]
	if prefix != "config" {
		return
	}
	key := ref.Path[dotIdx+1:]
	if v.config == nil {
		v.posErr(ref.Pos, "%s: config.%s references config but no config block is declared", context, key)
		return
	}
	if _, ok := v.config[key]; !ok {
		v.posErr(ref.Pos, "%s: config.%s references unknown config key %q", context, key, key)
	}
}

// validateBareOutputRefs walks expr and emits an error for any bare FieldRef
// whose first path segment matches a return-model field but NOT an input field.
//
// v4 field resolution rule (plan §3): bare names resolve to contract input fields;
// return-model fields must be prefixed with "output.".  When a name exists in both,
// input wins silently (no error) — this is documented in the error message path.
func (v *validator) validateBareOutputRefs(
	expr parser.Expr,
	inputFields map[string]*parser.Field,
	returnFields map[string]*parser.Field,
	context string,
) {
	if expr == nil || returnFields == nil {
		return
	}
	var walk func(e parser.Expr)
	walk = func(e parser.Expr) {
		if e == nil {
			return
		}
		switch node := e.(type) {
		case parser.FieldRef:
			top := topLevelField(node.Path)
			// Skip already-prefixed refs and reserved names.
			if top == "output" || top == "input" || top == "config" || top == "error" {
				return
			}
			// Skip enum names.
			if _, isEnum := v.enums[top]; isEnum {
				return
			}
			// Error only when the name is exclusively in the return model.
			_, inInput := inputFields[top]
			_, inReturn := returnFields[top]
			if inReturn && !inInput {
				v.posErr(node.Pos, "%s: bare reference %q refers to an output field; use \"output.%s\"",
					context, top, top)
			}
		case parser.BinaryOp:
			walk(node.Left)
			walk(node.Right)
		case parser.UnaryOp:
			walk(node.Operand)
		case parser.LenExpr:
			walk(node.Arg)
		case parser.ContainsExpr:
			walk(node.Haystack)
			walk(node.Needle)
		case parser.AllExpr:
			walk(node.Array)
			walk(node.Predicate)
		case parser.AnyExpr:
			walk(node.Array)
			walk(node.Predicate)
		case parser.IfExpr:
			walk(node.Condition)
			walk(node.Then)
			walk(node.Else)
		case parser.ExistsExpr:
			walk(node.Arg)
		case parser.HasKeyExpr:
			walk(node.Arg)
			walk(node.Key)
		case parser.ArrayLiteral:
			for _, elem := range node.Elements {
				walk(elem)
			}
		case parser.ObjectLiteral:
			for _, f := range node.Fields {
				walk(f.Value)
			}
		}
	}
	walk(expr)
}

// validateExprRefs validates enum and config references in an expression tree.
func (v *validator) validateExprRefs(expr parser.Expr, context string) {
	if expr == nil {
		return
	}
	switch e := expr.(type) {
	case parser.FieldRef:
		v.validateEnumRef(e, context)
		v.validateConfigRef(e, context)
	case parser.BinaryOp:
		v.validateExprRefs(e.Left, context)
		v.validateExprRefs(e.Right, context)
	case parser.UnaryOp:
		v.validateExprRefs(e.Operand, context)
	case parser.LenExpr:
		v.validateExprRefs(e.Arg, context)
	case parser.ContainsExpr:
		v.validateExprRefs(e.Haystack, context)
		v.validateExprRefs(e.Needle, context)
	case parser.AllExpr:
		v.validateExprRefs(e.Array, context)
		v.validateExprRefs(e.Predicate, context)
	case parser.AnyExpr:
		v.validateExprRefs(e.Array, context)
		v.validateExprRefs(e.Predicate, context)
	case parser.IfExpr:
		v.validateExprRefs(e.Condition, context)
		v.validateExprRefs(e.Then, context)
		v.validateExprRefs(e.Else, context)
	case parser.ExistsExpr:
		v.validateExprRefs(e.Arg, context)
	case parser.HasKeyExpr:
		v.validateExprRefs(e.Arg, context)
		v.validateExprRefs(e.Key, context)
	case parser.ArrayLiteral:
		for _, elem := range e.Elements {
			v.validateExprRefs(elem, context)
		}
	case parser.ObjectLiteral:
		for _, f := range e.Fields {
			v.validateExprRefs(f.Value, context)
		}
	}
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
			if _, ok := v.enums[te.Name]; !ok {
				v.posErr(te.Pos, "%s: unknown type %q", context, te.Name)
			}
		}
	}
}
