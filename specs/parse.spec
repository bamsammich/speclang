# Verifies the parser accepts valid specs and produces expected AST structure.
scope parse_valid {
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

  scenario minimal_spec {
    given {
      file: "testdata/self/minimal.spec"
    }
    then {
      exit_code: 0
      name: "Minimal"
    }
  }

  scenario transfer_spec {
    given {
      file: "examples/transfer.spec"
    }
    then {
      exit_code: 0
      name: "AccountAPI"
    }
  }
}

# Verifies the parser rejects malformed specs with a non-zero exit code.
scope parse_invalid {
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

  scenario unterminated_spec {
    given {
      file: "testdata/self/invalid_unterminated.spec"
    }
    then {
      exit_code: 1
    }
  }

  scenario circular_include {
    given {
      file: "testdata/include/circular/a.spec"
    }
    then {
      exit_code: 1
    }
  }

  # Import with no resolver registered should fail gracefully.
  scenario import_no_resolver {
    given {
      file: "testdata/openapi/import_no_resolver.spec"
    }
    then {
      exit_code: 1
    }
  }

  # Import with bad syntax (missing parens) should fail.
  scenario import_bad_syntax {
    given {
      file: "testdata/openapi/import_bad_syntax.spec"
    }
    then {
      exit_code: 1
    }
  }
}
