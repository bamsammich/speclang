# Speclang v3 Language Design

**Date:** 2026-03-29
**Status:** Approved

## Goals

1. **Variables** — named constants and computed values from action responses (`let`)
2. **Custom actions** — reusable, parameterized, return values, spec-level and scope-level
3. **Mixed adapters** — multiple adapters per scope, named inline per call
4. **Uniform syntax** — `adapter.method(args...)` for all adapter interactions
5. **Legibility** — readable by humans and LLMs with minimal indirection
6. **Explicit assertions** — `==` instead of `:` for equality; implicit AND, no OR
7. **Multi-spec execution** — `specrun verify spec1.spec spec2.spec` / glob patterns
8. **Migration** — `specrun migrate` to auto-convert v2 → v3

## What's Removed

- `use` directive — adapters named inline per call
- `locators` block — selectors are inline string arguments
- `@plugin.property` assertion syntax — replaced by `adapter.method(args...) op value`
- `:` for equality in `then` blocks — `==` required
- `target` block — replaced by namespaced adapter config blocks
- Scope-level `config` block — per-call paths replace `path`/`method` config
- Separate `Action`/`Assert` on adapter interface — unified `Call`

## Spec Structure

```speclang
spec MyApp {
  description: "My application"

  # Adapter configuration (namespaced)
  http {
    base_url: env(APP_URL, "http://localhost:8080")
  }
  playwright {
    base_url: env(APP_URL, "http://localhost:8080")
    headless: true
  }
  process {
    command: "./my-binary"
  }

  # Services
  services {
    app {
      build: "./server"
      port: 8080
    }
  }

  # Models
  model Account { id: string, balance: int }

  # Spec-level actions (reusable across scopes)
  action login(username: string, password: string) {
    let result = http.post("/api/auth/login", { username: username, password: password })
    http.header("Authorization", "Bearer " + result.body.access_token)
    return result.body
  }

  # Includes and imports
  include "scopes/transfer.spec"
  import openapi("schema.yaml")

  # Scopes
  scope transfer { ... }
}
```

## Scope Structure

```speclang
scope transfer {
  # Scope-level actions (private to this scope)
  action transfer(from: Account, to: Account, amount: int) {
    let result = http.post("/api/v1/accounts/transfer", {
      from: from, to: to, amount: amount
    })
    return result.body
  }

  # Prerequisites — runs before every scenario and invariant iteration
  before {
    let session = login("admin", "test")
  }

  # Contract — input shape, output shape, and execution recipe
  contract {
    input {
      from: Account
      to: Account
      amount: int { 0 < amount <= from.balance }
    }
    output {
      from: Account
      to: Account
      error: string?
    }
    action: transfer
  }

  # Invariants — checked against random generated inputs via contract action
  invariant conservation {
    when error == null:
      output.from.balance + output.to.balance == input.from.balance + input.to.balance
  }

  # Scenarios — concrete inputs fed to the same contract action
  scenario success {
    given {
      from: { id: "alice", balance: 100 }
      to: { id: "bob", balance: 50 }
      amount: 30
    }
    then {
      from.balance == from.balance - amount
      to.balance == to.balance + amount
      error == null
    }
  }

  scenario overdraft {
    when {
      amount > from.balance
    }
    then {
      error == "insufficient_funds"
    }
  }
}
```

## Execution Model

### Per invariant iteration:
1. Reset adapter state
2. Run `before` (prerequisites)
3. Generate inputs from contract constraints
4. Run contract `action` with generated inputs
5. Check invariant assertions

### Per scenario:
1. Reset adapter state
2. Run `before` (prerequisites)
3. Take `given` inputs (concrete) or generate from `when` predicate
4. Run contract `action` with inputs
5. Check `then` assertions

### Contract action requirement:
- Scopes with invariants MUST have a contract `action`
- The `action` references a scope-level or spec-level action by name
- The runtime matches contract input fields to action parameters by name
- Signature mismatch is a compile-time validation error

