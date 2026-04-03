# Speclang Syntax Reference

## File Structure

```
include "<path>"                     # optional: top-level include

spec <Name> {

  description: "<text>"              # optional: AI context about the system

  # Adapter configuration (namespaced blocks at spec level)
  http {
    base_url: env(APP_URL, "http://localhost:8080")
  }
  playwright {
    base_url: env(APP_URL, "http://localhost:3000")
    headless: "true"
    timeout: "5000"
  }
  process {
    command: "./my-binary"
  }

  services {                         # optional: Docker containers as test infra
    <name> {
      build: "<dockerfile-dir>"      # OR image: "<docker-image>"
      port: <port>                   # optional
      health: "<http-path>"          # optional
      env { KEY: "value" }           # optional
      volumes { "<host>": "<container>" }  # optional
    }
    # OR: compose: "<path>"          # docker-compose file
  }

  include "<path>"                   # spec-body include

  model <Name> {
    <field>: <type>
    <field>: <type> { <constraint> }
  }

  # Spec-level actions — reusable across scopes
  action <name>(<param>: <type>, ...) {
    let <var> = <adapter>.<method>(<args>)
    <adapter>.<method>(<args>)
    return <expr>
  }

  scope <name> {
    # Scope-level actions — private to this scope
    action <name>(<param>: <type>, ...) {
      let <var> = <adapter>.<method>(<args>)
      return <expr>
    }

    before {                         # optional: runs before each scenario/invariant
      let <var> = <action>(<args>)
      <adapter>.<method>(<args>)
    }

    after {                          # optional: runs after each scenario/invariant, even on failure
      <adapter>.<method>(<args>)     # errors are logged but never affect test results
    }

    contract {
      input {
        <field>: <type>
      }
      output {
        <field>: <type>
      }
      action: <action_name>         # references a scope-level or spec-level action
    }

    invariant <name> {
      when <predicate>:              # optional guard
        <assertion>
    }

    scenario <name> {
      given { ... }                  # concrete values and/or action calls
      when { ... }                   # predicate (generative)
      then { ... }                   # assertions
    }
  }
}
```

## Types

- `int` — integer
- `float` — floating-point number (e.g., `3.14`)
- `string` — string
- `bytes` — binary data (base64-encoded in JSON)
- `bool` — boolean
- `any` — untyped (passed through)
- `[]T` — array/slice of type T (e.g., `[]int`, `[]Account`)
- `map[K, V]` — map with key type K and value type V (e.g., `map[string, int]`)
- `enum("val1", "val2", ...)` — one of a fixed set of string values (e.g., `enum("http", "process", "playwright")`)
- `<ModelName>` — reference to a defined model
- Append `?` for optional: `string?`, `[]int?`, `enum("a", "b")?` (optional)

## Expressions

- **Literals**: `42`, `3.14`, `"hello"`, `'single-quoted'`, `true`, `false`, `null`
- **Field references**: `from.balance`, `output.error`
- **Environment**: `env(VAR)`, `env(VAR, "default")` — works in all positions (adapter config, given values, call args); returns `""` if unset with no default
- **Service reference**: `service(name)` — resolves to running container URL from `services` block
- **Objects**: `{ id: "alice", balance: 100 }`
- **Arrays**: `[expr, expr, ...]` — comma-separated list of expressions of the same type
- **Operators**: `==`, `!=`, `>`, `<`, `>=`, `<=`, `+`, `-`, `*`, `/`, `%`, `&&`, `||`, `!` — the `+` operator also performs string concatenation when either operand is a string (non-string operands are auto-converted: `"count: " + 42` produces `"count: 42"`)
- **Functions**:
  - `len(expr)` — returns length of array, map, or string
  - `all(array, elem => predicate)` — true if predicate holds for every element
  - `any(array, elem => predicate)` — true if predicate holds for at least one element
  - `contains(haystack, needle)` — returns `bool`. String haystack + string needle: substring check. `[]any` haystack + any needle: element membership check.
  - `exists(expr)` — returns `true` if the path resolves to a value (including `null`), `false` if the path doesn't exist
  - `has_key(expr, "key")` — returns `true` if the map contains the specified key, `false` otherwise
- **Conditionals**: `if condition then expr else expr` — condition must be bool; returns then-branch or else-branch. Nest with parentheses: `if a then (if b then x else y) else z`

## Strings

