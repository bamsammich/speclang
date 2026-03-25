# Go Package Integration Guide

## Overview

SpecLang is available as a Go package for programmatic spec parsing, input generation, and verification. Two packages form the public API:

- **`pkg/spec`** — types and interfaces: `Spec`, `Scope`, `Model`, `Adapter`, `Registry`, `Result`, etc.
- **`pkg/specrun`** — pipeline functions: `Parse`, `ParseFile`, `Verify`, `Generate`, `DefaultRegistry`

All implementation details (parser, generator, runner, validator, adapters) live in `internal/` and are not importable.

## Installation

```bash
go get github.com/bamsammich/speclang/v2
```

## Quick Start

Parse a spec file and run verification with default adapters:

```go
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/bamsammich/speclang/v2/pkg/specrun"
)

func main() {
	s, err := specrun.ParseFile("my.spec", nil)
	if err != nil {
		log.Fatal(err)
	}

	result, err := specrun.Verify(s, nil, specrun.Options{
		Seed:       42,
		Iterations: 100,
	})
	if err != nil {
		log.Fatal(err)
	}

	if len(result.Failures) > 0 {
		json.NewEncoder(os.Stderr).Encode(result.Failures)
		os.Exit(1)
	}
	fmt.Printf("All checks passed: %d scenarios, %d invariants\n",
		result.ScenariosPassed, result.InvariantsPassed)
}
```

## Package Structure

### `pkg/spec` — Types and Interfaces

Core AST types for representing parsed specs:

| Type | Description |
|------|-------------|
| `Spec` | Top-level node: name, target, models, scopes |
| `Scope` | Named grouping with plugin binding, contract, invariants, scenarios |
| `Model` | Named data structure with typed fields |
| `Field` | Typed field with optional constraint expression |
| `TypeExpr` | Type representation (int, string, []T, map[K,V], enum, model ref, T?) |
| `Contract` | Input/output boundary definition |
| `Invariant` | Universal property that must hold for all valid inputs |
| `Scenario` | Test case — concrete (given) or generative (when) |
| `Target` | System-under-test configuration (env vars, services) |
| `Service` | Docker container declaration for test infrastructure |

Adapter interface for plugin implementations:

| Type | Description |
|------|-------------|
| `Adapter` | Interface: `Init`, `Action`, `Assert`, `Close` |
| `Request` | JSON message sent to an adapter |
| `Response` | JSON message returned from an adapter |

Plugin registry:

| Type | Description |
|------|-------------|
| `Registry` | Holds registered plugins and their adapters |
| `PluginDef` | Plugin schema: actions, assertions, and adapter instance |
| `ActionDef` | Parameter schema for a plugin action |
| `AssertionDef` | Expected type for a plugin assertion |

Verification results:

| Type | Description |
|------|-------------|
| `Result` | Top-level outcome: scope results, failures, counts |
| `ScopeResult` | Per-scope checks |
| `CheckResult` | Single scenario or invariant outcome |
| `Failure` | Failed check with input, expected, actual, and shrunk flag |

Import support:

| Type | Description |
|------|-------------|
| `ImportResolver` | Interface to convert external schemas into AST nodes |
| `ImportRegistry` | Maps import scheme names to resolvers |

### `pkg/specrun` — Pipeline Functions

| Function | Description |
|----------|-------------|
| `Parse(source string)` | Parse a spec from source text |
| `ParseFile(path string, imports ImportRegistry)` | Parse from file with include/import resolution |
| `Validate(s *Spec)` | Check for semantic errors (returns `[]error`) |
| `FormatErrors(errs []error)` | Format validation errors for display |
| `Verify(s *Spec, registry *Registry, opts Options)` | Run full verification pipeline |
| `Generate(s *Spec, scopeName string, seed uint64)` | Produce one random input for a scope |
| `DefaultRegistry()` | Registry with built-in plugins (http, process, playwright) |

## Parsing Specs

### From a file

