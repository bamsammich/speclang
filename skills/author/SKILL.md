---
name: author
description: "Use when the user describes a feature, requirement, or behavior in natural language that should be formalized into a speclang .spec file. Also use when creating new features, adding functionality, or modifying behavior in a project that has speclang specs (.spec files). Triggers: user says 'write a spec for', 'add a feature', 'build X', or when the project contains .spec files and new functionality is being planned."
---

# Speclang Spec Authoring

Convert natural language requirements into speclang v4 specification files.

Read [docs/writing-specs-that-prove-things.md](../../docs/writing-specs-that-prove-things.md) before writing any spec — it documents the failure modes that make specs useless. A summary of critical anti-patterns is in this file under "Anti-Patterns".

## Process

1. Read the syntax reference: [references/api_reference.md](references/api_reference.md)
2. Understand what the user wants to build
3. Identify the adapter(s) needed (`http` for APIs, `process` for CLI tools, `playwright` for browser UIs)
4. Write the spec following the v4 structure below

## SLO/SLA Framing

A spec is a **measurable system of laws**, not a list of tests. Each contract is a behavioral promise: "for all inputs satisfying these constraints, the system will produce outputs satisfying these properties." Invariants are the laws. Scenarios are supporting evidence. A spec with only scenarios and no invariants is documentation, not verification.

Ask: "What can go wrong?" for each contract, then write invariants that would catch it.

## Spec Structure (v4)

A `.spec` file IS the spec — no `spec Name { }` wrapper. Top-level declarations appear directly in the file:

```
# adapter config at the top level
http {
  base_url: env(APP_URL, "http://localhost:8080")
}

services {
  app { build: "./server", port: 8080, health: "/healthz" }
}

include "shared/models.spec"

config {
  max_transfer: 1_000_000
}

model TransferResult {
  from: Account
  to: Account
  error: string?
}

scope transfers {
  before {
    login("admin", "test")
  }

  contract Transfer -> TransferResult {
    from: Account
    to: Account
    amount: int { 0 < amount <= from.balance }

    action {
      return http.post("/api/v1/accounts/transfer", {
        from: from, to: to, amount: amount
      })
    }

    invariant conservation {
      output.error == null implies
        output.from.balance + output.to.balance == from.balance + to.balance
    }

    scenario success {
      given {
        from: { id: "alice", balance: 100 }
        to: { id: "bob", balance: 50 }
        amount: 30
      }
      then {
        output.from.balance == input.from.balance - amount
        output.to.balance == input.to.balance + amount
        output.error == null
      }
    }
  }
}
```

## Spec Writing Checklist

- [ ] No `spec Name { }` wrapper — file is the spec
- [ ] Adapter config blocks at top level (`http { }`, `playwright { }`, `process { }`)
- [ ] If using Docker: `services` block with container definitions, `service(name)` for URLs
- [ ] `config { }` block for spec-level constants referenced as `config.key`
- [ ] `model` blocks for shared data structures
- [ ] `enum Name { variant1, variant2 }` for named value sets; reference as `EnumName.variant`
- [ ] `include "path"` at top level only (not inside scopes or contracts)
- [ ] Top-level `action` blocks for reusable flows (login, setup) with typed params, `let`, `return`
- [ ] `scope` blocks to group related contracts with shared `before`/`after`
- [ ] Each `contract Name -> ReturnType { fields... action { } invariants... scenarios... }`
- [ ] Contract `action` block: inlines the execution steps, uses `return` to emit output
- [ ] `output.` prefix required for all return-type field references in assertions/invariants
- [ ] Bare field names (`from`, `amount`) reference contract input fields
- [ ] At least one `invariant` per contract covering a universal property
- [ ] At least one `scenario` with `given` as a concrete smoke test
- [ ] `scenario` with `when` for input classes that produce a defined outcome
- [ ] Comments (`#`) explaining the intent of invariants
- [ ] `implies` for conditional properties: `condition implies consequence`
- [ ] `and`, `or`, `not` for logical operators (not `&&`, `||`, `!`)

## Choosing an Adapter

