http {
  base_url: "http://localhost:8080"
}

model TransferResult {
  ok: bool
  new_balance: int
}

scope implies_test {
  contract ImpliesContract(
    from_balance: int { from_balance >= 0 },
    amount: int { amount >= 0 },
  ) -> TransferResult {
    action {
      let result = http.post("/transfer", { from_balance: from_balance, amount: amount })
      return result
    }

    # implies: "if from_balance > 0, then output.ok must be true"
    # Equivalent to: not (from_balance > 0) or output.ok == true
    invariant success_on_positive_balance {
      from_balance > 0 implies output.ok == true
    }
  }
}
