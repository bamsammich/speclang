spec Quantifiers {
  http {
    base_url: "http://localhost:8080"
  }

  scope items {
    action run(ids: []int) {
      let result = http.get("/items")
      return result
    }

    contract {
      input {
        ids: []int
      }
      output {
        results: any
      }
      action: run
    }

    invariant all_positive {
      all(input.ids, x => x > 0)
    }

    invariant any_large {
      any(input.ids, x => x > 100)
    }
  }
}
