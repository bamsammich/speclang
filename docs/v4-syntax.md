# v4 Syntax Structure

A layer-by-layer structural reference for speclang v4. This doc answers "where can I write X?" — for the full semantics of each construct, see [language-reference.md](language-reference.md).

## Layer 1 — the spec file

A `.spec` file IS a spec. The filename is its identity; there is no `spec Name { }` wrapper.

**Contains** (in any order):

- `include "<path>"`
- `import openapi("<path>")` / `import proto("<path>")`
- Declarations: `model`, `enum`, `config`, `action`, `contract`, `scope`
- Adapter config blocks (`http { }`, `playwright { }`, `process { }`)
- `services { }`

**Include semantics**

- Use at the top level. Mechanically, `include "<path>"` is token-spliced (the included file's tokens replace the directive), so the parser doesn't enforce placement — but anywhere other than the top level will produce confusing parse errors.
- Paths are resolved relative to the including file's directory.
- Transitive includes are resolved recursively.
- Duplicate model, enum, action, scope, contract, or service names across the fully resolved token stream are errors.
- Circular includes are rejected.
- Included files are themselves valid spec files and can be executed standalone if they contain contracts.
- `include` is for shared library fragments (models, actions, adapter config); to run multiple independent specs use `specrun verify <glob>`. See [Includes vs. glob execution](language-reference.md#includes-vs-glob-execution).

**CLI execution**

- `specrun verify <glob>` — each matched file is an independent spec unit.
- Files with no contracts are logged (`no contracts, skipping`) and pass trivially.
- Glob patterns (`*`, `**`) expand before execution; duplicates are deduplicated.

## Layer 2 — top-level declarations

Every construct below lives at the file's top level. All declarations may appear in any order; forward references are permitted.

### `model`

```
model Account {
  id: string
  balance: int { balance >= 0 }
  tracking: string when status == "shipped"
}
```

Contains fields. Each field has a type, an optional `{ constraint }`, and an optional `when <predicate>` for state-dependent presence.

### `enum` (named)

```
enum Role { admin, user, viewer }
```

Contains variant identifiers. Referenced as `EnumName.variant` in expressions. Distinct from the inline `enum("a","b")` *type*, which takes string-literal variants.

### `config`

```
config {
  max_transfer: 1_000_000
  api_version: "v2"
}
```

Contains key-value expressions. Referenced as `config.<key>`. Multiple `config { }` blocks in one file are merged.

### `action` (reusable)

```
action login(username: string, password: string) {
  let result = http.post("/api/auth/login", { username: username, password: password })
  http.header("Authorization", "Bearer " + result.body.access_token)
  return result.body
}
```

Typed parameters, body of `let` / adapter call / action call / `return`. Callable from `before`, `after`, `given`, other action bodies, and contract `action { }` blocks.

### Adapter config blocks

```
http { base_url: service(app) }
playwright { headless: true, timeout: "5000" }
process { command: "./my-binary", args: ["verify", "--json"] }
```

One block per adapter used in the spec. Keys are adapter-specific. Values are expressions.

### `services`

```
services {
  app   { build: "./server",   port: 8080, health: "/healthz" }
  db    { image: "postgres:16", port: 5432, env { POSTGRES_PASSWORD: "test" } }
  stack { compose: "docker-compose.yml", port: 8080, health: "/healthz" }
}
```

Exactly one of `build:`, `image:`, or `compose:` per service. Referenced elsewhere with `service(name)`.

### `contract`

Top-level or inside a scope. See [Layer 3](#layer-3--contract).

### `scope`

Optional grouping for shared lifecycle hooks. See [Layer 4](#layer-4--scope).

## Layer 3 — `contract`

The behavioral promise — the primary verification unit.

```
contract <Name>(
  <field>: <type>,
  <field>: <type> { <constraint> },
  <field>: <type> when <condition>,
) -> <ReturnType> {
  action {
    <steps>
    return <expr>
  }

  invariant <name> { ... }
  scenario  <name> { ... }
}
```

Single-line and empty-parens forms are also valid:

```
contract Health() -> HealthResult {
  action { return http.get("/healthz") }
}

contract Login(username: string, password: string) -> AuthResult { action { ... } }
```

### With model inheritance

```
contract <Name>: <InputModel> -> <ReturnType> {
  constrain {
    <expr>
    <expr>
  }

  action { ... }
}
```

The inherited model supplies input fields; `constrain { }` adds bound-style expressions over those fields.

### What a contract contains

| Member | Where | Count | Purpose |
|--------|-------|-------|---------|
| Input fields | Signature parens | 0+ | Implicit inputs (bare identifiers in the body refer to these) |
| `constrain { }` | Body (inheritance form only) | 0 or 1 | Additional expressions over inherited fields |
| `action { }` | Body | 0 or 1 | Execution recipe — no signature, sees fields implicitly |
| `invariant` | Body | 0+ | Universal laws |
| `scenario` | Body | 0+ | Concrete or generative cases |

### Field resolution inside a contract

| Form | Resolves to |
|------|-------------|
| `<bare_name>` | Contract input field |
| `output.<field>` | Field on the return type |
| `config.<key>` | Spec-level config |
| `<EnumName>.<variant>` | Named enum variant |
| `error` (in `then`, when no `error` output field) | Adapter action error pseudo-field |

### Placement

- At the top level — no lifecycle hooks.
- Inside a `scope` — inherits the scope's `before`/`after`.

## Layer 4 — `scope`

Optional grouping of contracts sharing `before`/`after` hooks.

```
scope <name> {
  before { <steps> }
  after  { <steps> }

  contract <Name> -> <ReturnType> { ... }
  contract <Name> -> <ReturnType> { ... }
}
```

### What a scope contains

| Member | Count | Purpose |
|--------|-------|---------|
| `before { }` | 0 or 1 | Setup run before each iteration (after adapter reset) |
| `after { }` | 0 or 1 | Teardown run after each iteration (errors logged, not fatal) |
| `contract` | 0+ | Contracts sharing the scope's lifecycle |

In v4 scopes no longer own invariants, scenarios, or actions directly — those live inside contracts.

## Layer 5 — verification members

All three forms live *inside a contract body*.

### `invariant`

```
invariant <name> {
  <assertion>
  <assertion>
}

invariant <name> {
  when <guard>:
    <assertion>
    <assertion>
}
```

Universal law — must hold for every generated input satisfying the contract's constraints. Multiple assertions are implicitly ANDed.

### `scenario` with `given`

```
scenario <name> {
  given {
    <field>: <value>
    let <name> = <expr>
    <adapter>.<method>(<args>)
    <action>(<args>)
  }
  then {
    <assertion>
    <assertion>
  }
}
```

Concrete smoke test — fixed inputs, runs once. Assignments, `let` bindings, and calls may interleave; execution order is source order.

### `scenario` with `when`

```
scenario <name> {
  when {
    <predicate>
    <predicate>
  }
  then {
    <assertion>
  }
}
```

Generative test — the runtime draws many inputs matching the predicates and checks `then` against each. Multiple predicates are implicitly ANDed.

## Construct placement table

| Construct | Lives in | Contains |
|-----------|----------|----------|
| File | Filesystem | Everything (the file is the spec) |
| `include` | Top level | A string path to another spec file |
| `import` | Top level | An adapter-name + path (`openapi(...)`, `proto(...)`) |
| `model` | Top level | Fields |
| `enum` | Top level | Variant identifiers |
| `config` | Top level | Key-value expressions |
| `action` | Top level | Typed params + body (`let` / calls / `return`) |
| Adapter config (`http { }` etc.) | Top level | Key-value expressions |
| `services` | Top level | Service definitions |
| `scope` | Top level | `before`, `after`, contracts |
| `contract` | Top level *or* scope | Parens signature (fields), body: `constrain`, `action`, invariants, scenarios |
| `before` / `after` | Scope | Steps (`let` / adapter call / action call) |
| `action { }` block | Contract | Steps (`let` / adapter call / action call / `return`) |
| `constrain { }` block | Contract (with inheritance) | Boolean expressions over inherited fields |
| `invariant` | Contract | Optional `when <guard>:` + assertions |
| `scenario` | Contract | `given` or `when`, plus `then` |
| `given` | Scenario | Assignments, `let`, adapter/action calls |
| `when` (scenario) | Scenario | Predicate expressions |
| `then` | Scenario | Assertion expressions |
