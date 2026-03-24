# Working with Protobuf Schemas in Speclang

Speclang can import models and scope scaffolds directly from `.proto` files. This lets you start from an existing protobuf service definition and layer verification properties on top.

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
use http

spec UserAPI {
  target {
    base_url: env(APP_URL, "http://localhost:8080")
  }

  import proto("api.proto")
}
```

Then verify:

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

### Scopes (from `service` definitions)

Each **unary** RPC method becomes a scope:

- **Scope name**: RPC method name (e.g., `CreateUser`)
- **Config**: `service` and `method` populated from the service/RPC names
- **Contract input**: Fields from the request message
- **Contract output**: Fields from the response message
- **Streaming RPCs**: Skipped with warning (speclang contracts are strictly request → response)
- **No invariants or scenarios**: Those are hand-authored

## Limitations

- **No constraints**: Protobuf doesn't encode min/max. All fields are unconstrained.
- **No float/double**: Speclang has no float type (#21).
- **No repeated/array**: Speclang has no array type (#21).
- **No map**: Speclang has no map type (#21).
- **No oneof/enum**: No union or enum types.
- **No bytes**: No binary type.
- **Single-file only**: Cross-file `import` in proto files is not resolved.
- **Streaming RPCs**: Cannot be expressed in speclang's contract model.

## Example

See [`examples/proto/`](../examples/proto/) for a complete example importing a User service.

## Technical Details

The import uses [go-protoparser](https://github.com/yoheimuta/go-protoparser) for `.proto` file parsing (zero external dependencies, no `protoc` required). The converter produces standard speclang AST nodes, making imported schemas indistinguishable from hand-written ones to the generator, runner, and adapter.
