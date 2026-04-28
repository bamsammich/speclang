# Test fixture: transfer spec targeting a broken server (wrong balances).

http {
  base_url: env(BROKEN_APP_URL, "http://localhost:8081")
}

model Account {
  id: string
  balance: int
}

model BrokenTransferResult {
  from: Account
  to: Account
  error: string?
}

scope transfer {
  contract BrokenTransfer -> BrokenTransferResult {
    from: Account
    to: Account
    amount: int { 0 < amount <= from.balance }

    action {
      let result = http.post("/api/v1/accounts/transfer", {
        from: from, to: to, amount: amount
      })
      return result
    }

    invariant conservation {
      when output.error == null:
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
        output.from.balance == 70
        output.to.balance == 80
        output.error == null
      }
    }
  }
}
