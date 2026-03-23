# Array Literals in Expressions

**Date:** 2026-03-22
**Issue:** #29

## Problem

Array literals (`[...]`) are not supported in expressions. This makes it impossible to write concrete `given` scenarios for scopes with array-typed contract inputs. The parser errors with `unexpected token LBracket in expression`.

## Decision

Approach A: Minimal — parser + eval only. Follow the established `ObjectLiteral` pattern exactly.

Type checking of literals against contract types (for both arrays and objects) is deferred to a separate issue.

## Design

### AST (`pkg/parser/ast.go`)

Add `ArrayLiteral` node:

```go
type ArrayLiteral struct {
    Elements []Expr
}
```

Satisfies `Expr` via `exprNode()`. Elements can be any expression.

### Parser (`pkg/parser/parser.go`)

Add `case TokenLBracket:` in `parseAtom` calling `parseArrayLiteral`:

1. Consume `[`
2. Parse comma-separated `parseExpr()` calls (supports trailing comma)
3. Handle empty arrays `[]`
4. Consume `]`

Mirrors `parseObjectLiteral` structurally.

### Generator Eval (`pkg/generator/generator.go`)

Add `*ArrayLiteral` case in `evalCtx.eval`:

```go
case *ArrayLiteral:
    result := make([]any, len(e.Elements))
    for i, elem := range e.Elements {
        result[i] = ctx.eval(elem)
    }
    return result
```

### No Changes Needed

- **Lexer** — `[`, `]`, `,` already tokenized
- **HTTP/Process adapters** — JSON serialization handles `[]any` natively
- **Shrinker** — operates on generated values, not literals
- **Runner** — calls `eval` transparently

## Testing

1. **Parser tests** — `[1, 2, 3]`, `[]`, nested arrays, arrays of objects, trailing comma
2. **Generator eval tests** — `evalCtx.eval` produces correct `[]any`
3. **Integration** — spec fixture with array literals in `given`, run end-to-end

## Follow-up

File issue for compile-time type checking of literals against contract types (arrays and objects).
