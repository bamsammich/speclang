spec AccountAPI {
  description: "REST API for inter-account money transfers with balance tracking"

  http {
    base_url: service(app)
  }

  services {
    app {
      build: "./server"
      port: 8080
    }
  }

  model Account {
    id: string
    balance: int
  }

  scope transfer {
    action transfer(from: Account, to: Account, amount: int) {
      let result = http.post("/api/v1/accounts/transfer", { from: from, to: to, amount: amount })
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
      output.from.balance + output.to.balance == input.from.balance + input.to.balance
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
        from.balance == from.balance - amount
        to.balance == to.balance + amount
        error == null
      }
    }

    scenario overdraft {
      when {
        amount > from.balance
      }
      then {
        error == "insufficient_funds"
      }
    }

    scenario zero_transfer {
      when {
        amount == 0
      }
      then {
        error == "invalid_amount"
      }
    }
  }
}
