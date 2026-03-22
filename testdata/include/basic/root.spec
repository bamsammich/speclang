# Test fixture: basic include resolution (root -> models + scopes).
spec TestAPI {
  target {
    base_url: env(APP_URL, "http://localhost:8080")
  }

  include "models.spec"
  include "scopes.spec"
}
