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
type Spec struct {
	Pos            Pos                          `json:"pos,omitempty"`
	Name           string                       `json:"name"`
	Description    string                       `json:"description,omitempty"`
	AdapterConfigs map[string]map[string]Expr   `json:"adapter_configs,omitempty"` // v3: "http" -> {"base_url": expr}
	Services       []*Service                   `json:"services,omitempty"`        // v3: moved from Target
	Locators       map[string]string            `json:"locators,omitempty"`        // v2 compat
	Target         *Target                      `json:"target,omitempty"`          // v2 compat
	Models         []*Model                     `json:"models,omitempty"`
	Actions        []*ActionDef                 `json:"actions,omitempty"`         // v3: was []*Action
	Scopes         []*Scope                     `json:"scopes,omitempty"`
}

// Scope is a named grouping that owns a contract, invariants, and scenarios.
// Use declares which plugin adapter handles this scope's execution.
// The Config block is opaque key-value pairs interpreted by the adapter.
type Scope struct {
	Pos        Pos             `json:"pos,omitempty"`
	Name       string          `json:"name"`
	Use        string          `json:"use,omitempty"`    // v2 compat, v3 ignores
	Config     map[string]Expr `json:"config,omitempty"` // v2 compat, v3 ignores
	Before     *Block          `json:"before,omitempty"`
	After      *Block          `json:"after,omitempty"`
	Contract   *Contract       `json:"contract,omitempty"`
	Actions    []*ActionDef    `json:"actions,omitempty"` // v3: scope-level actions
	Invariants []*Invariant    `json:"invariants,omitempty"`
	Scenarios  []*Scenario     `json:"scenarios,omitempty"`
}

