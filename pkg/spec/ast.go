package spec

import (
	"fmt"
	"strings"
)

// Pos records the source position of an AST node.
type Pos struct {
	File string `json:"file,omitempty"`
	Line int    `json:"line,omitempty"`
	Col  int    `json:"col,omitempty"`
}

// String formats the position as "file:line:col", "line:col", or "".
func (p Pos) String() string {
	if p.File != "" {
		return fmt.Sprintf("%s:%d:%d", p.File, p.Line, p.Col)
	}
	if p.Line != 0 {
		return fmt.Sprintf("%d:%d", p.Line, p.Col)
	}
	return ""
}

// Spec is the top-level AST node for a parsed spec file.
// In v4, the file IS the spec — there is no spec Name { } wrapper.
// The filename is the spec's identity.
type Spec struct {
	Pos            Pos                        `json:"pos,omitempty"`
	AdapterConfigs map[string]map[string]Expr `json:"adapter_configs,omitempty"` // e.g. "http" -> {"base_url": expr}
	Services       []*Service                 `json:"services,omitempty"`
	Models         []*Model                   `json:"models,omitempty"`
	Enums          []*NamedEnum               `json:"enums,omitempty"`
	Actions        []*ActionDef               `json:"actions,omitempty"`
	Scopes         []*Scope                   `json:"scopes,omitempty"`
	Contracts      []*Contract                `json:"contracts,omitempty"` // top-level contracts (outside any scope)
	Config         map[string]Expr            `json:"config,omitempty"`   // spec-level config constants

	// Kept for v2/v3 compatibility (migration tool, OpenAPI/proto importers)
	Locators map[string]string `json:"locators,omitempty"` // v2 compat
	Target   *Target           `json:"target,omitempty"`   // v2 compat
}

// Scope groups related contracts that share lifecycle hooks (before/after).
// It no longer owns contracts directly — contracts declare themselves and
// are placed inside a scope for shared lifecycle management.
type Scope struct {
	Pos       Pos          `json:"pos,omitempty"`
	Name      string       `json:"name"`
	Before    *Block       `json:"before,omitempty"`
	After     *Block       `json:"after,omitempty"`
	Contracts []*Contract  `json:"contracts,omitempty"` // v4: multiple contracts per scope
}

