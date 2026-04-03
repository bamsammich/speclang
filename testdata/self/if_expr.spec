spec IfExprTest {
  http {
    base_url: "http://localhost:8080"
  }

  scope s {
    action run(x: int) {
      let result = http.post("/test", { x: x })
      return result
    }

    contract {
      input {
        x: int
      }
      output {
        result: int
      }
      action: run
    }

    invariant conditional_value {
      if x > 10 then output.result == x - 10 else output.result == 0
    }
  }
}
