# Verifies the parser accepts valid specs and produces expected AST structure.
scope parse_valid {
  contract ParseValidContract(file: string) -> ParseResult {
    action {
      let result = process.exec("parse", file)
      return result
    }

    # The minimal spec is an empty file — exit_code 0 and no scopes or models.
    scenario minimal_spec {
      given {
        file: "testdata/self/minimal.spec"
      }
      then {
        output.exit_code == 0
      }
    }

    # The examples/transfer.spec must parse with the expected models and scope name.
    scenario transfer_spec {
      given {
        file: "examples/transfer.spec"
      }
      then {
        output.exit_code == 0
        # Transfer spec defines Account and TransferResult models.
        output.models.0.name == "Account"
        output.models.1.name == "TransferResult"
        # The transfer scope must be present.
        output.scopes.0.name == "transfer"
      }
    }

    # Verifies that import openapi() parses and produces the correct models.
    scenario openapi_import {
      given {
        file: "testdata/openapi/import_valid.spec"
      }
      then {
        output.exit_code == 0
        # OpenAPI import must produce at least two models (Owner, Pet).
        output.models.0.name == "Owner"
        output.models.1.name == "Pet"
      }
    }

    # Verifies that import proto() parses and produces the correct models.
    scenario proto_import {
      given {
        file: "testdata/proto/import_valid.spec"
      }
      then {
        output.exit_code == 0
        # Proto import must produce models including CreateUserRequest.
        output.models.0.name == "CreateUserRequest"
      }
    }

    # Verifies that contains() built-in function parses in invariant expressions
    # and the scope appears in the AST.
    scenario contains_function {
      given {
        file: "testdata/self/contains.spec"
      }
      then {
        output.exit_code == 0
        output.scopes.0.name == "test"
        output.models.0.name == "ContainsResult"
      }
    }

    # Verifies that if/then/else conditional expressions parse correctly and the
    # scope is present in the AST.
    scenario if_expr {
      given {
        file: "testdata/self/if_expr.spec"
      }
      then {
        output.exit_code == 0
        output.scopes.0.name == "s"
        output.models.0.name == "IfExprResult"
      }
    }

    # Verifies that playwright spec syntax (locators, @assertions, mixed given) parses
    # and produces a scope named "login".
    scenario playwright_spec {
      given {
        file: "testdata/playwright/login.spec"
      }
      then {
        output.exit_code == 0
        output.scopes.0.name == "login"
      }
    }

    # Verifies that all() and any() quantifier expressions parse and the scope is present.
    scenario quantifier_spec {
      given {
        file: "testdata/self/quantifiers.spec"
      }
      then {
        output.exit_code == 0
        output.scopes.0.name == "items"
      }
    }

    # Verifies that before blocks parse correctly and appear in the scope AST.
    scenario before_block {
      given {
        file: "testdata/self/before_block.spec"
      }
      then {
        output.exit_code == 0
        output.scopes.0.name == "test_before"
      }
    }

    # Verifies that after blocks parse correctly and appear in the scope AST.
    scenario after_block {
      given {
        file: "testdata/self/after_block.spec"
      }
      then {
        output.exit_code == 0
        output.scopes.0.name == "test_after"
      }
    }

    # Verifies that plugin assertion targets (e.g., "status" for http) pass validation.
    scenario plugin_assertion_target {
      given {
        file: "testdata/self/plugin_assertion_target.spec"
      }
      then {
        output.exit_code == 0
        output.scopes.0.name == "test_http"
      }
    }
  }
}

model ParseResult {
  exit_code: int
}

