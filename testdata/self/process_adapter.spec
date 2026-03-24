# Process adapter integration test fixture.
# Exercises: exec action with args; exit_code, stdout, stderr, dot-path assertions.
# Requires ECHO_TOOL_BIN to point to the built echo_tool binary.
spec ProcessAdapterTest {
  description: "Verifies all process adapter actions and assertion properties"

  target {
    command: env(ECHO_TOOL_BIN, "./echo_tool")
  }

  # exec with JSON stdout — verify exit_code and dot-path traversal
  scope process_json_stdout {
    use process
    config {
      args: "greet"
    }

    contract {
      input {
        name: string
      }
      output {
        exit_code: int
        greeting: string
        name: string
      }
    }

    scenario greet_alice {
      given {
        name: "alice"
      }
      then {
        exit_code: 0
        greeting: "hello alice"
        name: "alice"
      }
    }
  }

  # exec with non-zero exit code
  scope process_exit_code {
    use process
    config {
      args: "exit"
    }

    contract {
      input {
        code: string
      }
      output {
        exit_code: int
      }
    }

    scenario exit_with_code {
      given {
        code: "3"
      }
      then {
        exit_code: 3
      }
    }
  }

  # exec with stderr output
  scope process_stderr {
    use process
    config {
      args: "stderr"
    }

    contract {
      input {
        message: string
      }
      output {
        exit_code: int
        stderr: string
      }
    }

    scenario stderr_output {
      given {
        message: "something went wrong"
      }
      then {
        exit_code: 1
        stderr: "something went wrong"
      }
    }
  }

  # exec with raw JSON passthrough
  scope process_raw_json {
    use process
    config {
      args: "json"
    }

    contract {
      input {
        payload: string
      }
      output {
        exit_code: int
        key: string
      }
    }

    scenario json_passthrough {
      given {
        payload: "{\"key\":\"value\"}"
      }
      then {
        exit_code: 0
        key: "value"
      }
    }
  }
}
