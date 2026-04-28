process {
  command: "echo"
}

model GlobEchoNested {
  exit_code: int
}

contract GlobEchoNested -> GlobEchoNested {
  action {
    let result = process.exec("nested")
    return result
  }

  invariant exits_ok {
    output.exit_code == 0
  }
}
