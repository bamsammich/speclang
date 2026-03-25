# Target Services

Declare Docker containers as test infrastructure directly in your spec. `specrun verify` manages the full container lifecycle so specs are self-contained -- no manual server startup required.

## Why Services?

Without services, you must start servers manually before running `specrun verify`:

```bash
go run ./server &
APP_URL=http://localhost:8080 specrun verify transfer.spec
```

With services, the spec declares its own infrastructure:

```
spec TransferAPI {
  target {
    services {
      app {
        build: "./server"
        port: 8080
      }
    }
    base_url: service(app)
  }
  # ...
}
```

Now `specrun verify transfer.spec` builds, starts, health-checks, verifies, and tears down the container automatically. Docker must be available on the host.

## Inline Services

Each service has a name and a set of configuration fields:

```
target {
  services {
    <name> {
      build: "<dockerfile-dir>"     # OR image: "<docker-image>"
      port: <container-port>        # optional
      health: "<http-path>"         # optional
      env { KEY: "value" }          # optional
      volumes {                     # optional
        "<host-path>": "<container-path>"
      }
    }
  }
}
```

### Fields

| Field | Required | Description |
|-------|----------|-------------|
| `build` | One of build/image | Directory containing a Dockerfile (relative to spec file) |
| `image` | One of build/image | Pre-built Docker image (e.g., `"postgres:16"`) |
| `port` | No | Container port to expose. Static host mapping when specified; dynamic allocation when omitted |
| `health` | No | HTTP GET path for health check. Falls back to TCP port check when absent |
| `env` | No | Environment variables passed to the container |
| `volumes` | No | Host-to-container bind mounts (paths relative to spec file) |

### Example: Simple HTTP Server

```
target {
  services {
    app {
      build: "./server"
      port: 8080
    }
  }
  base_url: service(app)
}
```

### Example: Database with Volumes

```
target {
  services {
    db {
      image: "postgres:16"
      port: 5432
      env {
        POSTGRES_PASSWORD: "test"
        POSTGRES_DB: "testdb"
      }
      volumes {
        "./fixtures/seed.sql": "/docker-entrypoint-initdb.d/seed.sql"
      }
    }
    app {
      build: "./server"
      port: 8080
      env { DATABASE_URL: "postgres://postgres:test@db:5432/testdb" }
    }
  }
  base_url: service(app)
}
```

## Compose Support

For complex multi-service setups, reference a docker-compose file instead of inline definitions:

```
target {
  services {
    compose: "docker-compose.yml"
  }
  base_url: service(app)
}
```

The compose path is relative to the spec file. Service names in `service()` references must match the service names defined in the compose file. `specrun` delegates to `docker compose up/down` for lifecycle management.

## `service(name)` Resolution

The `service(name)` expression resolves at runtime to `http://localhost:<port>`, where `<port>` is the actual host-mapped port of the named container.

- The name must match a service declared in the `services` block
- Unknown service names are rejected during spec validation (before any containers start)
- `service()` can be used anywhere a URL is expected: `base_url`, `command`, or other target fields

```
target {
  services {
    app { build: "./server"  port: 8080 }
  }
  base_url: service(app)     # resolves to e.g. "http://localhost:8080"
}
```

## Container Lifecycle

When `specrun verify` encounters a spec with declared services:

1. **Pre-flight cleanup** -- Remove stale containers from previous runs. Containers are labeled with the spec name so only related containers are affected.
2. **Build/pull** -- Build images from Dockerfiles or pull pre-built images as needed.
3. **Start** -- Start containers with configured port mappings, environment variables, and volumes.
4. **Health check** -- Wait for each service to become healthy:
   - If `health` is specified: HTTP GET to the health path until 200
   - If `health` is not specified: TCP connection check on the mapped port
   - Timeout causes verification to abort with a clear error
5. **Verify** -- Run the spec verification (scenarios, invariants)
6. **Teardown** -- Stop and remove containers, unless `--keep-services` is set
7. **Signal handling** -- SIGINT/SIGTERM triggers cleanup before exit

## CLI Flags

### `--keep-services`

Leave containers running after verification completes. Useful for debugging failed specs -- you can inspect the running service, check logs, or make requests manually.

```bash
specrun verify transfer.spec --keep-services
docker logs specrun-transfer-app    # inspect logs
docker stop specrun-transfer-app    # manual cleanup when done
```

## Volume Mounts

Use volumes to inject fixture data (seed files, config, etc.) into containers:

```
target {
  services {
    db {
      image: "postgres:16"
      port: 5432
      volumes {
        "./testdata/seed.sql": "/docker-entrypoint-initdb.d/seed.sql"
      }
    }
  }
}
```

Host paths are relative to the spec file's directory and resolved to absolute paths before mounting.
