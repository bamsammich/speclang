spec AfterBlock {
  http {
    base_url: "http://localhost:8080"
  }

  scope test_after {
    action run(name: string) {
      let result = http.post("/test", { name: name })
      return result
    }

    after {
      http.delete("/cleanup")
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
