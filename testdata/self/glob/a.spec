process {
  command: "echo"
}

model GlobEchoResult {
  exit_code: int
}

contract GlobEchoA() -> GlobEchoResult {
  action {
    let result = process.exec("a")
    return result
  }

  invariant exits_ok {
    output.exit_code == 0
  }
}
