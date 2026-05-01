# Verifies that env() expressions work in config and given blocks.

model ExprParseResult {
  exit_code: int
  models: any
}

scope env_in_config {
  contract EnvInConfigContract(file: string) -> ExprParseResult {
    action {
      let result = process.exec(env(SPECTEST_EXPR_ARGS, "parse"), file)
      return result
    }

    # env() in adapter call args must evaluate to the fallback "parse" so the
    # command runs and returns the correct AST. The model name "EnvConfigResult"
    # confirms the right file was parsed — not just that the command exited 0.
    scenario parse_with_env_config {
      given {
        file: "testdata/self/env_config.spec"
      }
      then {
        output.exit_code == 0
        output.models.0.name == "EnvConfigResult"
      }
    }
  }
}

# Verifies that string concatenation with + works in then block assertions.
# The then block asserts output.models.0.name == "Enum" + "Result", which
# requires the runner to evaluate "Enum" + "Result" = "EnumResult" at
# assertion time. A runner that skips concat evaluation would compare
# output.models.0.name to the unevaluated literal and fail.
scope string_concat {
  contract StringConcatContract(file: string) -> ExprParseResult {
    action {
      let result = process.exec("parse", file)
      return result
    }

    scenario concat_in_then {
      given {
        file: "testdata/self/enum.spec"
      }
      then {
        output.exit_code == 0
        # If string concat is not evaluated at assertion time, this fails:
        output.models.0.name == "Enum" + "Result"
      }
    }
  }
}

# Verifies that array-form args in config blocks work correctly.
# Uses enum.spec (which has a known model) to confirm the parse
# actually ran against this specific file and returned its structure.
scope array_args {
  contract ArrayArgsContract(file: string) -> ExprParseResult {
    action {
      let result = process.exec("parse", file)
      return result
    }

    scenario parse_with_array_args {
      given {
        file: "testdata/self/enum.spec"
      }
      then {
        output.exit_code == 0
        output.models.0.name == "EnumResult"
      }
    }
  }
}

scope env_in_given {
  contract EnvInGivenContract(file: string) -> ExprParseResult {
    action {
      let result = process.exec("parse", file)
      return result
    }

    # env() in the given block must fall back to "testdata/self/env_given.spec"
    # when the env var is unset. The model name "EnvGivenResult" confirms the
    # fallback path was used and the right file was parsed.
    scenario parse_with_env_given {
      given {
        file: env(SPECTEST_NONEXISTENT_FILE, "testdata/self/env_given.spec")
      }
      then {
        output.exit_code == 0
        output.models.0.name == "EnvGivenResult"
      }
    }
  }
}
