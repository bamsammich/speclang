spec Bad {
  scope broken {
    use http
    contract {
      input { x: int }
      output { y: int }
    }
    scenario test {
      given { x: 1 }
      then {
        scenario nested {}
      }
    }
  }
}