Both double-quoted (`"..."`) and single-quoted (`'...'`) strings are supported. Single-quoted strings are useful for CSS selectors containing double quotes:

```
playwright.fill('[data-testid="email"]', "alice@example.com")
```

## Comments

Lines starting with `#` are comments. Use them to explain intent.

```
# Money is neither created nor destroyed on successful transfers.
invariant conservation { ... }
```

## Adapter Config Blocks

Adapters are configured at the spec level using namespaced blocks. No `use` directive — adapters are called inline as `adapter.method(args)`.

```
spec MyApp {
  http {
    base_url: env(APP_URL, "http://localhost:8080")
  }
  playwright {
    base_url: env(APP_URL, "http://localhost:3000")
    headless: "true"
    timeout: "5000"
  }
  process {
    command: "./my-binary"
  }
}
```

## Custom Actions

Actions are reusable, parameterized blocks that can call adapters and return values. Defined at spec level (shared) or scope level (private).

```
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
```

Usage:
- In `before` blocks: `let session = login("admin", "test")`
- As contract action: `action: create_item`
- Return values captured via `let`

## Let Bindings

Immutable named values. Scoped to the block they appear in.

```
let result = http.post("/api/auth/login", { username: "admin" })
let token = result.body.access_token
http.header("Authorization", "Bearer " + token)
```

No reassignment — once bound, fixed.

## Contract with Action Reference

Contracts declare input/output shapes and reference an action by name. The runtime maps contract input fields to action parameters.

```
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
```

The `action:` field references a scope-level or spec-level action. Signature mismatch is a compile-time error.

## Three Scenario Types (Ascending Verification Strength)

### 1. `scenario` with `given` — Concrete smoke test

Fixed input values. Runs once. Documents expected behavior.

`then` assertions can use **relational expressions** that reference input fields. The expected value is computed from the input, not hardcoded.

```
scenario success {
  given {
    from: { id: "alice", balance: 100 }
    to: { id: "bob", balance: 50 }
    amount: 30
  }
  then {
    from.balance == from.balance - amount   # 100 - 30 = 70
    to.balance == to.balance + amount       # 50 + 30 = 80
    error == null                           # literals still work
  }
}
```

### 2. `scenario` with `when` — Generative predicate

Defines a predicate over the input space. Runtime generates many matching inputs.

```
scenario overdraft {
  when {
    amount > from.balance
  }
  then {
    error == "insufficient_funds"
  }
}
```

### 3. `invariant` — Universal law

Must hold for ALL valid inputs. Optional `when` guard filters to a subspace.

```
invariant conservation {
  when error == null:
    output.from.balance + output.to.balance
      == input.from.balance + input.to.balance
}

invariant non_negative {
  output.from.balance >= 0
  output.to.balance >= 0
}
```

## Assertion Syntax

All assertions use explicit operators. `:` for equality is removed — use `==`.

```
then {
  status == 200                                              # equality
  playwright.count('[data-testid="item"]') >= 1              # relational
  playwright.visible('[data-testid="welcome"]') == true      # adapter method assertion
  score != 0                                                 # inequality
}
```

Every line in `then`/invariant body is implicitly ANDed. No `||` between assertion lines — use separate scenarios for OR logic.

## Constraints

Model fields can have constraints that bound the generator:

```
model Transfer {
  amount: int { 0 < amount <= from.balance }
}
```

## Before Block

A `before` block runs before each scenario's `given` and each invariant iteration. The adapter state is reset to a clean slate before `before` executes, ensuring isolation between iterations.

```
scope create_group {
  before {
    let session = login("admin", "test")
    http.header("Authorization", "Bearer " + session.access_token)
  }

  contract { ... }
}
```

**Composition:** `before` + `given` compose by concatenation — before steps run first, then given steps. State established in `before` (headers, cookies) carries into `given` and the action.

**Failure:** If any `before` step fails, the entire scope is aborted.

**Reset:** Each iteration starts with a clean adapter state — fresh HTTP client, empty headers, new cookie jar. For Playwright, cookies and localStorage are cleared.

## After Block

An `after` block is the teardown counterpart to `before`. It runs after every scenario and invariant iteration, including iterations that fail.

```
scope create_group {
  after {
    http.delete("/api/cleanup")
  }

  contract { ... }
}
```

**Always runs:** `after` executes even when the scenario or invariant iteration fails — safe for cleanup that must happen regardless of outcome.

