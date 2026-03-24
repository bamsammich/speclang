# Verifies the parser and generator handle enum types.
scope parse_enum {
  use process
  config {
    args: "parse"
  }

  contract {
    input {
      file: string
    }
    output {
      exit_code: int
      name: string
    }
  }

  # Enum spec should parse successfully.
  scenario enum_spec {
    given {
      file: "testdata/self/enum.spec"
    }
    then {
      exit_code: 0
      name: "EnumTest"
    }
  }
}

# Verifies the parser rejects empty enum types.
scope parse_enum_invalid {
  use process
  config {
    args: "parse"
  }

  contract {
    input {
      file: string
    }
    output {
      exit_code: int
    }
  }

  # Empty enum() should fail at parse time.
  scenario empty_enum {
    given {
      file: "testdata/self/invalid_enum_empty.spec"
    }
    then {
      exit_code: 1
    }
  }
}

# Verifies the validator rejects invalid enum variants.
scope validate_enum_invalid {
  use process
  config {
    args: "parse"
  }

  contract {
    input {
      file: string
    }
    output {
      exit_code: int
    }
  }

  # Assigning a string not in the variant set should fail validation.
  scenario invalid_variant {
    given {
      file: "testdata/self/invalid_enum_variant.spec"
    }
    then {
      exit_code: 1
    }
  }
}

# Verifies the generator produces valid enum values.
scope generate_enum {
  use process
  config {
    args: "generate testdata/self/enum.spec --scope enum_inputs --seed"
  }

  contract {
    input {
      seed: int
    }
    output {
      exit_code: int
      adapter_name: any
      subcommand: any
    }
  }

  # Generation should succeed across seeds.
  invariant produces_output {
    exit_code == 0
  }

  # Generated adapter_name values must be valid variants.
  invariant adapter_name_is_valid {
    when exit_code == 0:
      output.adapter_name == "http" || output.adapter_name == "process" || output.adapter_name == "playwright"
  }

  # Generated subcommand values must be valid variants.
  invariant subcommand_is_valid {
    when exit_code == 0:
      output.subcommand == "parse" || output.subcommand == "generate" || output.subcommand == "verify" || output.subcommand == "install"
  }
}
