# Design: Playwright Adapter for UI Specifications

**Date**: 2026-03-21
**Issue**: #26
**Status**: Approved

## Overview

Add a built-in Playwright adapter (`use playwright`) enabling UI specification and verification. The adapter uses playwright-go to control browsers, with the same Adapter interface used by HTTP and process adapters.

## Key Design Decisions

### 1. Built-in Adapter (not separate binary)

Playwright is a built-in adapter compiled into specrun, like HTTP and process. The plugin architecture is updated to distinguish:

- **Built-in plugins** (http, process, playwright) — compiled into specrun
- **External plugins** — adapter binary on PATH, JSON IPC over stdin/stdout

The user installs Playwright browsers themselves (`npx playwright install chromium`). specrun detects missing browsers and gives a clear error with install instructions.

### 2. Given Blocks Are Ordered Call Sequences

`given` blocks are a uniform sequence of calls executed in order. Assignments (`key: value`) are syntactic sugar for `set(key, value)`. Both forms produce the same AST node (`*Call`).

```
given {
    playwright.goto("/transfer")
    set(from_account, "alice")         # explicit set
    playwright.fill(from_field, "alice")
    amount: 100                         # sugar for set(amount, 100)
    playwright.click(submit_btn)
}
```

This enables:
- Mixed data setup and UI actions in one block
- Multi-page flows (navigate, fill, click, navigate again)
- Consistent syntax: everything is a call
- Backward compatibility: assignment syntax keeps working

The `Block` AST struct uses `[]GivenStep` (interface satisfied by `*Assignment` and `*Call`) to preserve ordering.

### 3. `@plugin.property` Assertion Syntax (Optional, All Adapters)

The `locator@plugin.property: expected` syntax is available to all adapters, not Playwright-specific. The runner branches:

- `Assertion.Plugin` is set → resolve locator from `spec.Locators`, call `adapter.Assert(property, cssSelector, expected)`
- `Assertion.Plugin` is empty → existing path-based call `adapter.Assert(target, "", expected)`

HTTP specs keep working as-is. Playwright specs use `@` because they need locator resolution.

```
# HTTP (works today, unchanged):
then { from.balance: 70 }

# Playwright (uses @ for locator resolution):
then { welcome@playwright.visible: true }
```

### 4. New Page Per Iteration (Generative Tests)

For `when`-predicate generative scenarios, each iteration gets a fresh browser page:

```
Adapter.Init()     → launch browser
Scope start        → new_page + goto(config.url)
Each iteration     → close_page, new_page, goto(config.url)
Scope end          → close all pages
Adapter.Close()    → kill browser process
```

Prevents state leakage between iterations. Pages are tracked and cleaned up on scope end and adapter close.

## Adapter API

### Actions

| Action | Args | Description |
|--------|------|-------------|
| `goto(url)` | URL string | Navigate (prepends base_url if relative) |
| `click(selector)` | CSS selector | Click element |
| `fill(selector, value)` | selector + text | Clear and type into input |
| `type(selector, value)` | selector + text | Append text (no clear) |
| `select(selector, value)` | selector + option | Select dropdown |
| `check(selector)` | selector | Check checkbox |
| `uncheck(selector)` | selector | Uncheck checkbox |
| `wait(selector)` | selector | Wait for element visible |
| `new_page()` | none | Create fresh page |
| `close_page()` | none | Close current page |
| `clear_state()` | none | Clear cookies/localStorage |

### Assertions

| Property | Type | Description |
|----------|------|-------------|
| `visible` | bool | Element is visible |
| `text` | string | Text content |
| `value` | string | Input value |
| `checked` | bool | Checkbox state |
| `disabled` | bool | Disabled state |
| `count` | int | Number of matching elements |
| `attribute.<name>` | string | Attribute value |

### Configuration

Target block:
```
target {
    base_url: env(APP_URL, "http://localhost:3000")
    headless: "true"       # default, set "false" for debugging
    timeout: "5000"        # milliseconds, default 5s
}
```

## Parser Changes

### A. `parseThenBlock` — Handle `@` Syntax

After parsing the field path, peek for `TokenAt`. If present, consume `@`, read `plugin.property`, populate `Assertion.Plugin` and `Assertion.Property`.

### B. `parseGivenBlock` — Unified Call Sequences

`given` blocks accept both assignment syntax and call syntax, interleaved:
- `ident: expr` → desugar to `Call{Method: "set", Args: [ident, expr]}`
- `ident.ident(args)` → parse as `Call` (existing Call parsing)

Both produce `GivenStep` elements in an ordered slice.

### C. AST Changes

```go
type GivenStep interface{ givenStep() }
func (*Assignment) givenStep() {}
func (*Call) givenStep()       {}

type Block struct {
    Steps      []GivenStep    // ordered: assignments + calls (given blocks)
    Predicates []Expr         // when blocks
    Assertions []*Assertion   // then blocks
}
```

## Runner Changes

### Assertion Flow (Unified)

```
if a.Plugin != "":
    selector = spec.Locators[a.Target]  // resolve named locator
    adapter.Assert(a.Property, selector, expected)
else:
    adapter.Assert(a.Target, "", expected)  // existing path-based
```

### Given Block Execution

Walk steps in order:
- `*Assignment` → accumulate into input context `map[string]any`
- `*Call` with Method "set" (from desugared assignment) → accumulate into input context
- `*Call` with other Method → execute via `adapter.Action(call.Method, args)`

For HTTP scopes with only assignments, the runner collects them all and sends one request — same as today.

### Generative Iteration (Playwright)

```
for each generated input:
    adapter.Action("new_page")
    adapter.Action("goto", config.url)
    execute when-block actions with generated values
    check assertions
    adapter.Action("close_page")
```

## Plugin Architecture Update (CLAUDE.md)

Update settled decision from:
> "Plugin = spec + adapter binary"

To:
> "Plugins are either **built-in** (http, process, playwright — compiled into specrun) or **external** (adapter binary on PATH communicating via JSON stdin/stdout). Built-in plugins cover common use cases; external plugins extend the system without modifying specrun."

## Example Spec

```
use playwright

spec TransferApp {
    description: "Banking transfer UI"

    target {
        base_url: env(APP_URL, "http://localhost:3000")
    }

    locators {
        username_field: [data-testid=username]
        password_field: [data-testid=password]
        submit_btn:     [data-testid=submit]
        error_msg:      [data-testid=error]
        balance:        [data-testid=balance]
        welcome:        [data-testid=welcome]
    }

    action login(user, pass) {
        playwright.fill(username_field, user)
        playwright.fill(password_field, pass)
        playwright.click(submit_btn)
        playwright.wait(welcome)
    }

    scope login {
        config {
            url: "/login"
        }

        scenario successful_login {
            given {
                login("alice", "secret")
            }
            then {
                welcome@playwright.visible: true
                welcome@playwright.text: "Welcome, alice"
            }
        }

        scenario invalid_credentials {
            given {
                login("alice", "wrong")
            }
            then {
                error_msg@playwright.visible: true
                error_msg@playwright.text: "Invalid credentials"
            }
        }
    }
}
```

## Known Limitations

- **No NaN/Inf** in float assertions (JSON incompatible)
- **Generative UI testing** works best for form-like interactions (validation, bounds checking), not complex navigation flows
- **No screenshot on failure** in v1 (future enhancement)
- **Single adapter per spec** — `spec.Uses[0]` selects the adapter. Multi-adapter specs deferred.
- **No inline selectors** in v1 — all locators must be pre-declared in the `locators` block
