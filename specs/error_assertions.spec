# Verifies that the error pseudo-field works in specs.

# Verifies the parser accepts specs with error pseudo-field in then blocks.
scope parse_error_pseudo_field {
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

  scenario error_pseudo_field_parses {
    given {
      file: "testdata/self/error_pseudo_field.spec"
    }
    then {
      exit_code: 0
      name: "ErrorPseudoFieldTest"
    }
  }
}

# Verifies specrun verify passes on a spec using the error pseudo-field.
scope verify_error_pseudo_field {
  use process
  config {
    args: "verify --json"
  }

  contract {
    input {
      file: string
    }
    output {
      exit_code: int
      scenarios_run: int
      scenarios_passed: int
    }
  }

  scenario error_pseudo_field_passes {
    given {
      file: "testdata/self/error_pseudo_field.spec"
    }
    then {
      exit_code: 0
      scenarios_run: 1
      scenarios_passed: 1
    }
  }
}
