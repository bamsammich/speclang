# Getting Started

## Install

**With Go (recommended):**

```bash
go install github.com/bamsammich/speclang/v4/cmd/specrun@latest
```

**From source:**

```bash
git clone https://github.com/bamsammich/speclang.git
cd speclang
go build -o specrun ./cmd/specrun
```

## Write Your First Spec

Create a file called `transfer.spec`. In v4, the file IS the spec — no wrapper block, declarations appear directly at the top level:

```
http {
  base_url: env(APP_URL, "http://localhost:8080")
}

model Account {
  id: string
  balance: int
}

model TransferResult {
  from: Account
  to: Account
  error: string?
}

scope transfer {
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
        output.from.balance + output.to.balance == from.balance + to.balance
    }

    scenario overdraft {
      when { amount > from.balance }
      then { output.error == "insufficient_funds" }
    }
  }
}
```

This spec declares:

- An **adapter config** (`http`) pointing at the system under test
- Two **models** describing the data shapes
- A **scope** grouping related contracts
- A **contract** with input fields declared in the signature parens, an action block that invokes the HTTP endpoint, an invariant that must hold for every generated input, and a scenario asserting on a specific input class

## Run Verification

Start your server, then run:

```bash
APP_URL=http://localhost:8080 specrun verify transfer.spec
```

Sample output on success:

```
verifying transfer.spec (seed=42, iterations=100)

  scope transfer:
    ✓ invariant conservation (100 inputs)
    ✓ scenario overdraft (100 inputs)

Scenarios:  1/1 passed
Invariants: 1/1 passed
```

## Interpreting Results

**Pass:** Each scenario and invariant shows a checkmark. Generative checks (invariants, `when` scenarios) show how many inputs were tested.

**Fail:** A failing check shows the counterexample — the specific input that caused the failure, shrunk to a minimal reproducer:

```
  scope transfer:
    ✗ invariant conservation (failed after 23 inputs)
      counterexample (shrunk):
        from: { id: "", balance: 1 }
        to: { id: "", balance: 0 }
        amount: 1
      expected: output.from.balance + output.to.balance == from.balance + to.balance
      got: 0 + 0 == 1 + 0 (false)
```

The counterexample is binary-search shrunk to the smallest failing input, making it easier to diagnose the root cause.

## Splitting a Spec Across Files

For larger systems, split models and scopes across files. Use `include` for **shared library fragments** — models, actions, adapter config:

```
specs/
├── shared/
│   └── models.spec    # model Account, model TransferResult
├── transfer.spec      # include "shared/models.spec", scope transfer
└── audit.spec         # include "shared/models.spec", scope audit
```

Run both specs independently with a glob — each is its own verification unit:

```bash
specrun verify specs/*.spec
```

> **Important:** `include` is for shared fragments, not for composing runnable specs into a super-spec. Don't create a root `all.spec` that includes `transfer.spec` and `audit.spec` — if both declare the same model names, that is an error. Use the glob pattern above. See [Includes vs. glob execution](language-reference.md#includes-vs-glob-execution).

## Other Commands

```bash
specrun parse transfer.spec                      # parse spec, output AST as JSON
specrun generate transfer.spec --scope transfer  # generate one random input
specrun verify transfer.spec --json              # verify with machine-readable output
specrun migrate --to v4 old.spec                 # convert a v3 spec to v4 syntax
```

## Self-Contained Specs with Docker

Instead of manually starting servers before verification, declare services directly in the spec. `specrun verify` will build, start, health-check, and tear down containers automatically:

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

# ... models, scopes, etc.
```

Run verification — no manual server startup needed:

```bash
specrun verify transfer.spec
```

The `service(app)` expression resolves to the running container's URL at runtime. Docker must be available on the host.

## Self-Verification

Speclang verifies itself using its own spec language. Run:

```bash
go build -o specrun ./cmd/specrun
SPECRUN_BIN=./specrun ./specrun verify specs/speclang.spec
```

See [Self-Verification](self-verification.md) for details.

## Next Steps

- [Language Reference](language-reference.md) — complete syntax for models, types, expressions, constraints, and scenarios
- [v4 Syntax](v4-syntax.md) — layer-by-layer structural reference
- [Writing Specs That Prove Things](writing-specs-that-prove-things.md) — what makes a spec meaningful vs. a tautology
- [HTTP Adapter](adapters/http.md) — testing REST APIs
- [Process Adapter](adapters/process.md) — testing CLI tools and subprocesses
- [Playwright Adapter](adapters/playwright.md) — testing browser UIs
- [OpenAPI Import](imports/openapi.md) — importing from OpenAPI schemas
- [Protobuf Import](imports/protobuf.md) — importing from protobuf files
