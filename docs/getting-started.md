# Getting Started

## Install

**With Go (recommended):**

```bash
go install github.com/bamsammich/speclang/v2/cmd/specrun@latest
```

**Pre-built binary (Linux only):**

Download from [releases](https://github.com/bamsammich/speclang/releases). macOS binaries are not provided because unsigned binaries are blocked by Gatekeeper -- use `go install` instead.

**From source:**

```bash
git clone https://github.com/bamsammich/speclang.git
cd speclang
go build -o specrun ./cmd/specrun
```

## Write Your First Spec

Create a file called `transfer.spec`:

```
spec TransferAPI {
  description: "Bank transfer endpoint"

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
        error: null
      }
    }
  }
}
```

This spec declares:

- A **model** (`Account`) describing the data shape
- A **scope** (`transfer`) targeting an HTTP endpoint
- A **contract** defining valid inputs and expected outputs
- An **invariant** that money is conserved across transfers
- A **scenario** with concrete inputs as a smoke test

## Run Verification

Start your server, then run:

```bash
APP_URL=http://localhost:8080 specrun verify transfer.spec
```

Sample output on success:

```
verifying transfer.spec (TransferAPI) with seed=42, iterations=100

  scope transfer:
    ✓ scenario success
    ✓ invariant conservation (100 inputs)

Scenarios:  1/1 passed
Invariants: 1/1 passed
```

## Interpreting Results

**Pass:** Each scenario and invariant shows a checkmark. Generative checks (invariants, `when` scenarios) show how many inputs were tested.

**Fail:** A failing check shows the counterexample -- the specific input that caused the failure, shrunk to a minimal reproducer:

```
  scope transfer:
    ✗ invariant conservation (failed after 23 inputs)
      counterexample (shrunk):
        from: { id: "", balance: 1 }
        to: { id: "", balance: 0 }
        amount: 1
      expected: output.from.balance + output.to.balance == input.from.balance + input.to.balance
      got: 0 + 0 == 1 + 0 (false)
```

The counterexample is binary-search shrunk to the smallest failing input, making it easier to diagnose the root cause.

## Other Commands

```bash
specrun parse transfer.spec                      # parse spec, output AST as JSON
specrun generate transfer.spec --scope transfer  # generate one random input
specrun verify transfer.spec --json              # verify with JSON output
```

## Self-Verification

Speclang verifies itself using its own spec language. Run:

```bash
SPECRUN_BIN=./specrun specrun verify specs/speclang.spec
```

See [Self-Verification](self-verification.md) for details on how this works.

## Self-Contained Specs with Docker

Instead of manually starting servers before verification, you can declare services directly in the spec. `specrun verify` will build, start, health-check, and tear down containers automatically.

```
spec TransferAPI {
  target {
    services {
      app {
        build: "./server"
        port: 8080
      }
    }
    base_url: service(app)
  }

  # ... models, scopes, etc.
}
```

Run verification -- no manual server startup needed:

```bash
specrun verify transfer.spec
```

The `service(app)` expression resolves to the running container's URL at runtime. Docker must be available on the host. See [Target Services](services.md) for the full guide.

## Next Steps

- [Language Reference](language-reference.md) -- complete syntax for models, types, expressions, constraints, and scenarios
- [Target Services](services.md) -- Docker containers as test infrastructure
- [HTTP Adapter](adapters/http.md) -- testing REST APIs
- [Process Adapter](adapters/process.md) -- testing CLI tools and subprocesses
- [Playwright Adapter](adapters/playwright.md) -- testing browser UIs
- [OpenAPI Import](imports/openapi.md) -- importing from OpenAPI schemas
- [Protobuf Import](imports/protobuf.md) -- importing from protobuf files
