package github

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	defaultBaseURL        = "https://api.github.com"
	maxJSONResponseBytes  = 32 * 1024 * 1024
	maxErrorResponseBytes = 8 * 1024
	maxFileBytes          = 200 * 1024
)

type httpStatusError struct {
	Status  int
	Message string
}

func (e *httpStatusError) Error() string {
	return fmt.Sprintf("GitHub returned HTTP %d: %s", e.Status, e.Message)
}

func isNotFound(err error) bool {
	var e *httpStatusError
	if errors.As(err, &e) {
		return e.Status == 404
	}
	return false
}

type Config struct {
	BaseURL    string
	Token      string
	HTTPClient *http.Client
}

type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client

	mu       sync.Mutex
	lastRem  int
	apiLimit int
}

func (c *Client) noteRateLimit(resp *http.Response) {
	if rem := resp.Header.Get("X-RateLimit-Remaining"); rem != "" {
		if n, err := strconv.Atoi(rem); err == nil {
			c.mu.Lock()
			c.lastRem = n
			c.mu.Unlock()
		}
	}
	if lim := resp.Header.Get("X-RateLimit-Limit"); lim != "" {
		if n, err := strconv.Atoi(lim); err == nil {
			c.mu.Lock()
			c.apiLimit = n
			c.mu.Unlock()
		}
	}
}

func (c *Client) RateLimitRemaining() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.lastRem
}

func NewClient(cfg Config) (*Client, error) {
	base := strings.TrimSpace(cfg.BaseURL)
	if base == "" {
		base = defaultBaseURL
	}
	u, err := url.Parse(base)
	if err != nil {
		return nil, fmt.Errorf("invalid GITHUB_BASE_URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, errors.New("GITHUB_BASE_URL must use http or https")
	}
	if u.Host == "" {
		return nil, errors.New("GITHUB_BASE_URL must include a host")
	}
	u.Path = strings.TrimRight(u.Path, "/")

	token := strings.TrimSpace(cfg.Token)
	if token == "" {
		return nil, errors.New("GITHUB_TOKEN is required")
	}
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 60 * time.Second}
	}
	return &Client{baseURL: u.String(), token: token, httpClient: httpClient}, nil
}

func (c *Client) Get(ctx context.Context, endpoint string) (any, error) {
	return c.get(ctx, endpoint, nil)
}

func (c *Client) getWithAccept(ctx context.Context, endpoint string, query url.Values, accept string) (any, error) {
	raw, err := c.getBytesWithAccept(ctx, endpoint, query, accept)
	if err != nil {
		return nil, err
	}
	var result any
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("decode GitHub response: %w", err)
	}
	return result, nil
}

func (c *Client) getBytesWithAccept(ctx context.Context, endpoint string, query url.Values, accept string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.endpointURL(endpoint, query), nil)
	if err != nil {
		return nil, fmt.Errorf("create GitHub request: %w", err)
	}
	if strings.TrimSpace(accept) == "" {
		accept = "application/vnd.github+json"
	}
	req.Header.Set("Accept", accept)
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request GitHub: %w", err)
	}
	defer resp.Body.Close()
	c.noteRateLimit(resp)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		detail, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorResponseBytes))
		message := strings.TrimSpace(string(detail))
		if message == "" {
			message = http.StatusText(resp.StatusCode)
		}
		return nil, &httpStatusError{Status: resp.StatusCode, Message: message}
	}
	return io.ReadAll(io.LimitReader(resp.Body, maxJSONResponseBytes))
}

func (c *Client) endpointURL(endpoint string, query url.Values) string {
	u, _ := url.Parse(c.baseURL)
	u.Path = strings.TrimRight(u.Path, "/") + "/" + strings.TrimLeft(endpoint, "/")
	u.RawQuery = query.Encode()
	return u.String()
}

func (c *Client) get(ctx context.Context, endpoint string, query url.Values) (any, error) {
	raw, err := c.getBytes(ctx, endpoint, query)
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 {
		return nil, nil
	}
	var result any
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("decode GitHub response: %w", err)
	}
	return result, nil
}

