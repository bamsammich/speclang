# Language Reference

Complete syntax reference for the speclang specification language (v3).

## Spec Structure

Every spec file contains a single `spec` block:

```
spec <Name> {
  description: "<text>"              # optional

  http { ... }                       # adapter configuration (one block per adapter)
  playwright { ... }
  process { ... }

  services { ... }                   # Docker containers as test infrastructure

  model <Name> { ... }              # data models
  action <name>(<param>: <type>, ...) { ... }  # reusable actions
  scope <name> { ... }              # test scopes

  include "<path>"                   # splice another file
  import openapi("<path>")           # import from OpenAPI schema
  import proto("<path>")             # import from protobuf schema
}
```

## Include Directive

```
include "models/account.spec"
include "scopes/transfer.spec"
```

Splices the contents of another file at the point of inclusion. The included file's tokens are inserted directly into the token stream, so the content must be syntactically valid at that position.

- Paths are relative to the including file's directory
- Recursive includes are supported (A includes B which includes C)
- Circular includes are detected and produce an error
- Duplicate model or scope names across included files produce an error
- Can appear at top-level (before `spec`) or inside a `spec` block

## Import Directive

```
import openapi("schema.yaml")
import proto("service.proto")
```

Imports models and scopes from an external schema file. The adapter name determines the format.

- Paths are relative to the spec file's directory
- Imported scopes have contracts populated but no invariants or scenarios
- Duplicate model or scope names between imported and hand-written produce an error

See [OpenAPI Import](imports/openapi.md) and [Protobuf Import](imports/protobuf.md) for adapter-specific details.

## Adapter Configuration

Each adapter used in the spec gets a namespaced configuration block at the spec level. Only declare blocks for adapters you use.

```
spec MyApp {
  http {
    base_url: env(APP_URL, "http://localhost:8080")
  }
  playwright {
    base_url: env(APP_URL, "http://localhost:8080")
    headless: true
    timeout: "5000"
  }
  process {
    command: "./my-binary"
    args: ["verify", "--json"]
  }
}
```

Common configuration keys:

| Key | Adapter | Description |
|-----|---------|-------------|
| `base_url` | http, playwright | Server URL |
| `command` | process | Binary to execute |
| `args` | process | Arguments as `[]string` (preferred) or whitespace-split string |
| `headless` | playwright | `true` or `false` (default `true`) |
| `timeout` | playwright | Milliseconds (default `"5000"`) |

Values can use `env(VAR)` or `env(VAR, "default")` to read environment variables, or `service(name)` to reference a declared service URL.

## Services

The `services` block declares Docker containers as test infrastructure. When services are declared, `specrun verify` manages their full lifecycle: cleanup stale containers, build/pull, start, health-check, run verification, then stop and remove.

### Inline Services

```
spec MyApp {
  services {
    app {
      build: "./server"           # Dockerfile directory (relative to spec file)
      port: 8080                  # container port (host port may differ)
      health: "/healthz"          # HTTP health check path (optional)
      env { PORT: "8080" }        # environment variables (optional)
      volumes {                   # volume mounts (optional)
        "./fixtures": "/data"
      }
    }
    db {
      image: "postgres:16"        # pre-built image (alternative to build)
      port: 5432
      env { POSTGRES_PASSWORD: "test" }
    }
  }

  http {
    base_url: service(app)        # resolves to running container URL
  }
}
```

Each service must have either `build` (path to a directory containing a Dockerfile) or `image` (a Docker image reference). Service fields:

| Field | Required | Description |
|-------|----------|-------------|
| `build` | One of build/image | Dockerfile directory |
| `image` | One of build/image | Pre-built Docker image |
| `port` | No | Container port to expose (static mapping when specified, dynamic when omitted) |
| `health` | No | HTTP path for health check (falls back to TCP port check) |
| `env` | No | Environment variables passed to the container |
| `volumes` | No | Host-to-container volume mounts |

### Compose Support

For multi-service setups, reference a docker-compose file instead of inline definitions:

```
spec MyApp {
  services {
    compose: "docker-compose.yml"
  }

  http {
    base_url: service(app)
  }
}
```

