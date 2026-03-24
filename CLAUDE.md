# SpecLang

A specification language for AI-driven software development that serves as both a human-readable roadmap and a generative verification runtime against black-box systems.

## Problem Statement

LLMs tasked with writing code to satisfy a specification will optimize against visible test suites — hardcoding outputs, writing degenerate implementations, gaming the letter of the spec while violating its spirit. We need a spec language where:

1. The LLM reads the spec to understand **what** to build
2. A runtime reads the same spec to **generate unbounded, unpredictable test cases** at verification time
3. The test surface is unknowable to the implementer because inputs are generated from declared constraints, not enumerated

## Core Language Design

See [docs/language-reference.md](docs/language-reference.md) for the complete syntax reference.

### Settled Decisions

- **Calling convention**: `verb(args)` universally — both built-in primitives and user-defined actions
- **Plugin architecture**: Plugins are either **built-in** (http, process, playwright — compiled into specrun) or **external** (adapter binary on PATH communicating via JSON stdin/stdout)
- **Scope-level plugin declaration**: `use <plugin>` appears inside each `scope` block, not at spec level
- **Namespaced calls**: Plugin methods are called as `plugin.method()` (e.g., `playwright.fill(locator, value)`)
- **Assertion syntax in `then` blocks**: `locator@plugin.property: expected` (e.g., `error_msg@playwright.visible: true`)
- **Error pseudo-field**: `error` in `then` blocks asserts against action errors when `error` is NOT a contract output field
- **Three scenario types** (ascending verification strength):
  - `scenario` with `given` — concrete values, smoke test / documentation
  - `scenario` with `when` — predicate over input class, runtime generates across matching space
  - `invariant` — universal law, must hold for ALL valid inputs
- **Runtime is a Go binary** that parses specs, generates inputs, and delegates execution to adapter binaries over IPC
- **Scope-based grouping**: Contracts, invariants, and scenarios live inside named `scope` blocks with opaque `config` blocks
- **Counterexample shrinking**: Binary-search shrinking (ints toward 0, strings toward shorter prefixes, nested models recursively)

### Language Features

- **Types**: `int`, `float`, `string`, `bytes`, `bool`, `any`, `[]T` (array), `map[K,V]`, `enum("a","b",...)`, model references, `T?` (optional)
- **Expressions**: all arithmetic/comparison/logical operators, chained comparisons (`0 < x <= y`), division (`/`), modulo (`%`)
- **Built-in functions**: `len()`, `contains()`, `exists()`, `has_key()`, `all(arr, x => pred)`, `any(arr, x => pred)`
- **Conditional expressions**: `if cond then a else b`
- **Include/Import**: `include "path"`, `import openapi("path")`, `import proto("path")`
- **Dot-path array indexing**: `items.0.name` for array element access
- **Compile-time validation**: type checking, model resolution, given completeness, then field validation

### Anti-Gaming Properties

- Input generation uses randomized seeds, varying distributions, and boundary-weighted strategies
- Metamorphic test composition varies across runs
- `when`-predicate scenarios generate from the full valid input space, not enumerated examples
- The implementing agent sees property signatures but never the generator strategy

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

## Prototype Scope

Phase 1: **HTTP plugin + runtime core**
Phase 2: **Playwright plugin + built-in adapter**
Phase 3: Go unit plugin + adapter
Phase 4: Metamorphic relation support

## Project Structure

