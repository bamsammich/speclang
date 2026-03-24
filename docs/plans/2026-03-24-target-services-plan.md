# Target Services Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add Docker container lifecycle management to the `target` block so specs can declare their own test infrastructure via `services` and `service()`, making verification fully self-contained.

**Architecture:** New `services` sub-block in `target` with `service(name)` expression for URL resolution. New `pkg/infra/` package handles container lifecycle via Docker Go SDK. Runner starts services before verification, cleans up after (or on signal). Compose support via `docker compose` CLI. Existing adapters are unaware of Docker — they receive resolved URLs.

**Tech Stack:** Go, Docker Go SDK (`github.com/docker/docker/client`), `os/exec` for compose CLI, `os/signal` for cleanup

---

## Phase 1: Parser — `service()` expression and `services` block

### Task 1: Add `ServiceRef` AST node and `TokenService` keyword

**Files:**
- Modify: `pkg/parser/ast.go`
- Modify: `pkg/parser/lexer.go`

**Step 1: Add `ServiceRef` AST node**

In `pkg/parser/ast.go`, add after the `EnvRef` struct:

```go
// ServiceRef references a named service from the target services block.
// Resolves to the service's URL at runtime (e.g., "http://localhost:8080").
// Docker must be available; if not, specrun errors. Use env() or a literal string
// instead of service() for non-Docker environments.
type ServiceRef struct {
	Name string `json:"name"`
}

func (ServiceRef) exprNode() {}
```

**Step 2: Add `TokenService` to lexer**

In `pkg/parser/lexer.go`:
- Add `TokenService` to the const block (in the Keywords section, after `TokenElse`)
- Add `TokenService: "Service"` to `tokenNames`
- Add `"service": TokenService` to `keywords` map

**Step 3: Verify**

Run: `go build ./...`
Expected: compiles (no consumers yet)

**Step 4: Commit**

```bash
git add pkg/parser/ast.go pkg/parser/lexer.go
git commit -m "feat(parser): add ServiceRef AST node and TokenService keyword"
```

---

### Task 2: Parse `service(name)` expressions

**Files:**
- Modify: `pkg/parser/parser.go`
- Test: `pkg/parser/parser_test.go` (or new file `pkg/parser/service_test.go`)

**Step 1: Write failing test**

```go
func TestParseServiceRef(t *testing.T) {
	t.Parallel()
	src := `spec Test {
  target {
    base_url: service(app)
  }
}`
	spec, err := Parse(src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expr, ok := spec.Target.Fields["base_url"]
	if !ok {
		t.Fatal("missing base_url field")
	}
	ref, ok := expr.(ServiceRef)
	if !ok {
		t.Fatalf("expected ServiceRef, got %T", expr)
	}
	if ref.Name != "app" {
		t.Fatalf("expected name=app, got %q", ref.Name)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./pkg/parser/ -run TestParseServiceRef -v`
Expected: FAIL

**Step 3: Implement `parseServiceRef`**

In `pkg/parser/parser.go`, add method:

```go
func (p *parser) parseServiceRef() (Expr, error) {
	p.advance() // consume "service"
	if _, err := p.expect(TokenLParen); err != nil {
		return nil, err
	}
	name, err := p.expectIdent()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(TokenRParen); err != nil {
		return nil, err
	}
	return ServiceRef{Name: name.Value}, nil
}
```

In `parseAtom`, add case for `TokenService` (alongside `TokenEnv`):

```go
case TokenService:
	return p.parseServiceRef()
```

**Step 4: Run test to verify it passes**

Run: `go test ./pkg/parser/ -run TestParseServiceRef -v`
Expected: PASS

**Step 5: Commit**

```bash
git add pkg/parser/parser.go pkg/parser/service_test.go
git commit -m "feat(parser): parse service(name) expressions"
```

---

### Task 3: Add `Service` struct and parse `services` block in target

**Files:**
- Modify: `pkg/parser/ast.go`
- Modify: `pkg/parser/parser.go`
- Test: `pkg/parser/service_test.go`

**Step 1: Add `Service` struct to AST**

