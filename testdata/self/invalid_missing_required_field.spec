spec Bad {
  scope broken {
    contract {
      input {
        from: string
        to: string
      }
      output {
        result: int
      }
    }
    scenario smoke {
      given {
        from: "alice"
      }
      then {
        result == 0
      }
    }
  }
}
