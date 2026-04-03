scope transfer {
  action transfer(from: Account, to: Account, amount: int) {
    let result = http.post("/api/v1/accounts/transfer", {
      from: from, to: to, amount: amount
    })
    return result
  }

  contract {
    input {
      from: Account
      to: Account
      amount: int { 0 < amount <= from.balance }
    }
    output {
      from: Account
      to: Account
      error: string?
    }
    action: transfer
  }

  # Money is neither created nor destroyed on successful transfers.
  invariant conservation {
    when error == null:
      output.from.balance + output.to.balance
        == input.from.balance + input.to.balance
  }

  # Balances must never go negative, even on error.
  invariant non_negative {
    output.from.balance >= 0
    output.to.balance >= 0
  }

  # Failed transfers must not change any balances.
  invariant no_mutation_on_error {
    when error != null:
      output.from.balance == input.from.balance
      output.to.balance == input.to.balance
  }

  # Smoke test: a concrete successful transfer.
  scenario success {
    given {
      from: { id: "alice", balance: 100 }
      to: { id: "bob", balance: 50 }
      amount: 30
    }
    then {
      output.from.balance == input.from.balance - amount
      output.to.balance == input.to.balance + amount
      error == null
    }
  }

  # Generative: any amount exceeding balance must be rejected.
  scenario overdraft {
    when {
      amount > from.balance
    }
    then {
      error == "insufficient_funds"
    }
  }

  # Generative: zero-amount transfers are invalid.
  scenario zero_transfer {
    when {
      amount == 0
    }
    then {
      error == "invalid_amount"
    }
  }
}
