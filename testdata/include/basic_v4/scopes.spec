# Included by basic_v4/root.spec.
scope transfer {
  contract Transfer -> Account {
    from: Account
    to: Account
    amount: int
  }
}
