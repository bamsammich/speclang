---
name: author
description: "Use when the user describes a feature, requirement, or behavior in natural language that should be formalized into a speclang .spec file. Also use when creating new features, adding functionality, or modifying behavior in a project that has speclang specs (.spec files). Triggers: user says 'write a spec for', 'add a feature', 'build X', or when the project contains .spec files and new functionality is being planned."
---

# Speclang Spec Authoring

Convert natural language requirements into speclang specification files.

## Process

1. Read the language syntax reference: [references/api_reference.md](references/api_reference.md)
2. Understand what the user wants to build
3. Identify the plugin (`http` for APIs, `process` for CLI tools)
4. Write the spec following the structure below

## Spec Writing Checklist

- [ ] If the user has an OpenAPI spec, use `import openapi("path")` to import models and scopes
- [ ] `use <plugin>` at the top
- [ ] `spec <Name>` with a `description` explaining what the system is
- [ ] `target` block with connection config
- [ ] `model` blocks for shared data structures
- [ ] One `scope` per logical operation or endpoint
- [ ] `contract` with typed `input` and `output` in each scope
- [ ] At least one `scenario` with `given` as a concrete smoke test
- [ ] `scenario` with `when` for edge cases that should be tested generatively
- [ ] `invariant` blocks for universal properties (conservation laws, non-negativity, idempotency)
- [ ] Comments (`#`) explaining the intent of each invariant and scenario

## Choosing Scenario Types

| Pattern | Use when | Example |
|---------|----------|---------|
| `given` scenario | Documenting a specific expected behavior (use relational assertions to compute expected values from input) | "Transferring 30 from Alice(100) to Bob(50) should work" |
| `when` scenario | An entire class of inputs should produce the same outcome | "Any amount exceeding balance should fail" |
| `invariant` | A property that must hold universally | "Money is conserved across transfers" |

**Prefer relational assertions in `then` blocks** — write `from.balance: from.balance - amount` instead of `from.balance: 70`. This computes the expected value from the input, so the assertion adapts to any input and resists memorization. Literal values are still supported where appropriate (e.g., `error: null`).

**Prefer invariants over scenarios when possible.** Invariants are the strongest form of verification — they test across the full input space, not just a slice.

## Writing Good Invariants

Invariants express universal truths about the system. Think about:

- **Conservation**: Is anything preserved? (totals, counts, checksums)
- **Monotonicity**: Does something only increase or decrease?
- **Idempotency**: Does repeating an operation change the result?
- **Bounds**: Are there values that should never go negative, exceed a limit, or be null?
- **Error isolation**: On failure, is state left unchanged?

Use `when` guards to scope invariants to relevant conditions:

```
invariant conservation {
  when error == null:
    output.total == input.total
}
```

## Writing Good Constraints

Constraints on model fields bound the input generator. They should reflect real domain rules:

```
amount: int { 0 < amount <= from.balance }
```

This ensures the generator only produces valid transfer amounts, so scenarios and invariants test meaningful inputs.

## File Organization

For small specs, a single file is fine. For larger systems, split by concern:

```
specs/
├── myapp.spec              # root: use, spec name, target, includes
├── models/
│   └── user.spec           # model User { ... }
└── scopes/
    ├── auth.spec            # scope login { ... }
    └── transfer.spec        # scope transfer { ... }
```

## Output

After writing the spec, tell the user how to verify it:

```bash
specrun verify path/to/spec.spec
```

If the spec needs a running server or binary, mention the setup required.
