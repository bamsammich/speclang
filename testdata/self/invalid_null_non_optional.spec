model NullNonOptionalResult {
  result: int
}

scope broken {
  contract NullNonOptionalContract -> NullNonOptionalResult {
    name: string

    action {
      return http.get("/test")
    }

    scenario smoke {
      given {
        name: null
      }
      then {
        output.result == 0
      }
    }
  }
}
