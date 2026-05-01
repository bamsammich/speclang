# Verifies the exists() and has_key() functions parse and that the parser
# produces expected AST structure for specs containing these functions.

model ExistsParseResult {
  exit_code: int
}

scope parse_exists {
  contract ParseExistsContract(file: string) -> ExistsParseResult {
    action {
      let result = process.exec("parse", file)
      return result
    }

    # Spec containing exists() should parse successfully and produce a model
    # named "ExistsResult" plus a scope named "test" — proving the parser
    # extracted the correct structure, not just that the command didn't crash.
    scenario exists_spec {
      given {
        file: "testdata/self/exists_function.spec"
      }
      then {
        output.exit_code == 0
        output.models.0.name == "ExistsResult"
        output.scopes.0.name == "test"
      }
    }

    # Spec containing has_key() should parse successfully and produce a model
    # named "HasKeyResult" plus a scope named "test".
    scenario has_key_spec {
      given {
        file: "testdata/self/has_key_function.spec"
      }
      then {
        output.exit_code == 0
        output.models.0.name == "HasKeyResult"
        output.scopes.0.name == "test"
      }
    }
  }
}