## Variables

```speclang
# Immutable bindings via let
let result = http.post("/api/auth/login", { username: "admin" })
let token = result.body.access_token
http.header("Authorization", "Bearer " + token)

# Scoped to the block they appear in
# No reassignment — once bound, fixed
```

## Custom Actions

```speclang
# Spec-level: reusable across scopes
action login(username: string, password: string) {
  let result = http.post("/api/auth/login", { username: username, password: password })
  http.header("Authorization", "Bearer " + result.body.access_token)
  return result.body
}

# Scope-level: private to the scope
scope my_scope {
  action create_item(name: string) {
    let result = http.post("/api/items", { name: name })
    return result.body
  }
}

# Usage:
# - In before/given blocks: let session = login("admin", "test")
# - As contract action: action: create_item
# - Return values captured via let
```

## Mixed Adapters

Adapters named inline per call. No `use` directive.

```speclang
scope login_flow {
  action authenticate(username: string, password: string) {
    let result = http.post("/api/auth/login", { username: username, password: password })
    http.header("Authorization", "Bearer " + result.body.access_token)
    playwright.goto("/dashboard")
    return result.body
  }

  contract {
    input { username: string, password: string }
    output { authenticated: bool, error: string? }
    action: authenticate
  }

  invariant dashboard_on_success {
    when error == null:
      playwright.visible('[data-testid="dashboard"]') == true
  }

  scenario alice_logs_in {
    given {
      username: "alice"
      password: "secret"
    }
    then {
      authenticated == true
      playwright.text('[data-testid="welcome"]') == "Hello, alice"
    }
  }
}
```

## Uniform Syntax

Three patterns cover every line in every block:

| Pattern | Form | Used in |
|---------|------|---------|
| Assignment | `name: value` | given, config, adapter config |
| Action call | `adapter.method(args...)` | before, given, action bodies |
| Assertion | `expression operator value` | then, invariant bodies |

## Adapter Interface (Go)

```go
type Adapter interface {
    Init(config map[string]string) error
    Call(method string, args json.RawMessage) (*Response, error)
    Reset() error
    Close() error
}
```

Single `Call` method replaces `Action` + `Assert`. Context determines usage:
- In before/given/action bodies: call method, capture response
- In then/invariant bodies: call method, compare response against expected value

## Plugin Definition

```
plugin playwright {
  methods {
    goto(url: string)
    fill(selector: string, value: string)
    click(selector: string)
    visible(selector: string): bool
    text(selector: string): string
    count(selector: string): int
    value(selector: string): string
    disabled(selector: string): bool
  }
}
```

Methods with return types are usable in assertions. Methods without are action-only.

## Assertion Semantics

- `==`, `!=`, `>`, `>=`, `<`, `<=` — explicit operator required
- `:` for equality is removed
- Every line in `then`/invariant body is implicitly ANDed
- No `||` or boolean composition between assertion lines
- If you need OR, use separate scenarios

## Single-Quoted Strings

Added for CSS selectors containing double quotes:

```speclang
playwright.fill('[data-testid="email-input"]', "alice@example.com")
```

## Multi-Spec Execution

```bash
specrun verify spec1.spec spec2.spec
specrun verify specs/*.spec
specrun parse spec1.spec spec2.spec
```

CLI accepts multiple file arguments and glob patterns. Each spec is parsed and verified independently. Results reported per-spec.

## Migration

```bash
specrun migrate v2-spec.spec           # prints v3 to stdout
specrun migrate v2-spec.spec -w        # writes in place
specrun migrate specs/*.spec -w        # batch migrate
```

Mechanical transformations:
- `target { base_url: ... }` → `http { base_url: ... }`
- `use http` + `config { path, method }` → scope-level action + contract `action:`
- `locators { name: [sel] }` → inline selectors in action/assertion calls
- `name@playwright.property: value` → `playwright.property('[sel]') == value`
- `then { field: value }` → `then { field == value }`