In `pkg/parser/ast.go`:

```go
// Service declares a container to run as test infrastructure.
type Service struct {
	Name    string            `json:"name"`
	Build   string            `json:"build,omitempty"`   // Dockerfile directory path
	Image   string            `json:"image,omitempty"`   // pre-built image name
	Port    int               `json:"port,omitempty"`    // static host port (0 = dynamic)
	Health  string            `json:"health,omitempty"`  // HTTP health check path
	Env     map[string]string `json:"env,omitempty"`     // container environment variables
	Volumes map[string]string `json:"volumes,omitempty"` // host:container path mounts
}
```

Add `Services` and `Compose` fields to `Target`:

```go
type Target struct {
	Fields   map[string]Expr `json:"fields,omitempty"`
	Services []*Service      `json:"services,omitempty"`
	Compose  string          `json:"compose,omitempty"` // path to docker-compose.yml
}
```

**Step 2: Write failing test for services block parsing**

```go
func TestParseTargetServices(t *testing.T) {
	t.Parallel()
	src := `spec Test {
  target {
    services {
      app {
        build: "./examples/server"
        port: 8080
        health: "/health"
      }
      db {
        image: "postgres:15"
        port: 5432
        env {
          POSTGRES_PASSWORD: "test"
        }
        volumes {
          "./schema.sql": "/docker-entrypoint-initdb.d/schema.sql"
        }
      }
    }
    base_url: service(app)
  }
}`
	spec, err := Parse(src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(spec.Target.Services) != 2 {
		t.Fatalf("expected 2 services, got %d", len(spec.Target.Services))
	}
	app := spec.Target.Services[0]
	if app.Name != "app" || app.Build != "./examples/server" || app.Port != 8080 || app.Health != "/health" {
		t.Fatalf("unexpected app service: %+v", app)
	}
	db := spec.Target.Services[1]
	if db.Name != "db" || db.Image != "postgres:15" || db.Port != 5432 {
		t.Fatalf("unexpected db service: %+v", db)
	}
	if db.Env["POSTGRES_PASSWORD"] != "test" {
		t.Fatalf("expected POSTGRES_PASSWORD=test, got %v", db.Env)
	}
	if db.Volumes["./schema.sql"] != "/docker-entrypoint-initdb.d/schema.sql" {
		t.Fatalf("unexpected volumes: %v", db.Volumes)
	}
}

func TestParseTargetCompose(t *testing.T) {
	t.Parallel()
	src := `spec Test {
  target {
    services {
      compose: "./docker-compose.test.yml"
    }
    base_url: service(api)
  }
}`
	spec, err := Parse(src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if spec.Target.Compose != "./docker-compose.test.yml" {
		t.Fatalf("expected compose path, got %q", spec.Target.Compose)
	}
}
```

**Step 3: Implement services block parsing in `parseTarget`**

Modify `parseTarget` in `pkg/parser/parser.go` to detect a `services` key and parse the sub-block. When the key is `services`, parse `{ ... }` containing either `compose: "path"` or named service blocks. Each service block contains keys: `build`, `image`, `port`, `health`, `env { k: v }`, `volumes { k: v }`.

**Step 4: Run tests**

Run: `go test ./pkg/parser/ -run TestParseTarget -v`
Expected: PASS

**Step 5: Commit**

```bash
git add pkg/parser/ast.go pkg/parser/parser.go pkg/parser/service_test.go
git commit -m "feat(parser): parse services block and compose shorthand in target"
```

---

## Phase 2: Infrastructure Package — `pkg/infra/`

### Task 4: Create `pkg/infra/` with container lifecycle interface

**Files:**
- Create: `pkg/infra/infra.go`
- Create: `pkg/infra/infra_test.go`

**Step 1: Define the interface and types**

