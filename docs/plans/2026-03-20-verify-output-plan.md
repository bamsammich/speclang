# Verify Output Improvement Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Improve `specrun verify` output to show per-scope, per-scenario/invariant results with `✓`/`✗` markers.

**Architecture:** Add `ScopeResult` and `CheckResult` types to the runner. Refactor `runGivenScenario`, `runWhenScenario`, and `runInvariant` to return `(CheckResult, error)` instead of mutating `res` directly. `scopeRunner.run` orchestrates and builds the `ScopeResult`. Rewrite `printResults` in main.go. Update self-verification spec to assert on the new `scopes` structure.

**Tech Stack:** Go, no new dependencies

---

### Task 1: Add new result types and refactor runner methods

**Files:**
- Modify: `pkg/runner/runner.go`
- Test: `pkg/runner/runner_test.go`

**Step 1: Add the new types to `pkg/runner/runner.go`**

Add after the existing `Failure` struct:

```go
// ScopeResult captures per-scope verification results.
type ScopeResult struct {
	Name   string        `json:"name"`
	Checks []CheckResult `json:"checks"`
}

// CheckResult captures the outcome of a single scenario or invariant.
type CheckResult struct {
	Name      string   `json:"name"`
	Kind      string   `json:"kind"`               // "scenario" or "invariant"
	Passed    bool     `json:"passed"`
	InputsRun int      `json:"inputs_run"`          // 1 for given-scenarios, N for when/invariants
	FailedAt  int      `json:"failed_at,omitempty"` // which input number failed (0 if passed)
	Failure   *Failure `json:"failure,omitempty"`
}
```

