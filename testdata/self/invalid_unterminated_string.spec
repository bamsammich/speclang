spec Bad {
  scope broken {
    use http
    config {
      path: "/unterminated
    }
  }
}
