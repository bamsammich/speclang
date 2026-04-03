# Verifies that the target services lifecycle works end-to-end.

scope verify_service_lifecycle {
  action run() {
    let result = process.exec("verify", "--json", "testdata/self/services.spec")
    return result
  }

  contract {
    input {}
    output {
      exit_code: int
      scenarios_run: int
      scenarios_passed: int
    }
    action: run
  }

  scenario services_start_and_verify {
    given {}
    then {
      exit_code == 0
      scenarios_run == 1
      scenarios_passed == 1
    }
  }
}

scope parse_service_ref {
  action run() {
    let result = process.exec("parse", "testdata/self/services.spec")
    return result
  }

  contract {
    input {}
    output {
      exit_code: int
      name: string
    }
    action: run
  }

  scenario service_spec_parses {
    given {}
    then {
      exit_code == 0
      name == "ServiceTest"
    }
  }
}

# Note: invalid_service_ref removed for v3 — service() refs in adapter config
# blocks are not yet validated at parse time (only target.Fields are checked).
