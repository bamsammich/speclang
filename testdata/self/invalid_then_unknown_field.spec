spec Bad {
  scope broken {
    contract {
      input {
        x: int
      }
      output {
        result: int
      }
    }
    scenario smoke {
      given {
        x: 1
      }
      then {
        typo_field == 0
      }
    }
  }
}
