package migrate

import (
	"fmt"
	"strings"

	"github.com/bamsammich/speclang/v4/internal/v3parser"
	"github.com/bamsammich/speclang/v4/pkg/spec"
)

// MigrateV3File reads a v3 spec file and returns the migrated v4 text.
// Unlike the v2→v3 path, v3 specs are single files (includes are already
// resolved by the v3 parser), so this returns a single string.
func MigrateV3File(src string) (string, error) {
	s, err := v3parser.Parse(src)
	if err != nil {
		return "", fmt.Errorf("parsing v3 spec: %w", err)
	}
	return MigrateV3Spec(s, src)
}

// MigrateV3Spec transforms a v3 AST into formatted v4 spec text.
// The original source text is used for include extraction (includes are stripped
// from the v3 parser's view and must be re-emitted at the v4 top level).
func MigrateV3Spec(s *v3parser.Spec, originalSrc string) (string, error) {
	w := &v4Writer{}

	// Extract include directives from original source — v3 parser resolves them
	// but we need them preserved as top-level include declarations in v4.
	includes := extractIncludes(originalSrc)

	// Emit top-level includes first.
	for _, inc := range includes {
		w.line("include %q", inc)
	}
	if len(includes) > 0 {
		w.blank()
	}

	// Adapter configs (http { ... }, playwright { ... }).
	for _, name := range sortedKeys(s.AdapterConfigs) {
		cfg := s.AdapterConfigs[name]
		w.open("%s {", name)
		for _, k := range sortedKeys(cfg) {
			w.line("%s: %s", k, v4FormatExpr(cfg[k]))
		}
		w.close()
		w.blank()
	}

	// Services block (from spec-level or v2-compat target).
	services := s.Services
	if len(services) == 0 && s.Target != nil {
		services = s.Target.Services
	}
	if len(services) > 0 {
		w.emitV4Services(services)
		w.blank()
	}

	// Models.
	for _, m := range s.Models {
		w.emitV4Model(m)
		w.blank()
	}

	// Top-level actions.
	for _, a := range s.Actions {
		w.emitV4ActionDef(a)
		w.blank()
	}

	// Synthesized output models from scopes (if scope has anonymous output fields).
	synthesized := collectSynthesizedModels(s.Scopes)
	for _, m := range synthesized {
		w.emitV4Model(m)
		w.blank()
	}

	// Scopes → v4 contracts (wrapped in scope if before/after present).
	for i, sc := range s.Scopes {
		if i > 0 {
			w.blank()
		}
		if err := w.emitV4Scope(sc, synthesized); err != nil {
			return "", fmt.Errorf("emitting scope %q: %w", sc.Name, err)
		}
	}

	return w.String(), nil
}

// extractIncludes returns include paths from v3 source text (same pattern as
// v2→v3 migration — includes in v3 still use the same directive syntax).
func extractIncludes(src string) []string {
	var includes []string
	for _, m := range includeRe.FindAllStringSubmatch(src, -1) {
		includes = append(includes, m[1])
	}
	return includes
}

// collectSynthesizedModels returns models that need to be synthesized for scopes
// whose output block has multiple anonymous fields (not a single model reference).
func collectSynthesizedModels(scopes []*v3parser.Scope) []*spec.Model {
	var out []*spec.Model
	for _, sc := range scopes {
		if sc.Contract == nil {
			continue
		}
		m := synthesizeOutputModel(sc)
		if m != nil {
			out = append(out, m)
		}
	}
	return out
}

// synthesizeOutputModel returns a new model for the scope's output fields if they
// cannot be expressed as a single model reference. Returns nil if no model needed
// (i.e., the output is a single model-type field, meaning the output IS that model).
func synthesizeOutputModel(sc *v3parser.Scope) *spec.Model {
	if sc.Contract == nil || len(sc.Contract.Output) == 0 {
		return nil
	}
	// If the output is exactly one field whose type is a named model (not a primitive),
	// we treat the output type AS that field's type — no new model needed.
	if len(sc.Contract.Output) == 1 {
		f := sc.Contract.Output[0]
		if isModelRef(f.Type) {
			return nil
		}
	}
	// Multiple output fields, or a single primitive field → synthesize a model.
	name := contractOutputModelName(sc.Name)
	return &spec.Model{
		Name:   name,
		Fields: sc.Contract.Output,
	}
}

