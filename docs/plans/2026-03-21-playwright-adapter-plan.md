# Playwright Adapter Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a built-in Playwright adapter (`use playwright`) enabling UI specification and verification through browser automation.

**Architecture:** Built-in adapter using playwright-go, integrated into specrun alongside http and process adapters. Parser extended with `@plugin.property` assertion syntax and mixed call/assignment `given` blocks. Runner updated for locator resolution and stateful adapter execution.

**Tech Stack:** Go, playwright-go, existing parser/runner/adapter infrastructure

**Design Doc:** `docs/plans/2026-03-21-playwright-adapter-design.md`

---

### Task 1: Update Plugin Architecture in CLAUDE.md

**Files:**
- Modify: `CLAUDE.md` — the settled decisions section (~line 25)

**Step 1: Update the settled decision**

Change:
```
- **Plugin = spec + adapter binary**: Plugin spec declares typed actions/assertions. Adapter binary implements them over JSON stdin/stdout protocol.
```
To:
```
- **Plugin architecture**: Plugins are either **built-in** (http, process, playwright — compiled into specrun) or **external** (adapter binary on PATH communicating via JSON stdin/stdout). Built-in plugins cover common use cases; external plugins extend the system without modifying specrun.
```

**Step 2: Commit**

```bash
git add CLAUDE.md
git commit -m "docs: update plugin architecture — built-in vs external plugins"
```

---

### Task 2: AST Changes — GivenStep Interface and Block Refactor

**Files:**
- Modify: `pkg/parser/ast.go` — Block struct (~line 93), add GivenStep interface
- Test: `pkg/parser/parser_test.go` — existing tests must still pass

**Step 1: Write test verifying existing given block parsing still works**

In `pkg/parser/given_test.go`:
```go
package parser

import "testing"

func TestParseGivenBlock_Assignments(t *testing.T) {
    spec, err := Parse(`
use http
spec Test {
  scope test {
    contract {
      input { x: int }
      output { y: int }
    }
    scenario smoke {
      given {
        x: 42
      }
      then {
        y: 84
      }
    }
  }
}
`)
    if err != nil {
        t.Fatal(err)
    }
    sc := spec.Scopes[0].Scenarios[0]
    if len(sc.Given.Steps) != 1 {
        t.Fatalf("expected 1 given step, got %d", len(sc.Given.Steps))
    }
    a, ok := sc.Given.Steps[0].(*Assignment)
    if !ok {
        t.Fatalf("expected *Assignment, got %T", sc.Given.Steps[0])
    }
    if a.Path != "x" {
        t.Errorf("path = %q, want 'x'", a.Path)
    }
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./pkg/parser/ -run TestParseGivenBlock_Assignments -v`
Expected: FAIL — `Block` has no `Steps` field yet

**Step 3: Implement AST changes**

In `pkg/parser/ast.go`:

Add GivenStep interface:
```go
// GivenStep is a step in a given block — either an assignment or an action call.
type GivenStep interface{ givenStep() }

func (*Assignment) givenStep() {}
func (*Call) givenStep()       {}
```

Replace `Block.Assignments` with `Block.Steps`:
```go
type Block struct {
    Steps      []GivenStep    `json:"steps,omitempty"`      // ordered: assignments + calls (given blocks)
    Predicates []Expr         `json:"predicates,omitempty"` // when-predicate conditions
    Assertions []*Assertion   `json:"assertions,omitempty"` // then-block checks
}
```

**Step 4: Fix all compilation errors from Block.Assignments removal**

Files that reference `Block.Assignments`:
- `pkg/parser/parser.go` — `parseGivenBlock` (~line 782): change to append to `Steps`
- `pkg/runner/runner.go` — `runGivenScenario` (~line 261): change `assignmentsToMap(sc.Given.Assignments)` to extract assignments from `Steps`
- `pkg/runner/runner.go` — `assignmentsToMap` (~line 543): update parameter or add step-walking helper