**Errors are logged, not fatal:** If an `after` step fails, the error is logged but does not affect the pass/fail result. A failing `after` block will never turn a passing test into a failure.

## Mixed `given` Block Syntax

`given` blocks accept both **data assignments** and **action calls**, interleaved in any order. Steps execute in the order written.

```
# HTTP multi-step example
given {
  http.header("Authorization", "Bearer token")
  http.post("/api/items", { name: "widget" })
  http.get("/api/items/1")
}

# Playwright example
given {
  playwright.fill('[data-testid="amount"]', "50")
  from_balance: 100
  playwright.click('[data-testid="transfer"]')
  to_id: "bob"
}
```

You can also call named actions in `given` blocks:

```
given {
  let session = login("alice", "secret")
  user: "alice"
}
```

Array literals can be used in `given` blocks to assign to array fields:

```
scenario process_batch {
  given {
    ids: [1, 2, 3]
    names: ["alice", "bob", "charlie"]
  }
  then {
    count == 3
  }
}
```

## Error Assertions (Negative Testing)

Use the `error` pseudo-field in `then` blocks to assert that an action **should fail**. This activates when `error` is NOT declared in the scope's contract output fields.

```
scope bad_input {
  contract {
    input { value: string }
    output { ok: bool }
    action: submit_form
  }

  scenario click_nonexistent {
    given {
      playwright.click('[data-testid="nonexistent"]')
    }
    then {
      error == "element not found"
    }
  }

  scenario success_no_error {
    given {
      playwright.click('[data-testid="submit"]')
    }
    then {
      error == null
    }
  }
}
```

**Important:** If `error` is declared as a contract output field (e.g., `output { error: string? }`), it's treated as a normal response field, not the pseudo-field.

## Include Directive

Split specs across files. Paths are relative to the including file.

```
include "models/account.spec"
include "scopes/transfer.spec"
```

## Import Directive

Import models and scopes from external schema files. Supports OpenAPI 3.x and protobuf.

### OpenAPI

```
import openapi("schema.yaml")
```

Generates models from `components/schemas` and scopes from `paths`. Type mapping: `integer` -> `int`, `string` -> `string`, `boolean` -> `bool`, `$ref` -> model name. Constraints (`minimum`/`maximum`) are converted to field constraint expressions.

### Protobuf

```
import proto("service.proto")
```

Generates models from `message` definitions and scopes from unary `rpc` methods. Type mapping: all integer types -> `int`, `string` -> `string`, `bool` -> `bool`, message reference -> model name.

Unsupported types (array, float, enum, bytes, map) are skipped with a warning in both importers. Imported scopes have contracts populated but no invariants or scenarios — those are hand-authored.

## Validation

Specs are automatically validated after parsing, before generation and verification begin. Validation checks:

- **Model resolution**: All model references exist and are well-defined
- **Type checking**: Literal values match their declared types (e.g., assigning a string to an `int` field)
- **Array element types**: All elements in array literals match the array's element type
- **Object field validation**: Object literals contain only declared fields with matching types
- **Given completeness**: All required contract input fields are assigned in `given` blocks
- **Then field validation**: All fields in `then` blocks are valid model fields or adapter method assertions
- **Action signature matching**: Contract `action:` references match action parameter signatures

**Validation failures are hard errors:** They cause `specrun` to exit with code 1 and print detailed messages. Verification does not proceed if validation fails — the user sees validation errors first.

Example error output:

```
error validating spec:
  scope transfer / scenario success:
    given: required field "to" not assigned
    error: type mismatch: expected int, got string "pending"
```

### `http` adapter

For HTTP APIs. Config uses `base_url`.

Use action calls with explicit paths — each call specifies its own path. Headers and cookies persist across calls. `then` assertions apply to the last response.

#### Methods

| Method | Args | Description |
|--------|------|-------------|
| `http.get(path)` | URL path | GET request |
| `http.post(path, body)` | URL path + JSON body | POST request |
| `http.put(path, body)` | URL path + JSON body | PUT request |
| `http.delete(path)` | URL path | DELETE request |
| `http.header(name, value)` | header name + value | Set persistent header |

#### Assertion Fields

| Property | Type | Description |
|----------|------|-------------|
| `status` | `int` | HTTP status code |
| `body` | `any` | Full response body |
| `header.<name>` | `string` | Response header |
| `<field.path>` | `any` | Dot-path into JSON body |

