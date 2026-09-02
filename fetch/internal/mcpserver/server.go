package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"ntdsk.com/mcp/fetch/internal/fetch"
)

type Server struct {
	client *fetch.Client
	log    *log.Logger // sem prefixo, para o histórico (linhas iniciam em "## Fetch")
}

func New(client *fetch.Client, logFile *os.File) *Server {
	w := io.Writer(io.Discard)
	if logFile != nil {
		w = logFile
	}
	return &Server{client: client, log: log.New(w, "", 0)}
}

func (s *Server) Register(server *mcp.Server) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "fetch_request",
		Description: "Send an HTTP request to an allowlisted host and return status, timing breakdown (DNS/connect/TLS/first byte), headers and body as Markdown, plus an equivalent curl command to reproduce it. The host must be listed in FETCH_ALLOW_HOST. JSON and XML bodies are supported (Content-Type is inferred unless headers override it); JSON/XML responses are pretty-printed, binary responses are summarized instead of dumped. Cookies from Set-Cookie are saved and sent back automatically. Set noBody to skip the response body.",
	}, s.requestTool)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "fetch_cookie",
		Description: "Manage session cookies: 'action'='list' shows the saved cookies (domain, name, value, path, expiry, flags); 'action'='clear' removes them (all, per domain, or a single cookie by domain+name) and returns how many were removed.",
	}, s.cookieTool)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "fetch_history",
		Description: "Return the last N fetch requests from the log files. Useful to reproduce recent calls without retyping them.",
	}, s.historyTool)
}


type requestInput struct {
	Method           string            `json:"method,omitempty" jsonschema:"HTTP method: GET, POST, PUT, PATCH, DELETE, HEAD, OPTIONS (default GET)."`
	URL              string            `json:"url" jsonschema:"Full URL with scheme and host, e.g. http://localhost:8080/api/items. The host must be in the FETCH_ALLOW_HOST allowlist."`
	Query            map[string]string `json:"query,omitempty" jsonschema:"Query parameters to append to the URL."`
	Headers          map[string]string `json:"headers,omitempty" jsonschema:"Extra request headers (optional)."`
	Body             string            `json:"body,omitempty" jsonschema:"Request body: JSON, XML or plain text. Content-Type is inferred from the body ({, [, <) unless you set one in headers."`
	BasicAuth        string            `json:"basicAuth,omitempty" jsonschema:"Basic auth as user:pass."`
	BearerToken      string            `json:"bearerToken,omitempty" jsonschema:"Bearer token for Authorization header."`
	Timeout          int               `json:"timeout,omitempty" jsonschema:"Timeout in seconds (default 30)."`
	NoCookies       bool  `json:"noCookies,omitempty" jsonschema:"If true, saved cookies are not attached to this request."`
	FollowRedirects *bool `json:"followRedirects,omitempty" jsonschema:"If true, follows redirects (default true)."`
	MaxRedirects     int   `json:"maxRedirects,omitempty" jsonschema:"Maximum number of redirects to follow (default 10)."`
	NoBody          bool  `json:"noBody,omitempty" jsonschema:"If true, the response body is not read or shown (only status, headers and timing)."`
	Summary         bool  `json:"summary,omitempty" jsonschema:"If true, return a short summary with status, timing, content-type, size and curl only."`
	HTMLRaw         bool  `json:"htmlRaw,omitempty" jsonschema:"If true, HTML responses are shown as raw markup instead of extracted text."`
	HTMLMaxChars    int   `json:"htmlMaxChars,omitempty" jsonschema:"Max characters of extracted HTML text (default 1200). Only applies when the response is HTML and htmlRaw is false."`
	BodyMaxChars    int   `json:"bodyMaxChars,omitempty" jsonschema:"Max characters of the response body shown (default 1200). Larger bodies are truncated."`
}

