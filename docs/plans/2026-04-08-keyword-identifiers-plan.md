# Keyword Identifiers Implementation Plan (#113)

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Allow block-level keywords (`before`, `after`, `when`, `contract`, `invariant`, `scenario`) to be used as identifiers in name positions — specifically as action parameter names, model/action/scenario/invariant names, and object literal keys — so specs can mirror real-world API field names like cursor-pagination's `before`/`after`.

**Architecture:** The parser already has a two-tier token-acceptance scheme: `p.expect(TokenIdent)` for the strict case and `p.expectIdent()` (line 131) which accepts any token in `isIdentLike()` (line 113). Contract input/output field parsing (`parseField` line 722) and many other sites already use `expectIdent()` — this fix extends that pattern to the remaining name-position sites, primarily `parseParam` (line 1076) which is the reporter's exact repro, and broadens `isIdentLike` to cover the keywords the reporter called out.

**Tech Stack:** Go 1.x, `internal/parser` package. No new dependencies. Tests use the existing `Parse(source string)` entry point and `testing.T`.

---

## Scope Audit (determined during planning)

**Primary bug site:**
- `parseParam` at `internal/parser/parser.go:1076` uses `p.expect(TokenIdent)`. This is what the reporter hits.

**isIdentLike currently accepts** (line 113): `Ident, Input, Output, Model, Action, Target, Locators, Given, Then, Scope, Config, Before, After, Let, Return`.

