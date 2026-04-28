---
name: verify
description: "Use after implementation is complete and before merging, committing final work, or creating PRs in a project that has speclang specs (.spec files) — run specrun verify against the project's spec files to confirm the implementation satisfies its specification."
---

# Speclang Verification Gate

Run `specrun verify` to confirm the implementation satisfies its spec before merging.

## When to Run

- After finishing implementation of a feature that has a `.spec` file
- Before creating a PR, merging a branch, or claiming work is complete
- After fixing a bug that a spec covers

## Process

Specs are validated automatically before verification runs. Validation checks model resolution, type correctness, `output.` field references, and input completeness. If validation fails, `specrun` exits with code 1 and prints validation errors — verification does not proceed. Fix the spec/code and run verify again.

### 1. Find the spec files

```bash
find . -name "*.spec" -not -path "*/testdata/*"
```

**Prefer a glob over running one file at a time.** Always verify the full `specs/` directory when it exists — new spec files won't be caught if you only verify the root spec.

### 2. Ensure specrun is available

```bash
# If specrun is in the project
go build -o ./specrun ./cmd/specrun

# If specrun is installed globally
which specrun
```

### 2b. For playwright specs, ensure browsers are installed

```bash
specrun install playwright
```

### 2c. For specs with services, ensure Docker is available

If the spec declares a `services` block, Docker must be running. `specrun verify` will manage containers automatically.

### 3. Run verification

```bash
# Single file
specrun verify path/to/spec.spec

# Glob — preferred for project-wide gate
specrun verify specs/*.spec

# Recursive glob
specrun verify specs/**/*.spec
```

**Glob behavior:**
- Each matched file is verified independently as its own spec unit
- Files with no contracts log `<filename>: no contracts, skipping` and exit 0 (not an error)
- A glob matching zero files exits 1 (error)
- Included files that are also matched by the glob are verified independently too — this is correct behavior; each file is its own spec

If the spec declares services, containers start automatically before verification and stop after. Use `--keep-services` to leave containers running for debugging.

### 4. Interpret results

**All passing (human output):**
```
Verifying transfer.spec (seed=42, iterations=100)

  scope transfer:
    ✓ scenario success
    ✓ scenario overdraft (100 inputs)
    ✓ scenario zero_transfer (100 inputs)
    ✓ invariant conservation (100 inputs)
    ✓ invariant non_negative (100 inputs)
    ✓ invariant no_mutation_on_error (100 inputs)

Scenarios:  3/3 passed
Invariants: 3/3 passed
```

Proceed with merge/PR.

**Scope-level failure (invariant):**
```
  scope transfer:
    ✗ invariant conservation (failed on input 3/100, shrunk)
        description: output.from.balance + output.to.balance != from.balance + to.balance
        expected: 150
        actual: 145
        input:
          from: {balance: 100}
          to: {balance: 50}
          amount: 5
```

The `input` block shows the minimal shrunk counterexample. The system failed on this input.

**Scenario-level failure:**
```
  scope transfer:
    ✗ scenario success (failed)
        description: output.from.balance expected 70, got 100
        expected: 70
        actual: 100
        input:
          from: {id: "alice", balance: 100}
          to: {id: "bob", balance: 50}
          amount: 30
```

**Do NOT proceed on any failure.** Fix the implementation to satisfy the spec. The spec defines correct behavior — if the implementation disagrees, the implementation is wrong. If the spec is wrong, that is a separate conversation with the user.

### 5. Interpreting scope vs. invariant vs. scenario failures

| Failure type | Meaning |
|-------------|---------|
| Scope-level parse/validation error | Spec file has a syntax or type error — fix the spec |
| `invariant X (failed on input N/M, shrunk)` | The universal law broke. The shrunk input is the minimal reproducer. The implementation violates a property. |
| `scenario X (failed)` | A concrete fixed input produced wrong output. Usually a specific code path or condition. |
| `scenario X (failed on input N/M)` | A `when`-scenario generated N inputs; one failed. The input shown is the one that broke. |

Counterexample shrinking is automatic. The `input` shown in failures is already minimized — use it directly in a unit test or debugging session.

### 6. For JSON output

Use `--json` flag for programmatic consumption (CI, scripts):

```bash
specrun verify --json path/to/spec.spec
specrun verify --json specs/*.spec
```

JSON shape:
```json
{
  "spec": "transfer.spec",
  "scopes": [
    {
      "name": "transfer",
      "checks": [
        {
          "kind": "invariant",
          "name": "conservation",
          "passed": true,
          "inputs_run": 100
        },
        {
          "kind": "scenario",
          "name": "success",
          "passed": false,
          "failed_at": 1,
          "inputs_run": 1,
          "failure": {
            "description": "...",
            "expected": 70,
            "actual": 100,
            "input": { ... }
          }
        }
      ]
    }
  ],
  "scenarios_run": 3,
  "scenarios_passed": 2,
  "invariants_checked": 3,
  "invariants_passed": 3,
  "failures": [...]
}
```

`failures` is non-empty when any check failed. Exit code is 1.

## Key Rules

- **Never skip verification** to save time. The spec exists to catch exactly the bugs you think you don't have.
- **Never modify the spec to match a broken implementation.** If the spec is wrong, that's a separate conversation with the user.
- **Verification must pass before any of**: committing final work, creating a PR, merging a branch, or claiming completion.
- **Always use a glob that covers `specs/*.spec`**, not just the root spec — new spec files won't be caught otherwise.
- **If the specrun binary needs environment variables** (like `APP_URL` or `SPECRUN_BIN`), set them before running.
- **For self-verification** (speclang verifying itself): `SPECRUN_BIN=./specrun ./specrun verify specs/speclang.spec`
