package mcpserver

import (
	"context"
	"errors"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func (s *Server) readTool(ctx context.Context, _ *mcp.CallToolRequest, in readInput) (*mcp.CallToolResult, any, error) {
	action := strings.ToLower(strings.TrimSpace(in.Action))
	if action == "" {
		return nil, nil, errors.New("'action' é obrigatório")
	}
	switch action {
	case "info":
		return s.repoInfo(ctx, nil, repoInfoInput{Repo: in.Repo})
	case "status":
		return s.status(ctx, nil, statusInput{Repo: in.Repo})
	case "refs":
		return s.refs(ctx, nil, refsInput{Repo: in.Repo, Type: in.Data.Type, All: in.Data.All})
	case "file":
		return s.readFile(ctx, nil, readFileInput{Repo: in.Repo, Path: in.Data.Path, Ref: in.Data.Ref})
	case "show":
		return s.show(ctx, nil, showInput{Repo: in.Repo, Ref: in.Data.Ref})
	case "diff":
		return s.diff(ctx, nil, diffInput{
			Repo:    in.Repo,
			Base:    in.Data.Base,
			Head:    in.Data.Head,
			Staged:  in.Data.Staged,
			Stat:    in.Data.Stat,
			Path:    in.Data.Path,
			Context: in.Data.Context,
		})
	case "blame":
		return s.blame(ctx, nil, blameInput{Repo: in.Repo, Path: in.Data.Path, MaxLines: in.Data.MaxLines})
	default:
		return nil, nil, errors.New("'action' inválido: info, status, refs, file, show, diff, blame")
	}
}

type readInput struct {
	Repo   string   `json:"repo" jsonschema:"Path to the Git repository (required)."`
	Action string   `json:"action" jsonschema:"Action: info, status, refs, file, show, diff or blame (required)."`
	Data   readData `json:"data,omitempty" jsonschema:"Options for the chosen action; fields that do not apply are ignored."`
}

type readData struct {
	Type     string `json:"type,omitempty" jsonschema:"Refs type: branch, remote or tag."`
	All      bool   `json:"all,omitempty" jsonschema:"Include remote branches."`
	Ref      string `json:"ref,omitempty" jsonschema:"Revision (default: working tree for file, HEAD for show)."`
	Path     string `json:"path,omitempty" jsonschema:"File path (file, blame) or prefix (diff)."`
	Base     string `json:"base,omitempty" jsonschema:"Diff base ref."`
	Head     string `json:"head,omitempty" jsonschema:"Diff final ref."`
	Staged   bool   `json:"staged,omitempty" jsonschema:"Diff the index against HEAD."`
	Stat     bool   `json:"stat,omitempty" jsonschema:"Diff summary only."`
	Context  int    `json:"context,omitempty" jsonschema:"Diff context lines."`
	MaxLines int    `json:"maxLines,omitempty" jsonschema:"Max blame lines."`
}
