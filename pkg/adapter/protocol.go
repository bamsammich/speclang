package adapter

import "encoding/json"

// Request is sent from the runtime to an adapter.
type Request struct {
	Type     string          `json:"type"`               // "action" or "assert"
	Name     string          `json:"name,omitempty"`     // action/assertion name
	Args     json.RawMessage `json:"args,omitempty"`     // action arguments
	Locator  string          `json:"locator,omitempty"`  // for UI assertions
	Property string          `json:"property,omitempty"` // assertion property
	Expected json.RawMessage `json:"expected,omitempty"` // expected value
}

// Response is returned from an adapter to the runtime.
type Response struct {
	Error  string          `json:"error,omitempty"`
	Actual json.RawMessage `json:"actual,omitempty"`
	OK     bool            `json:"ok"`
}

// Adapter is the interface any plugin adapter must implement.
type Adapter interface {
	// Init is called once before any actions/assertions.
	Init(config map[string]string) error

	// Action executes a named action with arguments.
	Action(name string, args json.RawMessage) (*Response, error)

	// Assert checks an assertion against the system under test.
	Assert(property string, locator string, expected json.RawMessage) (*Response, error)

	// Close cleans up resources.
	Close() error
}
