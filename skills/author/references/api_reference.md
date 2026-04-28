# Speclang v4 Syntax Reference

## File Structure

A `.spec` file IS the spec — no `spec Name { }` wrapper.

```
# Top-level adapter config
http { base_url: env(APP_URL, "http://localhost:8080") }
playwright { base_url: env(APP_URL, "http://localhost:3000"), headless: "true", timeout: "5000" }
process { command: "./my-binary" }

# Top-level services (Docker containers)
services {
  <name> {
    build: "<dockerfile-dir>"       # OR image: "<docker-image>"
    compose: "<docker-compose.yml>" # OR build/image (mutually exclusive)
    port: <port>
    health: "<http-path>"
    env { KEY: "value" }
    volumes { "<host>": "<container>" }
  }
}

# Top-level includes (top-level only — not inside scopes or contracts)
include "<path>"

# Top-level constants
config { key: value, max_retries: 3 }

# Top-level named enum
enum <Name> { variant1, variant2, variant3 }

# Top-level model
model <Name> {
  <field>: <type>
  <field>: <type> { <constraint> }
  <field>: <type> when <condition>  # state-dependent field
}

# Top-level action (reusable across scopes)
action <name>(<param>: <type>, ...) {
  let <var> = <adapter>.<method>(<args>)
  <adapter>.<method>(<args>)
  return <expr>
}

# Top-level contract (or inside a scope)
contract <Name> -> <ReturnModel> {
  # Input fields
  <field>: <type>
  <field>: <type> { <constraint> }

  # Action block (inline — no external action reference)
  action {
    let <var> = <adapter>.<method>(<args>)
    return <expr>
  }

  invariant <name> { <assertions> }
  scenario <name> { given { ... } then { ... } }
  scenario <name> { when { ... } then { ... } }
}

# Contract with model inheritance (for imported schemas)
contract <Name>: <InputModel> -> <ReturnModel> {
  constrain { <additional constraints on inherited fields> }
  action { ... }
  invariant ...
}

# Scope (optional grouping — shared before/after across contracts)
scope <name> {
  before { <steps> }    # runs before each scenario/invariant iteration
  after { <steps> }     # runs after each iteration, even on failure; errors logged but not fatal

  contract <Name> -> <ReturnModel> { ... }
  contract <Name> -> <ReturnModel> { ... }
}
```

## Types

| Type | Description |
|------|-------------|
| `int` | Integer |
| `float` | Floating-point |
| `string` | String |
| `bytes` | Binary data (base64 in JSON) |
| `bool` | Boolean |
| `any` | Untyped (passed through) |
| `[]T` | Array of T (e.g., `[]int`, `[]Account`) |
| `map[K, V]` | Map (e.g., `map[string, int]`) |
| `enum("a", "b")` | Inline anonymous enum (string comparison) |
| `<EnumName>` | Named enum type (`enum Role { admin, user }`) |
| `<ModelName>` | Model reference |
| `T?` | Optional: `string?`, `[]int?`, `Role?` |

## Expressions

**Literals**: `42`, `1_000_000`, `3.14_159`, `"hello"`, `'single-quoted'`, `true`, `false`, `null`

**Field references**:
- `amount`, `from` — contract input field
- `input.from`, `input.amount` — contract input field (explicit)
- `output.balance`, `output.error` — return type field
- `config.max_transfer` — spec-level config constant
- `EnumName.variant` — named enum variant

**Environment**: `env(VAR)`, `env(VAR, "default")` — returns `""` if unset and no default

**Service reference**: `service(name)` — resolves to running container URL

**Objects**: `{ id: "alice", balance: 100 }`

**Arrays**: `[expr, expr, ...]`

**Operators** (in precedence order, lowest to highest):
| Operator | Description |
|----------|-------------|
| `implies` | Logical implication (lowest) |
| `or` | Logical or |
| `and` | Logical and |
| `not` | Logical not (prefix) |
| `==`, `!=`, `<`, `<=`, `>`, `>=` | Comparison |
| `in` | Membership: `x in (a, b, c)` |
| `+`, `-` | Arithmetic; `+` also concatenates strings |
| `*`, `/`, `%` | Arithmetic |

Chained comparisons: `0 < amount <= from.balance`

**Built-in functions**:
| Function | Description |
|----------|-------------|
| `len(expr)` | Length of array, map, or string |
| `all(array, elem => pred)` | True if pred holds for every element |
| `any(array, elem => pred)` | True if pred holds for at least one element |
| `contains(haystack, needle)` | Substring (strings) or membership (arrays) |
| `exists(expr)` | True if path resolves (including to null) |
| `has_key(expr, "key")` | True if map contains key |

**Conditionals**: `if condition then expr else expr`

## Comments

```
# Comment line
```

## Adapter Config Blocks

At top level — no `use` directive:

```
http { base_url: env(APP_URL, "http://localhost:8080") }
playwright { base_url: env(APP_URL, "http://localhost:3000"), headless: "true", timeout: "5000" }
process { command: "./my-binary" }
```

## Contract

```
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

  scenario overdraft {
    when { amount > from.balance }
    then { output.error == "insufficient_funds" }
  }
}
```

## Invariant

```
invariant <name> {
  <assertion>
  <assertion>
}
```

Lines are implicitly ANDed. Use `implies` for conditional properties:

```
invariant no_mutation_on_error {
  output.error != null implies
    output.from.balance == from.balance
    and output.to.balance == to.balance
}
```

Use `when` guard for multi-line conditional blocks:

```
invariant settled_fields {
  when output.status == "settled":
    output.settled_at != null
    output.amount_due == 0
}
```

## Scenario Types