// contractOutputModelName derives the v4 output model name from a v3 scope name.
// e.g. "transfer" → "TransferResult", "parse_valid" → "ParseValidResult"
func contractOutputModelName(scopeName string) string {
	parts := strings.Split(scopeName, "_")
	var b strings.Builder
	for _, p := range parts {
		if len(p) > 0 {
			b.WriteString(strings.ToUpper(p[:1]) + p[1:])
		}
	}
	b.WriteString("Result")
	return b.String()
}

// outputTypeName returns the v4 return type name for a v3 scope's output.
func outputTypeName(sc *v3parser.Scope, synthesized []*spec.Model) string {
	if sc.Contract == nil || len(sc.Contract.Output) == 0 {
		return "any"
	}
	// Single field whose type is a named model reference → use that type directly.
	if len(sc.Contract.Output) == 1 {
		f := sc.Contract.Output[0]
		if isModelRef(f.Type) {
			return formatV4TypeExpr(f.Type)
		}
	}
	// Synthesized model case.
	name := contractOutputModelName(sc.Name)
	for _, m := range synthesized {
		if m.Name == name {
			return name
		}
	}
	return "any"
}

// outputFieldNames returns the set of field names in the v3 output model.
// These are bare names that must be prefixed with "output." in v4 assertions.
func outputFieldNames(sc *v3parser.Scope) map[string]bool {
	names := make(map[string]bool)
	if sc.Contract == nil {
		return names
	}
	for _, f := range sc.Contract.Output {
		names[f.Name] = true
	}
	// Also collect fields from the output model type when the output is a single
	// model ref — in that case assertions already use output.field style in v3,
	// but we collect the top-level name so we don't double-prefix.
	return names
}

// isModelRef returns true if the type expression refers to a named model
// (i.e., not a primitive, array, map, or enum type).
func isModelRef(t spec.TypeExpr) bool {
	switch t.Name {
	case "int", "float", "string", "bool", "bytes", "any", "array", "map", "enum":
		return false
	default:
		return t.Name != ""
	}
}

// findScopeAction finds the action definition in the scope that matches the
// contract's action reference name. Returns nil if not found.
func findScopeAction(sc *v3parser.Scope) *spec.ActionDef {
	if sc.Contract == nil || sc.Contract.Action == "" {
		return nil
	}
	for _, a := range sc.Actions {
		if a.Name == sc.Contract.Action {
			return a
		}
	}
	return nil
}

// contractName derives the v4 contract name from a v3 scope name.
// e.g. "transfer" → "Transfer", "parse_valid" → "ParseValid"
func contractName(scopeName string) string {
	parts := strings.Split(scopeName, "_")
	var b strings.Builder
	for _, p := range parts {
		if len(p) > 0 {
			b.WriteString(strings.ToUpper(p[:1]) + p[1:])
		}
	}
	return b.String()
}

// --- v4Writer ---

type v4Writer struct {
	buf    strings.Builder
	indent int
}

func (w *v4Writer) line(format string, args ...any) {
	w.buf.WriteString(strings.Repeat("  ", w.indent))
	fmt.Fprintf(&w.buf, format, args...)
	w.buf.WriteByte('\n')
}

func (w *v4Writer) open(format string, args ...any) {
	w.line(format, args...)
	w.indent++
}

func (w *v4Writer) close() {
	w.indent--
	w.line("}")
}

func (w *v4Writer) blank() {
	w.buf.WriteByte('\n')
}

func (w *v4Writer) String() string {
	return w.buf.String()
}

func (w *v4Writer) emitV4Services(svcs []*spec.Service) {
	w.open("services {")
	for _, svc := range svcs {
		w.open("%s {", svc.Name)
		if svc.Build != "" {
			w.line("build: %q", svc.Build)
		}
		if svc.Compose != "" {
			w.line("compose: %q", svc.Compose)
		}
		if svc.Image != "" {
			w.line("image: %q", svc.Image)
		}
		if svc.Port != 0 {
			w.line("port: %d", svc.Port)
		}
		if svc.Health != "" {
			w.line("health: %q", svc.Health)
		}
		if len(svc.Env) > 0 {
			w.open("env {")
			for _, k := range sortedStringMapKeys(svc.Env) {
				w.line("%s: %q", k, svc.Env[k])
			}
			w.close()
		}
		if len(svc.Volumes) > 0 {
			w.open("volumes {")
			for _, k := range sortedStringMapKeys(svc.Volumes) {
				w.line("%s: %q", k, svc.Volumes[k])
			}
			w.close()
		}
		w.close()
	}
	w.close()
}