The compose path is relative to the spec file. Service names in `service()` references must match compose service names.

### `service(name)` Resolution

`service(name)` resolves at runtime to `http://localhost:<port>` where `<port>` is the actual mapped host port of the named container. The name must match a service declared in the `services` block; unknown names are rejected during validation.

### Health Checks

- If `health` is specified, an HTTP GET is sent to `http://localhost:<port><health>` until a 200 response is received
- If `health` is not specified, a TCP connection check is performed against the mapped port
- Health checks have a timeout; failure to become healthy causes verification to abort

### Container Lifecycle

1. **Pre-flight cleanup**: Remove any stale containers from previous runs (identified by labels)
2. **Build/pull**: Build images from Dockerfiles or pull pre-built images
3. **Start**: Start containers with port mappings and environment variables
4. **Health check**: Wait for each service to become healthy
5. **Verify**: Run the spec verification
6. **Teardown**: Stop and remove containers (unless `--keep-services`)

### CLI Flags

- `--keep-services` -- leave containers running after verification (useful for debugging)

## Models

```
model Account {
  id: string
  balance: int
  email: string?
  role: enum("admin", "user")
}
```

Models define data structures used in contracts. Fields have a name, type, and optional constraint.

### Types

| Type | Description | Example |
|------|-------------|---------|
| `int` | Integer | `42` |
| `float` | Floating-point number | `3.14` |
| `string` | String | `"hello"` |
| `bytes` | Binary data (base64-encoded in JSON) | |
| `bool` | Boolean | `true`, `false` |
| `any` | Untyped (passed through) | |
| `[]T` | Array of type T | `[]int`, `[]Account` |
| `map[K, V]` | Map with key type K, value type V | `map[string, int]` |
| `enum("a", "b", ...)` | Fixed set of string values | `enum("http", "process")` |
| `<ModelName>` | Reference to a defined model | `Account` |

Append `?` to make any type optional: `string?`, `[]int?`, `enum("a", "b")?`, `Account?`.

### Constraints

Constraints bound the input generator. They appear in braces after the type:

```
model Transfer {
  amount: int { 0 < amount <= from.balance }
  ratio: float { 0.0 < ratio && ratio <= 1.0 }
  name: string { len(name) > 0 && len(name) <= 100 }
  items: []string { len(items) >= 1 }
}
```

Constraints can reference other fields in the same model or parent context. Chained comparisons like `0 < amount <= from.balance` are supported.

## Expressions

Expressions appear in constraints, invariants, `when` predicates, `then` assertions, and `let` bindings.

### Literals

| Literal | Example |
|---------|---------|
| Integer | `42`, `-1`, `0` |
| Float | `3.14`, `0.5` |
| Double-quoted string | `"hello"` |
| Single-quoted string | `'[data-testid="email"]'` |
| Boolean | `true`, `false` |
| Null | `null` |
| Object | `{ id: "alice", balance: 100 }` |
| Array | `[1, 2, 3]`, `["a", "b"]` |

Single-quoted strings exist for CSS selectors and other values containing double quotes. Both string forms are interchangeable.

### Field References

Dot-separated paths into the data:

```
from.balance
output.error
input.from.id
result.body.items.0.name      # numeric segment indexes into array
```

Numeric segments in dot-paths index into arrays by position (zero-based).

### Service References

```
service(app)                  # resolves to URL of named service from services block
```

