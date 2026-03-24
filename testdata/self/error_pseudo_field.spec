# Test fixture for the error pseudo-field.
# Uses "error" in then block without it being in the contract output.
spec ErrorPseudoFieldTest {
  target {
    command: "echo"
  }

  scope test_error {
    use process
    config {
      args: "hello"
    }

    contract {
      input {}
      output {
        exit_code: int
      }
    }

    # error is not in the output contract — it's the pseudo-field.
    # Since the process adapter always returns {ok: true}, error: null should pass.
    scenario no_error_expected {
      given {}
      then {
        exit_code: 0
        error: null
      }
    }
  }
}