| Adapter | Use when |
|---------|----------|
| `http` | Testing a REST API |
| `process` | Testing a CLI tool or subprocess |
| `playwright` | Testing a browser UI |

A single spec can use multiple adapters. A single scope can mix adapters.

## Contract Structure

In v4 a contract is self-contained:

```
contract Name -> ReturnModel {
  # Input fields — the generator's input space
  field1: type
  field2: type { constraint }

  # Action block — how to execute (uses fields implicitly)
  action {
    let result = http.post("/path", { field1: field1, field2: field2 })
    return result
  }

  # Verification members
  invariant law_name { ... }
  scenario smoke_name { given { ... } then { ... } }
  scenario class_name { when { ... } then { ... } }
}
```

**No `action:` reference to an external action.** The action block is inline inside the contract. Named top-level actions are for `before` blocks and shared setup — not for contract dispatch.

**Contract model inheritance** (for imported schemas):
```
contract Name: InputModel -> ReturnModel {
  constrain { additional_field >= 0 }
  action { ... }
  invariant ...
}
```

## Choosing Scenario Types

| Pattern | Use when | Example |
|---------|----------|---------|
| `given` scenario | Documenting a specific expected behavior | "Transferring 30 from Alice(100) to Bob(50) should succeed" |
| `when` scenario | An entire class of inputs should produce the same outcome | "Any amount exceeding balance must fail" |
| `invariant` | A property that must hold universally | "Money is conserved across transfers" |

**Use `output.` prefix in `then` blocks and invariants.** Bare names refer to contract input fields; `output.<field>` refers to return type fields. Mixing them up is a compile-time error.

**Prefer relational assertions in `then` blocks** — write `output.from.balance == input.from.balance - amount` instead of `output.from.balance == 70`. Relational assertions adapt to any input and resist memorization.

**Prefer invariants over scenarios when possible.** Invariants test across the full input space, not just a slice.

## Field Resolution Rules

| Name form | Refers to |
|-----------|-----------|
| `from`, `amount` | Contract input field |
| `input.from`, `input.amount` | Contract input field (explicit) |
| `output.balance`, `output.error` | Return type field |
| `config.max_transfer` | Spec-level config constant |
| `EnumName.variant` | Named enum variant |

## Writing Good Invariants

Invariants express universal truths. Think about:

- **Conservation**: Is anything preserved? (totals, counts, checksums)
- **Monotonicity**: Does something only increase or decrease?
- **Idempotency**: Does repeating an operation change the result?
- **Bounds**: Values that should never go negative, exceed a limit, or be null
- **Error isolation**: On failure, is state left unchanged?

Use `implies` for conditional properties (preferred over `when` guards in invariants):

```
invariant conservation {
  output.error == null implies
    output.from.balance + output.to.balance == from.balance + to.balance
}

invariant no_mutation_on_error {
  output.error != null implies
    output.from.balance == from.balance
    and output.to.balance == to.balance
}
```

Use `when` guards when you want to assert multiple lines under a condition:

```
invariant settled_state {
  when output.status == "settled":
    output.settled_at != null
    output.amount_due == 0
}
```

Use `all()` and `any()` to assert over array elements:

```
invariant all_items_valid {
  all(output.items, item => item.status != "error")
}
```

## Writing Good Constraints

Constraints on contract fields bound the input generator. They should reflect real domain rules:

```
amount: int { 0 < amount <= from.balance }
```

Cross-field constraints (referencing another field) are supported. The generator respects them when generating inputs.

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

Use `error == null` to assert no error occurred. This only works when `error` is NOT declared in the contract's return type. If `error` IS declared in the return type (e.g., `error: string?`), assert via `output.error`.

## Named Enums

Define a named enum at top level, reference variants with qualified syntax:

```
enum OrderStatus { pending, processing, shipped, delivered, cancelled }

model Order {
  id: string
  status: OrderStatus
  tracking: string when status == OrderStatus.shipped
}

contract GetOrder -> Order {
  id: string
  action { return http.get("/api/orders/" + id) }

  scenario shipped_has_tracking {
    when { output.status == OrderStatus.shipped }
    then { output.tracking != null }
  }
}
```