`ParseFile` resolves `include` directives and `import` directives. Pass `nil` for imports if you don't need OpenAPI/protobuf import resolution:

```go
s, err := specrun.ParseFile("specs/api.spec", nil)
```

To resolve imports, provide an `ImportRegistry`:

```go
imports := spec.ImportRegistry{
	"openapi": myOpenAPIResolver,
	"proto":   myProtoResolver,
}
s, err := specrun.ParseFile("specs/api.spec", imports)
```

### From a string

```go
source := `spec MyAPI {
  model User { name: string  age: int { age >= 0 } }
  scope health {
    use http
    config { path: "/health" method: "GET" }
    contract { output { status: string } }
    scenario up {
      then { status: "ok" }
    }
  }
}`
s, err := specrun.Parse(source)
```

### Validation

After parsing, check for semantic errors:

```go
errs := specrun.Validate(s)
if len(errs) > 0 {
	fmt.Fprintln(os.Stderr, specrun.FormatErrors(errs))
	os.Exit(1)
}
```

## Constructing Specs Programmatically

You can build a `Spec` directly from Go structs without any `.spec` file:

```go
s := &spec.Spec{
	Name: "HealthCheck",
	Target: &spec.Target{
		Fields: map[string]spec.Expr{
			"base_url": spec.LiteralString{Value: "http://localhost:8080"},
		},
	},
	Scopes: []*spec.Scope{
		{
			Name: "health",
			Use:  "http",
			Config: map[string]spec.Expr{
				"path":   spec.LiteralString{Value: "/health"},
				"method": spec.LiteralString{Value: "GET"},
			},
			Contract: &spec.Contract{
				Output: []*spec.Field{
					{Name: "status", Type: spec.TypeExpr{Name: "string"}},
				},
			},
			Scenarios: []*spec.Scenario{
				{
					Name: "up",
					Then: &spec.Block{
						Assertions: []*spec.Assertion{
							{Target: "status", Expected: spec.LiteralString{Value: "ok"}},
						},
					},
				},
			},
		},
	},
}

result, err := specrun.Verify(s, nil, specrun.Options{Seed: 42})
```

## Custom Adapters

Implement the `spec.Adapter` interface to add support for a custom protocol.

### The Adapter interface

```go
type Adapter interface {
	Init(config map[string]string) error
	Action(name string, args json.RawMessage) (*Response, error)
	Assert(property string, locator string, expected json.RawMessage) (*Response, error)
	Close() error
}
```

- `Init` receives the scope's `config` block as string key-value pairs.
- `Action` executes a named action (e.g., `get`, `post`, `exec`) with JSON-encoded arguments.
- `Assert` checks a property against the system under test. Returns `Response{OK: true}` on success.
- `Close` is called once after all checks complete.

### Registering a custom adapter

Create a `PluginDef` with the action/assertion schema and your adapter, then register it:

```go
reg := specrun.DefaultRegistry()
reg.Register("grpc", spec.PluginDef{
	Actions: map[string]spec.ActionDef{
		"call": {
			Params: []spec.Param{
				{Name: "method", Type: spec.TypeExpr{Name: "string"}},
				{Name: "body", Type: spec.TypeExpr{Name: "any"}},
			},
		},
	},
	Assertions: map[string]spec.AssertionDef{
		"status": {Type: "int"},
		"body":   {Type: "any"},
	},
	Adapter: &MyGRPCAdapter{},
})

result, err := specrun.Verify(s, reg, specrun.Options{Seed: 42, Iterations: 100})
```

## Reading Results

The `Result` struct provides a full breakdown of the verification run:

