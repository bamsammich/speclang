package parser

import "testing"

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
	ref, ok := spec.Target.Fields["base_url"].(ServiceRef)
	if !ok {
		t.Fatalf("expected ServiceRef, got %T", spec.Target.Fields["base_url"])
	}
	if ref.Name != "app" {
		t.Fatalf("expected name=app, got %q", ref.Name)
	}
}

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
	if app.Name != "app" || app.Build != "./examples/server" ||
		app.Port != 8080 || app.Health != "/health" {
		t.Fatalf("unexpected app: %+v", app)
	}
	db := spec.Target.Services[1]
	if db.Name != "db" || db.Image != "postgres:15" || db.Port != 5432 {
		t.Fatalf("unexpected db: %+v", db)
	}
	if db.Env["POSTGRES_PASSWORD"] != "test" {
		t.Fatalf("unexpected env: %v", db.Env)
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
