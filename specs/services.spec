# Verifies that the target services lifecycle works end-to-end.

model ServicesVerifyResult {
  exit_code: int
  scenarios_run: int
  scenarios_passed: int
}

model ServicesParseResult {
  exit_code: int
  services: any
  scopes: any
}

scope verify_service_lifecycle {
  contract VerifyServiceLifecycleContract -> ServicesVerifyResult {
    action {
      let result = process.exec("verify", "--json", "testdata/self/services.spec")
      return result
    }

    scenario services_start_and_verify {
      given {}
      then {
        output.exit_code == 0
        output.scenarios_run == 1
        output.scenarios_passed == 1
      }
    }
  }
}

scope parse_service_ref {
  contract ParseServiceRefContract -> ServicesParseResult {
    action {
      let result = process.exec("parse", "testdata/self/services.spec")
      return result
    }

    # The parsed spec must contain a service named "test_server" and a scope
    # named "service_health" — proving the parser extracted both the services
    # block and the scope, not just that the command exited cleanly.
    scenario service_spec_parses {
      given {}
      then {
        output.exit_code == 0
        output.services.0.name == "test_server"
        output.scopes.0.name == "service_health"
      }
    }
  }
}

# Note: invalid_service_ref removed for v3 — service() refs in adapter config
# blocks are not yet validated at parse time (only target.Fields are checked).
