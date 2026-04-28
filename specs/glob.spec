# Verifies that specrun verify accepts glob patterns and processes each matched
# file as an independent unit.

model GlobResult {
  exit_code: int
}

# ─── Glob expansion ─────────────────────────────────────────────────────────

# Verifies a simple glob (*.spec) matches multiple files and passes.
# The fixture dir contains two specs with contracts + one models-only spec.
scope verify_glob_simple {
  contract VerifyGlobSimpleContract -> GlobResult {
    action {
      let result = process.exec("verify", "testdata/self/glob/*.spec")
      return result
    }

    # All matched files should verify successfully (models-only is skipped, not failed).
    scenario glob_matches_pass {
      given {}
      then {
        output.exit_code == 0
      }
    }
  }
}

# Verifies a recursive glob (**/*.spec) finds files in nested directories.
scope verify_glob_recursive {
  contract VerifyGlobRecursiveContract -> GlobResult {
    action {
      let result = process.exec("verify", "testdata/self/glob/**/*.spec")
      return result
    }

    scenario recursive_glob_passes {
      given {}
      then {
        output.exit_code == 0
      }
    }
  }
}

# Verifies that a file with no contracts is skipped (exit 0, not failure).
scope verify_glob_no_contracts {
  contract VerifyGlobNoContractsContract -> GlobResult {
    action {
      let result = process.exec("verify", "testdata/self/glob/models_only.spec")
      return result
    }

    scenario no_contract_skipped {
      given {}
      then {
        output.exit_code == 0
      }
    }
  }
}

# Verifies that a glob matching zero files produces exit code 1.
scope verify_glob_no_match {
  contract VerifyGlobNoMatchContract -> GlobResult {
    action {
      let result = process.exec("verify", "testdata/self/glob/nonexistent/*.spec")
      return result
    }

    scenario zero_matches_is_error {
      given {}
      then {
        output.exit_code == 1
      }
    }
  }
}
