# Verifies the generator produces constraint-satisfying outputs across seeds.
scope generate {
  config {
    args: "generate examples/transfer.spec --scope transfer --seed"
  }

  contract {
    input {
      seed: int
    }
    output {
      exit_code: int
      amount: int
      from: any
      to: any
    }
  }

  invariant produces_output {
    exit_code == 0
  }

  # Generated amounts must satisfy the declared constraint: 0 < amount <= from.balance.
  invariant constraints_satisfied {
    when exit_code == 0:
      output.amount > 0
      output.amount <= output.from.balance
  }
}
