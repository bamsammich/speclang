# Verifies that specrun verify passes correct implementations.
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
}
