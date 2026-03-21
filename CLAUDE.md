# SpecLang

A specification language for AI-driven software development that serves as both a human-readable roadmap and a generative verification runtime against black-box systems.

## Problem Statement

LLMs tasked with writing code to satisfy a specification will optimize against visible test suites — hardcoding outputs, writing degenerate implementations, gaming the letter of the spec while violating its spirit. We need a spec language where:

1. The LLM reads the spec to understand **what** to build
2. A runtime reads the same spec to **generate unbounded, unpredictable test cases** at verification time
3. The test surface is unknowable to the implementer because inputs are generated from declared constraints, not enumerated

## Core Language Design

### Settled Decisions

- **Calling convention**: `verb(args)` universally — both built-in primitives and user-defined actions
- **Plugin architecture**: The core language has zero built-in primitives. All interaction verbs and assertion types come from plugins (`use http`, `use playwright`, `use go`)
- **Namespaced calls**: Plugin methods are called as `plugin.method()` (e.g., `playwright.fill(locator, value)`)
- **Assertion syntax in `then` blocks**: `locator@plugin.property: expected` (e.g., `error_msg@playwright.visible: true`)
- **Three scenario types** (ascending verification strength):
  - `scenario` with `given` — concrete values, smoke test / documentation
  - `scenario` with `when` — predicate over input class, runtime generates across matching space
  - `invariant` — universal law, must hold for ALL valid inputs
- **Plugin = spec + adapter binary**: Plugin spec declares typed actions/assertions. Adapter binary implements them over JSON stdin/stdout protocol.
- **Runtime is a Go binary** that parses specs, generates inputs, and delegates execution to adapter binaries over IPC.
- **Scope-based grouping**: Contracts, invariants, and scenarios live inside named `scope` blocks. Each scope has an opaque `config` block for plugin-specific settings (e.g., HTTP path/method). The parser is agnostic to config semantics.
- **Counterexample shrinking**: When a failure is found, the runtime performs binary-search shrinking (ints toward 0, strings toward shorter prefixes, nested models recursively) to produce minimal counterexamples.

### Spec File Structure

```
use <plugin>
include "<path>"                     # top-level include

spec <Name> {

  target {
    base_url: env(APP_URL)          # optional, plugin-dependent config
  }

  include "<path>"                   # spec-body include

  locators {                         # UI-mode only
    <name>: [<css-selector>]
  }

  model <Name> {
    <field>: <type>
    <field>: <type> { <constraint> }
  }

  action <name>(<args>) {
    <plugin>.<verb>(<args>)
  }

  scope <name> {
    config {                          # opaque key-value pairs, passed to adapter
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
      when <predicate>:
        <assertion>
    }

    scenario <name> {
      given { ... }                  # concrete values OR action sequence
      when { ... }                   # predicate (generative) OR action sequence
      then { ... }                   # assertions
    }
  }
}
```

### Include Directive

`include "path/to/file.spec"` splices the contents of another file at the point of inclusion. The included file's tokens are inserted directly into the token stream, so the content must be syntactically valid at that position.

- **Paths are relative** to the including file's directory
- **Recursive includes** are supported (A includes B which includes C)
- **Circular includes** are detected and produce a clear error
- **Duplicate model or scope names** across included files produce an error
- **Downstream transparency**: generator, runner, and adapter see a single merged `*Spec` — no include-awareness needed

The include is resolved at the token level (pass 1) before parsing (pass 2). The parser has zero awareness of includes.

### Scope and Declaration Rules

- **Scope**: A named grouping that owns a contract, invariants, and scenarios. Plugin-specific config (path, method for HTTP; selectors for Playwright) goes in an opaque `config` block. The parser has zero awareness of config semantics — they're passed through to the adapter.
- Contracts, invariants, and scenarios must live inside a scope (not at spec top-level).

### Plugin Definition

```
plugin <name> {
  adapter: "<binary-name>"          # must be on PATH

  actions {
    <verb>(<param>: <type>, ...)
  }

  assertions {
    <property>: <type>
  }
}
```

### Adapter Protocol (JSON over stdin/stdout)

**Action request:**
```json
{"type": "action", "name": "goto", "args": ["/transfer"]}
```

**Action response:**
```json
{"ok": true}
```

**Assertion request:**
```json
{"type": "assert", "locator": "[data-testid=error-msg]", "property": "visible", "expected": true}
```

**Assertion response:**
```json
{"ok": true, "actual": true}
```

**Error response:**
```json
{"ok": false, "error": "element not found"}
```

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

## Prototype Scope

Phase 1: **HTTP plugin + runtime core**
- Parser: spec files → AST
- Generator: contract → random valid inputs
- HTTP adapter: built-in (Go stdlib, no subprocess needed)
- Runner: execute generated inputs, check assertions and invariants
- Target: verify a trivial HTTP API (e.g., bank transfer endpoint)

Phase 2: Playwright plugin + adapter
Phase 3: Go unit plugin + adapter
Phase 4: Metamorphic relation support

## Project Structure

```
speclang/
├── CLAUDE.md
├── go.mod
├── cmd/
│   └── specrun/          # CLI entrypoint
│       └── main.go
├── pkg/
│   ├── parser/           # spec file → AST
│   │   ├── lexer.go
│   │   ├── parser.go
│   │   ├── ast.go
│   │   └── include.go    # include resolution + duplicate validation
│   ├── generator/        # AST → test inputs
│   │   ├── generator.go
│   │   └── shrink.go     # counterexample shrinking
│   ├── runner/           # orchestrates generate → execute → check
│   │   └── runner.go
│   ├── adapter/          # adapter protocol + built-in adapters
│   │   ├── protocol.go   # JSON IPC types
│   │   └── http.go       # built-in HTTP adapter
│   └── plugin/           # plugin spec loading
│       └── plugin.go
├── plugins/
│   └── http.plugin       # HTTP plugin definition (spec file)
├── examples/
│   ├── transfer.spec     # root spec (includes models + scopes)
│   ├── models/
│   │   └── account.spec  # model Account
│   ├── scopes/
│   │   └── transfer.spec # scope transfer (contract, invariants, scenarios)
│   └── server/           # trivial Go HTTP server to test against
│       └── main.go
└── testdata/
    └── include/          # multi-file include test fixtures
        ├── basic/        # root includes models + scopes
        ├── nested/       # A → B → C transitive includes
        ├── circular/     # A ↔ B circular include (error case)
        ├── duplicate/    # duplicate model names (error case)
        └── duplicate_scope/  # duplicate scope names (error case)
```

## Tech Stack

- Go (latest stable)
- No external dependencies for core runtime
- `net/http` for built-in HTTP adapter
- `math/rand/v2` for input generation

## Commands

```bash
go build ./cmd/specrun                    # build the CLI
go test ./...                             # run all tests
./specrun verify examples/transfer.spec   # run verification
```
