package mcpserver

import (
	"context"
	"errors"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"ntdsk.com/mcp/search/internal/search"
)

type ContentConfig struct {
	Text                bool
	Highlights          bool
	Summary             bool
	TextMaxChars        int
	HighlightsMaxChars  int
	TextIncludeHtmlTags bool
	ExtrasLinks         int
	ExtrasImageLinks    int
	MaxAgeHours         *int
	LivecrawlTimeout    int
	Subpages            int
	SubpageTarget       []string
}

type Config struct {
	NumResults int
	SearchType string
	Search     ContentConfig
	Fetch      ContentConfig
}

type Server struct {
	client     *search.Client
	numResults int
	searchType string
	searchCfg  ContentConfig
	fetchCfg   ContentConfig
}

func New(client *search.Client, cfg Config) *Server {
	if cfg.NumResults < 1 {
		cfg.NumResults = 10
	}
	if cfg.SearchType == "" {
		cfg.SearchType = "auto"
	}
	return &Server{
		client:     client,
		numResults: cfg.NumResults,
		searchType: cfg.SearchType,
		searchCfg:  cfg.Search,
		fetchCfg:   cfg.Fetch,
	}
}

func DefaultSearchContent() ContentConfig {
	return ContentConfig{
		Text:                false,
		Highlights:          true,
		Summary:             false,
		TextMaxChars:        0,
		HighlightsMaxChars:  0,
		TextIncludeHtmlTags: false,
		ExtrasLinks:         0,
		ExtrasImageLinks:    0,
		MaxAgeHours:         nil,
		LivecrawlTimeout:    0,
		Subpages:            0,
		SubpageTarget:       nil,
	}
}

func DefaultFetchContent() ContentConfig {
	return ContentConfig{
		Text:                true,
		Highlights:          false,
		Summary:             false,
		TextMaxChars:        20000,
		HighlightsMaxChars:  0,
		TextIncludeHtmlTags: false,
		ExtrasLinks:         0,
		ExtrasImageLinks:    0,
		MaxAgeHours:         nil,
		LivecrawlTimeout:    0,
		Subpages:            0,
		SubpageTarget:       nil,
	}
}

func (s *Server) Register(server *mcp.Server) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "exa_search",
		Description: "Web search via the Exa Search API, returning highlights formatted in Markdown. The agent may only set: query, category, numResults, and the highlights character cap (maxCharacters). Search type, content formats and all advanced options are fixed by the server operator.",
	}, s.search)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "exa_fetch",
		Description: "Fetch the full contents of one or more web pages (by URL or Exa document ID) using the Exa Contents API, returning the extracted text formatted in Markdown. The agent may set: urls (or ids) and the text character cap (maxCharacters). Content format and all advanced options are fixed by the server operator and cannot be changed.",
	}, s.fetch)
}

type searchInput struct {
	Query         string `json:"query" jsonschema:"Natural language search query (supports long, semantically rich descriptions)."`
	Category      string `json:"category" jsonschema:"Content type. Allowed values: company, people, publication, news, personal site, financial report. Note: company/people disable some filters."`
	NumResults    int    `json:"numResults" jsonschema:"Number of results to return (1-100). Defaults to 10."`
	MaxCharacters int    `json:"maxCharacters" jsonschema:"Max characters for the returned highlights (upper bound 2000)."`
}

const maxHighlightChars = 2000

func (s *Server) search(ctx context.Context, _ *mcp.CallToolRequest, in searchInput) (*mcp.CallToolResult, any, error) {
	if strings.TrimSpace(in.Query) == "" {
		return nil, nil, errors.New("query is required")
	}
	hlMax := in.MaxCharacters
	if hlMax > maxHighlightChars {
		hlMax = maxHighlightChars
	}
	if hlMax < 0 {
		hlMax = 0
	}
	p := search.SearchParams{
		Query:                   in.Query,
		Type:                    s.searchType,
		NumResults:              orDefaultInt(in.NumResults, s.numResults),
		Category:                in.Category,
		Text:                    s.searchCfg.Text,
		TextMaxCharacters:       s.searchCfg.TextMaxChars,
		TextIncludeHtmlTags:     s.searchCfg.TextIncludeHtmlTags,
		Highlights:              s.searchCfg.Highlights,
		HighlightsMaxCharacters: hlMax,
		Summary:                 s.searchCfg.Summary,
		ExtrasLinks:             s.searchCfg.ExtrasLinks,
		ExtrasImageLinks:        s.searchCfg.ExtrasImageLinks,
		MaxAgeHours:             s.searchCfg.MaxAgeHours,
		LivecrawlTimeout:        s.searchCfg.LivecrawlTimeout,
		Subpages:                s.searchCfg.Subpages,
		SubpageTarget:           s.searchCfg.SubpageTarget,
	}
	resp, err := s.client.Search(ctx, p)
	if err != nil {
		return nil, nil, err
	}
	return textResult(formatSearch(*resp))
}

type fetchInput struct {
	URLs          []string `json:"urls" jsonschema:"One or more URLs to fetch the full contents of (required unless ids is provided)."`
	IDs           []string `json:"ids" jsonschema:"One or more Exa document IDs to fetch (interchangeable with urls)."`
	MaxCharacters int      `json:"maxCharacters" jsonschema:"Max characters for the returned text (upper bound 20000)."`
}

const maxFetchChars = 20000

func (s *Server) fetch(ctx context.Context, _ *mcp.CallToolRequest, in fetchInput) (*mcp.CallToolResult, any, error) {
	if len(in.URLs) == 0 && len(in.IDs) == 0 {
		return nil, nil, errors.New("at least one url or id is required")
	}
	maxChars := in.MaxCharacters
	if maxChars > maxFetchChars {
		maxChars = maxFetchChars
	}
	if maxChars < 0 {
		maxChars = 0
	}
	if maxChars == 0 {
		maxChars = s.fetchCfg.TextMaxChars
	}
	p := search.FetchParams{
		URLs:                in.URLs,
		IDs:                 in.IDs,
		Text:                s.fetchCfg.Text,
		TextMaxCharacters:   maxChars,
		TextIncludeHtmlTags: s.fetchCfg.TextIncludeHtmlTags,
		Highlights:          s.fetchCfg.Highlights,
		Summary:             s.fetchCfg.Summary,
		ExtrasLinks:         s.fetchCfg.ExtrasLinks,
		ExtrasImageLinks:    s.fetchCfg.ExtrasImageLinks,
		MaxAgeHours:         s.fetchCfg.MaxAgeHours,
		LivecrawlTimeout:    s.fetchCfg.LivecrawlTimeout,
		Subpages:            s.fetchCfg.Subpages,
		SubpageTarget:       s.fetchCfg.SubpageTarget,
	}
	resp, err := s.client.Fetch(ctx, p)
	if err != nil {
		return nil, nil, err
	}
	return textResult(formatContents(*resp))
}

func textResult(text string) (*mcp.CallToolResult, any, error) {
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: text}},
	}, nil, nil
}

func orDefaultInt(v, fallback int) int {
	if v <= 0 {
		return fallback
	}
	return v
}
