spec ExistsTest {
  scope test {
    use process
    contract {
      input { name: string }
      output { status: string }
    }
    invariant has_name {
      exists(output.name)
    }
  }
}
