use http

spec AccountAPI {

  target {
    base_url: env(APP_URL, "http://localhost:8080")
  }

  include "models/account.spec"
  include "scopes/transfer.spec"
}