Create helper in runner:
```go
func stepsToMap(steps []parser.GivenStep) map[string]any {
    result := make(map[string]any)
    for _, s := range steps {
        if a, ok := s.(*parser.Assignment); ok {
            setPath(result, a.Path, exprToValue(a.Value))
        }
    }
    return result
}
```

**Step 5: Run all tests**

Run: `go test ./... -count=1`
Expected: ALL PASS (no behavior change for existing specs)

**Step 6: Commit**

```bash
git add pkg/parser/ast.go pkg/parser/parser.go pkg/parser/given_test.go pkg/runner/runner.go
git commit -m "refactor(ast): replace Block.Assignments with ordered GivenStep slice"
```

---

### Task 3: Parser — Action Calls in Given Blocks

**Files:**
- Modify: `pkg/parser/parser.go` — `parseGivenBlock` (~line 782)
- Test: `pkg/parser/given_test.go`

**Step 1: Write test for action calls in given blocks**

```go
func TestParseGivenBlock_ActionCalls(t *testing.T) {
    spec, err := Parse(`
use playwright
spec Test {
  scope test {
    contract {
      input { x: int }
      output { ok: bool }
    }
    scenario ui_flow {
      given {
        playwright.fill(username, "alice")
        x: 42
        playwright.click(submit)
      }
      then {
        ok: true
      }
    }
  }
}
`)
    if err != nil {
        t.Fatal(err)
    }
    sc := spec.Scopes[0].Scenarios[0]
    if len(sc.Given.Steps) != 3 {
        t.Fatalf("expected 3 given steps, got %d", len(sc.Given.Steps))
    }

    // Step 0: playwright.fill(username, "alice")
    c0, ok := sc.Given.Steps[0].(*Call)
    if !ok {
        t.Fatalf("step 0: expected *Call, got %T", sc.Given.Steps[0])
    }
    if c0.Namespace != "playwright" || c0.Method != "fill" {
        t.Errorf("step 0: got %s.%s, want playwright.fill", c0.Namespace, c0.Method)
    }

    // Step 1: x: 42 (assignment)
    _, ok = sc.Given.Steps[1].(*Assignment)
    if !ok {
        t.Fatalf("step 1: expected *Assignment, got %T", sc.Given.Steps[1])
    }

    // Step 2: playwright.click(submit)
    c2, ok := sc.Given.Steps[2].(*Call)
    if !ok {
        t.Fatalf("step 2: expected *Call, got %T", sc.Given.Steps[2])
    }
    if c2.Method != "click" {
        t.Errorf("step 2: method = %q, want 'click'", c2.Method)
    }
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./pkg/parser/ -run TestParseGivenBlock_ActionCalls -v`
Expected: FAIL — parser doesn't handle calls in given blocks

**Step 3: Implement mixed given block parsing**

Modify `parseGivenBlock` to detect calls vs assignments:
- Peek at first two tokens: if `ident` followed by `.` → parse as Call
- If `ident` followed by `:` → parse as Assignment
- If `ident` followed by `(` → parse as Call (no namespace, like `set(x, 42)` or `login("alice")`)

**Step 4: Run tests**

Run: `go test ./pkg/parser/ -v -count=1`
Expected: ALL PASS

**Step 5: Commit**

```bash
git add pkg/parser/parser.go pkg/parser/given_test.go
git commit -m "feat(parser): support action calls in given blocks"
```

---

### Task 4: Parser — `@plugin.property` Assertion Syntax

**Files:**
- Modify: `pkg/parser/parser.go` — `parseThenBlock` (~line 834)
- Test: `pkg/parser/assertion_test.go` (new)

**Step 1: Write test for @ assertion syntax**

```go
package parser

import "testing"

func TestParseThenBlock_AtSyntax(t *testing.T) {
    spec, err := Parse(`
