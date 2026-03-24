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

Specs are validated automatically before verification runs. Validation checks model resolution, type correctness, and field completeness. If validation fails, `specrun` exits with code 1 and prints validation errors — verification does not proceed. Fix the spec and run verify again.

### 1. Find the spec files

```bash
find . -name "*.spec" -not -path "*/testdata/*"
```

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

### 2c. For specs with target services, ensure Docker is available

If the spec declares `services` in the `target` block, Docker must be running. `specrun verify` will manage containers automatically. If Docker is unavailable, set `SPECRUN_NO_SERVICES=1` and start servers manually.

### 3. Run verification

```bash
specrun verify path/to/spec.spec
```

If the spec declares services, containers will start automatically before verification and stop after. Use `--keep-services` to leave containers running for debugging. Set `SPECRUN_NO_SERVICES=1` to skip container management entirely.

For multiple spec files, verify each one. Start with the most relevant spec for the changes made.

### 4. Interpret results

**All passing:**
```
  scope transfer:
    ✓ scenario success
    ✓ scenario overdraft (10 inputs)
    ✓ invariant conservation (10 inputs)

Scenarios:  3/3 passed
Invariants: 3/3 passed
```

Proceed with merge/PR.

**Failures:**
```
  scope transfer:
    ✗ invariant conservation (failed on input 3/10, shrunk)
        input:
          {"from": {"balance": 50}, "amount": 25}
        expected: 100
        actual:   75

Invariants: 0/1 passed
```

Do NOT proceed. Fix the implementation to satisfy the spec. The spec defines correct behavior — if the implementation disagrees, the implementation is wrong.

### 5. For JSON output

Use `--json` flag for programmatic consumption:

```bash
specrun verify --json path/to/spec.spec
```

## Key Rules

- **Never skip verification** to save time. The spec exists to catch exactly the bugs you think you don't have.
- **Never modify the spec to match a broken implementation.** If the spec is wrong, that's a separate conversation with the user.
- **Verification must pass before any of**: committing final work, creating a PR, merging a branch, or claiming completion.
- **If the specrun binary needs environment variables** (like `APP_URL` or `SPECRUN_BIN`), set them before running.