func (w *v4Writer) emitV4Model(m *spec.Model) {
	w.open("model %s {", m.Name)
	for _, f := range m.Fields {
		w.emitV4Field(f)
	}
	w.close()
}

func (w *v4Writer) emitV4Field(f *spec.Field) {
	typeStr := formatV4TypeExpr(f.Type)
	if f.Constraint != nil {
		w.line("%s: %s { %s }", f.Name, typeStr, v4FormatExpr(f.Constraint))
	} else {
		w.line("%s: %s", f.Name, typeStr)
	}
}

func (w *v4Writer) emitV4ActionDef(a *spec.ActionDef) {
	params := make([]string, len(a.Params))
	for i, p := range a.Params {
		params[i] = p.Name + ": " + formatV4TypeExpr(p.Type)
	}
	w.open("action %s(%s) {", a.Name, strings.Join(params, ", "))
	for _, step := range a.Body {
		w.emitV4Step(step)
	}
	w.close()
}

func (w *v4Writer) emitV4Step(step spec.GivenStep) {
	switch s := step.(type) {
	case *spec.Assignment:
		w.line("%s: %s", s.Path, v4FormatExpr(s.Value))
	case *spec.LetBinding:
		w.line("let %s = %s", s.Name, v4FormatExpr(s.Value))
	case *spec.ReturnStmt:
		w.line("return %s", v4FormatExpr(s.Value))
	case *spec.AdapterCall:
		w.line("%s.%s(%s)", s.Adapter, s.Method, v4FormatCallArgs(s.Args))
	case *spec.Call:
		if s.Namespace != "" {
			w.line("%s.%s(%s)", s.Namespace, s.Method, v4FormatCallArgs(s.Args))
		} else {
			w.line("%s(%s)", s.Method, v4FormatCallArgs(s.Args))
		}
	}
}

// emitV4Scope converts a v3 scope to v4 syntax.
// If the scope has before/after, it emits a v4 scope wrapper.
// Otherwise, the contract is emitted at top level.
func (w *v4Writer) emitV4Scope(sc *v3parser.Scope, synthesized []*spec.Model) error {
	hasBefore := sc.Before != nil
	hasAfter := sc.After != nil
	hasLifecycle := hasBefore || hasAfter

	outFieldNames := outputFieldNames(sc)
	retTypeName := outputTypeName(sc, synthesized)

	if hasLifecycle {
		w.open("scope %s {", sc.Name)
		if hasBefore {
			w.emitV4Block(sc.Before, "before", outFieldNames)
			w.blank()
		}
		if hasAfter {
			w.emitV4Block(sc.After, "after", outFieldNames)
			w.blank()
		}
	}

	// Emit contract.
	if err := w.emitV4Contract(sc, retTypeName, outFieldNames); err != nil {
		return err
	}

	if hasLifecycle {
		w.close()
	}

	return nil
}

// emitV4Contract emits a v4 contract block from a v3 scope.
func (w *v4Writer) emitV4Contract(sc *v3parser.Scope, retTypeName string, outFieldNames map[string]bool) error {
	name := contractName(sc.Name)
	w.open("contract %s -> %s {", name, retTypeName)

	// Input fields.
	if sc.Contract != nil {
		for _, f := range sc.Contract.Input {
			w.emitV4Field(f)
		}
	}

	// Action block — inline the body from the referenced scope-level action.
	actionDef := findScopeAction(sc)
	if actionDef != nil {
		w.blank()
		w.open("action {")
		for _, step := range actionDef.Body {
			w.emitV4Step(step)
		}
		w.close()
	} else if sc.Contract != nil && sc.Contract.Action != "" {
		// Action is named but not found in scope actions — emit a placeholder.
		w.blank()
		w.open("action {")
		w.line("# TODO: inline action %q", sc.Contract.Action)
		w.close()
	}

	// Invariants (from scope level in v3).
	for _, inv := range sc.Invariants {
		w.blank()
		w.emitV4Invariant(inv, outFieldNames)
	}

	// Scenarios (from scope level in v3).
	for _, scenario := range sc.Scenarios {
		w.blank()
		w.emitV4Scenario(scenario, outFieldNames)
	}

	w.close()
	return nil
}

func (w *v4Writer) emitV4Block(b *spec.Block, kind string, outFieldNames map[string]bool) {
	w.open("%s {", kind)
	for _, step := range b.Steps {
		w.emitV4Step(step)
	}
	for _, pred := range b.Predicates {
		w.line("%s", v4FormatExpr(pred))
	}
	for _, a := range b.Assertions {
		isThen := kind == "then"
		w.line("%s", formatV4Assertion(a, outFieldNames, isThen))
	}
	w.close()
}

