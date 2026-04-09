# Speclang v4: Language Evolution

## Context

Evaluation of [Allium](https://juxt.github.io/allium/) identified readability improvements for speclang. Combined with long-standing friction around contract/action redundancy and implicit field resolution, v4 is a major language redesign: contracts become self-contained behavioral promises, operators become English words, and several new expression features ship.

## v4 Language Structure

### Layer 1: `spec` — the root container

Everything lives inside a single spec. It's the file-level declaration.

```
spec AccountAPI {
  description: "..."

  config { ... }           # spec-level constants
  http { ... }             # adapter configuration
  services { ... }         # Docker infrastructure
  model Account { ... }    # data structures
  enum Role { ... }        # named enumerations
  action login(...) { ... } # reusable procedures
  contract Transfer -> TransferResult { ... }  # behavioral promises
  scope admin { ... }      # optional grouping
  include "other.spec"
  import openapi("schema.yaml")
}
```

### Layer 2: Declarations — the building blocks

**`model`** — a data structure. Fields with types and optional constraints.
```
model Account {
  id: string
  balance: int { balance >= 0 }
  tracking: string when status == "shipped"  # state-dependent field
}
```

**`enum`** — a named set of values. Referenced by name as a field type. Variants referenced with qualified syntax `EnumName.variant` in assertions, constraints, and predicates.
```
enum Role { admin, user, viewer }

# As a field type
model User { role: Role }

# In assertions — qualified reference, validator checks variant exists
then { output.role == Role.admin }

# In predicates
when { status in (OrderStatus.pending, OrderStatus.cancelled) }

# In constraints
model Order {
  status: OrderStatus
  tracking: string when status == OrderStatus.shipped
}
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
**Lives in**: spec level or inside a scope

```
contract <Name> -> <ReturnModel> {
  # fields — the input
  <name>: <type>
  <name>: <type> { constraint }

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
| `spec` | file root | everything |
| `model` | spec | fields |
| `enum` | spec | variants |
| `config` | spec | key-value constants |
| `action` | spec | imperative steps (no verification) |
| `scope` | spec | before, after, contracts |
| `contract` | spec or scope | fields, action block, invariants, scenarios |
| `invariant` | contract | assertions |
| `scenario` | contract | given/when + then |

## Syntax Changes Summary

### Breaking changes
| Change | v3 | v4 |
|--------|----|----|
| Logical operators | `&&`, `\|\|`, `!` | `and`, `or`, `not` |
| Contract structure | `scope { contract { input/output/action: name } }` | `contract Name -> ReturnType { fields, action { }, invariants, scenarios }` |
| Output field refs | bare names in assertions | `output.` prefix required |
| Scope role | owns single contract + invariants + scenarios | optional grouping for shared before/after |

### Additive features
| Feature | Syntax |
|---------|--------|
| Named enums | `enum Role { admin, user, viewer }` |
| `in` keyword | `status in ("pending", "active")` |
| Underscore numeric separators | `1_000_000`, `3.14_159` |
| `implies` operator | `a implies b` (lowest precedence) |
| State-dependent field presence | `tracking: string when status == "shipped"` |
| Config block | `config { max_retries: 3 }` with `config.key` refs |
| Contract model inheritance | `contract Foo: InputModel -> OutputModel { constrain { ... } }` |
| Reserve `where` | Keyword reserved for future use |

## Full Example — v4

```
spec AccountAPI {
  description: "REST API for inter-account money transfers"

  http {
    base_url: service(app)
  }

  services {
    app {
      build: "./server"
      port: 8080
      health: "/healthz"
    }
  }

  config {
    max_transfer: 1_000_000
  }

  model Account {
    id: string
    balance: int { balance >= 0 }
  }

  model TransferResult {
    from: Account
    to: Account
    error: string?
  }

  enum TransferError { insufficient_funds, invalid_amount, same_account }

  action login(username: string, password: string) {
    let result = http.post("/api/auth/login", { username: username, password: password })
    http.header("Authorization", "Bearer " + result.body.access_token)
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
        then { output.error == "insufficient_funds" }
      }

      scenario zero_transfer {
        when { amount == 0 }
        then { output.error == "invalid_amount" }
      }
    }
  }
}
```

## Implementation Order

Ascending blast radius. Each step must pass `go test ./...` before proceeding.

### Step 1: Module path bump

`github.com/bamsammich/speclang/v3` → `github.com/bamsammich/speclang/v4` in `go.mod` and all Go imports.

### Step 2: Preserve v3 parser

Copy `internal/parser/` → `internal/v3parser/` before any modifications. The migration tool (Step 10) needs to parse v3 syntax.

### Step 3: Lexer changes

All token-level changes in one step:

- **Reserve keywords**: `where`, `in`, `implies`, `enum`, `constrain`
- **Word operators**: add `"and"`, `"or"`, `"not"` to keyword map → `TokenAnd`, `TokenOr`, `TokenNot`. Remove `&`/`|` operator lexing. `!` alone becomes error; only `!=` survives
- **Underscore numerics**: modify `lexNumber()` to consume `_` between digits

**Files**: `internal/parser/lexer.go`, `internal/parser/lexer_test.go`

### Step 4: AST changes

Restructure AST types for v4:

- **Contract struct**: `Name`, `Fields []Field`, `Inherits string`, `Constraints []Expr`, `ReturnType TypeExpr`, `Action *ActionBlock`, `Invariants`, `Scenarios`
- **ActionBlock struct**: `Body []Step` (no signature — uses contract fields)
- **Spec struct**: add `Contracts []*Contract`, `Enums []*NamedEnum`, `Config map[string]Expr`
- **Scope struct**: add `Contracts []*Contract`, remove single `Contract` pointer. Keep `Before`/`After`
- **Field struct**: add `When Expr` for state-dependent presence
- **NamedEnum struct**: `Name string`, `Variants []string`
- Update `FormatExpr`: space after `"not"`, handle `"in"`, `"implies"`
- All operator strings: `"and"`, `"or"`, `"not"`, `"in"`, `"implies"`

**Files**: `pkg/spec/ast.go`

### Step 5: Parser changes

- **Contract parsing**: `contract Name -> Type { fields... action { body } invariant... scenario... }` and `contract Name: Model -> Type { constrain { ... } ... }`. Contracts at spec level or inside scope
- **Named enums**: `enum Name { variant1, variant2 }`. Handle `TokenEnum` for both named declarations and inline `enum(...)`
- **Config block**: `config { key: expr }` at spec level
- **State-dependent fields**: `field: type when condition` in model/contract field parsing
- **`in` operator**: `TokenIn` at comparison precedence, RHS as parenthesized list
- **`implies` operator**: `precImplies` below `precOr`, renumber all precedence constants
- **Operator strings**: `opStrings` map updated, `parseUnary` emits `"not"`

**Files**: `internal/parser/parser.go`, `internal/parser/parser_test.go`, `internal/parser/quantifier_test.go`

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

**Files**: `internal/generator/generator.go`, `internal/generator/exists_test.go`

### Step 8: Runner changes

- Adapt to new contract structure (fields = input, action block = execution, return type = output)
- Resolve `output.` prefixed refs against return type
- Resolve bare refs against contract fields
- Handle contract-in-scope (inherit before/after) and standalone contracts
- Resolve `config.key`

**Files**: `internal/runner/runner.go`

### Step 9: Importers

**OpenAPI** (`internal/openapi/models.go`):
- `Op: "and"` (was `"&&"`)
- Generate contracts from operations (request body → contract fields, response → return type model)

**Protobuf** (`internal/proto/`):
- Generate contracts from RPCs (request message → inherited model, response → return type)

### Step 10: Migration tool (v3 → v4)

Two transformation categories:

**Token-level**: `&&`→`and`, `||`→`or`, `!`→`not ` (respecting string boundaries, not touching `!=`)

**Structural** (AST-based, uses preserved v3 parser):
- `scope { contract { input/output } action name(...) { } invariant... }` → `contract Name -> OutputModel { fields action { body } invariant... }`
- Extract output fields into a model declaration
- Move invariants/scenarios from scope into contract
- Wrap in scope if before/after exists
- Add `output.` prefix to bare output field refs in assertions

**Files**: `internal/v3parser/` (preserved copy), `internal/migrate/v4.go`, `internal/migrate/v4_test.go`, `internal/migrate/testdata/`, `cmd/specrun/main.go`

### Step 11: Update all spec files

Rewrite all `.spec` files in `specs/`, `testdata/`, `examples/` to v4 syntax. Add self-verification entries for new features.

### Step 12: Documentation & skills

- `docs/language-reference.md` — full rewrite for v4
- `docs/v4-syntax.md` — new: the layer-by-layer syntax structure from this design (spec → declarations → contract → scope → verification members)
- `docs/migration-v4.md` — new migration guide
- `CLAUDE.md` — update version refs, settled decisions, project structure
- `skills/author/SKILL.md` + `skills/author/references/api_reference.md` — syntax updates
- `skills/verify/SKILL.md` — update if needed
- `docs/adapters/*.md`, `docs/imports/*.md` — update examples

## Verification

1. `go test ./...` after every step
2. `go build ./cmd/specrun` after every step
3. After Step 11: `SPECRUN_BIN=./specrun ./specrun verify specs/speclang.spec`
4. After Step 10: migrate v3 fixtures → validate round-trip
5. Final: full test suite + self-verification + `go vet`

## Risk Areas

- **Contract/scope interaction**: Runner must know which scope a contract belongs to for before/after inheritance
- **Model inheritance resolution**: `contract Foo: Bar` requires Bar resolved first. Reject circular inheritance
- **`enum` keyword collision**: Currently matched as identifier (`typeEnum = "enum"`). Must handle both `enum(...)` inline and `enum Name { }` declaration
- **State-dependent field generation**: Topological sort needed. Circular `when` deps rejected
- **`not` spacing in FormatExpr**: `not expr` needs space; `-expr` does not
- **Migration structural complexity**: scope→contract restructuring is AST-level transformation, needs comprehensive fixtures
- **v3 parser preservation**: Must copy `internal/parser/` BEFORE modifying

## PR Topology

Single branch `v4` off `main`. One commit per step. One PR to merge `v4` → `main`.
