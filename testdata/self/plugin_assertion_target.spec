spec PluginAssertionTarget {
  http {
    base_url: "http://localhost:8080"
  }

  scope test_http {
    action run() {
      let result = http.get("/test")
      return result
    }

    contract {
      input {}
      output {
        data: string
      }
      action: run
    }

    scenario check_status {
      given {}
      then {
        status == 200
      }
    }
  }
}
