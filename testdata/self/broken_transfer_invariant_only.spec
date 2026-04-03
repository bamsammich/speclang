# Test fixture: invariant-only transfer spec targeting a broken server.
# Used by shrinking performance specs to verify counterexample minimality.
# The broken server credits the to-account but never debits the from-account,
# so the conservation invariant fails for any amount > 0.
spec BrokenTransferInvariantOnly {

  http {
    base_url: env(BROKEN_APP_URL, "http://localhost:8081")
  }

  model Account {
    id: string
    balance: int
  }

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

    invariant conservation {
      when error == null:
        output.from.balance + output.to.balance
          == input.from.balance + input.to.balance
    }
  }
}