```go
result, err := specrun.Verify(s, nil, specrun.Options{Seed: 42, Iterations: 100})
if err != nil {
	log.Fatal(err)
}

// Top-level summary
fmt.Printf("Scenarios: %d/%d passed\n", result.ScenariosPassed, result.ScenariosRun)
fmt.Printf("Invariants: %d/%d passed\n", result.InvariantsPassed, result.InvariantsChecked)

// Walk per-scope results
for _, scope := range result.Scopes {
	fmt.Printf("\nScope: %s\n", scope.Name)
	for _, check := range scope.Checks {
		status := "PASS"
		if !check.Passed {
			status = "FAIL"
		}
		fmt.Printf("  [%s] %s %s (%d inputs)\n", status, check.Kind, check.Name, check.InputsRun)

		if check.Failure != nil {
			f := check.Failure
			fmt.Printf("    Expected: %v\n", f.Expected)
			fmt.Printf("    Actual:   %v\n", f.Actual)
			if f.Shrunk {
				fmt.Printf("    (shrunk to minimal counterexample)\n")
			}
			if f.Input != nil {
				fmt.Printf("    Input: %v\n", f.Input)
			}
		}
	}
}

// Failures slice for quick access to all failures
for _, f := range result.Failures {
	fmt.Printf("FAIL: %s/%s — %s\n", f.Scope, f.Name, f.Description)
}
```

## Standalone Generation

Generate a random input that satisfies a scope's contract constraints:

```go
input, err := specrun.Generate(s, "transfer", 42)
if err != nil {
	log.Fatal(err)
}
// input is map[string]any with fields matching the contract's input definition
fmt.Printf("Generated: %+v\n", input)
```

Different seeds produce different inputs. The generator respects type constraints, value ranges, and model references.

## Real-World Example: Conformance Testing

This example shows how an external system would use speclang programmatically to run conformance tests against a tool's HTTP API. The tool publishes a manifest describing its endpoints, and we build a spec and custom adapter to verify it.

### The manifest

Suppose a tool serves a manifest at `/manifest.json`:

```json
{
  "name": "WidgetService",
  "version": "2.1.0",
  "endpoints": [
    {
      "name": "create_widget",
      "method": "POST",
      "path": "/api/widgets",
      "input": {"name": "string", "weight": "int"},
      "output": {"id": "string", "name": "string", "weight": "int"},
      "constraints": {"weight_positive": "weight > 0"}
    },
    {
      "name": "get_widget",
      "method": "GET",
      "path": "/api/widgets/{id}",
      "input": {"id": "string"},
      "output": {"id": "string", "name": "string", "weight": "int"}
    }
  ]
}
```

### The adapter

```go
package conformance

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/bamsammich/speclang/v2/pkg/spec"
)

type Manifest struct {
	Name      string     `json:"name"`
	Version   string     `json:"version"`
	Endpoints []Endpoint `json:"endpoints"`
}

type Endpoint struct {
	Name        string            `json:"name"`
	Method      string            `json:"method"`
	Path        string            `json:"path"`
	Input       map[string]string `json:"input"`
	Output      map[string]string `json:"output"`
	Constraints map[string]string `json:"constraints"`
}

// ToolAdapter implements spec.Adapter for the tool's HTTP API.
type ToolAdapter struct {
	BaseURL    string
	client     *http.Client
	lastStatus int
	lastBody   map[string]any
}

func (a *ToolAdapter) Init(config map[string]string) error {
	a.BaseURL = config["base_url"]
	a.client = &http.Client{}
	return nil
}

func (a *ToolAdapter) Action(name string, argsRaw json.RawMessage) (*spec.Response, error) {
	var args struct {
		Method string         `json:"method"`
		Path   string         `json:"path"`
		Body   map[string]any `json:"body"`
	}
	if err := json.Unmarshal(argsRaw, &args); err != nil {
		return nil, fmt.Errorf("unmarshal args: %w", err)
	}

	var bodyReader io.Reader
	if args.Body != nil {
		b, _ := json.Marshal(args.Body)
		bodyReader = bytes.NewReader(b)
	}

	req, err := http.NewRequest(args.Method, a.BaseURL+args.Path, bodyReader)
	if err != nil {
		return nil, err
	}
	if bodyReader != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := a.client.Do(req)
	if err != nil {
		return &spec.Response{OK: false, Error: err.Error()}, nil
	}
	defer resp.Body.Close()

	a.lastStatus = resp.StatusCode
	a.lastBody = make(map[string]any)
	json.NewDecoder(resp.Body).Decode(&a.lastBody)

	actual, _ := json.Marshal(a.lastBody)
	return &spec.Response{OK: true, Actual: actual}, nil
}

func (a *ToolAdapter) Assert(property, locator string, expected json.RawMessage) (*spec.Response, error) {
	switch property {
	case "status":
		actual, _ := json.Marshal(a.lastStatus)
		var exp int
		json.Unmarshal(expected, &exp)
		return &spec.Response{OK: a.lastStatus == exp, Actual: actual}, nil
	case "body":
		actual, _ := json.Marshal(a.lastBody)
		return &spec.Response{OK: true, Actual: actual}, nil
	default:
		// Field-level assertion: check a.lastBody[property]
		val, ok := a.lastBody[property]
		if !ok {
			return &spec.Response{OK: false, Error: fmt.Sprintf("field %q not in response", property)}, nil
		}
		actual, _ := json.Marshal(val)
		return &spec.Response{OK: true, Actual: actual}, nil
	}
}

func (a *ToolAdapter) Close() error {
	return nil
}
```

