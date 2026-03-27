# Design: `before` block + per-iteration adapter reset

**Date:** 2026-03-27
**Issue:** #94
**Status:** Approved

## Problem

Invariants can't express prerequisites like authentication. There's no `given` equivalent — specrun generates inputs and sends requests automatically. Any scope requiring auth headers, cookies, or fixture setup before invariant iterations has no mechanism to establish that state.

## Solution

Add a `before` block at scope level that runs before each scenario's `given` and each invariant iteration. Pair it with per-iteration adapter reset so each iteration starts from a clean slate.

## Execution lifecycle

```
for each scenario:
    reset adapter (fresh http client / fresh playwright page)
    execute before steps
    execute given steps
    execute request
    check assertions

for each invariant:
    for each iteration:
        reset adapter
        execute before steps
        execute generated request
        check assertions
```

## Adapter reset strategy

| Adapter | Reset mechanism | Cost |
|---------|----------------|------|
| `http` | New `HTTPAdapter` instance (new client, cookie jar, empty headers) | ~microseconds |
| `process` | New `ProcessAdapter` instance | ~nanoseconds |
| `playwright` | `new_page` + `clear_state` (reuse browser, fresh tab) | ~50-100ms |

## `before` failure semantics

If any step in `before` fails, the entire scope is aborted — remaining scenarios and invariant iterations are skipped. The failure is reported with `before` context.

## Composition

`before` + `given` compose by concatenation: before steps run first, then given steps. State established in `before` (headers, cookies, assignments) carries into `given` and the subsequent request for that single iteration.

## Syntax

`before` uses the same syntax as `given` — data assignments and action calls, interleaved:

```speclang
scope create_group {
  use http
  config {
    path: "/api/v1/groups"
    method: "POST"
  }

  before {
    http.post("/api/v1/auth/login", { provider: "google", id_token: "test-token" })
    http.header("Authorization", "Bearer " + body.access_token)
  }

  contract {
    input { name: string }
    output { group: any, error: string? }
  }

  invariant creator_is_manager {
    when error == null:
      output.group.membership.role == "manager"
  }
}
```

## Implementation

### AST (`pkg/spec/ast.go`)

Add `Before *Block` to `Scope`. Reuses the existing `Block` type.

### Lexer (`internal/parser/lexer.go`)

- Add `"before"` → `TokenBefore` to keyword map
- Add `TokenBefore` to `isIdentLike` (so it works as a field name)

### Parser (`internal/parser/parser.go`)

- In `parseScopeMember`, add `case TokenBefore:` — parse using same logic as `given` blocks
- Reject duplicate `before` blocks in the same scope

### Validator (`internal/validator/validator.go`)

- Apply the same type-checking to `before` assignments as `given` assignments
- Factor out shared validation logic

### Runner (`internal/runner/runner.go`)

- Add `resetAdapter()` on `scopeRunner`:
  - `http` / `process`: construct new adapter instance, call `Init()` with same config
  - `playwright`: call `new_page` + `clear_state` (reuse browser)
- Call sites (reset → before → continue):
  - `runGivenScenario`: reset → before → given → execute
  - `runWhenIteration`: reset → before → execute
  - `runInvariant` iteration loop: reset → before → execute
  - Shrink passes: reset → before → execute candidate
- `before` failure: return scope-level fatal error, abort remaining iterations

### Tests

- Parser: `before` block parses with assignments and action calls
- Parser: `before` keyword works as a field name
- Parser: duplicate `before` blocks rejected
- Runner: `before` steps execute before `given` steps
- Runner: adapter state is fresh each iteration
- Runner: `before` failure aborts scope
- Self-verification spec fixture
