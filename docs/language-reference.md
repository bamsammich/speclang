# Language Reference

Complete syntax reference for the speclang specification language.

## Spec Structure

Every spec file contains a single `spec` block:

```
spec <Name> {
  description: "<text>"              # optional

  target { ... }                     # plugin-dependent config
  locators { ... }                   # UI specs: named CSS selectors
  model <Name> { ... }              # data models
  action <name>(<params>) { ... }   # reusable action sequences
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
- Imported scopes have config and contracts populated but no invariants or scenarios
- Duplicate model or scope names between imported and hand-written produce an error

See [OpenAPI Import](imports/openapi.md) and [Protobuf Import](imports/protobuf.md) for adapter-specific details.

## Target Block

```
target {
  base_url: env(APP_URL, "http://localhost:8080")
  command: "./my-binary"
  headless: "true"
  timeout: "5000"
}
```

Plugin-dependent configuration. Common keys:

| Key | Plugin | Description |
|-----|--------|-------------|
| `base_url` | http, playwright | Server URL |
| `command` | process | Binary to execute |
| `headless` | playwright | `"true"` or `"false"` (default `"true"`) |
| `timeout` | playwright | Milliseconds (default `"5000"`) |

Values can use `env(VAR)` or `env(VAR, "default")` to read environment variables, or `service(name)` to reference a declared service URL.

## Target Services

The `services` block in `target` declares Docker containers as test infrastructure. When services are declared, `specrun verify` manages their full lifecycle: cleanup stale containers, build/pull, start, health-check, run verification, then stop and remove.

### Inline Services

```
target {
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
  base_url: service(app)          # resolves to running container URL
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
target {
  services {
    compose: "docker-compose.yml"
  }
  base_url: service(app)
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

Expressions appear in constraints, invariants, `when` predicates, and `then` assertions.

### Literals

| Literal | Example |
|---------|---------|
| Integer | `42`, `-1`, `0` |
| Float | `3.14`, `0.5` |
| String | `"hello"` |
| Boolean | `true`, `false` |
| Null | `null` |
| Object | `{ id: "alice", balance: 100 }` |
| Array | `[1, 2, 3]`, `["a", "b"]` |

### Field References

Dot-separated paths into the data:

```
from.balance
output.error
input.from.id
stdout.items.0.name      # numeric segment indexes into array
```

Numeric segments in dot-paths index into arrays by position (zero-based).

### Service References

```
service(app)                  # resolves to URL of named service from target services block
```

Resolves at runtime to the URL of a running container declared in the `target` `services` block. The name must match a declared service. See [Target Services](#target-services) for details.

### Environment Variables

```
env(APP_URL)                  # required, fails if unset
env(APP_URL, "http://localhost:8080")  # with default
```

### Operators

| Operator | Description | Example |
|----------|-------------|---------|
| `+` | Addition | `a + b` |
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

## Actions

Reusable action sequences defined at spec level:

```
action login(user, pass) {
  playwright.fill(username_field, user)
  playwright.fill(password_field, pass)
  playwright.click(submit_btn)
}
```

Called from `given` blocks: `login("alice", "secret")`.

## Locators

Named CSS selectors for UI specs, declared at spec level:

```
locators {
  username_field: [data-testid=username]
  submit_btn:     [data-testid=submit]
  error_msg:      [data-testid=error]
}
```

Locator names are used as arguments to plugin actions and in assertion syntax. All locators must be pre-declared; inline selectors in action calls are not supported.

## Scopes

Scopes are named groupings that own a contract, invariants, and scenarios. Each scope declares which plugin drives it.

```
scope transfer {
  use http                            # required: plugin declaration

  config {                            # opaque key-value pairs for the adapter
    path: "/api/v1/accounts/transfer"
    method: "POST"
  }

  contract { ... }
  invariant <name> { ... }
  scenario <name> { ... }
}
```

- `use <plugin>` is **required** in every scope and appears at the **scope** level, not spec level
- Different scopes in the same spec can use different plugins
- The parser is agnostic to `config` block semantics -- they're passed through to the adapter

### Contract

Defines typed input and output for the scope:

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
}
```

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
    from.balance: from.balance - amount
    to.balance: to.balance + amount
    error: null
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
    error: "insufficient_funds"
  }
}
```

#### Mixed `given` Blocks

`given` blocks can interleave data assignments and action calls. Steps execute in order:

```
given {
  amount: 1000
  playwright.goto("/transfer")
  playwright.fill(amount_input, amount)
  playwright.click(submit_btn)
}
```

Named actions can also be called:

```
given {
  login("alice", "secret")
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

### Plain Assertions

In `then` blocks, assert output field values:

```
then {
  status: 200
  from.balance: from.balance - amount
  error: null
}
```

### Plugin Assertions

For UI/plugin-specific properties, use `locator@plugin.property: expected`:

```
then {
  welcome@playwright.visible: true
  welcome@playwright.text: "Welcome, alice"
  error_msg@playwright.visible: false
}
```

### Error Pseudo-Field

`error` is a special pseudo-field that asserts against adapter action errors. It activates only when `error` is NOT declared in the scope's contract output fields.

```
scenario click_missing {
  given {
    playwright.click(nonexistent_btn)
  }
  then {
    error: "element not found"    # expect this specific error
  }
}

scenario no_error {
  given {
    playwright.click(submit_btn)
  }
  then {
    error: null                   # expect no error
  }
}
```

When `error` IS declared as a contract output field (e.g., `output { error: string? }`), it behaves as a normal field assertion, not the pseudo-field.

### Dot-Path Array Index Access

Numeric segments in dot-paths index into arrays:

```
then {
  stdout.items.0.name: "first"    # first element's name
  stdout.items.1.name: "second"   # second element's name
}
```

This works in both `then` assertions and field references in expressions. Out-of-range indices produce an assertion failure.

## Validation

Specs are validated after parsing, before generation and verification. Validation checks:

- Model resolution: all model references exist
- Type checking: literal values match declared types
- Array element types: all elements match the array's element type
- Object field validation: object literals contain only declared fields with matching types
- Given completeness: all required contract input fields are assigned
- Then field validation: all fields in `then` blocks are valid model fields or locator assertions

Validation failures cause `specrun` to exit with code 1 and print detailed messages:

```
error validating spec:
  scope transfer / scenario success:
    given: required field "to" not assigned
    error: type mismatch: expected int, got string "pending"
```

## Plugin Definition

External plugins are defined in `.plugin` files:

```
plugin <name> {
  adapter: "<binary-name>"

  actions {
    <verb>(<param>: <type>, ...)
  }

  assertions {
    <property>: <type>
  }
}
```

Built-in plugins (http, process, playwright) are compiled into specrun. External plugins communicate via JSON over stdin/stdout.

## Adapter Protocol

External adapters communicate via JSON over stdin/stdout:

**Action request/response:**
```json
{"type": "action", "name": "goto", "args": ["/transfer"]}
{"ok": true}
```

**Assertion request/response:**
```json
{"type": "assert", "locator": "[data-testid=error-msg]", "property": "visible", "expected": true}
{"ok": true, "actual": true}
```

**Error response:**
```json
{"ok": false, "error": "element not found"}
```
