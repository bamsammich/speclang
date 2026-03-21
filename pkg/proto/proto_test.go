package proto

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/bamsammich/speclang/pkg/parser"
)

func testdataPath(name string) string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "testdata", "proto", name)
}

func TestResolve_User(t *testing.T) {
	r := &Resolver{}
	models, scopes, err := r.Resolve(testdataPath("user.proto"))
	if err != nil {
		t.Fatal(err)
	}

	// 4 messages: CreateUserRequest, CreateUserResponse, GetUserRequest, GetUserResponse, User
	// But User is referenced as a message type in responses, not as a field with unsupported type
	if len(models) < 4 {
		t.Fatalf("expected at least 4 models, got %d", len(models))
	}

	// 2 unary RPCs
	if len(scopes) != 2 {
		t.Fatalf("expected 2 scopes, got %d", len(scopes))
	}
}

func TestConvertMessages_User(t *testing.T) {
	proto, err := parseProtoFile(testdataPath("user.proto"))
	if err != nil {
		t.Fatal(err)
	}

	models := convertMessages(proto)

	// Find User model
	var user *parser.Model
	for _, m := range models {
		if m.Name == "User" {
			user = m
		}
	}
	if user == nil {
		t.Fatal("User model not found")
	}

	// User has 4 fields: id (int), name (string), email (string), phone (string?)
	if len(user.Fields) != 4 {
		t.Fatalf("expected 4 fields in User, got %d", len(user.Fields))
	}

	// Fields sorted: email, id, name, phone
	assertField(t, user.Fields[0], "email", "string", false)
	assertField(t, user.Fields[1], "id", "int", false)
	assertField(t, user.Fields[2], "name", "string", false)
	assertField(t, user.Fields[3], "phone", "string", true) // optional
}

func TestConvertMessages_Nested(t *testing.T) {
	proto, err := parseProtoFile(testdataPath("nested.proto"))
	if err != nil {
		t.Fatal(err)
	}

	models := convertMessages(proto)

	// SearchResponse has a repeated field (skipped) and total_count
	// SearchResponse_Result is the nested message, flattened
	var result *parser.Model
	var search *parser.Model
	for _, m := range models {
		switch m.Name {
		case "SearchResponse_Result":
			result = m
		case "SearchResponse":
			search = m
		}
	}

	if result == nil {
		t.Fatal("SearchResponse_Result model not found")
	}
	if len(result.Fields) != 3 {
		t.Fatalf("expected 3 fields in SearchResponse_Result, got %d", len(result.Fields))
	}

	if search == nil {
		t.Fatal("SearchResponse model not found")
	}
	// SearchResponse.results is repeated (skipped), only total_count remains
	if len(search.Fields) != 1 {
		t.Fatalf("expected 1 field in SearchResponse (repeated skipped), got %d", len(search.Fields))
	}
	if search.Fields[0].Name != "total_count" {
		t.Errorf("expected field 'total_count', got %q", search.Fields[0].Name)
	}
}

func TestConvertMessages_Unsupported(t *testing.T) {
	proto, err := parseProtoFile(testdataPath("unsupported.proto"))
	if err != nil {
		t.Fatal(err)
	}

	models := convertMessages(proto)

	// MixedTypes: name (string), count (int), price (float), rating (float),
	// data (bytes) survive; repeated and map are skipped
	if len(models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(models))
	}

	m := models[0]
	if m.Name != "MixedTypes" {
		t.Fatalf("expected model MixedTypes, got %q", m.Name)
	}

	if len(m.Fields) != 5 {
		t.Fatalf("expected 5 supported fields, got %d", len(m.Fields))
	}

	// Fields sorted: count, data, name, price, rating
	assertField(t, m.Fields[0], "count", "int", false)
	assertField(t, m.Fields[1], "data", "bytes", false)
	assertField(t, m.Fields[2], "name", "string", false)
	assertField(t, m.Fields[3], "price", "float", false)
	assertField(t, m.Fields[4], "rating", "float", false)
}

func TestConvertMessages_Empty(t *testing.T) {
	proto, err := parseProtoFile(testdataPath("empty.proto"))
	if err != nil {
		t.Fatal(err)
	}

	models := convertMessages(proto)
	if len(models) != 0 {
		t.Errorf("expected 0 models, got %d", len(models))
	}
}

func TestConvertServices_User(t *testing.T) {
	proto, err := parseProtoFile(testdataPath("user.proto"))
	if err != nil {
		t.Fatal(err)
	}

	models := convertMessages(proto)
	scopes := convertServices(proto, models)

	if len(scopes) != 2 {
		t.Fatalf("expected 2 scopes, got %d", len(scopes))
	}

	// Sorted: CreateUser, GetUser
	assertScopeName(t, scopes, 0, "CreateUser")
	assertScopeName(t, scopes, 1, "GetUser")

	// CreateUser scope
	cu := scopes[0]
	assertConfigValue(t, cu, "service", "UserService")
	assertConfigValue(t, cu, "method", "CreateUser")

	if cu.Contract == nil {
		t.Fatal("CreateUser should have a contract")
	}
	if len(cu.Contract.Input) == 0 {
		t.Error("CreateUser should have contract input fields")
	}
	if len(cu.Contract.Output) == 0 {
		t.Error("CreateUser should have contract output fields")
	}
}

