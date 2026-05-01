# Working with Protobuf Schemas in Speclang

Speclang can import models and contract scaffolds directly from `.proto` files. This lets you start from an existing protobuf service definition and layer verification properties on top.

## Quick Start

Given a protobuf file `api.proto`:

```protobuf
syntax = "proto3";

message User {
  int64 id = 1;
  string name = 2;
  optional string email = 3;
}

message CreateUserRequest {
  string name = 1;
  string email = 2;
}

message CreateUserResponse {
  User user = 1;
  bool success = 2;
}

service UserService {
  rpc CreateUser(CreateUserRequest) returns (CreateUserResponse);
}
```

Write a speclang spec that imports it:

```
import proto("api.proto")

# The import generates:
#   model User { id: int  name: string  email: string? }
#   model CreateUserRequest { name: string  email: string }
#   model CreateUserResponse { user: User  success: bool }
#   scope CreateUser {
#     contract CreateUser(name: string, email: string) -> CreateUserResponse {
#       # action is empty — YOU must fill this in
#     }
#   }
#
# Fill in the action block with your transport call, then add invariants and scenarios.
```

Then verify (after completing the action block):

```bash
specrun verify userapi.spec
```

## What Gets Imported

### Models (from `message` definitions)

Each protobuf message becomes a speclang model.

| Protobuf type | Speclang type | Notes |
|---|---|---|
| `int32`, `int64`, `sint32`, `sint64` | `int` | All integer types collapse to `int` |
| `uint32`, `uint64`, `fixed32`, `fixed64` | `int` | Loss of sign/size distinction |
| `sfixed32`, `sfixed64` | `int` | |
| `string` | `string` | Direct |
| `bool` | `bool` | Direct |
| message reference | model name | e.g., `User` |
| `optional T` | `T?` | Optional field |
| `float`, `double` | — | Skipped with warning |
| `bytes` | — | Skipped with warning |
| `repeated T` | — | Skipped with warning |
| `map<K,V>` | — | Skipped with warning |
| `oneof` | — | Skipped with warning |
| `enum` | — | Skipped with warning |

**Note**: No constraints are generated from protobuf fields — protobuf does not encode numeric ranges natively.

### Nested Messages

Nested messages are flattened to top-level models with `Parent_Child` naming:

```protobuf
message SearchResponse {
  message Result {
    string url = 1;
  }
}
```

Produces models `SearchResponse` and `SearchResponse_Result`.

### Well-Known Types

| Type | Mapping |
|---|---|
| `google.protobuf.Timestamp` | `string` |
| `google.protobuf.Duration` | `string` |
| `google.protobuf.Empty` | omit (empty contract side) |
| `google.protobuf.BoolValue` | `bool?` |
| `google.protobuf.StringValue` | `string?` |
| `google.protobuf.Int32Value` / `Int64Value` | `int?` |
| `google.protobuf.Any` / `Struct` / `Value` | Skipped |

### Contracts (from `service` definitions)

Each **unary** RPC method becomes a **scope** containing one **contract**. This is the key v4 change from v3: the importer emits v4 contracts, not v3 scope/input/output structures.

For each RPC:

- **Scope name**: RPC method name (e.g., `CreateUser`)
- **Contract name**: same as scope name
- **Contract fields**: request message fields (or model inheritance via `contract Name: RequestModel`)
- **Contract return type**: response message model name
- **Contract action block**: **nil** — the importer does not know your transport
- **No invariants or scenarios**: those are hand-authored

**Streaming RPCs**: Skipped with warning. Speclang contracts are strictly request → response.

## Asymmetry vs. OpenAPI Import

**Protobuf import: action is nil.** This is intentional, not a bug.

OpenAPI schemas encode transport (HTTP methods, paths) alongside the schema, so the OpenAPI importer can generate a complete `action { return http.post(...) }` block automatically.

Protobuf schemas encode service contracts but not transport. A protobuf service might be called via:

- Native gRPC (e.g., using a `grpc_cli` wrapper via the process adapter)
- HTTP/JSON transcoding (e.g., gRPC-gateway, Connect, Twirp)
- A custom adapter

The importer leaves the action block empty, and you fill it in with the correct transport. Example patterns:

**Pattern 1: HTTP transcoding (gRPC-gateway or Connect)**
```
http {
  base_url: env(API_URL, "http://localhost:8080")
}

import proto("api.proto")

# After import, extend the generated contract by writing a companion spec:
scope CreateUser_verified {
  contract CreateUserVerified(
    name: string,
    email: string,
  ) -> CreateUserResponse {
    action {
      return http.post("/UserService/CreateUser", { name: name, email: email })
    }

    invariant created_user_id_positive {
      output.success == true implies output.user.id > 0
    }

    scenario creates_successfully {
      given { name: "alice", email: "alice@example.com" }
      then {
        output.success == true
        output.user.name == "alice"
      }
    }
  }
}
```

**Pattern 2: Process adapter wrapping `grpc_cli`**
```
process {
  command: env(GRPC_CLI_BIN, "grpc_cli")
}

import proto("api.proto")

scope CreateUser_via_cli {
  contract CreateUserCliContract(
    name: string,
    email: string,
  ) -> CreateUserResponse {
    action {
      return process.exec("call", "localhost:50051", "UserService.CreateUser",
        "name: '" + name + "', email: '" + email + "'")
    }

    scenario creates_successfully {
      given { name: "alice", email: "alice@example.com" }
      then { output.exit_code == 0 }
    }
  }
}
```

## Limitations

- **No constraints**: Protobuf doesn't encode min/max. All fields are unconstrained.
- **No float/double**: `float` and `double` fields are skipped.
- **No repeated/array**: `repeated` fields are skipped.
- **No map**: `map<K,V>` fields are skipped.
- **No oneof/enum**: No union or enum types.
- **No bytes**: Binary fields are skipped.
- **Single-file only**: Cross-file `import` in proto files is not resolved.
- **Streaming RPCs**: Cannot be expressed in speclang's contract model.
- **Action block nil**: You must complete the action block after importing.

## Example

See [`examples/proto/`](../../examples/proto/) for a complete example importing a User service.

## Technical Details

The import uses [go-protoparser](https://github.com/yoheimuta/go-protoparser) for `.proto` file parsing (zero external dependencies, no `protoc` required). The converter produces standard v4 AST nodes (`*parser.Model`, `*parser.Scope`, `*parser.Contract`), making imported schemas indistinguishable from hand-written ones to the generator, runner, and validator — except that the contract's action block is nil until you fill it in.
