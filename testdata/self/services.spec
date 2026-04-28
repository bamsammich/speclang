# Service lifecycle integration test fixture.
# Declares a service via build and verifies it responds.

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

model ServiceHealthResult {
  status: any
  count: int
}

scope service_health {
  contract ServiceHealthContract -> ServiceHealthResult {
    action {
      let result = http.get("/api/items")
      return result
    }

    scenario server_responds {
      given {}
      then {
        output.status == 200
        output.count == 2
      }
    }
  }
}
