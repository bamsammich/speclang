# Package Reorganization Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Reorganize speclang into `pkg/spec/` (public API) and `internal/` (implementation) so external consumers can import a clean, idiomatic Go package.

**Architecture:** Move AST types to `pkg/spec/` first (it's the foundation everything depends on), then move implementation packages to `internal/`, then create top-level convenience functions. The project must compile and all tests must pass after every task. Self-verification specs are NOT modified — they are the behavioral regression gate.

**Tech Stack:** Go, no new dependencies

---

## CRITICAL RULES

- **NEVER modify any .spec files.** They are the behavioral regression gate.
- **The project must compile after every task.** No "fix it later" commits.
- **All tests must pass after every task.** Run `go test ./... -count=1`.
- **golangci-lint must stay at zero issues.** Run `golangci-lint run ./...`.
- **No AI co-author on commits.**

---

## Phase 1: Create `pkg/spec/` with public types

### Task 1: Create `pkg/spec/ast.go` — move all AST types

**Files:**
- Create: `pkg/spec/ast.go`
- Modify: `pkg/parser/ast.go` — replace type definitions with type aliases pointing to `pkg/spec`

**Step 1: Create `pkg/spec/ast.go`**

Copy all exported types from `pkg/parser/ast.go` to `pkg/spec/ast.go`. This includes:
- `Spec`, `Scope`, `Service`, `Target`, `Model`, `Field`, `TypeExpr`, `Contract`, `Action`, `Param`, `Call`, `Invariant`, `Scenario`, `Block`, `Assertion`, `Assignment`
- `GivenStep` interface
- `Expr` interface and ALL concrete implementations (LiteralInt, LiteralFloat, LiteralString, LiteralBool, LiteralNull, FieldRef, EnvRef, ServiceRef, BinaryOp, UnaryOp, ObjectLiteral, ObjField, ArrayLiteral, LenExpr, AllExpr, AnyExpr, ContainsExpr, ExistsExpr, HasKeyExpr, RegexLiteral, IfExpr)

Package declaration: `package spec`

**Step 2: Make `pkg/parser/ast.go` re-export from `pkg/spec`**

Replace `pkg/parser/ast.go` with type aliases:
```go
package parser

import "github.com/bamsammich/speclang/v2/pkg/spec"

type Spec = spec.Spec
type Scope = spec.Scope
type Model = spec.Model
// ... every exported type gets an alias
type Expr = spec.Expr
type LiteralInt = spec.LiteralInt
// etc.
```

This maintains backward compatibility — everything that imports `parser.Spec` still works, but the canonical definition lives in `pkg/spec`.

**Step 3: Verify**

```bash
go build ./...
go test ./... -count=1
golangci-lint run ./...
```

ALL must pass. Every existing import of `parser.Spec` etc. still works via aliases.

**Step 4: Commit**

```bash
git add pkg/spec/ast.go pkg/parser/ast.go
git commit -m "refactor: move AST types to pkg/spec, alias from parser"
```

---

### Task 2: Create `pkg/spec/adapter.go` — move Adapter interface

**Files:**
- Create: `pkg/spec/adapter.go`
- Modify: `pkg/adapter/protocol.go` — replace with aliases

**Step 1: Create `pkg/spec/adapter.go`**

Move from `pkg/adapter/protocol.go`:
- `Adapter` interface
- `Request` struct
- `Response` struct

**Step 2: Alias from `pkg/adapter/protocol.go`**

```go
package adapter

import "github.com/bamsammich/speclang/v2/pkg/spec"

type Adapter = spec.Adapter
type Request = spec.Request
type Response = spec.Response
```

**Step 3: Verify** (build + test + lint)

**Step 4: Commit**

```bash
git commit -m "refactor: move Adapter interface to pkg/spec, alias from adapter"
```

---

### Task 3: Create `pkg/spec/result.go` — move Result types

**Files:**
- Create: `pkg/spec/result.go`
- Modify: `pkg/runner/runner.go` — replace Result type definitions with aliases

**Step 1: Create `pkg/spec/result.go`**

Move from `pkg/runner/runner.go`:
- `Result`
- `Failure`
- `ScopeResult`
- `CheckResult`

**Step 2: Alias from `pkg/runner/runner.go`**

Add at the top of runner.go:
```go
import "github.com/bamsammich/speclang/v2/pkg/spec"

type Result = spec.Result
type Failure = spec.Failure
type ScopeResult = spec.ScopeResult
type CheckResult = spec.CheckResult
```

Remove the original struct definitions.

**Step 3: Verify** (build + test + lint)

**Step 4: Commit**

```bash
git commit -m "refactor: move Result types to pkg/spec, alias from runner"
```

---

### Task 4: Create `pkg/spec/import.go` — move ImportResolver

**Files:**
- Create: `pkg/spec/import.go`
- Modify: `pkg/parser/import.go` — alias the types

**Step 1: Create `pkg/spec/import.go`**

Move from `pkg/parser/import.go`:
- `ImportResolver` interface
- `ImportRegistry` type

Note: `ImportResolver` references `*Model` and `*Scope` — these are now in `pkg/spec`, so the interface definition is self-contained.

**Step 2: Alias from `pkg/parser/import.go`**

```go
type ImportResolver = spec.ImportResolver
type ImportRegistry = spec.ImportRegistry
```

**Step 3: Verify** (build + test + lint)

**Step 4: Commit**

```bash
git commit -m "refactor: move ImportResolver to pkg/spec, alias from parser"
```

---

### Task 5: Create `pkg/spec/registry.go` — plugin registry

**Files:**
- Create: `pkg/spec/registry.go`

**Step 1: Implement the registry**

```go
package spec

// PluginDef declares a plugin's schema and implementation.
type PluginDef struct {
    Actions    map[string]ActionDef
    Assertions map[string]AssertionDef
    Adapter    Adapter
}

// ActionDef describes an action's parameter signature.
type ActionDef struct {
    Params []Param
}

// AssertionDef describes an assertion property's type.
type AssertionDef struct {
    Type string
}

// Registry holds named plugin definitions.
type Registry struct {
    plugins map[string]PluginDef
}

// NewRegistry creates an empty registry.
func NewRegistry() *Registry {
    return &Registry{plugins: make(map[string]PluginDef)}
}

// Register adds a plugin by name.
func (r *Registry) Register(name string, def PluginDef) {
    r.plugins[name] = def
}

// Adapter returns the adapter for a named plugin.
func (r *Registry) Adapter(name string) (Adapter, error) {
    def, ok := r.plugins[name]
    if !ok {
        return nil, fmt.Errorf("plugin %q not registered", name)
    }
    return def.Adapter, nil
}

// Plugin returns the full definition for a named plugin.
func (r *Registry) Plugin(name string) (PluginDef, bool) {
    def, ok := r.plugins[name]
    return def, ok
}

// Plugins returns all registered plugin names.
func (r *Registry) Plugins() []string {
    names := make([]string, 0, len(r.plugins))
    for name := range r.plugins {
        names = append(names, name)
    }
    sort.Strings(names)
    return names
}
```

`DefaultRegistry()` will be added in Phase 3 (after adapter implementations move to internal/).

**Step 2: Write tests**

Create `pkg/spec/registry_test.go`:
- Test Register + Adapter lookup
- Test unknown plugin returns error
- Test Plugins() returns sorted names

**Step 3: Verify** (build + test + lint)

**Step 4: Commit**

```bash
git commit -m "feat(spec): add plugin registry with schema and adapter registration"
```

---

## Phase 2: Move implementation packages to `internal/`

### Task 6: Move `pkg/parser/` → `internal/parser/`

**Files:**
- Move: `pkg/parser/*.go` → `internal/parser/*.go` (except ast.go which is now aliases)
- Update: all files that import `pkg/parser` → `internal/parser`

**Step 1: Create `internal/parser/`**

```bash
mkdir -p internal/parser
```

Move all files EXCEPT `ast.go` (which has aliases — it can stay or be deleted once all consumers import `pkg/spec` directly):
- `parser.go`, `lexer.go`, `include.go`, `import.go` (logic parts)
- All `*_test.go` files

**Step 2: Update the package import path**

In `internal/parser/`, all files already say `package parser`. The import path changes from `github.com/bamsammich/speclang/v2/pkg/parser` to `github.com/bamsammich/speclang/v2/internal/parser`.

Update these files to use the new import path:
- `cmd/specrun/main.go`
- `internal/parser/*.go` (self-references if any)
- Any file that imports `pkg/parser` for the `Parse`/`ParseFile` functions

But wait — `pkg/generator`, `pkg/runner`, `pkg/validator`, `pkg/openapi`, `pkg/proto` all import `pkg/parser` for types. Since types are aliased from `pkg/spec`, these can either:
- Keep importing `pkg/parser` (aliases still work)
- Switch to importing `pkg/spec` directly

The cleanest approach: switch ALL type imports to `pkg/spec`, keep `internal/parser` imports only for the `Parse`/`ParseFile` functions.

**IMPORTANT:** This is the trickiest task. Do it carefully:
1. First update all type references from `parser.Spec` to `spec.Spec` across the codebase
2. Then move the parser package to internal/
3. Then delete the `pkg/parser/` alias files

Actually, a safer approach: move parser to internal/ but KEEP `pkg/parser/ast.go` as aliases temporarily. Delete the aliases only after all consumers are updated.

**Step 3: Verify** (build + test + lint)

**Step 4: Commit**

```bash
git commit -m "refactor: move parser implementation to internal/parser"
```

---

### Task 7: Move `pkg/adapter/` → `internal/adapter/`

**Files:**
- Move: `pkg/adapter/http.go`, `process.go`, `playwright.go` and their tests → `internal/adapter/`
- Keep: `pkg/adapter/protocol.go` as aliases (or delete if all consumers import `pkg/spec`)

**Step 1: Move implementation files**

Move `http.go`, `process.go`, `playwright.go`, `http_test.go`, `process_test.go`, `playwright_test.go` to `internal/adapter/`.

**Step 2: Update imports**

The adapter implementations reference `Adapter`, `Response`, `Request` — these are now in `pkg/spec`. Update imports in the moved files.

**Step 3: Verify** (build + test + lint)

**Step 4: Commit**

```bash
git commit -m "refactor: move adapter implementations to internal/adapter"
```

---

### Task 8: Move remaining packages to `internal/`

Move each in sequence, updating imports after each:

**Step 1: Move `pkg/generator/` → `internal/generator/`**

All files. Update imports: `parser.*` types → `spec.*`.

**Step 2: Move `pkg/runner/` → `internal/runner/`**

All files except the Result type definitions (already aliased). Update imports.

**Step 3: Move `pkg/validator/` → `internal/validator/`**

All files. Update imports.

**Step 4: Move `pkg/infra/` → `internal/infra/`**

All files. No type changes needed (infra doesn't import parser types).

**Step 5: Move `pkg/openapi/` → `internal/openapi/`**

All files. Update imports: `parser.*` types → `spec.*`.

**Step 6: Move `pkg/proto/` → `internal/proto/`**

All files. Update imports: `parser.*` types → `spec.*`.

**Step 7: Delete `pkg/plugin/`**

It's unused. The registry in `pkg/spec/registry.go` replaces it.

**Step 8: Verify** (build + test + lint)

**Step 9: Commit one per package or batch**

```bash
git commit -m "refactor: move generator, runner, validator, infra, openapi, proto to internal/"
```

---

### Task 9: Clean up alias files

**Files:**
- Delete: `pkg/parser/ast.go` (aliases)
- Delete: `pkg/adapter/protocol.go` (aliases)
- Delete: remaining `pkg/parser/`, `pkg/adapter/`, `pkg/runner/` directories

All consumers should now import from `pkg/spec` for types and `internal/*` for implementation. The alias files are no longer needed.

**Step 1: Delete alias files and empty directories**

**Step 2: Verify** (build + test + lint)

**Step 3: Commit**

```bash
git commit -m "chore: remove type alias bridge files, clean up empty pkg/ directories"
```

---

## Phase 3: Create top-level convenience functions

### Task 10: Create `pkg/spec/spec.go` — Verify, Generate, Parse, ParseFile

**Files:**
- Create: `pkg/spec/spec.go`

**Step 1: Implement top-level functions**

```go
package spec

import (
    "context"

    "github.com/bamsammich/speclang/v2/internal/parser"
    "github.com/bamsammich/speclang/v2/internal/generator"
    "github.com/bamsammich/speclang/v2/internal/runner"
    "github.com/bamsammich/speclang/v2/internal/validator"
)

// Options configures verification behavior.
type Options struct {
    Seed       uint64
    Iterations int
}

// Parse parses a spec from source text.
func Parse(source string) (*Spec, error) {
    return parser.Parse(source)
}

// ParseFile parses a spec from a file, resolving includes and imports.
func ParseFile(path string, imports ImportRegistry) (*Spec, error) {
    return parser.ParseFileWithImports(path, imports)
}

// Validate checks a spec for semantic errors.
func Validate(s *Spec) []error {
    return validator.Validate(s)
}

// Verify runs the full verification pipeline.
// If registry is nil, DefaultRegistry() is used.
func Verify(ctx context.Context, s *Spec, registry *Registry, opts Options) (*Result, error) {
    if registry == nil {
        registry = DefaultRegistry()
    }

    // Build adapter map from registry based on plugins used in spec
    adapters := make(map[string]Adapter)
    for _, scope := range s.Scopes {
        if scope.Use == "" {
            continue
        }
        if _, exists := adapters[scope.Use]; exists {
            continue
        }
        adp, err := registry.Adapter(scope.Use)
        if err != nil {
            return nil, err
        }
        adapters[scope.Use] = adp
    }

    r := runner.New(s, adapters, opts.Seed)
    if opts.Iterations > 0 {
        r.SetN(opts.Iterations)
    }
    return r.Verify()
}

// Generate produces one random input for a named scope.
func Generate(s *Spec, scopeName string, seed uint64) (map[string]any, error) {
    for _, scope := range s.Scopes {
        if scope.Name == scopeName {
            if scope.Contract == nil {
                return nil, fmt.Errorf("scope %q has no contract", scopeName)
            }
            models := make(map[string]*Model)
            for _, m := range s.Models {
                models[m.Name] = m
            }
            g := generator.New(scope.Contract, s.Models, seed)
            return g.GenerateInput()
        }
    }
    return nil, fmt.Errorf("scope %q not found", scopeName)
}
```

**Step 2: Implement DefaultRegistry**

Add to `pkg/spec/registry.go`:
```go
// DefaultRegistry returns a new registry with built-in adapters (http, process, playwright).
func DefaultRegistry() *Registry {
    r := NewRegistry()
    // Register built-in plugins with their schemas and adapters
    // HTTP
    httpAdapter, _ := adapter.NewHTTPAdapter()
    r.Register("http", PluginDef{
        Actions: map[string]ActionDef{
            "get":    {Params: []Param{{Name: "path", Type: TypeExpr{Name: "string"}}}},
            "post":   {Params: []Param{{Name: "path", Type: TypeExpr{Name: "string"}}, {Name: "body", Type: TypeExpr{Name: "any"}}}},
            "put":    {Params: []Param{{Name: "path", Type: TypeExpr{Name: "string"}}, {Name: "body", Type: TypeExpr{Name: "any"}}}},
            "delete": {Params: []Param{{Name: "path", Type: TypeExpr{Name: "string"}}}},
            "header": {Params: []Param{{Name: "name", Type: TypeExpr{Name: "string"}}, {Name: "value", Type: TypeExpr{Name: "string"}}}},
        },
        Assertions: map[string]AssertionDef{
            "status": {Type: "int"},
            "body":   {Type: "any"},
            "header": {Type: "string"},
        },
        Adapter: httpAdapter,
    })
    // Process and Playwright similarly...
    return r
}
```

Note: `DefaultRegistry()` creates new adapter instances each time. This is intentional — each Verify call gets fresh adapters.

**Step 3: Write tests for Verify and Generate**

Create `pkg/spec/spec_test.go` with basic tests:
- Parse a simple spec, call Generate, verify output has expected fields
- Parse a simple spec with a mock adapter, call Verify, check results

**Step 4: Verify** (build + test + lint)

**Step 5: Commit**

```bash
git commit -m "feat(spec): add Verify, Generate, Parse, ParseFile, DefaultRegistry"
```

---

### Task 11: Create `pkg/spec/errors.go`

**Files:**
- Create: `pkg/spec/errors.go`

**Step 1: Add FormatErrors**

```go
package spec

import "github.com/bamsammich/speclang/v2/internal/validator"

// FormatErrors formats validation errors into a human-readable string.
func FormatErrors(errs []error) string {
    return validator.FormatErrors(errs)
}
```

**Step 2: Verify** (build + test + lint)

**Step 3: Commit**

```bash
git commit -m "feat(spec): add FormatErrors for validation error display"
```

---

## Phase 4: Update `cmd/specrun/`

### Task 12: Update CLI to use `pkg/spec` and `internal/`

**Files:**
- Modify: `cmd/specrun/main.go`

**Step 1: Update imports**

Change all imports from `pkg/parser`, `pkg/adapter`, `pkg/runner`, etc. to use `pkg/spec` for types and `internal/*` for implementation that the CLI needs directly (like `internal/infra` for service lifecycle).

The CLI's `runVerify` can either:
- Call `spec.Verify()` (if it does everything needed)
- Or continue wiring internals directly (for service lifecycle, progress output, etc.)

Since the CLI has special needs (service lifecycle, progress output, signal handling, `--keep-services`), it will likely still wire some internals directly. But it should use `pkg/spec` types everywhere.

**Step 2: Update `createAdapters` to use `spec.Registry`**

Replace the hardcoded switch in `createSingleAdapter` with registry lookup. The CLI builds a `spec.DefaultRegistry()` and passes it through.

**Step 3: Verify** (build + test + lint + self-verification)

Run the full self-verification to confirm behavioral equivalence:
```bash
go build -o ./specrun ./cmd/specrun
go build -o ./echo_tool ./testdata/self/echo_tool
# If Docker available:
SPECRUN_BIN=./specrun ECHO_TOOL_BIN=./echo_tool ./specrun verify specs/speclang.spec
```

**Step 4: Commit**

```bash
git commit -m "refactor(cli): update specrun to use pkg/spec and internal/"
```

---

## Phase 5: Documentation

### Task 13: Create `docs/package.md` — library integration guide

**Files:**
- Create: `docs/package.md`

Write a comprehensive guide covering:

1. **Getting started** — import path, basic usage
2. **Parsing specs** — from files and programmatically
3. **Constructing specs programmatically** — building AST structs
4. **Registering custom adapters** — implementing the Adapter interface, creating a PluginDef, registering in a Registry
5. **Running verification** — calling Verify(), reading Results
6. **Generating inputs** — calling Generate() for standalone input generation
7. **Slopinary integration example** — concrete example showing how an external system would use the package:
   - Fetch a manifest
   - Build a spec from manifest + corpus
   - Implement a custom adapter
   - Register it
   - Run verification
   - Read structured results
   - Build a conformance report

Commit: `docs: add Go package integration guide with slopinary example`

---

### Task 14: Update CLAUDE.md and README

**Files:**
- Modify: `CLAUDE.md` — update project structure tree
- Modify: `README.md` — add "Library Usage" section linking to docs/package.md

Commit: `docs: update CLAUDE.md project structure and README library section`

---

## Phase 6: Final verification and PR

### Task 15: Final verification

**Step 1: Full checks**

```bash
golangci-lint run ./...    # zero issues
go test ./... -count=1     # all pass
```

**Step 2: Self-verification**

```bash
go build -o ./specrun ./cmd/specrun
go build -o ./echo_tool ./testdata/self/echo_tool
SPECRUN_BIN=./specrun ECHO_TOOL_BIN=./echo_tool ./specrun verify specs/speclang.spec
```

ALL scenarios and invariants must pass. This is the behavioral regression gate — no specs were modified, so identical pass = equivalent behavior.

**Step 3: Verify no .spec files were modified**

```bash
git diff --name-only main | grep "\.spec$"
# Must return nothing
```

**Step 4: Create PR**

```bash
git push -u origin <branch>
gh pr create --title "refactor: reorganize packages into pkg/spec (public) and internal/ (private) (#76)" --body "..."
gh pr checks <number> --watch
gh pr merge <number> --squash --delete-branch
```