### Building the spec from the manifest

```go
package conformance

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/bamsammich/speclang/v2/pkg/spec"
)

func typeExprFrom(t string) spec.TypeExpr {
	return spec.TypeExpr{Name: t}
}

func BuildSpec(manifest Manifest, baseURL string) *spec.Spec {
	s := &spec.Spec{
		Name: manifest.Name + "Conformance",
		Target: &spec.Target{
			Fields: map[string]spec.Expr{
				"base_url": spec.LiteralString{Value: baseURL},
			},
		},
	}

	for _, ep := range manifest.Endpoints {
		scope := &spec.Scope{
			Name: ep.Name,
			Use:  "tool",
			Config: map[string]spec.Expr{
				"base_url": spec.LiteralString{Value: baseURL},
			},
			Contract: &spec.Contract{},
		}

		// Build input fields
		for name, typ := range ep.Input {
			scope.Contract.Input = append(scope.Contract.Input, &spec.Field{
				Name: name,
				Type: typeExprFrom(typ),
			})
		}

		// Build output fields
		for name, typ := range ep.Output {
			scope.Contract.Output = append(scope.Contract.Output, &spec.Field{
				Name: name,
				Type: typeExprFrom(typ),
			})
		}

		// Add an invariant for each declared constraint
		for name, expr := range ep.Constraints {
			// Parse the constraint expression from the manifest
			constraintSpec := fmt.Sprintf(`spec _tmp {
				scope _s {
					use tool
					contract { input { x: int } }
					invariant %s { %s }
				}
			}`, name, expr)
			tmp, err := Parse(constraintSpec)
			if err == nil && len(tmp.Scopes) > 0 && len(tmp.Scopes[0].Invariants) > 0 {
				scope.Invariants = append(scope.Invariants, tmp.Scopes[0].Invariants[0])
			}
		}

		s.Scopes = append(s.Scopes, scope)
	}

	return s
}

// Parse is a local alias for convenience.
var Parse = func(source string) (*spec.Spec, error) {
	// Import here to avoid circular reference in the example.
	// In real code, import specrun directly.
	return nil, fmt.Errorf("placeholder — use specrun.Parse")
}
```

### Running the conformance check

