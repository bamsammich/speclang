# Working with OpenAPI Schemas in Speclang

Speclang can import models and scope scaffolds directly from OpenAPI 3.x schema files. This lets you start from an existing API definition and layer verification properties (invariants, scenarios) on top.

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
              $ref: "#/components/schemas/User"
      responses:
        "201":
          description: Created
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/User"
components:
  schemas:
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
use http

spec MyAPI {
  target {
    base_url: env(APP_URL, "http://localhost:8080")
  }

  import openapi("api.yaml")

  # The import generates:
  #   model User { id: int { 1 <= id }  name: string  email: string? }
  #   scope create_user { config { path: "/users"  method: "POST" }  contract { ... } }
}
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
| `type: number` | `int` | Mapped with warning (no float type) |
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
| Both min and max | Combined with `&&` |

### Scopes (from `paths`)

Each path + HTTP method becomes a scope:

- **Scope name**: `operationId` if present, otherwise `<method>_<sanitized_path>` (e.g., `post_api_v1_users`)
- **Config**: `path` and `method` populated from the OpenAPI path and HTTP method
- **Contract input**: Fields from the `requestBody` JSON schema
- **Contract output**: Fields from the `200` or `201` response JSON schema
- **No invariants or scenarios**: Those are hand-authored

## Adding Verification Properties

Imported scopes are scaffolds. The real value comes from adding invariants and scenarios:

```
use http

spec MyAPI {
  target {
    base_url: env(APP_URL, "http://localhost:8080")
  }

  import openapi("api.yaml")

  # You can reference imported models in hand-written scopes,
  # or add invariants/scenarios to imported scopes by defining
  # them in separate included files.
}
```

Since invariants and scenarios must live inside scope blocks, and imported scopes are generated without them, you can define additional scopes that share the same contract or write complementary scopes in included files.

## Limitations

- **Array types**: OpenAPI `type: array` is not supported in speclang. Array fields are skipped with a warning.
- **Float types**: OpenAPI `type: number` (float) is mapped to `int` with a warning.
- **Enum types**: Not supported. Enum fields are mapped to their base type without value enforcement.
- **Composition**: `oneOf`, `anyOf`, `allOf` are not directly supported.
- **External $ref**: Only internal references (`#/components/schemas/...`) are resolved. File-based `$ref` is not supported.
- **Path parameters**: Not yet mapped to contract input fields.

## Example

See [`examples/openapi/`](../examples/openapi/) for a complete example importing a Petstore API.

## Technical Details

The import uses [kin-openapi](https://github.com/getkin/kin-openapi) for parsing and `$ref` resolution, so it stays current with OpenAPI spec evolution. The converter produces standard speclang AST nodes (`*Model`, `*Scope`), making imported schemas indistinguishable from hand-written ones to the generator, runner, and adapter.
