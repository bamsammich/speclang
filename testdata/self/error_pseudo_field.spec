# Test fixture for the error pseudo-field.
# Uses "error" in then block without it being in the contract output.
process {
  command: "echo"
}

model ErrorPseudoResult {
  exit_code: int
}

scope test_error {
  contract ErrorPseudoContract() -> ErrorPseudoResult {
    action {
      let result = process.exec("hello")
      return result
    }

    # error is not in the output contract — it's the pseudo-field.
    # Since the process adapter always returns {ok: true}, error == null should pass.
    scenario no_error_expected {
      given {}
      then {
        output.exit_code == 0
        error == null
      }
    }
  }
}
