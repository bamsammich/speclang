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

## Putting It Together: Custom Adapter Example

This example shows a complete integration: a custom adapter that tests a calculator REST API, with the spec constructed programmatically.

### The adapter

```go
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"

	"github.com/bamsammich/speclang/v2/pkg/spec"
)

// CalcAdapter implements spec.Adapter for a calculator HTTP API.
type CalcAdapter struct {
	baseURL  string
	client   *http.Client
	lastBody map[string]any
}

func (a *CalcAdapter) Init(config map[string]string) error {
	a.baseURL = config["base_url"]
	a.client = &http.Client{}
	return nil
}

func (a *CalcAdapter) Action(name string, argsRaw json.RawMessage) (*spec.Response, error) {
	var args []json.RawMessage
	if err := json.Unmarshal(argsRaw, &args); err != nil {
		return nil, fmt.Errorf("unmarshal args: %w", err)
	}

	var path string
	if err := json.Unmarshal(args[0], &path); err != nil {
		return nil, fmt.Errorf("unmarshal path: %w", err)
	}

	var body json.RawMessage
	if len(args) > 1 {
		body = args[1]
	}

	req, err := http.NewRequest("POST", a.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		return &spec.Response{OK: false, Error: err.Error()}, nil
	}
	defer resp.Body.Close()

	a.lastBody = make(map[string]any)
	json.NewDecoder(resp.Body).Decode(&a.lastBody)

	actual, _ := json.Marshal(a.lastBody)
	return &spec.Response{OK: true, Actual: actual}, nil
}

func (a *CalcAdapter) Assert(property, _ string, expected json.RawMessage) (*spec.Response, error) {
	val, ok := a.lastBody[property]
	if !ok {
		return &spec.Response{OK: false, Error: fmt.Sprintf("field %q not found", property)}, nil
	}

	actual, _ := json.Marshal(val)

	var actualNorm, expectedNorm any
	json.Unmarshal(actual, &actualNorm)
	json.Unmarshal(expected, &expectedNorm)

	if reflect.DeepEqual(actualNorm, expectedNorm) {
		return &spec.Response{OK: true, Actual: actual}, nil
	}
	return &spec.Response{
		OK:    false,
		Actual: actual,
		Error: fmt.Sprintf("expected %s, got %s", expected, actual),
	}, nil
}

func (a *CalcAdapter) Close() error { return nil }
```

### Building the spec and running verification

```go
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/bamsammich/speclang/v2/pkg/spec"
	"github.com/bamsammich/speclang/v2/pkg/specrun"
)

func main() {
	// Build the spec programmatically
	s := &spec.Spec{
		Name: "CalculatorAPI",
		Scopes: []*spec.Scope{
			{
				Name: "add",
				Use:  "calc",
				Contract: &spec.Contract{
					Input: []*spec.Field{
						{Name: "a", Type: spec.TypeExpr{Name: "int"}},
						{Name: "b", Type: spec.TypeExpr{Name: "int"}},
					},
					Output: []*spec.Field{
						{Name: "result", Type: spec.TypeExpr{Name: "int"}},
					},
				},
				Invariants: []*spec.Invariant{
					{
						Name: "correct_sum",
						Assertions: []*spec.Assertion{
							{Expr: &spec.BinaryOp{
								Left: &spec.FieldRef{Path: "output.result"},
								Op:   "==",
								Right: &spec.BinaryOp{
									Left:  &spec.FieldRef{Path: "input.a"},
									Op:    "+",
									Right: &spec.FieldRef{Path: "input.b"},
								},
							}},
						},
					},
				},
			},
		},
	}

	// Register the custom adapter
	reg := specrun.DefaultRegistry()
	reg.Register("calc", spec.PluginDef{
		Actions: map[string]spec.ActionDef{
			"compute": {Params: []spec.Param{
				{Name: "path", Type: spec.TypeExpr{Name: "string"}},
				{Name: "body", Type: spec.TypeExpr{Name: "any"}},
			}},
		},
		Assertions: map[string]spec.AssertionDef{
			"result": {Type: "int"},
		},
		Adapter: &CalcAdapter{},
	})

	// Run verification
	result, err := specrun.Verify(context.Background(), s, reg, spec.Options{
		Seed:       42,
		Iterations: 100,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	// Print results
	for _, scope := range result.Scopes {
		for _, check := range scope.Checks {
			status := "PASS"
			if !check.Passed {
				status = "FAIL"
			}
			fmt.Printf("[%s] %s/%s\n", status, scope.Name, check.Name)
		}
	}

	if len(result.Failures) > 0 {
		os.Exit(1)
	}
}
```

This demonstrates the full pattern: construct types, implement an adapter, register it, run verification, and read structured results. The same approach works for any protocol — gRPC, WebSocket, message queues, or custom binary protocols.
