# Process adapter integration test fixture.
# Exercises: exec action with args; exit_code, stdout, stderr, dot-path assertions.
# Requires ECHO_TOOL_BIN to point to the built echo_tool binary.

process {
  command: env(ECHO_TOOL_BIN, "./echo_tool")
}

model ProcessGreetResult {
  exit_code: int
  greeting: string
  name: string
}

model ProcessExitResult {
  exit_code: int
}

model ProcessStderrResult {
  exit_code: int
  stderr: string
}

model ProcessJSONResult {
  exit_code: int
  key: string
}

# exec with JSON stdout — verify exit_code and dot-path traversal
scope process_json_stdout {
  contract ProcessJSONStdoutContract -> ProcessGreetResult {
    name: string

    action {
      let result = process.exec("greet", name)
      return result
    }

    scenario greet_alice {
      given {
        name: "alice"
      }
      then {
        output.exit_code == 0
        output.greeting == "hello alice"
        output.name == "alice"
      }
    }
  }
}

# exec with non-zero exit code
scope process_exit_code {
  contract ProcessExitCodeContract -> ProcessExitResult {
    code: string

    action {
      let result = process.exec("exit", code)
      return result
    }

    scenario exit_with_code {
      given {
        code: "3"
      }
      then {
        output.exit_code == 3
      }
    }
  }
}

# exec with stderr output
scope process_stderr {
  contract ProcessStderrContract -> ProcessStderrResult {
    message: string

    action {
      let result = process.exec("stderr", message)
      return result
    }

    scenario stderr_output {
      given {
        message: "something went wrong"
      }
      then {
        output.exit_code == 1
        output.stderr == "something went wrong"
      }
    }
  }
}

# exec with raw JSON passthrough
scope process_raw_json {
  contract ProcessRawJSONContract -> ProcessJSONResult {
    payload: string

    action {
      let result = process.exec("json", payload)
      return result
    }

    scenario json_passthrough {
      given {
        payload: "{\"key\":\"value\"}"
      }
      then {
        output.exit_code == 0
        output.key == "value"
      }
    }
  }
}
