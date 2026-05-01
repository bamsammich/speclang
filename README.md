# SpecLang

**A specification language where the same file is the LLM's roadmap and the verifier's test-case generator — so the implementer can't see the test surface it's being graded on.**

## The Problem

Give an LLM a unit test suite and it will optimize against the tests. It hardcodes outputs. It writes degenerate implementations. It games the letter of the spec while violating its spirit. The tests pass. The code is wrong.

The root cause is visibility: when the test inputs are enumerated in source, the implementer can read them and tune to them. What's needed is a language where the implementing agent sees the **constraints** — "balance must be non-negative", "an insufficient-funds error must not mutate state" — but never the **inputs** used to check them. The test surface has to be unknowable at implementation time.

SpecLang is that language. You write one `.spec` file. The LLM reads it to understand what to build. At verification time, the runtime reads the same file, generates thousands of unpredictable inputs from the declared constraints, executes them against your system through an adapter (HTTP, process, browser), and shrinks any counterexample to its minimal form. You can't hardcode against a test case you've never seen.

## How it works

```
  .spec file                  system under test (black box)
      │                                  │
      ▼                                  │
  ┌─────────┐                            │
  │ Parser  │                            │
  └────┬────┘                            │
       ▼                                 │
  ┌──────────────┐                       │
  │ Generator    │  property-based,      │
  │              │  seeded, boundary-    │
  │              │  weighted             │
  └──────┬───────┘                       │
         ▼                               ▼
  ┌──────────────────────────────────────────┐
  │  Adapter  (http | process | playwright)  │
  └──────────────────┬───────────────────────┘
                     ▼
              ┌─────────────┐
              │ Shrinker    │  binary-search
              └──────┬──────┘  minimal counterexample
                     ▼
              Verdict + failures
```

Three levels of verification, ascending strength:

- **`scenario` with `given`** — a concrete smoke test. Fixed inputs. Runs once. Documents an interesting point.
- **`scenario` with `when`** — a predicate over the input space. The generator produces many matching inputs; the `then` block must hold for all of them.
- **`invariant`** — a universal law. Must hold for **every** input the generator produces.

## Your first spec

```
http {
  base_url: "http://localhost:8080"
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
      output.from.balance + output.to.balance == from.balance + to.balance
    }

    invariant no_mutation_on_error {
      output.error != null implies
        output.from.balance == from.balance
        and output.to.balance == to.balance
    }

    scenario overdraft {
      when { amount > from.balance }
      then { output.error == "insufficient_funds" }
    }
  }
}
```

Run it:

```bash
specrun verify examples/transfer.spec
```

Expected output:

```
verifying examples/transfer.spec (seed=42, iterations=100)

  scope transfer:
    ✓ invariant conservation (100 inputs)
    ✓ invariant no_mutation_on_error (100 inputs)
    ✓ scenario overdraft (100 inputs)

Scenarios:  1/1 passed
Invariants: 2/2 passed
```

When a check fails, you get a shrunk counterexample — the minimal input that still breaks the invariant — not a random 4KB JSON blob the generator happened to produce.

## Language at a glance

v4 is the current syntax. The file itself is the spec — no wrapper block. The filename is the spec's identity.

- **No spec wrapper.** Top-level declarations appear directly in the file.
- **`contract Name(field: type, ...) -> ReturnModel { action { }, invariants, scenarios }`** — the primary unit of verification. Input fields declared in the signature parens, an `action` block that invokes the system, invariants the response must satisfy, scenarios for generative or concrete probes.
- **`model` / `enum`** — data structures and named variant sets. Named enums are referenced as `Role.admin`, validated at parse time against their declaration.
- **`scope`** — optional grouping for contracts that share `before` / `after` lifecycle hooks.
- **`config { max_transfer: 1_000_000 }`** — spec-level constants, referenced as `config.max_transfer` in expressions.
- **State-dependent fields** — `tracking: string when status == "shipped"` declares a field that only exists in certain input states.
- **Word operators** — `and`, `or`, `not`, `in`, `implies`. `a implies b` has the lowest precedence and is the natural way to write conditional invariants.
- **`include "path"`** — pulls in declarations from another file (include-once by resolved absolute path).
- **Underscore numerics** — `1_000_000`, `3.14_159`.

> **Composing specs.** Use `include` for shared fragments (models, actions, adapter config). Use `specrun verify <glob>` to run multiple independent specs. Don't try to assemble runnable specs into a super-spec via `include` — each file in the include graph must have unique declarations, so two specs that both need `model Account` have to factor it into a shared include. See [Includes vs. glob execution](docs/language-reference.md#includes-vs-glob-execution) for the full pattern.

Full syntax reference: [docs/language-reference.md](docs/language-reference.md). Complete v4 design rationale: [docs/plans/2026-04-09-v4-language-design.md](docs/plans/2026-04-09-v4-language-design.md).

## Adapters

SpecLang executes actions against your system through adapters. The runtime ships with three built-in:

- **`http`** — REST and JSON HTTP APIs. Base URL, headers, methods, body serialization. Covers most server work. [docs/adapters/http.md](docs/adapters/http.md)
- **`process`** — CLIs and subprocesses. Invokes a binary, captures stdout/stderr/exit code as structured output. Used by speclang's self-verification to drive `specrun` itself. [docs/adapters/process.md](docs/adapters/process.md)
- **`playwright`** — browser UIs. Fill, click, visibility assertions against a headless Chromium. [docs/adapters/playwright.md](docs/adapters/playwright.md)

