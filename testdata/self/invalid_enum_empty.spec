model EnumEmptyResult {
  ok: bool
}

scope test {
  contract EnumEmptyContract -> EnumEmptyResult {
    status: enum()

    action {
      return http.get("/test")
    }
  }
}
