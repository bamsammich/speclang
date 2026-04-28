model TypeMismatchResult {
  result: int
}

scope broken {
  contract TypeMismatchContract -> TypeMismatchResult {
    count: int

    action {
      return http.get("/test")
    }

    scenario smoke {
      given {
        count: "not_an_int"
      }
      then {
        output.result == 0
      }
    }
  }
}