use playwright
spec Test {
  locators {
    welcome: [data-testid=welcome]
  }
  scope test {
    contract {
      input { x: int }
      output { ok: bool }
    }
    scenario check_ui {
      given { x: 1 }
      then {
        welcome@playwright.visible: true
        welcome@playwright.text: "hello"
      }
    }
  }
}
`)
    if err != nil {
        t.Fatal(err)
    }
    assertions := spec.Scopes[0].Scenarios[0].Then.Assertions

    if len(assertions) != 2 {
        t.Fatalf("expected 2 assertions, got %d", len(assertions))
    }

    a0 := assertions[0]
    if a0.Target != "welcome" {
        t.Errorf("a0.Target = %q, want 'welcome'", a0.Target)
    }
    if a0.Plugin != "playwright" {
        t.Errorf("a0.Plugin = %q, want 'playwright'", a0.Plugin)
    }
    if a0.Property != "visible" {
        t.Errorf("a0.Property = %q, want 'visible'", a0.Property)
    }
}

func TestParseThenBlock_PathAssertion_Unchanged(t *testing.T) {
    spec, err := Parse(`
use http
spec Test {
  scope test {
    contract {
      input { x: int }
      output { y: int }
    }
    scenario smoke {
      given { x: 1 }
      then { y: 2 }
    }
  }
}
`)
    if err != nil {
        t.Fatal(err)
    }
    a := spec.Scopes[0].Scenarios[0].Then.Assertions[0]
    if a.Target != "y" || a.Plugin != "" || a.Property != "" {
        t.Errorf("path assertion should have empty Plugin/Property, got Plugin=%q Property=%q", a.Plugin, a.Property)
    }
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./pkg/parser/ -run TestParseThenBlock_AtSyntax -v`
Expected: FAIL — parser doesn't handle `@`

**Step 3: Implement @ syntax in parseThenBlock**

After parsing the field path (Target), peek for `TokenAt`:
```go
// After parseFieldPath returns target:
if p.peek().Type == TokenAt {
    p.advance() // consume @
    plugin, err := p.expectIdent()  // "playwright"
    if _, err := p.expect(TokenDot); err != nil { ... }
    property, err := p.expectIdent()  // "visible"
    a.Plugin = plugin.Value
    a.Property = property.Value
}
```

**Step 4: Run tests**

Run: `go test ./pkg/parser/ -v -count=1`
Expected: ALL PASS (both new @ tests and existing path assertion tests)

**Step 5: Commit**

```bash
git add pkg/parser/parser.go pkg/parser/assertion_test.go
git commit -m "feat(parser): add @plugin.property assertion syntax"
```

---

### Task 5: Runner — Locator Resolution in Assertions

**Files:**
- Modify: `pkg/runner/runner.go` — `checkThenAssertions` (~line 343)
- Test: `pkg/runner/runner_test.go`

**Step 1: Write test for locator-resolved assertions**

Test that when `Assertion.Plugin` is set, the runner resolves the locator name from `spec.Locators` and passes it + property to the adapter.

**Step 2: Implement locator resolution**

In `checkThenAssertions`, before the adapter.Assert call:
```go
property := a.Target
locator := ""
if a.Plugin != "" {
    // Resolve named locator to CSS selector
    selector, ok := sr.runner.spec.Locators[a.Target]
    if !ok {
        return nil, fmt.Errorf("locator %q not defined in locators block", a.Target)
    }
    locator = selector
    property = a.Property
}
resp, err := sr.runner.adapter.Assert(property, locator, expected)
```

**Step 3: Run all tests**

Run: `go test ./... -count=1`
Expected: ALL PASS (HTTP adapter ignores locator, so existing behavior unchanged)

**Step 4: Commit**

```bash
git add pkg/runner/runner.go
git commit -m "feat(runner): resolve locators for @plugin.property assertions"
```

---

### Task 6: Runner — Execute Given Steps in Order

**Files:**
- Modify: `pkg/runner/runner.go` — `runGivenScenario` (~line 260)

**Step 1: Implement step-by-step given execution**

Update `runGivenScenario` to walk `sc.Given.Steps` in order:
- `*Assignment` → accumulate into input map
- `*Call` → marshal args, call `adapter.Action(call.Method, args)`, check for error

For HTTP/process specs (all assignments), behavior is unchanged — steps are collected into a map and one action is sent.