func (s *Server) requestTool(ctx context.Context, _ *mcp.CallToolRequest, in requestInput) (*mcp.CallToolResult, any, error) {
	if strings.TrimSpace(in.URL) == "" {
		return nil, nil, errors.New("'url' é obrigatório")
	}
	follow := in.FollowRedirects == nil || *in.FollowRedirects
	resp, err := s.client.Do(ctx, fetch.Req{
		Method:          in.Method,
		URL:             in.URL,
		Query:           in.Query,
		Body:            in.Body,
		Headers:         in.Headers,
		BasicAuth:       in.BasicAuth,
		BearerToken:     in.BearerToken,
		NoCookies:       in.NoCookies,
		FollowRedirects: follow,
		MaxRedirects:    in.MaxRedirects,
		NoBody:          in.NoBody,
		Timeout:         time.Duration(in.Timeout) * time.Second,
	})
	if err != nil {
		return nil, nil, err
	}
	var md string
	bodyMaxChars := in.BodyMaxChars
	if bodyMaxChars <= 0 {
		bodyMaxChars = defaultBodyMaxChars
	}
	if in.Summary {
		md = formatSummary(resp)
	} else {
		md = formatResponse(resp, s.client.MaxBody(), in.HTMLRaw, in.HTMLMaxChars, bodyMaxChars)
	}
	if md != "" {
		s.log.Println(md)
	}
	return textResult(md)
}

func (s *Server) cookieTool(ctx context.Context, _ *mcp.CallToolRequest, in cookieInput) (*mcp.CallToolResult, any, error) {
	switch strings.ToLower(strings.TrimSpace(in.Action)) {
	case "clear":
		n := s.client.ClearCookies(in.Domain, in.Name)
		return textResult(clearMessage(n, in.Domain, in.Name))
	default:
		return textResult(formatCookies(s.client.Cookies()))
	}
}

type historyInput struct {
	Limit int `json:"limit,omitempty" jsonschema:"Number of recent entries to return (default 20, max 200)."`
}

func (s *Server) historyTool(ctx context.Context, _ *mcp.CallToolRequest, in historyInput) (*mcp.CallToolResult, any, error) {
	limit := in.Limit
	if limit <= 0 || limit > 200 {
		limit = 20
	}
	home, err := os.UserHomeDir()
	if err != nil {
		home = os.Getenv("USERPROFILE")
	}
	logDir := filepath.Join(home, ".local", "share", "mcp", "fetch", "logs")
	files, err := os.ReadDir(logDir)
	if err != nil {
		return textResult(fmt.Sprintf("## Fetch history\n\nLog directory not found: %s\n", logDir))
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Name() < files[j].Name() })
	var entries []string
	buf := ""
	for i := len(files) - 1; i >= 0 && len(entries) < limit; i-- {
		f := files[i]
		if f.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(logDir, f.Name()))
		if err != nil {
			continue
		}
		for _, ln := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(ln, "## Fetch") {
				if strings.TrimSpace(buf) != "" {
					entries = append(entries, strings.TrimSpace(buf))
					if len(entries) >= limit {
						break
					}
				}
				buf = ln
			} else if buf != "" {
				buf += "\n" + ln
			}
		}
		if len(entries) < limit && strings.TrimSpace(buf) != "" {
			entries = append(entries, strings.TrimSpace(buf))
			buf = ""
		}
	}
	var b strings.Builder
	fmt.Fprintf(&b, "## Fetch history (last %d entries)\n\n", limit)
	if len(entries) == 0 {
		b.WriteString("_Nenhum registro encontrado nos logs._\n")
	} else {
		for i, e := range entries {
			fmt.Fprintf(&b, "### %d\n\n%s\n\n---\n\n", i+1, e)
		}
	}
	return textResult(strings.TrimSpace(b.String()))
}

type cookieInput struct {
	Action string `json:"action,omitempty" jsonschema:"list (default) or clear."`
	Domain string `json:"domain,omitempty" jsonschema:"For clear: domain to clear (e.g. localhost). Empty clears all domains."`
	Name   string `json:"name,omitempty" jsonschema:"For clear: cookie name. Empty clears the whole domain (or everything when domain is also empty)."`
}

func textResult(text string) (*mcp.CallToolResult, any, error) {
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: text}},
	}, nil, nil
}
