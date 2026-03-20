package parser

// Spec is the top-level AST node for a parsed spec file.
type Spec struct {
	Uses     []string
	Name     string
	Target   *Target
	Locators map[string]string // name -> selector
	Models   []*Model
	Actions  []*Action
	Scopes   []*Scope
}

// Scope is a named grouping that owns a contract, invariants, and scenarios.
// The Config block is opaque key-value pairs interpreted by the adapter.
type Scope struct {
	Name       string
	Config     map[string]Expr // opaque key-value pairs, interpreted by adapter
	Contract   *Contract
	Invariants []*Invariant
	Scenarios  []*Scenario
}

// Target holds configuration for the system under test.
type Target struct {
	Fields map[string]Expr // key -> value expression (may be EnvRef, LiteralString, etc.)
}

// Model defines a named data structure.
type Model struct {
	Name   string
	Fields []*Field
}

// Field is a typed field with an optional constraint.
type Field struct {
	Constraint Expr // optional constraint expression (nil when absent)
	Name       string
	Type       TypeExpr
}

// TypeExpr represents a type in the spec language.
type TypeExpr struct {
	Name     string // "int", "string", "bool", or model name
	Optional bool   // trailing ?
}

// Contract defines the input/output boundary of the system under test.
type Contract struct {
	Input  []*Field
	Output []*Field
}

// Action is a named reusable sequence of plugin calls.
type Action struct {
	Name   string
	Params []*Param
	Steps  []*Call
}

// Param is a named, typed parameter.
type Param struct {
	Name string
	Type TypeExpr
}

// Call is an invocation: plugin.verb(args...) or action(args...)
type Call struct {
	Namespace string // plugin name, empty for local actions
	Method    string
	Args      []Expr
}

// Invariant is a universal property that must hold across all valid inputs.
type Invariant struct {
	When       Expr // optional guard predicate (nil when absent)
	Name       string
	Assertions []*Assertion // body assertions
}

// Scenario is a test case -- concrete (given) or generative (when-predicate).
type Scenario struct {
	Given *Block // concrete values
	When  *Block // predicate block (generative)
	Then  *Block // assertions
	Name  string
}

// Block is a braced section containing assignments, predicates, or assertions.
type Block struct {
	Assignments []*Assignment // concrete values (given blocks)
	Predicates  []Expr        // when-predicate conditions (when blocks)
	Assertions  []*Assertion  // then-block checks
}

// Assertion is a check. Two forms:
//   - Path assertion (then blocks): Target + Expected are set. E.g. "from.balance: 70"
//   - Expression assertion (invariants): Expr is set. E.g. "output.from.balance >= 0"
type Assertion struct {
	// Expression assertion field
	Expr Expr // boolean expression (invariants)

	// Path assertion fields
	Expected Expr   // expected value
	Target   string // field path or locator name
	Plugin   string // plugin namespace (from @ syntax)
	Property string // assertion property name
}

// Assignment sets a concrete value: field: value
type Assignment struct {
	Value Expr
	Path  string
}

// Expr is an expression node.
type Expr interface {
	exprNode()
}

type LiteralInt struct{ Value int }
type LiteralString struct{ Value string }
type LiteralBool struct{ Value bool }
type LiteralNull struct{}
type FieldRef struct{ Path string }       // e.g., "input.from.balance", "output.error"
type EnvRef struct{ Var, Default string } // env(VAR) or env(VAR, "default")

type BinaryOp struct {
	Left  Expr
	Right Expr
	Op    string // ==, !=, >, <, >=, <=, +, -, *, &&, ||
}

type UnaryOp struct {
	Operand Expr
	Op      string // !, -
}

type ObjectLiteral struct {
	Fields []*ObjField // ordered key-value pairs
}

type ObjField struct {
	Value Expr
	Key   string
}

type RegexLiteral struct{ Pattern string }

func (LiteralInt) exprNode()    {}
func (LiteralString) exprNode() {}
func (LiteralBool) exprNode()   {}
func (LiteralNull) exprNode()   {}
func (FieldRef) exprNode()      {}
func (BinaryOp) exprNode()      {}
func (UnaryOp) exprNode()       {}
func (ObjectLiteral) exprNode() {}
func (EnvRef) exprNode()        {}
func (RegexLiteral) exprNode()  {}
