scope transfer {
  use http
  config {
    path: "/api/transfer"
    method: "POST"
  }

  contract {
    input {
      from: Account
      to: Account
      amount: int
    }
    output {
      error: string?
    }
  }

  scenario success {
    given {
      from: { id: "a", balance: 100 }
      to: { id: "b", balance: 50 }
      amount: 30
    }
    then {
      error: null
    }
  }
}