#### Example

```
scope create_and_verify {
  action create_widget(name: string) {
    let result = http.post("/api/resources", { name: name })
    return result.body
  }

  contract {
    input { name: string }
    output { id: int, name: string }
    action: create_widget
  }

  scenario create_then_get {
    given {
      name: "widget"
    }
    then {
      id == 1
      name == "widget"
    }
  }

  scenario with_auth_header {
    given {
      http.header("Authorization", "Bearer token")
      http.get("/api/protected")
    }
    then {
      status == 200
    }
  }
}
```

### `process` adapter

For CLI tools. Runs subprocesses, captures exit code/stdout/stderr. Config uses `command`. Scope config uses `args` (string or array form).

Config `args` accepts two forms: string (split on whitespace) or array (each element is one argument, preferred). Array form preserves spaces:
```
config { args: "verify --json" }                                    # string form
config { args: ["verify", "--json", "path with spaces/file.spec"] } # array form
```

Dot-paths in `then` blocks support numeric array indexing. `stdout.items.0.name` accesses the first element of the `items` array in the parsed JSON response, then its `name` field. Out-of-range indices produce an assertion failure.

### `playwright` adapter

For browser UI testing. Controls a real browser via Playwright. Config uses `base_url`, `headless` (default `"true"`), and `timeout` (milliseconds, default `"5000"`).

Requires Playwright browsers to be installed: `specrun install playwright`

Selectors are passed as inline string arguments — no `locators` block.

> For the complete syntax reference, see [docs/language-reference.md](../../../docs/language-reference.md).

#### Methods

| Method | Args | Description |
|--------|------|-------------|
| `playwright.goto(url)` | URL string | Navigate (prepends `base_url` if relative) |
| `playwright.click(selector)` | CSS selector string | Click element |
| `playwright.fill(selector, value)` | CSS selector + text | Clear and type into input |
| `playwright.type(selector, value)` | CSS selector + text | Append text (no clear) |
| `playwright.select(selector, value)` | CSS selector + option | Select dropdown option |
| `playwright.check(selector)` | CSS selector | Check checkbox |
| `playwright.uncheck(selector)` | CSS selector | Uncheck checkbox |
| `playwright.wait(selector)` | CSS selector | Wait for element to be visible |
| `playwright.new_page()` | -- | Create a fresh browser page |
| `playwright.close_page()` | -- | Close current page |
| `playwright.clear_state()` | -- | Clear cookies and localStorage |

#### Assertion Methods

| Method | Return type | Description |
|--------|-------------|-------------|
| `playwright.visible(selector)` | `bool` | Element is visible |
| `playwright.text(selector)` | `string` | Text content |
| `playwright.value(selector)` | `string` | Input field value |
| `playwright.checked(selector)` | `bool` | Checkbox state |
| `playwright.disabled(selector)` | `bool` | Whether element is disabled |
| `playwright.count(selector)` | `int` | Number of matching elements |

#### Full Example

```
spec LoginApp {
  description: "Login UI verification"

  http {
    base_url: env(APP_URL, "http://localhost:3000")
  }
  playwright {
    base_url: env(APP_URL, "http://localhost:3000")
    headless: "true"
    timeout: "5000"
  }

  action login(user: string, pass: string) {
    playwright.fill('[data-testid="username"]', user)
    playwright.fill('[data-testid="password"]', pass)
    playwright.click('[data-testid="submit"]')
  }

  scope login {
    action do_login(user: string, pass: string) {
      login(user, pass)
      let welcome_visible = playwright.visible('[data-testid="welcome"]')
      return { ok: welcome_visible }
    }

    contract {
      input {
        user: string
        pass: string
      }
      output {
        ok: bool
      }
      action: do_login
    }

    scenario successful_login {
      given {
        user: "alice"
        pass: "secret"
      }
      then {
        playwright.visible('[data-testid="welcome"]') == true
        playwright.text('[data-testid="welcome"]') == "Welcome, alice"
        playwright.visible('[data-testid="error"]') == false
      }
    }

    scenario invalid_credentials {
      when {
        pass != "secret"
      }
      then {
        playwright.visible('[data-testid="error"]') == true
      }
    }

    # The welcome banner must never appear when login fails.
    invariant no_welcome_on_failure {
      when ok == false:
        playwright.visible('[data-testid="welcome"]') == false
    }
  }
}
```
