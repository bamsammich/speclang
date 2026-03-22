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

### Install

**With Go (macOS or Linux):**

```bash
go install github.com/bamsammich/speclang/cmd/specrun@latest
```

**Pre-built binary (Linux only):**

Download from [releases](https://github.com/bamsammich/speclang/releases). macOS binaries are not provided because unsigned binaries are blocked by Gatekeeper — use `go install` instead.

**From source:**

```bash
go build -o specrun ./cmd/specrun
```

### Write a spec

Here's a minimal spec for an HTTP API. Note that `use <plugin>` is declared inside each `scope`, not at the top of the file:

```
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
    use http

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

    # Smoke test: expected values computed from input, not hardcoded.
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

## OpenAPI Import

If you have an existing OpenAPI 3.x schema, you can import models and scope scaffolds directly:

```
spec MyAPI {
  target {
    base_url: env(APP_URL, "http://localhost:8080")
  }

  import openapi("schema.yaml")
}
```

Imported scopes will need a `use http` declaration added to each scope, along with invariants and scenarios.

This generates models from `components/schemas` and scopes from `paths`, letting you layer invariants and scenarios on top of your existing API definition.

See [docs/openapi-import.md](docs/openapi-import.md) for the full guide, type mapping, and limitations.

## Protobuf Import

Import models and scopes from protobuf service definitions:

```
import proto("service.proto")
```

This generates models from `message` definitions and scopes from unary `rpc` methods.

See [docs/protobuf-import.md](docs/protobuf-import.md) for details.

## Plugins

speclang uses a plugin architecture for interacting with different systems. Each `scope` declares `use <plugin>` to select which plugin drives it — a single spec can mix plugins across scopes.

| Plugin | Use case | Target config | Scope config |
|--------|----------|---------------|--------------|
| `http` | REST APIs | `base_url` | `path`, `method` |
| `process` | CLI tools / subprocesses | `command` | `args` |
| `playwright` | Browser UIs | `base_url`, `headless`, `timeout` | `url` |

### Playwright

Use `playwright` to write specs for browser-driven UIs. It controls a real browser via [Playwright](https://playwright.dev/).

**Requirements:** Install Playwright browsers before running:

```bash
npx playwright install chromium
```

**Locators:** Declare named element locators at spec level, then reference them by name in actions and assertions:

```
locators {
  username_field: [data-testid=username]
  submit_btn:     [data-testid=submit]
  welcome:        [data-testid=welcome]
  error_msg:      [data-testid=error]
}
```

**Assertion syntax:** Use `locator@playwright.property: expected` in `then` blocks:

```
then {
  welcome@playwright.visible: true
  welcome@playwright.text: "Welcome, alice"
}
```

**Full example:**

```
spec LoginApp {
  description: "Login UI verification"

  target {
    base_url: env(APP_URL, "http://localhost:3000")
  }

  locators {
    username_field: [data-testid=username]
    password_field: [data-testid=password]
    submit_btn:     [data-testid=submit]
    welcome:        [data-testid=welcome]
    error_msg:      [data-testid=error]
  }

  action login(user, pass) {
    playwright.fill(username_field, user)
    playwright.fill(password_field, pass)
    playwright.click(submit_btn)
  }

  scope login {
    use playwright

    config {
      url: "/login"
    }

    contract {
      input {
        user: string
        pass: string
      }
      output {
        ok: bool
      }
    }

    scenario successful_login {
      given {
        login("alice", "secret")
        user: "alice"
      }
      then {
        welcome@playwright.visible: true
        welcome@playwright.text: "Welcome, alice"
      }
    }

    scenario invalid_credentials {
      when {
        pass != "secret"
      }
      then {
        error_msg@playwright.visible: true
        welcome@playwright.visible: false
      }
    }

    # Welcome banner must never appear when login failed.
    invariant no_welcome_on_failure {
      when ok == false:
        welcome@playwright.visible: false
    }
  }
}
```

## License

MIT
