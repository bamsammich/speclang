# Verifies the generator produces constraint-satisfying outputs for all supported types.
scope generate_all_types {
  use process
  config {
    args: "generate testdata/self/all_types.spec --scope all_types --seed"
  }

  contract {
    input {
      seed: int
    }
    output {
      exit_code: int
      name: any
      flag: any
      data: any
      tags: any
      metadata: any
      count: any
      score: any
      opt_name: any
      opt_count: any
    }
  }

  # Generation should succeed across seeds.
  invariant produces_output {
    exit_code == 0
  }

  # String constraint: len(name) >= 1 && len(name) <= 10
  invariant string_length_constraint {
    when exit_code == 0:
      len(output.name) >= 1
      len(output.name) <= 10
  }

  # Bool generates valid boolean values.
  invariant bool_is_valid {
    when exit_code == 0:
      output.flag == true || output.flag == false
  }

  # Bytes generates a string (base64-encoded).
  invariant bytes_is_string {
    when exit_code == 0:
      len(output.data) >= 0
  }

  # Array constraint: len(tags) >= 1
  invariant array_length_constraint {
    when exit_code == 0:
      len(output.tags) >= 1
  }

  # Map generates a valid structure (non-negative length).
  invariant map_is_valid {
    when exit_code == 0:
      len(output.metadata) >= 0
  }

  # Int constraint: count >= 0 && count <= 100
  invariant int_constraint {
    when exit_code == 0:
      output.count >= 0
      output.count <= 100
  }

  # Float constraint: score >= 0.0 && score <= 1000.0
  invariant float_constraint {
    when exit_code == 0:
      output.score >= 0.0
      output.score <= 1000.0
  }

  # Optional string: when non-nil, has valid string length.
  invariant optional_string_valid {
    when exit_code == 0 && output.opt_name != null:
      len(output.opt_name) >= 1
  }

  # Cross-field: int and float constraints satisfied simultaneously.
  invariant cross_field_constraints {
    when exit_code == 0:
      output.count >= 0 && output.count <= 100 && output.score >= 0.0 && output.score <= 1000.0
  }
}
