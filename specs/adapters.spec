# Verifies each adapter's integration fixtures pass end-to-end.
# Each scope uses the process adapter to run specrun verify on a fixture spec.

model AdapterVerifyResult {
  exit_code: int
  scenarios_run: int
  scenarios_passed: int
}

scope verify_http_adapter {
  contract VerifyHTTPAdapterContract(file: string) -> AdapterVerifyResult {
    action {
      let result = process.exec("verify", "--json", file)
      return result
    }

    scenario http_fixtures_pass {
      given {
        file: "testdata/self/http_adapter.spec"
      }
      then {
        output.exit_code == 0
        output.scenarios_run == 8
        output.scenarios_passed == 8
      }
    }
  }
}

scope verify_process_adapter {
  contract VerifyProcessAdapterContract(file: string) -> AdapterVerifyResult {
    action {
      let result = process.exec("verify", "--json", file)
      return result
    }

    scenario process_fixtures_pass {
      given {
        file: "testdata/self/process_adapter.spec"
      }
      then {
        output.exit_code == 0
        output.scenarios_run == 4
        output.scenarios_passed == 4
      }
    }
  }
}
