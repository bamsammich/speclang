use http
spec ProtoImportTest {
  target {
    base_url: "http://localhost:8080"
  }
  import proto("user.proto")
}
