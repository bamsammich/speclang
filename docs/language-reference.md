# Language Reference

Complete syntax reference for speclang v4.

## 1. Overview

A `.spec` file IS a spec. There is no `spec Name { ... }` wrapper — top-level declarations appear directly in the file, and the filename is the spec's identity. Each file is verified independently by `specrun verify`; files containing no contracts are silently skipped.

## 2. File structure

A spec file contains any number of top-level declarations, in any order:

| Declaration | Purpose |
|-------------|---------|
| `include "<path>"` | Import declarations from another file |
| `import openapi("<path>")` / `import proto("<path>")` | Import models/contracts from external schema |
| `model <Name> { ... }` | Named data structure |
| `enum <Name> { <variant>, ... }` | Named enumeration |
| `config { <key>: <expr> }` | Spec-level constants |
| `action <name>(<params>) { ... }` | Reusable action with typed signature |
| `contract <Name>(<params>) -> <Type> { ... }` | Behavioral promise (top-level or in scope) |
| `scope <name> { ... }` | Optional grouping of contracts sharing `before`/`after` |
| `<adapter> { <key>: <expr> }` | Adapter configuration (e.g. `http { ... }`) |
| `services { ... }` | Docker services as test infrastructure |

Top-level order does not affect semantics (forward references are permitted). Includes are resolved before parsing; see [Includes](#15-includes).

## 3. Types

| Type | Syntax |
|------|--------|
| Integer | `int` |
| Floating-point | `float` |
| String | `string` |
| Binary | `bytes` (base64-encoded in JSON) |
| Boolean | `bool` |
| Untyped | `any` |
| Array | `[]T` |
| Map | `map[K, V]` |
| Inline enum | `enum("a", "b", ...)` |
| Named enum | `<EnumName>` (refers to a top-level `enum` declaration) |
| Model | `<ModelName>` |
| Optional | `T?` (trailing `?` binds to the outermost type — `[]int?` is an optional array, not an array of optionals) |

Inline enums are anonymous sets of string variants. Named enums are declared at the top level (see [§5 Declarations](#5-declarations)); variants are referenced as `EnumName.variant`.

## 4. Expressions

Expressions appear in constraints, `when` predicates, invariants, `then` assertions, `let` bindings, contract `constrain` blocks, adapter call arguments, and config values.

### 4.1 Literals

| Form | Example |
|------|---------|
| Integer | `42`, `-1`, `1_000_000` |
| Float | `3.14`, `0.5`, `3.14_159` |
| Double-quoted string | `"hello"` |
| Single-quoted string | `'[data-testid="email"]'` |
| Boolean | `true`, `false` |
| Null | `null` |
| Object | `{ id: "alice", balance: 100 }` |
| Array | `[1, 2, 3]`, `["a", "b"]` |

Underscores are allowed between digits in numeric literals and are stripped before parsing (`1_000_000` is lexed as `1000000`). Single-quoted strings are a convenience for CSS selectors containing double quotes; the two forms are otherwise interchangeable.

### 4.2 Identifiers and member access

Dot-separated paths into the data. Numeric segments index arrays by position (zero-based):

```
from.balance
output.error
config.max_transfer
result.body.items.0.name
Role.admin
```

### 4.3 Built-in references

| Form | Resolves to |
|------|-------------|
| `env(VAR)` | Environment variable (empty string if unset) |
| `env(VAR, "default")` | Environment variable with default |
| `service(name)` | URL of a running service declared in `services { }` |
| `config.<key>` | Value from the spec's `config { }` block |

### 4.4 Operators

| Category | Operators | Notes |
|----------|-----------|-------|
| Arithmetic | `+` `-` `*` `/` `%` | `+` also concatenates when either operand is a string |
| Comparison | `==` `!=` `<` `<=` `>` `>=` | Chained form supported: `0 < x <= y` |
| Membership | `in` | RHS is a list of values. Parenthesis form preferred: `status in ("pending", "active")`. Bracket form also accepted: `status in ["pending", "active"]`. Both are equivalent; parens read more naturally as a membership set. |
| Logical | `and` `or` `not` | Word operators only — `&&`, `\|\|`, `!` are rejected by the lexer |
| Implication | `implies` | Lowest precedence; `a implies b` ≡ `not a or b` |
| Unary | `-` `not` | |

Precedence (lowest to highest): `implies` < `or` < `and` < `==`, `!=` < `<`, `<=`, `>`, `>=`, `in` < `+`, `-` < `*`, `/`, `%` < unary. All infix operators are left-associative.

### 4.5 Built-in functions

| Function | Signature | Description |
|----------|-----------|-------------|
| `len(x)` | `len(expr) -> int` | Length of string, array, or map |
| `contains(h, n)` | `contains(expr, expr) -> bool` | Substring (strings) or element membership (arrays) |
| `exists(p)` | `exists(path) -> bool` | True if path resolves (including to `null`) |
| `has_key(m, k)` | `has_key(expr, expr) -> bool` | True if map contains the key |
| `all(arr, x => pred)` | `all(expr, ident => expr) -> bool` | Universal quantifier |
| `any(arr, x => pred)` | `any(expr, ident => expr) -> bool` | Existential quantifier |

### 4.6 Conditional expression

```
if <cond> then <expr> else <expr>
```

The condition must evaluate to a boolean. Nesting requires parentheses around the inner `if`.

## 5. Declarations

### 5.1 `model`

```
model Account {
  id: string
  balance: int { balance >= 0 }
  email: string?
  tracking: string when status == "shipped"
}
```

A model is a named record of typed fields. Each field may have a constraint (in braces after the type) and/or a state-dependent presence condition (`when <expr>` after the type/constraint — see [§11](#11-state-dependent-fields)). Optional commas between fields are tolerated.

### 5.2 `enum` (named)

```
enum Role { admin, user, viewer }
enum OrderStatus { pending, confirmed, shipped, cancelled }
```

A named enumeration. Variants are identifiers. Reference as `EnumName.variant`:

```
model User { role: Role }

then { output.role == Role.admin }

when { status in (OrderStatus.pending, OrderStatus.cancelled) }
```

Inline `enum("a","b")` types are a separate construct (see [§3](#3-types)) and are matched using string literals, not qualified variants.

### 5.3 `config`

```
config {
  max_transfer: 1_000_000
  api_version: "v2"
  retries: 3
}
```

Spec-level constants. Values are any expression. Reference with `config.<key>`:

```
amount: int { amount <= config.max_transfer }
```

Multiple `config { }` blocks are merged.

### 5.4 `action` (reusable)

```
action login(username: string, password: string) {
  let result = http.post("/api/auth/login", { username: username, password: password })
  http.header("Authorization", "Bearer " + result.body.access_token)
  return result.body
}
```

A reusable procedure with typed parameters. The body is a sequence of steps: `let` bindings, adapter calls, local action calls, and `return`. Action calls appear in `before`/`after` blocks, `given` blocks, and contract `action { }` bodies.

Actions are defined at the top level only (they are no longer scoped to a `scope` in v4).

### 5.5 `scope`

```
scope transfers {
  before { login("admin", "test") }
  after  { http.delete("/api/session") }

  contract Transfer(from: Account, to: Account, amount: int) -> TransferResult { ... }
  contract Reverse(from: Account, to: Account, amount: int) -> TransferResult { ... }
}
```

An optional grouping of contracts that share `before` and `after` lifecycle blocks. Scopes do not own invariants, scenarios, or actions in v4 — those live inside contracts.

### 5.6 `contract`

See [§6](#6-contract).

### 5.7 Adapter configuration blocks

```
http { base_url: service(app) }
playwright { headless: true, timeout: "5000" }
process { command: "./my-binary", args: ["verify", "--json"] }
```

One block per adapter used in the spec. Values are expressions (including `env()` and `service()`).

### 5.8 `services`

See [§13](#13-services).

## 6. Contract

The centerpiece of verification. A contract is a self-contained behavioral promise: inputs, execution, and the properties that must hold.

### 6.1 Syntax (declared form)

Input fields are declared in the signature parens. The body contains only the `action` block, invariants, and scenarios.

```
contract <Name>(
  <field>: <type>,
  <field>: <type> { <constraint> },
  <field>: <type> when <condition>,
) -> <ReturnType> {
  action {
    <steps...>
    return <expr>
  }

  invariant <name> { <assertions> }
  scenario  <name> { given { ... } then { ... } }
  scenario  <name> { when  { ... } then { ... } }
}
```

Single-line and empty-parens forms are also valid:

```
contract Health() -> HealthResult {
  action { return http.get("/healthz") }
}

contract Login(username: string, password: string) -> AuthResult { action { ... } }
```

Trailing commas in the parameter list are allowed. Commas between params are required.

### 6.2 Syntax (model-inheritance form)

```
contract <Name>: <InputModel> -> <ReturnType> {
  constrain {
    <expression>
    <expression>
  }

  action { ... }
  invariant ...
  scenario  ...
}
```

When `InputModel` is specified, the contract inherits its fields as inputs — no parens needed on the signature. The optional `constrain { }` block adds expressions (bound-style constraints) over those inherited fields. This form is used when pairing with imported schemas (see [§14 Imports](#14-imports)).

### 6.3 Field resolution rules

Inside a contract, bare identifiers resolve as follows:

| Form | Resolves to |
|------|-------------|
| `<name>` | Contract input field (declared or inherited) |
| `output.<name>` | Field of the return type |
| `config.<name>` | Spec-level config value |
| `<EnumName>.<variant>` | Named enum variant |
| `error` | Adapter action error, *when* `error` is not declared as an output field (see [§8](#8-assertions-in-then-blocks)) |

### 6.4 The `action` block

The action block has no signature — it sees the contract's input fields implicitly as named values. Its body is a sequence of steps (`let`, adapter/action calls, `return`):

```
action {
  let result = http.post("/api/v1/accounts/transfer", {
    from: from, to: to, amount: amount
  })
  return result.body
}
```

The action block's `return` value becomes `output` in assertions. A contract whose assertions reference `output.*` must therefore return a value from its action block.

### 6.5 Placement

Contracts may be declared at the top level *or* inside a `scope`. A scoped contract inherits the scope's `before`/`after` hooks; a top-level contract has no lifecycle hooks.

## 7. Verification members

All three forms live inside a contract body.

### 7.1 `invariant`

A universal law that must hold across *all* generated inputs satisfying the contract's constraints.

```
invariant conservation {
  output.error == null implies
    output.from.balance + output.to.balance == from.balance + to.balance
}

invariant non_negative {
  output.from.balance >= 0
  output.to.balance >= 0
}
```

Multiple expressions in the body are implicitly ANDed — every assertion must hold.

An optional `when <guard>:` prefix limits the invariant to inputs/outputs matching the guard:

```
invariant no_mutation_on_error {
  when output.error != null:
    output.from.balance == from.balance
    output.to.balance == to.balance
}
```

### 7.2 `scenario` with `given`

A concrete smoke test. Fixed inputs, runs once:

```
scenario success {
  given {
    from: { id: "alice", balance: 100 }
    to: { id: "bob", balance: 50 }
    amount: 30
  }
  then {
    output.from.balance == from.balance - amount
    output.to.balance == to.balance + amount
    output.error == null
  }
}
```

Prefer relational assertions (`from.balance - amount`) over hardcoded results (`70`) so the scenario resists memorization.

`given` blocks may also contain `let` bindings and adapter/action calls — they execute in order alongside the assignments:

```
given {
  let session = login("alice", "secret")
  amount: 1000
  playwright.fill('[data-testid="amount"]', amount)
}
```

### 7.3 `scenario` with `when`

A generative test. The runtime produces many inputs matching the predicate and checks the `then` block against each:

```
scenario overdraft {
  when { amount > from.balance }
  then { output.error == TransferError.insufficient_funds }
}
```

Multiple predicates in a `when` block are implicitly ANDed.

## 8. Assertions in `then` blocks

Assertions are expressions. The supported operators are `==`, `!=`, `<`, `<=`, `>`, `>=`. Lines in a `then` block are implicitly ANDed — all must hold.

### 8.1 The `output.` prefix rule

Inputs resolve as bare names; the return value of the action is named `output`. **Always prefix return-type fields with `output.`**:

```
then {
  output.from.balance == from.balance - amount   # output. on the LHS, bare input on the RHS
  output.error == null
}
```

Adapter call results used as expressions need no prefix:

```
then {
  playwright.visible('[data-testid="welcome"]') == true
  playwright.text('[data-testid="welcome"]') == "Hello, alice"
  playwright.count('[data-testid="items"]') >= 1
}
```

### 8.2 The `error` pseudo-field

When `error` is *not* declared as a field of the return type, the bare identifier `error` in a `then` block refers to the adapter action's error:

```
scenario click_missing {
  given { playwright.click('[data-testid="nonexistent"]') }
  then { error == "element not found" }
}
```

When `error` *is* declared on the return type, `output.error` behaves as a normal field reference and the pseudo-field is inactive for that contract.

### 8.3 Array index access

Numeric dot-path segments index into arrays (zero-based). Out-of-range access produces an assertion failure:

```
then {
  output.items.0.name == "first"
  output.items.1.name == "second"
}
```

## 9. Lifecycle (`before` / `after`)

```
scope orders {
  before {
    let session = login("admin", "test")
  }
  after {
    http.delete("/api/session")
  }

  contract CreateOrder(...) -> OrderResult { ... }
  contract UpdateOrder(...) -> OrderResult { ... }
}
```

- `before` runs before each scenario's `given` steps and each invariant iteration. Adapter state is reset first — a fresh HTTP client, empty headers, cleared cookies/localStorage.
- `after` runs after each iteration, **even if that iteration failed**. Errors in `after` steps are logged but do not affect the verdict.
- Scenes established in `before` (headers, cookies, let-bound values) carry into the contract's action block.
- A failing `before` aborts the remaining contracts in the scope.

Top-level contracts (outside any scope) have no `before`/`after`.

## 10. Actions (reusable)

See [§5.4](#54-action-reusable) for the declaration form. Body steps:

| Step | Form |
|------|------|
| Variable binding | `let <name> = <expr>` |
| Adapter call | `<adapter>.<method>(<args>)` |
| Local call | `<action>(<args>)` |
| Return | `return <expr>` |

`let` bindings are immutable and visible only within the block in which they are defined. The right-hand side can be any expression, including an adapter or action call (the result becomes the bound value).

## 11. State-dependent fields

A field's presence can be made conditional on another field's value using a trailing `when <expr>`:

```
model Shipment {
  status: string
  tracking: string when status == "shipped"
}

contract Update(
  status: string,
  tracking: string when status == "shipped",
) -> ShipmentResult {
  action { ... }
}
```

- The field is generated/required only when the condition holds.
- The condition may reference *sibling* fields of the same model or contract.
- References to unknown fields are rejected at validation time.
- Circular `when` dependencies (A depends on B, B depends on A) are rejected.

## 12. Adapter calls

All adapter interaction uses the `<adapter>.<method>(<args>...)` form. Adapters are named inline per call; there is no ambient adapter context.

```
http.get("/api/items/123")
http.post("/api/items", { name: "widget" })
http.header("Authorization", "Bearer abc123")
playwright.goto("/dashboard")
playwright.fill('[data-testid="email"]', "alice@example.com")
playwright.click('[data-testid="submit"]')
process.run("echo", ["hello", "world"])
```

Adapter calls appear in action bodies, `before`/`after` blocks, `given` blocks, and (as value expressions) in `then` assertions. Every adapter used in a spec needs its config block at the top level (e.g. `http { base_url: ... }`).

Built-in adapters:

- [`http`](adapters/http.md) — REST APIs
- [`process`](adapters/process.md) — CLI tools, subprocesses
- [`playwright`](adapters/playwright.md) — browser UI

## 13. Services

The `services { }` block declares Docker containers as test infrastructure. `specrun verify` manages their lifecycle: pre-flight cleanup of stale containers, build or pull, start, health check, run verification, stop and remove.

### 13.1 Syntax

```
services {
  app {
    build: "./server"          # Dockerfile directory (relative to the spec file)
    port: 8080                 # container port to expose
    health: "/healthz"         # HTTP GET path for health check (optional)
    env { PORT: "8080" }       # optional environment variables
    volumes {                  # optional volume mounts (host: container)
      "./fixtures": "/data"
    }
  }

  db {
    image: "postgres:16"       # pre-built image, alternative to build
    port: 5432
    env { POSTGRES_PASSWORD: "test" }
  }

  stack {
    compose: "docker-compose.yml"   # multi-service compose file
    port: 8080
    health: "/healthz"
  }
}
```

### 13.2 Source selection

Each service needs exactly one of:

- `build: "<path>"` — a directory containing a `Dockerfile`
- `image: "<ref>"` — a pre-built image reference
- `compose: "<path>"` — a `docker-compose.yml` describing the full stack (mutually exclusive with `build`/`image`)

### 13.3 `service(name)` reference

The `service(name)` expression resolves at runtime to `http://localhost:<port>` for the named container (using the actual mapped host port):

```
http { base_url: service(app) }
```

The name must match a service declared in `services { }`; unknown names fail validation.

### 13.4 Health checks

- If `health:` is set, HTTP GET is polled at `http://localhost:<port><health>` until 200.
- If `health:` is absent, a TCP connection check against the mapped port is used.
- Timeout failures abort verification.

### 13.5 `--keep-services`

Pass `--keep-services` to `specrun verify` to leave containers running after verification for debugging.

## 14. Imports

```
import openapi("./schema.yaml")
import proto("./service.proto")
```

Imports generate models and contracts from an external schema file. Paths are relative to the spec file's directory. Imported contracts have fields and return types populated but no invariants or scenarios — add those in the importing spec.

Duplicate names between imported and hand-written declarations are errors.

See [OpenAPI import](imports/openapi.md) and [Protobuf import](imports/protobuf.md) for format-specific detail.

## 15. Includes

```
include "shared/infra.spec"
include "shared/models.spec"
```

- `include` is a top-level directive; always write it at the top level of a file. (Mechanically, the directive is resolved by token-splicing the included file's contents in its place, so technically the parser doesn't enforce placement — but anything else invites confusing parse errors downstream.)
- Paths are relative to the *including* file's directory (not the root).
- Transitive includes resolve recursively.
- Duplicate model, enum, action, scope, contract, or service names across the fully resolved token stream are errors.
- Circular includes are detected and rejected.
- Included files are themselves valid spec files — they can be run standalone if they declare contracts.

### Includes vs. glob execution

`include` and `specrun verify <glob>` are two different tools for two different jobs:

| Tool | Purpose |
|------|---------|
| `include "path"` | Import shared, non-runnable fragments — models, actions, adapter config, service definitions — into the current spec's declaration namespace. |
| `specrun verify <glob>` | Batch-run independent, self-contained specs. Each matched file is a separate verification unit with its own result. |

**Do not use `include` to compose runnable specs into a super-spec.** Every declaration in the include graph must be unique; if two specs both define `model Account`, including them both from a root file is an error. The right model for multiple independent specs that both need the same models is:

1. Factor the shared declarations into a library file:

```
# shared/models.spec
model Account {
  id: string
  balance: int
}
```

2. Each spec that needs them includes the library:

```
# specs/transfer.spec
include "shared/models.spec"

scope transfer {
  contract Transfer(...) -> TransferResult { ... }
}
```

```
# specs/audit.spec
include "shared/models.spec"

scope audit {
  contract Audit(...) -> AuditLog { ... }
}
```

3. Run both specs with a glob — each is verified independently:

```bash
specrun verify specs/*.spec
```

The library file (`shared/models.spec`) has no contracts, so it is skipped with a "no contracts" notice if matched by the glob. That is expected and harmless.

## 16. CLI

```bash
specrun verify <spec-file|glob> [<spec-file|glob>...]
                                             # each matched file is verified independently
                                             # flags: --seed N --iterations N --json --keep-services

specrun parse    <spec-file>                 # print AST as JSON
specrun generate <spec-file> --scope <name>  # generate one input (flags: --seed N)
                                             # --scope matches either a scope name or a
                                             # top-level contract's identifier

specrun migrate --to v4 <spec-file> [<spec-file>...]
                                             # rewrite a v3 spec to v4
                                             # flags: -w/--write (rewrite in place); --to v3|v4

specrun install playwright                   # install Playwright Chromium browser
```

Glob patterns (`*`, `**`) expand to every matching file. Files with no contracts are logged and skipped, not errored:

```
mymodels.spec: no contracts, skipping
```

## 17. Error messages

Common parse and validation errors and their causes:

| Message | Cause |
|---------|-------|
| `'spec Name { }' wrapper is removed in v4; top-level declarations appear directly in the file` | Legacy v2/v3 spec — run `specrun migrate --to v4` |
| `'use' directive is not valid; adapters are named inline per call` | Legacy v2 spec — migrate to v4 |
| `unexpected character '!' (use 'not' for logical negation)` | Lone `!` — use `not` |
| `include requires a string path` | `include` not followed by a string literal |
| `circular include detected` | Include cycle between files |
| `duplicate declaration: model "..."` / `duplicate declaration: scope "..."` / etc. | Same name declared in multiple included files; see [Includes vs. glob execution](#includes-vs-glob-execution) |
| `references unknown field "..."` | A `when` condition or assertion names a non-existent field |
| `circular when dependency involving field "..."` | Two or more fields' `when` expressions reference each other |
| `inherits unknown model "..."` | `contract Foo: Bar` with no `model Bar` in scope |
| `enum variants must be string literals` | Inline `enum()` with identifier variants |
| `enum type requires at least one variant` | Empty inline `enum()` |
