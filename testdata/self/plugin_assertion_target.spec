http {
  base_url: "http://localhost:8080"
}

model PluginAssertionResult {
  data: string
}

scope test_http {
  contract PluginAssertionContract() -> PluginAssertionResult {
    action {
      let result = http.get("/test")
      return result
    }

    scenario check_status {
      given {}
      then {
        output.status == 200
      }
    }
  }
}