For Playwright specs (mixed calls and assignments), calls execute sequentially against the adapter, assignments accumulate into the input context for assertion evaluation.

**Step 2: Run all tests**

Run: `go test ./... -count=1`
Expected: ALL PASS

**Step 3: Commit**

```bash
git add pkg/runner/runner.go
git commit -m "feat(runner): execute given steps in order (calls + assignments)"
```

---

### Task 7: Playwright Adapter — Core Implementation

**Files:**
- Create: `pkg/adapter/playwright.go`
- Modify: `cmd/specrun/main.go` — `createAdapter` (~line 251)
- Modify: `go.mod` — add playwright-go dependency

**Step 1: Add playwright-go dependency**

```bash
go get github.com/playwright-community/playwright-go
```

**Step 2: Implement PlaywrightAdapter**

In `pkg/adapter/playwright.go`:

```go
type PlaywrightAdapter struct {
    pw      *playwright.Playwright
    browser playwright.Browser
    page    playwright.Page
    baseURL string
    timeout float64
}
```

**Init**: Read `base_url` (required), `headless` (default "true"), `timeout` (default "5000"). Call `playwright.Run()`, launch browser, create initial page. On failure: detect missing browsers and give clear error with install instructions.

**Action dispatch**:
- `goto` → `page.Goto(baseURL + url)`
- `click` → `page.Locator(selector).Click()`
- `fill` → `page.Locator(selector).Fill(value)`
- `type` → `page.Locator(selector).Type(value)`
- `select` → `page.Locator(selector).SelectOption(value)`
- `check` → `page.Locator(selector).Check()`
- `uncheck` → `page.Locator(selector).Uncheck()`
- `wait` → `page.Locator(selector).WaitFor()`
- `new_page` → create new page, store as current
- `close_page` → close current page
- `clear_state` → clear cookies and localStorage

**Assert**: Query `page.Locator(locator)`, check property:
- `visible` → `locator.IsVisible()`
- `text` → `locator.TextContent()`
- `value` → `locator.InputValue()`
- `checked` → `locator.IsChecked()`
- `disabled` → `locator.IsDisabled()`
- `count` → `locator.Count()`
- `attribute.<name>` → `locator.GetAttribute(name)`

Compare actual vs expected using `json.Marshal`/`reflect.DeepEqual` (same pattern as HTTP adapter).

**Close**: Close all pages, close browser, stop playwright.

**Step 3: Register in createAdapter**

```go
case "playwright":
    adp := adapter.NewPlaywrightAdapter()
    if err := adp.Init(targetConfig); err != nil {
        return nil, err
    }
    return adp, nil
```

**Step 4: Build**

Run: `go build ./cmd/specrun`
Expected: Compiles successfully

**Step 5: Commit**

```bash
git add pkg/adapter/playwright.go cmd/specrun/main.go go.mod go.sum
git commit -m "feat(adapter): add built-in Playwright adapter using playwright-go"
```

---

### Task 8: Runner — Generative Iteration with Page Isolation

**Files:**
- Modify: `pkg/runner/runner.go` — the when-scenario execution path

**Step 1: Implement per-iteration page lifecycle**

In the generative scenario runner (where `when` blocks generate N inputs), detect if the adapter supports page lifecycle actions. Before each iteration:
1. `adapter.Action("new_page", nil)`
2. `adapter.Action("goto", [config.url])`

After each iteration:
3. `adapter.Action("close_page", nil)`

The HTTP/process adapters will return errors for `new_page`/`close_page` — so only inject these calls when `spec.Uses[0] == "playwright"` or when the adapter supports them (try, ignore error if unsupported).

Better: add a method to detect adapter capabilities, or just use the plugin name.

**Step 2: Run all tests**

Run: `go test ./... -count=1`
Expected: ALL PASS

**Step 3: Commit**

```bash
git add pkg/runner/runner.go
git commit -m "feat(runner): per-iteration page isolation for playwright scopes"
```

---

### Task 9: Plugin Definition File

**Files:**
- Create: `plugins/playwright.plugin`

