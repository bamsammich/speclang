spec Bad {
  scope broken {
    use http
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
        result: 0
      }
    }
  }
}
