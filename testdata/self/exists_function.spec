process {
  command: "echo"
}

model ExistsResult {
  status: string
}

scope test {
  contract ExistsContract(name: string) -> ExistsResult {
    action {
      let result = process.exec(name)
      return result
    }

    invariant has_name {
      exists(output.name)
    }
  }
}
