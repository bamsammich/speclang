# Verifies CLI flag behaviors: seeds, iterations, JSON output, error handling, flag positioning, help output.

# --help exits zero and is handled by urfave/cli.
scope cli_help {
  action run() {
    let result = process.exec("--help")
    return result
  }

  contract {
    input {}
    output {
      exit_code: int
    }
    action: run
  }

  scenario help_exits_zero {
    given {}
    then {
      exit_code == 0
    }
  }
}

scope cli_verify_help {
  action run() {
    let result = process.exec("verify", "--help")
    return result
  }

  contract {
    input {}
    output {
      exit_code: int
    }
    action: run
  }

  scenario verify_help_exits_zero {
    given {}
    then {
      exit_code == 0
    }
  }
}

# Different seeds produce different generated output.
# Seed 1 and seed 2 produce different amounts for the transfer scope.
scope generate_seed_1 {
  action run() {
    let result = process.exec("generate", "examples/transfer.spec", "--scope", "transfer", "--seed", "1")
    return result
  }

  contract {
    input {}
    output {
      exit_code: int
      amount: int
    }
    action: run
  }

  scenario seed_1_output {
    given {}
    then {
      exit_code == 0
      amount == 791
    }
  }
}

scope generate_seed_2 {
  action run() {
    let result = process.exec("generate", "examples/transfer.spec", "--scope", "transfer", "--seed", "2")
    return result
  }

  contract {
    input {}
    output {
      exit_code: int
      amount: int
    }
    action: run
  }

  scenario seed_2_output {
    given {}
    then {
      exit_code == 0
      amount == 586
    }
  }
}

# Iteration count is respected: --iterations controls inputs_run in JSON output.
# scopes.0.checks.3 is the first invariant ("conservation") in the transfer scope.
scope verify_iterations {
  action run(iterations: int, file: string) {
    let result = process.exec("verify", "--json", "--iterations", iterations, file)
    return result
  }

  contract {
    input {
      iterations: int
      file: string
    }
    output {
      exit_code: int
      scopes: any
    }
    action: run
  }

  scenario iterations_10 {
    given {
      iterations: 10
      file: "examples/transfer.spec"
    }
    then {
      exit_code == 0
      scopes.0.checks.3.inputs_run == 10
    }
  }
}

# JSON flag changes output format: verify --json produces parseable JSON with expected fields.
scope verify_json_output {
  action run() {
    let result = process.exec("verify", "--json", "examples/transfer.spec")
    return result
  }

  contract {
    input {}
    output {
      exit_code: int
      scenarios_run: int
      scenarios_passed: int
      invariants_checked: int
      invariants_passed: int
    }
    action: run
  }

  scenario json_output_fields {
    given {}
    then {
      exit_code == 0
      scenarios_run == 3
      scenarios_passed == 3
      invariants_checked == 3
      invariants_passed == 3
    }
  }
}

# Unknown subcommand is rejected with exit code 1.
scope cli_unknown_command {
  action run() {
    let result = process.exec("unknown")
    return result
  }

  contract {
    input {}
    output {
      exit_code: int
    }
    action: run
  }

  scenario unknown_rejected {
    given {}
    then {
      exit_code == 1
    }
  }
}

# Missing required args: generate with no spec file exits with error.
scope cli_missing_args_generate {
  action run() {
    let result = process.exec("generate")
    return result
  }

  contract {
    input {}
    output {
      exit_code: int
    }
    action: run
  }

  scenario no_spec_file {
    given {}
    then {
      exit_code == 1
    }
  }
}

# Missing required args: parse with no spec file exits with error.
scope cli_missing_args_parse {
  action run() {
    let result = process.exec("parse")
    return result
  }

  contract {
    input {}
    output {
      exit_code: int
    }
    action: run
  }

  scenario no_spec_file {
    given {}
    then {
      exit_code == 1
    }
  }
}

# Flag position flexibility: flags before or after spec file produce same output.
# urfave/cli handles interspersed flags natively.
scope generate_flags_after {
  action run() {
    let result = process.exec("generate", "examples/transfer.spec", "--scope", "transfer", "--seed", "1")
    return result
  }

  contract {
    input {}
    output {
      exit_code: int
      amount: int
    }
    action: run
  }

  scenario flags_after_spec {
    given {}
    then {
      exit_code == 0
      amount == 791
    }
  }
}

scope generate_flags_before {
  action run() {
    let result = process.exec("generate", "--scope", "transfer", "--seed", "1", "examples/transfer.spec")
    return result
  }

  contract {
    input {}
    output {
      exit_code: int
      amount: int
    }
    action: run
  }

  scenario flags_before_spec {
    given {}
    then {
      exit_code == 0
      amount == 791
    }
  }
}