Resolves at runtime to the URL of a running container declared in the `services` block. The name must match a declared service. See [Services](#services) for details.

### Environment Variables

```
env(APP_URL)                  # returns "" if unset
env(APP_URL, "http://localhost:8080")  # with default
```

`env()` expressions work everywhere: adapter config, given block values, and call arguments. When the variable is unset and no default is provided, the expression evaluates to an empty string.

### Operators

| Operator | Description | Example |
|----------|-------------|---------|
| `+` | Addition / String concatenation | `a + b`, `"hello" + " world"` |
| `-` | Subtraction | `a - b` |
| `*` | Multiplication | `a * b` |
| `/` | Division | `a / b` |
| `%` | Modulo | `a % 2` |
| `==` | Equal | `status == 200` |
| `!=` | Not equal | `error != null` |
| `>` | Greater than | `amount > 0` |
| `<` | Less than | `balance < 1000` |
| `>=` | Greater or equal | `count >= 1` |
| `<=` | Less or equal | `amount <= from.balance` |
| `&&` | Logical AND | `a > 0 && b > 0` |
| `\|\|` | Logical OR | `error != null \|\| status == 200` |
| `!` | Logical NOT | `!exists(field)` |

The `+` operator performs string concatenation when either operand is a string. Non-string operands are automatically converted: `"count: " + 42` produces `"count: 42"`.

Chained comparisons are supported: `0 < amount <= from.balance` is equivalent to `0 < amount && amount <= from.balance`.

### Built-in Functions

| Function | Signature | Description |
|----------|-----------|-------------|
| `len()` | `len(expr) -> int` | Length of string, array, or map |
| `contains()` | `contains(haystack, needle) -> bool` | Substring check (strings) or element membership (arrays) |
| `exists()` | `exists(expr) -> bool` | `true` if path resolves to a value (including `null`), `false` if path doesn't exist |
| `has_key()` | `has_key(expr, "key") -> bool` | `true` if map contains the specified key |
| `all()` | `all(array, elem => predicate) -> bool` | `true` if predicate holds for every element |
| `any()` | `any(array, elem => predicate) -> bool` | `true` if predicate holds for at least one element |

Examples:

```
len(name) > 0
contains(output.tags, "featured")
exists(output.metadata.created_at)
has_key(output.headers, "content-type")
all(output.items, item => item.price > 0)
any(output.users, u => u.role == "admin")
```

### Conditional Expressions

```
if condition then expr else expr
```

The condition must evaluate to a boolean. Nesting is supported with parentheses:

```
if error == null then output.balance else (if error == "retry" then input.balance else 0)
```

## Comments

Lines starting with `#` are comments:

```
# Money is neither created nor destroyed on successful transfers.
invariant conservation { ... }
```

## Variables

`let` bindings create immutable named values. They are scoped to the block they appear in (action body, `before`, `given`).

```
let result = http.post("/api/auth/login", { username: "admin", password: "secret" })
let token = result.body.access_token
http.header("Authorization", "Bearer " + token)
```

- Once bound, a variable cannot be reassigned
- Variables are visible only within the block where they are defined
- The right-hand side can be any expression: a literal, a field reference, an adapter call, or a custom action call

## Actions

Actions are reusable, parameterized sequences of steps. They have typed parameters, can contain `let` bindings and adapter calls, and can return values.

### Spec-Level Actions

Defined at spec level, callable from any scope:

```
action login(username: string, password: string) {
  let result = http.post("/api/auth/login", { username: username, password: password })
  http.header("Authorization", "Bearer " + result.body.access_token)
  return result.body
}
```

### Scope-Level Actions

Defined inside a scope, private to that scope:

```
scope transfer {
  action transfer(from: Account, to: Account, amount: int) {
    let result = http.post("/api/v1/accounts/transfer", {
      from: from, to: to, amount: amount
    })
    return result.body
  }
}
```

### Action Syntax

```
action <name>(<param>: <type>, ...) {
  let <name> = <expr>           # variable binding
  <adapter>.<method>(args...)   # adapter call
  <action_name>(args...)        # call another action
  return <expr>                 # return a value (optional)
}
```

### Calling Actions

Actions are called by name. Return values are captured with `let`:

```
before {
  let session = login("admin", "test")
}

given {
  let order = create_order("widget-1", 3)
  order_id: order.id
}
```

### Contract Action

Scopes with invariants must declare which action the runtime calls with generated inputs. The `action:` field in the contract references a scope-level or spec-level action by name:

```
scope transfer {
  action transfer(from: Account, to: Account, amount: int) {
    let result = http.post("/api/v1/accounts/transfer", {
      from: from, to: to, amount: amount
    })
    return result.body
  }

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
}
```

The runtime matches contract input field names to action parameter names. A signature mismatch is a compile-time validation error.

## Adapter Calls

All adapter interactions use the uniform `adapter.method(args...)` syntax. The adapter name is explicit in every call -- there is no ambient adapter context.

```
http.post("/api/items", { name: "widget" })
http.get("/api/items/123")
http.header("Authorization", "Bearer abc123")
playwright.goto("/dashboard")
playwright.fill('[data-testid="email"]', "alice@example.com")
playwright.click('[data-testid="submit"]')
process.run("echo", ["hello", "world"])
```

Adapter calls can appear in action bodies, `before` blocks, and `given` blocks. In `then` and invariant blocks, adapter calls that return values are used as expressions in assertions.

## Scopes

Scopes are named groupings that own a contract, invariants, and scenarios.

```
scope transfer {
  action transfer(from: Account, to: Account, amount: int) { ... }

  before { ... }
  after { ... }
  contract { ... }
  invariant <name> { ... }
  scenario <name> { ... }
}
```

### Before Block

A `before` block runs before each scenario's `given` and each invariant iteration. The adapter state is reset to a clean slate before `before` executes, ensuring isolation between iterations.

```
scope orders {
  before {
    let session = login("admin", "test")
  }

  contract { ... }
}
```

**Composition:** `before` + `given` compose by concatenation -- before steps run first, then given steps. State established in `before` (headers, cookies) carries into `given` and the action execution.

**Failure:** If any `before` step fails, the entire scope is aborted.

**Reset:** Each iteration starts with a clean adapter state -- fresh HTTP client, empty headers, new cookie jar. For Playwright, cookies and localStorage are cleared.

### After Block

An `after` block is the teardown counterpart to `before`. It runs after every scenario and invariant iteration, including iterations that fail.

```
scope orders {
  before {
    let session = login("admin", "test")
  }

  after {
    http.delete("/api/session")
  }

  contract { ... }
}
```

**Always runs:** `after` executes even when the scenario or invariant iteration fails. This makes it safe for cleanup that must happen regardless of outcome.

**Errors are logged, not fatal:** If an `after` step fails, the error is logged but does not affect the pass/fail result of the scenario or invariant. A failing `after` block will never turn a passing test into a failure.

**Reset:** Like `before`, `after` runs within the iteration's adapter state before that state is discarded.

### Contract

Defines typed input and output for the scope, and the action to execute:

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

The `action:` field is required when the scope contains invariants. It references the action the runtime calls with generated inputs.

### Invariant

A universal law that must hold for ALL valid inputs. The runtime generates inputs from the full contract input space and checks each one.

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

The `when` guard is optional. Without it, the invariant applies to all inputs unconditionally. Multiple assertions can appear in an invariant body -- all must hold.

### Scenario

Three types of scenario, in ascending verification strength:

#### `scenario` with `given` -- Concrete Smoke Test

Fixed input values. Runs once. Documents expected behavior.

```
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
```

Prefer **relational assertions** (`from.balance - amount`) over hardcoded values (`70`). The expected value is computed from the input, making the assertion resistant to memorization.

#### `scenario` with `when` -- Generative Predicate

Defines a predicate over the input space. The runtime generates many matching inputs.

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

#### Mixed `given` Blocks

`given` blocks can interleave data assignments, action calls, and `let` bindings. Steps execute in order:

```
given {
  amount: 1000
  let page = playwright.goto("/transfer")
  playwright.fill('[data-testid="amount"]', amount)
  playwright.click('[data-testid="submit"]')
}
```

Named actions can also be called:

```
given {
  let session = login("alice", "secret")
  user: "alice"
}
```

Array literals work in `given` blocks:

```
given {
  ids: [1, 2, 3]
  names: ["alice", "bob"]
}
```

## Assertions

Assertions appear in `then` blocks and invariant bodies. Every assertion uses an explicit comparison operator. Lines within a block are implicitly ANDed -- all must hold. There is no OR between assertion lines; use separate scenarios for disjunctive cases.

### Field Assertions

Assert output field values with explicit operators:

```
then {
  status == 200
  from.balance == from.balance - amount
  error == null
  score != 0
  count >= 1
}
```

Supported operators: `==`, `!=`, `>`, `>=`, `<`, `<=`.

### Adapter Assertions

Adapter methods that return values can be used as expressions in assertions:

```
then {
  playwright.visible('[data-testid="welcome"]') == true
  playwright.text('[data-testid="welcome"]') == "Hello, alice"
  playwright.count('[data-testid="items"]') >= 1
  playwright.disabled('[data-testid="submit"]') == false
}
```

The adapter call returns a value, and the comparison operator checks it against the expected value.

### Error Pseudo-Field

`error` is a special pseudo-field that asserts against adapter action errors. It activates only when `error` is NOT declared in the scope's contract output fields.

```
scenario click_missing {
  given {
    playwright.click('[data-testid="nonexistent"]')
  }
  then {
    error == "element not found"
  }
}

scenario no_error {
  given {
    playwright.click('[data-testid="submit"]')
  }
  then {
    error == null
  }
}
```

When `error` IS declared as a contract output field (e.g., `output { error: string? }`), it behaves as a normal field assertion, not the pseudo-field.

### Dot-Path Array Index Access

Numeric segments in dot-paths index into arrays:

```
then {
  result.items.0.name == "first"
  result.items.1.name == "second"
}
```

This works in both assertions and field references in expressions. Out-of-range indices produce an assertion failure.

## Mixed Adapters

A single scope can use multiple adapters. Adapters are named inline per call, so there is no restriction on which adapters appear in a given scope.

```
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

## Validation

Specs are validated after parsing, before generation and verification. Validation checks:

- Model resolution: all model references exist
- Type checking: literal values match declared types
- Array element types: all elements match the array's element type
- Object field validation: object literals contain only declared fields with matching types
- Given completeness: all required contract input fields are assigned
- Then field validation: all fields in `then` blocks are valid model fields or adapter assertions
- Contract action signature: action parameters match contract input field names and types

Validation failures cause `specrun` to exit with code 1 and print detailed messages:

```
error validating spec:
  scope transfer / scenario success:
    given: required field "to" not assigned
    error: type mismatch: expected int, got string "pending"
  scope transfer / contract:
    action: parameter "amount" type mismatch: expected float, got int
```

## Plugin Definition

Plugins are defined in `.plugin` files. Built-in plugins (http, process, playwright) are compiled into specrun. External plugins communicate via JSON over stdin/stdout.

```
plugin <name> {
  adapter: "<binary-name>"

  methods {
    <name>(<param>: <type>, ...): <return-type>
    <name>(<param>: <type>, ...)
  }
}
```

Methods with return types can be used in assertions. Methods without return types are action-only.

Example:

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

## Adapter Protocol

External adapters communicate via JSON over stdin/stdout. v3 uses a unified `call` message type:

**Call request/response:**
```json
{"type": "call", "method": "post", "args": ["/api/items", {"name": "widget"}]}
{"ok": true, "value": {"status": 201, "body": {"id": "123"}}}
```

**Call with return value (assertion context):**
```json
{"type": "call", "method": "visible", "args": ["[data-testid=\"welcome\"]"]}
{"ok": true, "value": true}
```

**Error response:**
```json
{"ok": false, "error": "element not found"}
```

## Complete Example

A full spec demonstrating v3 features:

```
spec TransferService {
  description: "Bank account transfer API"

  services {
    app {
      build: "./server"
      port: 8080
      health: "/healthz"
    }
  }

  http {
    base_url: service(app)
  }

  model Account {
    id: string
    balance: int { balance >= 0 }
  }

  action login(username: string, password: string) {
    let result = http.post("/api/auth/login", { username: username, password: password })
    http.header("Authorization", "Bearer " + result.body.access_token)
    return result.body
  }

  scope transfer {
    action transfer(from: Account, to: Account, amount: int) {
      let result = http.post("/api/v1/accounts/transfer", {
        from: from, to: to, amount: amount
      })
      return result.body
    }

    before {
      let session = login("admin", "test")
    }

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

    # Money is neither created nor destroyed.
    invariant conservation {
      when error == null:
        output.from.balance + output.to.balance
          == input.from.balance + input.to.balance
    }

    invariant non_negative {
      output.from.balance >= 0
      output.to.balance >= 0
    }

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
}
```
