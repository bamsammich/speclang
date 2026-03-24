spec AllTypesTest {
  description: "Fixture exercising all types with constraints for generator invariant testing"

  target {
    base_url: "http://localhost:8080"
  }

  scope all_types {
    use http
    config {
      path: "/test"
      method: "POST"
    }

    contract {
      input {
        name: string { len(name) >= 1 && len(name) <= 10 }
        flag: bool
        data: bytes
        tags: []string { len(tags) >= 1 }
        metadata: map[string, int]
        count: int { count >= 0 && count <= 100 }
        score: float { score >= 0.0 && score <= 1000.0 }
        opt_name: string?
        opt_count: int?
      }
      output {
        ok: bool
      }
    }
  }
}