The `in` operator works with enum variants:

```
when { status in (OrderStatus.pending, OrderStatus.cancelled) }
```

## State-Dependent Fields

Fields that only exist when a condition holds:

```
model Shipment {
  id: string
  status: string
  tracking: string when status == "shipped"
}
```

The generator only includes `tracking` when it generates `status == "shipped"`. The validator enforces that assertions on conditional fields are guarded.

## Let Bindings

```
before {
  let result = http.post("/api/auth/login", { username: "admin", password: "test" })
  http.header("Authorization", "Bearer " + result.body.access_token)
}
```

`let` bindings are immutable and scoped to the block they appear in.

## Custom Actions (for before/setup, not contract dispatch)

```
action login(username: string, password: string) {
  let result = http.post("/api/auth/login", { username: username, password: password })
  http.header("Authorization", "Bearer " + result.body.access_token)
}

scope transfer {
  before {
    login("admin", "test")
  }

  contract Transfer -> TransferResult { ... }
}
```

Top-level actions are reusable across scopes. Scope-level actions are private to the scope.

## Playwright-Specific Guidance

Use CSS selectors as inline string arguments (not named locators from a `locators` block — that is v2 syntax):

```
playwright.fill('[data-testid="username"]', "alice")
playwright.click('[data-testid="submit"]')
```

Single-quoted strings for selectors containing double quotes:

```
playwright.fill('[data-testid="email"]', "alice@example.com")
```

Assertions in `then` blocks:

```
then {
  playwright.visible('[data-testid="welcome"]') == true
  playwright.text('[data-testid="welcome"]') == "Welcome, alice"
  playwright.visible('[data-testid="error"]') == false
}
```

## File Organization

For large systems, split by concern:

```
specs/
├── myapp.spec              # root: adapter configs, services, includes
├── models.spec             # model declarations
└── orders.spec             # contract Transfer -> ..., etc.
```

Run all specs with a glob:

```bash
specrun verify specs/*.spec
```

## Anti-Patterns

See [docs/writing-specs-that-prove-things.md](../../docs/writing-specs-that-prove-things.md) for the full discussion. Key failures:

**1. Invariant that can't fail (references only input or nullability)**
```
# BAD — only checks a constraint that the generator already guarantees
invariant amount_positive {
  amount > 0
}
```
Invariants must reference `output.*`. If the assertion doesn't involve the system's output, it proves nothing about the system.

**2. Assertion on input instead of output in `then` blocks**
```
# BAD — asserts on input field, not what the system returned
then {
  amount == 30
}
# GOOD
then {
  output.from.balance == input.from.balance - amount
}
```

**3. Missing `when` predicate in a generative scenario**
```
# BAD — generator doesn't know what input class to target
scenario overdraft {
  then { output.error != null }
}
# GOOD
scenario overdraft {
  when { amount > from.balance }
  then { output.error != null }
}
```

**4. Action that re-implements the invariant's check**
The contract action should call the system under test and return its output. It should not apply the invariant logic itself — that defeats the point of verification.

**5. Only happy-path invariants, no error-path invariants**
Every contract should have at least one invariant about what happens when the system returns an error or fails. Error isolation is often the most important property.

**6. Scope with zero verification members**
A contract with no invariants and only `given` scenarios is documentation, not verification. Add at least one `invariant` or one `when` scenario.

**7. Composing runnable specs via `include` instead of glob**
`include` is for shared library fragments — models, actions, adapter config. Do NOT include one complete runnable spec into another. Every declaration in the include graph must be unique; two specs that both define `model Account` cannot both be included into a root file. Instead, factor shared models into a library file and run each spec independently:
```bash
specrun verify specs/*.spec
```
Each matched file is its own verification unit. The library file (models only, no contracts) logs "no contracts, skipping" and is harmless in a glob.

## Output

After writing the spec, tell the user how to verify it:

```bash
specrun verify path/to/spec.spec
# or verify all specs in a directory
specrun verify specs/*.spec
```

If the spec declares `services`, Docker must be available. Containers are managed automatically.

For `playwright` specs:

```bash
specrun install playwright
```