| Type | Runs | When to use |
|------|------|-------------|
| `given` (concrete) | Once | Smoke test, fixed known input |
| `when` (generative) | N times (default 100) | Input class → outcome |
| `invariant` | N times (all valid inputs) | Universal law |

## `then` Block Assertions

```
then {
  output.status == 200
  output.error == null
  playwright.visible('[data-testid="welcome"]') == true
  output.items.0.name == "first"
}
```

Assertions use `==`, `!=`, `<`, `<=`, `>`, `>=`. No `:` for equality.

## Error Assertions

`error` pseudo-field (when `error` is NOT in the return type):

```
then { error == "element not found" }
then { error == null }
```

## `given` Block

Accepts data assignments and action calls, interleaved:

```
given {
  from: { id: "alice", balance: 100 }
  to: { id: "bob", balance: 50 }
  amount: 30
}
```

```
given {
  http.header("Authorization", "Bearer token")
  http.get("/api/items/1")
  name: "widget"
}
```

## `before` / `after` Blocks

```
scope transfer {
  before {
    let session = login("admin", "test")
    http.header("Authorization", "Bearer " + session.access_token)
  }

  after {
    http.delete("/api/cleanup")   # errors logged, never fatal
  }

  contract Transfer -> TransferResult { ... }
}
```

## Named Enums

```
enum Role { admin, user, viewer }

model User { role: Role }

# In assertions
then { output.role == Role.admin }

# In when predicates
when { role in (Role.admin, Role.viewer) }
```

## State-Dependent Fields

```
model Shipment {
  status: string
  tracking: string when status == "shipped"
}
```

## Config Block

```
config {
  max_transfer: 1_000_000
  api_version: "v2"
}

# Referenced in expressions
amount: int { amount <= config.max_transfer }
```

## Include Directive

Top-level only (not inside scopes or contracts):

```
include "shared/models.spec"
include "shared/infra.spec"
```

Include-once: diamond includes are safe (second include silently skipped).

## Import Directive

```
import openapi("api.yaml")   # generates models + contracts with HTTP action
import proto("service.proto") # generates models + contracts (action nil — user fills in)
```

## Services Block

```
services {
  app {
    build: "./server"
    port: 8080
    health: "/healthz"
  }
  db {
    image: "postgres:15"
    port: 5432
    env { POSTGRES_PASSWORD: "secret" }
  }
}
```

Use `service(name)` anywhere a URL is expected: `base_url: service(app)`.

## HTTP Adapter Methods

| Method | Args | Description |
|--------|------|-------------|
| `http.get(path)` | URL path | GET |
| `http.post(path, body)` | path + JSON body | POST |
| `http.put(path, body)` | path + JSON body | PUT |
| `http.delete(path)` | URL path | DELETE |
| `http.header(name, value)` | name + value | Set persistent header |

HTTP assertion fields: `output.status` (int), `output.body` (any), `output.header.<name>` (string), `output.<field.path>` (dot-path into JSON body), `output.items.0.name` (array index).

## Process Adapter

Config: `process { command: "./my-binary" }`. Action: `process.exec(arg, arg, ...)`.

Assertion fields: `output.exit_code` (int), `output.stdout` (any), `output.stderr` (string), `output.stdout.<field.path>` (dot-path).

## Playwright Adapter

Config: `playwright { base_url: "...", headless: "true", timeout: "5000" }`.

Selectors are inline CSS strings — no `locators` block:

```
playwright.fill('[data-testid="username"]', user)
playwright.click('[data-testid="submit"]')
```

| Action | Description |
|--------|-------------|
| `playwright.goto(url)` | Navigate |
| `playwright.click(selector)` | Click |
| `playwright.fill(selector, value)` | Clear and type |
| `playwright.type(selector, value)` | Append text |
| `playwright.select(selector, value)` | Select dropdown |
| `playwright.check(selector)` | Check checkbox |
| `playwright.uncheck(selector)` | Uncheck checkbox |
| `playwright.wait(selector)` | Wait for visible |
| `playwright.new_page()` | Fresh page |
| `playwright.close_page()` | Close current page |
| `playwright.clear_state()` | Clear cookies and localStorage |

| Assertion method | Type | Description |
|-----------------|------|-------------|
| `playwright.visible(selector)` | `bool` | Element is visible |
| `playwright.text(selector)` | `string` | Text content |
| `playwright.value(selector)` | `string` | Input value |
| `playwright.checked(selector)` | `bool` | Checkbox state |
| `playwright.disabled(selector)` | `bool` | Disabled state |
| `playwright.count(selector)` | `int` | Count of matching elements |

## CLI Commands

```bash
specrun verify path/to/spec.spec          # verify one spec
specrun verify specs/*.spec               # verify all specs (glob)
specrun verify specs/**/*.spec            # recursive glob
specrun parse path/to/spec.spec           # parse → AST JSON (exit 0/1)
specrun generate path/to/spec.spec --scope <name> [--seed N]
specrun migrate --to v4 path/to/spec.spec [-w]
specrun install playwright
```

## Validation

Validation runs automatically before verification. Hard errors on:
- Unresolved model/enum references
- Type mismatches (literal vs. declared type)
- `output.` references that don't match the return type
- Action signature mismatches
- Given completeness (all required fields assigned)

## Construct Placement

| Construct | Lives in |
|-----------|----------|
| `include` | Top level only |
| `model` | Top level |
| `enum` | Top level |
| `config` | Top level |
| `action` | Top level or scope |
| `scope` | Top level |
| `contract` | Top level or inside scope |
| `invariant` | Inside contract |
| `scenario` | Inside contract |
| `before` / `after` | Inside scope |