// Service declares a container to run as test infrastructure.
type Service struct {
	Pos     Pos               `json:"pos,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
	Volumes map[string]string `json:"volumes,omitempty"`
	Name    string            `json:"name"`
	Build   string            `json:"build,omitempty"`
	Image   string            `json:"image,omitempty"`
	Health  string            `json:"health,omitempty"`
	Port    int               `json:"port,omitempty"`
}

// Target holds configuration for the system under test.
type Target struct {
	Pos      Pos             `json:"pos,omitempty"`
	Fields   map[string]Expr `json:"fields,omitempty"`   // key -> value expression (may be EnvRef, LiteralString, etc.)
	Compose  string          `json:"compose,omitempty"`  // docker-compose file path (mutually exclusive with services)
	Services []*Service      `json:"services,omitempty"` // named container services
}

// Model defines a named data structure.
type Model struct {
	Pos    Pos      `json:"pos,omitempty"`
	Name   string   `json:"name"`
	Fields []*Field `json:"fields,omitempty"`
}

// Field is a typed field with an optional constraint.
type Field struct {
	Pos        Pos      `json:"pos,omitempty"`
	Constraint Expr     `json:"constraint,omitempty"` // optional constraint expression (nil when absent)
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
	Variants []string  `json:"variants,omitempty"`  // enum variants (for enum type)
	Optional bool      `json:"optional,omitempty"`  // trailing ?
}

// Contract defines the input/output boundary of the system under test.
type Contract struct {
	Pos    Pos      `json:"pos,omitempty"`
	Input  []*Field `json:"input,omitempty"`
	Output []*Field `json:"output,omitempty"`
	Action string   `json:"action,omitempty"` // v3: name of action to execute
}

// Action is a named reusable sequence of plugin calls.
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

// ActionDef is a reusable action with typed parameters and a body of steps.
type ActionDef struct {
	Pos    Pos         `json:"pos,omitempty"`
	Name   string      `json:"name"`
	Params []*Param    `json:"params,omitempty"`
	Body   []GivenStep `json:"body,omitempty"` // steps including let bindings, calls, return
}

// LetBinding binds an expression result to a name. Implements GivenStep.
type LetBinding struct {
	Pos   Pos    `json:"pos,omitempty"`
	Name  string `json:"name"`
	Value Expr   `json:"value"` // can be an expression or an AdapterCall
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
	Adapter string `json:"adapter"`     // "http", "playwright", "process"
	Method  string `json:"method"`      // "post", "fill", "visible"
	Args    []Expr `json:"args,omitempty"`
}

// Call is an invocation: plugin.verb(args...) or action(args...)
type Call struct {
	Pos       Pos    `json:"pos,omitempty"`
	Namespace string `json:"namespace,omitempty"` // plugin name, empty for local actions
	Method    string `json:"method"`
	Args      []Expr `json:"args,omitempty"`
}

// Invariant is a universal property that must hold across all valid inputs.
type Invariant struct {
	Pos        Pos          `json:"pos,omitempty"`
	When       Expr         `json:"when,omitempty"` // optional guard predicate (nil when absent)
	Name       string       `json:"name"`
	Assertions []*Assertion `json:"assertions,omitempty"` // body assertions
}

// Scenario is a test case -- concrete (given) or generative (when-predicate).
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
	Predicates []Expr       `json:"predicates,omitempty"` // when-predicate conditions (when blocks)
	Assertions []*Assertion `json:"assertions,omitempty"` // then-block checks
}

// Assertion is a check. Two forms:
//   - Path assertion (then blocks): Target + Expected are set. E.g. "from.balance: 70"
//   - Expression assertion (invariants): Expr is set. E.g. "output.from.balance >= 0"
type Assertion struct {
	Pos Pos `json:"pos,omitempty"`

	// Expression assertion field
	Expr Expr `json:"expr,omitempty"` // boolean expression (invariants)

	// Path assertion fields
	Expected Expr   `json:"expected,omitempty"` // expected value
	Target   string `json:"target,omitempty"`   // field path or locator name
	Plugin   string `json:"plugin,omitempty"`   // plugin namespace (from @ syntax)
	Property string `json:"property,omitempty"` // assertion property name
	Operator string `json:"operator,omitempty"` // comparison operator: ==, !=, >, >=, <, <= (default ==)
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
	Path string `json:"path"` // e.g., "input.from.balance", "output.error"
}

type EnvRef struct {
	Pos     Pos    `json:"pos,omitempty"`
	Var     string `json:"var"`
	Default string `json:"default,omitempty"`
} // env(VAR) or env(VAR, "default")

// ServiceRef references a named service from the target services block.
// Resolves to the service's URL at runtime. Docker must be available.
type ServiceRef struct {
	Pos  Pos    `json:"pos,omitempty"`
	Name string `json:"name"`
}

type BinaryOp struct {
	Pos   Pos    `json:"pos,omitempty"`
	Left  Expr   `json:"left,omitempty"`
	Right Expr   `json:"right,omitempty"`
	Op    string `json:"op"` // ==, !=, >, <, >=, <=, +, -, *, /, %, &&, ||
}

type UnaryOp struct {
	Pos     Pos    `json:"pos,omitempty"`
	Operand Expr   `json:"operand,omitempty"`
	Op      string `json:"op"` // !, -
}

type ObjectLiteral struct {
	Pos    Pos         `json:"pos,omitempty"`
	Fields []*ObjField `json:"fields,omitempty"` // ordered key-value pairs
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

// AllExpr represents all(array, elem => predicate) — true if predicate holds for every element.
type AllExpr struct {
	Pos       Pos    `json:"pos,omitempty"`
	Array     Expr   `json:"array"`     // expression evaluating to an array
	Predicate Expr   `json:"predicate"` // boolean expression evaluated per element
	BoundVar  string `json:"bound_var"` // loop variable name
}

// AnyExpr represents any(array, elem => predicate) — true if predicate holds for at least one element.
type AnyExpr struct {
	Pos       Pos    `json:"pos,omitempty"`
	Array     Expr   `json:"array"`     // expression evaluating to an array
	Predicate Expr   `json:"predicate"` // boolean expression evaluated per element
	BoundVar  string `json:"bound_var"` // loop variable name
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
// Used in failure messages to identify which assertion failed.
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
