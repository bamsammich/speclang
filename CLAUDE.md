# SpecLang

A specification language for AI-driven software development that serves as both a human-readable roadmap and a generative verification runtime against black-box systems.

## Problem Statement

LLMs tasked with writing code to satisfy a specification will optimize against visible test suites — hardcoding outputs, writing degenerate implementations, gaming the letter of the spec while violating its spirit. We need a spec language where:

1. The LLM reads the spec to understand **what** to build
2. A runtime reads the same spec to **generate unbounded, unpredictable test cases** at verification time
3. The test surface is unknowable to the implementer because inputs are generated from declared constraints, not enumerated

## Core Language Design

See [docs/language-reference.md](docs/language-reference.md) for the complete syntax reference and [docs/v4-syntax.md](docs/v4-syntax.md) for the layer-by-layer structural reference.

### Settled Decisions

- **v4 is the current syntax version** — see [docs/plans/2026-04-09-v4-language-design.md](docs/plans/2026-04-09-v4-language-design.md) for the full design and [docs/migration-v4.md](docs/migration-v4.md) for migration from v3
- **Filename is spec identity** — no `spec Name { }` wrapper; top-level declarations live directly in the file
- **Contract is the unit of verification** — `contract Name -> ReturnModel { fields, action { body }, invariants, scenarios }`
- **Optional inheritance** — `contract Name: InputModel -> ReturnModel { constrain { ... } ... }` to layer constraints onto an inherited input model
- **Field resolution rules**: bare names → contract fields (input); `output.<field>` → return type; `config.<key>` → spec config
- **Logical operators are word-form**: `and`, `or`, `not`, `in`, `implies` (lowest precedence)
- **Calling convention**: `adapter.method(args)` for all adapter interactions — both actions and assertions
- **Plugin architecture**: Plugins are either **built-in** (http, process, playwright — compiled into specrun) or **external** (adapter binary on PATH communicating via JSON stdin/stdout)
- **Adapter naming**: Adapters are named inline per call (e.g., `http.post(...)`, `playwright.fill(...)`) — no `use` directive
- **Adapter config**: Namespaced config blocks at spec level (`http { base_url: ... }`, `playwright { headless: true }`)
- **Assertion syntax in `then` blocks**: `expr operator value` (e.g., `output.balance >= 0`, `output.error == null`). Operators: `==`, `!=`, `>`, `>=`, `<`, `<=`. No `:` for equality.
- **Error pseudo-field**: `error` in assertions checks action errors when `error` is NOT a contract output field
- **Variables**: `let name = expr` for immutable bindings — captures action results, scoped to block
- **Reusable actions**: `action name(param: type, ...) { body }` at top level, with `let`, `return`, typed params — invoked from `before`/`after`/`given` blocks
- **Three verification members** (ascending strength):
  - `scenario` with `given` — concrete values, smoke test / documentation
  - `scenario` with `when` — predicate over input class, runtime generates across matching space
  - `invariant` — universal law, must hold for ALL valid inputs
- **Runtime is a Go binary** that parses specs, generates inputs, and delegates execution to adapters
- **Optional `scope` grouping**: When two or more contracts share before/after lifecycle, group them in a named `scope { before { } after { } contract... contract... }`. Contracts can also live at the top level standalone.
- **Counterexample shrinking**: Binary-search shrinking (ints toward 0, strings toward shorter prefixes, nested models recursively)
- **Services**: `services` block at spec level declares Docker containers as test infrastructure. `service(name)` expression resolves to the running container's URL. Compose support via `compose: "path"` for multi-service setups
- **Composing specs**: `include` is for shared, non-runnable fragments (models, actions, adapter config). `specrun verify <glob>` is for running multiple independent runnable specs. **Don't compose runnable specs via `include`** — see [docs/language-reference.md](docs/language-reference.md) section "Includes vs. glob execution".

### Language Features

- **Types**: `int`, `float`, `string`, `bytes`, `bool`, `any`, `[]T` (array), `map[K,V]`, inline `enum("a","b",...)`, named enum refs (`enum Role { admin, user, viewer }` referenced as `Role.admin`), model references, `T?` (optional)
- **Expressions**: arithmetic (`+`, `-`, `*`, `/`, `%`), comparison (`==`, `!=`, `<`, `<=`, `>`, `>=`), chained comparisons (`0 < x <= y`), logical (`and`, `or`, `not`), `in` (with array-literal RHS: `status in ["pending", "active"]`), `implies` (lowest precedence)
- **Numeric literals**: underscore separators allowed (`1_000_000`, `3.14_159`)
- **Built-in functions**: `len()`, `contains()`, `exists()`, `has_key()`, `all(arr, x => pred)`, `any(arr, x => pred)`
- **Conditional expressions**: `if cond then a else b`
- **State-dependent fields**: `tracking: string when status == "shipped"` — field is conditionally present based on sibling field values
- **Config block**: `config { max_transfer: 1_000_000 }` at top level; referenced as `config.max_transfer` in expressions
- **Variables**: `let name = expr` for immutable bindings in before/after/given/action bodies
- **After block**: `after { steps... }` at scope level — runs after every scenario/invariant iteration, even on failure; errors are logged but never affect test results
- **Single-quoted strings**: `'[data-testid="email"]'` for CSS selectors containing double quotes
- **Include/Import**: `include "path"` (top-level only), `import openapi("path")`, `import proto("path")`
- **Dot-path array indexing**: `items.0.name` for array element access
- **Compile-time validation**: type checking, model resolution, named enum variant validation, `output.` prefix enforcement, circular `when` detection, assertion operator validation

