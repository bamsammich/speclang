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
- **Plugin architecture**: Plugins are either **built-in** (http, process, playwright — compiled into specrun) or **external** (adapter binary on PATH communicating via JSON stdin/stdout). Built-in plugins cover common use cases; external plugins extend the system without modifying specrun.
- **Scope-level plugin declaration**: `use <plugin>` appears inside each `scope` block, not at spec level. Each scope independently declares which plugin drives it.
- **Namespaced calls**: Plugin methods are called as `plugin.method()` (e.g., `playwright.fill(locator, value)`)
- **Assertion syntax in `then` blocks**: `locator@plugin.property: expected` (e.g., `error_msg@playwright.visible: true`)
- **Error pseudo-field**: `error` in `then` blocks asserts against action errors when `error` is NOT a contract output field. When an adapter action returns `{ok: false}`, the error string is captured and assertable via `then { error: "expected message" }`. Use `error: null` to assert no error occurred.
- **Three scenario types** (ascending verification strength):
  - `scenario` with `given` — concrete values, smoke test / documentation
  - `scenario` with `when` — predicate over input class, runtime generates across matching space
  - `invariant` — universal law, must hold for ALL valid inputs
- **Runtime is a Go binary** that parses specs, generates inputs, and delegates execution to adapter binaries over IPC.
- **Scope-based grouping**: Contracts, invariants, and scenarios live inside named `scope` blocks. Each scope has an opaque `config` block for plugin-specific settings (e.g., HTTP path/method). The parser is agnostic to config semantics.
- **Counterexample shrinking**: When a failure is found, the runtime performs binary-search shrinking (ints toward 0, strings toward shorter prefixes, nested models recursively) to produce minimal counterexamples.
- **Built-in functions**:
  - `len(expr)` — returns length of string, array, or map
  - `contains(haystack, needle)` — returns `bool`. String haystack + string needle performs substring check; `[]any` haystack + any needle performs element membership check.
  - `exists(expr)` — returns `true` if path resolves to a value (including `null`), `false` if path doesn't exist
  - `has_key(expr, "key")` — returns `true` if map contains the specified key

### Spec File Structure

