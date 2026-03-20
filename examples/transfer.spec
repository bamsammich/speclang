use http

spec AccountAPI {

  target {
    base_url: env(APP_URL, "http://localhost:8080")
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

    invariant non_negative {
      output.from.balance >= 0
      output.to.balance >= 0
    }

    invariant no_mutation_on_error {
      when error != null:
        output.from.balance == input.from.balance
        output.to.balance == input.to.balance
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

    scenario overdraft {
      when {
        amount > from.balance
      }
      then {
        error: "insufficient_funds"
      }
    }

    scenario zero_transfer {
      when {
        amount == 0
      }
      then {
        error: "invalid_amount"
      }
    }
  }
}
