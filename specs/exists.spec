# Verifies the exists() and has_key() functions parse and that the parser
# produces expected AST structure for specs containing these functions.
scope parse_exists {
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

  # Spec containing exists() should parse successfully.
  scenario exists_spec {
    given {
      file: "testdata/self/exists_function.spec"
    }
    then {
      exit_code == 0
      name == "ExistsTest"
    }
  }

  # Spec containing has_key() should parse successfully.
  scenario has_key_spec {
    given {
      file: "testdata/self/has_key_function.spec"
    }
    then {
      exit_code == 0
      name == "HasKeyTest"
    }
  }
}
