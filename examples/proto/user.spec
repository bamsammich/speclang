use http

spec UserAPI {
  description: "User service API imported from protobuf schema"

  target {
    base_url: env(APP_URL, "http://localhost:8080")
  }

  # Import models and scope scaffolds from protobuf schema.
  import proto("user.proto")

  # The import generates:
  #   model User { email: string  id: int  name: string  phone: string? }
  #   model CreateUserRequest { email: string  name: string }
  #   model CreateUserResponse { success: bool  user: User }
  #   model GetUserRequest { id: int }
  #   model GetUserResponse { user: User }
  #   scope CreateUser { config { service: "UserService"  method: "CreateUser" } contract { ... } }
  #   scope GetUser { config { service: "UserService"  method: "GetUser" } contract { ... } }
  #
  # Add invariants and scenarios on top of the imported scaffolds.
}
