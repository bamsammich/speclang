package spec

import "encoding/json"

// Request is sent from the runtime to an adapter.
type Request struct {
	Name string          `json:"name,omitempty"` // method name
	Args json.RawMessage `json:"args,omitempty"` // method arguments
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

	// Call executes a named method (action or query) with arguments.
	// For actions, it performs the operation and returns the result.
	// For queries (formerly assertions), it returns the current value in Response.Actual.
	Call(method string, args json.RawMessage) (*Response, error)

	// Reset clears accumulated state (headers, cookies, responses) for a fresh iteration.
	Reset() error

	// Close cleans up resources.
	Close() error
}
