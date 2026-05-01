# Verifies that the error pseudo-field works in specs.

model ErrorParseResult {
  exit_code: int
}

model ErrorVerifyResult {
  exit_code: int
  scenarios_run: int
  scenarios_passed: int
}

# Verifies the parser accepts specs with error pseudo-field in then blocks.
scope parse_error_pseudo_field {
  contract ParseErrorPseudoFieldContract(file: string) -> ErrorParseResult {
    action {
      let result = process.exec("parse", file)
      return result
    }

    # The parsed spec must include the "ErrorPseudoResult" model and the
    # "test_error" scope — proving the parser handled the error pseudo-field
    # syntax rather than just returning exit_code 0 on an empty parse.
    scenario error_pseudo_field_parses {
      given {
        file: "testdata/self/error_pseudo_field.spec"
      }
      then {
        output.exit_code == 0
        output.models.0.name == "ErrorPseudoResult"
        output.scopes.0.name == "test_error"
      }
    }
  }
}

# Verifies specrun verify passes on a spec using the error pseudo-field.
scope verify_error_pseudo_field {
  contract VerifyErrorPseudoFieldContract(file: string) -> ErrorVerifyResult {
    action {
      let result = process.exec("verify", "--json", file)
      return result
    }

    scenario error_pseudo_field_passes {
      given {
        file: "testdata/self/error_pseudo_field.spec"
      }
      then {
        output.exit_code == 0
        output.scenarios_run == 1
        output.scenarios_passed == 1
      }
    }
  }
}
