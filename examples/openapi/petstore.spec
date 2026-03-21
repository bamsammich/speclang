use http

spec PetstoreAPI {
  description: "Pet store API imported from OpenAPI schema"

  target {
    base_url: env(APP_URL, "http://localhost:8080")
  }

  # Import models and scope scaffolds from OpenAPI schema.
  import openapi("petstore.yaml")

  # The import generates:
  #   model Pet { id: int { 1 <= id }  name: string  tag: string? }
  #   scope create_pet { config { path: "/pets"  method: "POST" } contract { ... } }
  #   scope list_pets  { config { path: "/pets"  method: "GET" }  contract { ... } }
  #
  # Add invariants and scenarios on top of the imported scaffolds.
}
