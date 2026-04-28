model MissingFieldResult {
  result: int
}

scope broken {
  contract MissingFieldContract -> MissingFieldResult {
    from: string
    to: string

    action {
      return http.get("/test")
    }

    scenario smoke {
      given {
        from: "alice"
      }
      then {
        output.result == 0
      }
    }
  }
}
