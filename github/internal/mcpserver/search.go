package mcpserver

import (
	"context"
	"errors"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func (s *Server) search(ctx context.Context, _ *mcp.CallToolRequest, in searchInput) (*mcp.CallToolResult, any, error) {
	action := strings.ToLower(strings.TrimSpace(in.Action))
	d := in.Data
	switch action {
	case "code":
		return s.find(ctx, "code", d.Query, d.Sort, d.Order, d.PerPage, d.Page)
	case "repo", "repository", "repositories":
		return s.find(ctx, "repositories", d.Query, d.Sort, d.Order, d.PerPage, d.Page)
	case "issue", "issues":
		return s.find(ctx, "issues", d.Query, d.Sort, d.Order, d.PerPage, d.Page)
	case "pr", "pull", "pullrequest", "pull_request", "pull-request":
		return s.find(ctx, "issues", withIsPR(strings.TrimSpace(d.Query)), d.Sort, d.Order, d.PerPage, d.Page)
	case "commit", "commits":
		q := strings.TrimSpace(d.Query)
		if queryHasFreeTerm(q) {
			return s.find(ctx, "commits", q, d.Sort, d.Order, d.PerPage, d.Page)
		}
		return s.listCommitsFallback(ctx, commitSearchInput{
			Query:   d.Query,
			Sort:    d.Sort,
			Order:   d.Order,
			PerPage: d.PerPage,
			Page:    d.Page,
		})
	case "user", "users":
		return s.find(ctx, "users", d.Query, d.Sort, d.Order, d.PerPage, d.Page)
	case "":
		return nil, nil, errors.New("'action' é obrigatório")
	default:
		return nil, nil, errors.New("'action' inválido: code, repo, issue, pr, commit, user")
	}
}

type searchInput struct {
	Action string     `json:"action" jsonschema:"code, repo, issue, pr, commit or user (required)."`
	Data   searchData `json:"data,omitempty" jsonschema:"Options for the chosen action."`
}

type searchData struct {
	Query   string `json:"query" jsonschema:"Search query with GitHub qualifiers, e.g. 'extension:go repo:owner/name'."`
	Sort    string `json:"sort,omitempty" jsonschema:"Sort order (depends on action)."`
	Order   string `json:"order,omitempty" jsonschema:"asc or desc."`
	PerPage int    `json:"perPage,omitempty" jsonschema:"Results per page (1-100, default 30)."`
	Page    int    `json:"page,omitempty" jsonschema:"Page number (default 1)."`
}
