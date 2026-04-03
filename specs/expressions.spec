# Verifies that env() expressions work in config and given blocks.
scope env_in_config {
  action run(file: string) {
    let result = process.exec(env(SPECTEST_EXPR_ARGS, "parse"), file)
    return result
  }

  contract {
    input {
      file: string
    }
    output {
      exit_code: int
      name: string
    }
    action: run
  }

  scenario parse_with_env_config {
    given {
      file: "testdata/self/env_config.spec"
    }
    then {
      exit_code == 0
      name == "EnvConfig"
    }
  }
}

# Verifies that string concatenation with + works in then block assertions.
scope string_concat {
  action run(file: string) {
    let result = process.exec("parse", file)
    return result
  }

  contract {
    input {
      file: string
    }
    output {
      exit_code: int
      name: string
    }
    action: run
  }

  scenario concat_in_then {
    given {
      file: "testdata/self/minimal.spec"
    }
    then {
      exit_code == 0
      name == "Mini" + "mal"
    }
  }
}

# Verifies that array-form args in config blocks work correctly.
scope array_args {
  action run(file: string) {
    let result = process.exec("parse", file)
    return result
  }

  contract {
    input {
      file: string
    }
    output {
      exit_code: int
      name: string
    }
    action: run
  }

  scenario parse_with_array_args {
    given {
      file: "testdata/self/minimal.spec"
    }
    then {
      exit_code == 0
      name == "Minimal"
    }
  }
}

scope env_in_given {
  action run(file: string) {
    let result = process.exec("parse", file)
    return result
  }

  contract {
    input {
      file: string
    }
    output {
      exit_code: int
      name: string
    }
    action: run
  }

  scenario parse_with_env_given {
    given {
      file: env(SPECTEST_NONEXISTENT_FILE, "testdata/self/env_given.spec")
    }
    then {
      exit_code == 0
      name == "EnvGiven"
    }
  }
}
