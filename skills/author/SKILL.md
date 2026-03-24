---
name: author
description: "Use when the user describes a feature, requirement, or behavior in natural language that should be formalized into a speclang .spec file. Also use when creating new features, adding functionality, or modifying behavior in a project that has speclang specs (.spec files). Triggers: user says 'write a spec for', 'add a feature', 'build X', or when the project contains .spec files and new functionality is being planned."
---

# Speclang Spec Authoring

Convert natural language requirements into speclang specification files.

## Process

1. Read the language syntax reference: [references/api_reference.md](references/api_reference.md)
2. Understand what the user wants to build
3. Identify the plugin (`http` for APIs, `process` for CLI tools, `playwright` for browser UIs)
4. Write the spec following the structure below

## Spec Writing Checklist

- [ ] If the user has an OpenAPI spec, use `import openapi("path")` to import models and scopes
- [ ] `spec <Name>` with a `description` explaining what the system is
- [ ] `target` block with connection config (plugin-dependent)
- [ ] If using Docker: `services` block in `target` with container definitions, and `service(name)` for URLs
- [ ] For `playwright` specs: `locators` block declaring all named element locators
- [ ] For `playwright` specs: `action` blocks for reusable UI sequences (login, navigation)
- [ ] `model` blocks for shared data structures
- [ ] One `scope` per logical operation, endpoint, or page flow — each with `use <plugin>`
- [ ] `contract` with typed `input` and `output` in each scope
- [ ] At least one `scenario` with `given` as a concrete smoke test
- [ ] `scenario` with `when` for edge cases that should be tested generatively
- [ ] `invariant` blocks for universal properties (conservation laws, non-negativity, idempotency)
- [ ] Comments (`#`) explaining the intent of each invariant and scenario

**Note:** `use <plugin>` goes inside each `scope` block, not at spec level.

## Choosing a Plugin

| Plugin | Use when |
|--------|----------|
| `use http` | Testing a REST API |
| `use process` | Testing a CLI tool or subprocess |
| `use playwright` | Testing a browser UI |

A single spec can have scopes with different plugins. For example, an app spec might have `use http` scopes for its API and `use playwright` scopes for its UI.

## Choosing Scenario Types

| Pattern | Use when | Example |
|---------|----------|---------|
| `given` scenario | Documenting a specific expected behavior (use relational assertions to compute expected values from input) | "Transferring 30 from Alice(100) to Bob(50) should work" |
| `when` scenario | An entire class of inputs should produce the same outcome | "Any amount exceeding balance should fail" |
| `invariant` | A property that must hold universally | "Money is conserved across transfers" |

**Prefer relational assertions in `then` blocks** — write `from.balance: from.balance - amount` instead of `from.balance: 70`. This computes the expected value from the input, so the assertion adapts to any input and resists memorization. Literal values are still supported where appropriate (e.g., `error: null`).

**Prefer invariants over scenarios when possible.** Invariants are the strongest form of verification — they test across the full input space, not just a slice.

## Asserting on Errors (Negative Testing)

Use the `error` pseudo-field in `then` blocks to assert that an action should fail:

```
scenario missing_element {
  given {
    playwright.click(nonexistent)
  }
  then {
    error: "element not found"
  }
}
```

Use `error: null` to assert that no error occurred. This only works when `error` is NOT a contract output field. If `error` IS declared in the output (like `output { error: string? }`), it's treated as a normal response field.

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
    welcome@playwright.visible: false
}
```

## Writing Good Constraints

Constraints on model fields bound the input generator. They should reflect real domain rules:

```
amount: int { 0 < amount <= from.balance }
```

This ensures the generator only produces valid transfer amounts, so scenarios and invariants test meaningful inputs.

## Playwright-Specific Guidance

### Locators

Declare all UI element locators in the spec-level `locators` block. Use descriptive names that match the element's role:

```
locators {
  username_field: [data-testid=username]
  submit_btn:     [data-testid=submit]
  error_msg:      [data-testid=error]
}
```

Prefer `data-testid` attributes over CSS classes or IDs — they're stable across styling changes.

### Action Blocks

Extract repeated UI flows into named `action` blocks:

```
action login(user, pass) {
  playwright.fill(username_field, user)
  playwright.fill(password_field, pass)
  playwright.click(submit_btn)
  playwright.wait(welcome)
}
```

Then call them from `given` blocks:

```
given {
  login("alice", "secret")
  user: "alice"
}
```

### Mixed `given` Blocks

`given` blocks can interleave action calls and data assignments. Steps run in order:

```
given {
  playwright.fill(amount_field, "50")
  from_balance: 100
  playwright.click(transfer_btn)
  to_id: "bob"
}
```

### Assertion Syntax

Use `locator@playwright.property: expected` in `then` blocks:

```
then {
  welcome@playwright.visible: true
  welcome@playwright.text: "Welcome, alice"
  error_msg@playwright.visible: false
}
```

Available assertion properties: `visible`, `text`, `value`, `checked`, `disabled`, `count`, `attribute.<name>`.

## File Organization

For small specs, a single file is fine. For larger systems, split by concern:

```
specs/
├── myapp.spec              # root: spec name, target, locators, includes
├── models/
│   └── user.spec           # model User { ... }
└── scopes/
    ├── auth.spec            # scope login { use playwright ... }
    └── transfer.spec        # scope transfer { use http ... }
```

## Output

After writing the spec, tell the user how to verify it:

```bash
specrun verify path/to/spec.spec
```

If the spec declares `services` in the `target` block, Docker must be available on the host. The containers will be managed automatically by `specrun verify`. If Docker is unavailable, the user must start the servers manually and set `SPECRUN_NO_SERVICES=1`.

If the spec needs a running server or binary without services, mention the setup required. For `playwright` specs, the user also needs:

```bash
specrun install playwright
```
