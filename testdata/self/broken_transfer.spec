# Test fixture: transfer spec targeting a broken server (wrong balances).
use http

spec BrokenTransfer {

  target {
    base_url: env(BROKEN_APP_URL, "http://localhost:8081")
  }

  model Account {
    id: string
    balance: int
  }

  scope transfer {
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

    scenario success {
      given {
        from: { id: "alice", balance: 100 }
        to: { id: "bob", balance: 50 }
        amount: 30
      }
      then {
        from.balance: 70
        to.balance: 80
        error: null
      }
    }
  }
}
