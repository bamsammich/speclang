# Verifies that specrun verify detects incorrect implementations.

model VerifyFailResult {
  exit_code: int
  scenarios_run: int
  scenarios_passed: int
  invariants_checked: int
  invariants_passed: int
}

scope verify_fail {
  contract VerifyFailContract -> VerifyFailResult {
    file: string

    action {
      let result = process.exec("verify", "--json", file)
      return result
    }

    # The broken server credits the to-account but never debits the from-account,
    # so the conservation invariant must fail.
    scenario broken_server_detected {
      given {
        file: "testdata/self/broken_transfer.spec"
      }
      then {
        output.exit_code == 1
        output.scenarios_run == 1
        output.scenarios_passed == 0
        output.invariants_checked == 1
        output.invariants_passed == 0
      }
    }
  }
}
