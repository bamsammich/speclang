# Verifies that env() expressions work in config and given blocks.
scope env_in_config {
  use process
  config {
    args: env(SPECTEST_EXPR_ARGS, "parse")
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

  scenario parse_with_env_config {
    given {
      file: "testdata/self/env_config.spec"
    }
    then {
      exit_code: 0
      name: "EnvConfig"
    }
  }
}

scope env_in_given {
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

  scenario parse_with_env_given {
    given {
      file: env(SPECTEST_NONEXISTENT_FILE, "testdata/self/env_given.spec")
    }
    then {
      exit_code: 0
      name: "EnvGiven"
    }
  }
}
