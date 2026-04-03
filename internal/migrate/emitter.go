package migrate

import (
	"fmt"
	"strings"

	"github.com/bamsammich/speclang/v3/pkg/spec"
)

// v3Writer emits formatted v3 spec text from AST nodes.
type v3Writer struct {
	buf    strings.Builder
	indent int
}

func (w *v3Writer) line(format string, args ...any) {
	w.buf.WriteString(strings.Repeat("  ", w.indent))
	fmt.Fprintf(&w.buf, format, args...)
	w.buf.WriteByte('\n')
}

func (w *v3Writer) raw(s string) {
	w.buf.WriteString(s)
}

func (w *v3Writer) open(format string, args ...any) {
	w.line(format, args...)
	w.indent++
}

func (w *v3Writer) close() {
	w.indent--
	w.line("}")
}

func (w *v3Writer) blank() {
	w.buf.WriteByte('\n')
}

func (w *v3Writer) comment(text string) {
	w.line("# %s", text)
}

func (w *v3Writer) String() string {
	return w.buf.String()
}

func (w *v3Writer) emitSpec(s *spec.Spec) {
	w.open("spec %s {", s.Name)

	if s.Description != "" {
		w.line("description: %q", s.Description)
		w.blank()
	}

	// Adapter configs (transformed from target)
	adapterConfigs := inferAdapterConfigs(s)
	for _, name := range sortedKeys(adapterConfigs) {
		cfg := adapterConfigs[name]
		w.open("%s {", name)
		for _, k := range sortedKeys(cfg) {
			w.line("%s: %s", k, cfg[k])
		}
		w.close()
		w.blank()
	}

	// Services (extracted from target or already at spec level)
	services := s.Services
	if services == nil && s.Target != nil {
		services = s.Target.Services
	}
	if len(services) > 0 {
		w.emitServices(services)
		w.blank()
	}

	// Models
	for _, m := range s.Models {
		w.emitModel(m)
		w.blank()
	}

	// Spec-level actions
	for _, a := range s.Actions {
		w.emitActionDef(a)
		w.blank()
	}

	// Scopes
	locators := s.Locators
	if locators == nil {
		locators = make(map[string]string)
	}
	for i, sc := range s.Scopes {
		if i > 0 {
			w.blank()
		}
		w.emitScope(sc, locators)
	}

	w.close()
}