```go
package conformance

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/bamsammich/speclang/v2/pkg/spec"
	"github.com/bamsammich/speclang/v2/pkg/specrun"
)

type ConformanceReport struct {
	Tool       string        `json:"tool"`
	Version    string        `json:"version"`
	BaseURL    string        `json:"base_url"`
	Passed     bool          `json:"passed"`
	Summary    string        `json:"summary"`
	Scopes     []ScopeReport `json:"scopes"`
	TotalRun   int           `json:"total_run"`
	TotalPass  int           `json:"total_passed"`
	TotalFail  int           `json:"total_failed"`
}

type ScopeReport struct {
	Name     string         `json:"name"`
	Passed   bool           `json:"passed"`
	Checks   []CheckReport  `json:"checks"`
}

type CheckReport struct {
	Name     string `json:"name"`
	Kind     string `json:"kind"`
	Passed   bool   `json:"passed"`
	Detail   string `json:"detail,omitempty"`
}

func RunConformance(baseURL string) (*ConformanceReport, error) {
	// 1. Fetch the manifest
	resp, err := http.Get(baseURL + "/manifest.json")
	if err != nil {
		return nil, fmt.Errorf("fetch manifest: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read manifest: %w", err)
	}

	var manifest Manifest
	if err := json.Unmarshal(body, &manifest); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}

	// 2. Build a spec from the manifest
	s := BuildSpec(manifest, baseURL)

	// 3. Register the custom adapter
	reg := specrun.DefaultRegistry()
	reg.Register("tool", spec.PluginDef{
		Actions: map[string]spec.ActionDef{
			"call": {
				Params: []spec.Param{
					{Name: "method", Type: spec.TypeExpr{Name: "string"}},
					{Name: "path", Type: spec.TypeExpr{Name: "string"}},
					{Name: "body", Type: spec.TypeExpr{Name: "any"}},
				},
			},
		},
		Assertions: map[string]spec.AssertionDef{
			"status": {Type: "int"},
			"body":   {Type: "any"},
		},
		Adapter: &ToolAdapter{},
	})

	// 4. Run verification
	result, err := specrun.Verify(s, reg, specrun.Options{
		Seed:       42,
		Iterations: 100,
	})
	if err != nil {
		return nil, fmt.Errorf("verify: %w", err)
	}

	// 5. Build the conformance report
	report := &ConformanceReport{
		Tool:    manifest.Name,
		Version: manifest.Version,
		BaseURL: baseURL,
		Passed:  len(result.Failures) == 0,
		Summary: fmt.Sprintf("%d/%d scenarios, %d/%d invariants",
			result.ScenariosPassed, result.ScenariosRun,
			result.InvariantsPassed, result.InvariantsChecked),
	}

	for _, sr := range result.Scopes {
		scopeReport := ScopeReport{Name: sr.Name, Passed: true}
		for _, cr := range sr.Checks {
			check := CheckReport{
				Name:   cr.Name,
				Kind:   cr.Kind,
				Passed: cr.Passed,
			}
			if !cr.Passed && cr.Failure != nil {
				check.Detail = cr.Failure.Description
				scopeReport.Passed = false
			}
			scopeReport.Checks = append(scopeReport.Checks, check)
			report.TotalRun++
			if cr.Passed {
				report.TotalPass++
			} else {
				report.TotalFail++
			}
		}
		report.Scopes = append(report.Scopes, scopeReport)
	}

	return report, nil
}

func main() {
	baseURL := os.Getenv("TOOL_URL")
	if baseURL == "" {
		baseURL = "http://localhost:9090"
	}

	report, err := RunConformance(baseURL)
	if err != nil {
		log.Fatal(err)
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.Encode(report)

	if !report.Passed {
		os.Exit(1)
	}
}
```

This produces structured JSON output:

```json
{
  "tool": "WidgetService",
  "version": "2.1.0",
  "base_url": "http://localhost:9090",
  "passed": false,
  "summary": "2/2 scenarios, 1/2 invariants",
  "scopes": [
    {
      "name": "create_widget",
      "passed": false,
      "checks": [
        {"name": "weight_positive", "kind": "invariant", "passed": false, "detail": "..."}
      ]
    }
  ],
  "total_run": 3,
  "total_passed": 2,
  "total_failed": 1
}
```
