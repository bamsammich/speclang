# Verifies the parser and generator handle extended types (float, bytes, array, map).
scope parse_types {
  config {
    args: "parse"
  }

  contract {
    input {
      file: string
    }
    output {
      exit_code: int
      name: string
    }
  }

  # The types spec should parse successfully.
  scenario types_spec {
    given {
      file: "testdata/self/types.spec"
    }
    then {
      exit_code: 0
      name: "TypesTest"
    }
  }
}

# Verifies the generator produces valid outputs for extended types.
scope generate_types {
  config {
    args: "generate testdata/self/types.spec --scope typed_inputs --seed"
  }

  contract {
    input {
      seed: int
    }
    output {
      exit_code: int
      rating: any
      data: any
      tags: any
      metadata: any
      items: any
    }
  }

  # Generation should succeed across seeds.
  invariant produces_output {
    exit_code == 0
  }

  # Float constraint: rating >= 0.0
  invariant float_constraint {
    when exit_code == 0:
      output.rating >= 0.0
  }
}
