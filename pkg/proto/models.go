package proto

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/bamsammich/speclang/pkg/parser"
	pb "github.com/yoheimuta/go-protoparser/v4/parser"
)

// convertMessages extracts all messages from a proto file and converts
// them to speclang models. Nested messages are flattened with Parent_Child naming.
func convertMessages(proto *pb.Proto) []*parser.Model {
	var models []*parser.Model
	for _, item := range proto.ProtoBody {
		switch v := item.(type) {
		case *pb.Message:
			models = append(models, flattenMessage("", v)...)
		}
	}
	sort.Slice(models, func(i, j int) bool { return models[i].Name < models[j].Name })
	return models
}

// flattenMessage converts a message and its nested messages to models.
// The prefix is used for collision-safe naming of nested messages.
func flattenMessage(prefix string, msg *pb.Message) []*parser.Model {
	name := msg.MessageName
	if prefix != "" {
		name = prefix + "_" + msg.MessageName
	}

	m := &parser.Model{Name: name}
	var nested []*parser.Model

	for _, item := range msg.MessageBody {
		switch v := item.(type) {
		case *pb.Field:
			field := protoFieldToField(v)
			if field != nil {
				m.Fields = append(m.Fields, field)
			}
		case *pb.Message:
			// Nested message — flatten with prefix
			nested = append(nested, flattenMessage(name, v)...)
		case *pb.MapField:
			fmt.Fprintf(os.Stderr, "warning: unsupported map field %q in message %q, skipping\n", v.MapName, name)
		case *pb.Oneof:
			fmt.Fprintf(os.Stderr, "warning: unsupported oneof %q in message %q, skipping\n", v.OneofName, name)
		}
	}

	// Sort fields by name for deterministic output
	sort.Slice(m.Fields, func(i, j int) bool { return m.Fields[i].Name < m.Fields[j].Name })

	var models []*parser.Model
	if len(m.Fields) > 0 {
		models = append(models, m)
	}
	models = append(models, nested...)
	return models
}

// protoFieldToField converts a protobuf field to a speclang Field.
// Returns nil if the type is unsupported.
func protoFieldToField(f *pb.Field) *parser.Field {
	typeExpr, ok := mapProtoType(f.Type)
	if !ok {
		fmt.Fprintf(os.Stderr, "warning: unsupported type %q for field %q, skipping\n", f.Type, f.FieldName)
		return nil
	}

	// Wrap in array for repeated fields.
	if f.IsRepeated {
		typeExpr = parser.TypeExpr{Name: "array", ElemType: &typeExpr}
	}

	if f.IsOptional {
		typeExpr.Optional = true
	}

	return &parser.Field{
		Name: f.FieldName,
		Type: typeExpr,
	}
}

// mapProtoType maps a protobuf type string to a speclang TypeExpr.
func mapProtoType(typ string) (parser.TypeExpr, bool) {
	switch typ {
	// Integer types
	case "int32", "int64", "sint32", "sint64",
		"uint32", "uint64",
		"fixed32", "fixed64", "sfixed32", "sfixed64":
		return parser.TypeExpr{Name: "int"}, true

	// String
	case "string":
		return parser.TypeExpr{Name: "string"}, true

	// Boolean
	case "bool":
		return parser.TypeExpr{Name: "bool"}, true

	// Float types
	case "float", "double":
		return parser.TypeExpr{Name: "float"}, true

	// Bytes
	case "bytes":
		return parser.TypeExpr{Name: "bytes"}, true

	default:
		// Check for well-known types
		te, ok := mapWellKnownType(typ)
		if ok {
			return te, true
		}
		// Unsupported well-known types return explicitly false
		if strings.HasPrefix(typ, "google.protobuf.") {
			return parser.TypeExpr{}, false
		}
		// Assume it's a message reference
		return parser.TypeExpr{Name: normalizeMessageRef(typ)}, true
	}
}

// mapWellKnownType handles google.protobuf.* well-known types.
func mapWellKnownType(typ string) (parser.TypeExpr, bool) {
	switch typ {
	case "google.protobuf.Timestamp", "google.protobuf.Duration",
		"google.protobuf.FieldMask":
		return parser.TypeExpr{Name: "string"}, true

	case "google.protobuf.BoolValue":
		return parser.TypeExpr{Name: "bool", Optional: true}, true

	case "google.protobuf.StringValue":
		return parser.TypeExpr{Name: "string", Optional: true}, true

	case "google.protobuf.Int32Value", "google.protobuf.Int64Value",
		"google.protobuf.UInt32Value", "google.protobuf.UInt64Value":
		return parser.TypeExpr{Name: "int", Optional: true}, true

	case "google.protobuf.FloatValue", "google.protobuf.DoubleValue":
		return parser.TypeExpr{Name: "float", Optional: true}, true

	case "google.protobuf.BytesValue":
		return parser.TypeExpr{Name: "bytes", Optional: true}, true

	case "google.protobuf.Any", "google.protobuf.Struct",
		"google.protobuf.Value", "google.protobuf.ListValue":
		return parser.TypeExpr{}, false

	default:
		return parser.TypeExpr{}, false
	}
}

// normalizeMessageRef extracts the simple name from a potentially
// package-qualified message reference. E.g., "mypackage.User" → "User".
func normalizeMessageRef(typ string) string {
	if idx := strings.LastIndex(typ, "."); idx >= 0 {
		return typ[idx+1:]
	}
	return typ
}