### Anti-Gaming Properties

- Input generation uses randomized seeds, varying distributions, and boundary-weighted strategies
- Metamorphic test composition varies across runs
- `when`-predicate scenarios generate from the full valid input space, not enumerated examples
- The implementing agent sees property signatures but never the generator strategy
- See [docs/writing-specs-that-prove-things.md](docs/writing-specs-that-prove-things.md) for the SLO-thinking framing on writing specs that actually prove something rather than passing tautologically

## Runtime Architecture

```
spec files (.spec)              implementation (black box)
       │                              │
       ▼                              │
  ┌─────────┐                         │
  │ Parser   │  (Go)                   │
  └────┬─────┘                         │
       ▼                              │
  ┌──────────────┐                     │
  │ Generator    │  (Go, PBT engine)   │
  └──────┬───────┘                     │
         ▼                            ▼
  ┌─────────────────────────────────────┐
  │    Adapter (subprocess, JSON IPC)   │
  └──────────────┬──────────────────────┘
                 ▼
  ┌──────────────────┐
  │ Shrinker         │  (binary search on counterexamples)
  └──────┬───────────┘
         ▼
         Verdict + Minimal Counterexamples
```

## Adapters

| Plugin | Use case | Docs |
|--------|----------|------|
| `http` | REST APIs | [docs/adapters/http.md](docs/adapters/http.md) |
| `process` | CLI tools / subprocesses | [docs/adapters/process.md](docs/adapters/process.md) |
| `playwright` | Browser UIs | [docs/adapters/playwright.md](docs/adapters/playwright.md) |

## Importers

| Source | Docs |
|--------|------|
| OpenAPI 3.0 | [docs/imports/openapi.md](docs/imports/openapi.md) |
| Protobuf 3 | [docs/imports/protobuf.md](docs/imports/protobuf.md) |

## Project Structure

