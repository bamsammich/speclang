http {
  base_url: "http://localhost:8080"
}

model PaymentInput {
  amount: int
  currency: string
}

model PaymentOutput {
  ok: bool
  reference: string
}

# Contract inherits fields from PaymentInput (amount, currency)
# and adds constraints on the inherited fields.
contract PaymentContract: PaymentInput -> PaymentOutput {
  constrain {
    amount > 0
  }

  action {
    let result = http.post("/pay", { amount: amount, currency: currency })
    return result
  }

  scenario basic_payment {
    given {
      amount: 100
      currency: "USD"
    }
    then {
      output.ok == true
    }
  }
}
