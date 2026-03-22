scope transfer {
  use http
  config {
    path: "/a"
    method: "POST"
  }
  contract {
    input {
      x: int
    }
    output {
      y: int
    }
  }
}
