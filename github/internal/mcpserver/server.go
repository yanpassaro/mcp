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
		Name:        "github_search_code",
		Description: "Search code on GitHub (endpoint /search/code). The query parameter accepts GitHub qualifiers, e.g.: 'func Validate extension:go repo:owner/name', 'authentication extension:md path:docs', 'retry language:Python org:octocat'. Useful qualifiers: repo:, org:, user:, language:, path:, extension:, filename:, in:file, size:>100. Ideal for finding code snippets and documentation files.",
	}, s.searchCode)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "github_search_repositories",
		Description: "Search repositories on GitHub (endpoint /search/repositories). Qualifiers: language:, stars:>10, forks:>5, user:, org:, topic:, pushed:>2024-01-01, created:<2020-01-01, size:>1000, is:public/is:private.",
	}, s.searchRepositories)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "github_search_issues",
		Description: "Search issues and pull requests on GitHub (endpoint /search/issues). Qualifiers: repo:, org:, is:issue, is:pr, is:open, is:closed, label:, author:, assignee:, involves:, type:, state:open, no:label, comments:>5. Output includes type (Issue/PR), state (open/closed/merged), author, repository, labels, assignees, comment count, creation/update dates and the snippet that matched. Great for finding discussions, known bugs and code decisions.",
	}, s.searchIssues)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "github_search_commits",
		Description: "Search or list commits on GitHub. If the query has a free-text term, uses /search/commits (qualifiers: repo:, org:, author:, committer:, author-date:>2024-01-01, committer-date:<2024-06-01, merge:true, parent:.<n>). If it has ONLY qualifiers (e.g. 'repo:owner/name'), lists the repository's commits via /repos/{owner}/{repo}/commits (supports author:, committer:, path:, since:, until:; always newest first). Output includes author and committer, date, repository, merge detection (number of parents), signature status and the message snippet that matched.",
	}, s.searchCommits)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "github_search_pull_requests",
		Description: "Search pull requests on GitHub (search/issues with is:pr). Qualifiers: repo:, org:, is:open, is:closed, is:merged, label:, author:, assignee:, involves:, base:, head:, review:, comments:>5. Output is the same as issues (type PR, state, merged, author, repository, labels, assignees, comments, dates and matched snippet).",
	}, s.searchPullRequests)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "github_search_users",
		Description: "Search users on GitHub (endpoint /search/users). Qualifiers: type:user, type:org, repos:>5, followers:>100, location:Brazil, language:Go.",
	}, s.searchUsers)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "github_discover",
		Description: "Discover libraries and projects across the ENTIRE GitHub without specifying a repo or org. Ideal for finding a library (e.g. an Excel parser): pass a free-text query like 'excel' plus optional language:/topic: filters. Results are sorted by stars by default. Equivalent to a global repository search (endpoint /search/repositories).",
	}, s.discover)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "github_list_releases",
		Description: "List a repository's releases (endpoint /repos/{owner}/{repo}/releases), most recent first. Shows tag, name, target branch, author, publish date, whether it is a draft/pre-release, notes excerpt and attached assets.",
	}, s.listReleases)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "github_get_release_latest",
		Description: "Return the latest published release of a repository (endpoint /repos/{owner}/{repo}/releases/latest).",
	}, s.getLatestRelease)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "github_list_wiki",
		Description: "List a repository's wiki pages. The wiki is accessed as the git repository {repo}.wiki via git trees. Shows the wiki file tree (Home.md, Sidebar.md, etc.). ref is optional (usually master for wikis).",
	}, s.listWiki)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "github_get_insights",
		Description: "Return a repository's Insights metrics (stats API). metric: contributors (list of contributors and commit counts), commit_activity (commits per week), code_frequency (additions/removals per week), participation (owner vs all commits over 52 weeks), punch_card (busiest day/hour). Some metrics take time to compute (HTTP 202) and require a retry.",
	}, s.getInsights)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "github_fetch_wiki",
		Description: "Read the content of a wiki page (endpoint /repos/{owner}/{repo}.wiki/contents/{path}). Returns the page text (usually markdown) in UTF-8. Use github_list_wiki first to discover the exact paths (e.g. Home.md). path is required; ref is optional (uses the wiki branch, usually master, when omitted, or falls back to the default branch if the given one does not exist). Files over 200KB are truncated.",
	}, s.fetchWiki)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "github_get_tree",
		Description: "List a repository's file tree (git trees). Provide owner and repo; ref is optional (branch, tag or SHA). If omitted, uses the default branch; if provided but invalid, also falls back to the default branch. recursive=true returns the entire tree. Useful for understanding folder/file structure and where to find documentation or code.",
	}, s.getTree)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "github_fetch_file",
		Description: "Read the full content of a repository file (endpoint /repos/{owner}/{repo}/contents/{path}). Returns the file text (markdown, code, etc.) in UTF-8. ref is optional (uses the default branch when omitted, or falls back to the default branch if the given one does not exist). Files over 200KB are truncated.",
	}, s.fetchFile)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "github_get_repo",
		Description: "Read a single repository by owner/name (endpoint /repos/{owner}/{repo}): stars, forks, license, default branch, topics, description, dates.",
	}, s.getRepo)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "github_get_issue",
		Description: "Read a single issue by number (endpoint /repos/{owner}/{repo}/issues/{number}): state, author, assignees, labels, dates and full body.",
	}, s.getIssue)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "github_get_pull_request",
		Description: "Read a single pull request by number (endpoint /repos/{owner}/{repo}/pulls/{number}): state (open/closed/merged), source/target branch, mergeable, commits, +additions/-deletions and full body.",
	}, s.getPullRequest)
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
	return textResult(formatRepoDetail(m) + s.rateLimitFooter())
}

