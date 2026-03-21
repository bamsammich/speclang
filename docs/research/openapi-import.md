# OpenAPI Import Investigation

**Date:** 2026-03-21
**Issue:** #9, sub-issues #11, #12, #13

## Findings

OpenAPI schema import is feasible via a language-level `import openapi "path"` directive.

### Architecture

- Syntax: `openapi("schema.yaml")` — follows `verb(args)` calling convention
- Resolved at parser level (not token level like `include`) since it produces AST nodes
- New package `pkg/openapi/` handles YAML/JSON parsing and AST node generation
- Produces standard `*Model` and `*Scope` nodes — downstream (generator, runner, adapter) unaware
- Uses [kin-openapi](https://github.com/getkin/kin-openapi) for OpenAPI parsing and $ref resolution (instead of custom structs)

### Type Mapping

| OpenAPI | Speclang | Status |
|---------|----------|--------|
| `integer` | `int` | Supported |
| `string` | `string` | Supported |
| `boolean` | `bool` | Supported |
| `$ref` | model name | Supported |
| `number` (float) | — | Gap: no float type |
| `array` | — | Gap: no array type |
| `enum` | — | Gap: no enum type |
| `oneOf`/`anyOf`/`allOf` | — | Gap: no union types |

### Constraint Mapping

- `minimum`/`maximum` → `BinaryOp` constraints (supported)
- `exclusiveMinimum`/`exclusiveMaximum` → strict comparison `BinaryOp` (supported)
- `minLength`/`maxLength` → not supported (no string length in type system)
- `pattern` → `RegexLiteral` exists in AST but parser doesn't wire it yet

### Key Files

- Lexer: `pkg/parser/lexer.go` — add `TokenOpenAPI` keyword
- Parser: `pkg/parser/parser.go` — add `parseOpenAPI()` in `parseSpecMember()`
- Include system: `pkg/parser/include.go` — pattern reference (token-level splicing)
- AST: `pkg/parser/ast.go` — no changes needed, uses existing types
- Duplicate validation: `pkg/parser/include.go:validateNoDuplicates()` — already catches collisions

### Implementation Order

1. #11 — `import` keyword + parser dispatch
2. #12 — `pkg/openapi/` converter (YAML→AST)
3. #13 — End-to-end wiring, examples, docs, skills
