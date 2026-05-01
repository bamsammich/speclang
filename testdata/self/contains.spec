http {
  base_url: "http://localhost:8080"
}

model ContainsResult {
  ok: bool
}

scope test {
  contract ContainsContract(msg: string, items: []int) -> ContainsResult {
    action {
      let result = http.post("/api/test", { msg: msg, items: items })
      return result
    }

    # Verify contains() works in invariant expressions
    invariant error_has_keyword {
      when output.ok == false:
        contains(msg, "error")
    }

    # Verify contains() works with array membership
    invariant items_has_element {
      when contains(items, 1):
        output.ok == true
    }
  }
}
