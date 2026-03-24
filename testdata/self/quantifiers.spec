spec Quantifiers {
  scope items {
    use http
    config {
      path: "/items"
      method: "GET"
    }
    contract {
      input {
        ids: []int
      }
      output {
        results: any
      }
    }
    invariant all_positive {
      all(input.ids, x => x > 0)
    }
    invariant any_large {
      any(input.ids, x => x > 100)
    }
  }
}