```
include "<path>"                     # top-level include

spec <Name> {

  description: "<description>"            # optional, for AI context

  target {
    base_url: env(APP_URL)          # optional, plugin-dependent config
  }

  include "<path>"                   # spec-body include
  import openapi("<path>")           # import models/scopes from OpenAPI schema
  import proto("<path>")             # import models/scopes from protobuf schema

  locators {                         # named selectors for UI specs
    <name>: [<css-selector>]
  }

  model <Name> {
    <field>: <type>                    # int, float, string, bytes, bool, any,
                                       # []T, map[K,V], enum("a","b",...), ModelName
                                       # append ? for optional: string?, enum(...)?
    <field>: <type> { <constraint> }
  }

  action <name>(<args>) {
    <plugin>.<verb>(<args>)
  }

  scope <name> {
    use <plugin>                      # declares which plugin drives this scope

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
      given {                        # concrete assignments and/or action calls
        <field>: <value>
        <plugin>.<verb>(<args>)
      }
      when { ... }                   # predicate (generative) OR action sequence
      then {                         # assertions
        <locator>@<plugin>.<property>: <expected>
      }
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

### Import Directive

`import <adapter>("path")` imports models and scopes from an external schema file. The adapter name determines the format (currently only `openapi` is supported).

```
import openapi("schema.yaml")
```

- **Paths are relative** to the spec file's directory
- **OpenAPI 3.x** (YAML or JSON) schemas are supported via [kin-openapi](https://github.com/getkin/kin-openapi)
- **Models** are generated from `components/schemas` (object types with properties)
- **Scopes** are generated from `paths` (each path+method → scope with config and contract)
- **Type mapping**: `integer` → `int`, `string` → `string`, `boolean` → `bool`, `$ref` → model name
- **Constraints**: `minimum`/`maximum` → field constraint expressions
- **Unsupported types** (array, float, enum) are skipped with a warning
- **Duplicate model or scope names** between imported and hand-written produce an error
- **Downstream transparency**: generator, runner, and adapter see standard AST nodes — no import-awareness needed

The import is resolved at parse time. The parser dispatches to a pluggable `ImportResolver` based on the adapter name.

### Scope and Declaration Rules

- **Scope**: A named grouping that owns a contract, invariants, and scenarios. Plugin-specific config (path, method for HTTP; selectors for Playwright) goes in an opaque `config` block. The parser has zero awareness of config semantics — they're passed through to the adapter.
- Each scope must declare `use <plugin>` to specify which adapter drives it. Different scopes in the same spec can use different plugins.
- Contracts, invariants, and scenarios must live inside a scope (not at spec top-level).

### Expressions

Expressions appear in constraints, invariants, `when` predicates, and `then` assertions.

- **Literals**: `42`, `3.14`, `"hello"`, `true`, `false`, `null`
- **Field references**: `from.balance`, `output.error`
- **Environment**: `env(VAR)`, `env(VAR, "default")`
- **Objects**: `{ id: "alice", balance: 100 }`
- **Arrays**: `[1, 2, 3]`
- **Operators**: `==`, `!=`, `>`, `<`, `>=`, `<=`, `+`, `-`, `*`, `&&`, `||`, `!`
- **Functions**: `len(expr)`
- **Conditionals**: `if condition then expr else expr` — condition must evaluate to bool, returns the then-branch or else-branch value. Nesting is supported with parentheses: `if a then (if b then x else y) else z`

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

This protocol applies to **external** adapters (subprocess communication). **Built-in adapters** (http, process, playwright) implement the same `Adapter` interface in Go directly — no JSON IPC, no subprocess.

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

### Error Pseudo-Field

`error` is a special pseudo-field in `then` blocks that asserts against adapter action errors. It activates only when `error` is NOT declared in the scope's contract output fields. When `error` IS a contract output field, it behaves as a normal field assertion.

When an adapter's `Action()` returns `{ok: false, error: "..."}`, the error string is captured instead of failing the test. The `then` block can then assert on it:

```
then {
  error: "element not found"     # expect this specific error
}
```

```
then {
  error: null                     # expect no error
}
```

Behavior:
- If `error` is asserted and the action fails, the error string is compared against the expected value.
- If `error` is asserted as `null` and no error occurred, the assertion passes.
- If `error` is asserted but no error occurred, the assertion fails.
- If an action fails but `error` is NOT asserted in `then`, the test fails as before (backward compatible).
- If `error` is declared in the contract's output fields, it's treated as a regular output field — not the pseudo-field.

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

Phase 2: **Playwright plugin + built-in adapter** (scope-level `use`, locators, action sequences in `given`)
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

## HTTP Adapter

The HTTP adapter (`use http`) tests HTTP APIs. It supports single-request scopes (via `config` with `path` and `method`) and multi-step workflows (via action calls in `given` blocks).

### Config (from `target` block)

- `base_url` — API base URL (required); supports `env()` expressions

### Config (from scope `config` block)

- `path` — request path (for single-request scopes)
- `method` — HTTP method: GET, POST, PUT, DELETE (for single-request scopes)

### Actions

- `http.get(path)` — GET request
- `http.post(path, body)` — POST request with JSON body
- `http.put(path, body)` — PUT request with JSON body
- `http.delete(path)` — DELETE request
- `http.header(name, value)` — set a persistent header for all subsequent requests

### Assertions

- `status` — HTTP status code (int)
- `body` — full response body (parsed JSON)
- `header.<name>` — response header value
- `<field.path>` — dot-path traversal into JSON response body

### Multi-step `given` blocks

HTTP scopes support action calls in `given` blocks for multi-step workflows. Headers and cookies persist across calls within a scenario. `then` assertions apply to the last response.

```
scope create_and_verify {
  use http

  scenario create_then_get {
    given {
      http.post("/api/resources", { name: "widget" })
      http.get("/api/resources/1")
    }
    then {
      status: 200
      name: "widget"
    }
  }
}
```

When `given` contains only field assignments (no action calls), the scope's `config` block `path` and `method` determine the single request. When `given` contains action calls, each call executes independently and `config` is not used for request dispatch.

## Process Adapter

The process adapter (`use process`) executes subprocesses and asserts against their output. It mirrors the HTTP adapter's pattern.

### Config (from `target` block)

- `command` — binary to run (required)

### Config (from scope `config` block)

- `args` — base arguments prepended to every exec call (optional, space-separated)

### Action: `exec`

Runs `command [...args] [...input_fields]`. Captures exit code, stdout (best-effort JSON parse), and stderr.

### Assertions

- `exit_code` — integer comparison
- `stdout` — full parsed JSON body (or raw string if not JSON)
- `stdout.field.path` — dot-path traversal into parsed JSON
- `stdout.items.0.name` — numeric segment in dot-path indexes into a JSON array by position
- `stderr` — raw string

## Playwright Adapter

The playwright adapter (`use playwright`) drives a browser via [playwright-go](https://github.com/playwright-community/playwright-go) and is compiled into specrun (no subprocess). It uses the `locators` block for named CSS selectors and supports interleaved action calls and field assignments in `given` blocks.

### Config (from `target` block)

- `base_url` — browser start URL (required)

### Actions

- `playwright.goto(url)` — navigate to URL
- `playwright.fill(locator, value)` — fill an input
- `playwright.click(locator)` — click an element

### Assertions

- `<locator>@playwright.visible: <bool>` — element visibility
- `<locator>@playwright.text: <string>` — element text content
- `<locator>@playwright.value: <string>` — input value

`<locator>` is either a named locator from the spec's `locators` block or a raw CSS selector string.

### Mixed `given` blocks

`given` blocks may interleave field assignments and action calls. This works with any adapter that supports action calls (Playwright and HTTP):

```
# Playwright example
given {
  amount: 1000
  playwright.goto("/transfer")
  playwright.fill(amount_input, amount)
  playwright.click(submit_btn)
}

# HTTP multi-step example
given {
  http.header("Authorization", "Bearer token")
  http.post("/api/items", { name: "widget" })
  http.get("/api/items/1")
}
```

## Self-Verification

Speclang verifies itself with its own specs via `specs/speclang.spec`. This is black-box verification through the runtime — speclang is both the verifier and the system under test.

The self-verification spec uses the process adapter to invoke `specrun` subcommands and verify their behavior:

- **parse_valid** — verifies the parser accepts valid specs and produces expected AST structure
- **parse_invalid** — verifies the parser rejects malformed specs with exit code 1
- **import_openapi** — verifies OpenAPI imports produce correct models (names, fields, types, optionality) and scopes
- **import_openapi_constraints** — verifies minimum/maximum constraints are preserved from OpenAPI schemas
- **import_openapi_refs** — verifies $ref fields resolve to correct model type names
- **import_proto** — verifies protobuf imports produce correct models (names, fields, types) and scopes from service RPCs
- **import_proto_streaming** — verifies streaming RPCs are skipped, only unary RPCs produce scopes
- **generate** — verifies the generator produces constraint-satisfying outputs across seeds
- **verify_pass** — verifies that `specrun verify` passes correct implementations
- **verify_fail** — verifies that `specrun verify` detects incorrect implementations
- **shrinking** — verifies that counterexample shrinking produces minimal values (ints near boundary, empty strings, zero balances)

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
