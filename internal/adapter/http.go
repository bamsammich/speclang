package adapter

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"reflect"
	"strconv"
	"strings"
)

type lastResponse struct {
	body       map[string]any
	headers    http.Header
	rawBody    json.RawMessage
	statusCode int
}

// HTTPAdapter is the built-in adapter for testing HTTP APIs.
type HTTPAdapter struct {
	client  *http.Client
	headers map[string]string
	last    *lastResponse
	BaseURL string
}

func NewHTTPAdapter() (*HTTPAdapter, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("creating cookie jar: %w", err)
	}
	return &HTTPAdapter{
		client:  &http.Client{Jar: jar},
		headers: make(map[string]string),
	}, nil
}

func (a *HTTPAdapter) Init(config map[string]string) error {
	url, ok := config["base_url"]
	if !ok {
		return errors.New("http adapter requires base_url in target config")
	}
	a.BaseURL = url
	return nil
}

func (a *HTTPAdapter) doRequest(method, path string, body json.RawMessage) (*Response, error) {
	var reqBody io.Reader
	if len(body) > 0 {
		reqBody = bytes.NewReader(body)
	}

	req, err := http.NewRequest(method, a.BaseURL+path, reqBody)
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}

	if (method == http.MethodPost || method == http.MethodPut) && len(body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}

	for k, v := range a.headers {
		req.Header.Set(k, v)
	}

	//nolint:gosec // HTTP adapter intentionally sends requests to user-configured URLs from spec
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	rawBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	var parsed map[string]any
	// Best-effort parse; non-JSON responses leave parsed nil.
	_ = json.Unmarshal(rawBody, &parsed) //nolint:errcheck // intentional best-effort

	a.last = &lastResponse{
		statusCode: resp.StatusCode,
		headers:    resp.Header,
		body:       parsed,
		rawBody:    json.RawMessage(rawBody),
	}

	return &Response{OK: true, Actual: json.RawMessage(rawBody)}, nil
}

func (a *HTTPAdapter) Action(name string, args json.RawMessage) (*Response, error) {
	var rawArgs []json.RawMessage
	if len(args) > 0 {
		if err := json.Unmarshal(args, &rawArgs); err != nil {
			return nil, fmt.Errorf("parsing action args: %w", err)
		}
	}

	switch name {
	case "get", "post", "put", "delete":
		return a.doHTTPAction(name, rawArgs)
	case "header":
		return a.doHeaderAction(rawArgs)
	default:
		return nil, fmt.Errorf("unknown http action %q", name)
	}
}

func (a *HTTPAdapter) doHTTPAction(name string, rawArgs []json.RawMessage) (*Response, error) {
	if len(rawArgs) < 1 {
		return nil, fmt.Errorf("action %q requires at least a path argument", name)
	}

	var path string
	if err := json.Unmarshal(rawArgs[0], &path); err != nil {
		return nil, fmt.Errorf("parsing path argument: %w", err)
	}

	var body json.RawMessage
	if len(rawArgs) > 1 {
		body = rawArgs[1]
	}

	return a.doRequest(strings.ToUpper(name), path, body)
}

func (a *HTTPAdapter) doHeaderAction(rawArgs []json.RawMessage) (*Response, error) {
	if len(rawArgs) < 2 {
		return nil, errors.New("action \"header\" requires name and value arguments")
	}
	var headerName, headerValue string
	if err := json.Unmarshal(rawArgs[0], &headerName); err != nil {
		return nil, fmt.Errorf("parsing header name: %w", err)
	}
	if err := json.Unmarshal(rawArgs[1], &headerValue); err != nil {
		return nil, fmt.Errorf("parsing header value: %w", err)
	}
	a.headers[headerName] = headerValue
	return &Response{OK: true}, nil
}

func (a *HTTPAdapter) Assert(
	property string,
	_ string,
	expected json.RawMessage,
) (*Response, error) {
	if a.last == nil {
		return nil, errors.New("no request has been made yet")
	}

	var actual any

	switch {
	case property == "status":
		actual = float64(a.last.statusCode)

	case strings.HasPrefix(property, "header."):
		headerName := strings.TrimPrefix(property, "header.")
		actual = a.last.headers.Get(headerName)

	case property == "body":
		actual = a.last.body

	default:
		// Treat as body field path.
		if a.last.body == nil {
			return nil, fmt.Errorf("response body is not JSON, cannot extract path %q", property)
		}
		val, err := extractPath(a.last.body, property)
		if err != nil {
			return nil, err
		}
		actual = val
	}

	// Normalize both sides through JSON for consistent comparison.
	actualJSON, err := json.Marshal(actual)
	if err != nil {
		return nil, fmt.Errorf("marshaling actual value: %w", err)
	}

	var actualNorm, expectedNorm any
	if err := json.Unmarshal(actualJSON, &actualNorm); err != nil {
		return nil, fmt.Errorf("normalizing actual: %w", err)
	}
	if err := json.Unmarshal(expected, &expectedNorm); err != nil {
		return nil, fmt.Errorf("normalizing expected: %w", err)
	}

	if reflect.DeepEqual(actualNorm, expectedNorm) {
		return &Response{OK: true, Actual: actualJSON}, nil
	}

	return &Response{
		OK:     false,
		Actual: actualJSON,
		Error:  fmt.Sprintf("expected %s, got %s", string(expected), string(actualJSON)),
	}, nil
}

func (*HTTPAdapter) Close() error {
	return nil
}

// extractPath walks a nested map/array by dot-separated path segments.
func extractPath(obj map[string]any, path string) (any, error) {
	parts := strings.Split(path, ".")
	var current any = obj

	for _, part := range parts {
		switch v := current.(type) {
		case map[string]any:
			val, exists := v[part]
			if !exists {
				return nil, fmt.Errorf("key %q not found", part)
			}
			current = val
		case []any:
			idx, err := strconv.Atoi(part)
			if err != nil {
				return nil, fmt.Errorf("array requires numeric index, got %q", part)
			}
			if idx < 0 || idx >= len(v) {
				return nil, fmt.Errorf("array index %d out of range (length %d)", idx, len(v))
			}
			current = v[idx]
		default:
			return nil, fmt.Errorf("cannot traverse into %T at %q", current, part)
		}
	}

	return current, nil
}
