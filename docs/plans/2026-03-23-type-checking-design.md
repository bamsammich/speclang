# Compile-Time Type Checking for Literal Expressions

**Date:** 2026-03-23
**Issue:** #30

## Problem

Array literals and object literals in `given`/`then` blocks are not validated against contract types. Type names in contracts are not validated against known models. Given blocks are not checked for completeness. Then blocks are not checked against output fields. All of these are silent failures that only surface at runtime (or not at all).

## Decision

Post-parse validation pass. Hard errors (no warnings, no opt-out). Full scope: literal type checking + model resolution + given completeness + then field validation.

## Design

### New Package: `pkg/validator`

A `Validate(*parser.Spec) []error` function that runs after parsing and before generation/execution. Returns all errors found (not just the first).

### Validation Checks

Six checks, in dependency order:

1. **Model resolution** — every `TypeExpr.Name` that isn't a primitive (`int`, `string`, `bool`, `float`, `bytes`, `array`, `map`) must match a model in `Spec.Models`.

2. **Literal type matching** — every `Assignment.Value` in `given` and `then` blocks is checked against the corresponding contract field type:
   - `LiteralInt` must match `int`
   - `LiteralString` must match `string`
   - `LiteralBool` must match `bool`
   - `LiteralFloat` must match `float`
   - `LiteralNull` only valid for optional fields (`Optional: true`)
   - `ArrayLiteral` must match an `array` type
   - `ObjectLiteral` must match a model type

3. **Array element type checking** — each element in an `ArrayLiteral` is checked recursively against the array's `ElemType`.

4. **Object field checking** — each field in an `ObjectLiteral` is checked against the referenced model's fields: field names must exist in the model, and values must match the model field's type. Extra fields are errors; missing fields are not errors (partial objects may be valid).

5. **Given completeness** — every required (non-optional) contract input field must have an assignment in `given` blocks that use concrete values only (no calls). Skipped for `when`-predicate scenarios.

6. **Then field validation** — every path assertion target in `then` blocks must correspond to a contract output field (or a dot-path into one).

### Error Format

Hierarchical, grouped by scope then scenario:

```
validation errors:

  scope create_order:
    contract:
      - field "items": unknown type "Itme"
    scenario smoke_test:
      - field "items": expected []Item, got string literal
      - field "items[0]": unknown field "colour" in model Item
      - missing required field "customer_id"

  scope transfer:
    scenario basic:
      - then target "balnce" does not match any output field
```

### Integration Points

- `specrun parse` — call `Validate()` after `Parse()`, print errors to stderr, exit 1
- `specrun verify` — same, before runner execution
- `specrun generate` — same, before generation
- Self-verification specs must pass validation

### What It Does NOT Do

- No type checking of `when`-predicate expressions (boolean expressions, not literal assignments)
- No type checking of constraint expressions (evaluated at generation time)
- No type checking of action call arguments (adapter-specific)
- Missing object fields are not errors (partial objects are valid)

## Testing

1. Unit tests — inline specs exercising each check
2. Multi-error test — all errors collected, not just the first
3. Valid specs pass — existing examples and self-verification specs produce zero errors
4. Integration — `specrun parse` exits non-zero on invalid specs
