# Service lifecycle integration test fixture.
# Declares a service via build and verifies it responds.
spec ServiceTest {
  target {
    services {
      test_server {
        build: "./http_server"
        port: 9090
        env { PORT: "9090" }
        health: "/api/items"
      }
    }
    base_url: service(test_server)
  }

  scope service_health {
    use http
    config {
      path: "/api/items"
      method: "GET"
    }
    contract {
      input {}
      output {
        status: any
        count: int
      }
    }
    scenario server_responds {
      given {}
      then {
        status: 200
        count: 2
      }
    }
  }
}
