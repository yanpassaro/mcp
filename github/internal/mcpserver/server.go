package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"ntdsk.com/mcp/github/internal/github"
)

type Server struct {
	client *github.Client
}

func New(client *github.Client) *Server {
	return &Server{client: client}
}

func (s *Server) Register(server *mcp.Server) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "github_search",
		Description: "Unified GitHub search. 'type' selects the endpoint: code (/search/code), repo (/search/repositories), issue (/search/issues), pr (/search/issues + is:pr), commit (/search/commits; or lists a repo's commits with repo:owner/name), user (/search/users). 'query' takes GitHub qualifiers (e.g. 'func Validate extension:go repo:owner/name', 'topic:llm stars:>100', 'bug is:issue is:open'); sort/order/perPage/page are optional.",
	}, s.searchTool)


	mcp.AddTool(server, &mcp.Tool{
		Name:        "github_get_tree",
		Description: "List a repository's file tree (git trees). Provide owner and repo; ref is optional (branch, tag or SHA). If omitted, uses the default branch; if provided but invalid, also falls back to the default branch. recursive=true returns the entire tree. Useful for understanding folder/file structure and where to find documentation or code.",
	}, s.getTree)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "github_read_file",
		Description: "Read the full content of a repository file (endpoint /repos/{owner}/{repo}/contents/{path}). Returns the file text (markdown, code, etc.) in UTF-8. ref is optional (uses the default branch when omitted, or falls back to the default branch if the given one does not exist). Files over 200KB are truncated.",
	}, s.fetchFile)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "github_repo_info",
		Description: "Repository info by owner/name (endpoint /repos/{owner}/{repo}): stars, forks, license, default branch, topics, description, dates — plus the latest 5 releases.",
	}, s.getRepo)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "github_get_item",
		Description: "Read a single issue or pull request by number (endpoint /repos/{owner}/{repo}/issues/{number} for issue, /pulls/{number} for PR). 'type' selects: issue (state, author, assignees, labels, dates, body) or pr (state open/closed/merged, source/target branch, mergeable, commits, +additions/-deletions, body).",
	}, s.getItem)
}

func (s *Server) rateLimitFooter() string {
	if rem := s.client.RateLimitRemaining(); rem > 0 {
		return fmt.Sprintf("\n\n---\nRate limit GitHub: **%d** requisições restantes.", rem)
	}
	return ""
}

type repoInput struct {
	Owner string `json:"owner" jsonschema:"Repository owner (user or organization)"`
	Repo  string `json:"repo" jsonschema:"Repository name"`
}

func (s *Server) getRepo(ctx context.Context, _ *mcp.CallToolRequest, in repoInput) (*mcp.CallToolResult, any, error) {
	if strings.TrimSpace(in.Owner) == "" || strings.TrimSpace(in.Repo) == "" {
		return nil, nil, errors.New("owner e repo são obrigatórios")
	}
	raw, err := s.client.Get(ctx, "/repos/"+in.Owner+"/"+in.Repo)
	if err != nil {
		return nil, nil, err
	}
	m, ok := raw.(map[string]any)
	if !ok {
		return nil, nil, errors.New("resposta inesperada do GitHub")
	}
	var b strings.Builder
	b.WriteString(formatRepoDetail(m))
	if items, err := s.client.ListReleases(ctx, in.Owner, in.Repo, 5, 1); err == nil && len(items) > 0 {
		b.WriteString("\n\n")
		b.WriteString(formatReleases(items))
	}
	b.WriteString(s.rateLimitFooter())
	return textResult(b.String())
}

type itemInput struct {
	Owner  string `json:"owner" jsonschema:"Repository owner (user or organization)"`
	Repo   string `json:"repo" jsonschema:"Repository name"`
	Number int    `json:"number" jsonschema:"Issue or PR number"`
	Type   string `json:"type" jsonschema:"What to read: issue or pr (required)."`
}

func (s *Server) getItem(ctx context.Context, _ *mcp.CallToolRequest, in itemInput) (*mcp.CallToolResult, any, error) {
	if strings.TrimSpace(in.Owner) == "" || strings.TrimSpace(in.Repo) == "" || in.Number <= 0 {
		return nil, nil, errors.New("owner, repo e number são obrigatórios")
	}
	switch strings.ToLower(strings.TrimSpace(in.Type)) {
	case "issue", "issues":
		raw, err := s.client.Get(ctx, fmt.Sprintf("/repos/%s/%s/issues/%d", in.Owner, in.Repo, in.Number))
		if err != nil {
			return nil, nil, err
		}
		m, ok := raw.(map[string]any)
		if !ok {
			return nil, nil, errors.New("resposta inesperada do GitHub")
		}
		return textResult(formatIssueDetail(m) + s.rateLimitFooter())
	case "pr", "pull", "pullrequest", "pull_request", "pull-request":
		raw, err := s.client.Get(ctx, fmt.Sprintf("/repos/%s/%s/pulls/%d", in.Owner, in.Repo, in.Number))
		if err != nil {
			return nil, nil, err
		}
		m, ok := raw.(map[string]any)
		if !ok {
			return nil, nil, errors.New("resposta inesperada do GitHub")
		}
		return textResult(formatPullRequestDetail(m) + s.rateLimitFooter())
	default:
		return nil, nil, fmt.Errorf("'type' inválido: use issue ou pr")
	}
}