**Step 1: Write the plugin spec**

```
plugin playwright {
    adapter: "builtin"

    actions {
        goto(url: string)
        click(selector: string)
        fill(selector: string, value: string)
        type(selector: string, value: string)
        select(selector: string, value: string)
        check(selector: string)
        uncheck(selector: string)
        wait(selector: string)
        new_page()
        close_page()
        clear_state()
    }

    assertions {
        visible: bool
        text: string
        value: string
        checked: bool
        disabled: bool
        count: int
    }
}
```

**Step 2: Commit**

```bash
git add plugins/playwright.plugin
git commit -m "feat: add playwright plugin definition"
```

---

### Task 10: Write Self-Verification Spec

**Files:**
- Create: `testdata/playwright/login.spec` — test fixture spec using playwright
- Modify: `specs/parse.spec` — add scenario verifying playwright spec parses

Since we can't run a real browser in the self-verification (no test server), test that:
1. The parser accepts a spec with `use playwright`, locators, `@` assertions, and mixed given blocks
2. The `specrun parse` command produces correct AST output

**Step 1: Create test fixture**

`testdata/playwright/login.spec`:
```
use playwright

spec LoginUI {
    description: "Login page UI verification"

    target {
        base_url: env(APP_URL, "http://localhost:3000")
    }

    locators {
        username: [data-testid=username]
        password: [data-testid=password]
        submit:   [data-testid=submit]
        welcome:  [data-testid=welcome]
        error:    [data-testid=error]
    }

    scope login {
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
                playwright.fill(username, "alice")
                playwright.fill(password, "secret")
                user: "alice"
                pass: "secret"
                playwright.click(submit)
            }
            then {
                welcome@playwright.visible: true
                welcome@playwright.text: "Welcome, alice"
            }
        }
    }
}
```

**Step 2: Add self-verification scenario**

In `specs/parse.spec`, add to `parse_valid`:
```
scenario playwright_spec {
    given {
        file: "testdata/playwright/login.spec"
    }
    then {
        exit_code: 0
        name: "LoginUI"
    }
}
```

**Step 3: Build and verify**

```bash
go build -o specrun ./cmd/specrun
./specrun parse testdata/playwright/login.spec
SPECRUN_BIN=./specrun ./specrun verify specs/speclang.spec
```

**Step 4: Commit**

```bash
git add testdata/playwright/ specs/parse.spec
git commit -m "test: add self-verification spec for playwright syntax parsing"
```

---

### Task 11: Documentation Updates

**Files:**
- Modify: `CLAUDE.md` — add playwright to spec structure example, project structure
- Modify: `skills/author/references/api_reference.md` — add playwright plugin docs
- Modify: `skills/author/SKILL.md` — mention playwright in checklist
- Modify: `README.md` — add playwright section

**Step 1: Update all docs**

Add to API reference:
- Playwright plugin actions and assertions
- `@plugin.property` assertion syntax
- `locators` block usage
- Mixed given block syntax

Add to CLAUDE.md:
- Playwright in project structure
- Updated plugin architecture settled decision (if not done in Task 1)

Add to README:
- Playwright section with example spec

**Step 2: Commit**

```bash
git add CLAUDE.md README.md skills/
git commit -m "docs: add playwright adapter documentation and skill references"
```

---

### Task 12: End-to-End Verification

**Step 1: Run full test suite**

```bash
go test ./... -count=1
```

**Step 2: Run self-verification**

```bash
go build -o specrun ./cmd/specrun
SPECRUN_BIN=./specrun ./specrun verify specs/speclang.spec
```

**Step 3: Test parse on playwright spec**

```bash
./specrun parse testdata/playwright/login.spec | head -30
```

Verify the AST contains:
- Locators block with CSS selectors
- Assertions with `Plugin` and `Property` fields populated
- Given block with mixed Call and Assignment steps

**Step 4: Push and create PR**

```bash
git push -u origin feat/playwright-adapter
gh pr create --title "feat: add Playwright adapter for UI specifications"
```
