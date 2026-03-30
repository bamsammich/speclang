// Package proto implements an ImportResolver that converts protobuf
// schema files (.proto) into speclang AST nodes (models and scopes).
//
// It uses go-protoparser for parsing, so no protoc installation is required.
package proto

import (
	"fmt"
	"os"

	protoparser "github.com/yoheimuta/go-protoparser/v4"
	pb "github.com/yoheimuta/go-protoparser/v4/parser"

	"github.com/bamsammich/speclang/v3/internal/parser"
)

// Resolver implements parser.ImportResolver for protobuf files.
type Resolver struct{}

// Resolve reads a .proto file and returns speclang models and scopes.
func (*Resolver) Resolve(absPath string) ([]*parser.Model, []*parser.Scope, error) {
	proto, err := parseProtoFile(absPath)
	if err != nil {
		return nil, nil, err
	}

	models := convertMessages(proto)
	scopes := convertServices(proto, models)

	return models, scopes, nil
}

// parseProtoFile reads and parses a .proto file.
func parseProtoFile(path string) (*pb.Proto, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("reading proto file: %w", err)
	}
	defer f.Close()

	proto, err := protoparser.Parse(
		f,
		protoparser.WithPermissive(true),
		protoparser.WithFilename(path),
	)
	if err != nil {
		return nil, fmt.Errorf("parsing proto file: %w", err)
	}
	return proto, nil
}
