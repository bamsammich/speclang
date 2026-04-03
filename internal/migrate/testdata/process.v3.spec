spec CLITool {
  description: "CLI tool verification"

  scope parse_valid {
    action parse_valid(file: string) {
      let result = process.exec(file)
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
      action: parse_valid
    }

    scenario minimal_spec {
      given {
        file: "testdata/minimal.spec"
      }
      then {
        exit_code == 0
        name == "Minimal"
      }
    }
  }
}
