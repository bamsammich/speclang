model EnumEmptyResult {
  ok: bool
}

scope test {
  contract EnumEmptyContract(status: enum()) -> EnumEmptyResult {
    action {
      return http.get("/test")
    }
  }
}
