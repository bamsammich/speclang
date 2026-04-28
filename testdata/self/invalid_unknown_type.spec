model BrokenResult {
  result: int
}

scope broken {
  contract BrokenContract -> BrokenResult {
    item: Widget

    action {
      return http.get("/test")
    }
  }
}
