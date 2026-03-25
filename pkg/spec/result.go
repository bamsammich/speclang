package spec

// Result captures the outcome of a verification run.
type Result struct {
	Spec              string        `json:"spec"`
	Scopes            []ScopeResult `json:"scopes"`
	Failures          []Failure     `json:"failures"`
	ScenariosRun      int           `json:"scenarios_run"`
	ScenariosPassed   int           `json:"scenarios_passed"`
	InvariantsChecked int           `json:"invariants_checked"`
	InvariantsPassed  int           `json:"invariants_passed"`
}

// Failure captures a single test failure with context for debugging.
type Failure struct {
	Name        string `json:"name"`
	Scope       string `json:"scope"`
	Input       any    `json:"input,omitempty"`
	Expected    any    `json:"expected,omitempty"`
	Actual      any    `json:"actual,omitempty"`
	Description string `json:"description"`
	Shrunk      bool   `json:"shrunk"`
}

// ScopeResult captures per-scope verification results.
type ScopeResult struct {
	Name   string        `json:"name"`
	Checks []CheckResult `json:"checks"`
}

// CheckResult captures the outcome of a single scenario or invariant.
type CheckResult struct {
	Failure   *Failure `json:"failure,omitempty"`
	Name      string   `json:"name"`
	Kind      string   `json:"kind"`                // "scenario" or "invariant"
	InputsRun int      `json:"inputs_run"`          // 1 for given-scenarios, N for when/invariants
	FailedAt  int      `json:"failed_at,omitempty"` // which input number failed (0 if passed)
	Passed    bool     `json:"passed"`
}
