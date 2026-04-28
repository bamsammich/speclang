http {
  base_url: "http://localhost:8080"
}

model QuantifiersResult {
  results: any
}

scope items {
  contract QuantifiersContract -> QuantifiersResult {
    ids: []int

    action {
      let result = http.get("/items")
      return result
    }

    invariant all_positive {
      all(ids, x => x > 0)
    }

    invariant any_large {
      any(ids, x => x > 100)
    }
  }
}
