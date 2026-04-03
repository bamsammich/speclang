spec ExistsTest {
  process {
    command: "echo"
  }

  scope test {
    action run(name: string) {
      let result = process.exec(name)
      return result
    }

    contract {
      input { name: string }
      output { status: string }
      action: run
    }

    invariant has_name {
      exists(output.name)
    }
  }
}
