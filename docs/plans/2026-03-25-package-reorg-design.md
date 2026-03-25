# Package Reorganization Design

**Date:** 2026-03-25
**Status:** Approved
**Issue:** #76

## Goal

Reorganize speclang's package structure into `pkg/spec/` (public API) and `internal/` (implementation), providing a clean importable Go package for external consumers.

## Package Structure

```
speclang/
├── cmd/specrun/              # CLI binary
├── pkg/spec/                 # PUBLIC API
│   ├── ast.go                # All types: Spec, Scope, Model, Contract, Field,
│   │                         #   TypeExpr, Expr interface + all concrete Expr types,
│   │                         #   Target, Service, Invariant, Scenario, Block,
│   │                         #   Assertion, Assignment, Call, GivenStep, Action, Param
│   ├── adapter.go            # Adapter interface, Response, Request,
│   │                         #   NewHTTPAdapter, NewProcessAdapter, NewPlaywrightAdapter
│   ├── registry.go           # PluginDef, ActionDef, AssertionDef, Registry,
│   │                         #   DefaultRegistry(), programmatic registration
│   ├── spec.go               # Verify(), Generate(), Parse(), ParseFile(), Options
│   ├── result.go             # Result, ScopeResult, CheckResult, Failure
│   ├── import.go             # ImportResolver interface, ImportRegistry type
│   └── errors.go             # Validate(), FormatErrors()
├── internal/
│   ├── parser/               # lexer, parser, include/import resolution
│   ├── generator/            # input generation, expression eval, shrinking
│   ├── runner/               # orchestration (generate → execute → assert)
│   ├── validator/            # type checking, semantic validation
│   ├── infra/                # Docker/compose container lifecycle
│   ├── openapi/              # OpenAPI import resolver
│   └── proto/                # Protobuf import resolver
├── plugins/                  # .plugin definition files (unchanged)
├── specs/                    # self-verification (unchanged, NOT modified)
├── examples/                 # (unchanged)
├── testdata/                 # (unchanged)
└── docs/                     # (unchanged + new package.md)
```

## Public API Surface

### spec.go
```go
func Parse(source string) (*Spec, error)
func ParseFile(path string, imports ImportRegistry) (*Spec, error)
func Validate(spec *Spec) []error
func Verify(ctx context.Context, s *Spec, registry *Registry, opts Options) (*Result, error)
func Generate(s *Spec, scopeName string, seed uint64) (map[string]any, error)

type Options struct {
    Seed       uint64
    Iterations int
}
```

`Verify` takes a `*Registry`. Pass nil to use `DefaultRegistry()` (built-in adapters).

### registry.go
```go
type PluginDef struct {
    Actions    map[string]ActionDef
    Assertions map[string]AssertionDef
    Adapter    Adapter
}
type ActionDef struct { Params []Param }
type AssertionDef struct { Type string }

type Registry struct { ... }
func NewRegistry() *Registry
func DefaultRegistry() *Registry  // returns fresh copy with http, process, playwright
func (r *Registry) Register(name string, def PluginDef)
func (r *Registry) Adapter(name string) (Adapter, error)
```

### Default registry pattern
```go
// Built-in only (90% of callers):
result, _ := spec.Verify(ctx, s, nil, opts)

// Custom adapter:
reg := spec.DefaultRegistry()
reg.Register("slopinary", spec.PluginDef{...})
result, _ := spec.Verify(ctx, s, reg, opts)
```

## Migration Strategy

### Phase order
1. Create `pkg/spec/ast.go` — move all types from `pkg/parser/ast.go`
2. Create `pkg/spec/adapter.go` — move Adapter, Request, Response from `pkg/adapter/protocol.go`, re-export adapter constructors
3. Create `pkg/spec/result.go` — move Result types from `pkg/runner/runner.go`
4. Create `pkg/spec/import.go` — move ImportResolver, ImportRegistry from `pkg/parser/import.go`
5. Create `pkg/spec/registry.go` — new PluginDef, Registry, DefaultRegistry
6. Move `pkg/` → `internal/` — parser, generator, runner, validator, infra, openapi, proto
7. Create `pkg/spec/spec.go` — top-level functions wiring internal packages
8. Update `cmd/specrun/` — import from new paths
9. Delete `pkg/plugin/` — replaced by registry

### Testing strategy
- Specs are NOT modified. Self-verification is the regression gate.
- Unit tests move with their packages into `internal/`.
- `specrun verify specs/speclang.spec` must pass after reorg.

### What moves where

| Current | Destination |
|---|---|
| `pkg/parser/ast.go` types | `pkg/spec/ast.go` |
| `pkg/parser/*.go` logic | `internal/parser/` |
| `pkg/parser/import.go` types | `pkg/spec/import.go` |
| `pkg/adapter/protocol.go` | `pkg/spec/adapter.go` |
| `pkg/adapter/*.go` implementations | `internal/adapter/` |
| `pkg/runner/runner.go` types | `pkg/spec/result.go` |
| `pkg/runner/runner.go` logic | `internal/runner/` |
| `pkg/generator/` | `internal/generator/` |
| `pkg/validator/` | `internal/validator/` |
| `pkg/infra/` | `internal/infra/` |
| `pkg/openapi/` | `internal/openapi/` |
| `pkg/proto/` | `internal/proto/` |
| `pkg/plugin/` | Deleted |

## Decisions

- **Version**: Stay v2 (no external consumers, no real breakage)
- **Registry**: Default registry with built-in adapters. nil = use default. Clone to extend.
- **External plugins**: Separate effort (#77). Registry supports programmatic registration now.
- **Adapter interface**: Keep `json.RawMessage` for now. `any` is a follow-up (#75).
- **Specs**: NOT modified. They are the behavioral regression gate.

## Documentation

- New `docs/package.md` — Go library integration guide with slopinary example
- Update `CLAUDE.md` — project structure tree
- Update `README.md` — link to package docs
