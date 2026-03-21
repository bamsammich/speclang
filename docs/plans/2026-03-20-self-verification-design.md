# Self-Verification Design

**Date:** 2026-03-20
**Status:** Approved

## Goal

Speclang verifies itself with its own specs. This is black-box verification through the runtime — speclang is both the verifier and the system under test.

## Adapter: Process Adapter

A new adapter that invokes a subprocess, captures stdout/stderr/exit code, and asserts against the result. Mirrors the HTTP adapter's pattern exactly.

### Config (from scope `config` block)

- `command` — binary to run (required)
- `args` — base arguments prepended to every exec call (optional)

### Action: `exec`

Runs `command [...args] [...action_args]`. Captures:
- `exit_code` (int)
- `stdout` (parsed as JSON best-effort, like HTTP adapter does with response bodies)
- `stderr` (raw string)

### Assertions

Check properties of the last execution result:
- `exit_code` — integer comparison
- `stdout` — full parsed JSON body
- `stdout.field.path` — dot-path traversal into parsed JSON (reuses `extractPath` logic)
- `stderr` — raw string

### Implementation

- `pkg/adapter/process.go` — ProcessAdapter struct, mirrors HTTPAdapter
- `plugins/process.plugin` — plugin definition
- Registered as built-in adapter (like HTTP), no subprocess IPC for the adapter itself

## CLI Commands

Three additions to make speclang's internals observable:

### `specrun parse <file>`

Parses the spec (including include resolution), outputs the AST as JSON to stdout.
- Exit 0: JSON AST on stdout (`{"name": "...", "uses": [...], "models": [...], "scopes": [...]}`)
- Exit 1: error on stderr

### `specrun generate <file> --scope <name> --seed <N>`

Generates one input from the named scope's contract.
- Exit 0: generated input as JSON on stdout
- Exit 1: scope not found or generation fails

### `specrun verify <file> [--json]`

Existing command gains `--json` flag for structured output.
- JSON output: `{"spec": "...", "scenarios_run": N, "scenarios_passed": N, "invariants_checked": N, "invariants_passed": N, "failures": [...]}`
- Exit codes unchanged (0 pass, 1 fail)

## Self-Verification Spec

### File Structure

```
specs/
├── speclang.spec           # root: use process, target, includes
├── parse.spec              # parse_valid + parse_invalid scopes
├── generate.spec           # generator constraint satisfaction
└── verify.spec             # verify_pass + verify_fail scopes
```

### Root Spec

```
use process

spec Speclang {

  target {
    command: env(SPECRUN_BIN, "./specrun")
  }

  include "parse.spec"
  include "generate.spec"
  include "verify.spec"
}
```

### Scope 1: parse_valid

Verifies the parser accepts valid specs and produces expected AST structure.

```
scope parse_valid {
  config {
    command: "specrun"
    args: ["parse"]
  }

  contract {
    input {
      file: string
    }
    output {
      exit_code: int
      stdout: any
    }
  }

  scenario minimal_spec {
    given {
      file: "testdata/self/minimal.spec"
    }
    then {
      exit_code: 0
      stdout.name: "Minimal"
    }
  }

  invariant parse_succeeds {
    exit_code == 0
  }
}
```

### Scope 2: parse_invalid

Verifies the parser rejects malformed specs.

```
scope parse_invalid {
  config {
    command: "specrun"
    args: ["parse"]
  }

  scenario unterminated_spec {
    given {
      file: "testdata/self/invalid_unterminated.spec"
    }
    then {
      exit_code: 1
    }
  }

  scenario circular_include {
    given {
      file: "testdata/include/circular/a.spec"
    }
    then {
      exit_code: 1
    }
  }
}
```

### Scope 3: generate

Verifies the generator always produces constraint-satisfying outputs. The runtime generates different seed values, so this checks the generator across many seeds.

```
scope generate {
  config {
    command: "specrun"
    args: ["generate", "--scope", "transfer"]
  }

  contract {
    input {
      file: string
      seed: int
    }
    output {
      exit_code: int
      stdout: any
    }
  }

  invariant constraints_satisfied {
    when exit_code == 0:
      stdout.amount > 0
      stdout.amount <= stdout.from.balance
  }

  invariant produces_output {
    exit_code == 0
  }
}
```

### Scope 4: verify_pass

Verifies that `specrun verify` passes correct implementations.

```
scope verify_pass {
  config {
    command: "specrun"
    args: ["verify", "--json"]
  }

  contract {
    input {
      file: string
      seed: int
      iterations: int
    }
    output {
      exit_code: int
      stdout: any
    }
  }

  scenario transfer_spec_passes {
    given {
      file: "examples/transfer.spec"
      seed: 42
      iterations: 50
    }
    then {
      exit_code: 0
      stdout.scenarios_run: 3
      stdout.scenarios_passed: 3
      stdout.invariants_checked: 3
      stdout.invariants_passed: 3
    }
  }

  invariant no_failures_means_exit_zero {
    when stdout.failures == null:
      exit_code == 0
  }
}
```

### Scope 5: verify_fail

Verifies that `specrun verify` catches broken implementations.

```
scope verify_fail {
  config {
    command: "specrun"
    args: ["verify", "--json"]
  }

  scenario broken_server_fails {
    given {
      file: "testdata/self/broken_transfer.spec"
      seed: 42
      iterations: 10
    }
    then {
      exit_code: 1
    }
  }

  invariant failures_means_exit_one {
    when stdout.failures != null:
      exit_code == 1
  }
}
```

## Test Fixtures

- `testdata/self/minimal.spec` — smallest valid spec (e.g., `spec Minimal {}`)
- `testdata/self/invalid_unterminated.spec` — deliberately malformed (`spec Bad {`)
- `testdata/self/broken_transfer.spec` — valid spec pointing at a broken server
- `testdata/self/server/main.go` — deliberately broken HTTP server (e.g., doesn't debit from-account, so conservation invariant fails)

## Implementation Phases

### Phase 1: CLI commands

Add `parse` and `generate` subcommands, add `--json` to `verify`. Testable with Go unit tests. No new adapter.

### Phase 2: Process adapter

`pkg/adapter/process.go` following HTTP adapter pattern. `plugins/process.plugin` definition. Wire into plugin registry. Test with simple subprocess.

### Phase 3: Self-verification spec

Write `specs/speclang.spec` with includes. Create test fixtures and broken server. Run `specrun verify specs/speclang.spec`.

Phase 3 is the payoff; phases 1 and 2 are independently valuable (JSON CLI output, reusable process adapter).

## Dependencies

- The `verify_pass` and `verify_fail` scopes require a running HTTP server for the transfer spec. The test setup needs to build and start the broken server before verification.
- `target.command` uses `env(SPECRUN_BIN, "./specrun")` so CI can point at the built binary.