```
speclang/
├── CLAUDE.md
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
│   └── specrun/          # CLI entrypoint
│       └── main.go
├── docs/
│   ├── getting-started.md
│   ├── language-reference.md
│   ├── self-verification.md
│   ├── adapters/
│   │   ├── http.md
│   │   ├── process.md
│   │   └── playwright.md
│   └── imports/
│       ├── openapi.md
│       └── protobuf.md
├── pkg/
│   ├── parser/           # spec file → AST
│   │   ├── lexer.go
│   │   ├── parser.go
│   │   ├── ast.go
│   │   ├── include.go    # include resolution + duplicate validation
│   │   └── import.go     # import directive + ImportResolver interface
│   ├── generator/        # AST → test inputs
│   │   ├── generator.go
│   │   └── shrink.go     # counterexample shrinking
│   ├── runner/           # orchestrates generate → execute → check
│   │   └── runner.go
│   ├── adapter/          # adapter protocol + built-in adapters
│   │   ├── protocol.go   # JSON IPC types
│   │   ├── http.go       # built-in HTTP adapter
│   │   ├── process.go    # built-in process adapter (subprocess execution)
│   │   └── playwright.go # built-in Playwright adapter (compiled into specrun)
│   ├── openapi/          # OpenAPI import resolver
│   │   ├── openapi.go    # Resolver implementing ImportResolver
│   │   ├── document.go   # OpenAPI doc loading via kin-openapi
│   │   ├── models.go     # schema → Model conversion
│   │   └── scopes.go     # path → Scope conversion
│   ├── proto/            # Protobuf import resolver
│   │   ├── proto.go      # Resolver implementing ImportResolver
│   │   ├── models.go     # message → Model conversion
│   │   └── scopes.go     # service/RPC → Scope conversion
│   └── plugin/           # plugin spec loading
│       └── plugin.go
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
│   │   ├── petstore.yaml # sample OpenAPI 3.0 spec
│   │   └── petstore.spec # spec importing from OpenAPI schema
│   ├── proto/            # Protobuf import example
│   │   ├── user.proto    # sample protobuf service definition
│   │   └── user.spec     # spec importing from protobuf schema
│   └── server/           # trivial Go HTTP server to test against
│       └── main.go
├── specs/                # self-verification specs (speclang verifying itself)
│   ├── speclang.spec     # root: use process, includes parse/generate/verify
│   ├── parse.spec        # parse_valid + parse_invalid scopes
│   ├── import.spec       # import behavioral verification (OpenAPI + Protobuf)
│   ├── generate.spec     # generator constraint satisfaction
│   ├── verify.spec       # verify_pass scope
│   ├── verify_fail.spec  # verify_fail scope (broken implementation detection)
│   └── shrinking.spec    # shrinking scope (counterexample minimality)
└── testdata/
    ├── include/          # multi-file include test fixtures
    │   ├── basic/        # root includes models + scopes
    │   ├── nested/       # A → B → C transitive includes
    │   ├── circular/     # A ↔ B circular include (error case)
    │   ├── duplicate/    # duplicate model names (error case)
    │   └── duplicate_scope/  # duplicate scope names (error case)
    ├── playwright/       # Playwright adapter test fixtures
    └── self/             # self-verification test fixtures
        ├── minimal.spec
        ├── invalid_unterminated.spec
        ├── broken_transfer.spec
        ├── broken_transfer_invariant_only.spec
        └── broken_server/main.go
```

## Tech Stack

- Go (latest stable)
- No external dependencies for core runtime
- `net/http` for built-in HTTP adapter
- `os/exec` for built-in process adapter
- `math/rand/v2` for input generation

## Commands

```bash
go build ./cmd/specrun                                          # build the CLI
go test ./...                                                   # run all tests
./specrun verify examples/transfer.spec                         # run verification
./specrun parse examples/transfer.spec                          # parse spec, output AST as JSON
./specrun generate examples/transfer.spec --scope transfer      # generate one input as JSON
./specrun verify examples/transfer.spec --json                  # verify with JSON output
./specrun verify specs/speclang.spec                            # self-verification
./specrun install playwright                                    # install playwright browsers (chromium)
```

## Self-Verification

Speclang verifies itself with its own specs via `specs/speclang.spec`. See [docs/self-verification.md](docs/self-verification.md) for details.

The self-verification spec uses the process adapter to invoke `specrun` subcommands and verify their behavior:

- **parse_valid** — parser accepts valid specs and produces expected AST structure
- **parse_invalid** — parser rejects malformed specs with exit code 1
- **parse_validation** — parser validates types and produces type errors
- **import_openapi_*** — OpenAPI imports produce correct models, constraints, and refs
- **import_proto_*** — protobuf imports produce correct models and scopes
- **generate** — generator produces constraint-satisfying outputs across seeds
- **generate_types** — generator handles all types (float, bytes, arrays, maps, optionals)
- **verify_pass** — `specrun verify` passes correct implementations
- **verify_fail** — `specrun verify` detects incorrect implementations
- **shrinking** — counterexample shrinking produces minimal values

Run self-verification:
```bash
SPECRUN_BIN=./specrun ./specrun verify specs/speclang.spec
```

## Claude Code Plugin

This repo is a Claude Code plugin. It ships skills (`speclang:author`, `speclang:verify`), slash commands (`/spec`, `/verify-spec`), and a session-start hook.

**ALWAYS keep skills up-to-date.** When the spec language syntax, CLI commands, output format, or verification behavior changes, update the corresponding skill files and syntax reference:

- `skills/author/SKILL.md` — authoring guidance and checklist
- `skills/author/references/api_reference.md` — language syntax reference
- `skills/verify/SKILL.md` — verification process and output interpretation
- `hooks/session-start.sh` — session-start detection logic

If a change to the runtime would make a skill give incorrect guidance, the skill update is part of the same change, not a follow-up.
