# Test fixture: basic include resolution (root -> models + scopes) for v4.
http {
  base_url: env(APP_URL, "http://localhost:8080")
}

include "models.spec"
include "scopes.spec"
