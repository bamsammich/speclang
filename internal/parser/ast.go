package parser

import "github.com/bamsammich/speclang/v3/pkg/spec"

// AST type aliases — all types are defined in pkg/spec and re-exported here
// for backward compatibility.

type Pos = spec.Pos
type Spec = spec.Spec
type Scope = spec.Scope
type Service = spec.Service
type Target = spec.Target
type Model = spec.Model
type Field = spec.Field
type TypeExpr = spec.TypeExpr
type Contract = spec.Contract
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
type LetBinding = spec.LetBinding
type ReturnStmt = spec.ReturnStmt
type AdapterCall = spec.AdapterCall
