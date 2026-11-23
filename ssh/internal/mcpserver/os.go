package mcpserver

import (
	"context"
	"errors"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func (s *Server) os(ctx context.Context, _ *mcp.CallToolRequest, in osInput) (*mcp.CallToolResult, any, error) {
	action := strings.ToLower(strings.TrimSpace(in.Action))
	d := in.Data
	switch action {
	case "upload":
		return s.transfer(ctx, nil, transferInput{Target: in.Target, Direction: "upload", Local: d.Local, Remote: d.Remote, TimeoutSeconds: d.Timeout})
	case "download":
		return s.transfer(ctx, nil, transferInput{Target: in.Target, Direction: "download", Local: d.Local, Remote: d.Remote, TimeoutSeconds: d.Timeout})
	case "":
		return nil, nil, errors.New("'action' é obrigatório")
	default:
		return nil, nil, errors.New("'action' inválido: upload, download")
	}
}

type osInput struct {
	Target string `json:"target,omitempty" jsonschema:"SSH connection alias (optional when only one is configured)."`
	Action string `json:"action" jsonschema:"upload or download (required)."`
	Data   osData `json:"data,omitempty" jsonschema:"Options for the chosen action."`
}

type osData struct {
	Local   string `json:"local,omitempty" jsonschema:"File inside the shared mnt dir."`
	Remote  string `json:"remote,omitempty" jsonschema:"Remote file path."`
	Timeout int    `json:"timeout,omitempty" jsonschema:"Timeout in seconds."`
}
