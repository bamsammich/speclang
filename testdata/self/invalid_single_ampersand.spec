spec Bad {
  scope broken {
    use http
    contract {
      input { x: int }
      output { y: int }
    }
    invariant bad_op {
      x > 0 & x < 10
    }
  }
}
