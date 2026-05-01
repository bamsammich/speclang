http {
  base_url: "http://localhost:8080"
}

model TransferResult {
  ok: bool
}

scope underscore_numeric {
  contract UnderscoreNumericContract(
    # Underscore separators make large numbers readable.
    # 1_000_000 should parse as 1000000.
    amount: int { amount <= 1_000_000 },
  ) -> TransferResult {
    action {
      let result = http.post("/transfer", { amount: amount })
      return result
    }

    scenario max_amount {
      given {
        amount: 500_000
      }
      then {
        output.ok == true
      }
    }
  }
}
