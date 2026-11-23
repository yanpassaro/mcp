package mcpserver

import (
	"context"
	"errors"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func (s *Server) read(ctx context.Context, _ *mcp.CallToolRequest, in input) (*mcp.CallToolResult, any, error) {
	action := strings.ToLower(strings.TrimSpace(in.Action))
	d := in.Data
	switch action {
	case "probe":
		return s.probe(ctx, nil, probeInput{Target: in.Target, TimeoutSeconds: d.Timeout})
	case "read":
		return s.fs(ctx, nil, fsInput{Target: in.Target, Action: "read", Path: d.Path, MaxBytes: d.Max, TimeoutSeconds: d.Timeout})
	case "list":
		return s.fs(ctx, nil, fsInput{Target: in.Target, Action: "list", Path: d.Path, TimeoutSeconds: d.Timeout})
	case "stat":
		return s.fs(ctx, nil, fsInput{Target: in.Target, Action: "stat", Path: d.Path, TimeoutSeconds: d.Timeout})
	case "":
		return nil, nil, errors.New("'action' é obrigatório")
	default:
		return nil, nil, errors.New("'action' inválido: probe, read, list, stat")
	}
}

type input struct {
	Target string `json:"target,omitempty" jsonschema:"SSH connection alias (optional when only one is configured)."`
	Action string `json:"action" jsonschema:"probe, read, list or stat (required)."`
	Data   data   `json:"data,omitempty" jsonschema:"Options for the chosen action."`
}

type data struct {
	Path    string `json:"path,omitempty" jsonschema:"Remote path (read, list, stat)."`
	Max     int    `json:"max,omitempty" jsonschema:"Max bytes to read (default 262144, max 33554432)."`
	Timeout int    `json:"timeout,omitempty" jsonschema:"Timeout in seconds."`
}
