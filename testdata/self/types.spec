http {
  base_url: "http://localhost:8080"
}

model Item {
  name: string
  price: float { price >= 0.0 }
}

model TypesResult {
  ok: bool
}

scope typed_inputs {
  contract TypesContract -> TypesResult {
    rating: float { rating >= 0.0 }
    data: bytes
    tags: []string { len(tags) >= 1 }
    metadata: map[string, int]
    items: []Item

    action {
      let result = http.post("/test", { rating: rating, data: data, tags: tags, metadata: metadata, items: items })
      return result
    }
  }
}