External adapters are single binaries on `PATH` communicating via JSON on stdin/stdout — so you can wrap anything the built-ins don't cover without patching the runtime.

## Importers

Skip hand-transcribing schemas. Point at an existing contract, let the parser build models and constraints from it:

- **OpenAPI** — `import openapi("petstore.yaml")` produces models for each schema and scopes wired to each operation, with types and constraints derived from the OpenAPI. [docs/imports/openapi.md](docs/imports/openapi.md)
- **Protobuf** — `import proto("user.proto")` produces models for each message and scopes for each RPC. Useful for gRPC and any proto-defined wire format. [docs/imports/protobuf.md](docs/imports/protobuf.md)

Imports give you the shape of the interface for free. You still write the invariants — the part that actually encodes the system's promise.

## Writing specs that prove something

This is the hardest part of using SpecLang and the part that matters most.

A unit test that asserts `1 + 1 == 2` is obviously a tautology. A spec that asserts `output != null` on a server that always returns a response body is **also** a tautology — but it runs 100 generated inputs, reports 100 passing invariants, and looks substantial. It proves nothing.

Writing a spec is like writing an SLO or an SLA. It's easy to commit to uptime "greater than 0%". It's hard to commit to uptime you'd actually stake the product on. Specs have the same shape. The question isn't "does my spec pass" — it's **"does the fact that my spec passes mean anything"**.

A few habits that distinguish specs that prove something from specs that don't:

- Every invariant references `output.<something>`. If nothing in an invariant mentions the response, you're not checking the system — you're restating a constraint you already gave the generator.
- Every invariant would fail on a wrong implementation. Sketch a buggy version in your head. Would the spec catch it? If not, you're missing a law.
- Error paths are first-class. Every promise about the happy path has a dual on the error path. "Conservation holds" is half a law; "nothing mutates on error" is the other half.
- `scenario when` narrows to a class; the `then` must say something **specific to that class**, not something that would be true everywhere.

The full treatment — anti-pattern gallery, positive patterns, and a pre-commit checklist — lives in [docs/writing-specs-that-prove-things.md](docs/writing-specs-that-prove-things.md). Read it before you ship your first spec.

## Install

From source:

```bash
git clone https://github.com/bamsammich/speclang.git
cd speclang
go build -o specrun ./cmd/specrun
```

Via `go install`:

```bash
go install github.com/bamsammich/speclang/v4/cmd/specrun@latest
```

## CLI

```bash
specrun verify <file-or-glob>          # verify one spec or many
specrun verify 'specs/**/*.spec'       # recursive glob; each match is an independent spec unit
specrun verify spec.spec --json        # machine-readable output
specrun verify spec.spec --seed 7 --iterations 1000
specrun verify spec.spec --keep-services   # leave Docker containers running after verify

specrun parse <file>                   # parse spec, output AST as JSON (useful for debugging)
specrun generate <file> --scope <name> --seed <n>   # generate one input from a scope

specrun migrate --to v4 <file>         # convert v3 spec → v4 (stdout; add -w to write in place)
specrun install playwright             # download chromium for the playwright adapter
```

Glob execution is the recommended workflow for anything beyond a single file: each matched file is verified as its own spec unit, so failures in one don't obscure others.

## Project status

Experimental, but self-verified. The speclang runtime is verified by specs written in the speclang language, executed through the runtime itself — coverage spans parse, generate, verify, import, services, shrinking, CLI flags, adapters, and v4-specific features. See [docs/self-verification.md](docs/self-verification.md) for the current scenario and invariant counts.

v4 is the current syntax version. For v3 specs, `specrun migrate --to v4 <file>` handles the mechanical conversion (word operators, spec wrapper removal, contract restructuring, `output.` prefix insertion).

Prototype scope: HTTP plugin + runtime core are production-shaped; Playwright is working; process adapter drives the self-verification suite; metamorphic relations are next.

## Versioning and stability

speclang v4.x.y follows Go module semver. Breaking changes require a new major version at a new module path (`github.com/bamsammich/speclang/v5`). Patch and minor versions within v4 are backward-compatible at the spec-file level: a spec that verifies on v4.0.0 will verify on every subsequent v4.x.y. CLI flags and library APIs in `pkg/` follow the same rule.

v3 users can migrate with `specrun migrate --to v4` — see [docs/migration-v4.md](docs/migration-v4.md).

Until v4.0.0 is tagged, the v4 branch should be treated as a release candidate.

## Claude Code plugin

This repo ships as a [Claude Code](https://claude.com/claude-code) plugin. Two skills and two slash commands:

- **`speclang:author`** — converts natural-language requirements into a `.spec` file. Trigger: `/spec`.
- **`speclang:verify`** — runs `specrun verify` as a merge gate. Trigger: `/verify-spec`.

Install:

```
/plugin marketplace add bamsammich/speclang-marketplace
/plugin install speclang@speclang-marketplace
```

A session-start hook detects the presence of `.spec` files in the repo and loads speclang awareness automatically.

## License

MIT
