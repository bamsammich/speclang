spec Bad {
  scope broken {
    use http
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
        result: 0
      }
    }
  }
}
