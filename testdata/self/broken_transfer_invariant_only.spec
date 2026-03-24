# Test fixture: invariant-only transfer spec targeting a broken server.
# Used by shrinking performance specs to verify counterexample minimality.
# The broken server credits the to-account but never debits the from-account,
# so the conservation invariant fails for any amount > 0.
spec BrokenTransferInvariantOnly {

  target {
    base_url: env(BROKEN_APP_URL, "http://localhost:8081")
  }

  model Account {
    id: string
    balance: int
  }

  scope transfer {
    use http
    config {
      path: "/api/v1/accounts/transfer"
      method: "POST"
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
    }

    invariant conservation {
      when error == null:
        output.from.balance + output.to.balance
          == input.from.balance + input.to.balance
    }
  }
}
