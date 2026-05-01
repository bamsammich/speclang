# Verifies that counterexample shrinking produces minimal values.
# This is a performance/quality spec — it asserts the RESULT is small,
# not the algorithm used to get there.
#
# The broken server credits the to-account but never debits from-account.
# The conservation invariant fails for any amount > 0. After shrinking,
# the counterexample should converge toward the failure boundary.

model ShrinkingResult {
  exit_code: int
  invariants_checked: int
  invariants_passed: int
  failures: any
}

scope shrinking {
  contract ShrinkingContract(file: string) -> ShrinkingResult {
    action {
      let result = process.exec("verify", "--json", file)
      return result
    }

    # The invariant-only fixture has a single generative failure.
    # After shrinking, the counterexample must be marked as shrunk.
    scenario counterexample_is_shrunk {
      given {
        file: "testdata/self/broken_transfer_invariant_only.spec"
      }
      then {
        output.exit_code == 1
        output.invariants_checked == 1
        output.invariants_passed == 0
        output.failures.0.shrunk == true
      }
    }

    # The shrunk amount should be minimal — near the failure boundary.
    # The invariant fails for any amount > 0, so the minimum is 1.
    scenario shrunk_int_is_minimal {
      given {
        file: "testdata/self/broken_transfer_invariant_only.spec"
      }
      then {
        output.failures.0.input.amount == 1
      }
    }

    # Shrunk strings should be empty (the shortest valid string).
    scenario shrunk_string_is_minimal {
      given {
        file: "testdata/self/broken_transfer_invariant_only.spec"
      }
      then {
        output.failures.0.input.from.id == ""
        output.failures.0.input.to.id == ""
      }
    }

    # Shrunk ints without lower constraints should reach 0.
    scenario shrunk_unconstrained_int_reaches_zero {
      given {
        file: "testdata/self/broken_transfer_invariant_only.spec"
      }
      then {
        output.failures.0.input.to.balance == 0
      }
    }
  }
}