func (c *Client) getBytes(ctx context.Context, endpoint string, query url.Values) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.endpointURL(endpoint, query), nil)
	if err != nil {
		return nil, fmt.Errorf("create GitHub request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request GitHub: %w", err)
	}
	defer resp.Body.Close()
	c.noteRateLimit(resp)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		detail, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorResponseBytes))
		message := strings.TrimSpace(string(detail))
		if message == "" {
			message = http.StatusText(resp.StatusCode)
		}
		return nil, &httpStatusError{Status: resp.StatusCode, Message: message}
	}
	return io.ReadAll(io.LimitReader(resp.Body, maxJSONResponseBytes))
}

const searchAccept = "application/vnd.github+json, application/vnd.github.text-match+json"

func (c *Client) Search(ctx context.Context, kind string, params url.Values) ([]any, int, error) {
	raw, err := c.getWithAccept(ctx, "/search/"+strings.TrimLeft(kind, "/"), params, searchAccept)
	if err != nil {
		return nil, 0, err
	}
	m, ok := raw.(map[string]any)
	if !ok {
		return nil, 0, fmt.Errorf("unexpected search response")
	}
	items, _ := m["items"].([]any)
	total := toInt(m["total_count"])
	return items, total, nil
}

func (c *Client) GetDefaultBranch(ctx context.Context, owner, repo string) (string, error) {
	raw, err := c.get(ctx, "/repos/"+owner+"/"+repo, nil)
	if err != nil {
		return "", err
	}
	m, ok := raw.(map[string]any)
	if !ok {
		return "", fmt.Errorf("unexpected repository response")
	}
	return toString(m["default_branch"]), nil
}

func (c *Client) GetTree(ctx context.Context, owner, repo, ref string, recursive bool) ([]any, error) {
	ref = strings.TrimSpace(ref)
	candidates := []string{}
	if ref != "" {
		candidates = append(candidates, ref)
	}
	if def, err := c.GetDefaultBranch(ctx, owner, repo); err == nil && def != "" {
		if ref == "" || def != ref {
			candidates = append(candidates, def)
		}
	}
	var lastErr error
	for _, r := range candidates {
		treeSHA, err := c.resolveTreeSHA(ctx, owner, repo, r)
		if err != nil {
			lastErr = err
			if !isNotFound(err) {
				return nil, err
			}
			continue
		}
		items, ferr := c.fetchTreeItems(ctx, owner, repo, treeSHA, recursive)
		if ferr != nil {
			lastErr = ferr
			if !isNotFound(ferr) {
				return nil, ferr
			}
			continue
		}
		return items, nil
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("não foi possível resolver a árvore para %s/%s", owner, repo)
}

func (c *Client) fetchTreeItems(ctx context.Context, owner, repo, treeSHA string, recursive bool) ([]any, error) {
	params := url.Values{}
	if recursive {
		params.Set("recursive", "1")
	}
	raw, err := c.get(ctx, "/repos/"+owner+"/"+repo+"/git/trees/"+treeSHA, params)
	if err != nil {
		return nil, err
	}
	m, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("unexpected tree response")
	}
	tree, _ := m["tree"].([]any)
	return tree, nil
}

func (c *Client) resolveTreeSHA(ctx context.Context, owner, repo, ref string) (string, error) {
	raw, err := c.get(ctx, "/repos/"+owner+"/"+repo+"/commits/"+ref, nil)
	if err != nil {
		return "", err
	}
	m, ok := raw.(map[string]any)
	if !ok {
		return "", fmt.Errorf("unexpected commit response for ref %q", ref)
	}
	if cm, ok := m["commit"].(map[string]any); ok {
		if tm, ok := cm["tree"].(map[string]any); ok {
			if sha := toStr(tm["sha"]); sha != "" {
				return sha, nil
			}
		}
	}
	return "", fmt.Errorf("could not resolve tree SHA for ref %q", ref)
}

func (c *Client) GetFile(ctx context.Context, owner, repo, path, ref string) (content string, truncated bool, err error) {
	ref = strings.TrimSpace(ref)
	content, truncated, err = c.getFileAtRef(ctx, owner, repo, path, ref)
	if err != nil && ref != "" && isNotFound(err) {
		def, derr := c.GetDefaultBranch(ctx, owner, repo)
		if derr == nil && def != "" && def != ref {
			return c.getFileAtRef(ctx, owner, repo, path, def)
		}
	}
	return content, truncated, err
}

