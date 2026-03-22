scope transfer {
  use http
  config {
    path: "/b"
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
