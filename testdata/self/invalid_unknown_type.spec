spec Bad {
  scope broken {
    use http
    contract {
      input {
        item: Widget
      }
      output {
        result: int
      }
    }
  }
}
