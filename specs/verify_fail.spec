# Verifies that specrun verify detects incorrect implementations.
scope verify_fail {
  action run(file: string) {
    let result = process.exec("verify", "--json", file)
    return result
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
    }
    action: run
  }

  # The broken server credits the to-account but never debits the from-account,
  # so the conservation invariant must fail.
  scenario broken_server_detected {
    given {
      file: "testdata/self/broken_transfer.spec"
    }
    then {
      exit_code == 1
      scenarios_run == 1
      scenarios_passed == 0
      invariants_checked == 1
      invariants_passed == 0
    }
  }
}
