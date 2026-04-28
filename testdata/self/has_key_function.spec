process {
  command: "echo"
}

model HasKeyResult {
  status: string
}

scope test {
  contract HasKeyContract -> HasKeyResult {
    name: string

    action {
      let result = process.exec(name)
      return result
    }

    invariant check_key {
      has_key(output, "status")
    }
  }
}
