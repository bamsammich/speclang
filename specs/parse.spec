# Verifies the parser accepts valid specs and produces expected AST structure.
scope parse_valid {
  use process
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

  scenario minimal_spec {
    given {
      file: "testdata/self/minimal.spec"
    }
    then {
      exit_code: 0
      name: "Minimal"
    }
  }

  scenario transfer_spec {
    given {
      file: "examples/transfer.spec"
    }
    then {
      exit_code: 0
      name: "AccountAPI"
    }
  }

  # Verifies that import openapi() parses and produces the correct spec name.
  scenario openapi_import {
    given {
      file: "testdata/openapi/import_valid.spec"
    }
    then {
      exit_code: 0
      name: "ImportTest"
    }
  }

  # Verifies that import proto() parses and produces the correct spec name.
  scenario proto_import {
    given {
      file: "testdata/proto/import_valid.spec"
    }
    then {
      exit_code: 0
      name: "ProtoImportTest"
    }
  }

  # Verifies that contains() built-in function parses in invariant expressions.
  scenario contains_function {
    given {
      file: "testdata/self/contains.spec"
    }
    then {
      exit_code: 0
      name: "ContainsTest"
    }
  }

  # Verifies that if/then/else conditional expressions parse correctly.
  scenario if_expr {
    given {
      file: "testdata/self/if_expr.spec"
    }
    then {
      exit_code: 0
      name: "IfExprTest"
    }
  }

  # Verifies that playwright spec syntax (locators, @assertions, mixed given) parses.
  scenario playwright_spec {
    given {
      file: "testdata/playwright/login.spec"
    }
    then {
      exit_code: 0
      name: "LoginUI"
    }
  }

  # Verifies that all() and any() quantifier expressions parse.
  scenario quantifier_spec {
    given {
      file: "testdata/self/quantifiers.spec"
    }
    then {
      exit_code: 0
      name: "Quantifiers"
    }
  }

  # Verifies that plugin assertion targets (e.g., "status" for http) pass validation.
  scenario plugin_assertion_target {
    given {
      file: "testdata/self/plugin_assertion_target.spec"
    }
    then {
      exit_code: 0
      name: "PluginAssertionTarget"
    }
  }
}

# Verifies the parser rejects malformed specs with a non-zero exit code.
scope parse_invalid {
  use process
  config {
    args: "parse"
  }

  contract {
    input {
      file: string
    }
    output {
      exit_code: int
    }
  }

  scenario unterminated_spec {
    given {
      file: "testdata/self/invalid_unterminated.spec"
    }
    then {
      exit_code: 1
    }
  }

  scenario circular_include {
    given {
      file: "testdata/include/circular/a.spec"
    }
    then {
      exit_code: 1
    }
  }

  # Import with unknown adapter should fail.
  scenario import_unknown_adapter {
    given {
      file: "testdata/openapi/import_unknown_adapter.spec"
    }
    then {
      exit_code: 1
    }
  }

  # Import with bad syntax (missing parens) should fail.
  scenario import_bad_syntax {
    given {
      file: "testdata/openapi/import_bad_syntax.spec"
    }
    then {
      exit_code: 1
    }
  }

  # Scope without 'use <plugin>' should fail.
  scenario missing_use_directive {
    given {
      file: "testdata/self/invalid_missing_use.spec"
    }
    then {
      exit_code: 1
    }
  }

  # Multiple 'use' directives in same scope should fail.
  scenario multiple_use_directives {
    given {
      file: "testdata/self/invalid_multiple_use.spec"
    }
    then {
      exit_code: 1
    }
  }

  # 'use' at spec level (outside scope) should fail.
  scenario use_at_spec_level {
    given {
      file: "testdata/self/invalid_use_at_spec_level.spec"
    }
    then {
      exit_code: 1
    }
  }

  # Unknown token in spec body should fail.
  scenario unknown_token_in_spec {
    given {
      file: "testdata/self/invalid_unknown_token.spec"
    }
    then {
      exit_code: 1
    }
  }

  # Unexpected keyword inside contract block should fail.
  scenario malformed_contract {
    given {
      file: "testdata/self/invalid_malformed_contract.spec"
    }
    then {
      exit_code: 1
    }
  }

  # Unexpected token inside then block should fail.
  scenario malformed_then {
    given {
      file: "testdata/self/invalid_malformed_then.spec"
    }
    then {
      exit_code: 1
    }
  }

  # Unterminated string literal should fail at lex time.
  scenario unterminated_string {
    given {
      file: "testdata/self/invalid_unterminated_string.spec"
    }
    then {
      exit_code: 1
    }
  }

  # Single '&' (incomplete operator) should fail at lex time.
  scenario incomplete_operator {
    given {
      file: "testdata/self/invalid_single_ampersand.spec"
    }
    then {
      exit_code: 1
    }
  }

  # Duplicate model names across includes should fail.
  scenario duplicate_model_include {
    given {
      file: "testdata/include/duplicate/root.spec"
    }
    then {
      exit_code: 1
    }
  }

  # Duplicate scope names across includes should fail.
  scenario duplicate_scope_include {
    given {
      file: "testdata/include/duplicate_scope/root.spec"
    }
    then {
      exit_code: 1
    }
  }
}

# Verifies the validator rejects semantically invalid specs with a non-zero exit code.
scope validate_invalid {
  use process
  config {
    args: "parse"
  }

  contract {
    input {
      file: string
    }
    output {
      exit_code: int
    }
  }

  # Unknown type in model field should fail validation.
  scenario unknown_type {
    given {
      file: "testdata/self/invalid_unknown_type.spec"
    }
    then {
      exit_code: 1
    }
  }

  # String literal for int field should fail validation.
  scenario type_mismatch_in_given {
    given {
      file: "testdata/self/invalid_type_mismatch.spec"
    }
    then {
      exit_code: 1
    }
  }

  # Null for non-optional field should fail validation.
  scenario null_non_optional {
    given {
      file: "testdata/self/invalid_null_non_optional.spec"
    }
    then {
      exit_code: 1
    }
  }

  # Missing required field in given block should fail validation.
  scenario missing_required_field {
    given {
      file: "testdata/self/invalid_missing_required_field.spec"
    }
    then {
      exit_code: 1
    }
  }

  # Then target not in contract output should fail validation.
  scenario then_unknown_output {
    given {
      file: "testdata/self/invalid_then_unknown_field.spec"
    }
    then {
      exit_code: 1
    }
  }
}
