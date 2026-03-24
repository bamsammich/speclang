package infra

import (
	"testing"
)

func TestParseComposePort(t *testing.T) {
	tests := []struct {
		input    string
		expected int
		wantErr  bool
	}{
		{"0.0.0.0:12345", 12345, false},
		{"0.0.0.0:80", 80, false},
		{"", 0, true},
		{"noport", 0, true},
		{"0.0.0.0:notanumber", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			port, err := parseComposePort(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error for input %q", tt.input)
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if port != tt.expected {
				t.Errorf("expected port %d, got %d", tt.expected, port)
			}
		})
	}
}

func TestSanitizeProjectName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"MySpec", "specrun-myspec"},
		{"my spec", "specrun-my-spec"},
		{"UPPER", "specrun-upper"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := sanitizeProjectName(tt.input)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestNewComposeManager_MissingFile(t *testing.T) {
	cfg := Config{
		ComposePath: "/nonexistent/docker-compose.yml",
		SpecName:    "test",
	}
	_, err := NewComposeManager(cfg)
	if err == nil {
		t.Error("expected error for missing compose file")
	}
}
