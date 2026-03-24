# Repo Organization & Go Idiom Cleanup Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Fix all 71 golangci-lint violations, refactor high-complexity functions, clean up the repo, and rewrite documentation for new users.

**Architecture:** Four PRs in sequence: (1a) auto-fixable lint + Makefile + dead code, (1b) complexity refactoring + error handling + token enum, (2) repo cleanup, (3) documentation rewrite. Each PR is independently mergeable.

**Tech Stack:** Go, golangci-lint 2.10.1, gh CLI

---

## PR 1a: Auto-fixable Lint + Makefile + Dead Code

### Task 1: Add Makefile

**Files:**
- Create: `Makefile`

**Step 1: Create Makefile**

```makefile
.PHONY: build lint fmt test clean

build:
	go build -o ./specrun ./cmd/specrun

lint:
	golangci-lint run ./...

fmt:
	golangci-lint fmt ./...

test:
	go test -race -count=1 ./...

clean:
	rm -f ./specrun ./echo_tool
```

**Step 2: Verify**

Run: `make lint` (should show the 71 existing issues — we'll fix them next)
Run: `make test`
Run: `make build && make clean`

**Step 3: Commit**

```bash
git add Makefile
git commit -m "chore: add Makefile with build, lint, fmt, test, clean targets"
```

---

### Task 2: Fix formatting issues (gci + golines)

**Files:**
- Modify: `cmd/specrun/main.go` (gci import ordering)
- Modify: `pkg/adapter/playwright.go` (gci import ordering)
- Modify: `pkg/runner/runner.go` (gci import ordering)
- Modify: `pkg/generator/array_test.go` (golines)
- Modify: `pkg/generator/enum_test.go` (golines)
- Modify: `pkg/generator/quantifier_test.go` (golines)

**Step 1: Auto-fix**

Run: `make fmt`

**Step 2: Verify**

Run: `golangci-lint run ./... 2>&1 | grep -E "gci|golines"` — should be empty.

**Step 3: Commit**

```bash
git add -A
git commit -m "style: fix import ordering and line length (gci, golines)"
```

---

### Task 3: Remove dead code

**Files:**
- Modify: `pkg/runner/runner.go:892-900` — remove `exprToJSON` function
- Modify: `pkg/parser/lexer_test.go:174-188` — remove `findTokenSeq` function
- Modify: `pkg/validator/validator_test.go:851` — remove `_ = fmt.Sprintf("test")` line and the unnecessary `fmt` import

**Step 1: Remove each piece of dead code**

Delete `exprToJSON` (runner.go lines 892-900), `findTokenSeq` (lexer_test.go lines 174-188), and the `_ = fmt.Sprintf("test")` line (validator_test.go line 851). Remove the `fmt` import from validator_test.go if it becomes unused.

**Step 2: Verify**

Run: `go build ./...` — should compile.
Run: `make test` — should pass.
Run: `golangci-lint run ./... 2>&1 | grep -E "unused|S1039"` — should be empty.

**Step 3: Commit**

```bash
git add pkg/runner/runner.go pkg/parser/lexer_test.go pkg/validator/validator_test.go
git commit -m "chore: remove dead code (exprToJSON, findTokenSeq, unused fmt.Sprintf)"
```

---

### Task 4: Fix struct field alignment (govet)

**Files:**
- Multiple files flagged by govet fieldalignment — run `golangci-lint run ./... 2>&1 | grep fieldalignment` to find them all.

**Step 1: Reorder struct fields**

For each flagged struct, reorder fields from largest to smallest alignment (pointers/interfaces first, then int/float64, then strings, then bools). The govet output tells you the optimal order.

**Step 2: Verify**

Run: `golangci-lint run ./... 2>&1 | grep govet` — should be empty.
Run: `make test` — should pass (field reordering doesn't change behavior, but JSON tags ensure marshaling is stable).

**Step 3: Commit**

```bash
git add -A
git commit -m "perf: optimize struct field alignment (govet fieldalignment)"
```

---

### Task 5: Fix goconst, gocritic, staticcheck, revive issues

**Files:**
- Modify: `pkg/parser/parser.go` — extract `"map"` and `"enum"` to constants (goconst)
- Modify: `pkg/runner/runner.go:135` — rewrite if-else chain as switch (gocritic)
- Modify: `pkg/runner/runner.go:530` — extract `"error"` to constant (goconst)
- Modify: `pkg/adapter/process.go:91` — remove unnecessary nil check around range (staticcheck S1031)
- Modify: `pkg/adapter/process.go:130` — use `strings.TrimPrefix` (staticcheck S1017)
- Modify: files flagged by revive — fix early-return opportunities, unused receivers, etc.

**Step 1: Fix each issue**

Follow the lint output for each issue. For goconst, declare package-level constants. For gocritic, rewrite the if-else as a switch. For staticcheck, apply the suggested simplification. For revive, apply early returns and remove unused named receivers.

**Step 2: Verify**

Run: `golangci-lint run ./... 2>&1 | grep -E "goconst|gocritic|staticcheck|revive"` — should be empty or reduced to only the complexity-related revive issues (which are PR 1b).
Run: `make test` — should pass.

**Step 3: Commit**

```bash
git add -A
git commit -m "refactor: fix goconst, gocritic, staticcheck, and revive lint issues"
```

---

### Task 6: Address gosec and nolintlint issues

**Files:**
- Files flagged by gosec — evaluate each: if it's a false positive on internal CLI code, add `//nolint:gosec // G705: <reason>` with justification. If it's a real issue, fix it.
- Modify: `pkg/adapter/playwright.go` — add explanation to existing nolint directives (nolintlint)
- Modify: `pkg/runner/runner.go` — add explanation to existing nolint directives (nolintlint)

**Step 1: Evaluate each gosec finding**

The G705 XSS findings on `fmt.Fprintf(os.Stderr, ...)` with `os.Args` are false positives — this is a CLI tool writing to stderr, not a web response. Add `//nolint:gosec // G705: CLI tool writing to stderr, not a web response`.

For the nolintlint issues, every existing `//nolint` must have a linter name and explanation.

**Step 2: Fix nolint directives**

For each `//nolint` without explanation:
- If the underlying issue can be fixed, fix it and remove the nolint
- If it's genuinely unavoidable, add `//nolint:<linter> // <reason>`

**Step 3: Verify**

Run: `golangci-lint run ./... 2>&1 | grep -E "gosec|nolintlint"` — should be empty.
Run: `make test`

**Step 4: Commit**

```bash
git add -A
git commit -m "fix: address gosec false positives and add nolint explanations"
```

---

### Task 7: Address ireturn issue

**Files:**
- Modify: `pkg/openapi/models.go` — the `buildConstraint` function returns an interface

**Step 1: Evaluate the ireturn finding**

Read the function. If it returns `parser.Expr` (an interface), this is likely correct — the function constructs different expression types. Add `//nolint:ireturn // factory function returns different concrete Expr types` if it's genuinely a dispatch function.

**Step 2: Verify**

Run: `golangci-lint run ./... 2>&1 | grep ireturn` — should be empty.

**Step 3: Commit**

```bash
git add -A
git commit -m "fix: address ireturn lint finding in openapi models"
```

---

### Task 8: Create PR 1a

**Step 1: Run full verification**

```bash
make lint    # should show ONLY gocyclo + complexity-related revive (PR 1b scope)
make test    # all pass
```

**Step 2: Push and create PR**

```bash
git push -u origin <branch>
gh pr create --title "chore: fix auto-fixable lint issues, add Makefile, remove dead code" --body "..."
```

**Step 3: Wait for CI, squash merge**

```bash
gh pr checks <number> --watch
gh pr merge <number> --squash --delete-branch
```

---

## PR 1b: Complexity Refactoring + Error Handling + Token Enum

### Task 9: Fix token enum zero-value

**Files:**
- Modify: `pkg/parser/lexer.go:27-93`

**Step 1: Add TokenUnknown sentinel**

Insert `TokenUnknown TokenType = iota` as the first constant (line 29), before `TokenIdent`. This shifts `TokenIdent` to value 1. The zero-value of `TokenType` now means "uninitialized" rather than being a valid token.

```go
const (
	// Unknown/uninitialized token.
	TokenUnknown TokenType = iota

	// Literals.
	TokenIdent
	TokenInt
	// ... rest unchanged
)
```

Update `tokenNames` map to include `TokenUnknown: "UNKNOWN"`.

**Step 2: Check for zero-value assumptions**

Search codebase for `TokenType(0)`, `== 0`, or any code that assumes `TokenIdent` is zero. Also check if any code constructs `Token{}` and relies on the default type being `TokenIdent`.

**Step 3: Verify**

Run: `make test` — all tests should pass. If any fail, it means code was relying on the zero-value being TokenIdent — fix those.

**Step 4: Commit**

```bash
git add pkg/parser/lexer.go
git commit -m "refactor(parser): add TokenUnknown sentinel so zero-value means uninitialized"
```

---

### Task 10: Fix error handling (errcheck violations)

**Files:**
- Modify: `pkg/adapter/http.go:32` — handle `cookiejar.New` error
- Modify: `pkg/runner/runner.go:480` — handle `json.Marshal` error
- Modify: `pkg/adapter/http.go:80` and `pkg/adapter/process.go:85,96` — fix or justify nolint on best-effort JSON parse
- Modify: `pkg/parser/import_test.go:38,90,148` — use `require.NoError` or `t.Fatal` on `os.WriteFile`
- Modify: `pkg/runner/runner_test.go` — fix all errcheck violations (lines 145, 147, 362, 378, 447, 451, 456, 460)

**Step 1: Fix production code errcheck**

For `cookiejar.New`: handle the error explicitly (`if err != nil { return nil, fmt.Errorf("creating cookie jar: %w", err) }`).

For `json.Marshal` in runner.go:480: this marshals `[]string{url}` which cannot fail, but handle it anyway or use a `mustMarshalJSON` helper for test-like internal usage.

For best-effort JSON parse in adapters: these are intentional — stdout may not be JSON. If the nolint is kept, add full justification: `//nolint:errcheck // best-effort JSON parse: stdout may not be valid JSON, raw string is fallback`.

**Step 2: Fix test errcheck**

For import_test.go: wrap `os.WriteFile` calls with `if err != nil { t.Fatal(err) }`.

For runner_test.go: wrap `json.NewEncoder(w).Encode(...)` with error checks, or use a test helper. For `w.Write(...)`, check the error.

**Step 3: Verify**

Run: `golangci-lint run ./... 2>&1 | grep errcheck` — should be empty.
Run: `make test`

**Step 4: Commit**

```bash
git add -A
git commit -m "fix: handle all unchecked errors (errcheck)"
```

---

### Task 11: Refactor eval complexity

**Files:**
- Modify: `pkg/generator/generator.go`

**Step 1: Extract helper methods from eval (line 262)**

Extract each non-trivial case into its own method on `evalCtx`:

- `evalFieldRef(e *parser.FieldRef) (any, bool)` — extract from line 274
- `evalBinaryOp(e *parser.BinaryOp) (any, bool)` — extract from line 276
- `evalObjectLiteral(e *parser.ObjectLiteral) (any, bool)` — extract from line 278
- `evalArrayLiteral(e *parser.ArrayLiteral) (any, bool)` — extract from line 288
- `evalLenExpr(e *parser.LenExpr) (any, bool)` — extract from line 298
- `evalContainsExpr(e *parser.ContainsExpr) (any, bool)` — extract from line 317
- `evalExistsExpr(e *parser.ExistsExpr) (any, bool)` — extract from line 343
- `evalHasKeyExpr(e *parser.HasKeyExpr) (any, bool)` — extract from line 350
- `evalUnaryOp(e *parser.UnaryOp) (any, bool)` — extract from line 369
- `evalIfExpr(e *parser.IfExpr) (any, bool)` — extract from line 371

The `eval` method becomes a thin dispatcher:
```go
func (ctx *evalCtx) eval(expr parser.Expr) (any, bool) {
    switch e := expr.(type) {
    case *parser.LiteralInt:    return e.Value, true
    case *parser.LiteralFloat:  return e.Value, true
    case *parser.LiteralString: return e.Value, true
    case *parser.LiteralBool:   return e.Value, true
    case *parser.LiteralNull:   return nil, true
    case *parser.FieldRef:      return ctx.evalFieldRef(e)
    case *parser.BinaryOp:      return ctx.evalBinaryOp(e)
    case *parser.ObjectLiteral: return ctx.evalObjectLiteral(e)
    case *parser.ArrayLiteral:  return ctx.evalArrayLiteral(e)
    case *parser.LenExpr:       return ctx.evalLenExpr(e)
    case *parser.AllExpr:       return ctx.evalAll(e)
    case *parser.AnyExpr:       return ctx.evalAny(e)
    case *parser.ContainsExpr:  return ctx.evalContainsExpr(e)
    case *parser.ExistsExpr:    return ctx.evalExistsExpr(e)
    case *parser.HasKeyExpr:    return ctx.evalHasKeyExpr(e)
    case *parser.UnaryOp:       return ctx.evalUnaryOp(e)
    case *parser.IfExpr:        return ctx.evalIfExpr(e)
    default:                    return nil, false
    }
}
```

Note: `evalAll` and `evalAny` may already exist as separate methods — check before creating duplicates.

**Step 2: Verify**

Run: `make test` — all pass.
Run: `golangci-lint run ./... 2>&1 | grep "generator.go"` — gocyclo for `eval` should be gone.

**Step 3: Commit**

```bash
git add pkg/generator/generator.go
git commit -m "refactor(generator): extract eval helpers to reduce cyclomatic complexity"
```

---

### Task 12: Refactor checkExprType complexity

**Files:**
- Modify: `pkg/validator/validator.go`

**Step 1: Extract per-type check helpers**

Extract each case in `checkExprType` (line 170) into helpers:

- `checkIntExpr(expr parser.Expr, scope, scenario string) []error`
- `checkFloatExpr(expr parser.Expr, scope, scenario string) []error`
- `checkStringExpr(expr parser.Expr, scope, scenario string) []error`
- `checkBoolExpr(expr parser.Expr, scope, scenario string) []error`
- `checkEnumExpr(expr parser.Expr, te parser.TypeExpr, scope, scenario string) []error`
- `checkArrayExpr(expr parser.Expr, te parser.TypeExpr, models map[string]*parser.Model, scope, scenario string) []error`
- `checkModelExpr(expr parser.Expr, te parser.TypeExpr, models map[string]*parser.Model, scope, scenario string) []error`

The `checkExprType` method becomes a thin switch dispatching to these.

Also refactor `validateGivenBlock` (complexity 21) and `FormatErrors` (complexity 23) if flagged by revive.

**Step 2: Verify**

Run: `make test`
Run: `golangci-lint run ./... 2>&1 | grep "validator.go"` — complexity issues should be gone.

**Step 3: Commit**

```bash
git add pkg/validator/validator.go
git commit -m "refactor(validator): extract type-check helpers to reduce complexity"
```

---

### Task 13: Refactor parser complexity

**Files:**
- Modify: `pkg/parser/parser.go`

**Step 1: Extract helpers from parseAtom (line 1097)**

Extract built-in function dispatch into separate methods:
- `parseObjectLiteralExpr() (Expr, error)` — from TokenLBrace case
- `parseArrayLiteralExpr() (Expr, error)` — from TokenLBracket case
- `parseGroupedExpr() (Expr, error)` — from TokenLParen case
- `parseBuiltinCall(name string) (Expr, error)` — dispatches len/all/any/contains/exists/has_key

**Step 2: Extract helpers from parseTypeExprInner (line 494)**

- `parseArrayType() (TypeExpr, error)` — from TokenLBracket case
- `parseMapType(name Token) (TypeExpr, error)` — from `name.Value == "map"` case
- `parseEnumType() (TypeExpr, error)` — from `name.Value == "enum"` case

**Step 3: Verify**

Run: `make test`
Run: `golangci-lint run ./... 2>&1 | grep "parser.go"` — complexity issues should be gone.

**Step 4: Commit**

```bash
git add pkg/parser/parser.go
git commit -m "refactor(parser): extract atom and type helpers to reduce complexity"
```

---

### Task 14: Refactor runner complexity

**Files:**
- Modify: `pkg/runner/runner.go`

**Step 1: Extract helpers from runWhenScenario (line 385)**

This function has complexity 19. Extract:
- Page lifecycle (newPageWithNavigation / closePage) into a helper
- Input generation + execution loop into a helper
- Failure handling into a helper

Also refactor `generateValue` (complexity 16 in generator.go) if still flagged after Task 11.

**Step 2: Verify**

Run: `make test`
Run: `golangci-lint run ./... 2>&1 | grep gocyclo` — should be empty.

**Step 3: Commit**

```bash
git add pkg/runner/runner.go pkg/generator/generator.go
git commit -m "refactor(runner): extract when-scenario helpers to reduce complexity"
```

---

### Task 15: Rename Registry.Get to Registry.Adapter + defensive copy

**Files:**
- Modify: `pkg/plugin/plugin.go` — rename `Get` to `Adapter`
- Modify: all callers of `Registry.Get` (search with `grep -r "\.Get(" pkg/ cmd/`)
- Modify: `pkg/generator/generator.go:89-94` — return a copy of the slice

**Step 1: Rename Get → Adapter**

In `pkg/plugin/plugin.go`, rename `func (r *Registry) Get(name string)` to `func (r *Registry) Adapter(name string)`. Update all call sites.

**Step 2: Add defensive copy to ContractInput**

```go
func (g *Generator) ContractInput() []*parser.Field {
    if g.contract == nil {
        return nil
    }
    out := make([]*parser.Field, len(g.contract.Input))
    copy(out, g.contract.Input)
    return out
}
```

**Step 3: Verify**

Run: `make test`

**Step 4: Commit**

```bash
git add -A
git commit -m "refactor: rename Registry.Get to Adapter, add defensive slice copy"
```

---

### Task 16: Final lint check + create PR 1b

**Step 1: Verify zero lint issues**

Run: `make lint` — should pass cleanly (zero issues).
Run: `make test`

**Step 2: Push and create PR**

```bash
git push -u origin <branch>
gh pr create --title "refactor: reduce complexity, fix error handling, clean up token enum" --body "..."
```

**Step 3: Wait for CI, squash merge**

---

## PR 2: Repo Cleanup

### Task 17: Remove committed binary + fix gitignore

**Files:**
- Remove: `echo_tool` (binary at repo root)
- Modify: `.gitignore`

**Step 1: Remove binary and update gitignore**

```bash
git rm echo_tool
```

Add `/echo_tool` to `.gitignore` after the `/specrun` line.

**Step 2: Commit**

```bash
git add .gitignore
git commit -m "chore: remove committed echo_tool binary, add to gitignore"
```

---

### Task 18: Add t.Parallel() to stateless tests

**Files:**
- Modify: test files that don't use `t.Parallel()`:
  - `pkg/adapter/http_test.go`
  - `pkg/adapter/process_test.go`
  - `pkg/openapi/openapi_test.go`
  - `pkg/proto/proto_test.go`
  - `pkg/parser/include_test.go`
  - `pkg/parser/import_test.go`
  - `pkg/parser/given_test.go`
  - `pkg/parser/assertion_test.go`
  - `pkg/parser/float_test.go`
  - `pkg/parser/lexer_test.go`
  - `pkg/validator/validator_test.go`
  - `pkg/validator/enum_test.go`

**Step 1: Add t.Parallel()**

For each test function that doesn't share mutable state (no shared server, no global vars), add `t.Parallel()` as the first line. For table-driven tests, also add `t.Parallel()` in each `t.Run` subtest (with a local copy of the loop variable if needed).

Skip: `pkg/adapter/playwright_test.go` (browser tests may need serial execution), any test that starts an HTTP server on a fixed port.

**Step 2: Verify**

Run: `make test` — all pass (parallel execution may reveal hidden shared state — fix if found).

**Step 3: Commit**

```bash
git add -A
git commit -m "test: add t.Parallel() to stateless test functions"
```

---

### Task 19: Fix masked test setup errors

**Files:**
- Modify: `pkg/adapter/playwright_test.go:256-265`

**Step 1: Replace blank captures with error checks**

In the "failed login shows error" subtest, replace:
```go
gotoArgs, _ := json.Marshal(...)
resp, _ := adp.Action("goto", gotoArgs)
```

With:
```go
gotoArgs, err := json.Marshal(...)
require.NoError(t, err)
resp, err := adp.Action("goto", gotoArgs)
require.True(t, resp.OK, "setup action failed: %s", resp.Error)
```

Apply the same pattern to all action calls in test setup.

**Step 2: Verify**

Run: `make test`

**Step 3: Commit**

```bash
git add pkg/adapter/playwright_test.go
git commit -m "test: fix masked setup errors in playwright tests"
```

---

### Task 20: Create PR 2

Push, create PR, wait for CI, squash merge.

---

## PR 3: Documentation Rewrite

### Task 21: Create docs structure and move existing files

**Files:**
- Create: `docs/getting-started.md`
- Create: `docs/language-reference.md`
- Create: `docs/self-verification.md`
- Create: `docs/adapters/http.md`
- Create: `docs/adapters/process.md`
- Create: `docs/adapters/playwright.md`
- Move: `docs/openapi-import.md` → `docs/imports/openapi.md`
- Move: `docs/protobuf-import.md` → `docs/imports/protobuf.md`

**Step 1: Create directory structure**

```bash
mkdir -p docs/adapters docs/imports
git mv docs/openapi-import.md docs/imports/openapi.md
git mv docs/protobuf-import.md docs/imports/protobuf.md
```

**Step 2: Write getting-started.md**

Cover: install specrun, write your first spec (the transfer example simplified), run `specrun verify`, interpret output, next steps (links to language-reference, adapters).

**Step 3: Write language-reference.md**

Consolidate from CLAUDE.md and api_reference.md. Cover ALL features:
- Spec structure, include, import
- Types: int, string, bool, float, bytes, array (`[]T`), map (`map[K,V]`), enum, optional (`T?`), model references
- Expressions: all operators (+, -, *, /, %), comparisons, boolean, chained comparisons
- Built-in functions: `len()`, `contains()`, `exists()`, `has_key()`, `all()`, `any()`
- Conditional: `if/then/else`
- Models, contracts, scopes, config
- Scenarios (given/when/then), invariants (when guard)
- Assertion syntax: plain, `@plugin.property`, error pseudo-field
- Actions, locators

**Step 4: Write adapter docs**

`docs/adapters/http.md` — config, all actions (get/post/put/delete/header), all assertions (status/body/header.*/dot-path), multi-step given, array index access in dot-paths, examples.

`docs/adapters/process.md` — config, exec action, all assertions (exit_code/stdout/stderr/dot-path), array index access, examples.

`docs/adapters/playwright.md` — config, all actions, all assertions, locators, page lifecycle, examples.

**Step 5: Write self-verification.md**

Explain the self-verification concept, the performance spec pattern for shrinking, the current coverage (66 scenarios, 18 invariants), how to run it.

**Step 6: Update cross-references in import docs**

Update `docs/imports/openapi.md` and `docs/imports/protobuf.md` to use new relative paths for any cross-links.

**Step 7: Commit**

```bash
git add docs/
git commit -m "docs: create structured documentation (getting-started, language-reference, adapters, self-verification)"
```

---

### Task 22: Rewrite README.md

**Files:**
- Modify: `README.md`

**Step 1: Rewrite as concise landing page (~150 lines)**

Structure:
1. Title + one-line description
2. Problem statement (3-4 sentences on anti-gaming)
3. Install (go install, from source)
4. Quick example (trimmed transfer spec, ~20 lines)
5. Run verification + sample output
6. Documentation links table (getting-started, language-reference, adapters, imports)
7. Claude Code plugin section (brief)
8. Self-verification mention + link

**Step 2: Verify all doc links work**

Check each relative link resolves to an existing file.

**Step 3: Commit**

```bash
git add README.md
git commit -m "docs: rewrite README as concise landing page with doc links"
```

---

### Task 23: Trim CLAUDE.md and update skills

**Files:**
- Modify: `CLAUDE.md` — remove duplicated reference content, point to docs/
- Modify: `skills/author/references/api_reference.md` — ensure consistency with language-reference.md
- Modify: `skills/author/SKILL.md` — fix `npx playwright install chromium` → `specrun install playwright`, ensure all features mentioned
- Modify: `skills/verify/SKILL.md` — review for accuracy

**Step 1: Trim CLAUDE.md**

Keep: problem statement, settled decisions summary, project structure, commands, architecture diagram, self-verification section, plugin section.
Remove: detailed syntax that now lives in `docs/language-reference.md`. Replace with a pointer: "See `docs/language-reference.md` for complete syntax reference."
Ensure all recently added features are at least mentioned (enum, contains, exists, has_key, all/any, if/then/else, div/mod, array index access, error pseudo-field).

**Step 2: Update skills**

In `skills/author/SKILL.md`: fix `npx playwright install chromium` → `specrun install playwright`. Verify all language features are mentioned in the authoring checklist.

In `skills/author/references/api_reference.md`: ensure it references docs/ for full details or is kept in sync.

**Step 3: Verify**

Run self-verification to ensure nothing broke:
```bash
make build
# start servers...
SPECRUN_BIN=./specrun APP_URL=... BROKEN_APP_URL=... HTTP_TEST_URL=... ECHO_TOOL_BIN=./echo_tool ./specrun verify specs/speclang.spec
```

**Step 4: Commit**

```bash
git add CLAUDE.md skills/
git commit -m "docs: trim CLAUDE.md, update skills for consistency"
```

---

### Task 24: Create PR 3

Push, create PR, wait for CI, squash merge.