func (w *v3Writer) emitServices(svcs []*spec.Service) {
	w.open("services {")
	for _, svc := range svcs {
		w.open("%s {", svc.Name)
		if svc.Build != "" {
			w.line("build: %q", svc.Build)
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

func (w *v3Writer) emitModel(m *spec.Model) {
	w.open("model %s {", m.Name)
	for _, f := range m.Fields {
		w.emitField(f)
	}
	w.close()
}

func (w *v3Writer) emitField(f *spec.Field) {
	typeStr := formatTypeExpr(f.Type)
	if f.Constraint != nil {
		w.line("%s: %s { %s }", f.Name, typeStr, spec.FormatExpr(f.Constraint))
	} else {
		w.line("%s: %s", f.Name, typeStr)
	}
}

func (w *v3Writer) emitScope(sc *spec.Scope, locators map[string]string) {
	w.open("scope %s {", sc.Name)

	// Synthesize action from v2 use+config if the scope has adapter-specific config
	// (method/path for HTTP, command for process). Skip for adapters like playwright
	// that don't use config-driven action synthesis.
	var synthAction *synthesizedAction
	if sc.Use != "" && sc.Contract != nil && hasSynthesizableConfig(sc) {
		synthAction = synthesizeAction(sc)
		w.emitSynthesizedAction(synthAction)
		w.blank()
	}

	// Existing v3 scope-level actions
	for _, a := range sc.Actions {
		w.emitActionDef(a)
		w.blank()
	}

	// Before block
	if sc.Before != nil {
		w.emitBlock(sc.Before, "before", sc.Use, locators)
		w.blank()
	}

	// After block
	if sc.After != nil {
		w.emitBlock(sc.After, "after", sc.Use, locators)
		w.blank()
	}

	// Contract
	if sc.Contract != nil {
		actionName := sc.Contract.Action
		if actionName == "" && synthAction != nil {
			actionName = synthAction.name
		}
		w.emitContract(sc.Contract, actionName)
		w.blank()
	}

	// Invariants
	for _, inv := range sc.Invariants {
		w.emitInvariant(inv)
		w.blank()
	}

	// Scenarios
	for i, s := range sc.Scenarios {
		if i > 0 {
			w.blank()
		}
		w.emitScenario(s, locators)
	}

	w.close()
}

func (w *v3Writer) emitSynthesizedAction(sa *synthesizedAction) {
	// Build param list
	params := make([]string, len(sa.params))
	for i, p := range sa.params {
		params[i] = p.Name + ": " + formatTypeExpr(p.Type)
	}
	w.open("action %s(%s) {", sa.name, strings.Join(params, ", "))

	// Emit header calls if any
	for _, h := range sa.headerCalls {
		w.line("%s", h)
	}

	// Emit the main call with let binding
	w.line("let result = %s", sa.callExpr)
	w.line("return result")
	w.close()
}

func (w *v3Writer) emitActionDef(a *spec.ActionDef) {
	params := make([]string, len(a.Params))
	for i, p := range a.Params {
		params[i] = p.Name + ": " + formatTypeExpr(p.Type)
	}
	w.open("action %s(%s) {", a.Name, strings.Join(params, ", "))
	for _, step := range a.Body {
		w.emitGivenStep(step, "", nil)
	}
	w.close()
}

func (w *v3Writer) emitContract(c *spec.Contract, actionName string) {
	w.open("contract {")
	if len(c.Input) > 0 {
		w.open("input {")
		for _, f := range c.Input {
			w.emitField(f)
		}
		w.close()
	}
	if len(c.Output) > 0 {
		w.open("output {")
		for _, f := range c.Output {
			w.emitField(f)
		}
		w.close()
	}
	if actionName != "" {
		w.line("action: %s", actionName)
	}
	w.close()
}

func (w *v3Writer) emitScenario(sc *spec.Scenario, locators map[string]string) {
	w.open("scenario %s {", sc.Name)
	if sc.Given != nil {
		w.emitBlock(sc.Given, "given", "", locators)
	}
	if sc.When != nil {
		w.emitBlock(sc.When, "when", "", locators)
	}
	if sc.Then != nil {
		w.emitBlock(sc.Then, "then", "", locators)
	}
	w.close()
}

func (w *v3Writer) emitInvariant(inv *spec.Invariant) {
	w.open("invariant %s {", inv.Name)
	if inv.When != nil {
		w.line("when %s:", spec.FormatExpr(inv.When))
	}
	for _, a := range inv.Assertions {
		w.emitAssertion(a, nil)
	}
	w.close()
}

func (w *v3Writer) emitBlock(b *spec.Block, kind string, scopeAdapter string, locators map[string]string) {
	w.open("%s {", kind)

	// Steps (given/before/after blocks)
	if len(b.Steps) > 0 {
		steps := b.Steps
		if kind == "before" || kind == "after" {
			steps = transformBodyRefs(steps, scopeAdapter)
		}
		for _, step := range steps {
			w.emitGivenStep(step, scopeAdapter, locators)
		}
	}

	// Predicates (when blocks)
	for _, pred := range b.Predicates {
		w.line("%s", spec.FormatExpr(pred))
	}

	// Assertions (then blocks)
	for _, a := range b.Assertions {
		w.emitAssertion(a, locators)
	}

	w.close()
}

func (w *v3Writer) emitGivenStep(step spec.GivenStep, scopeAdapter string, locators map[string]string) {
	switch s := step.(type) {
	case *spec.Assignment:
		w.line("%s: %s", s.Path, spec.FormatExpr(s.Value))
	case *spec.Call:
		// v2 Call: namespace.method(args) or action(args)
		args := formatCallArgs(s.Args, locators)
		if s.Namespace != "" {
			w.line("%s.%s(%s)", s.Namespace, s.Method, args)
		} else {
			w.line("%s(%s)", s.Method, args)
		}
	case *spec.LetBinding:
		w.line("let %s = %s", s.Name, spec.FormatExpr(s.Value))
	case *spec.ReturnStmt:
		w.line("return %s", spec.FormatExpr(s.Value))
	case *spec.AdapterCall:
		args := formatCallArgs(s.Args, locators)
		w.line("%s.%s(%s)", s.Adapter, s.Method, args)
	}
}

func (w *v3Writer) emitAssertion(a *spec.Assertion, locators map[string]string) {
	w.line("%s", transformAssertion(a, locators))
}

// formatTypeExpr renders a TypeExpr as valid v3 syntax.
func formatTypeExpr(t spec.TypeExpr) string {
	base := ""
	switch t.Name {
	case "array":
		if t.ElemType != nil {
			base = "[]" + formatTypeExpr(*t.ElemType)
		} else {
			base = "[]any"
		}
	case "map":
		k := "string"
		v := "any"
		if t.KeyType != nil {
			k = formatTypeExpr(*t.KeyType)
		}
		if t.ValType != nil {
			v = formatTypeExpr(*t.ValType)
		}
		base = fmt.Sprintf("map[%s,%s]", k, v)
	case "enum":
		quoted := make([]string, len(t.Variants))
		for i, v := range t.Variants {
			quoted[i] = fmt.Sprintf("%q", v)
		}
		base = "enum(" + strings.Join(quoted, ", ") + ")"
	default:
		base = t.Name
	}
	if t.Optional {
		return base + "?"
	}
	return base
}

// formatCallArgs formats a list of expressions as comma-separated args,
// resolving locator references if needed.
func formatCallArgs(args []spec.Expr, locators map[string]string) string {
	if len(args) == 0 {
		return ""
	}
	parts := make([]string, len(args))
	for i, a := range args {
		parts[i] = formatExprWithLocators(a, locators)
	}
	return strings.Join(parts, ", ")
}

// formatExprWithLocators formats an expression, resolving FieldRef names
// that match locator names to inline selector strings.
func formatExprWithLocators(e spec.Expr, locators map[string]string) string {
	if locators == nil {
		return spec.FormatExpr(e)
	}
	if ref, ok := e.(spec.FieldRef); ok {
		if sel, found := locators[ref.Path]; found {
			return formatSelector(sel)
		}
	}
	return spec.FormatExpr(e)
}
