# Migrating from v3 to v4

## Why v4

v4 reshapes the language around three ideas surfaced by real authoring experience in v3:

- **Contracts as self-contained behavioral promises.** A contract now declares its inputs, execution (`action { }`), and the properties that must hold (invariants, scenarios) in one block. The v3 split — contract says *what*, separate action says *how*, and invariants/scenarios float around the scope — is gone.
- **Readability borrowed from Allium.** English word operators (`and`, `or`, `not`), a new `implies` connective, `in` for membership, and optional underscore numeric separators (`1_000_000`). `&&`/`||`/`!` no longer exist.
- **Stop implicit-namespace surprises.** Return-type fields must be prefixed `output.` in assertions. Contracts and scopes no longer share the same symbol pool.

Plus: named enums, `config { }` constants, state-dependent fields, contract inheritance from models, and glob execution at the CLI.

## Breaking changes

| Change | v3 | v4 |
|--------|----|----|
| Spec wrapper | `spec Name { ... }` around everything | No wrapper — file is the spec |
| Spec name / description | `spec Name { description: "..." }` | Removed; filename is identity, description goes in a comment |
| Logical operators | `&&`, `\|\|`, `!` | `and`, `or`, `not` (symbolic forms rejected by the lexer) |
| Contract structure | `scope s { contract { input { } output { } action: <name> } action <name>(...) { } invariant ... scenario ... }` | `contract Name -> ReturnModel { <input fields> action { <body> } invariant ... scenario ... }` |
| Output field refs | Bare names in `then` blocks | `output.` prefix required (`output.balance`, not `balance`) |
| Role of `scope` | Owns a single contract + its invariants/scenarios + its action | Optional grouping; only holds `before`/`after` + contracts |
| Action definition location | Top-level *or* inside a scope | Top-level only (reusable actions) or inside a contract as the `action { }` block |
| Actions with signatures in scopes | `scope s { action foo(...) { } }` | Inline the body into the contract's `action { }` block, or lift to a top-level `action` |
| `include` placement | Top-level or inside a `spec` block; token splicing | Still token-splicing under the hood; write at the top level (there's no wrapper to put it inside) |
| Includes with duplicates | Silently concatenated | Duplicate model/scope names across the resolved stream are errors |

## Additive features (no migration pain)

| Feature | Syntax |
|---------|--------|
| Named enums | `enum Role { admin, user, viewer }`, referenced as `Role.admin` |
| `in` operator | `status in ("pending", "active")` — parenthesis form preferred; `[...]` also accepted |
| `implies` connective | `a implies b` (lowest precedence; equivalent to `not a or b`) |
| Underscore numeric separators | `1_000_000`, `3.14_159` |
| State-dependent fields | `tracking: string when status == "shipped"` |
| `config { }` block | `config { max_transfer: 1_000_000 }`, referenced as `config.max_transfer` |
| Contract inheritance | `contract Pay: PaymentInput -> PaymentResult { constrain { amount > 0 } ... }` |
| Glob CLI execution | `specrun verify specs/**/*.spec` runs each file independently |
| Files without contracts | Logged and skipped, not errored |

## Mechanical migration

```bash
specrun migrate --to v4 my-spec.spec           # prints v4 to stdout
specrun migrate --to v4 my-spec.spec -w        # rewrites in place
specrun migrate --to v4 specs/*.spec -w        # batch
```

### What the tool handles automatically

- Strips the `spec Name { }` wrapper.
- Converts `scope { contract { input { } output { } action: <name> } }` → `contract <Name> -> <OutputModel> { <fields> action { <body> } ... }`, inlining the referenced action's body into the contract.
- Synthesizes an output model (`<ScopeName>Result`) when the v3 output block had multiple fields or a single primitive; falls through to the field's type when the output was a single model-typed field.
- Lifts invariants and scenarios from the scope into the contract.
- Keeps the scope wrapper *only* if `before`/`after` were present; otherwise emits the contract at top level.
- Prefixes bare output-field references with `output.` in invariants and `then`-block assertions.
- Rewrites `&&` → `and`, `||` → `or`, bare `!` → `not `.
- Re-emits `include` directives at the top level.

### What may need manual cleanup

- **Complex action bodies.** The migrator inlines the action referenced by `action:` into the contract's `action { }` block. Multi-step actions with intermediate `let` bindings that mixed adapter scopes may be worth lifting back to a top-level reusable `action`.
- **Scope nesting.** v3 specs that used multiple scopes to group related contracts convert cleanly, but nested or heavily reused `before`/`after` often read better as a top-level reusable `action` called from `before`.
- **Descriptions.** `description:` is dropped silently. If a description carries real meaning, add it as a leading `#` comment (or restructure the filename).
- **Bare refs that should have been `output.` in v3.** If a v3 `then` block had `balance == 100` and `balance` existed on both input and output, the migrator prefixes based on the output-field set — spot-check assertions that mixed both sides.
- **Named enums and `in`/`implies`/`config`.** The migrator never introduces these — they are additive. Adopt them by hand where they improve readability.

Always run `specrun parse <file>` after migrating, then `specrun verify` against a known-good implementation to confirm behavior is preserved.

## Worked example

### Before (v3)

```speclang
spec TransferService {
  description: "Bank account transfer API"

  http {
    base_url: service(app)
  }

  services {
    app { build: "./server", port: 8080, health: "/healthz" }
  }

  model Account {
    id: string
    balance: int { balance >= 0 }
  }

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
      when { amount > from.balance }
      then { error == "insufficient_funds" }
    }
  }
}
```

### After (v4)

```speclang
# Bank account transfer API

http {
  base_url: service(app)
}

services {
  app { build: "./server", port: 8080, health: "/healthz" }
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

scope transfer {
  contract Transfer -> TransferResult {
    from: Account
    to: Account
    amount: int { 0 < amount <= from.balance }

    action {
      let result = http.post("/api/v1/accounts/transfer", {
        from: from, to: to, amount: amount
      })
      return result.body
    }

    invariant conservation {
      when output.error == null:
        output.from.balance + output.to.balance
          == from.balance + to.balance
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
        output.from.balance == from.balance - amount
        output.to.balance == to.balance + amount
        output.error == null
      }
    }

    scenario overdraft {
      when { amount > from.balance }
      then { output.error == "insufficient_funds" }
    }
  }
}
```

Notable transformations:

- Spec wrapper and `description:` removed.
- `output { ... }` extracted into `model TransferResult { ... }`.
- `scope transfer` retained (no `before`/`after` in this example, but kept by the migrator when present; here it reads fine as a group).
- The v3 `action transfer(...) { }` body is inlined as the contract's `action { }`.
- `input.from.balance` in the v3 invariant becomes bare `from.balance` (the v4 contract field resolution makes the `input.` prefix unnecessary).
- All `then`-block assertions gained `output.` on their LHS.
- If the spec had used `&&`/`||`/`!`, those would be rewritten to `and`/`or`/`not`.

After migration, you can optionally modernize:

```speclang
enum TransferError { insufficient_funds, invalid_amount, same_account }
```

and in the scenario:

```speclang
scenario overdraft {
  when { amount > from.balance }
  then { output.error == TransferError.insufficient_funds }
}
```

## Gotchas

- **`output.` is required** even for a bare field name you could previously write. `balance == 100` is an input reference in v4 — if you meant the return value, write `output.balance == 100`.
- **Bare names in contracts are inputs.** v3's flat namespace mixed input and output; v4 splits them cleanly. Any assertion that ignores this split will either fail to parse (unknown field) or silently check the wrong side.
- **Includes inside blocks are a bad idea.** v3 allowed `include` anywhere because the token stream was spliced at the point of inclusion. v4 still splices mechanically, but there's no `spec { }` wrapper anymore — write all includes at the top level. A mid-block include will either splice invalidly or leak declarations out of the block and fail later.
- **Named enums and inline enums are different constructs.** `enum Role { admin }` declares a type; variants are `Role.admin` in expressions. `role: enum("admin","user")` is still an inline anonymous enum whose members are string literals — assertions use `role == "admin"`, not `role == Role.admin`.
- **v3 users who used `include` to pull whole scope files into a root spec** should consider whether those scopes should now be independently runnable specs verified via glob. In v4, every declaration in the include graph must be unique — a `duplicate declaration` error means two included files define the same name. The fix is either to factor the shared declaration into a library file that both specs include, or to drop the super-spec root entirely and run each spec independently with `specrun verify specs/*.spec`. See [Includes vs. glob execution](language-reference.md#includes-vs-glob-execution).
- **`description:` is gone.** Replace with a leading `#` comment, or let the filename carry the identity.
- **Symbolic operators are hard errors.** The lexer rejects `&&` (it sees a lone `&` and errors) and `!` as a unary. This catches half-migrated files immediately instead of silently misparsing.
- **Migration does not add new v4 idioms.** Named enums, `config.`, `implies`, `in`, state-dependent fields — all manual. The tool only translates what v3 expressed.

## CLI breaking changes

### `specrun verify --json` with a glob

When `--json` is combined with a glob pattern (`specrun verify 'specs/**/*.spec' --json`), the output format changed in v4:

- **v3:** A single JSON document (one top-level object) regardless of how many files were matched.
- **v4:** JSON Lines — one JSON object per matched spec file, one per line, written to stdout in the order files are verified.

Downstream consumers reading `--json` output for a single file are **not affected** — a single file still produces a single JSON object on stdout. Only pipelines that process multi-file glob output need updating. To migrate:

```bash
# v3 (single JSON document from a glob)
specrun verify 'specs/*.spec' --json | jq '.failures[]'

# v4 (JSON Lines — parse line by line)
specrun verify 'specs/*.spec' --json | jq -r '.failures[]?'
```

If you are using `jq` in a pipeline, add `-s` to slurp all lines into an array before processing, or switch to `jq -c` on the writing side and `jq -R 'fromjson'` on the reading side.

## Verification after migration

1. Parse: `specrun parse <file>` — confirms the file is syntactically valid v4 and passes type/semantic validation.
2. Verify: `specrun verify <file>` — runs the full suite against your implementation. A passing v3 spec should still pass in v4.
3. Diff-check the JSON AST if you need confidence that semantics match — parse with both the v4 binary and a preserved v3 binary, and compare the shape of fields/invariants/scenarios after migration.
4. For multi-file specs: `specrun verify specs/**/*.spec` runs each file independently. Keep the glob narrow while iterating.
