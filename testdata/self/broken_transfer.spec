# Test fixture: transfer spec targeting a broken server (wrong balances).
spec BrokenTransfer {

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

    scenario success {
      given {
        from: { id: "alice", balance: 100 }
        to: { id: "bob", balance: 50 }
        amount: 30
      }
      then {
        from.balance == 70
        to.balance == 80
        error == null
      }
    }
  }
}
