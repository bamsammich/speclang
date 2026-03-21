# Speclang Syntax Reference

## File Structure

```
use <plugin>                         # required: which adapter to use
include "<path>"                     # optional: top-level include

spec <Name> {

  description: "<text>"              # optional: AI context about the system

  target {
    base_url: env(APP_URL)           # plugin-dependent config
  }

  include "<path>"                   # spec-body include

  model <Name> {
    <field>: <type>
    <field>: <type> { <constraint> }
  }

  scope <name> {
    config {                         # opaque key-value pairs for adapter
      path: "/api/v1/resource"
      method: "POST"
    }

    contract {
      input {
        <field>: <type>
      }
      output {
        <field>: <type>
      }
    }

    invariant <name> {
      when <predicate>:              # optional guard
        <assertion>
    }

    scenario <name> {
      given { ... }                  # concrete values (smoke test)
      when { ... }                   # predicate (generative)
      then { ... }                   # assertions
    }
  }
}
```

## Types

- `int` — integer
- `string` — string
- `bool` — boolean
- `any` — untyped (passed through)
- `<ModelName>` — reference to a defined model
- Append `?` for optional: `string?`

## Expressions

- **Literals**: `42`, `"hello"`, `true`, `false`, `null`
- **Field references**: `from.balance`, `output.error`
- **Environment**: `env(VAR)`, `env(VAR, "default")`
- **Objects**: `{ id: "alice", balance: 100 }`
- **Operators**: `==`, `!=`, `>`, `<`, `>=`, `<=`, `+`, `-`, `*`, `&&`, `||`, `!`

## Comments

Lines starting with `#` are comments. Use them to explain intent.

```
# Money is neither created nor destroyed on successful transfers.
invariant conservation { ... }
```

## Three Scenario Types (Ascending Verification Strength)

### 1. `scenario` with `given` — Concrete smoke test

Fixed input values. Runs once. Documents expected behavior.

```
scenario success {
  given {
    from: { id: "alice", balance: 100 }
    to: { id: "bob", balance: 50 }
    amount: 30
  }
  then {
    from.balance: 70
    to.balance: 80
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
    error: "insufficient_funds"
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

## Constraints

Model fields can have constraints that bound the generator:

```
model Transfer {
  amount: int { 0 < amount <= from.balance }
}
```

## Include Directive

Split specs across files. Paths are relative to the including file.

```
include "models/account.spec"
include "scopes/transfer.spec"
```

## Available Plugins

### `use http`

For HTTP APIs. Scope config uses `path` and `method`. Target uses `base_url`.

### `use process`

For CLI tools. Runs subprocesses, captures exit code/stdout/stderr. Target uses `command`. Scope config uses `args`.
