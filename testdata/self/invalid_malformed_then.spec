spec Bad {
  scope broken {
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
