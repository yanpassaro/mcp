package mcpserver

import (
	"context"
	"errors"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func (s *Server) searchTool(ctx context.Context, _ *mcp.CallToolRequest, in searchInput) (*mcp.CallToolResult, any, error) {
	action := strings.ToLower(strings.TrimSpace(in.Action))
	if action == "" {
		return nil, nil, errors.New("'action' é obrigatório")
	}
	switch action {
	case "commits":
		return s.log(ctx, nil, logInput{
			Repo:     in.Repo,
			MaxCount: in.Data.MaxCount,
			Query:    in.Data.Query,
			Author:   in.Data.Author,
			Path:     in.Data.Path,
			Since:    in.Data.Since,
			Until:    in.Data.Until,
			Stat:     in.Data.Stat,
		})
	case "files":
		return s.tree(ctx, nil, treeInput{
			Repo:    in.Repo,
			Ref:     in.Data.Ref,
			Path:    in.Data.Path,
			Pattern: in.Data.Pattern,
		})
	default:
		return nil, nil, errors.New("'action' inválido: commits, files")
	}
}

type searchInput struct {
	Repo   string     `json:"repo" jsonschema:"Path to the Git repository (required)."`
	Action string     `json:"action" jsonschema:"Action: commits (history and message search) or files (tracked files at a ref) (required)."`
	Data   searchData `json:"data,omitempty" jsonschema:"Options for the chosen action; fields that do not apply are ignored."`
}

type searchData struct {
	Query    string `json:"query,omitempty" jsonschema:"Message text (case-insensitive)."`
	Author   string `json:"author,omitempty" jsonschema:"Author name or e-mail."`
	Path     string `json:"path,omitempty" jsonschema:"Touched file (commits) or path prefix (files)."`
	Since    string `json:"since,omitempty" jsonschema:"Start date (YYYY-MM-DD)."`
	Until    string `json:"until,omitempty" jsonschema:"End date (YYYY-MM-DD)."`
	Stat     bool   `json:"stat,omitempty" jsonschema:"Show changed files per commit."`
	MaxCount int    `json:"maxCount,omitempty" jsonschema:"Max commits (default 30)."`
	Ref      string `json:"ref,omitempty" jsonschema:"Ref (default HEAD)."`
	Pattern  string `json:"pattern,omitempty" jsonschema:"Glob or substring filter."`
}
