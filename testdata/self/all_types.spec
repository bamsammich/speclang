spec AllTypesTest {
  description: "Fixture exercising all types with constraints for generator invariant testing"

  http {
    base_url: "http://localhost:8080"
  }

  scope all_types {
    action run(name: string, flag: bool, data: bytes, tags: []string, metadata: map[string, int], count: int, score: float, opt_name: string, opt_count: int) {
      let result = http.post("/test", { name: name, flag: flag, data: data, tags: tags, metadata: metadata, count: count, score: score, opt_name: opt_name, opt_count: opt_count })
      return result
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
      action: run
    }
  }
}