```
speclang/
├── CLAUDE.md
├── README.md
├── go.mod
├── .claude-plugin/
│   └── plugin.json           # Claude Code plugin manifest
├── skills/
│   ├── author/               # speclang:author — spec authoring from natural language
│   │   ├── SKILL.md
│   │   └── references/
│   │       └── api_reference.md
│   └── verify/               # speclang:verify — verification gate before merge
│       └── SKILL.md
├── commands/
│   ├── spec.md               # /spec slash command
│   └── verify-spec.md        # /verify-spec slash command
├── hooks/
│   ├── hooks.json            # session-start hook registration
│   └── session-start.sh      # injects speclang awareness on session start
├── cmd/
│   └── specrun/              # CLI entrypoint
│       └── main.go           # verify, parse, generate, migrate, install
├── docs/
│   ├── getting-started.md
│   ├── language-reference.md             # full v4 syntax reference
│   ├── v4-syntax.md                      # layer-by-layer structural reference
│   ├── migration-v4.md                   # v3 → v4 migration guide
│   ├── writing-specs-that-prove-things.md # SLO-thinking guide / anti-patterns
│   ├── self-verification.md
│   ├── adapters/
│   │   ├── http.md
│   │   ├── process.md
│   │   └── playwright.md
│   └── imports/
│       ├── openapi.md
│       └── protobuf.md
├── pkg/
│   ├── spec/             # Public API — types, interfaces, registry
│   │   ├── ast.go        # Spec, Scope, Model, Field, Contract, NamedEnum, Expr types
│   │   ├── adapter.go    # Adapter interface, Request, Response
│   │   ├── registry.go   # Registry, PluginDef, ActionDef, AssertionDef
│   │   ├── result.go     # Result, ScopeResult, CheckResult, Failure
│   │   └── import.go     # ImportResolver, ImportRegistry
│   └── specrun/          # Public API — Verify, Generate, Parse, DefaultRegistry
│       ├── specrun.go    # Parse, ParseFile, Validate, Verify, Generate
│       └── registry.go   # DefaultRegistry (http, process, playwright)
├── internal/
│   ├── parser/           # v4 spec file → AST (lexer, parser, includes, imports)
│   ├── v3parser/         # preserved v3 parser (used by migration tool)
│   ├── generator/        # AST → test inputs + counterexample shrinking
│   ├── runner/           # orchestrates generate → execute → check
│   ├── validator/        # compile-time type checking and semantic validation
│   ├── adapter/          # built-in adapters (http, process, playwright)
│   ├── infra/            # Docker/compose service lifecycle management
│   ├── openapi/          # OpenAPI import resolver
│   ├── proto/            # Protobuf import resolver
│   ├── migrate/          # v2 → v3 → v4 migration transforms
│   └── plugin/           # plugin spec file loading
├── plugins/
│   ├── http.plugin       # HTTP plugin definition
│   ├── process.plugin    # process plugin definition (subprocess execution)
│   └── playwright.plugin # Playwright plugin definition
├── examples/
│   ├── transfer.spec     # root spec (includes models + scopes)
│   ├── models/
│   │   └── account.spec  # model Account
│   ├── scopes/
│   │   └── transfer.spec # scope transfer (contract, invariants, scenarios)
│   ├── openapi/          # OpenAPI import example
│   └── proto/            # Protobuf import example
├── specs/                # self-verification specs (speclang verifying itself)
│   ├── speclang.spec     # root: aggregates all self-verification scopes
│   ├── parse.spec        # parser behavior (valid, invalid, validation)
│   ├── import.spec       # OpenAPI + Protobuf import behavior
│   ├── generate.spec     # generator constraint satisfaction
│   ├── generate_types.spec # generator coverage across all types
│   ├── verify.spec       # verify_pass scope
│   ├── verify_fail.spec  # verify_fail scope (broken implementation detection)
│   ├── shrinking.spec    # counterexample shrinking minimality
│   ├── services.spec     # service lifecycle, service ref parsing
│   ├── glob.spec         # glob CLI matching, no-contract skip behavior
│   ├── v4_features.spec  # v4-specific features (enum, in, implies, config, etc.)
│   ├── adapters.spec     # adapter behavior
│   ├── cli_flags.spec    # CLI flag handling
│   ├── enum.spec         # enum semantics
│   ├── error_assertions.spec # error pseudo-field
│   ├── exists.spec       # exists() built-in
│   ├── expressions.spec  # expression evaluation
│   └── types.spec        # type system
└── testdata/
    ├── include/          # multi-file include test fixtures
    ├── playwright/       # Playwright adapter test fixtures
    ├── migrate_v4/       # v3 → v4 migration fixtures
    └── self/             # self-verification fixtures
```

## Tech Stack

- Go (latest stable)
- No external dependencies for core runtime
- `net/http` for built-in HTTP adapter
- `os/exec` for built-in process adapter
- `math/rand/v2` for input generation
- `github.com/docker/docker` for container lifecycle management (services feature)

## Commands

```bash
go build ./cmd/specrun                                              # build the CLI
go test ./...                                                       # run all tests
./specrun verify examples/transfer.spec                             # verify a single spec
./specrun verify specs/*.spec                                       # verify all matched specs (each independent)
./specrun verify specs/**/*.spec                                    # recursive glob
./specrun parse examples/transfer.spec                              # parse spec, output AST as JSON
./specrun generate examples/transfer.spec --scope transfer          # generate one input as JSON
./specrun verify examples/transfer.spec --json                      # verify with JSON-lines output
./specrun verify specs/speclang.spec                                # self-verification
./specrun verify spec.spec --keep-services                          # keep containers running after verify
./specrun migrate --to v4 path/to/v3-spec.spec                      # migrate v3 → v4 (default target)
./specrun install playwright                                        # install playwright browsers (chromium)
```

## Self-Verification

Speclang verifies itself with its own specs via `specs/speclang.spec`. See [docs/self-verification.md](docs/self-verification.md) for details.

The self-verification spec uses the process adapter to invoke `specrun` subcommands and verify their behavior. The root spec (`specs/speclang.spec`) declares services for `transfer_server`, `broken_server`, and `http_test_server` in its `services` block — these containers are managed automatically during verification when Docker is available.

Run self-verification:
```bash
SPECRUN_BIN=./specrun ./specrun verify specs/speclang.spec
```

Current coverage: **86 scenarios + 21 invariants** across parser, validator, generator, runner, importers, services, glob CLI, and v4 language features.

## Claude Code Plugin

This repo is a Claude Code plugin. It ships skills (`speclang:author`, `speclang:verify`), slash commands (`/spec`, `/verify-spec`), and a session-start hook.

**ALWAYS keep skills up-to-date.** When the spec language syntax, CLI commands, output format, or verification behavior changes, update the corresponding skill files and syntax reference:

- `skills/author/SKILL.md` — authoring guidance and checklist
- `skills/author/references/api_reference.md` — language syntax reference
- `skills/verify/SKILL.md` — verification process and output interpretation
- `hooks/session-start.sh` — session-start detection logic

If a change to the runtime would make a skill give incorrect guidance, the skill update is part of the same change, not a follow-up.
