---
name: author
description: "Use when the user describes a feature, requirement, or behavior in natural language that should be formalized into a speclang .spec file. Also use when creating new features, adding functionality, or modifying behavior in a project that has speclang specs (.spec files). Triggers: user says 'write a spec for', 'add a feature', 'build X', or when the project contains .spec files and new functionality is being planned."
---

# Speclang Spec Authoring

Convert natural language requirements into speclang specification files.

## Process

1. Read the language syntax reference: [references/api_reference.md](references/api_reference.md)
2. Understand what the user wants to build
3. Identify the adapter(s) needed (`http` for APIs, `process` for CLI tools, `playwright` for browser UIs)
4. Write the spec following the structure below

## Spec Writing Checklist

- [ ] If the user has an OpenAPI spec, use `import openapi("path")` to import models and scopes
- [ ] `spec <Name>` with a `description` explaining what the system is
- [ ] Adapter config blocks at spec level (`http {}`, `playwright {}`, `process {}`) with connection config
- [ ] If using Docker: `services` block with container definitions, and `service(name)` for URLs
- [ ] `model` blocks for shared data structures
- [ ] Spec-level `action` blocks for reusable flows (login, setup) with typed params, `let`, `return`
- [ ] One `scope` per logical operation, endpoint, or page flow
- [ ] Scope-level `action` blocks for scope-private logic
- [ ] `contract` with typed `input`, `output`, and `action:` referencing a named action
- [ ] At least one `scenario` with `given` as a concrete smoke test
- [ ] `scenario` with `when` for edge cases that should be tested generatively
- [ ] `invariant` blocks for universal properties (conservation laws, non-negativity, idempotency)
- [ ] Comments (`#`) explaining the intent of each invariant and scenario

**Note:** Adapters are configured at spec level and called inline as `adapter.method(args)` — there is no `use` directive.

## Choosing an Adapter

| Adapter | Use when |
|---------|----------|
| `http` | Testing a REST API |
| `process` | Testing a CLI tool or subprocess |
| `playwright` | Testing a browser UI |

A single spec can use multiple adapters. A single scope can mix adapters — call `http.post(...)` and `playwright.goto(...)` in the same action.

## Choosing Scenario Types

| Pattern | Use when | Example |
|---------|----------|---------|
| `given` scenario | Documenting a specific expected behavior (use relational assertions to compute expected values from input) | "Transferring 30 from Alice(100) to Bob(50) should work" |
| `when` scenario | An entire class of inputs should produce the same outcome | "Any amount exceeding balance should fail" |
| `invariant` | A property that must hold universally | "Money is conserved across transfers" |

**Prefer relational assertions in `then` blocks** — write `from.balance == from.balance - amount` instead of `from.balance == 70`. This computes the expected value from the input, so the assertion adapts to any input and resists memorization. Literal values are still supported where appropriate (e.g., `error == null`).

**Prefer invariants over scenarios when possible.** Invariants are the strongest form of verification — they test across the full input space, not just a slice.

## Asserting on Errors (Negative Testing)

Use the `error` pseudo-field in `then` blocks to assert that an action should fail:

```
scenario missing_element {
  given {
    playwright.click('[data-testid="nonexistent"]')
  }
  then {
    error == "element not found"
  }
}
```

Use `error == null` to assert that no error occurred. This only works when `error` is NOT a contract output field. If `error` IS declared in the output (like `output { error: string? }`), it's treated as a normal response field.

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

Use `all()` and `any()` to assert over array elements:

```
invariant all_items_valid {
  all(output.items, item => item.status != "error")
}

invariant has_primary {
  any(output.items, item => item.primary == true)
}
```

For UI specs, invariants over visible state are also useful:

```
invariant no_welcome_on_failure {
  when ok == false:
    playwright.visible('[data-testid="welcome"]') == false
}
```

## Writing Good Constraints

Constraints on model fields bound the input generator. They should reflect real domain rules:

```
amount: int { 0 < amount <= from.balance }
```

This ensures the generator only produces valid transfer amounts, so scenarios and invariants test meaningful inputs.

## Custom Actions

Actions are reusable, parameterized, and can return values. Define them at spec level (shared across scopes) or scope level (private).

```
# Spec-level action — reusable across scopes
action login(username: string, password: string) {
  let result = http.post("/api/auth/login", { username: username, password: password })
  http.header("Authorization", "Bearer " + result.body.access_token)
  return result.body
}

# Scope-level action — private to scope
scope transfer {
  action transfer(from: Account, to: Account, amount: int) {
    let result = http.post("/api/v1/accounts/transfer", {
      from: from, to: to, amount: amount
    })
    return result.body
  }
}
```

Key points:
- `let` creates immutable bindings — capture adapter responses, extract fields
- `return` sends a value back to the caller
- Actions can call other actions: `let session = login("admin", "test")`
- Contract `action:` references an action by name — the runtime maps contract input fields to action parameters

## Let Bindings

Use `let` to name intermediate values in `before` blocks and action bodies:

```
before {
  let session = login("admin", "test")
  let token = session.access_token
  http.header("Authorization", "Bearer " + token)
}
```

`let` bindings are:
- Immutable (no reassignment)
- Scoped to the block they appear in
- Available in subsequent lines of the same block

## Playwright-Specific Guidance

### Selectors

Use inline CSS selectors as string arguments. Prefer `data-testid` attributes — they're stable across styling changes. Single-quoted strings work for selectors containing double quotes:

```
playwright.fill('[data-testid="username"]', "alice")
playwright.click('[data-testid="submit"]')
```

### Mixed Adapter Scopes

A scope can mix adapters freely. For example, authenticate via HTTP then verify the UI:

```
action authenticate(username: string, password: string) {
  let result = http.post("/api/auth/login", { username: username, password: password })
  http.header("Authorization", "Bearer " + result.body.access_token)
  playwright.goto("/dashboard")
  return result.body
}
```

### Assertion Syntax

Use `adapter.method(args) == expected` in `then` and invariant blocks:

```
then {
  playwright.visible('[data-testid="welcome"]') == true
  playwright.text('[data-testid="welcome"]') == "Welcome, alice"
  playwright.visible('[data-testid="error"]') == false
}
```

Available assertion methods: `visible`, `text`, `value`, `checked`, `disabled`, `count`, `attribute.<name>`.

## File Organization

For small specs, a single file is fine. For larger systems, split by concern:

```
specs/
├── myapp.spec              # root: spec name, adapter configs, includes
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

If the spec declares `services`, Docker must be available on the host. The containers will be managed automatically by `specrun verify`.

If the spec needs a running server or binary without services, mention the setup required. For `playwright` specs, the user also needs:

```bash
specrun install playwright
```