```go
package infra

import (
	"context"
	"fmt"
)

// RunningService represents a started container with a resolved URL.
type RunningService struct {
	Name string
	URL  string // e.g., "http://localhost:8080"
	Port int    // actual mapped host port
}

// ServiceManager handles container lifecycle.
type ServiceManager interface {
	// Start launches all declared services and returns their resolved URLs.
	Start(ctx context.Context) ([]RunningService, error)

	// Stop tears down all managed services.
	Stop(ctx context.Context) error

	// Cleanup removes orphaned containers from previous runs.
	Cleanup(ctx context.Context) error
}

// Config holds the service declarations from a parsed spec target.
type Config struct {
	Services    []ServiceDef
	ComposePath string
	SpecName    string // used for container labeling
	SpecDir     string // base directory for resolving relative paths
}

// ServiceDef mirrors parser.Service with resolved paths.
type ServiceDef struct {
	Name    string
	Build   string            // Dockerfile directory (absolute path)
	Image   string            // pre-built image
	Port    int               // static port (0 = dynamic)
	Health  string            // HTTP health path
	Env     map[string]string
	Volumes map[string]string // host:container (absolute paths)
}
```

**Step 2: Commit**

```bash
git add pkg/infra/infra.go
git commit -m "feat(infra): define ServiceManager interface and types"
```

---

### Task 5: Implement Docker-based ServiceManager

**Files:**
- Create: `pkg/infra/docker.go`
- Create: `pkg/infra/docker_test.go`

This task adds the Docker Go SDK dependency and implements `DockerManager`.

**Step 1: Add Docker SDK dependency**

```bash
go get github.com/docker/docker@latest
go get github.com/docker/go-connections@latest
```

**Step 2: Implement `DockerManager`**

`pkg/infra/docker.go` — implements `ServiceManager`:

- `NewDockerManager(cfg Config) (*DockerManager, error)` — creates Docker client via `client.FromEnv` + `client.WithAPIVersionNegotiation()`
- `Start(ctx)` — for each `ServiceDef`:
  - If `Build` is set: build image from Dockerfile directory (`ImageBuild` API)
  - If `Image` is set: pull image (`ImagePull` API)
  - Create container with labels (`specrun.spec=<name>`, `specrun.run-id=<uuid>`), port mapping, env vars, volume mounts
  - Start container
  - Wait for health (HTTP probe if `Health` is set, TCP probe on port otherwise)
  - Return `RunningService` with resolved URL
- `Stop(ctx)` — stop and remove all managed containers
- `Cleanup(ctx)` — list containers with `specrun.spec=<name>` label, stop and remove orphans

Key implementation details:
- Port mapping: if `Port > 0`, bind to that host port. If `Port == 0`, let Docker pick a free port and read it back from the container's `NetworkSettings.Ports`.
- Health check: poll with 500ms interval, 30s timeout. HTTP probe sends GET to `http://localhost:<port><health>`. TCP probe dials `localhost:<port>`.
- Labels: `specrun.spec`, `specrun.run-id` (UUID generated at manager creation)
- Volume mounts: resolve relative paths against `SpecDir`

**Step 3: Write tests**

Tests should use Docker (integration tests). Skip if Docker is not available:

```go
func skipIfNoDocker(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker not available")
	}
}
```

Test cases:
- Start a simple container (e.g., `nginx:alpine`), verify it's running, stop it
- Health check waits for container to be ready
- Cleanup removes orphaned containers by label
- Dynamic port mapping assigns a free port

**Step 4: Commit**

```bash
git add pkg/infra/docker.go pkg/infra/docker_test.go go.mod go.sum
git commit -m "feat(infra): implement Docker-based ServiceManager"
```

---

### Task 6: Implement compose-based ServiceManager

**Files:**
- Create: `pkg/infra/compose.go`
- Create: `pkg/infra/compose_test.go`

**Step 1: Implement `ComposeManager`**

Uses `os/exec` to shell out to `docker compose`:

- `NewComposeManager(cfg Config) (*ComposeManager, error)` — validates compose file exists
- `Start(ctx)` — runs `docker compose -f <file> -p specrun-<spec> up -d --wait --build`
- `Stop(ctx)` — runs `docker compose -f <file> -p specrun-<spec> down -v`
- `Cleanup(ctx)` — runs `docker compose -f <file> -p specrun-<spec> down -v` (same as stop for compose)
- `ResolveServicePort(name string) (int, error)` — runs `docker compose -f <file> -p specrun-<spec> port <name> <port>` to get the mapped host port

