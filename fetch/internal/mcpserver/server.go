package mcpserver

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"ntdsk.com/mcp/fetch/internal/fetch"
)

type Server struct {
	client *fetch.Client
}

func New(client *fetch.Client) *Server {
	return &Server{client: client}
}

func (s *Server) Register(server *mcp.Server) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "fetch_request",
		Description: "Send an HTTP request to an allowlisted host and return status, timing breakdown (DNS/connect/TLS/first byte), headers and body as Markdown, plus an equivalent curl command to reproduce it. The host must be listed in FETCH_ALLOW_HOST. JSON and XML bodies are supported (Content-Type is inferred unless headers override it); JSON/XML responses are pretty-printed, binary responses are summarized instead of dumped. Cookies from Set-Cookie are saved and sent back automatically. Set noBody to skip the response body.",
	}, s.requestTool)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "cookie_list",
		Description: "List the cookies saved for the current fetch session (domain, name, value, path, expiry and flags). Cookies are saved from Set-Cookie responses and persisted between restarts.",
	}, s.cookieListTool)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "cookie_clear",
		Description: "Clear saved cookies: everything (no args), all cookies of one domain, or a single cookie (domain + name). Returns how many were removed.",
	}, s.cookieClearTool)
}

type emptyInput struct{}

type requestInput struct {
	Method           string            `json:"method,omitempty" jsonschema:"HTTP method: GET, POST, PUT, PATCH, DELETE, HEAD, OPTIONS (default GET)."`
	URL              string            `json:"url" jsonschema:"Full URL with scheme and host, e.g. http://localhost:8080/api/items. The host must be in the FETCH_ALLOW_HOST allowlist."`
	Headers          map[string]string `json:"headers,omitempty" jsonschema:"Extra request headers (optional)."`
	Body             string            `json:"body,omitempty" jsonschema:"Request body: JSON, XML or plain text. Content-Type is inferred from the body ({, [, <) unless you set one in headers."`
	Timeout          int               `json:"timeout,omitempty" jsonschema:"Timeout in seconds (default 30)."`
	NoCookies       bool  `json:"noCookies,omitempty" jsonschema:"If true, saved cookies are not attached to this request."`
	FollowRedirects *bool `json:"followRedirects,omitempty" jsonschema:"If true, follows redirects (default true)."`
	NoBody          bool  `json:"noBody,omitempty" jsonschema:"If true, the response body is not read or shown (only status, headers and timing)."`
	HTMLRaw         bool  `json:"htmlRaw,omitempty" jsonschema:"If true, HTML responses are shown as raw markup instead of extracted text."`
	HTMLMaxChars    int   `json:"htmlMaxChars,omitempty" jsonschema:"Max characters of extracted HTML text (default 1200). Only applies when the response is HTML and htmlRaw is false."`
}

func (s *Server) requestTool(ctx context.Context, _ *mcp.CallToolRequest, in requestInput) (*mcp.CallToolResult, any, error) {
	if strings.TrimSpace(in.URL) == "" {
		return nil, nil, errors.New("'url' é obrigatório")
	}
	follow := in.FollowRedirects == nil || *in.FollowRedirects
	resp, err := s.client.Do(ctx, fetch.Req{
		Method:          in.Method,
		URL:             in.URL,
		Body:            in.Body,
		Headers:         in.Headers,
		NoCookies:       in.NoCookies,
		FollowRedirects: follow,
		NoBody:          in.NoBody,
		Timeout:         time.Duration(in.Timeout) * time.Second,
	})
	if err != nil {
		return nil, nil, err
	}
	return textResult(formatResponse(resp, s.client.MaxBody(), in.HTMLRaw, in.HTMLMaxChars))
}

func (s *Server) cookieListTool(ctx context.Context, _ *mcp.CallToolRequest, _ emptyInput) (*mcp.CallToolResult, any, error) {
	return textResult(formatCookies(s.client.Cookies()))
}

func (s *Server) cookieClearTool(ctx context.Context, _ *mcp.CallToolRequest, in cookieClearInput) (*mcp.CallToolResult, any, error) {
	n := s.client.ClearCookies(in.Domain, in.Name)
	return textResult(clearMessage(n, in.Domain, in.Name))
}

type cookieClearInput struct {
	Domain string `json:"domain,omitempty" jsonschema:"Domain to clear (e.g. localhost). Empty clears all domains."`
	Name   string `json:"name,omitempty" jsonschema:"Cookie name to clear. Empty clears the whole domain (or everything when domain is also empty)."`
}

func textResult(text string) (*mcp.CallToolResult, any, error) {
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: text}},
	}, nil, nil
}
