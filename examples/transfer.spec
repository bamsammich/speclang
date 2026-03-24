spec AccountAPI {
  description: "REST API for inter-account money transfers with balance tracking"

  target {
    services {
      app {
        build: "./server"
        port: 8080
      }
    }
    base_url: service(app)
  }

  include "models/account.spec"
  include "scopes/transfer.spec"
}
