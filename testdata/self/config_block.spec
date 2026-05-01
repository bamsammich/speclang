config {
  max_transfer: 1000000
  api_version: "v2"
  retries: 3
}

model ConfigResult {
  ok: bool
}

scope config_block_test {
  contract ConfigBlockContract(amount: int { amount <= config.max_transfer }) -> ConfigResult {
    action {
      return { ok: true }
    }

    scenario small_amount {
      given {
        amount: 100
      }
      then {
        output.ok == true
      }
    }
  }
}
