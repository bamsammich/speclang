http {
  base_url: "http://localhost:8080"
}

model AllTypesResult {
  ok: bool
}

scope all_types {
  contract AllTypesContract(
    name: string { len(name) >= 1 and len(name) <= 10 },
    flag: bool,
    data: bytes,
    tags: []string { len(tags) >= 1 },
    metadata: map[string, int],
    count: int { count >= 0 and count <= 100 },
    score: float { score >= 0.0 and score <= 1000.0 },
    opt_name: string?,
    opt_count: int?,
  ) -> AllTypesResult {
    action {
      let result = http.post("/test", { name: name, flag: flag, data: data, tags: tags, metadata: metadata, count: count, score: score, opt_name: opt_name, opt_count: opt_count })
      return result
    }
  }
}
