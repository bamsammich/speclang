package v2parser

import "github.com/bamsammich/speclang/v4/pkg/spec"

// Re-export stable types that are unchanged between v2 and v4.
type Pos = spec.Pos
type Service = spec.Service
type Target = spec.Target
type Model = spec.Model
type Field = spec.Field
type TypeExpr = spec.TypeExpr
type Action = spec.Action
type Param = spec.Param
type Call = spec.Call
type Invariant = spec.Invariant
type Scenario = spec.Scenario
type GivenStep = spec.GivenStep
type Block = spec.Block
type Assertion = spec.Assertion
type Assignment = spec.Assignment
type Expr = spec.Expr
type LiteralInt = spec.LiteralInt
type LiteralFloat = spec.LiteralFloat
type LiteralString = spec.LiteralString
type LiteralBool = spec.LiteralBool
type LiteralNull = spec.LiteralNull
type FieldRef = spec.FieldRef
type EnvRef = spec.EnvRef
type ServiceRef = spec.ServiceRef
type BinaryOp = spec.BinaryOp
type UnaryOp = spec.UnaryOp
type ObjectLiteral = spec.ObjectLiteral
type ObjField = spec.ObjField
type ArrayLiteral = spec.ArrayLiteral
type LenExpr = spec.LenExpr
type AllExpr = spec.AllExpr
type AnyExpr = spec.AnyExpr
type ContainsExpr = spec.ContainsExpr
type ExistsExpr = spec.ExistsExpr
type HasKeyExpr = spec.HasKeyExpr
type RegexLiteral = spec.RegexLiteral
type IfExpr = spec.IfExpr
type ActionDef = spec.ActionDef

// V2-specific types: these had different structure in v2 that was redesigned in v4.

// Spec is the v2 top-level AST node.
type Spec struct {
	Pos            spec.Pos
	Name           string
	Description    string
	AdapterConfigs map[string]map[string]spec.Expr
	Services       []*spec.Service
	Locators       map[string]string
	Target         *spec.Target
	Models         []*spec.Model
	Actions        []*ActionDef
	Scopes         []*Scope
}

// Scope is the v2 scope. In v2, scopes used a 'use' directive for the adapter.
type Scope struct {
	Pos        spec.Pos
	Name       string
	Use        string
	Config     map[string]spec.Expr
	Before     *spec.Block
	After      *spec.Block
	Contract   *Contract
	Actions    []*ActionDef
	Invariants []*spec.Invariant
	Scenarios  []*spec.Scenario
}

// Contract is the v2/v3 contract with input/output field blocks and action name.
type Contract struct {
	Pos    spec.Pos
	Input  []*spec.Field
	Output []*spec.Field
	Action string
}
