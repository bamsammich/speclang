package proto

import (
	"fmt"
	"os"
	"sort"

	"github.com/bamsammich/speclang/v2/pkg/parser"
	pb "github.com/yoheimuta/go-protoparser/v4/parser"
)

// convertServices extracts RPC methods from proto services and converts
// them to speclang scopes. Only unary RPCs are supported.
func convertServices(proto *pb.Proto, models []*parser.Model) []*parser.Scope {
	modelMap := make(map[string]*parser.Model, len(models))
	for _, m := range models {
		modelMap[m.Name] = m
	}

	var scopes []*parser.Scope
	for _, item := range proto.ProtoBody {
		svc, ok := item.(*pb.Service)
		if !ok {
			continue
		}
		for _, body := range svc.ServiceBody {
			rpc, ok := body.(*pb.RPC)
			if !ok {
				continue
			}
			scope := rpcToScope(svc.ServiceName, rpc, modelMap)
			if scope != nil {
				scopes = append(scopes, scope)
			}
		}
	}
	sort.Slice(scopes, func(i, j int) bool { return scopes[i].Name < scopes[j].Name })
	return scopes
}

// rpcToScope converts a single RPC method to a speclang Scope.
// Returns nil for streaming RPCs.
func rpcToScope(serviceName string, rpc *pb.RPC, models map[string]*parser.Model) *parser.Scope {
	// Skip streaming RPCs
	if rpc.RPCRequest.IsStream || rpc.RPCResponse.IsStream {
		fmt.Fprintf(os.Stderr, "warning: skipping streaming RPC %s.%s\n", serviceName, rpc.RPCName)
		return nil
	}

	scope := &parser.Scope{
		Name: rpc.RPCName,
		Use:  "http",
		Config: map[string]parser.Expr{
			"service": parser.LiteralString{Value: serviceName},
			"method":  parser.LiteralString{Value: rpc.RPCName},
		},
	}

	contract := &parser.Contract{}

	// Request → contract input
	reqType := normalizeMessageRef(rpc.RPCRequest.MessageType)
	if reqType != "Empty" && reqType != "google.protobuf.Empty" {
		if m, ok := models[reqType]; ok {
			contract.Input = copyFields(m.Fields)
		}
	}

	// Response → contract output
	respType := normalizeMessageRef(rpc.RPCResponse.MessageType)
	if respType != "Empty" && respType != "google.protobuf.Empty" {
		if m, ok := models[respType]; ok {
			contract.Output = copyFields(m.Fields)
		}
	}

	if contract.Input != nil || contract.Output != nil {
		scope.Contract = contract
	}

	return scope
}

// copyFields creates a shallow copy of a field slice for contract use.
func copyFields(fields []*parser.Field) []*parser.Field {
	if len(fields) == 0 {
		return nil
	}
	out := make([]*parser.Field, len(fields))
	copy(out, fields)
	return out
}