type issueInput struct {
	Owner  string `json:"owner" jsonschema:"Repository owner (user or organization)"`
	Repo   string `json:"repo" jsonschema:"Repository name"`
	Number int    `json:"number" jsonschema:"Issue number"`
}

func (s *Server) getIssue(ctx context.Context, _ *mcp.CallToolRequest, in issueInput) (*mcp.CallToolResult, any, error) {
	if strings.TrimSpace(in.Owner) == "" || strings.TrimSpace(in.Repo) == "" || in.Number <= 0 {
		return nil, nil, errors.New("owner, repo e number são obrigatórios")
	}
	raw, err := s.client.Get(ctx, fmt.Sprintf("/repos/%s/%s/issues/%d", in.Owner, in.Repo, in.Number))
	if err != nil {
		return nil, nil, err
	}
	m, ok := raw.(map[string]any)
	if !ok {
		return nil, nil, errors.New("resposta inesperada do GitHub")
	}
	return textResult(formatIssueDetail(m) + s.rateLimitFooter())
}

type prInput struct {
	Owner  string `json:"owner" jsonschema:"Repository owner (user or organization)"`
	Repo   string `json:"repo" jsonschema:"Repository name"`
	Number int    `json:"number" jsonschema:"Pull request number"`
}

func (s *Server) getPullRequest(ctx context.Context, _ *mcp.CallToolRequest, in prInput) (*mcp.CallToolResult, any, error) {
	if strings.TrimSpace(in.Owner) == "" || strings.TrimSpace(in.Repo) == "" || in.Number <= 0 {
		return nil, nil, errors.New("owner, repo e number são obrigatórios")
	}
	raw, err := s.client.Get(ctx, fmt.Sprintf("/repos/%s/%s/pulls/%d", in.Owner, in.Repo, in.Number))
	if err != nil {
		return nil, nil, err
	}
	m, ok := raw.(map[string]any)
	if !ok {
		return nil, nil, errors.New("resposta inesperada do GitHub")
	}
	return textResult(formatPullRequestDetail(m) + s.rateLimitFooter())
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

func (s *Server) searchCode(ctx context.Context, _ *mcp.CallToolRequest, in codeSearchInput) (*mcp.CallToolResult, any, error) {
	return s.search(ctx, "code", in.Query, in.Sort, in.Order, in.PerPage, in.Page)
}

func (s *Server) searchRepositories(ctx context.Context, _ *mcp.CallToolRequest, in repoSearchInput) (*mcp.CallToolResult, any, error) {
	return s.search(ctx, "repositories", in.Query, in.Sort, in.Order, in.PerPage, in.Page)
}

func (s *Server) searchIssues(ctx context.Context, _ *mcp.CallToolRequest, in issueSearchInput) (*mcp.CallToolResult, any, error) {
	return s.search(ctx, "issues", in.Query, in.Sort, in.Order, in.PerPage, in.Page)
}

func (s *Server) searchCommits(ctx context.Context, _ *mcp.CallToolRequest, in commitSearchInput) (*mcp.CallToolResult, any, error) {
	q := strings.TrimSpace(in.Query)
	if queryHasFreeTerm(q) {
		return s.search(ctx, "commits", q, in.Sort, in.Order, in.PerPage, in.Page)
	}
	return s.listCommitsFallback(ctx, in)
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

func (s *Server) searchPullRequests(ctx context.Context, _ *mcp.CallToolRequest, in prSearchInput) (*mcp.CallToolResult, any, error) {
	q := strings.TrimSpace(in.Query)
	if !strings.Contains(strings.ToLower(q), "is:pr") {
		if q != "" {
			q += " "
		}
		q += "is:pr"
	}
	return s.search(ctx, "issues", q, in.Sort, in.Order, in.PerPage, in.Page)
}

func (s *Server) listReleases(ctx context.Context, _ *mcp.CallToolRequest, in releasesInput) (*mcp.CallToolResult, any, error) {
	if strings.TrimSpace(in.Owner) == "" || strings.TrimSpace(in.Repo) == "" {
		return nil, nil, errors.New("owner e repo são obrigatórios")
	}
	items, err := s.client.ListReleases(ctx, in.Owner, in.Repo, in.PerPage, in.Page)
	if err != nil {
		return nil, nil, err
	}
	return textResult(formatReleases(items) + s.rateLimitFooter())
}

func (s *Server) getLatestRelease(ctx context.Context, _ *mcp.CallToolRequest, in releaseInput) (*mcp.CallToolResult, any, error) {
	if strings.TrimSpace(in.Owner) == "" || strings.TrimSpace(in.Repo) == "" {
		return nil, nil, errors.New("owner e repo são obrigatórios")
	}
	m, err := s.client.GetLatestRelease(ctx, in.Owner, in.Repo)
	if err != nil {
		return nil, nil, err
	}
	return textResult(formatReleases([]any{m}) + s.rateLimitFooter())
}

func (s *Server) listWiki(ctx context.Context, _ *mcp.CallToolRequest, in wikiInput) (*mcp.CallToolResult, any, error) {
	if strings.TrimSpace(in.Owner) == "" || strings.TrimSpace(in.Repo) == "" {
		return nil, nil, errors.New("owner e repo são obrigatórios")
	}
	items, err := s.client.GetTree(ctx, in.Owner, in.Repo+".wiki", in.Ref, true)
	if err != nil {
		return nil, nil, err
	}
	return textResult(formatTree(items, len(items)) + s.rateLimitFooter())
}

func (s *Server) getInsights(ctx context.Context, _ *mcp.CallToolRequest, in insightsInput) (*mcp.CallToolResult, any, error) {
	if strings.TrimSpace(in.Owner) == "" || strings.TrimSpace(in.Repo) == "" {
		return nil, nil, errors.New("owner e repo são obrigatórios")
	}
	metric := strings.TrimSpace(in.Metric)
	if metric == "" {
		metric = "contributors"
	}
	data, err := s.client.GetInsight(ctx, in.Owner, in.Repo, metric)
	if err != nil {
		return nil, nil, err
	}
	return textResult(formatInsights(metric, data) + s.rateLimitFooter())
}

func (s *Server) fetchWiki(ctx context.Context, _ *mcp.CallToolRequest, in wikiPageInput) (*mcp.CallToolResult, any, error) {
	if strings.TrimSpace(in.Owner) == "" || strings.TrimSpace(in.Repo) == "" || strings.TrimSpace(in.Path) == "" {
		return nil, nil, errors.New("owner, repo e path são obrigatórios")
	}
	content, truncated, err := s.client.GetFile(ctx, in.Owner, in.Repo+".wiki", in.Path, in.Ref)
	if err != nil {
		return nil, nil, err
	}
	var b strings.Builder
	fmt.Fprintf(&b, "📄 %s/%s.wiki — %s", in.Owner, in.Repo, in.Path)
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

func (s *Server) searchUsers(ctx context.Context, _ *mcp.CallToolRequest, in userSearchInput) (*mcp.CallToolResult, any, error) {
	return s.search(ctx, "users", in.Query, in.Sort, in.Order, in.PerPage, in.Page)
}

func (s *Server) discover(ctx context.Context, _ *mcp.CallToolRequest, in discoverInput) (*mcp.CallToolResult, any, error) {
	q := strings.TrimSpace(in.Query)
	if l := strings.TrimSpace(in.Language); l != "" {
		if q != "" {
			q += " "
		}
		q += "language:" + l
	}
	if t := strings.TrimSpace(in.Topic); t != "" {
		if q != "" {
			q += " "
		}
		q += "topic:" + t
	}
	if strings.TrimSpace(q) == "" {
		return nil, nil, errors.New("query (or language/topic) is required")
	}
	sort := strings.TrimSpace(in.Sort)
	if sort == "" {
		sort = "stars"
	}
	order := strings.TrimSpace(in.Order)
	if order == "" {
		order = "desc"
	}
	return s.search(ctx, "repositories", q, sort, order, in.PerPage, in.Page)
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

type codeSearchInput struct {
	Query   string `json:"query" jsonschema:"GitHub code search query with qualifiers (e.g.: 'func Validate extension:go repo:owner/name')."`
	Sort    string `json:"sort" jsonschema:"Sort order: best-match (default) or indexed."`
	Order   string `json:"order" jsonschema:"asc or desc."`
	PerPage int    `json:"perPage" jsonschema:"Results per page (1-100, default 30)."`
	Page    int    `json:"page" jsonschema:"Page number (default 1)."`
}

type repoSearchInput struct {
	Query   string `json:"query" jsonschema:"Repository search query with qualifiers (e.g.: 'topic:llm language:Python stars:>100')."`
	Sort    string `json:"sort" jsonschema:"Sort: stars, forks, updated, created, pushed, followers."`
	Order   string `json:"order" jsonschema:"asc or desc."`
	PerPage int    `json:"perPage" jsonschema:"Results per page (1-100, default 30)."`
	Page    int    `json:"page" jsonschema:"Page number (default 1)."`
}

type issueSearchInput struct {
	Query   string `json:"query" jsonschema:"Issue/PR search query with qualifiers (e.g.: 'bug is:issue is:open repo:owner/name label:bug')."`
	Sort    string `json:"sort" jsonschema:"Sort: comments, reactions, reactions-+, reactions--, created, updated."`
	Order   string `json:"order" jsonschema:"asc or desc."`
	PerPage int    `json:"perPage" jsonschema:"Results per page (1-100, default 30)."`
	Page    int    `json:"page" jsonschema:"Page number (default 1)."`
}

type commitSearchInput struct {
	Query   string `json:"query" jsonschema:"Commit search query with qualifiers (e.g.: 'fix timeout repo:owner/name author:octocat')."`
	Sort    string `json:"sort" jsonschema:"Sort: author-date, committer-date."`
	Order   string `json:"order" jsonschema:"asc or desc."`
	PerPage int    `json:"perPage" jsonschema:"Results per page (1-100, default 30)."`
	Page    int    `json:"page" jsonschema:"Page number (default 1)."`
}

type prSearchInput struct {
	Query   string `json:"query" jsonschema:"PR search query with qualifiers (e.g.: 'bug fix repo:owner/name is:merged label:bug'). is:pr is added automatically."`
	Sort    string `json:"sort" jsonschema:"Sort: comments, reactions, reactions-+, reactions--, created, updated."`
	Order   string `json:"order" jsonschema:"asc or desc."`
	PerPage int    `json:"perPage" jsonschema:"Results per page (1-100, default 30)."`
	Page    int    `json:"page" jsonschema:"Page number (default 1)."`
}

type releasesInput struct {
	Owner   string `json:"owner" jsonschema:"Repository owner (e.g.: octocat)."`
	Repo    string `json:"repo" jsonschema:"Repository name (e.g.: Hello-World)."`
	PerPage int    `json:"perPage" jsonschema:"Results per page (1-100, default 30)."`
	Page    int    `json:"page" jsonschema:"Page number (default 1)."`
}

type releaseInput struct {
	Owner string `json:"owner" jsonschema:"Repository owner (e.g.: octocat)."`
	Repo  string `json:"repo" jsonschema:"Repository name (e.g.: Hello-World)."`
}

type wikiInput struct {
	Owner string `json:"owner" jsonschema:"Repository owner (e.g.: octocat)."`
	Repo  string `json:"repo" jsonschema:"Repository name (e.g.: Hello-World)."`
	Ref   string `json:"ref" jsonschema:"Wiki branch/tag (optional; defaults to the .wiki repo branch, usually master)."`
}

type wikiPageInput struct {
	Owner string `json:"owner" jsonschema:"Repository owner (e.g.: octocat)."`
	Repo  string `json:"repo" jsonschema:"Repository name (e.g.: Hello-World)."`
	Path  string `json:"path" jsonschema:"Wiki page path (e.g.: Home.md, Sidebar.md). Use github_list_wiki to discover paths."`
	Ref   string `json:"ref" jsonschema:"Wiki branch/tag (optional; defaults to the .wiki repo branch, usually master, or falls back to default)."`
}

type insightsInput struct {
	Owner  string `json:"owner" jsonschema:"Repository owner (e.g.: octocat)."`
	Repo   string `json:"repo" jsonschema:"Repository name (e.g.: Hello-World)."`
	Metric string `json:"metric" jsonschema:"Insights metric: contributors, commit_activity, code_frequency, participation, punch_card."`
}

type userSearchInput struct {
	Query   string `json:"query" jsonschema:"User search query with qualifiers (e.g.: 'language:Go location:Brazil followers:>100')."`
	Sort    string `json:"sort" jsonschema:"Sort: followers, repositories, joined."`
	Order   string `json:"order" jsonschema:"asc or desc."`
	PerPage int    `json:"perPage" jsonschema:"Results per page (1-100, default 30)."`
	Page    int    `json:"page" jsonschema:"Page number (default 1)."`
}

type discoverInput struct {
	Query    string `json:"query" jsonschema:"What you want to find, e.g.: 'excel', 'http client', 'machine learning'. Searches the entire GitHub — no repo: or org: needed."`
	Language string `json:"language" jsonschema:"Optional language filter (e.g.: Python, Go, TypeScript). Added as a language: qualifier."`
	Topic    string `json:"topic" jsonschema:"Optional topic filter (e.g.: excel, cli, sdk). Added as a topic: qualifier."`
	Sort     string `json:"sort" jsonschema:"Sort: stars (default), forks, updated, created, pushed, followers."`
	Order    string `json:"order" jsonschema:"asc or desc"`
	PerPage  int    `json:"perPage" jsonschema:"Results per page (1-100, default 30)."`
	Page     int    `json:"page" jsonschema:"Page number (default 1)."`
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
