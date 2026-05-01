model BrokenResult {
  result: int
}

scope broken {
  contract BrokenContract(item: Widget) -> BrokenResult {
    action {
      return http.get("/test")
    }
  }
}
