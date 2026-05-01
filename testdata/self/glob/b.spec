process {
  command: "echo"
}

model GlobEchoResult2 {
  exit_code: int
}

contract GlobEchoB() -> GlobEchoResult2 {
  action {
    let result = process.exec("b")
    return result
  }

  invariant exits_ok {
    output.exit_code == 0
  }
}
