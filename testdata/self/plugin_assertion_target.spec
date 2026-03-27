spec PluginAssertionTarget {
  scope test_http {
    use http
    config {
      path: "/test"
      method: "GET"
    }
    contract {
      input {}
      output {
        data: string
      }
    }
    scenario check_status {
      given {}
      then {
        status: 200
      }
    }
  }
}
