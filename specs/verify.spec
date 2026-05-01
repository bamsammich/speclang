# Verifies that specrun verify passes correct implementations.

model VerifyResult {
  exit_code: int
  scenarios_run: int
  scenarios_passed: int
  invariants_checked: int
  invariants_passed: int
  scopes: any
}

scope verify_pass {
  contract VerifyPassContract(file: string) -> VerifyResult {
    action {
      let result = process.exec("verify", "--json", file)
      return result
    }

    # End-to-end: the transfer example must pass all checks.
    scenario transfer_spec_passes {
      given {
        file: "examples/transfer.spec"
      }
      then {
        output.exit_code == 0
        output.scenarios_run == 3
        output.scenarios_passed == 3
        output.invariants_checked == 3
        output.invariants_passed == 3
      }
    }

    # Every scope in the verify JSON has a non-empty name.
    invariant all_scopes_have_names {
      when output.exit_code == 0:
        all(output.scopes, s => s.name != "")
    }

    # Every check across all scopes passes.
    invariant all_checks_pass {
      when output.exit_code == 0:
        all(output.scopes, s => all(s.checks, c => c.passed == true))
    }
  }
}
