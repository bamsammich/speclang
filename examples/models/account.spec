# Shared account model used across transfer scenarios.
model Account {
  id: string
  balance: int
}

model TransferResult {
  from: Account
  to: Account
  error: string?
}
