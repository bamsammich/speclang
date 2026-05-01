scope transfer {
  contract Transfer(
    from: Account,
    to: Account,
    amount: int { 0 < amount <= from.balance },
  ) -> TransferResult {
    action {
      return http.post("/api/v1/accounts/transfer", {
        from: from, to: to, amount: amount
      })
    }

    # Money is neither created nor destroyed on any transfer (error or not).
    invariant conservation {
      output.from.balance + output.to.balance == from.balance + to.balance
    }

    # Balances must never go negative.
    invariant non_negative {
      output.from.balance >= 0
      output.to.balance >= 0
    }

    # Failed transfers must not change any balances.
    invariant no_mutation_on_error {
      when output.error != null:
        output.from.balance == from.balance
        output.to.balance == to.balance
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
        output.error == null
      }
    }

    # Generative: any amount exceeding balance must be rejected.
    scenario overdraft {
      when {
        amount > from.balance
      }
      then {
        output.error == "insufficient_funds"
      }
    }

    # Generative: zero-amount transfers are invalid.
    scenario zero_transfer {
      when {
        amount == 0
      }
      then {
        output.error == "invalid_amount"
      }
    }
  }
}
