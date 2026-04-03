# Test fixture for the error pseudo-field.
# Uses "error" in then block without it being in the contract output.
spec ErrorPseudoFieldTest {
  process {
    command: "echo"
  }

  scope test_error {
    action run() {
      let result = process.exec("hello")
      return result
    }

    contract {
      input {}
      output {
        exit_code: int
      }
      action: run
    }

    # error is not in the output contract — it's the pseudo-field.
    # Since the process adapter always returns {ok: true}, error == null should pass.
    scenario no_error_expected {
      given {}
      then {
        exit_code == 0
        error == null
      }
    }
  }
}