func (s *Server) search(ctx context.Context, kind string, query string, sort, order string, perPage, page int) (*mcp.CallToolResult, any, error) {
	if strings.TrimSpace(query) == "" {
		return nil, nil, errors.New("query é obrigatório")
	}
	params := url.Values{}
	params.Set("q", strings.TrimSpace(query))
	if strings.TrimSpace(sort) != "" {
		params.Set("sort", sort)
	}
	if strings.TrimSpace(order) != "" {
		params.Set("order", order)
	}
	params.Set("per_page", strconv.Itoa(clampPerPage(perPage)))
	if page > 0 {
		params.Set("page", strconv.Itoa(page))
	}
	items, total, err := s.client.Search(ctx, kind, params)
	if err != nil {
		return nil, nil, err
	}
	return textResult(formatSearchKind(kind, items, total) + s.rateLimitFooter())
}

func (s *Server) searchTool(ctx context.Context, _ *mcp.CallToolRequest, in searchInput) (*mcp.CallToolResult, any, error) {
	switch strings.ToLower(strings.TrimSpace(in.Type)) {
	case "code":
		return s.search(ctx, "code", in.Query, in.Sort, in.Order, in.PerPage, in.Page)
	case "repo", "repository", "repositories":
		return s.search(ctx, "repositories", in.Query, in.Sort, in.Order, in.PerPage, in.Page)
	case "issue", "issues":
		return s.search(ctx, "issues", in.Query, in.Sort, in.Order, in.PerPage, in.Page)
	case "pr", "pull", "pullrequest", "pull_request", "pull-request":
		q := strings.TrimSpace(in.Query)
		if !strings.Contains(strings.ToLower(q), "is:pr") {
			if q != "" {
				q += " "
			}
			q += "is:pr"
		}
		return s.search(ctx, "issues", q, in.Sort, in.Order, in.PerPage, in.Page)
	case "commit", "commits":
		q := strings.TrimSpace(in.Query)
		if queryHasFreeTerm(q) {
			return s.search(ctx, "commits", q, in.Sort, in.Order, in.PerPage, in.Page)
		}
		return s.listCommitsFallback(ctx, commitSearchInput{
			Query:   in.Query,
			Sort:    in.Sort,
			Order:   in.Order,
			PerPage: in.PerPage,
			Page:    in.Page,
		})
	case "user", "users":
		return s.search(ctx, "users", in.Query, in.Sort, in.Order, in.PerPage, in.Page)
	default:
		return nil, nil, fmt.Errorf("'type' inválido: use code, repo, issue, pr, commit ou user")
	}
}

func (s *Server) listCommitsFallback(ctx context.Context, in commitSearchInput) (*mcp.CallToolResult, any, error) {
	owner, repo, params := commitListParams(in.Query)
	if owner == "" || repo == "" {
		return nil, nil, errors.New("para listar commits sem um termo de texto, informe o qualificador repo:owner/nome (ex.: 'repo:octocat/Hello-World')")
	}
	params.Set("per_page", strconv.Itoa(clampPerPage(in.PerPage)))
	if in.Page > 0 {
		params.Set("page", strconv.Itoa(in.Page))
	}
	items, err := s.client.ListCommits(ctx, owner, repo, params)
	if err != nil {
		return nil, nil, err
	}
	return textResult(formatCommitSearch(items, len(items)) + s.rateLimitFooter())
}

func queryHasFreeTerm(q string) bool {
	for _, tok := range strings.Fields(q) {
		if !strings.Contains(tok, ":") {
			return true
		}
	}
	return false
}

func commitListParams(q string) (owner, repo string, params url.Values) {
	params = url.Values{}
	for _, tok := range strings.Fields(q) {
		i := strings.Index(tok, ":")
		if i <= 0 {
			continue
		}
		key := strings.ToLower(tok[:i])
		val := tok[i+1:]
		switch key {
		case "repo":
			parts := strings.SplitN(val, "/", 2)
			if len(parts) == 2 {
				owner, repo = parts[0], parts[1]
			}
		case "author":
			params.Set("author", val)
		case "committer":
			params.Set("committer", val)
		case "path":
			params.Set("path", val)
		case "since":
			params.Set("since", val)
		case "until":
			params.Set("until", val)
		}
	}
	return owner, repo, params
}




func (s *Server) getTree(ctx context.Context, _ *mcp.CallToolRequest, in treeInput) (*mcp.CallToolResult, any, error) {
	if strings.TrimSpace(in.Owner) == "" || strings.TrimSpace(in.Repo) == "" {
		return nil, nil, errors.New("owner e repo são obrigatórios")
	}
	items, err := s.client.GetTree(ctx, in.Owner, in.Repo, in.Ref, in.Recursive)
	if err != nil {
		return nil, nil, err
	}
	return textResult(formatTree(items, len(items)) + s.rateLimitFooter())
}