The project name (`-p specrun-<spec>`) ensures isolation between different specs.

**Step 2: Write tests (skip if no docker compose)**

**Step 3: Commit**

```bash
git add pkg/infra/compose.go pkg/infra/compose_test.go
git commit -m "feat(infra): implement compose-based ServiceManager"
```

---

### Task 7: Factory function to create the right manager

**Files:**
- Modify: `pkg/infra/infra.go`

**Step 1: Add factory**

```go
// NewManager creates a ServiceManager based on the config.
// Returns nil (not an error) if no services are declared.
func NewManager(cfg Config) (ServiceManager, error) {
	if cfg.ComposePath != "" {
		return NewComposeManager(cfg)
	}
	if len(cfg.Services) > 0 {
		return NewDockerManager(cfg)
	}
	return nil, nil // no services declared
}
```

**Step 2: Commit**

```bash
git add pkg/infra/infra.go
git commit -m "feat(infra): add factory function for ServiceManager creation"
```

---

## Phase 3: Runner Integration

### Task 8: Integrate services into `runVerify`

**Files:**
- Modify: `cmd/specrun/main.go`

**Step 1: Add signal handling and service lifecycle to `runVerify`**

The flow becomes:

1. Parse spec
2. Build `infra.Config` from `spec.Target.Services` / `spec.Target.Compose`
3. Create `ServiceManager` via `infra.NewManager`
4. If manager is non-nil:
   a. `manager.Cleanup(ctx)` — pre-flight orphan removal
   b. `manager.Start(ctx)` — start services, get `[]RunningService`
   c. Register signal handler: on SIGINT/SIGTERM, call `manager.Stop(ctx)` then exit
   d. `defer manager.Stop(ctx)` (unless `--keep-services`)
5. `resolveTargetConfig(spec.Target, runningServices)` — resolve `service()` refs using running service URLs
6. `createAdapters(spec, config)` — unchanged
7. `runner.Verify()` — unchanged
8. Post-flight: deferred `manager.Stop(ctx)` runs (or signal handler runs)

**Step 2: Modify `resolveExprToString` to handle `ServiceRef`**

Add a `runningServices` parameter (or make it a method on a resolver struct):

```go
case parser.ServiceRef:
	for _, svc := range runningServices {
		if svc.Name == e.Name {
			return svc.URL
		}
	}
	return "" // service not running — adapter Init will error on empty URL
```

**Step 3: Add `--keep-services` flag**

In `runVerify`, add:
```go
keepServices := fs.Bool("keep-services", false, "keep containers running after verification for debugging")
```

Skip the deferred `manager.Stop()` if `*keepServices` is true.

**Step 4: Add signal handler**

```go
sigCh := make(chan os.Signal, 1)
signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
go func() {
	<-sigCh
	fmt.Fprintln(os.Stderr, "\ninterrupted, cleaning up services...")
	manager.Stop(context.Background())
	os.Exit(1)
}()
```

**Step 5: Verify**

Run: `go build ./cmd/specrun` — compiles
Run: `go test ./cmd/specrun/ -v` — existing tests pass (no services configured = nil manager = no change)

**Step 6: Commit**

```bash
git add cmd/specrun/main.go
git commit -m "feat(runner): integrate service lifecycle into verify command"
```

---

### Task 9: Add `--keep-services` to CLI help and validate `service()` references

**Files:**
- Modify: `cmd/specrun/main.go`
- Modify: `pkg/validator/validator.go` (optional — validate `ServiceRef` names match declared services)

**Step 1: Validate `service()` references at parse time**

In `pkg/validator/validator.go`, add a check: if a `ServiceRef` names a service not declared in `target.services` (and no compose file is set), emit an error.

**Step 2: Test and commit**

```bash
git add cmd/specrun/main.go pkg/validator/validator.go
git commit -m "feat(validator): validate service() references against declared services"
```

---

