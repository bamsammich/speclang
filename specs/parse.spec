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
}
