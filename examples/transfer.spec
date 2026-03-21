use http

spec AccountAPI {
  description: "REST API for inter-account money transfers with balance tracking"

  target {
    base_url: env(APP_URL, "http://localhost:8080")
  }

  include "models/account.spec"
  include "scopes/transfer.spec"
}
