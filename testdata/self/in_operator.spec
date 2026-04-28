http {
  base_url: "http://localhost:8080"
}

model StatusResult {
  ok: bool
}

scope in_operator_test {
  contract InOperatorContract -> StatusResult {
    status: string

    action {
      let result = http.post("/status", { status: status })
      return result
    }

    # Scenario with when using in operator and array literal.
    scenario pending_or_active {
      when {
        status in ["pending", "active"]
      }
      then {
        output.ok == true
      }
    }

    # Invariant using in operator — binary form with single string.
    invariant single_in {
      status in "pending" implies output.ok == true
    }
  }
}