**isIdentLike is missing** (per issue #113's "same is likely true for..." list): `Contract, Invariant, Scenario, When`.

**Other `p.expect(TokenIdent)` sites that should become `expectIdent()`** (all are user-named identifier positions):
- `154` — spec name (`spec Name {}`)
- `316` — service block key (`services { name: {...} }`)
- `342` — target block field key (v2 compat, but consistent)
- `378` — adapter config block field key
- `648` — locators block key (v2 compat, but consistent)
- `694` — `parseModel` name
- `930` — `parseAction` name
- `1027` — adapter method name in `adapter.method(...)` call
- `1076` — `parseParam` name (PRIMARY BUG)
- `1093, 1101` — `parseCall` (v2 legacy, keep consistent)
- `1134` — `parseInvariant` name
- `1176` — `parseScenario` name
- `1841` — object literal key

**Intentionally NOT changed:**
- `1797` — `env(VAR)` env-var name. Env var names are `SHOUTY_SNAKE` convention and don't collide with keywords; scope creep.
- The `parseSpecMember` / `parseScopeMember` dispatch sites. These intentionally switch on specific keyword tokens at statement-start position to decide *which kind of block* to parse. Leaving them alone preserves keyword behavior at statement start — the contextual-keyword trick works because dispatch happens before `isIdentLike` is consulted.

---

## Task 1: Add failing test for the reporter's exact repro

**Files:**
- Create: `internal/parser/param_keyword_test.go`

**Step 1: Write the failing test**

```go
package parser

import (
	"testing"
)

// TestParseAction_BeforeAsParamName is the exact reproducer for issue #113.
// `before` is a natural cursor-pagination parameter name but was rejected by
// parseParam's strict TokenIdent check.
func TestParseAction_BeforeAsParamName(t *testing.T) {
	t.Parallel()
	spec, err := Parse(`
spec Test {
  scope session_history {
    action session_history(limit: int?, before: string?) {
      let result = http.get("/api/v1/sessions/history")
      return result
    }

    contract {
      input {
        limit: int?
        before: string?
      }
      output {
        sessions: string?
        error: string?
      }
      action: session_history
    }
  }
}
`)
	if err != nil {
		t.Fatalf("expected parse to succeed, got: %v", err)
	}
	if len(spec.Scopes) != 1 {
		t.Fatalf("expected 1 scope, got %d", len(spec.Scopes))
	}
	if len(spec.Scopes[0].Actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(spec.Scopes[0].Actions))
	}
	action := spec.Scopes[0].Actions[0]
	if len(action.Params) != 2 {
		t.Fatalf("expected 2 params, got %d", len(action.Params))
	}
	if action.Params[0].Name != "limit" {
		t.Errorf("expected first param 'limit', got %q", action.Params[0].Name)
	}
	if action.Params[1].Name != "before" {
		t.Errorf("expected second param 'before', got %q", action.Params[1].Name)
	}
}
```

**Note:** This test assumes `Scope.Actions` is the field holding scope-level action definitions. Verify by grepping `pkg/spec/ast.go` for `Scope struct` before implementing — if the field is named differently (e.g., `ActionDefs`), fix the test accordingly. Do not guess.

**Step 2: Run test to verify it fails**

```bash
go test ./internal/parser/ -run TestParseAction_BeforeAsParamName -v
```

Expected: FAIL with an error containing `expected Ident, got Before ("before")` — this is the current `parseParam` error.

**Step 3: Commit the failing test**

```bash
git add internal/parser/param_keyword_test.go
git commit -m "test(parser): add failing test for 'before' as action param name (#113)"
```

---

## Task 2: Fix the primary bug — parseParam

**Files:**
- Modify: `internal/parser/parser.go:1076`

**Step 1: Change the call**

Replace:
```go
func (p *parser) parseParam() (*Param, error) {
	name, err := p.expect(TokenIdent)
```
with:
```go
func (p *parser) parseParam() (*Param, error) {
	name, err := p.expectIdent()
```

**Step 2: Run the Task 1 test**

```bash
go test ./internal/parser/ -run TestParseAction_BeforeAsParamName -v
```

Expected: PASS.

**Step 3: Run the full parser test suite to catch regressions**

```bash
go test ./internal/parser/ -v
```

Expected: all existing tests still pass.

**Step 4: Commit**

```bash
git add internal/parser/parser.go
git commit -m "fix(parser): accept keyword tokens as action parameter names (#113)"
```

---

## Task 3: Extend isIdentLike to cover Contract, Invariant, Scenario, When

**Files:**
- Modify: `internal/parser/parser.go:113-127`

**Step 1: Write the failing test first**

Append to `internal/parser/param_keyword_test.go`:

```go
// TestParseAction_MoreKeywordsAsParamNames covers the "same is likely true for
// other scope-block keywords" list from issue #113: contract, invariant,
// scenario, when, after — in addition to before which Task 1 covers.
func TestParseAction_MoreKeywordsAsParamNames(t *testing.T) {
	t.Parallel()
	cases := []string{"after", "contract", "invariant", "scenario", "when"}
	for _, kw := range cases {
		kw := kw
		t.Run(kw, func(t *testing.T) {
			t.Parallel()
			src := `
spec Test {
  scope s {
    action foo(` + kw + `: string) {
      return ` + kw + `
    }
  }
}
`
			spec, err := Parse(src)
			if err != nil {
				t.Fatalf("expected parse to succeed for param name %q, got: %v", kw, err)
			}
			got := spec.Scopes[0].Actions[0].Params[0].Name
			if got != kw {
				t.Errorf("expected param name %q, got %q", kw, got)
			}
		})
	}
}
```

**Step 2: Run it to verify `after` passes but the rest fail**

```bash
go test ./internal/parser/ -run TestParseAction_MoreKeywordsAsParamNames -v
```

Expected: the `after` subtest passes (already in `isIdentLike`); `contract`, `invariant`, `scenario`, `when` subtests FAIL with `expected identifier, got ...`.

**Step 3: Add the missing tokens to isIdentLike**

Change `parser.go:113`:
```go
func isIdentLike(typ TokenType) bool {
	switch typ {
	case TokenIdent,
		TokenInput, TokenOutput,
		TokenModel, TokenAction,
		TokenTarget, TokenLocators,
		TokenGiven, TokenThen,
		TokenScope, TokenConfig,
		TokenBefore, TokenAfter,
		TokenLet, TokenReturn:
		return true
	default:
		return false
	}
}
```
to:
```go
func isIdentLike(typ TokenType) bool {
	switch typ {
	case TokenIdent,
		TokenInput, TokenOutput,
		TokenModel, TokenAction,
		TokenTarget, TokenLocators,
		TokenGiven, TokenWhen, TokenThen,
		TokenScope, TokenConfig,
		TokenContract, TokenInvariant, TokenScenario,
		TokenBefore, TokenAfter,
		TokenLet, TokenReturn:
		return true
	default:
		return false
	}
}
```

**Step 4: Re-run the test**

```bash
go test ./internal/parser/ -run TestParseAction_MoreKeywordsAsParamNames -v
```

Expected: all five subtests pass.

**Step 5: Run the full parser test suite to verify no regressions from the broadened keyword acceptance**

```bash
go test ./internal/parser/ -v
```

Expected: all tests pass. If any fail, the most likely cause is a test that was relying on a keyword *being rejected* as an identifier — re-examine before adjusting.

**Step 6: Commit**

```bash
git add internal/parser/parser.go internal/parser/param_keyword_test.go
git commit -m "feat(parser): allow contract/invariant/scenario/when as identifiers (#113)"
```

---

## Task 4: Sweep remaining `expect(TokenIdent)` sites for name positions

**Files:**
- Modify: `internal/parser/parser.go` — lines 154, 316, 342, 378, 648, 694, 930, 1027, 1093, 1101, 1134, 1176, 1841

**Step 1: Write a sweep test covering the less-common but still-valid positions**

Append to `internal/parser/param_keyword_test.go`:

```go
// TestKeywordsInNamePositions confirms keywords can appear in every
// identifier-like name position in a spec, not just action parameters.
func TestKeywordsInNamePositions(t *testing.T) {
	t.Parallel()

	t.Run("model name", func(t *testing.T) {
		t.Parallel()
		_, err := Parse(`spec T { model before { x: int } }`)
		if err != nil {
			t.Fatalf("model named 'before' should parse: %v", err)
		}
	})

	t.Run("scenario name", func(t *testing.T) {
		t.Parallel()
		_, err := Parse(`
spec T {
  scope s {
    contract { input { x: int } output { y: int } }
    scenario before {
      given { x: 1 }
      then { y == 1 }
    }
  }
}
`)
		if err != nil {
			t.Fatalf("scenario named 'before' should parse: %v", err)
		}
	})

	t.Run("invariant name", func(t *testing.T) {
		t.Parallel()
		_, err := Parse(`
spec T {
  scope s {
    contract { input { x: int } output { y: int } action: foo }
    action foo(x: int) { return x }
    invariant before {
      then { y == x }
    }
  }
}
`)
		if err != nil {
			t.Fatalf("invariant named 'before' should parse: %v", err)
		}
	})

	t.Run("object literal key", func(t *testing.T) {
		t.Parallel()
		_, err := Parse(`
spec T {
  scope s {
    scenario smoke {
      given {
        http.post("/x", { before: "2026-01-01", after: "2026-12-31" })
      }
      then { true == true }
    }
  }
}
`)
		if err != nil {
			t.Fatalf("object literal key 'before' should parse: %v", err)
		}
	})
}
```

**Step 2: Run the sweep test to see which positions fail**

```bash
go test ./internal/parser/ -run TestKeywordsInNamePositions -v
```

Expected: some subtests fail. Record which ones.

**Step 3: For each failing subtest, find the responsible site and convert it**

For each site in the audit list above (lines 154, 316, 342, 378, 648, 694, 930, 1027, 1093, 1101, 1134, 1176, 1841), change `p.expect(TokenIdent)` to `p.expectIdent()`. The call signatures are identical — both return `(Token, error)`.

Do this as a single mechanical pass. Review each one to confirm the call is in a name/identifier position (not, e.g., a type name where you want strictness).

**Step 4: Re-run the sweep test**

```bash
go test ./internal/parser/ -run TestKeywordsInNamePositions -v
```

Expected: all subtests pass.

**Step 5: Run the full parser test suite**

```bash
go test ./internal/parser/ -v
```

Expected: all pass.

**Step 6: Run the full repo test suite to catch anything downstream**

```bash
go test ./...
```

Expected: all pass.

**Step 7: Commit**

```bash
git add internal/parser/parser.go internal/parser/param_keyword_test.go
git commit -m "refactor(parser): accept keywords in all identifier-name positions (#113)"
```

---

## Task 5: Self-verification

**Step 1: Build and run self-verification**

```bash
go build -o specrun ./cmd/specrun
SPECRUN_BIN=./specrun ./specrun verify specs/speclang.spec
```

Expected: all scopes pass. Note: this requires Docker for service-backed scopes; if Docker is unavailable, scope it down to parse-related scopes with `--scope parse_valid` etc. (verify the flag exists first).

**Step 2: Commit the built binary only if the repo tracks it** (it doesn't — `specrun` is gitignored per project convention; skip this step).

---

## Task 6: Update language reference docs

**Files:**
- Modify: `docs/language-reference.md`

**Step 1: Find the reserved-words section, if it exists**

```bash
grep -n -i "reserved\|keyword" docs/language-reference.md
```

**Step 2: Add or update a note**

If there's a reserved-word list, add a note that block keywords (`before`, `after`, `when`, `contract`, `invariant`, `scenario`, etc.) are **contextual** — reserved only at statement-start positions inside a scope, and usable as identifiers in name positions (action parameters, field names, model/action/scenario/invariant names, object literal keys).

If there is no reserved-words section, add a short subsection under "Lexical rules" or equivalent:

```markdown
### Reserved keywords

Block keywords (`spec`, `scope`, `model`, `action`, `contract`, `input`,
`output`, `scenario`, `invariant`, `given`, `when`, `then`, `before`, `after`,
`target`, `config`, `use`, `let`, `return`, `if`, `else`, `include`, `import`,
`service`) are reserved **only at statement-start positions**. They can be
used as identifiers anywhere a name is expected — action parameters, field
names, model/action/scenario/invariant names, and object literal keys. For
example, cursor pagination with `before`/`after` query parameters works:

    action list_items(before: string?, after: string?) {
      return http.get("/items")
    }
```

**Step 3: Commit**

```bash
git add docs/language-reference.md
git commit -m "docs: clarify that block keywords are contextual identifiers (#113)"
```

---

## Task 7: Open PR

**Step 1: Push branch**

```bash
git push -u origin fix/113-keyword-identifiers
```

**Step 2: Create PR**

```bash
gh pr create --title "fix(parser): accept block keywords as identifiers (#113)" --body "$(cat <<'EOF'
## Summary
- Fixes #113: `before` (and friends) can now be used as action parameter names.
- Broadens `isIdentLike` to include `contract`, `invariant`, `scenario`, `when`.
- Sweeps all identifier-name positions in the parser to use `expectIdent()` instead of the strict `expect(TokenIdent)`, making block keywords contextual — reserved only at statement-start, usable as names elsewhere.

## Test plan
- [ ] `go test ./internal/parser/` passes, including the new `TestParseAction_BeforeAsParamName`, `TestParseAction_MoreKeywordsAsParamNames`, and `TestKeywordsInNamePositions` tests.
- [ ] `go test ./...` passes.
- [ ] `SPECRUN_BIN=./specrun ./specrun verify specs/speclang.spec` passes (or the parse-only subset if Docker is unavailable).
EOF
)"
```

Expected: PR URL printed.
