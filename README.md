# SpecLang

A specification language for AI-driven software development that serves as both a human-readable roadmap and a generative verification runtime against black-box systems.

## Why SpecLang?

LLMs tasked with writing code to satisfy a specification will optimize against visible test suites -- hardcoding outputs, writing degenerate implementations, gaming the letter of the spec while violating its spirit. SpecLang solves this: you write one file that the LLM reads to understand what to build, and the runtime reads to generate unbounded, unpredictable test cases at verification time. You can't game a test you can't predict.

## Install

```bash
go install github.com/bamsammich/speclang/v2/cmd/specrun@latest
```

Or from source:

```bash
git clone https://github.com/bamsammich/speclang.git
cd speclang
go build -o specrun ./cmd/specrun
```

## Quick Example

```
spec TransferAPI {
  target { base_url: env(APP_URL, "http://localhost:8080") }

  model Account { id: string  balance: int }

  scope transfer {
    use http
    config { path: "/api/v1/accounts/transfer"  method: "POST" }

    contract {
      input {
        from: Account
        to: Account
        amount: int { 0 < amount <= from.balance }
      }
      output { from: Account  to: Account  error: string? }
    }

    invariant conservation {
      when error == null:
        output.from.balance + output.to.balance
          == input.from.balance + input.to.balance
    }

    scenario success {
      given {
        from: { id: "alice", balance: 100 }
        to: { id: "bob", balance: 50 }
        amount: 30
      }
      then {
        from.balance: from.balance - amount
        to.balance: to.balance + amount
      }
    }

    scenario overdraft {
      when { amount > from.balance }
      then { error: "insufficient_funds" }
    }
  }
}
```

## Run

```bash
APP_URL=http://localhost:8080 specrun verify examples/transfer.spec
```

```
Verifying AccountAPI (seed=42, iterations=100)

  scope transfer:
    ✓ scenario success
    ✓ scenario overdraft (100 inputs)
    ✓ invariant conservation (100 inputs)

Scenarios:  2/2 passed
Invariants: 1/1 passed
```

Failures show shrunk counterexamples -- the minimal input that reproduces the bug.

## Three Levels of Verification

| Type | What it tests | How it works |
|------|--------------|--------------|
| `scenario` with `given` | One specific case | Fixed inputs, runs once |
| `scenario` with `when` | A class of inputs | Predicate over input space, runtime generates many |
| `invariant` | A universal law | Must hold for ALL valid inputs |

## Library Usage

SpecLang is available as a Go package for programmatic verification:

```go
import (
	"github.com/bamsammich/speclang/v2/pkg/spec"
	"github.com/bamsammich/speclang/v2/pkg/specrun"
)

s, _ := specrun.ParseFile("my.spec", nil)
result, _ := specrun.Verify(s, nil, specrun.Options{Seed: 42, Iterations: 100})
```

See [Package Guide](docs/package.md) for full documentation including custom adapters, programmatic spec construction, and result handling.

## Documentation

| Guide | Description |
|-------|-------------|
| [Getting Started](docs/getting-started.md) | Install, first spec, verify |
| [Language Reference](docs/language-reference.md) | Complete syntax reference |
| [HTTP Adapter](docs/adapters/http.md) | REST API testing |
| [Process Adapter](docs/adapters/process.md) | CLI/subprocess testing |
| [Playwright Adapter](docs/adapters/playwright.md) | Browser UI testing |
| [OpenAPI Import](docs/imports/openapi.md) | Import from OpenAPI schemas |
| [Protobuf Import](docs/imports/protobuf.md) | Import from proto files |
| [Target Services](docs/services.md) | Docker containers as test infrastructure |
| [Package Guide](docs/package.md) | Go library integration (programmatic use) |
| [Self-Verification](docs/self-verification.md) | How speclang tests itself |

## Claude Code Plugin

SpecLang ships as a [Claude Code](https://claude.com/claude-code) plugin with two skills:

- **`speclang:author`** -- converts natural language requirements into `.spec` files. Trigger with `/spec`.
- **`speclang:verify`** -- runs `specrun verify` as a gate before merging. Trigger with `/verify-spec`.

Install in Claude Code:

```
/plugin marketplace add bamsammich/speclang-marketplace
/plugin install speclang@speclang-marketplace
```

## License

MIT
