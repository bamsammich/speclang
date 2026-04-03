# Included by v3basic/root.spec.
scope transfer {
  contract {
    input {
      from: Account
      to: Account
      amount: int
    }
    output {
      from: Account
      to: Account
      error: string?
    }
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
