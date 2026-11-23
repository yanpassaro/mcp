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
	case "containers":
		return s.get(ctx, nil, getInput{Resource: "containers", RunningOnly: d.RunningOnly, Last: d.Last, Filter: d.Filter, Limit: d.Limit})
	case "images":
		return s.get(ctx, nil, getInput{Resource: "images", All: d.All, Filter: d.Filter, Limit: d.Limit})
	case "volumes":
		return s.get(ctx, nil, getInput{Resource: "volumes", Filter: d.Filter, Limit: d.Limit})
	case "networks":
		return s.get(ctx, nil, getInput{Resource: "networks", Filter: d.Filter, Limit: d.Limit})
	case "info":
		return s.get(ctx, nil, getInput{Resource: "info"})
	case "stats":
		return s.get(ctx, nil, getInput{Resource: "stats", Container: d.Name})
	case "disk":
		return s.get(ctx, nil, getInput{Resource: "disk", Verbose: d.Verbose})
	case "inspect":
		return s.inspect(ctx, nil, inspectInput{Resource: d.Resource, Target: d.Name})
	case "logs":
		return s.control(ctx, nil, controlInput{Action: "logs", Target: d.Name, Tail: d.Tail, Since: d.Since, Until: d.Until, Timestamps: d.Timestamps})
	case "":
		return nil, nil, errors.New("'action' é obrigatório")
	default:
		return nil, nil, errors.New("'action' inválido: containers, images, volumes, networks, info, stats, disk, inspect, logs")
	}
}

type input struct {
	Action string `json:"action" jsonschema:"containers, images, volumes, networks, info, stats, disk, inspect or logs (required)."`
	Data   data   `json:"data,omitempty" jsonschema:"Options for the chosen action."`
}

type data struct {
	Name        string `json:"name,omitempty" jsonschema:"Container name/ID (stats, logs) or resource name (inspect)."`
	Resource    string `json:"resource,omitempty" jsonschema:"Inspect type: container, image, volume or network."`
	Filter      string `json:"filter,omitempty" jsonschema:"Filters as key=value, comma-separated."`
	Limit       int    `json:"limit,omitempty" jsonschema:"Max rows; 0 = all."`
	Last        int    `json:"last,omitempty" jsonschema:"Last N created containers."`
	RunningOnly bool   `json:"runningOnly,omitempty" jsonschema:"Only running containers."`
	All         bool   `json:"all,omitempty" jsonschema:"Include intermediate images."`
	Verbose     bool   `json:"verbose,omitempty" jsonschema:"Disk: per-object usage."`
	Tail        int    `json:"tail,omitempty" jsonschema:"Logs: lines from the end (default 200, max 10000)."`
	Since       string `json:"since,omitempty" jsonschema:"Logs: since timestamp or duration (e.g. 10m)."`
	Until       string `json:"until,omitempty" jsonschema:"Logs: until timestamp or duration."`
	Timestamps  bool   `json:"timestamps,omitempty" jsonschema:"Logs: prefix lines with timestamps."`
}
