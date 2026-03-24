# Repo Organization & Go Idiom Cleanup Design

**Date:** 2026-03-24
**Status:** Approved

## Goal

Make the speclang repo approachable to new users and idiomatic for Go developers. Fix all lint violations, reorganize documentation, rewrite README as a concise landing page, and create structured docs.

## Execution Order

### PR 1a: Auto-fixable lint + Makefile + dead code
- Add Makefile with `lint`, `fmt`, `test`, `build` targets
- Fix import ordering (3 gci issues)
- Fix formatting (3 golines issues)
- Fix goconst suggestions (extract repeated strings)
- Fix struct field alignment (11 govet issues)
- Fix revive issues (early returns, unused receivers, etc.)
- Fix staticcheck issues (unnecessary nil check, strings.TrimPrefix)
- Remove dead code: `exprToJSON` (runner.go), `findTokenSeq` (lexer_test.go), `_ = fmt.Sprintf` hack (validator_test.go)

### PR 1b: Complexity refactoring + error handling + token enum
- Token enum: add `TokenUnknown = iota` sentinel, shift all types
- Error handling: fix all 15 errcheck violations, remove blank captures, address 3 nolint directives
- Complexity refactoring:
  - `(*evalCtx).eval` (38) → extract per-type eval helpers
  - `(*validator).checkExprType` (32) → extract per-type check helpers
  - `(*parser).parseAtom` (23) → extract literal/function parsers
  - `(*parser).parseTypeExprInner` (20) → extract array/map/enum type parsers
- `ContractInput()` returns a defensive copy
- `Registry.Get()` → rename to `Registry.Adapter()`

### PR 2: Repo cleanup
- `git rm echo_tool`, add `/echo_tool` to `.gitignore`
- Add `t.Parallel()` to all stateless test functions
- Fix masked test setup errors in playwright_test.go
- Standardize blackbox test packages where appropriate

### PR 3: Documentation rewrite
- Rewrite README.md as concise landing page (~150 lines):
  - Problem statement, one spec example, install, quick verify, links to docs/
- Create docs structure:
  ```
  docs/
  ├── getting-started.md
  ├── language-reference.md
  ├── adapters/
  │   ├── http.md
  │   ├── process.md
  │   └── playwright.md
  ├── imports/
  │   ├── openapi.md          # moved from docs/openapi-import.md
  │   └── protobuf.md         # moved from docs/protobuf-import.md
  ├── self-verification.md
  ├── plans/
  └── research/
  ```
- Trim CLAUDE.md: remove duplicated reference content, point to docs/language-reference.md
- Update skills: api_reference.md, SKILL.md consistency, fix playwright install command
- Ensure all recent features documented (enum, contains, exists, has_key, all/any, if/then/else, div/mod, array index access, error pseudo-field)

## Design Decisions

- **Docs in-repo (`docs/`)** not GitHub Wiki — versioned with code, allows future static site generation
- **Specs stay flat** in `specs/` — naming convention over subdirectories
- **Keep `main() → runX() int` pattern** — works, not worth refactoring
- **Never nolint** unless provably unavoidable — fix the underlying issue
- **CI builds echo_tool** (explicit step) — idiomatic for Go, Go test helper remains as local dev convenience
