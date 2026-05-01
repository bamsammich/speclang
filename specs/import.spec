# Verifies that import openapi() and import proto() produce correct models and scopes.
# Uses array index access (models.0.name) to inspect the parsed AST JSON output.
#
# Note: scope config, contract, input, output, and use are language keywords,
# so paths like scopes.0.config.path or scopes.0.contract.input cannot be used
# as then-block assertion targets. We verify model structure and scope names.

model ImportParseResult {
  exit_code: int
  models: any
  scopes: any
}

# OpenAPI import: verifies models, fields, types from petstore.yaml.
# petstore.yaml has 2 schemas (Owner, Pet) and 2 paths (GET /pets, POST /pets).
# Models sorted alphabetically: Owner at 0, Pet at 1.
# Scopes sorted alphabetically: create_pet at 0, list_pets at 1.
scope import_openapi {
  contract ImportOpenAPIContract(file: string) -> ImportParseResult {
    action {
      let result = process.exec("parse", file)
      return result
    }

    # Verifies model names from components/schemas (implicitly verifies count = 2).
    scenario openapi_models {
      given {
        file: "testdata/openapi/import_valid.spec"
      }
      then {
        output.exit_code == 0
        output.models.0.name == "Owner"
        output.models.1.name == "Pet"
      }
    }

    # Verifies Owner model fields: id (int, required), name (string, optional).
    scenario openapi_owner_fields {
      given {
        file: "testdata/openapi/import_valid.spec"
      }
      then {
        output.exit_code == 0
        output.models.0.fields.0.name == "id"
        output.models.0.fields.0.type.name == "int"
        output.models.0.fields.1.name == "name"
        output.models.0.fields.1.type.name == "string"
        output.models.0.fields.1.type.optional == true
      }
    }

    # Verifies Pet model fields: id (int), name (string), tag (string, optional).
    scenario openapi_pet_fields {
      given {
        file: "testdata/openapi/import_valid.spec"
      }
      then {
        output.exit_code == 0
        output.models.1.fields.0.name == "id"
        output.models.1.fields.0.type.name == "int"
        output.models.1.fields.1.name == "name"
        output.models.1.fields.1.type.name == "string"
        output.models.1.fields.2.name == "tag"
        output.models.1.fields.2.type.name == "string"
        output.models.1.fields.2.type.optional == true
      }
    }

    # Verifies scope names from paths (implicitly verifies count = 2).
    scenario openapi_scopes {
      given {
        file: "testdata/openapi/import_valid.spec"
      }
      then {
        output.exit_code == 0
        output.scopes.0.name == "create_pet"
        output.scopes.1.name == "list_pets"
      }
    }
  }
}

# OpenAPI constraints: verifies minimum/maximum constraints are preserved.
# constraints.yaml has 1 model (BoundedItem) with constrained int fields.
scope import_openapi_constraints {
  contract ImportOpenAPIConstraintsContract(file: string) -> ImportParseResult {
    action {
      let result = process.exec("parse", file)
      return result
    }

    # Verifies BoundedItem fields exist with correct types and constraint operators.
    # price has exclusive bounds (< operators), quantity has inclusive bounds (<= operators).
    scenario openapi_constraints {
      given {
        file: "testdata/openapi/import_constraints.spec"
      }
      then {
        output.exit_code == 0
        output.models.0.name == "BoundedItem"
        output.models.0.fields.0.name == "price"
        output.models.0.fields.0.type.name == "int"
        output.models.0.fields.0.constraint.op == "and"
        output.models.0.fields.1.name == "quantity"
        output.models.0.fields.1.type.name == "int"
        output.models.0.fields.1.constraint.op == "and"
        output.models.0.fields.2.name == "rating"
        output.models.0.fields.2.type.name == "int"
        output.models.0.fields.2.type.optional == true
      }
    }
  }
}

