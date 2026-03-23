package validator

import (
	"fmt"
	"strings"

	"github.com/bamsammich/speclang/v2/pkg/parser"
)

// primitives are type names that don't need model resolution.
var primitives = map[string]bool{
	"int": true, "string": true, "bool": true,
	"float": true, "bytes": true, "array": true, "map": true,
}

// Validate performs post-parse semantic validation on a spec.
// It returns all errors found (not just the first).
func Validate(spec *parser.Spec) []error {
	v := &validator{
		models: buildModelRegistry(spec.Models),
	}

	for _, scope := range spec.Scopes {
		v.scope = scope.Name
		v.validateContract(scope)
		v.validateScenarios(scope)
	}

	return v.errs
}

type validator struct {
	models map[string]*parser.Model
	errs   []error
	scope  string
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

	for _, a := range sc.Then.Assertions {
		// Skip plugin assertions (e.g., welcome@playwright.visible)
		if a.Plugin != "" {
			continue
		}
		// Skip expression assertions (invariant-style, no Target)
		if a.Target == "" {
			continue
		}

		fieldName := topLevelField(a.Target)
		if _, ok := outputFields[fieldName]; !ok {
			v.errorf("scope %q, scenario %q: then target %q does not match any output field",
				v.scope, sc.Name, a.Target)
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
		if _, ok := step.(*parser.Call); ok {
			hasCalls = true
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
		assigned := make(map[string]bool)
		for _, step := range sc.Given.Steps {
			if a, ok := step.(*parser.Assignment); ok {
				assigned[topLevelField(a.Path)] = true
			}
		}
		for name, f := range inputFields {
			if !f.Type.Optional && !assigned[name] {
				v.errorf("scope %q, scenario %q: missing required field %q",
					v.scope, sc.Name, name)
			}
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
	if _, isNull := expr.(parser.LiteralNull); isNull {
		if !te.Optional {
			v.errorf("%s: null is not valid for non-optional type %s", context, typeName(te))
		}
		return
	}

	switch te.Name {
	case "int":
		if _, ok := expr.(parser.LiteralInt); !ok {
			if !isNonLiteral(expr) {
				v.errorf("%s: expected int, got %s", context, exprTypeName(expr))
			}
		}
	case "float":
		switch expr.(type) {
		case parser.LiteralFloat, parser.LiteralInt:
			// ok — accept int literals for float fields
		default:
			if !isNonLiteral(expr) {
				v.errorf("%s: expected float, got %s", context, exprTypeName(expr))
			}
		}
	case "string":
		if _, ok := expr.(parser.LiteralString); !ok {
			if !isNonLiteral(expr) {
				v.errorf("%s: expected string, got %s", context, exprTypeName(expr))
			}
		}
	case "bool":
		if _, ok := expr.(parser.LiteralBool); !ok {
			if !isNonLiteral(expr) {
				v.errorf("%s: expected bool, got %s", context, exprTypeName(expr))
			}
		}
	case "array":
		arr, ok := expr.(parser.ArrayLiteral)
		if !ok {
			if !isNonLiteral(expr) {
				v.errorf("%s: expected array, got %s", context, exprTypeName(expr))
			}
			return
		}
		if te.ElemType != nil {
			for i, elem := range arr.Elements {
				v.checkExprType(elem, *te.ElemType,
					fmt.Sprintf("%s[%d]", context, i))
			}
		}
	default:
		// Model type — expect ObjectLiteral
		obj, ok := expr.(parser.ObjectLiteral)
		if !ok {
			if !isNonLiteral(expr) {
				v.errorf("%s: expected %s (object), got %s", context, te.Name, exprTypeName(expr))
			}
			return
		}
		// Validate object fields against model
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
				v.errorf("%s: unknown field %q in model %s", context, of.Key, te.Name)
				continue
			}
			v.checkExprType(of.Value, mf.Type,
				fmt.Sprintf("%s.%s", context, of.Key))
		}
	}
}

// isNonLiteral returns true for expressions that can't be statically type-checked.
func isNonLiteral(expr parser.Expr) bool {
	switch expr.(type) {
	case parser.FieldRef, parser.BinaryOp, parser.UnaryOp,
		parser.EnvRef, parser.LenExpr, parser.RegexLiteral:
		return true
	}
	return false
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

	type scopeErrors struct {
		contract      []string
		scenarios     map[string][]string
		scenarioOrder []string
	}
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

	var b strings.Builder
	b.WriteString("validation errors:\n")

	for _, name := range scopeOrder {
		se := scopes[name]
		fmt.Fprintf(&b, "\n  scope %s:\n", name)
		if len(se.contract) > 0 {
			b.WriteString("    contract:\n")
			for _, msg := range se.contract {
				fmt.Fprintf(&b, "      - %s\n", msg)
			}
		}
		for _, scName := range se.scenarioOrder {
			msgs := se.scenarios[scName]
			fmt.Fprintf(&b, "    scenario %s:\n", scName)
			for _, msg := range msgs {
				fmt.Fprintf(&b, "      - %s\n", msg)
			}
		}
	}

	for _, msg := range ungrouped {
		fmt.Fprintf(&b, "  - %s\n", msg)
	}

	return b.String()
}

// extractScope parses 'scope "name", rest' from an error message.
func extractScope(msg string) (scope, rest string, ok bool) {
	const prefix = "scope \""
	if !strings.HasPrefix(msg, prefix) {
		return "", "", false
	}
	msg = msg[len(prefix):]
	idx := strings.Index(msg, "\"")
	if idx < 0 {
		return "", "", false
	}
	scope = msg[:idx]
	rest = strings.TrimLeft(msg[idx+1:], ", ")
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
			v.errorf("%s: unknown type %q", context, te.Name)
		}
	}
}