# Verifies the parser rejects malformed specs with a non-zero exit code.
scope parse_invalid {
  contract ParseInvalidContract(file: string) -> ParseResult {
    action {
      let result = process.exec("parse", file)
      return result
    }

    scenario unterminated_spec {
      given {
        file: "testdata/self/invalid_unterminated.spec"
      }
      then {
        output.exit_code == 1
      }
    }

    scenario circular_include {
      given {
        file: "testdata/include/circular/a.spec"
      }
      then {
        output.exit_code == 1
      }
    }

    # Import with unknown adapter should fail.
    scenario import_unknown_adapter {
      given {
        file: "testdata/openapi/import_unknown_adapter.spec"
      }
      then {
        output.exit_code == 1
      }
    }

    # Import with bad syntax (missing parens) should fail.
    scenario import_bad_syntax {
      given {
        file: "testdata/openapi/import_bad_syntax.spec"
      }
      then {
        output.exit_code == 1
      }
    }

    # 'use' in scope body is rejected in v3.
    scenario multiple_use_directives {
      given {
        file: "testdata/self/invalid_multiple_use.spec"
      }
      then {
        output.exit_code == 1
      }
    }

    # 'use' at spec level (outside scope) should fail (v3 rejects 'use' entirely).
    scenario use_at_spec_level {
      given {
        file: "testdata/self/invalid_use_at_spec_level.spec"
      }
      then {
        output.exit_code == 1
      }
    }

    # Unknown token in spec body should fail.
    scenario unknown_token_in_spec {
      given {
        file: "testdata/self/invalid_unknown_token.spec"
      }
      then {
        output.exit_code == 1
      }
    }

    # Unexpected keyword inside contract block should fail.
    scenario malformed_contract {
      given {
        file: "testdata/self/invalid_malformed_contract.spec"
      }
      then {
        output.exit_code == 1
      }
    }

    # Unexpected token inside then block should fail.
    scenario malformed_then {
      given {
        file: "testdata/self/invalid_malformed_then.spec"
      }
      then {
        output.exit_code == 1
      }
    }

    # Unterminated string literal should fail at lex time.
    scenario unterminated_string {
      given {
        file: "testdata/self/invalid_unterminated_string.spec"
      }
      then {
        output.exit_code == 1
      }
    }

    # Single '&' (incomplete operator) should fail at lex time.
    scenario incomplete_operator {
      given {
        file: "testdata/self/invalid_single_ampersand.spec"
      }
      then {
        output.exit_code == 1
      }
    }

    # Duplicate model names across includes should fail.
    scenario duplicate_model_include {
      given {
        file: "testdata/include/duplicate_v4/root.spec"
      }
      then {
        output.exit_code == 1
      }
    }

    # Duplicate scope names across includes should fail.
    scenario duplicate_scope_include {
      given {
        file: "testdata/include/duplicate_scope_v4/root.spec"
      }
      then {
        output.exit_code == 1
      }
    }
  }
}

# Verifies the validator rejects semantically invalid specs with a non-zero exit code.
scope validate_invalid {
  contract ValidateInvalidContract(file: string) -> ParseResult {
    action {
      let result = process.exec("parse", file)
      return result
    }

    # Unknown type in model field should fail validation.
    scenario unknown_type {
      given {
        file: "testdata/self/invalid_unknown_type.spec"
      }
      then {
        output.exit_code == 1
      }
    }

    # String literal for int field should fail validation.
    scenario type_mismatch_in_given {
      given {
        file: "testdata/self/invalid_type_mismatch.spec"
      }
      then {
        output.exit_code == 1
      }
    }

    # Null for non-optional field should fail validation.
    scenario null_non_optional {
      given {
        file: "testdata/self/invalid_null_non_optional.spec"
      }
      then {
        output.exit_code == 1
      }
    }

    # Missing required field in given block should fail validation.
    scenario missing_required_field {
      given {
        file: "testdata/self/invalid_missing_required_field.spec"
      }
      then {
        output.exit_code == 1
      }
    }

    # Named enum reference with undeclared variant should fail validation.
    scenario named_enum_invalid_variant {
      given {
        file: "testdata/self/invalid_named_enum_variant.spec"
      }
      then {
        output.exit_code == 1
      }
    }
  }
}