func (w *v4Writer) emitV4Invariant(inv *spec.Invariant, outFieldNames map[string]bool) {
	w.open("invariant %s {", inv.Name)
	if inv.When != nil {
		// v3 used "when expr:" inside invariant; v4 just emits the expr directly
		// as it's already an expression form.
		w.line("when %s:", v4FormatExpr(prefixOutputRefs(inv.When, outFieldNames)))
	}
	for _, a := range inv.Assertions {
		w.line("%s", formatV4Assertion(a, outFieldNames, false))
	}
	w.close()
}

func (w *v4Writer) emitV4Scenario(sc *spec.Scenario, outFieldNames map[string]bool) {
	w.open("scenario %s {", sc.Name)
	if sc.Given != nil {
		w.emitV4Block(sc.Given, "given", outFieldNames)
	}
	if sc.When != nil {
		w.emitV4Block(sc.When, "when", outFieldNames)
	}
	if sc.Then != nil {
		w.emitV4Block(sc.Then, "then", outFieldNames)
	}
	w.close()
}

// formatV4Assertion formats an assertion for v4, prefixing output field refs.
//
// isThen controls the prefixing strategy:
//   - isThen=false (invariants): all bare output field refs are prefixed throughout
//     the entire expression tree.
//   - isThen=true (then blocks): in v3, then-block expression assertions have the
//     form "output_field == input_expression". Only the LEFT side of a top-level
//     comparison is prefixed (it is the output result); the right side remains as-is
//     (it is an expression over input fields).
func formatV4Assertion(a *spec.Assertion, outFieldNames map[string]bool, isThen bool) string {
	if a.Expr != nil {
		if isThen {
			return v4FormatExpr(prefixOutputRefsLHSOnly(a.Expr, outFieldNames))
		}
		return v4FormatExpr(prefixOutputRefs(a.Expr, outFieldNames))
	}

	// Plugin assertion: target@plugin.property
	if a.Plugin != "" {
		op := a.Operator
		if op == "" || op == ":" {
			op = "=="
		}
		return fmt.Sprintf("%s.%s(%s) %s %s", a.Plugin, a.Property, a.Target, op, v4FormatExpr(a.Expected))
	}

	// Path assertion: field op value
	op := a.Operator
	if op == "" || op == ":" {
		op = "=="
	}
	target := prefixOutputFieldRef(a.Target, outFieldNames)
	return fmt.Sprintf("%s %s %s", target, op, v4FormatExpr(a.Expected))
}

// prefixOutputRefsLHSOnly handles then-block assertions where the left side of
// a comparison is an output field and the right side is an input expression.
// Only the left branch of a top-level comparison BinaryOp is prefixed.
// Non-comparison expressions are left unchanged (e.g., error == null stays as-is
// after the top-level left FieldRef is prefixed).
func prefixOutputRefsLHSOnly(e spec.Expr, outFieldNames map[string]bool) spec.Expr {
	bop, ok := e.(spec.BinaryOp)
	if !ok {
		return e
	}
	switch bop.Op {
	case "==", "!=", ">", ">=", "<", "<=":
		// Prefix only the left side.
		return spec.BinaryOp{
			Left:  prefixOutputRefs(bop.Left, outFieldNames),
			Right: bop.Right,
			Op:    bop.Op,
		}
	default:
		// Not a comparison — leave as-is.
		return e
	}
}

// prefixOutputFieldRef prefixes a path string with "output." if it starts with
// an output field name (and doesn't already start with "output.").
func prefixOutputFieldRef(path string, outFieldNames map[string]bool) string {
	if strings.HasPrefix(path, "output.") || strings.HasPrefix(path, "input.") {
		return path
	}
	// Check if the first segment of the path is an output field name.
	first := strings.SplitN(path, ".", 2)[0]
	if outFieldNames[first] {
		return "output." + path
	}
	return path
}

