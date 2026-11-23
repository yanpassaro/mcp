package mcpserver

import (
	"context"
	"errors"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func (s *Server) act(ctx context.Context, _ *mcp.CallToolRequest, in actInput) (*mcp.CallToolResult, any, error) {
	action := strings.ToLower(strings.TrimSpace(in.Action))
	d := in.Data
	switch action {
	case "run":
		return s.run(ctx, nil, runInput{Target: in.Target, Command: d.Command, Workdir: d.Workdir, Timeout: d.Timeout})
	case "write":
		return s.fs(ctx, nil, fsInput{Target: in.Target, Action: "write", Path: d.Path, Content: d.Content, Append: d.Append, TimeoutSeconds: d.Timeout})
	case "remove":
		return s.fs(ctx, nil, fsInput{Target: in.Target, Action: "remove", Path: d.Path, Recursive: d.Recursive, TimeoutSeconds: d.Timeout})
	case "":
		return nil, nil, errors.New("'action' é obrigatório")
	default:
		return nil, nil, errors.New("'action' inválido: run, write, remove")
	}
}

type actInput struct {
	Target string  `json:"target,omitempty" jsonschema:"SSH connection alias (optional when only one is configured)."`
	Action string  `json:"action" jsonschema:"run, write or remove (required)."`
	Data   actData `json:"data,omitempty" jsonschema:"Options for the chosen action."`
}

type actData struct {
	Command   string `json:"command,omitempty" jsonschema:"Shell command (run)."`
	Workdir   string `json:"workdir,omitempty" jsonschema:"Directory for the command (run)."`
	Path      string `json:"path,omitempty" jsonschema:"Remote path (write, remove)."`
	Content   string `json:"content,omitempty" jsonschema:"Text to write."`
	Append    bool   `json:"append,omitempty" jsonschema:"Write: append instead of overwrite."`
	Recursive bool   `json:"recursive,omitempty" jsonschema:"Remove: recurse into directories."`
	Timeout   int    `json:"timeout,omitempty" jsonschema:"Timeout in seconds."`
}
