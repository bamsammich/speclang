# Verifies CLI flag behaviors: seeds, iterations, JSON output, error handling, flag positioning, help output.

model CLIExitResult {
  exit_code: int
}

model CLIAmountResult {
  exit_code: int
  amount: int
}

model CLIScopesResult {
  exit_code: int
  scopes: any
}

model CLIVerifyResult {
  exit_code: int
  scenarios_run: int
  scenarios_passed: int
  invariants_checked: int
  invariants_passed: int
}

# --help exits zero and is handled by urfave/cli.
scope cli_help {
  contract CLIHelpContract -> CLIExitResult {
    action {
      let result = process.exec("--help")
      return result
    }

    scenario help_exits_zero {
      given {}
      then {
        output.exit_code == 0
      }
    }
  }
}

scope cli_verify_help {
  contract CLIVerifyHelpContract -> CLIExitResult {
    action {
      let result = process.exec("verify", "--help")
      return result
    }

    scenario verify_help_exits_zero {
      given {}
      then {
        output.exit_code == 0
      }
    }
  }
}

# Different seeds produce different generated output.
# Seed 1 and seed 2 produce different amounts for the transfer scope.
scope generate_seed_1 {
  contract GenerateSeed1Contract -> CLIAmountResult {
    action {
      let result = process.exec("generate", "examples/transfer.spec", "--scope", "transfer", "--seed", "1")
      return result
    }

    scenario seed_1_output {
      given {}
      then {
        output.exit_code == 0
        output.amount == 791
      }
    }
  }
}

scope generate_seed_2 {
  contract GenerateSeed2Contract -> CLIAmountResult {
    action {
      let result = process.exec("generate", "examples/transfer.spec", "--scope", "transfer", "--seed", "2")
      return result
    }

    scenario seed_2_output {
      given {}
      then {
        output.exit_code == 0
        output.amount == 586
      }
    }
  }
}

# Iteration count is respected: --iterations controls inputs_run in JSON output.
# scopes.0.checks.3 is the first invariant ("conservation") in the transfer scope.
scope verify_iterations {
  contract VerifyIterationsContract -> CLIScopesResult {
    iterations: int
    file: string

    action {
      let result = process.exec("verify", "--json", "--iterations", iterations, file)
      return result
    }

    scenario iterations_10 {
      given {
        iterations: 10
        file: "examples/transfer.spec"
      }
      then {
        output.exit_code == 0
        output.scopes.0.checks.3.inputs_run == 10
      }
    }
  }
}

# JSON flag changes output format: verify --json produces parseable JSON with expected fields.
scope verify_json_output {
  contract VerifyJSONOutputContract -> CLIVerifyResult {
    action {
      let result = process.exec("verify", "--json", "examples/transfer.spec")
      return result
    }

    scenario json_output_fields {
      given {}
      then {
        output.exit_code == 0
        output.scenarios_run == 3
        output.scenarios_passed == 3
        output.invariants_checked == 3
        output.invariants_passed == 3
      }
    }
  }
}

# Unknown subcommand is rejected with exit code 1.
scope cli_unknown_command {
  contract CLIUnknownCommandContract -> CLIExitResult {
    action {
      let result = process.exec("unknown")
      return result
    }

    scenario unknown_rejected {
      given {}
      then {
        output.exit_code == 1
      }
    }
  }
}

# Missing required args: generate with no spec file exits with error.
scope cli_missing_args_generate {
  contract CLIMissingArgsGenerateContract -> CLIExitResult {
    action {
      let result = process.exec("generate")
      return result
    }

    scenario no_spec_file {
      given {}
      then {
        output.exit_code == 1
      }
    }
  }
}

# Missing required args: parse with no spec file exits with error.
scope cli_missing_args_parse {
  contract CLIMissingArgsParseContract -> CLIExitResult {
    action {
      let result = process.exec("parse")
      return result
    }

    scenario no_spec_file {
      given {}
      then {
        output.exit_code == 1
      }
    }
  }
}

# Flag position flexibility: flags before or after spec file produce same output.
# urfave/cli handles interspersed flags natively.
scope generate_flags_after {
  contract GenerateFlagsAfterContract -> CLIAmountResult {
    action {
      let result = process.exec("generate", "examples/transfer.spec", "--scope", "transfer", "--seed", "1")
      return result
    }

    scenario flags_after_spec {
      given {}
      then {
        output.exit_code == 0
        output.amount == 791
      }
    }
  }
}

scope generate_flags_before {
  contract GenerateFlagsBeforeContract -> CLIAmountResult {
    action {
      let result = process.exec("generate", "--scope", "transfer", "--seed", "1", "examples/transfer.spec")
      return result
    }

    scenario flags_before_spec {
      given {}
      then {
        output.exit_code == 0
        output.amount == 791
      }
    }
  }
}
