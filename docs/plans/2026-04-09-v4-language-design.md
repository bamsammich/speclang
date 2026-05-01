# Speclang v4: Language Evolution

## Context

Evaluation of [Allium](https://juxt.github.io/allium/) identified readability improvements for speclang. Combined with long-standing friction around contract/action redundancy and implicit field resolution, v4 is a major language redesign: contracts become self-contained behavioral promises, operators become English words, the `spec` wrapper is removed, and several new expression features ship.

## v4 Language Structure

### Layer 1: Spec file — the root

A `.spec` file IS a spec. No wrapper keyword. The filename is the spec's identity. Top-level declarations appear directly in the file.

```
# transfer.spec — the filename is the spec identity
http {
  base_url: service(app)
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

contract Transfer(from: Account, to: Account, amount: int) -> TransferResult { ... }
```

**Include semantics:**
- `include` is a **top-level-only directive** — imports declarations from another file
- **Include-once** by resolved absolute path — diamond includes are safe (second include silently skipped)
- No fragment files — included files must be valid top-level declaration files
- Duplicate declarations from includes are errors (no structural dedupe)
- Included files are themselves valid spec files (can be run standalone if they have contracts)

**CLI execution:**
- `specrun verify <glob>` — each matched file is an independent spec unit, verified independently
- Files with no contracts are valid but skipped (logged: "transfer_models.spec: no contracts, skipping")
- Included files resolved relative to the including file's directory

### Layer 2: Declarations — the building blocks

**`model`** — a data structure. Fields with types and optional constraints.
```
model Account {
  id: string
  balance: int { balance >= 0 }
  tracking: string when status == "shipped"  # state-dependent field
}
```

**`enum`** — a named set of values. Variants referenced with qualified syntax `EnumName.variant`.
```
enum Role { admin, user, viewer }

# As a field type
model User { role: Role }

# In assertions — qualified reference, validator checks variant exists
then { output.role == Role.admin }

# In predicates
when { status in (OrderStatus.pending, OrderStatus.cancelled) }
```

Inline enums (`enum("a", "b")`) still use string comparison — they have no name to qualify.

**`config`** — spec-level constants. Referenced as `config.key` in expressions.
```
config {
  max_transfer: 1_000_000
  api_version: "v2"
}
```

**`action`** — a reusable procedure. Has its own typed signature. No verification, no output type. Used in `before`/`given` blocks and contract action bodies.
```
action login(username: string, password: string) {
  let result = http.post("/api/auth/login", { username: username, password: password })
  http.header("Authorization", "Bearer " + result.body.access_token)
}
```

**Adapter blocks** — configuration for adapters used in the spec.
```
http { base_url: service(app) }
playwright { headless: true, timeout: "5000" }
process { command: "./my-binary" }
```

**`services`** — Docker containers as test infrastructure.
```
services {
  app { build: "./server", port: 8080, health: "/healthz" }
}
```

### Layer 3: `contract` — the behavioral promise

The primary unit of verification. Declares what the system promises and proves it.

**Contains**: fields (input), `action` block (implementation), invariants, scenarios
**Lives in**: top level or inside a scope

```
contract <Name>(
  <name>: <type>,
  <name>: <type> { constraint },
) -> <ReturnModel> {
  # action — how to execute (uses fields implicitly, no signature)
  action {
    <steps...>
    return <expr>
  }

  # verification — what must hold
  invariant <name> { <assertions> }
  scenario <name> { given { ... } then { ... } }
  scenario <name> { when { ... } then { ... } }
}
```

**With model inheritance** (for imported schemas):
```
contract <Name>: <InputModel> -> <ReturnModel> {
  constrain { <additional constraints on inherited fields> }
  action { ... }
  invariant ...
}
```

**Field resolution rules:**
- Bare names (`from`, `amount`) → contract fields (input)
- `output.<field>` → return type fields
- `config.<key>` → spec-level config values

### Layer 4: `scope` — optional grouping

Groups related contracts that share lifecycle hooks. Not required.

```
scope <name> {
  before { <setup steps> }
  after { <teardown steps> }

  contract <Name> -> <ReturnModel> { ... }
  contract <Name> -> <ReturnModel> { ... }
}
```

### Layer 5: Verification members — inside contracts

**`invariant`** — universal law, must hold for ALL generated inputs.
```
invariant <name> {
  <assertion>
}
```

**`scenario` with `given`** — concrete smoke test, fixed inputs, runs once.
```
scenario <name> {
  given { <field>: <value> }
  then { <assertion> }
}
```

**`scenario` with `when`** — generative test, runtime generates many matching inputs.
```
scenario <name> {
  when { <predicate over input fields> }
  then { <assertion> }
}
```

### Construct placement summary

| Construct | Lives in | Contains |
|-----------|----------|----------|
| file | filesystem | everything (file = spec) |
| `include` | top level only | imports declarations from another file |
| `model` | top level | fields |
| `enum` | top level | variants |
| `config` | top level | key-value constants |
| `action` | top level | imperative steps (no verification) |
| `scope` | top level | before, after, contracts |
| `contract` | top level or scope | parens signature (fields), body: action block, invariants, scenarios |
| `invariant` | contract | assertions |
| `scenario` | contract | given/when + then |

## Syntax Changes Summary

### Breaking changes
| Change | v3 | v4 |
|--------|----|----|
| Spec wrapper | `spec Name { ... }` | No wrapper — file is the spec |
| Logical operators | `&&`, `\|\|`, `!` | `and`, `or`, `not` |
| Contract structure | `scope { contract { input/output/action: name } }` | `contract Name(field: type, ...) -> ReturnType { action { }, invariants, scenarios }` — input fields in signature parens |
| Output field refs | bare names in assertions | `output.` prefix required |
| Scope role | owns single contract + invariants + scenarios | optional grouping for shared before/after |
| Include semantics | Token splicing, anywhere in block | Top-level-only declaration import, include-once |
| Description | `description: "..."` inside spec block | Removed (use comments, filename is identity) |

### Additive features
| Feature | Syntax |
|---------|--------|
| Named enums | `enum Role { admin, user, viewer }` with `Role.admin` refs |
| `in` keyword | `status in ("pending", "active")` — parenthesis form preferred; bracket form `[...]` also accepted |
| Underscore numeric separators | `1_000_000`, `3.14_159` |
| `implies` operator | `a implies b` (lowest precedence) |
| State-dependent field presence | `tracking: string when status == "shipped"` |
| Config block | `config { max_retries: 3 }` with `config.key` refs |
| Contract model inheritance | `contract Foo: InputModel -> OutputModel { constrain { ... } }` |
| Glob execution | `specrun verify specs/*.spec` runs each match independently |
| Reserve `where` | Keyword reserved for future use |

## Full Example — v4

### Shared infrastructure (shared/infra.spec)
```
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

action login(username: string, password: string) {
  let result = http.post("/api/auth/login", { username: username, password: password })
  http.header("Authorization", "Bearer " + result.body.access_token)
}
```

### Shared models (shared/models.spec)
```
model Account {
  id: string
  balance: int { balance >= 0 }
}

enum TransferError { insufficient_funds, invalid_amount, same_account }
```

### Transfer spec (transfer.spec)
```
include "shared/infra.spec"
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

  contract Transfer(
    from: Account,
    to: Account,
    amount: int { 0 < amount <= from.balance },
  ) -> TransferResult {
    action {
      return http.post("/api/v1/accounts/transfer", {
        from: from, to: to, amount: amount
      })
    }

    invariant conservation {
      output.error == null implies
        output.from.balance + output.to.balance
          == from.balance + to.balance
    }

    invariant non_negative {
      output.from.balance >= 0
      output.to.balance >= 0
    }

    invariant no_mutation_on_error {
      output.error != null implies
        output.from.balance == from.balance
        and output.to.balance == to.balance
    }

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

    scenario overdraft {
      when { amount > from.balance }
      then { output.error == TransferError.insufficient_funds }
    }

    scenario zero_transfer {
      when { amount == 0 }
      then { output.error == TransferError.invalid_amount }
    }
  }
}
```

### CLI usage
```bash
specrun verify transfer.spec              # verify single spec
specrun verify specs/*.spec               # verify all specs independently
specrun verify specs/**/*.spec            # recursive glob
```

## Implementation Order

Ascending blast radius. Each step must pass `go test ./...` before proceeding.

### Step 1: Module path bump (DONE)

`github.com/bamsammich/speclang/v3` → `github.com/bamsammich/speclang/v4` in `go.mod` and all Go imports.

### Step 2: Preserve v3 parser (DONE)

Copy `internal/parser/` → `internal/v3parser/` before any modifications.

### Step 3: Lexer changes (DONE)

Word operators, new keywords, underscore numerics, arrow token.

### Step 4: AST changes

Restructure AST types for v4:

- **Remove `Spec.Name`** — filename is identity, no wrapper
- **Remove `Spec.Description`** — dropped
- **Contract struct**: `Name`, `Params []Field` (signature parens), `Inherits string`, `Constraints []Expr`, `ReturnType TypeExpr`, `Action *ActionBlock`, `Invariants`, `Scenarios`
- **ActionBlock struct**: `Body []Step` (no signature — uses contract fields)
- **Spec struct**: add `Contracts []*Contract`, `Enums []*NamedEnum`, `Config map[string]Expr`. Keep `Models`, `Actions`, `Scopes`, `AdapterConfigs`, `Services`
- **Scope struct**: add `Contracts []*Contract`, remove single `Contract` pointer. Keep `Before`/`After`
- **Field struct**: add `When Expr` for state-dependent presence
- **NamedEnum struct**: `Name string`, `Variants []string`

**Files**: `pkg/spec/ast.go`

### Step 5: Parser changes

- **Remove `spec Name { }` wrapper parsing** — top-level declarations parsed directly
- **Include-once semantics** — track included paths, skip duplicates
- **Include is top-level only** — error if include appears inside a block
- **Contract parsing**: `contract Name(field: type, ...) -> Type { action { body } invariant... scenario... }` — fields in signature parens
- **Contract inheritance**: `contract Name: Model -> Type { constrain { ... } ... }`
- **Named enums**: `enum Name { variant1, variant2 }`
- **Config block**: `config { key: expr }` at top level
- **State-dependent fields**: `field: type when condition`
- **`in` operator**: RHS as parenthesized list
- **`implies` operator**: lowest precedence

**Files**: `internal/parser/parser.go`, `internal/parser/parser_test.go`, `internal/parser/include.go`

### Step 6: Validator changes

- Resolve named enum references (enum registry)
- Validate contract structure (fields, action block, return type resolves)
- Enforce `output.` prefix in assertion contexts
- Validate `constrain` expressions reference inherited model fields
- Validate `when` expressions reference sibling fields, reject circular deps
- Resolve `config.key` references

**Files**: `internal/validator/validator.go`

### Step 7: Generator changes

- Operator evaluation: `"and"`, `"or"`, `"not"`, `"in"`, `"implies"`
- Generate from named enum variants
- Generate state-dependent fields (topological order, evaluate `when`)
- Resolve `config.key` in expressions
- Generate contract fields as input (new contract structure)

**Files**: `internal/generator/generator.go`

### Step 8: Runner changes

- Adapt to new contract structure (fields = input, action block = execution, return type = output)
- Resolve `output.` prefixed refs against return type
- Resolve bare refs against contract fields
- Handle contract-in-scope (inherit before/after) and standalone contracts
- Resolve `config.key`

**Files**: `internal/runner/runner.go`

### Step 9: CLI changes

- `specrun verify` accepts glob patterns, runs each matched file as independent spec unit
- Skip files with no contracts (log note, not error)
- `specrun parse` works without spec wrapper

**Files**: `cmd/specrun/main.go`

### Step 10: Importers

**OpenAPI** (`internal/openapi/models.go`):
- Generate contracts from operations (request body → contract fields, response → return type model)

**Protobuf** (`internal/proto/`):
- Generate contracts from RPCs (request message → inherited model, response → return type)

### Step 11: Migration tool (v3 → v4)

**Token-level**: `&&`→`and`, `||`→`or`, `!`→`not `

**Structural** (AST-based, uses preserved v3 parser):
- Strip `spec Name { }` wrapper
- `scope { contract { input/output } action name(...) { } invariant... }` → `contract Name -> OutputModel { fields action { body } invariant... }`
- Extract output fields into a model declaration
- Move invariants/scenarios from scope into contract
- Wrap in scope if before/after exists
- Add `output.` prefix to bare output field refs in assertions
- Convert token-splicing includes to top-level includes

**Files**: `internal/v3parser/`, `internal/migrate/v4.go`, `internal/migrate/v4_test.go`, `cmd/specrun/main.go`

### Step 12: Update all spec files

Rewrite all `.spec` files in `specs/`, `testdata/`, `examples/` to v4 syntax. Add self-verification entries for new features.

### Step 13: Documentation & skills

- `docs/language-reference.md` — full rewrite for v4
- `docs/v4-syntax.md` — layer-by-layer syntax structure
- `docs/migration-v4.md` — migration guide
- `CLAUDE.md` — update version refs, settled decisions, project structure
- `skills/author/SKILL.md` + `skills/author/references/api_reference.md` — syntax updates
- `skills/verify/SKILL.md` — update if needed
- `docs/adapters/*.md`, `docs/imports/*.md` — update examples

## Verification

1. `go test ./...` after every step
2. `go build ./cmd/specrun` after every step
3. After Step 12: `SPECRUN_BIN=./specrun ./specrun verify specs/speclang.spec`
4. After Step 11: migrate v3 fixtures → validate round-trip
5. Final: full test suite + self-verification + `go vet`

## Risk Areas

- **No spec wrapper → parser entry point**: Parser currently expects `spec Name {`. Must be rewritten to parse top-level declarations directly. This is a significant parser change.
- **Include-once tracking**: Must track resolved absolute paths. Relative path resolution must be consistent.
- **Glob execution**: Must handle the case where included files are also matched by the glob (skip gracefully).
- **Contract/scope interaction**: Runner must know which scope a contract belongs to for before/after inheritance.
- **Model inheritance resolution**: `contract Foo: Bar` requires Bar resolved first. Reject circular inheritance.
- **`enum` keyword collision**: Must handle both `enum(...)` inline and `enum Name { }` declaration.
- **State-dependent field generation**: Topological sort needed. Circular `when` deps rejected.
- **Migration complexity**: Stripping the spec wrapper + restructuring contracts is a multi-pass transformation.

## PR Topology

Single branch `v4` off `main`. One commit per step. One PR to merge `v4` → `main`.
