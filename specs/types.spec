# Verifies the parser and generator handle extended types (float, bytes, array, map).

model TypesParseResult {
  exit_code: int
  models: any
  scopes: any
}

model TypesGenerateResult {
  exit_code: int
  rating: any
  data: any
  tags: any
  metadata: any
  items: any
}

scope parse_types {
  contract ParseTypesContract(file: string) -> TypesParseResult {
    action {
      let result = process.exec("parse", file)
      return result
    }

    # The types spec should parse successfully. The first model must be "Item"
    # (the domain model), and the scope must be "typed_inputs" — proving the
    # parser extracted the struct definitions, not just that the command ran.
    scenario types_spec {
      given {
        file: "testdata/self/types.spec"
      }
      then {
        output.exit_code == 0
        output.models.0.name == "Item"
        output.scopes.0.name == "typed_inputs"
      }
    }
  }
}

# Verifies the generator produces valid outputs for extended types.
scope generate_types {
  contract GenerateTypesContract(seed: int) -> TypesGenerateResult {
    action {
      let result = process.exec("generate", "testdata/self/types.spec", "--scope", "typed_inputs", "--seed", seed)
      return result
    }

    # Generation should succeed across seeds.
    invariant produces_output {
      output.exit_code == 0
    }

    # Float constraint: rating >= 0.0
    invariant float_constraint {
      when output.exit_code == 0:
        output.rating >= 0.0
    }

    # Array constraint: tags must have at least one element (len(tags) >= 1).
    # A tautological len(tags) >= 0 would never catch a generator emitting
    # an empty array in violation of the constraint.
    invariant tags_length_constraint {
      when output.exit_code == 0:
        len(output.tags) >= 1
    }
  }
}
