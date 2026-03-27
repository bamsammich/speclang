spec BeforeBlock {
  scope test_before {
    use http
    config {
      path: "/test"
      method: "POST"
    }
    before {
      http.header("X-Test", "before-value")
    }
    contract {
      input { name: string }
      output { ok: bool }
    }
    scenario smoke {
      given { name: "test" }
      then { ok: true }
    }
  }
}
