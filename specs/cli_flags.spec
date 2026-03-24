# Verifies CLI flag behaviors: seeds, iterations, JSON output, error handling, flag positioning.

# Different seeds produce different generated output.
# Seed 1 and seed 2 produce different amounts for the transfer scope.
scope generate_seed_1 {
  use process
  config {
    args: "generate examples/transfer.spec --scope transfer --seed 1"
  }

  contract {
    input {}
    output {
      exit_code: int
      amount: int
    }
  }

  scenario seed_1_output {
    given {}
    then {
      exit_code: 0
      amount: 791
    }
  }
}

scope generate_seed_2 {
  use process
  config {
    args: "generate examples/transfer.spec --scope transfer --seed 2"
  }

  contract {
    input {}
    output {
      exit_code: int
      amount: int
    }
  }

  scenario seed_2_output {
    given {}
    then {
      exit_code: 0
      amount: 586
    }
  }
}

# Iteration count is respected: --iterations controls inputs_run in JSON output.
# scopes.0.checks.3 is the first invariant ("conservation") in the transfer scope.
scope verify_iterations {
  use process
  config {
    args: "verify --json --no-services --iterations"
  }

  contract {
    input {
      iterations: int
      file: string
    }
    output {
      exit_code: int
      scopes: array
    }
  }

  scenario iterations_10 {
    given {
      iterations: 10
      file: "examples/transfer.spec"
    }
    then {
      exit_code: 0
      scopes.0.checks.3.inputs_run: 10
    }
  }
}

# JSON flag changes output format: verify --json produces parseable JSON with expected fields.
scope verify_json_output {
  use process
  config {
    args: "verify --json --no-services examples/transfer.spec"
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
  }

  scenario json_output_fields {
    given {}
    then {
      exit_code: 0
      scenarios_run: 3
      scenarios_passed: 3
      invariants_checked: 3
      invariants_passed: 3
    }
  }
}

# Unknown subcommand is rejected with exit code 1.
scope cli_unknown_command {
  use process
  config {
    args: "unknown"
  }

  contract {
    input {}
    output {
      exit_code: int
    }
  }

  scenario unknown_rejected {
    given {}
    then {
      exit_code: 1
    }
  }
}

# Missing required args: generate with no spec file exits with error.
scope cli_missing_args_generate {
  use process
  config {
    args: "generate"
  }

  contract {
    input {}
    output {
      exit_code: int
    }
  }

  scenario no_spec_file {
    given {}
    then {
      exit_code: 1
    }
  }
}

# Missing required args: parse with no spec file exits with error.
scope cli_missing_args_parse {
  use process
  config {
    args: "parse"
  }

  contract {
    input {}
    output {
      exit_code: int
    }
  }

  scenario no_spec_file {
    given {}
    then {
      exit_code: 1
    }
  }
}

# Flag position flexibility: flags before or after spec file produce same output.
# generate supports flexible flag positioning via splitFlagsAndPositional.
scope generate_flags_after {
  use process
  config {
    args: "generate examples/transfer.spec --scope transfer --seed 1"
  }

  contract {
    input {}
    output {
      exit_code: int
      amount: int
    }
  }

  scenario flags_after_spec {
    given {}
    then {
      exit_code: 0
      amount: 791
    }
  }
}

scope generate_flags_before {
  use process
  config {
    args: "generate --scope transfer --seed 1 examples/transfer.spec"
  }

  contract {
    input {}
    output {
      exit_code: int
      amount: int
    }
  }

  scenario flags_before_spec {
    given {}
    then {
      exit_code: 0
      amount: 791
    }
  }
}
