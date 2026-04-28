spec OperatorSwap {
  model Foo {
    x: int
    y: int
  }

  scope check {
    action check(x: int, y: int) {
      let result = http.post("/check", { x: x, y: y })
      return result
    }

    contract {
      input {
        x: int
        y: int
      }
      output {
        ok: bool
      }
      action: check
    }

    invariant both_positive {
      x > 0 && y > 0
    }

    invariant either_zero {
      x == 0 || y == 0
    }
  }
}
