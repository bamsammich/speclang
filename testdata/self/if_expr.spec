spec IfExprTest {
  scope s {
    use http
    config {
      path: "/test"
      method: "POST"
    }
    contract {
      input {
        x: int
      }
      output {
        result: int
      }
    }
    invariant conditional_value {
      if x > 10 then output.result == x - 10 else output.result == 0
    }
  }
}
