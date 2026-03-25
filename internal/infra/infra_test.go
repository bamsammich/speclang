package infra

import (
	"testing"
)

func TestNewManager_NoServices(t *testing.T) {
	mgr, err := NewManager(Config{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mgr != nil {
		t.Error("expected nil manager when no services declared")
	}
}

func TestNewManager_WithServices(t *testing.T) {
	skipIfNoDocker(t)

	cfg := Config{
		SpecName: "factorytest",
		Services: []ServiceDef{
			{Name: "svc", Image: "nginx:alpine"},
		},
	}
	mgr, err := NewManager(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mgr == nil {
		t.Fatal("expected non-nil manager")
	}
	if _, ok := mgr.(*DockerManager); !ok {
		t.Error("expected DockerManager for inline services")
	}
}
