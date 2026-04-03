# Verifies each adapter's integration fixtures pass end-to-end.
# Each scope uses the process adapter to run specrun verify on a fixture spec.

scope verify_http_adapter {
  action run(file: string) {
    let result = process.exec("verify", "--json", file)
    return result
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
    action: run
  }

  scenario http_fixtures_pass {
    given {
      file: "testdata/self/http_adapter.spec"
    }
    then {
      exit_code == 0
      scenarios_run == 8
      scenarios_passed == 8
    }
  }
}

scope verify_process_adapter {
  action run(file: string) {
    let result = process.exec("verify", "--json", file)
    return result
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
    action: run
  }

  scenario process_fixtures_pass {
    given {
      file: "testdata/self/process_adapter.spec"
    }
    then {
      exit_code == 0
      scenarios_run == 4
      scenarios_passed == 4
    }
  }
}
