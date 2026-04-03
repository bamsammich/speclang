# Service lifecycle integration test fixture.
# Declares a service via build and verifies it responds.
spec ServiceTest {
  http {
    base_url: service(test_server)
  }

  services {
    test_server {
      build: "./http_server"
      port: 9090
      env { PORT: "9090" }
      health: "/api/items"
    }
  }

  scope service_health {
    action run() {
      let result = http.get("/api/items")
      return result
    }

    contract {
      input {}
      output {
        status: any
        count: int
      }
      action: run
    }

    scenario server_responds {
      given {}
      then {
        status == 200
        count == 2
      }
    }
  }
}
