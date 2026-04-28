# Self-verification for v4 language features:
# named enums, in operator, implies operator, underscore numerics, contract inheritance.

model V4ParseResult {
  exit_code: int
}

model V4GenerateResult {
  exit_code: int
  role: any
}

# ─── Named Enums ────────────────────────────────────────────────────────────

# Verifies the parser accepts named enum declarations and model fields using
# a named enum type, and that enum variants appear in the AST.
scope parse_named_enum {
  contract ParseNamedEnumContract -> V4ParseResult {
    file: string

    action {
      let result = process.exec("parse", file)
      return result
    }

    # enum Name { variant, ... } at top level parses successfully and the
    # enum declaration appears in the AST with the correct name and variants.
    scenario named_enum_spec {
      given {
        file: "testdata/self/named_enum.spec"
      }
      then {
        output.exit_code == 0
        # The first declared enum must be named "Role".
        output.enums.0.name == "Role"
        # All three declared variants must be present.
        output.enums.0.variants.0 == "admin"
        output.enums.0.variants.1 == "user"
        output.enums.0.variants.2 == "viewer"
      }
    }
  }
}

# Verifies the generator only produces declared variants for a named-enum field.
scope generate_named_enum {
  contract GenerateNamedEnumContract -> V4GenerateResult {
    seed: int

    action {
      let result = process.exec("generate", "testdata/self/named_enum.spec", "--scope", "named_enum_inputs", "--seed", seed)
      return result
    }

    # Generator should succeed across all seeds.
    invariant produces_output {
      output.exit_code == 0
    }

    # Generated role must be one of the declared variants.
    invariant role_is_valid_variant {
      when output.exit_code == 0:
        output.role == "admin" or output.role == "user" or output.role == "viewer"
    }
  }
}

# ─── `in` Operator ──────────────────────────────────────────────────────────

# Verifies the `in` operator parses in when predicates (array literal RHS)
# and in invariant expressions (single-value binary form).
scope parse_in_operator {
  contract ParseInOperatorContract -> V4ParseResult {
    file: string

    action {
      let result = process.exec("parse", file)
      return result
    }

    # The `in` operator spec parses without error and the scope is present in
    # the AST, proving the parser handled `status in [...]` correctly.
    scenario in_operator_spec {
      given {
        file: "testdata/self/in_operator.spec"
      }
      then {
        output.exit_code == 0
        # The parsed spec must have exactly one scope named "in_operator_test".
        output.scopes.0.name == "in_operator_test"
      }
    }
  }
}

# ─── `implies` Operator ─────────────────────────────────────────────────────

# Verifies the `implies` operator parses in invariant assertions.
# implies has the lowest precedence: `a implies b` means `not a or b`.
scope parse_implies {
  contract ParseImpliesContract -> V4ParseResult {
    file: string

    action {
      let result = process.exec("parse", file)
      return result
    }

    # The implies spec parses and the invariant assertion uses the "implies" op.
    scenario implies_spec {
      given {
        file: "testdata/self/implies.spec"
      }
      then {
        output.exit_code == 0
        output.scopes.0.name == "implies_test"
        # The first invariant assertion's binary op must be "implies".
        output.scopes.0.contracts.0.invariants.0.assertions.0.expr.op == "implies"
      }
    }
  }
}

# ─── Underscore Numeric Separators ──────────────────────────────────────────

# Verifies that underscore-separated numeric literals (1_000_000, 500_000)
# parse as their underlying integer values.
scope parse_underscore_numeric {
  contract ParseUnderscoreNumericContract -> V4ParseResult {
    file: string

    action {
      let result = process.exec("parse", file)
      return result
    }

    # 1_000_000 must appear as 1000000 in the AST constraint — not as a string
    # or any other representation. This proves the lexer strips underscores.
    scenario underscore_numeric_spec {
      given {
        file: "testdata/self/underscore_numeric.spec"
      }
      then {
        output.exit_code == 0
        # The field constraint's right-hand literal must equal 1000000.
        output.scopes.0.contracts.0.fields.0.constraint.right.value == 1000000
      }
    }
  }
}