func TestConvertServices_Streaming(t *testing.T) {
	proto, err := parseProtoFile(testdataPath("streaming.proto"))
	if err != nil {
		t.Fatal(err)
	}

	models := convertMessages(proto)
	scopes := convertServices(proto, models)

	// Only SendEvent is unary; the other 3 are streaming and should be skipped
	if len(scopes) != 1 {
		t.Fatalf("expected 1 scope (unary only), got %d", len(scopes))
	}
	if scopes[0].Name != "SendEvent" {
		t.Errorf("expected scope 'SendEvent', got %q", scopes[0].Name)
	}
}

func TestConvertMessages_MessageRef(t *testing.T) {
	proto, err := parseProtoFile(testdataPath("user.proto"))
	if err != nil {
		t.Fatal(err)
	}

	models := convertMessages(proto)

	// CreateUserResponse has a 'user' field of type User (message reference)
	var resp *parser.Model
	for _, m := range models {
		if m.Name == "CreateUserResponse" {
			resp = m
		}
	}
	if resp == nil {
		t.Fatal("CreateUserResponse model not found")
	}

	var userField *parser.Field
	for _, f := range resp.Fields {
		if f.Name == "user" {
			userField = f
		}
	}
	if userField == nil {
		t.Fatal("user field not found in CreateUserResponse")
	}
	if userField.Type.Name != "User" {
		t.Errorf("expected type 'User', got %q", userField.Type.Name)
	}
}

func TestMapProtoType(t *testing.T) {
	tests := []struct {
		input string
		want  string
		ok    bool
	}{
		{"int32", "int", true},
		{"int64", "int", true},
		{"uint32", "int", true},
		{"uint64", "int", true},
		{"sint32", "int", true},
		{"sint64", "int", true},
		{"fixed32", "int", true},
		{"fixed64", "int", true},
		{"sfixed32", "int", true},
		{"sfixed64", "int", true},
		{"string", "string", true},
		{"bool", "bool", true},
		{"float", "float", true},
		{"double", "float", true},
		{"bytes", "bytes", true},
		{"MyMessage", "MyMessage", true},
		{"package.MyMessage", "MyMessage", true},
		{"google.protobuf.Timestamp", "string", true},
		{"google.protobuf.BoolValue", "bool", true},
		{"google.protobuf.StringValue", "string", true},
		{"google.protobuf.Int32Value", "int", true},
		{"google.protobuf.Any", "", false},
	}

	for _, tt := range tests {
		got, ok := mapProtoType(tt.input)
		if ok != tt.ok {
			t.Errorf("mapProtoType(%q): ok = %v, want %v", tt.input, ok, tt.ok)
			continue
		}
		if ok && got.Name != tt.want {
			t.Errorf("mapProtoType(%q) = %q, want %q", tt.input, got.Name, tt.want)
		}
	}
}

func TestParseProtoFile_Invalid(t *testing.T) {
	_, err := parseProtoFile("/nonexistent/file.proto")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

// --- helpers ---

func assertField(t *testing.T, f *parser.Field, name, typeName string, optional bool) {
	t.Helper()
	if f.Name != name {
		t.Errorf("field.Name = %q, want %q", f.Name, name)
	}
	if f.Type.Name != typeName {
		t.Errorf("field %q: Type.Name = %q, want %q", name, f.Type.Name, typeName)
	}
	if f.Type.Optional != optional {
		t.Errorf("field %q: Optional = %v, want %v", name, f.Type.Optional, optional)
	}
}

func assertScopeName(t *testing.T, scopes []*parser.Scope, idx int, name string) {
	t.Helper()
	if scopes[idx].Name != name {
		t.Errorf("scopes[%d].Name = %q, want %q", idx, scopes[idx].Name, name)
	}
}

func assertConfigValue(t *testing.T, scope *parser.Scope, key, expected string) {
	t.Helper()
	expr, ok := scope.Config[key]
	if !ok {
		t.Errorf("scope %q: config key %q not found", scope.Name, key)
		return
	}
	lit, ok := expr.(parser.LiteralString)
	if !ok {
		t.Errorf("scope %q: config %q is not a LiteralString", scope.Name, key)
		return
	}
	if lit.Value != expected {
		t.Errorf("scope %q: config %q = %q, want %q", scope.Name, key, lit.Value, expected)
	}
}