## Phase 4: Dockerfiles for Test Services

### Task 10: Add Dockerfiles for broken_server, http_server, echo_tool

**Files:**
- Create: `testdata/self/broken_server/Dockerfile`
- Create: `testdata/self/http_server/Dockerfile`
- Create: `testdata/self/echo_tool/Dockerfile`

Each follows the same pattern as `examples/server/Dockerfile`:

```dockerfile
FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /app ./<path>

FROM scratch
COPY --from=build /app /app
EXPOSE <port>
ENTRYPOINT ["/app"]
```

Note: These Dockerfiles need access to `go.mod` and `go.sum` from the repo root. The build context must be the repo root with the Dockerfile path specified via `-f`. Alternatively, each service directory can have its own `go.mod` (simpler isolation). Evaluate which approach works better.

**Step 1: Create Dockerfiles**

**Step 2: Test locally**

```bash
docker build -f testdata/self/broken_server/Dockerfile -t specrun-broken-server .
docker run --rm -p 8081:8081 specrun-broken-server
# verify it responds
docker stop <id>
```

**Step 3: Commit**

```bash
git add testdata/self/broken_server/Dockerfile testdata/self/http_server/Dockerfile testdata/self/echo_tool/Dockerfile
git commit -m "chore: add Dockerfiles for test services"
```

---

## Phase 5: Update Existing Specs to Use Services

### Task 11: Update `examples/transfer.spec` to use services

**Files:**
- Modify: `examples/transfer.spec`

**Step 1: Add services block**

Change from:
```
target {
  base_url: env(APP_URL, "http://localhost:8080")
}
```

To:
```
target {
  services {
    app {
      build: "./examples/server"
      port: 8080
      health: "/health"
    }
  }
  base_url: service(app)
}
```

`service(app)` starts the container and resolves to its URL. Docker is required. For environments without Docker, use the original `env(APP_URL, "http://localhost:8080")` form instead — the two are not mixed.

**Step 2: Verify**

```bash
./specrun verify examples/transfer.spec
```

Should start the container automatically, verify, then stop it.

**Step 3: Commit**

```bash
git add examples/transfer.spec
git commit -m "feat(examples): update transfer spec to use target services"
```

---

### Task 12: Update self-verification specs to use services

**Files:**
- Modify: `specs/speclang.spec` — add services block to target
- Modify: `testdata/self/broken_transfer.spec` — add services block
- Modify: `testdata/self/broken_transfer_invariant_only.spec` — add services block
- Modify: `testdata/self/http_adapter.spec` — add services block

**Step 1: Update `specs/speclang.spec`**

```
spec Speclang {
  target {
    services {
      transfer_server {
        build: "./examples/server"
        port: 8080
      }
      broken_server {
        build: "./testdata/self/broken_server"
        port: 8081
      }
      http_test_server {
        build: "./testdata/self/http_server"
        port: 8082
      }
    }
    command: env(SPECRUN_BIN, "./specrun")
  }
  ...
}
```