# ─── Contract Model Inheritance ─────────────────────────────────────────────

# Verifies that `contract Name: InputModel -> OutputModel { constrain { ... } }`
# parses correctly: the `inherits` field and `constrain` block are present in the AST.
scope parse_contract_inheritance {
  contract ParseContractInheritanceContract -> V4ParseResult {
    file: string

    action {
      let result = process.exec("parse", file)
      return result
    }

    # The inheritance contract must have `inherits == "PaymentInput"` in the AST
    # and at least one constraint expression in the `constraints` array.
    scenario contract_inheritance_spec {
      given {
        file: "testdata/self/contract_inheritance.spec"
      }
      then {
        output.exit_code == 0
        # The top-level contract must inherit "PaymentInput".
        output.contracts.0.inherits == "PaymentInput"
        # The constrain block must produce at least one constraint expression.
        output.contracts.0.constraints.0.op == ">"
      }
    }
  }
}

# ─── Config Block ───────────────────────────────────────────────────────────

# Verifies that a top-level `config { key: expr }` block parses successfully.
# "config" lexes as TokenConfig (a keyword), which required routing via
# parseTopLevelDecl → parseTopLevelIdentDecl.
scope parse_config_block {
  contract ParseConfigBlockContract -> V4ParseResult {
    file: string

    action {
      let result = process.exec("parse", file)
      return result
    }

    # The config block must appear in the AST with max_transfer == 1000000.
    # If config parsing regresses, this key will be absent or have the wrong value.
    scenario config_block_spec {
      given {
        file: "testdata/self/config_block.spec"
      }
      then {
        output.exit_code == 0
        # config.max_transfer must equal 1000000 (the literal from the file).
        output.config.max_transfer.value == 1000000
      }
    }
  }
}

# ─── State-Dependent Fields ──────────────────────────────────────────────────

# Verifies that `field: type when condition` parses correctly into the AST
# (Field.When is populated). Both model fields and contract input fields are exercised.
scope parse_state_dependent_fields {
  contract ParseStateDependentContract -> V4ParseResult {
    file: string

    action {
      let result = process.exec("parse", file)
      return result
    }

    # The model's second field ("tracking") must have a non-null `when` key,
    # proving the parser stored the when-condition in Field.When.
    scenario state_dependent_spec {
      given {
        file: "testdata/self/state_dependent.spec"
      }
      then {
        output.exit_code == 0
        # models[0].fields[1] is "tracking". Its when.op must be "==" (status == "shipped").
        output.models.0.fields.1.name == "tracking"
        output.models.0.fields.1.when.op == "=="
      }
    }
  }
}

# ─── No-wrapper Top-level Declarations ──────────────────────────────────────

# In v4, the file IS the spec — no `spec Name { }` wrapper required.
# Every .spec file in testdata/self/ uses this format. This test explicitly
# verifies the feature by parsing a spec with top-level model + scope + contract
# with no enclosing spec block.
scope parse_no_wrapper {
  contract ParseNoWrapperContract -> V4ParseResult {
    file: string

    action {
      let result = process.exec("parse", file)
      return result
    }

    # A spec with only top-level declarations (no spec{} wrapper) parses and
    # the enum is present at the top level (not nested under a spec key).
    scenario top_level_decls {
      given {
        file: "testdata/self/named_enum.spec"
      }
      then {
        output.exit_code == 0
        # If the wrapper were required, enums would be absent or nested wrongly.
        output.enums.0.name == "Role"
      }
    }

    # A spec with only top-level model declarations (no scope or contract) parses
    # and the model is present at the top level.
    scenario model_only_file {
      given {
        file: "testdata/self/contract_inheritance.spec"
      }
      then {
        output.exit_code == 0
        # The first top-level model must be named "PaymentInput".
        output.models.0.name == "PaymentInput"
      }
    }
  }
}
