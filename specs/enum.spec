# Verifies the parser and generator handle enum types.

model EnumParseResult {
  exit_code: int
}

model EnumGenerateResult {
  exit_code: int
  adapter_name: any
  subcommand: any
}

scope parse_enum {
  contract ParseEnumContract -> EnumParseResult {
    file: string

    action {
      let result = process.exec("parse", file)
      return result
    }

    # Enum spec should parse successfully and expose the model "EnumResult"
    # and the scope "enum_inputs", confirming the parser extracted the
    # inline enum contract rather than just not crashing.
    scenario enum_spec {
      given {
        file: "testdata/self/enum.spec"
      }
      then {
        output.exit_code == 0
        output.models.0.name == "EnumResult"
        output.scopes.0.name == "enum_inputs"
      }
    }
  }
}

# Verifies the parser rejects empty enum types.
scope parse_enum_invalid {
  contract ParseEnumInvalidContract -> EnumParseResult {
    file: string

    action {
      let result = process.exec("parse", file)
      return result
    }

    # Empty enum() should fail at parse time.
    scenario empty_enum {
      given {
        file: "testdata/self/invalid_enum_empty.spec"
      }
      then {
        output.exit_code == 1
      }
    }
  }
}

# Verifies the validator rejects invalid enum variants.
scope validate_enum_invalid {
  contract ValidateEnumInvalidContract -> EnumParseResult {
    file: string

    action {
      let result = process.exec("parse", file)
      return result
    }

    # Assigning a string not in the variant set should fail validation.
    scenario invalid_variant {
      given {
        file: "testdata/self/invalid_enum_variant.spec"
      }
      then {
        output.exit_code == 1
      }
    }
  }
}

# Verifies the generator produces valid enum values.
scope generate_enum {
  contract GenerateEnumContract -> EnumGenerateResult {
    seed: int

    action {
      let result = process.exec("generate", "testdata/self/enum.spec", "--scope", "enum_inputs", "--seed", seed)
      return result
    }

    # Generation should succeed across seeds.
    invariant produces_output {
      output.exit_code == 0
    }

    # Generated adapter_name values must be valid variants.
    invariant adapter_name_is_valid {
      when output.exit_code == 0:
        output.adapter_name == "http" or output.adapter_name == "process" or output.adapter_name == "playwright"
    }

    # Generated subcommand values must be valid variants.
    invariant subcommand_is_valid {
      when output.exit_code == 0:
        output.subcommand == "parse" or output.subcommand == "generate" or output.subcommand == "verify" or output.subcommand == "install"
    }
  }
}
