spec Bad {
  scope broken {
    use http
    contract {
      input {
        count: int
      }
      output {
        result: int
      }
    }
    scenario smoke {
      given {
        count: "not_an_int"
      }
      then {
        result: 0
      }
    }
  }
}