// Service declares a container to run as test infrastructure.
type Service struct {
	Pos     Pos               `json:"pos,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
	Volumes map[string]string `json:"volumes,omitempty"`
	Name    string            `json:"name"`
	Build   string            `json:"build,omitempty"`
	Compose string            `json:"compose,omitempty"` // docker-compose file path; mutually exclusive with build/image
	Image   string            `json:"image,omitempty"`
	Health  string            `json:"health,omitempty"`
	Port    int               `json:"port,omitempty"`
}

// Target holds configuration for the system under test (v2 compat).
type Target struct {
	Pos      Pos             `json:"pos,omitempty"`
	Fields   map[string]Expr `json:"fields,omitempty"`
	Compose  string          `json:"compose,omitempty"`
	Services []*Service      `json:"services,omitempty"`
}

// Model defines a named data structure.
type Model struct {
	Pos    Pos      `json:"pos,omitempty"`
	Name   string   `json:"name"`
	Fields []*Field `json:"fields,omitempty"`
}

// NamedEnum is a named enumeration at top level.
// Variants are referenced as EnumName.variant in expressions.
type NamedEnum struct {
	Pos      Pos      `json:"pos,omitempty"`
	Name     string   `json:"name"`
	Variants []string `json:"variants"`
}

// Field is a typed field with an optional constraint and optional state-dependent presence.
type Field struct {
	Pos        Pos      `json:"pos,omitempty"`
	Constraint Expr     `json:"constraint,omitempty"` // optional constraint expression
	When       Expr     `json:"when,omitempty"`       // optional presence condition (state-dependent fields)
	Name       string   `json:"name"`
	Type       TypeExpr `json:"type"`
}

// TypeExpr represents a type in the spec language.
type TypeExpr struct {
	Pos      Pos       `json:"pos,omitempty"`
	Name     string    `json:"name"`                // "int", "string", "bool", "float", "bytes", "array", "map", "enum", or model name
	ElemType *TypeExpr `json:"elem_type,omitempty"` // element type for arrays
	KeyType  *TypeExpr `json:"key_type,omitempty"`  // key type for maps
	ValType  *TypeExpr `json:"val_type,omitempty"`  // value type for maps
	Variants []string  `json:"variants,omitempty"`  // inline enum variants
	Optional bool      `json:"optional,omitempty"`  // trailing ?
}

// Contract is the primary unit of verification. It is a self-contained behavioral
// promise: it declares its inputs (fields), how to execute (action block), and what
// must hold (invariants, scenarios). It specifies its return type via ReturnType.
//
// Syntax: contract Name -> ReturnModel { fields... action { body } invariants... scenarios... }
// With inheritance: contract Name: InputModel -> ReturnModel { constrain { ... } ... }
type Contract struct {
	Pos         Pos          `json:"pos,omitempty"`
	Name        string       `json:"name"`
	Inherits    string       `json:"inherits,omitempty"`    // optional: model name to inherit fields from
	Fields      []*Field     `json:"fields,omitempty"`      // input fields (declared in the contract body)
	Constraints []Expr       `json:"constraints,omitempty"` // constrain block (for inherited fields)
	ReturnType  TypeExpr     `json:"return_type"`           // -> ReturnModel
	Action      *ActionBlock `json:"action,omitempty"`      // action { body }
	Invariants  []*Invariant `json:"invariants,omitempty"`
	Scenarios   []*Scenario  `json:"scenarios,omitempty"`
}

// ActionBlock is the body of a contract's action block.
// It has no signature — it uses the contract's fields as its implicit inputs.
type ActionBlock struct {
	Pos  Pos         `json:"pos,omitempty"`
	Body []GivenStep `json:"body,omitempty"`
}

// ActionDef is a reusable named action with typed parameters and a body of steps.
// Used in before/given blocks and referenced in contract action bodies.
type ActionDef struct {
	Pos    Pos         `json:"pos,omitempty"`
	Name   string      `json:"name"`
	Params []*Param    `json:"params,omitempty"`
	Body   []GivenStep `json:"body,omitempty"`
}

// Action is a named reusable sequence of plugin calls (v2 compat).
type Action struct {
	Pos    Pos      `json:"pos,omitempty"`
	Name   string   `json:"name"`
	Params []*Param `json:"params,omitempty"`
	Steps  []*Call  `json:"steps,omitempty"`
}

// Param is a named, typed parameter.
type Param struct {
	Pos  Pos      `json:"pos,omitempty"`
	Name string   `json:"name"`
	Type TypeExpr `json:"type"`
}

// LetBinding binds an expression result to a name. Implements GivenStep.
type LetBinding struct {
	Pos   Pos    `json:"pos,omitempty"`
	Name  string `json:"name"`
	Value Expr   `json:"value"`
}

// ReturnStmt returns a value from an action body. Implements GivenStep.
type ReturnStmt struct {
	Pos   Pos  `json:"pos,omitempty"`
	Value Expr `json:"value"`
}

// AdapterCall represents adapter.method(args...) — used in action bodies, before, given.
// When used as an Expr (e.g., right side of let), it evaluates to the response.
// When used as a GivenStep, it executes the call.
type AdapterCall struct {
	Pos     Pos    `json:"pos,omitempty"`
	Adapter string `json:"adapter"`
	Method  string `json:"method"`
	Args    []Expr `json:"args,omitempty"`
}

// Call is an invocation: plugin.verb(args...) or action(args...)
type Call struct {
	Pos       Pos    `json:"pos,omitempty"`
	Namespace string `json:"namespace,omitempty"`
	Method    string `json:"method"`
	Args      []Expr `json:"args,omitempty"`
}

// Invariant is a universal property that must hold across all valid inputs.
type Invariant struct {
	Pos        Pos          `json:"pos,omitempty"`
	When       Expr         `json:"when,omitempty"` // optional guard predicate
	Name       string       `json:"name"`
	Assertions []*Assertion `json:"assertions,omitempty"`
}

// Scenario is a test case — concrete (given) or generative (when-predicate).
type Scenario struct {
	Pos   Pos    `json:"pos,omitempty"`
	Given *Block `json:"given,omitempty"` // concrete values
	When  *Block `json:"when,omitempty"`  // predicate block (generative)
	Then  *Block `json:"then,omitempty"`  // assertions
	Name  string `json:"name"`
}

// GivenStep is a step in a given block — either an assignment or an action call.
type GivenStep interface{ givenStep() }

func (*Assignment) givenStep()  {}
func (*Call) givenStep()        {}
func (*LetBinding) givenStep()  {}
func (*ReturnStmt) givenStep()  {}
func (*AdapterCall) givenStep() {}

// Block is a braced section containing steps, predicates, or assertions.
type Block struct {
	Pos        Pos          `json:"pos,omitempty"`
	Steps      []GivenStep  `json:"steps,omitempty"`      // ordered: assignments + calls (given blocks)
	Predicates []Expr       `json:"predicates,omitempty"` // when-predicate conditions
	Assertions []*Assertion `json:"assertions,omitempty"` // then-block checks
}

// Assertion is a check. Two forms:
//   - Path assertion (then blocks): Target + Expected are set.
//   - Expression assertion (invariants): Expr is set.
type Assertion struct {
	Pos Pos `json:"pos,omitempty"`

	// Expression assertion field
	Expr Expr `json:"expr,omitempty"` // boolean expression (invariants)

	// Path assertion fields
	Expected Expr   `json:"expected,omitempty"`
	Target   string `json:"target,omitempty"`
	Plugin   string `json:"plugin,omitempty"`
	Property string `json:"property,omitempty"`
	Operator string `json:"operator,omitempty"` // ==, !=, >, >=, <, <= (default ==)
}

// Assignment sets a concrete value: field: value
type Assignment struct {
	Pos   Pos    `json:"pos,omitempty"`
	Value Expr   `json:"value,omitempty"`
	Path  string `json:"path"`
}

// Expr is an expression node.
type Expr interface {
	exprNode()
}

type LiteralInt struct {
	Pos   Pos `json:"pos,omitempty"`
	Value int `json:"value"`
}

type LiteralFloat struct {
	Pos   Pos     `json:"pos,omitempty"`
	Value float64 `json:"value"`
}

type LiteralString struct {
	Pos   Pos    `json:"pos,omitempty"`
	Value string `json:"value"`
}

type LiteralBool struct {
	Pos   Pos  `json:"pos,omitempty"`
	Value bool `json:"value"`
}

type LiteralNull struct {
	Pos Pos `json:"pos,omitempty"`
}

type FieldRef struct {
	Pos  Pos    `json:"pos,omitempty"`
	Path string `json:"path"` // e.g., "from.balance", "output.error", "config.key", "Role.admin"
}

type EnvRef struct {
	Pos     Pos    `json:"pos,omitempty"`
	Var     string `json:"var"`
	Default string `json:"default,omitempty"`
}

// ServiceRef references a named service from the services block.
// Resolves to the service's URL at runtime.
type ServiceRef struct {
	Pos  Pos    `json:"pos,omitempty"`
	Name string `json:"name"`
}

type BinaryOp struct {
	Pos   Pos    `json:"pos,omitempty"`
	Left  Expr   `json:"left,omitempty"`
	Right Expr   `json:"right,omitempty"`
	Op    string `json:"op"` // ==, !=, >, <, >=, <=, +, -, *, /, %, and, or, implies, in
}

type UnaryOp struct {
	Pos     Pos    `json:"pos,omitempty"`
	Operand Expr   `json:"operand,omitempty"`
	Op      string `json:"op"` // not, -
}

type ObjectLiteral struct {
	Pos    Pos         `json:"pos,omitempty"`
	Fields []*ObjField `json:"fields,omitempty"`
}

type ObjField struct {
	Pos   Pos    `json:"pos,omitempty"`
	Value Expr   `json:"value,omitempty"`
	Key   string `json:"key"`
}

type ArrayLiteral struct {
	Pos      Pos    `json:"pos,omitempty"`
	Elements []Expr `json:"elements,omitempty"`
}

type LenExpr struct {
	Pos Pos  `json:"pos,omitempty"`
	Arg Expr `json:"arg"`
}

// AllExpr represents all(array, elem => predicate).
type AllExpr struct {
	Pos       Pos    `json:"pos,omitempty"`
	Array     Expr   `json:"array"`
	Predicate Expr   `json:"predicate"`
	BoundVar  string `json:"bound_var"`
}

// AnyExpr represents any(array, elem => predicate).
type AnyExpr struct {
	Pos       Pos    `json:"pos,omitempty"`
	Array     Expr   `json:"array"`
	Predicate Expr   `json:"predicate"`
	BoundVar  string `json:"bound_var"`
}

type ContainsExpr struct {
	Pos      Pos  `json:"pos,omitempty"`
	Haystack Expr `json:"haystack"`
	Needle   Expr `json:"needle"`
}

type ExistsExpr struct {
	Pos Pos  `json:"pos,omitempty"`
	Arg Expr `json:"arg"`
}

type HasKeyExpr struct {
	Pos Pos  `json:"pos,omitempty"`
	Arg Expr `json:"arg"`
	Key Expr `json:"key"`
}

type RegexLiteral struct {
	Pos     Pos    `json:"pos,omitempty"`
	Pattern string `json:"pattern"`
}

type IfExpr struct {
	Pos       Pos  `json:"pos,omitempty"`
	Condition Expr `json:"condition"`
	Then      Expr `json:"then"`
	Else      Expr `json:"else"`
}

func (LiteralInt) exprNode()    {}
func (LiteralFloat) exprNode()  {}
func (LiteralString) exprNode() {}
func (LiteralBool) exprNode()   {}
func (LiteralNull) exprNode()   {}
func (FieldRef) exprNode()      {}
func (BinaryOp) exprNode()      {}
func (UnaryOp) exprNode()       {}
func (ObjectLiteral) exprNode() {}
func (ArrayLiteral) exprNode()  {}
func (EnvRef) exprNode()        {}
func (ServiceRef) exprNode()    {}
func (LenExpr) exprNode()       {}
func (AllExpr) exprNode()       {}
func (AnyExpr) exprNode()       {}
func (ContainsExpr) exprNode()  {}
func (ExistsExpr) exprNode()    {}
func (HasKeyExpr) exprNode()    {}
func (RegexLiteral) exprNode()  {}
func (IfExpr) exprNode()        {}
func (AdapterCall) exprNode()   {}

// FormatExpr returns a human-readable string representation of an expression.
func FormatExpr(e Expr) string {
	if e == nil {
		return "<nil>"
	}
	switch v := e.(type) {
	case LiteralInt:
		return fmt.Sprintf("%d", v.Value)
	case LiteralFloat:
		return fmt.Sprintf("%g", v.Value)
	case LiteralString:
		return fmt.Sprintf("%q", v.Value)
	case LiteralBool:
		if v.Value {
			return "true"
		}
		return "false"
	case LiteralNull:
		return "null"
	case FieldRef:
		return v.Path
	case BinaryOp:
		return FormatExpr(v.Left) + " " + v.Op + " " + FormatExpr(v.Right)
	case UnaryOp:
		if v.Op == "not" {
			return "not " + FormatExpr(v.Operand)
		}
		return v.Op + FormatExpr(v.Operand)
	case LenExpr:
		return "len(" + FormatExpr(v.Arg) + ")"
	case ContainsExpr:
		return "contains(" + FormatExpr(v.Haystack) + ", " + FormatExpr(v.Needle) + ")"
	case ExistsExpr:
		return "exists(" + FormatExpr(v.Arg) + ")"
	case HasKeyExpr:
		return "has_key(" + FormatExpr(v.Arg) + ", " + FormatExpr(v.Key) + ")"
	case AllExpr:
		return "all(" + FormatExpr(v.Array) + ", " + v.BoundVar + " => " + FormatExpr(v.Predicate) + ")"
	case AnyExpr:
		return "any(" + FormatExpr(v.Array) + ", " + v.BoundVar + " => " + FormatExpr(v.Predicate) + ")"
	case EnvRef:
		if v.Default != "" {
			return fmt.Sprintf("env(%s, %q)", v.Var, v.Default)
		}
		return "env(" + v.Var + ")"
	case ServiceRef:
		return "service(" + v.Name + ")"
	case IfExpr:
		return "if " + FormatExpr(v.Condition) + " then " + FormatExpr(v.Then) + " else " + FormatExpr(v.Else)
	case ObjectLiteral:
		if len(v.Fields) == 0 {
			return "{}"
		}
		parts := make([]string, len(v.Fields))
		for i, f := range v.Fields {
			parts[i] = f.Key + ": " + FormatExpr(f.Value)
		}
		return "{ " + strings.Join(parts, ", ") + " }"
	case ArrayLiteral:
		if len(v.Elements) == 0 {
			return "[]"
		}
		parts := make([]string, len(v.Elements))
		for i, e := range v.Elements {
			parts[i] = FormatExpr(e)
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case AdapterCall:
		args := formatArgs(v.Args)
		return v.Adapter + "." + v.Method + "(" + args + ")"
	default:
		return fmt.Sprintf("<%T>", e)
	}
}

func formatArgs(args []Expr) string {
	if len(args) == 0 {
		return ""
	}
	s := FormatExpr(args[0])
	for _, a := range args[1:] {
		s += ", " + FormatExpr(a)
	}
	return s
}
