# Process adapter integration test fixture.
# Exercises: exec action with args; exit_code, stdout, stderr, dot-path assertions.
# Requires ECHO_TOOL_BIN to point to the built echo_tool binary.
spec ProcessAdapterTest {
  description: "Verifies all process adapter actions and assertion properties"

  process {
    command: env(ECHO_TOOL_BIN, "./echo_tool")
  }

  # exec with JSON stdout — verify exit_code and dot-path traversal
  scope process_json_stdout {
    action run(name: string) {
      let result = process.exec("greet", name)
      return result
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
      action: run
    }

    scenario greet_alice {
      given {
        name: "alice"
      }
      then {
        exit_code == 0
        greeting == "hello alice"
        name == "alice"
      }
    }
  }

  # exec with non-zero exit code
  scope process_exit_code {
    action run(code: string) {
      let result = process.exec("exit", code)
      return result
    }

    contract {
      input {
        code: string
      }
      output {
        exit_code: int
      }
      action: run
    }

    scenario exit_with_code {
      given {
        code: "3"
      }
      then {
        exit_code == 3
      }
    }
  }

  # exec with stderr output
  scope process_stderr {
    action run(message: string) {
      let result = process.exec("stderr", message)
      return result
    }

    contract {
      input {
        message: string
      }
      output {
        exit_code: int
        stderr: string
      }
      action: run
    }

    scenario stderr_output {
      given {
        message: "something went wrong"
      }
      then {
        exit_code == 1
        stderr == "something went wrong"
      }
    }
  }

  # exec with raw JSON passthrough
  scope process_raw_json {
    action run(payload: string) {
      let result = process.exec("json", payload)
      return result
    }

    contract {
      input {
        payload: string
      }
      output {
        exit_code: int
        key: string
      }
      action: run
    }

    scenario json_passthrough {
      given {
        payload: "{\"key\":\"value\"}"
      }
      then {
        exit_code == 0
        key == "value"
      }
    }
  }
}
