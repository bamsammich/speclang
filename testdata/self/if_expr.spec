http {
  base_url: "http://localhost:8080"
}

model IfExprResult {
  result: int
}

scope s {
  contract IfExprContract -> IfExprResult {
    x: int

    action {
      let result = http.post("/test", { x: x })
      return result
    }

    invariant conditional_value {
      if x > 10 then output.result == x - 10 else output.result == 0
    }
  }
}
