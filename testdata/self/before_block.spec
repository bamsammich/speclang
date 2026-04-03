spec BeforeBlock {
  http {
    base_url: "http://localhost:8080"
  }

  scope test_before {
    action run(name: string) {
      let result = http.post("/test", { name: name })
      return result
    }

    before {
      http.header("X-Test", "before-value")
    }

    contract {
      input { name: string }
      output { ok: bool }
      action: run
    }

    scenario smoke {
      given { name: "test" }
      then { ok == true }
    }
  }
}
