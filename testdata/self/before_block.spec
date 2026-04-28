http {
  base_url: "http://localhost:8080"
}

model BeforeResult {
  ok: bool
}

scope test_before {
  before {
    http.header("X-Test", "before-value")
  }

  contract BeforeContract -> BeforeResult {
    name: string

    action {
      let result = http.post("/test", { name: name })
      return result
    }

    scenario smoke {
      given { name: "test" }
      then { output.ok == true }
    }
  }
}