Note: `command` stays as `env()` because the process adapter runs specrun locally, not in a container. The `ECHO_TOOL_BIN` env var for the echo_tool is used inside fixture specs (not the root spec's target), so it may also need updating.

**Step 2: Update fixture specs**

Each fixture spec that currently uses `env(BROKEN_APP_URL, ...)` or `env(HTTP_TEST_URL, ...)` should switch to `service()` referencing the service declared in the root spec's target, OR declare its own services block.

Decision: Since fixture specs are verified through the process adapter (specrun runs them as subprocesses), they need their own target config. The root spec's services are not visible to subprocess invocations. This means either:
- (a) Each fixture spec declares its own services block (duplicated)
- (b) Fixture specs keep using `env()` and the root spec's services set the env vars (how?)
- (c) The runner passes service URLs as env vars to subprocess adapters

Option (c) is cleanest: the runner exports `SPECRUN_SERVICE_<NAME>=<URL>` env vars before running subprocesses, so fixture specs can use `env(SPECRUN_SERVICE_TRANSFER_SERVER)`.

Actually, the simpler approach: keep fixture specs using `env()`. The CI and local dev will set those env vars (either manually or via the root spec's services). The root spec's services handle starting the containers; the env vars are the interface between the root verification and the subprocess verifications.

**Step 3: Update CI to use specrun services (or keep as-is)**

If the root spec now starts its own services, CI just needs:
```bash
./specrun verify specs/speclang.spec
```

No more `go run ./examples/server &` steps. But this requires Docker in CI. Add Docker setup step to CI workflow.

**Step 4: Commit**

```bash
git add specs/speclang.spec testdata/self/*.spec .github/workflows/ci.yml
git commit -m "feat(specs): update self-verification to use target services"
```

---

## Phase 6: New Specs for Services Functionality

### Task 13: Write self-verification specs for the services feature

**Files:**
- Create: `testdata/self/services.spec` — fixture spec using a service
- Create: `specs/services.spec` — self-verification scopes
- Modify: `specs/speclang.spec` — include services.spec

**Step 1: Create fixture spec**

`testdata/self/services.spec` — a spec that declares a service and verifies it starts:
```
spec ServiceTest {
  target {
    services {
      echo_server {
        build: "./testdata/self/http_server"
        port: 9090
        health: "/api/items"
      }
    }
    base_url: service(echo_server)
  }

  scope service_health {
    use http
    config { path: "/api/items", method: "GET" }
    contract {
      input {}
      output { status: any, count: int }
    }
    scenario server_responds {
      given {}
      then {
        status: 200
        count: 2
      }
    }
  }
}
```

**Step 2: Write self-verification scopes**

`specs/services.spec`:
```
# Verifies that target services start containers and resolve service() URLs.

scope verify_service_lifecycle {
  use process
  config {
    args: "verify --json testdata/self/services.spec"
  }

  contract {
    input {}
    output {
      exit_code: int
      scenarios_run: int
      scenarios_passed: int
    }
  }

  scenario services_start_and_verify {
    given {}
    then {
      exit_code: 0
      scenarios_run: 1
      scenarios_passed: 1
    }
  }
}

scope parse_service_ref {
  use process
  config {
    args: "parse testdata/self/services.spec"
  }

  contract {
    input {}
    output {
      exit_code: int
      name: string
    }
  }

  scenario service_spec_parses {
    given {}
    then {
      exit_code: 0
      name: "ServiceTest"
    }
  }
}
```

**Step 3: Run self-verification**

```bash
SPECRUN_BIN=./specrun ./specrun verify specs/speclang.spec
```

**Step 4: Commit**

```bash
git add testdata/self/services.spec specs/services.spec specs/speclang.spec
git commit -m "spec: add self-verification for target services lifecycle"
```

---

### Task 14: Write specs for compose support

**Files:**
- Create: `testdata/self/docker-compose.test.yml`
- Create: `testdata/self/compose.spec`
- Modify: `specs/services.spec` — add compose verification scope

**Step 1: Create test compose file**

`testdata/self/docker-compose.test.yml`:
```yaml
services:
  test_http:
    build:
      context: ../..
      dockerfile: testdata/self/http_server/Dockerfile
    ports:
      - "9091:8082"
```

**Step 2: Create fixture spec using compose**

`testdata/self/compose.spec`:
```
spec ComposeTest {
  target {
    services {
      compose: "./testdata/self/docker-compose.test.yml"
    }
    base_url: service(test_http)
  }

  scope compose_service {
    use http
    config { path: "/api/items", method: "GET" }
    contract {
      input {}
      output { status: any }
    }
    scenario compose_responds {
      given {}
      then { status: 200 }
    }
  }
}
```

**Step 3: Add self-verification scope**

Add to `specs/services.spec`:
```
scope verify_compose_lifecycle {
  use process
  config {
    args: "verify --json testdata/self/compose.spec"
  }
  ...
}
```

**Step 4: Commit**

```bash
git add testdata/self/docker-compose.test.yml testdata/self/compose.spec specs/services.spec
git commit -m "spec: add self-verification for compose-based services"
```

---

### Task 15: Write specs for error cases

**Files:**
- Create: `testdata/self/invalid_service_ref.spec`
- Modify: `specs/services.spec`

**Step 1: Create fixture for invalid service reference**

`testdata/self/invalid_service_ref.spec`:
```
spec BadRef {
  target {
    base_url: service(nonexistent)
  }
  scope test { use http config { path: "/", method: "GET" } }
}
```

**Step 2: Add validation error scenario**

In `specs/services.spec` (or `specs/parse.spec`):
```
scope invalid_service_ref {
  use process
  config { args: "parse testdata/self/invalid_service_ref.spec" }
  contract {
    input {}
    output { exit_code: int }
  }
  scenario rejects_unknown_service {
    given {}
    then { exit_code: 1 }
  }
}
```

**Step 3: Commit**

```bash
git add testdata/self/invalid_service_ref.spec specs/services.spec
git commit -m "spec: add validation error specs for invalid service references"
```

---

## Phase 7: Documentation and Plugin Updates

### Task 16: Update CLAUDE.md

**Files:**
- Modify: `CLAUDE.md`

Add to the Settled Decisions section:
- Target services: `services` block in `target` declares Docker containers
- `service(name)` expression resolves to running container URL
- Compose support for multi-service setups with inter-container networking
- Container lifecycle: pre-flight cleanup, health checks, signal handling, `--keep-services` flag

Update the Project Structure to include `pkg/infra/`.

Update the Commands section to document `--keep-services`.

**Commit:** `docs: update CLAUDE.md with target services feature`

---

### Task 17: Update docs/

**Files:**
- Modify: `docs/language-reference.md` — add `service()` expression, `services` block syntax
- Modify: `docs/getting-started.md` — add a "Self-Contained Specs" section showing services
- Create: `docs/services.md` — dedicated guide for target services (inline, compose, volumes, health checks, cleanup, --keep-services)
- Modify: `docs/adapters/http.md` — mention services as an alternative to env vars for `base_url`
- Modify: `docs/adapters/process.md` — mention services for command targets
- Modify: `docs/self-verification.md` — update to reflect services-based self-verification

**Commit:** `docs: add target services documentation and update guides`

---

### Task 18: Update README.md

**Files:**
- Modify: `README.md`

Add a link to `docs/services.md` in the documentation table. Optionally update the quick example to show `service()` alongside the `env()` version.

**Commit:** `docs: add services guide link to README`

---

### Task 19: Update skills and plugin

**Files:**
- Modify: `skills/author/SKILL.md` — add services to the target config section, update the "Choosing a Plugin" table to mention services as infrastructure (not a plugin)
- Modify: `skills/author/references/api_reference.md` — add `service()` expression, `services` block syntax, `compose` shorthand
- Modify: `skills/verify/SKILL.md` — update to mention that services start automatically when declared, remove the "set env vars manually" guidance for service-backed specs
- Modify: `.claude-plugin/plugin.json` — bump version to reflect new capability

**Commit:** `docs: update skills and plugin manifest for target services`

---

### Task 20: Update CI workflow

**Files:**
- Modify: `.github/workflows/ci.yml`

Two options:
1. **Keep current pattern** — CI still starts servers manually (no Docker requirement in CI)
2. **Use services** — CI installs Docker, runs `specrun verify` which starts its own containers

Recommended: Keep current pattern for now (simpler CI, no Docker dependency). Add a separate CI job that tests the services feature with Docker. Update env var names if fixture specs changed.

**Commit:** `ci: update workflow for services feature`

---

### Task 21: Final verification and PR

**Step 1: Full lint check**

```bash
make lint   # zero issues
make test   # all pass
```

**Step 2: Self-verification**

```bash
make build
# Start servers (or let services handle it if CI has Docker)
SPECRUN_BIN=./specrun ./specrun verify specs/speclang.spec
```

**Step 3: Create PR**

```bash
git push -u origin feat/target-services
gh pr create --title "feat: add target services for self-contained specs (#56)" --body "..."
gh pr checks <number> --watch
gh pr merge <number> --squash --delete-branch
```
