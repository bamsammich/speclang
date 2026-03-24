# Verifies that specrun verify passes correct implementations.
scope verify_pass {
  use process
  config {
    args: "verify --json --no-services"
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

  # End-to-end: the transfer example must pass all checks.
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

  # Every scope in the verify JSON has a non-empty name.
  invariant all_scopes_have_names {
    when exit_code == 0:
      all(output.scopes, s => s.name != "")
  }

  # Every check across all scopes passes.
  invariant all_checks_pass {
    when exit_code == 0:
      all(output.scopes, s => all(s.checks, c => c.passed == true))
  }
}