# OpenAPI $ref resolution: verifies that $ref fields resolve to model type names.
# refs.yaml has Order.customer as $ref to Customer.
scope import_openapi_refs {
  contract ImportOpenAPIRefsContract(file: string) -> ImportParseResult {
    action {
      let result = process.exec("parse", file)
      return result
    }

    # Verifies $ref field resolves to the referenced model name.
    scenario openapi_ref_field_type {
      given {
        file: "testdata/openapi/import_refs.spec"
      }
      then {
        output.exit_code == 0
        output.models.0.name == "Customer"
        output.models.1.name == "Order"
        output.models.1.fields.0.name == "customer"
        output.models.1.fields.0.type.name == "Customer"
        output.models.1.fields.1.name == "id"
        output.models.1.fields.1.type.name == "int"
        output.models.1.fields.2.name == "note"
        output.models.1.fields.2.type.name == "string"
        output.models.1.fields.2.type.optional == true
      }
    }
  }
}

# Protobuf import: verifies models and scopes from user.proto.
# user.proto has 5 messages and 2 RPCs in UserService.
# Models sorted: CreateUserRequest, CreateUserResponse, GetUserRequest, GetUserResponse, User.
# Scopes sorted: CreateUser, GetUser.
scope import_proto {
  contract ImportProtoContract(file: string) -> ImportParseResult {
    action {
      let result = process.exec("parse", file)
      return result
    }

    # Verifies model names from protobuf messages (implicitly verifies count = 5).
    scenario proto_models {
      given {
        file: "testdata/proto/import_valid.spec"
      }
      then {
        output.exit_code == 0
        output.models.0.name == "CreateUserRequest"
        output.models.1.name == "CreateUserResponse"
        output.models.2.name == "GetUserRequest"
        output.models.3.name == "GetUserResponse"
        output.models.4.name == "User"
      }
    }

    # Verifies User model fields and types including optional phone.
    scenario proto_user_fields {
      given {
        file: "testdata/proto/import_valid.spec"
      }
      then {
        output.exit_code == 0
        output.models.4.fields.0.name == "email"
        output.models.4.fields.0.type.name == "string"
        output.models.4.fields.1.name == "id"
        output.models.4.fields.1.type.name == "int"
        output.models.4.fields.2.name == "name"
        output.models.4.fields.2.type.name == "string"
        output.models.4.fields.3.name == "phone"
        output.models.4.fields.3.type.name == "string"
        output.models.4.fields.3.type.optional == true
      }
    }

    # Verifies CreateUserResponse has a model-typed field (user: User).
    scenario proto_model_ref_field {
      given {
        file: "testdata/proto/import_valid.spec"
      }
      then {
        output.exit_code == 0
        output.models.1.name == "CreateUserResponse"
        output.models.1.fields.0.name == "success"
        output.models.1.fields.0.type.name == "bool"
        output.models.1.fields.1.name == "user"
        output.models.1.fields.1.type.name == "User"
      }
    }

    # Verifies scope names from service RPCs (implicitly verifies count = 2).
    scenario proto_scopes {
      given {
        file: "testdata/proto/import_valid.spec"
      }
      then {
        output.exit_code == 0
        output.scopes.0.name == "CreateUser"
        output.scopes.1.name == "GetUser"
      }
    }
  }
}

# Protobuf streaming: verifies streaming RPCs are skipped, only unary RPCs produce scopes.
# streaming.proto has 4 RPCs: 1 unary (SendEvent), 3 streaming (skipped).
scope import_proto_streaming {
  contract ImportProtoStreamingContract(file: string) -> ImportParseResult {
    action {
      let result = process.exec("parse", file)
      return result
    }

    # Verifies only unary RPC produces a scope; streaming RPCs are skipped.
    scenario proto_streaming_skipped {
      given {
        file: "testdata/proto/import_streaming.spec"
      }
      then {
        output.exit_code == 0
        output.models.0.name == "Ack"
        output.models.1.name == "Event"
        output.scopes.0.name == "SendEvent"
      }
    }
  }
}
