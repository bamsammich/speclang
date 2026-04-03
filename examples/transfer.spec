spec AccountAPI {
  description: "REST API for inter-account money transfers with balance tracking"

  http {
    base_url: service(app)
  }

  services {
    app {
      build: "./server"
      port: 8080
      health: "/healthz"
    }
  }

  include "models/account.spec"
  include "scopes/transfer.spec"
}