func (s *Server) fetchFile(ctx context.Context, _ *mcp.CallToolRequest, in fetchFileInput) (*mcp.CallToolResult, any, error) {
	if strings.TrimSpace(in.Owner) == "" || strings.TrimSpace(in.Repo) == "" || strings.TrimSpace(in.Path) == "" {
		return nil, nil, errors.New("owner, repo e path são obrigatórios")
	}
	content, truncated, err := s.client.GetFile(ctx, in.Owner, in.Repo, in.Path, in.Ref)
	if err != nil {
		return nil, nil, err
	}
	var b strings.Builder
	fmt.Fprintf(&b, "📄 %s/%s — %s", in.Owner, in.Repo, in.Path)
	if truncated {
		b.WriteString(" (truncado em 200KB)")
	}
	b.WriteString("\n\n")
	fmt.Fprintf(&b, "```%s\n", langFromPath(in.Path))
	b.WriteString(content)
	if !strings.HasSuffix(content, "\n") {
		b.WriteString("\n")
	}
	b.WriteString("```")
	return textResult(b.String() + s.rateLimitFooter())
}

type searchInput struct {
	Type    string `json:"type" jsonschema:"Type of search: code, repo, issue, pr, commit or user (required)."`
	Query   string `json:"query" jsonschema:"Search query with GitHub qualifiers, e.g.: 'func Validate extension:go repo:owner/name', 'topic:llm stars:>100', 'bug is:issue is:open label:bug'."`
	Sort    string `json:"sort,omitempty" jsonschema:"Sort order (depends on type)."`
	Order   string `json:"order,omitempty" jsonschema:"asc or desc."`
	PerPage int    `json:"perPage,omitempty" jsonschema:"Results per page (1-100, default 30)."`
	Page    int    `json:"page,omitempty" jsonschema:"Page number (default 1)."`
}

type commitSearchInput struct {
	Query   string `json:"query" jsonschema:"Commit search query with qualifiers (e.g.: 'fix timeout repo:owner/name author:octocat')."`
	Sort    string `json:"sort" jsonschema:"Sort: author-date, committer-date."`
	Order   string `json:"order" jsonschema:"asc or desc."`
	PerPage int    `json:"perPage" jsonschema:"Results per page (1-100, default 30)."`
	Page    int    `json:"page" jsonschema:"Page number (default 1)."`
}



type treeInput struct {
	Owner     string `json:"owner" jsonschema:"Repository owner (e.g.: octocat)."`
	Repo      string `json:"repo" jsonschema:"Repository name (e.g.: Hello-World)."`
	Ref       string `json:"ref" jsonschema:"Branch, tag or SHA. Optional; uses the default branch when omitted."`
	Recursive bool   `json:"recursive" jsonschema:"If true, returns the full tree recursively."`
}

type fetchFileInput struct {
	Owner string `json:"owner" jsonschema:"Repository owner (e.g.: octocat)."`
	Repo  string `json:"repo" jsonschema:"Repository name (e.g.: Hello-World)."`
	Path  string `json:"path" jsonschema:"File path in the repository (e.g.: docs/README.md)."`
	Ref   string `json:"ref" jsonschema:"Branch, tag or SHA. Optional; uses the default branch when omitted."`
}

func clampPerPage(p int) int {
	if p <= 0 {
		return 30
	}
	if p > 100 {
		return 100
	}
	return p
}

func formatSearchKind(kind string, items []any, total int) string {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "code":
		return formatCodeSearch(items, total)
	case "repositories":
		return formatRepoSearch(items, total)
	case "issues":
		return formatIssueSearch(items, total)
	case "commits":
		return formatCommitSearch(items, total)
	case "users":
		return formatUserSearch(items, total)
	default:
		return formatCodeSearch(items, total)
	}
}

func langFromPath(path string) string {
	if idx := strings.LastIndex(path, "."); idx >= 0 {
		path = path[idx+1:]
	}
	ext := strings.ToLower(path)
	switch ext {
	case "rs":
		return "rust"
	case "go":
		return "go"
	case "py", "pyi":
		return "python"
	case "js", "mjs", "cjs":
		return "javascript"
	case "ts":
		return "typescript"
	case "tsx", "jsx":
		return "tsx"
	case "json":
		return "json"
	case "yml", "yaml":
		return "yaml"
	case "toml":
		return "toml"
	case "md", "markdown":
		return "markdown"
	case "sh", "bash", "zsh":
		return "bash"
	case "c", "h":
		return "c"
	case "cpp", "cc", "cxx", "hpp":
		return "cpp"
	case "java":
		return "java"
	case "rb":
		return "ruby"
	case "php":
		return "php"
	case "cs":
		return "csharp"
	case "swift":
		return "swift"
	case "kt", "kts":
		return "kotlin"
	case "sql":
		return "sql"
	case "html", "htm":
		return "html"
	case "css", "scss", "sass":
		return "css"
	case "xml":
		return "xml"
	case "proto":
		return "protobuf"
	default:
		return ""
	}
}

func textResult(text string) (*mcp.CallToolResult, any, error) {
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: text}},
	}, nil, nil
}
