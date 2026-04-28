# Working with OpenAPI Schemas in Speclang

Speclang can import models and contract scaffolds directly from OpenAPI 3.x schema files. This lets you start from an existing API definition and layer verification properties (invariants, scenarios) on top.

## Quick Start

Given an OpenAPI spec `api.yaml`:

```yaml
openapi: "3.0.0"
info:
  title: My API
  version: "1.0.0"
paths:
  /users:
    post:
      operationId: create_user
      requestBody:
        content:
          application/json:
            schema:
              $ref: "#/components/schemas/CreateUserRequest"
      responses:
        "201":
          description: Created
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/User"
components:
  schemas:
    CreateUserRequest:
      type: object
      required: [name]
      properties:
        name:
          type: string
        email:
          type: string
    User:
      type: object
      required: [id, name]
      properties:
        id:
          type: integer
          minimum: 1
        name:
          type: string
        email:
          type: string
```

Write a speclang spec that imports it:

```
http {
  base_url: env(APP_URL, "http://localhost:8080")
}

import openapi("api.yaml")

# The import generates:
#   model CreateUserRequest { name: string  email: string? }
#   model User { id: int { 1 <= id }  name: string  email: string? }
#   scope create_user {
#     contract create_user -> User {
#       name: string
#       email: string?
#       action { return http.post("/users", { name: name, email: email }) }
#     }
#   }
#
# Add invariants and scenarios on top of the imported scaffolds.
```

Then verify:

```bash
specrun verify myapi.spec
```

## What Gets Imported

### Models (from `components/schemas`)

Each OpenAPI schema with `type: object` and `properties` becomes a speclang model.

| OpenAPI | Speclang | Notes |
|---------|----------|-------|
| `type: integer` | `int` | Direct mapping |
| `type: string` | `string` | Direct mapping |
| `type: boolean` | `bool` | Direct mapping |
| `$ref: "#/.../Name"` | `Name` | Model name reference |
| `type: number` | — | Skipped with warning (no float type in v3; v4 has `float` but importer maps to int) |
| `type: array` | — | Skipped with warning |
| `enum` | — | Skipped with warning |

Fields listed in the schema's `required` array become non-optional; others become optional (`type?`).

### Constraints (from validation keywords)

| OpenAPI | Speclang constraint |
|---------|-------------------|
| `minimum: N` | `N <= field` |
| `maximum: N` | `field <= N` |
| `exclusiveMinimum: true` + `minimum: N` | `N < field` |
| `exclusiveMaximum: true` + `maximum: N` | `field < N` |
| Both min and max | Combined with `and` |

### Contracts (from `paths`)

Each path + HTTP method becomes a **scope** containing one **contract**. This is the key v4 change from v3: the importer emits v4 contracts, not v3 scope/input/output structures.

For each operation:

- **Scope name**: `operationId` if present, otherwise `<method>_<sanitized_path>` (e.g., `post_api_v1_users`)
- **Contract name**: same as scope name
- **Contract fields**: request body schema properties → contract input fields
- **Contract return type**: `200` or `201` response schema model name (or `any` if unresolvable)
- **Contract action block**: pre-populated with the HTTP call:
  ```
  action {
    return http.post("/path", { field1: field1, field2: field2 })
  }
  ```
  `GET` and `DELETE` calls omit the body argument.
- **No invariants or scenarios**: those are hand-authored

The `http` adapter config block at spec level is used by the generated action blocks. The importer assumes you have `http { base_url: ... }` declared.

## Adding Verification Properties

Imported scopes are scaffolds. The real value comes from adding invariants and scenarios inside the imported contracts. Because the contracts are generated into the AST, you can't edit them inline — define additional scopes that reference the same models, or use `include` to compose specs:

```
http {
  base_url: env(APP_URL, "http://localhost:8080")
}

import openapi("api.yaml")

# Hand-authored: additional verification on the imported endpoint
scope create_user_extended {
  contract CreateUserIdempotentCheck -> User {
    name: string
    email: string?

    action {
      return http.post("/users", { name: name, email: email })
    }

    # Every created user must have a positive id.
    invariant valid_id {
      output.id > 0
    }

    # Created user name must match what was submitted.
    invariant name_preserved {
      output.name == name
    }

    scenario creates_user {
      given { name: "alice", email: "alice@example.com" }
      then {
        output.id != null
        output.name == "alice"
      }
    }
  }
}
```

## Asymmetry vs. Protobuf Import

**OpenAPI import: action is populated.** The importer knows the transport (HTTP) from the schema, so it generates a complete action block calling `http.post(...)`, `http.get(...)`, etc. The contract is ready to verify as-is (though it has no invariants or scenarios yet).

**Protobuf import: action is nil.** Protobuf schemas describe RPCs but not transport. The importer emits the contract fields and return type, but leaves the action block empty. You must fill in the action with a transport call — either via a process adapter wrapping `grpc_cli`, a custom adapter, or an HTTP transcoding endpoint. This is not a bug; it is a deliberate distinction. See [docs/imports/protobuf.md](protobuf.md).

## Limitations

- **Array types**: OpenAPI `type: array` is not supported. Array fields are skipped with a warning.
- **Float types**: OpenAPI `type: number` is skipped with a warning (the importer currently does not map to `float`).
- **Enum types**: Not supported. Enum fields are skipped with a warning.
- **Composition**: `oneOf`, `anyOf`, `allOf` are not directly supported.
- **External `$ref`**: Only internal references (`#/components/schemas/...`) are resolved.
- **Path parameters**: Not yet mapped to contract input fields.

## Example

See [`examples/openapi/`](../../examples/openapi/) for a complete example importing a Petstore API.

## Technical Details

The import uses [kin-openapi](https://github.com/getkin/kin-openapi) for parsing and `$ref` resolution. The converter produces standard v4 AST nodes (`*parser.Model`, `*parser.Scope`, `*parser.Contract`), making imported schemas indistinguishable from hand-written ones to the generator, runner, and validator.
