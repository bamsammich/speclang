package parser

// Spec is the top-level AST node for a parsed spec file.
type Spec struct {
	Uses        []string          `json:"uses,omitempty"`
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Target      *Target           `json:"target,omitempty"`
	Locators    map[string]string `json:"locators,omitempty"`
	Models      []*Model          `json:"models,omitempty"`
	Actions     []*Action         `json:"actions,omitempty"`
	Scopes      []*Scope          `json:"scopes,omitempty"`
}

// Scope is a named grouping that owns a contract, invariants, and scenarios.
// The Config block is opaque key-value pairs interpreted by the adapter.
type Scope struct {
	Name       string              `json:"name"`
	Config     map[string]Expr     `json:"config,omitempty"` // opaque key-value pairs, interpreted by adapter
	Contract   *Contract           `json:"contract,omitempty"`
	Invariants []*Invariant        `json:"invariants,omitempty"`
	Scenarios  []*Scenario         `json:"scenarios,omitempty"`
}

// Target holds configuration for the system under test.
type Target struct {
	Fields map[string]Expr `json:"fields,omitempty"` // key -> value expression (may be EnvRef, LiteralString, etc.)
}

// Model defines a named data structure.
type Model struct {
	Name   string   `json:"name"`
	Fields []*Field `json:"fields,omitempty"`
}

// Field is a typed field with an optional constraint.
type Field struct {
	Constraint Expr     `json:"constraint,omitempty"` // optional constraint expression (nil when absent)
	Name       string   `json:"name"`
	Type       TypeExpr `json:"type"`
}

// TypeExpr represents a type in the spec language.
type TypeExpr struct {
	Name     string    `json:"name"`                    // "int", "string", "bool", "float", "bytes", "array", "map", or model name
	ElemType *TypeExpr `json:"elem_type,omitempty"`     // element type for arrays
	KeyType  *TypeExpr `json:"key_type,omitempty"`      // key type for maps
	ValType  *TypeExpr `json:"val_type,omitempty"`      // value type for maps
	Optional bool      `json:"optional,omitempty"`      // trailing ?
}

// Contract defines the input/output boundary of the system under test.
type Contract struct {
	Input  []*Field `json:"input,omitempty"`
	Output []*Field `json:"output,omitempty"`
}

// Action is a named reusable sequence of plugin calls.
type Action struct {
	Name   string   `json:"name"`
	Params []*Param `json:"params,omitempty"`
	Steps  []*Call  `json:"steps,omitempty"`
}

// Param is a named, typed parameter.
type Param struct {
	Name string   `json:"name"`
	Type TypeExpr `json:"type"`
}

// Call is an invocation: plugin.verb(args...) or action(args...)
type Call struct {
	Namespace string `json:"namespace,omitempty"` // plugin name, empty for local actions
	Method    string `json:"method"`
	Args      []Expr `json:"args,omitempty"`
}

// Invariant is a universal property that must hold across all valid inputs.
type Invariant struct {
	When       Expr         `json:"when,omitempty"` // optional guard predicate (nil when absent)
	Name       string       `json:"name"`
	Assertions []*Assertion `json:"assertions,omitempty"` // body assertions
}

// Scenario is a test case -- concrete (given) or generative (when-predicate).
type Scenario struct {
	Given *Block `json:"given,omitempty"` // concrete values
	When  *Block `json:"when,omitempty"`  // predicate block (generative)
	Then  *Block `json:"then,omitempty"`  // assertions
	Name  string `json:"name"`
}

// Block is a braced section containing assignments, predicates, or assertions.
type Block struct {
	Assignments []*Assignment `json:"assignments,omitempty"` // concrete values (given blocks)
	Predicates  []Expr        `json:"predicates,omitempty"`  // when-predicate conditions (when blocks)
	Assertions  []*Assertion  `json:"assertions,omitempty"`  // then-block checks
}

// Assertion is a check. Two forms:
//   - Path assertion (then blocks): Target + Expected are set. E.g. "from.balance: 70"
//   - Expression assertion (invariants): Expr is set. E.g. "output.from.balance >= 0"
type Assertion struct {
	// Expression assertion field
	Expr Expr `json:"expr,omitempty"` // boolean expression (invariants)

	// Path assertion fields
	Expected Expr   `json:"expected,omitempty"` // expected value
	Target   string `json:"target,omitempty"`   // field path or locator name
	Plugin   string `json:"plugin,omitempty"`   // plugin namespace (from @ syntax)
	Property string `json:"property,omitempty"` // assertion property name
}

// Assignment sets a concrete value: field: value
type Assignment struct {
	Value Expr   `json:"value,omitempty"`
	Path  string `json:"path"`
}

// Expr is an expression node.
type Expr interface {
	exprNode()
}

type LiteralInt struct {
	Value int `json:"value"`
}

type LiteralFloat struct {
	Value float64 `json:"value"`
}

type LiteralString struct {
	Value string `json:"value"`
}

type LiteralBool struct {
	Value bool `json:"value"`
}

type LiteralNull struct{}

type FieldRef struct {
	Path string `json:"path"` // e.g., "input.from.balance", "output.error"
}

type EnvRef struct {
	Var     string `json:"var"`
	Default string `json:"default,omitempty"`
} // env(VAR) or env(VAR, "default")

type BinaryOp struct {
	Left  Expr   `json:"left,omitempty"`
	Right Expr   `json:"right,omitempty"`
	Op    string `json:"op"` // ==, !=, >, <, >=, <=, +, -, *, &&, ||
}

type UnaryOp struct {
	Operand Expr   `json:"operand,omitempty"`
	Op      string `json:"op"` // !, -
}

type ObjectLiteral struct {
	Fields []*ObjField `json:"fields,omitempty"` // ordered key-value pairs
}

type ObjField struct {
	Value Expr   `json:"value,omitempty"`
	Key   string `json:"key"`
}

type RegexLiteral struct {
	Pattern string `json:"pattern"`
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
func (EnvRef) exprNode()        {}
func (RegexLiteral) exprNode()  {}
