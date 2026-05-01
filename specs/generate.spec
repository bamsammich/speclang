# Verifies the generator produces constraint-satisfying outputs across seeds.

model GenerateResult {
  exit_code: int
  amount: int
  from: any
  to: any
}

scope generate {
  contract GenerateContract(seed: int) -> GenerateResult {
    action {
      let result = process.exec("generate", "examples/transfer.spec", "--scope", "transfer", "--seed", seed)
      return result
    }

    invariant produces_output {
      output.exit_code == 0
    }

    # Generated amounts must satisfy the declared constraint: 0 < amount <= from.balance.
    invariant constraints_satisfied {
      when output.exit_code == 0:
        output.amount > 0
        output.amount <= output.from.balance
    }
  }
}
