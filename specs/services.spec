# Verifies that the target services lifecycle works end-to-end.

scope verify_service_lifecycle {
  use process
  config {
    args: "verify --json testdata/self/services.spec"
  }

  contract {
    input {}
    output {
      exit_code: int
      scenarios_run: int
      scenarios_passed: int
    }
  }

  scenario services_start_and_verify {
    given {}
    then {
      exit_code: 0
      scenarios_run: 1
      scenarios_passed: 1
    }
  }
}

scope parse_service_ref {
  use process
  config {
    args: "parse testdata/self/services.spec"
  }

  contract {
    input {}
    output {
      exit_code: int
      name: string
    }
  }

  scenario service_spec_parses {
    given {}
    then {
      exit_code: 0
      name: "ServiceTest"
    }
  }
}
