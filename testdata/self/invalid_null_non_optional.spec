spec Bad {
  scope broken {
    contract {
      input {
        name: string
      }
      output {
        result: int
      }
    }
    scenario smoke {
      given {
        name: null
      }
      then {
        result == 0
      }
    }
  }
}
