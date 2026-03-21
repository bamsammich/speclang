# speclang

A specification language for AI-driven software development that serves as both a human-readable roadmap and a generative verification runtime against black-box systems.

## Why speclang?

LLMs tasked with writing code to satisfy a specification will optimize against visible test suites — hardcoding outputs, writing degenerate implementations, gaming the letter of the spec while violating its spirit.

speclang solves this with a single file that serves two purposes:

1. **The LLM reads it** to understand what to build
2. **The runtime reads it** to generate unbounded, unpredictable test cases at verification time

The test surface is unknowable to the implementer because inputs are generated from declared constraints, not enumerated. You can't game a test you can't predict.

### Three levels of verification strength

| Type | What it tests | How it works |
|------|--------------|--------------|
| `scenario` with `given` | One specific case | Fixed inputs, runs once — a smoke test |
| `scenario` with `when` | A class of inputs | Predicate defines which inputs; runtime generates many |
| `invariant` | A universal law | Must hold for ALL valid inputs from the full space |

## Getting Started

### Prerequisites

- Go (latest stable)

### Build

```bash
go build -o specrun ./cmd/specrun
```

### Write a spec

Here's a minimal spec for an HTTP API:

```
use http

spec AccountAPI {
  description: "REST API for inter-account money transfers"

  target {
    base_url: env(APP_URL, "http://localhost:8080")
  }

  model Account {
    id: string
    balance: int
  }

  scope transfer {
    config {
      path: "/api/v1/accounts/transfer"
      method: "POST"
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
    }

    # Money is neither created nor destroyed.
    invariant conservation {
      when error == null:
        output.from.balance + output.to.balance
          == input.from.balance + input.to.balance
    }

    # Balances must never go negative.
    invariant non_negative {
      output.from.balance >= 0
      output.to.balance >= 0
    }

    # Smoke test with concrete values.
    scenario success {
      given {
        from: { id: "alice", balance: 100 }
        to: { id: "bob", balance: 50 }
        amount: 30
      }
      then {
        from.balance: 70
        to.balance: 80
        error: null
      }
    }

    # Any overdraft must be rejected.
    scenario overdraft {
      when {
        amount > from.balance
      }
      then {
        error: "insufficient_funds"
      }
    }
  }
}
```

### Verify an implementation

Start your server, then run:

```bash
APP_URL=http://localhost:8080 ./specrun verify examples/transfer.spec
```

Output:

```
verifying examples/transfer.spec (AccountAPI) with seed=42, iterations=100

  scope transfer:
    ✓ scenario success
    ✓ scenario overdraft (100 inputs)
    ✓ scenario zero_transfer (100 inputs)
    ✓ invariant conservation (100 inputs)
    ✓ invariant non_negative (100 inputs)
    ✓ invariant no_mutation_on_error (100 inputs)

Scenarios:  3/3 passed
Invariants: 3/3 passed
```

### Other commands

```bash
./specrun parse examples/transfer.spec                      # parse spec, output AST as JSON
./specrun generate examples/transfer.spec --scope transfer  # generate one random input
./specrun verify --json examples/transfer.spec              # verify with JSON output
```

## Claude Code Plugin

speclang ships as a Claude Code plugin with two skills that integrate specification-driven development into your AI workflow.

### Installation

In Claude Code, run:

```
/plugin marketplace add bamsammich/speclang-marketplace
/plugin install speclang@speclang-marketplace
```

### Skills

#### `speclang:author`

Converts natural language requirements into speclang spec files. Use it when describing a new feature or behavior — it will generate the `.spec` file with models, contracts, invariants, and scenarios.

Trigger with `/spec` or by describing a feature in a project with `.spec` files.

#### `speclang:verify`

Runs `specrun verify` against your spec files before merging. Acts as a gate — if verification fails, the implementation needs fixing, not the spec.

Trigger with `/verify-spec` or automatically before creating PRs in projects with `.spec` files.

### Session hook

When working in a project with `.spec` files, the plugin automatically detects them and reminds Claude that speclang skills are available.

## Plugins

speclang uses a plugin architecture for interacting with different systems:

| Plugin | Use case | Target config | Scope config |
|--------|----------|---------------|--------------|
| `http` | REST APIs | `base_url` | `path`, `method` |
| `process` | CLI tools / subprocesses | `command` | `args` |

## License

MIT
