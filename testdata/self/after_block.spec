http {
  base_url: "http://localhost:8080"
}

model AfterResult {
  ok: bool
}

scope test_after {
  after {
    http.delete("/cleanup")
  }

  contract AfterContract(name: string) -> AfterResult {
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
