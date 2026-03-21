# Verify Output Design

**Date:** 2026-03-20
**Status:** Approved

## Goal

Improve `specrun verify` output to show per-scope, per-scenario/invariant results instead of just aggregate counts and a flat failure list.

## Data Model

Add `ScopeResult` and `CheckResult` types to `pkg/runner/runner.go`. The `Result` struct gains a `Scopes` field.

```go
type Result struct {
    Spec              string        `json:"spec"`
    Scopes            []ScopeResult `json:"scopes"`
    Failures          []Failure     `json:"failures"`
    ScenariosRun      int           `json:"scenarios_run"`
    ScenariosPassed   int           `json:"scenarios_passed"`
    InvariantsChecked int           `json:"invariants_checked"`
    InvariantsPassed  int           `json:"invariants_passed"`
}

type ScopeResult struct {
    Name   string        `json:"name"`
    Checks []CheckResult `json:"checks"`
}

type CheckResult struct {
    Name      string   `json:"name"`
    Kind      string   `json:"kind"`               // "scenario" or "invariant"
    Passed    bool     `json:"passed"`
    InputsRun int      `json:"inputs_run"`          // 1 for given-scenarios, N for when/invariants
    FailedAt  int      `json:"failed_at,omitempty"` // which input number failed (0 if passed)
    Failure   *Failure `json:"failure,omitempty"`
}
```

Aggregate counts (`ScenariosRun`, `ScenariosPassed`, etc.) are computed from `CheckResult` data for backwards compatibility. The flat `Failures` slice is also preserved.

## Human-Readable Output

### Success

```
verifying examples/transfer.spec (AccountAPI) with seed=42, iterations=50

  scope transfer:
    ✓ scenario success
    ✓ scenario overdraft (50 inputs)
    ✓ scenario zero_transfer (50 inputs)
    ✓ invariant conservation (50 inputs)
    ✓ invariant non_negative (50 inputs)
    ✓ invariant no_mutation_on_error (50 inputs)

Scenarios:  3/3 passed
Invariants: 3/3 passed
```

### Failure

```
verifying examples/transfer.spec (AccountAPI) with seed=42, iterations=50

  scope transfer:
    ✓ scenario success
    ✗ scenario overdraft (failed on input 12/50)
        input:
          {
            "from": {"id": "abc", "balance": 100},
            "to": {"id": "def", "balance": 50},
            "amount": 150
          }
        expected: error = "insufficient_funds"
        actual:   error = null
    ✓ scenario zero_transfer (50 inputs)
    ✗ invariant conservation (failed on input 3/50, shrunk)
        input:
          {
            "from": {"id": "a", "balance": 1},
            "to": {"id": "b", "balance": 0},
            "amount": 1
          }
        expected: output.from.balance + output.to.balance == input.from.balance + input.to.balance
        actual:   invariant evaluated to false
    ✓ invariant non_negative (50 inputs)
    ✓ invariant no_mutation_on_error (50 inputs)

Scenarios:  2/3 passed
Invariants: 2/3 passed
```

### Formatting Rules

- `given`-scenarios show no input count (always 1)
- `when`-scenarios and invariants show `(N inputs)` on pass, `(failed on input M/N)` on fail
- `(shrunk)` appended when counterexample was shrunk
- Failed items expand with indented input/expected/actual
- Summary counts unchanged at bottom

## Implementation

### Files Changed

1. **`pkg/runner/runner.go`** — Add `ScopeResult` and `CheckResult` types. `runGivenScenario`, `runWhenScenario`, and `runInvariant` return `(CheckResult, error)`. `scopeRunner.run` orchestrates and appends `ScopeResult` to `res.Scopes`. Aggregate counts computed from check results.

2. **`cmd/specrun/main.go`** — Rewrite `printResults` to iterate `res.Scopes` and render per-item output. No changes to `--json` path.

3. **`specs/verify.spec`** — Amend `verify_pass` scenario to assert on the new `scopes` structure in JSON output.

### No Changes To

- Parser, generator, adapter, plugin definitions
- Self-verification spec structure (parse, generate scopes unchanged)
