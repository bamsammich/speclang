use http

spec TypesTest {
  description: "Test spec exercising float, bytes, array, map, and len()"

  target {
    base_url: "http://localhost:8080"
  }

  model Item {
    name: string
    price: float { price >= 0.0 }
  }

  scope typed_inputs {
    config {
      path: "/test"
      method: "POST"
    }

    contract {
      input {
        rating: float { rating >= 0.0 }
        data: bytes
        tags: []string { len(tags) >= 1 }
        metadata: map[string, int]
        items: []Item
      }
      output {
        ok: bool
      }
    }
  }
}
