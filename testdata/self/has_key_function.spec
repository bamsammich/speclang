process {
  command: "echo"
}

model HasKeyResult {
  status: string
}

scope test {
  contract HasKeyContract(name: string) -> HasKeyResult {
    action {
      let result = process.exec(name)
      return result
    }

    invariant check_key {
      has_key(output, "status")
    }
  }
}