// prefixOutputRefs walks an expression tree and prefixes bare output field refs
// with "output." so that v4 assertions are correctly qualified.
func prefixOutputRefs(e spec.Expr, outFieldNames map[string]bool) spec.Expr {
	if e == nil {
		return nil
	}
	switch v := e.(type) {
	case spec.FieldRef:
		return spec.FieldRef{Path: prefixOutputFieldRef(v.Path, outFieldNames)}
	case spec.BinaryOp:
		return spec.BinaryOp{
			Left:  prefixOutputRefs(v.Left, outFieldNames),
			Right: prefixOutputRefs(v.Right, outFieldNames),
			Op:    v.Op,
		}
	case spec.UnaryOp:
		return spec.UnaryOp{
			Operand: prefixOutputRefs(v.Operand, outFieldNames),
			Op:      v.Op,
		}
	case spec.ObjectLiteral:
		fields := make([]*spec.ObjField, len(v.Fields))
		for i, f := range v.Fields {
			fields[i] = &spec.ObjField{Key: f.Key, Value: prefixOutputRefs(f.Value, outFieldNames)}
		}
		return spec.ObjectLiteral{Fields: fields}
	case spec.ArrayLiteral:
		elems := make([]spec.Expr, len(v.Elements))
		for i, elem := range v.Elements {
			elems[i] = prefixOutputRefs(elem, outFieldNames)
		}
		return spec.ArrayLiteral{Elements: elems}
	default:
		return e
	}
}

// normalizeOperatorsInExpr walks an expression tree and translates v3 symbolic
// operators to their v4 word-operator equivalents:
//   - BinaryOp "&&" → "and"
//   - BinaryOp "||" → "or"
//   - UnaryOp  "!"  → "not"
//
// All other node types are returned unchanged. String literal nodes (LiteralString)
// are leaves and are never inspected, so there is no risk of corrupting string values
// that happen to contain &&, ||, or !.
func normalizeOperatorsInExpr(e spec.Expr) spec.Expr {
	if e == nil {
		return nil
	}
	switch v := e.(type) {
	case spec.BinaryOp:
		op := v.Op
		switch op {
		case "&&":
			op = "and"
		case "||":
			op = "or"
		}
		return spec.BinaryOp{
			Pos:   v.Pos,
			Left:  normalizeOperatorsInExpr(v.Left),
			Op:    op,
			Right: normalizeOperatorsInExpr(v.Right),
		}
	case spec.UnaryOp:
		op := v.Op
		if op == "!" {
			op = "not"
		}
		return spec.UnaryOp{
			Pos:     v.Pos,
			Op:      op,
			Operand: normalizeOperatorsInExpr(v.Operand),
		}
	case spec.ObjectLiteral:
		fields := make([]*spec.ObjField, len(v.Fields))
		for i, f := range v.Fields {
			fields[i] = &spec.ObjField{Key: f.Key, Value: normalizeOperatorsInExpr(f.Value)}
		}
		return spec.ObjectLiteral{Fields: fields}
	case spec.ArrayLiteral:
		elems := make([]spec.Expr, len(v.Elements))
		for i, el := range v.Elements {
			elems[i] = normalizeOperatorsInExpr(el)
		}
		return spec.ArrayLiteral{Elements: elems}
	case spec.IfExpr:
		return spec.IfExpr{
			Pos:       v.Pos,
			Condition: normalizeOperatorsInExpr(v.Condition),
			Then:      normalizeOperatorsInExpr(v.Then),
			Else:      normalizeOperatorsInExpr(v.Else),
		}
	default:
		// Leaves: LiteralInt, LiteralFloat, LiteralString, LiteralBool, LiteralNull,
		// FieldRef, EnvRef, ServiceRef, LenExpr, ContainsExpr, ExistsExpr, HasKeyExpr,
		// AllExpr, AnyExpr, AdapterCall — none of these contain sub-expressions that
		// could carry symbolic operators, so return as-is.
		return e
	}
}

// v4FormatExpr normalizes symbolic operators in e to word operators, then formats
// the expression. Use this instead of spec.FormatExpr when emitting v4 output.
func v4FormatExpr(e spec.Expr) string {
	return spec.FormatExpr(normalizeOperatorsInExpr(e))
}

// v4FormatCallArgs formats a list of call arguments with v4 operator normalization.
func v4FormatCallArgs(args []spec.Expr) string {
	if len(args) == 0 {
		return ""
	}
	parts := make([]string, len(args))
	for i, a := range args {
		parts[i] = v4FormatExpr(a)
	}
	return strings.Join(parts, ", ")
}

// formatV4TypeExpr renders a TypeExpr as valid v4 syntax.
// This is functionally identical to formatTypeExpr (v3) since type syntax
// is unchanged; we have a separate function for clarity.
func formatV4TypeExpr(t spec.TypeExpr) string {
	return formatTypeExpr(t)
}
