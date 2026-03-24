spec HasKeyTest {
  scope test {
    use process
    contract {
      input { name: string }
      output { status: string }
    }
    invariant check_key {
      has_key(output, "status")
    }
  }
}