Add `Scopes` field to `Result`:

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
```

**Step 2: Refactor `runGivenScenario` to return `(CheckResult, error)`**

Change the signature and body. It no longer mutates `res`:

```go
func (sr *scopeRunner) runGivenScenario(sc *parser.Scenario) (CheckResult, error) {
	input := assignmentsToMap(sc.Given.Assignments)

	if _, err := sr.executeInput(input); err != nil {
		return CheckResult{}, err
	}

	check := CheckResult{
		Name:      sc.Name,
		Kind:      "scenario",
		InputsRun: 1,
		Passed:    true,
	}

	if sc.Then != nil {
		if f, err := sr.checkThenAssertions(sc.Name, input, sc.Then); err != nil {
			return CheckResult{}, err
		} else if f != nil {
			check.Passed = false
			check.FailedAt = 1
			check.Failure = f
		}
	}

	return check, nil
}
```

**Step 3: Refactor `runWhenScenario` to return `(CheckResult, error)`**

```go
func (sr *scopeRunner) runWhenScenario(sc *parser.Scenario) (CheckResult, error) {
	predicate := buildPredicate(sc.When.Predicates)

	check := CheckResult{
		Name:   sc.Name,
		Kind:   "scenario",
		Passed: true,
	}

	for i := range sr.runner.n {
		input, err := sr.generator.GenerateMatching(predicate)
		if err != nil {
			return CheckResult{}, err
		}

		if _, err := sr.executeInput(input); err != nil {
			return CheckResult{}, err
		}

		check.InputsRun = i + 1

		if sc.Then == nil {
			continue
		}

		if f, err := sr.checkThenAssertions(sc.Name, input, sc.Then); err != nil {
			return CheckResult{}, err
		} else if f != nil {
			f = sr.shrinkFailure(f, sc.Then)
			check.Passed = false
			check.FailedAt = i + 1
			check.Failure = f
			return check, nil
		}
	}

	return check, nil
}
```

**Step 4: Refactor `runInvariant` to return `(CheckResult, error)`**

```go
func (sr *scopeRunner) runInvariant(inv *parser.Invariant) (CheckResult, error) {
	check := CheckResult{
		Name:   inv.Name,
		Kind:   "invariant",
		Passed: true,
	}

	for i := range sr.runner.n {
		input, err := sr.generator.GenerateInput()
		if err != nil {
			return CheckResult{}, err
		}

		output, err := sr.executeInput(input)
		if err != nil {
			return CheckResult{}, err
		}

		ctx := buildInvariantContext(input, output)

		if !evalGuard(inv.When, ctx) {
			continue
		}

		check.InputsRun = i + 1

		if f := checkInvariantAssertions(inv.Name, sr.scope, input, inv.Assertions, ctx); f != nil {
			f = sr.shrinkInvariantFailure(f, inv)
			check.Passed = false
			check.FailedAt = i + 1
			check.Failure = f
			return check, nil
		}
	}

	return check, nil
}
```

**Step 5: Rewrite `scopeRunner.run` to use the new return types**

```go
func (sr *scopeRunner) run(res *Result) error {
	scopeRes := ScopeResult{Name: sr.scope}

	for _, sc := range sr.scenarios() {
		var check CheckResult
		var err error

		if sc.Given != nil {
			check, err = sr.runGivenScenario(sc)
		} else if sc.When != nil {
			check, err = sr.runWhenScenario(sc)
		} else {
			continue
		}

		if err != nil {
			return fmt.Errorf("scope %q scenario %q: %w", sr.scope, sc.Name, err)
		}

		scopeRes.Checks = append(scopeRes.Checks, check)
		res.ScenariosRun++
		if check.Passed {
			res.ScenariosPassed++
		} else if check.Failure != nil {
			res.Failures = append(res.Failures, *check.Failure)
		}
	}

	for _, inv := range sr.invariants() {
		check, err := sr.runInvariant(inv)
		if err != nil {
			return fmt.Errorf("scope %q invariant %q: %w", sr.scope, inv.Name, err)
		}

		scopeRes.Checks = append(scopeRes.Checks, check)
		res.InvariantsChecked++
		if check.Passed {
			res.InvariantsPassed++
		} else if check.Failure != nil {
			res.Failures = append(res.Failures, *check.Failure)
		}
	}

	res.Scopes = append(res.Scopes, scopeRes)
	return nil
}
```

**Step 6: Run existing tests**

Run: `mise exec -- go test ./pkg/runner/ -v`
Expected: PASS — all existing behavior preserved, `Scopes` field is now populated but not checked by existing tests.

**Step 7: Add a test for the new `Scopes` field**

Add to `pkg/runner/runner_test.go`:

```go
func TestVerify_ScopeResults(t *testing.T) {
	spec, err := parser.ParseFile("../../examples/transfer.spec")
	if err != nil {
		t.Fatalf("parsing spec: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/accounts/transfer", transferHandler)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	adp := adapter.NewHTTPAdapter()
	if err := adp.Init(map[string]string{"base_url": srv.URL}); err != nil {
		t.Fatal(err)
	}

	r := runner.New(spec, adp, 42)
	r.SetN(10)
	res, err := r.Verify()
	if err != nil {
		t.Fatalf("verify: %v", err)
	}

	if len(res.Scopes) != 1 {
		t.Fatalf("expected 1 scope, got %d", len(res.Scopes))
	}

	scope := res.Scopes[0]
	if scope.Name != "transfer" {
		t.Errorf("expected scope name 'transfer', got %q", scope.Name)
	}

	// 3 scenarios + 3 invariants = 6 checks
	if len(scope.Checks) != 6 {
		t.Fatalf("expected 6 checks, got %d", len(scope.Checks))
	}

	for _, check := range scope.Checks {
		if !check.Passed {
			t.Errorf("check %q (%s) failed", check.Name, check.Kind)
		}
		if check.InputsRun < 1 {
			t.Errorf("check %q has InputsRun=%d, expected >= 1", check.Name, check.InputsRun)
		}
	}

	// Verify the first check is a scenario
	if scope.Checks[0].Kind != "scenario" {
		t.Errorf("expected first check to be scenario, got %q", scope.Checks[0].Kind)
	}
}
```

**Step 8: Run tests**

Run: `mise exec -- go test ./pkg/runner/ -v`
Expected: PASS

**Step 9: Full suite + commit**

```bash
mise exec -- go test ./...
git add pkg/runner/runner.go pkg/runner/runner_test.go
git commit -m "refactor(runner): add ScopeResult and CheckResult types"
```

---

### Task 2: Rewrite `printResults` for per-item output

**Files:**
- Modify: `cmd/specrun/main.go`
- Test: `cmd/specrun/main_test.go`

**Step 1: Write a test for the new output format**

Add to `cmd/specrun/main_test.go`:

```go
func TestVerify_HumanOutput(t *testing.T) {
	bin := specrunBin(t)

	srv := startTransferServer(t)
	defer srv.Close()

	specFile, err := filepath.Abs("../../examples/transfer.spec")
	if err != nil {
		t.Fatalf("abs path: %v", err)
	}

	cmd := exec.Command(bin, "verify", "--seed", "42", "--iterations", "10", specFile)
	cmd.Env = append(os.Environ(), "APP_URL="+srv.URL)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("verify failed: %v\n%s", err, out)
	}

	output := string(out)

	// Check per-scope structure
	if !strings.Contains(output, "scope transfer:") {
		t.Errorf("missing scope header in output:\n%s", output)
	}

	// Check per-item markers
	if !strings.Contains(output, "✓ scenario success") {
		t.Errorf("missing scenario success line:\n%s", output)
	}
	if !strings.Contains(output, "✓ invariant conservation") {
		t.Errorf("missing invariant conservation line:\n%s", output)
	}

	// Check summary
	if !strings.Contains(output, "Scenarios:  3/3 passed") {
		t.Errorf("missing scenario summary:\n%s", output)
	}
}
```

Add `"strings"` to imports.

**Step 2: Run test to verify it fails**

Run: `mise exec -- go test ./cmd/specrun/ -run TestVerify_HumanOutput -v`
Expected: FAIL — old format doesn't have scope headers or ✓ markers

**Step 3: Rewrite `printResults` in `cmd/specrun/main.go`**

```go
func printResults(res *runner.Result) {
	for _, scope := range res.Scopes {
		fmt.Printf("  scope %s:\n", scope.Name)
		for _, check := range scope.Checks {
			if check.Passed {
				printPassedCheck(check)
			} else {
				printFailedCheck(check)
			}
		}
		fmt.Println()
	}

	fmt.Printf("Scenarios:  %d/%d passed\n", res.ScenariosPassed, res.ScenariosRun)
	fmt.Printf("Invariants: %d/%d passed\n", res.InvariantsPassed, res.InvariantsChecked)
}

