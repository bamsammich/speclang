spec CLITool {
  description: "CLI tool verification"

  scope parse_valid {
    use process
    config {
      command: "./specrun"
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
        file: "testdata/minimal.spec"
      }
      then {
        exit_code: 0
        name: "Minimal"
      }
    }
  }
}
