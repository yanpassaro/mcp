package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func (s *Server) read(ctx context.Context, _ *mcp.CallToolRequest, in input) (*mcp.CallToolResult, any, error) {
	action := strings.ToLower(strings.TrimSpace(in.Action))
	d := in.Data
	switch action {
	case "info":
		return s.getRepo(ctx, nil, repoInput{Owner: in.Owner, Repo: in.Repo})
	case "tree":
		return s.getTree(ctx, nil, treeInput{Owner: in.Owner, Repo: in.Repo, Ref: d.Ref, Recursive: d.Recursive})
	case "file":
		return s.fetchFile(ctx, nil, fetchFileInput{Owner: in.Owner, Repo: in.Repo, Path: d.Path, Ref: d.Ref})
	case "item":
		return s.getItem(ctx, nil, itemInput{Owner: in.Owner, Repo: in.Repo, Number: d.Number, Type: d.Type})
	case "releases":
		return s.releases(ctx, in.Owner, in.Repo, d.PerPage, d.Page)
	case "insights":
		return s.insights(ctx, in.Owner, in.Repo, d.Metric)
	case "":
		return nil, nil, errors.New("'action' é obrigatório")
	default:
		return nil, nil, errors.New("'action' inválido: info, tree, file, item, releases, insights")
	}
}

func (s *Server) releases(ctx context.Context, owner, repo string, perPage, page int) (*mcp.CallToolResult, any, error) {
	if !hasOwnerRepo(owner, repo) {
		return nil, nil, errors.New("owner e repo são obrigatórios")
	}
	items, err := s.client.ListReleases(ctx, owner, repo, perPage, page)
	if err != nil {
		return nil, nil, err
	}
	return textResult(fmt.Sprintf("%s%s", formatReleases(items), s.rateLimitFooter()))
}

func (s *Server) insights(ctx context.Context, owner, repo, metric string) (*mcp.CallToolResult, any, error) {
	if !hasOwnerRepo(owner, repo) {
		return nil, nil, errors.New("owner e repo são obrigatórios")
	}
	metric = strings.ToLower(strings.TrimSpace(metric))
	if metric == "" {
		return nil, nil, errors.New("'metric' é obrigatório: contributors, commit_activity, code_frequency, participation, punch_card")
	}
	data, err := s.client.GetInsight(ctx, owner, repo, metric)
	if err != nil {
		return nil, nil, err
	}
	return textResult(fmt.Sprintf("%s%s", formatInsights(metric, data), s.rateLimitFooter()))
}

type input struct {
	Owner  string `json:"owner,omitempty" jsonschema:"Repository owner (user or organization)."`
	Repo   string `json:"repo,omitempty" jsonschema:"Repository name."`
	Action string `json:"action" jsonschema:"info, tree, file, item, releases or insights (required)."`
	Data   data   `json:"data,omitempty" jsonschema:"Options for the chosen action."`
}

type data struct {
	Ref       string `json:"ref,omitempty" jsonschema:"Branch, tag or SHA (tree, file)."`
	Path      string `json:"path,omitempty" jsonschema:"File path (file)."`
	Recursive bool   `json:"recursive,omitempty" jsonschema:"Full tree (tree)."`
	Number    int    `json:"number,omitempty" jsonschema:"Issue or PR number (item)."`
	Type      string `json:"type,omitempty" jsonschema:"item: issue or pr."`
	Metric    string `json:"metric,omitempty" jsonschema:"insights: contributors, commit_activity, code_frequency, participation or punch_card."`
	PerPage   int    `json:"size,omitempty" jsonschema:"releases: results per page."`
	Page      int    `json:"page,omitempty" jsonschema:"releases: page number."`
}