func printPassedCheck(check runner.CheckResult) {
	if check.InputsRun <= 1 {
		fmt.Printf("    ✓ %s %s\n", check.Kind, check.Name)
	} else {
		fmt.Printf("    ✓ %s %s (%d inputs)\n", check.Kind, check.Name, check.InputsRun)
	}
}

func printFailedCheck(check runner.CheckResult) {
	suffix := ""
	if check.Failure != nil && check.Failure.Shrunk {
		suffix = ", shrunk"
	}
	if check.InputsRun <= 1 {
		fmt.Printf("    ✗ %s %s (failed%s)\n", check.Kind, check.Name, suffix)
	} else {
		fmt.Printf("    ✗ %s %s (failed on input %d/%d%s)\n",
			check.Kind, check.Name, check.FailedAt, check.InputsRun, suffix)
	}

	if check.Failure == nil {
		return
	}

	f := check.Failure
	if f.Input != nil {
		if inputJSON, err := json.MarshalIndent(f.Input, "          ", "  "); err == nil {
			fmt.Printf("        input:\n          %s\n", inputJSON)
		}
	}
	if f.Expected != nil {
		fmt.Printf("        expected: %v\n", f.Expected)
	}
	if f.Actual != nil {
		fmt.Printf("        actual:   %v\n", f.Actual)
	}
}
```

**Step 4: Run test**

Run: `mise exec -- go test ./cmd/specrun/ -run TestVerify_HumanOutput -v`
Expected: PASS

**Step 5: Full suite + commit**

```bash
mise exec -- go test ./...
git add cmd/specrun/main.go cmd/specrun/main_test.go
git commit -m "feat(cli): rewrite verify output with per-item results"
```

---

### Task 3: Update self-verification spec for new JSON structure

**Files:**
- Modify: `specs/verify.spec`
- Test: `cmd/specrun/main_test.go` (existing `TestSelfVerification_Parse`)

**Step 1: Update `specs/verify.spec`**

The `verify_pass` scenario should assert on the new `scopes` field. The JSON output now includes `scopes[0].name` and `scopes[0].checks` array. We can assert that the first scope exists and has the right name:

```
scope verify_pass {
  config {
    args: "verify --json"
  }

  contract {
    input {
      file: string
    }
    output {
      exit_code: int
      scenarios_run: int
      scenarios_passed: int
      invariants_checked: int
      invariants_passed: int
      scopes: any
    }
  }

  scenario transfer_spec_passes {
    given {
      file: "examples/transfer.spec"
    }
    then {
      exit_code: 0
      scenarios_run: 3
      scenarios_passed: 3
      invariants_checked: 3
      invariants_passed: 3
    }
  }
}
```

Note: We add `scopes: any` to the output contract so the field is declared. We don't assert deeply on `scopes` structure in the `then` block because the process adapter's dot-path traversal into arrays (`scopes.0.name`) is not implemented — that would require array indexing in `extractPath`. The aggregate counts already validate correctness. The `scopes` field's presence in the JSON output is verified by the output contract declaring it.

**Step 2: Run self-verification test**

Run: `mise exec -- go test ./cmd/specrun/ -run TestSelfVerification -v -timeout 60s`
Expected: PASS

**Step 3: Commit**

```bash
mise exec -- go test ./...
git add specs/verify.spec
git commit -m "feat(spec): update verify spec with scopes output field"
```
