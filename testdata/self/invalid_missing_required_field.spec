model MissingFieldResult {
  result: int
}

scope broken {
  contract MissingFieldContract(from: string, to: string) -> MissingFieldResult {
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