func (c *Client) getFileAtRef(ctx context.Context, owner, repo, path, ref string) (content string, truncated bool, err error) {
	params := url.Values{}
	if strings.TrimSpace(ref) != "" {
		params.Set("ref", ref)
	}
	raw, err := c.getBytes(ctx, "/repos/"+owner+"/"+repo+"/contents/"+strings.TrimLeft(path, "/"), params)
	if err != nil {
		return "", false, err
	}

	var m map[string]any
	if json.Unmarshal(raw, &m) == nil {
		if enc, _ := m["encoding"].(string); enc == "base64" {
			if b64, _ := m["content"].(string); b64 != "" {
				b64 = strings.ReplaceAll(b64, "\n", "")
				decoded, derr := base64.StdEncoding.DecodeString(b64)
				if derr != nil {
					return "", false, fmt.Errorf("decode file content: %w", derr)
				}
				raw = decoded
			}
		}
	}

	if len(raw) > maxFileBytes {
		return string(raw[:maxFileBytes]), true, nil
	}
	return string(raw), false, nil
}

func (c *Client) ListReleases(ctx context.Context, owner, repo string, perPage, page int) ([]any, error) {
	params := url.Values{}
	params.Set("per_page", strconv.Itoa(clampPerPage(perPage)))
	if page > 0 {
		params.Set("page", strconv.Itoa(page))
	}
	raw, err := c.get(ctx, "/repos/"+owner+"/"+repo+"/releases", params)
	if err != nil {
		return nil, err
	}
	arr, ok := raw.([]any)
	if !ok {
		return nil, nil
	}
	return arr, nil
}

func (c *Client) GetLatestRelease(ctx context.Context, owner, repo string) (map[string]any, error) {
	raw, err := c.get(ctx, "/repos/"+owner+"/"+repo+"/releases/latest", nil)
	if err != nil {
		return nil, err
	}
	m, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("unexpected release response")
	}
	return m, nil
}

func (c *Client) ListCommits(ctx context.Context, owner, repo string, params url.Values) ([]any, error) {
	raw, err := c.get(ctx, "/repos/"+owner+"/"+repo+"/commits", params)
	if err != nil {
		return nil, err
	}
	arr, ok := raw.([]any)
	if !ok {
		return nil, nil
	}
	return arr, nil
}

func (c *Client) GetInsight(ctx context.Context, owner, repo, metric string) (any, error) {
	switch strings.ToLower(strings.TrimSpace(metric)) {
	case "contributors":
		return c.get(ctx, "/repos/"+owner+"/"+repo+"/contributors", url.Values{"per_page": []string{"100"}})
	case "commit_activity":
		return c.get(ctx, "/repos/"+owner+"/"+repo+"/stats/commit_activity", nil)
	case "code_frequency":
		return c.get(ctx, "/repos/"+owner+"/"+repo+"/stats/code_frequency", nil)
	case "participation":
		return c.get(ctx, "/repos/"+owner+"/"+repo+"/stats/participation", nil)
	case "punch_card":
		return c.get(ctx, "/repos/"+owner+"/"+repo+"/stats/punch_card", nil)
	default:
		return nil, fmt.Errorf("metric de insights inválida: %s (use contributors, commit_activity, code_frequency, participation ou punch_card)", metric)
	}
}

func toString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func toStr(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(t)
	case bool:
		if t {
			return "sim"
		}
		return "não"
	case float64:
		if t == float64(int64(t)) {
			return strconv.Itoa(int(t))
		}
		return strconv.FormatFloat(t, 'f', 2, 64)
	case int:
		return strconv.Itoa(t)
	case []any:
		parts := make([]string, 0, len(t))
		for _, x := range t {
			if s := toStr(x); s != "" {
				parts = append(parts, s)
			}
		}
		return strings.Join(parts, ", ")
	case map[string]any:
		for _, k := range []string{"name", "login", "title", "full_name", "message", "description", "path"} {
			if s, ok := t[k].(string); ok && s != "" {
				return strings.TrimSpace(s)
			}
		}
		return ""
	default:
		return fmt.Sprintf("%v", v)
	}
}

func toInt(v any) int {
	switch t := v.(type) {
	case float64:
		return int(t)
	case int:
		return t
	case string:
		if n, err := strconv.Atoi(strings.TrimSpace(t)); err == nil {
			return n
		}
		return 0
	default:
		return 0
	}
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
