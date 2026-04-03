spec WithIncludes {
  description: "Multi-file v2 spec"

  target {
    base_url: "http://localhost:8080"
  }

  model Account {
    id: string
    balance: int
  }

  include "scopes.v2.spec"
}
