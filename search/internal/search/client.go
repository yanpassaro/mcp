package search

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const defaultBaseURL = "https://api.exa.ai"

type Config struct {
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
}

type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

func NewClient(cfg Config) (*Client, error) {
	base := strings.TrimSpace(cfg.BaseURL)
	if base == "" {
		base = defaultBaseURL
	}
	key := strings.TrimSpace(cfg.APIKey)
	if key == "" {
		return nil, errors.New("EXA_API_KEY is required")
	}
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	return &Client{
		baseURL:    strings.TrimRight(base, "/"),
		apiKey:     key,
		httpClient: httpClient,
	}, nil
}

type TextConfig struct {
	MaxCharacters   int      `json:"maxCharacters,omitempty"`
	IncludeHtmlTags bool     `json:"includeHtmlTags,omitempty"`
	Verbosity       string   `json:"verbosity,omitempty"`
	IncludeSections []string `json:"includeSections,omitempty"`
	ExcludeSections []string `json:"excludeSections,omitempty"`
}

type HighlightsConfig struct {
	Query         string `json:"query,omitempty"`
	MaxCharacters int    `json:"maxCharacters,omitempty"`
}

type SummaryConfig struct {
	Query  string `json:"query,omitempty"`
	Schema any    `json:"schema,omitempty"`
}

type ExtrasConfig struct {
	Links      int `json:"links,omitempty"`
	ImageLinks int `json:"imageLinks,omitempty"`
}

type contentsConfig struct {
	Text             any           `json:"text,omitempty"`
	Highlights       any           `json:"highlights,omitempty"`
	Summary          any           `json:"summary,omitempty"`
	MaxAgeHours      *int          `json:"maxAgeHours,omitempty"`
	LivecrawlTimeout int           `json:"livecrawlTimeout,omitempty"`
	Subpages         int           `json:"subpages,omitempty"`
	SubpageTarget    any           `json:"subpageTarget,omitempty"`
	Extras           *ExtrasConfig `json:"extras,omitempty"`
}

type SearchParams struct {
	Query              string
	Type               string
	NumResults         int
	Category           string
	UserLocation       string
	IncludeDomains     []string
	ExcludeDomains     []string
	StartPublishedDate string
	EndPublishedDate   string
	Moderation         bool
	SystemPrompt       string
	AdditionalQueries  []string

	Text                bool
	TextMaxCharacters   int
	TextVerbosity       string
	TextIncludeSections []string
	TextExcludeSections []string
	TextIncludeHtmlTags bool

	Highlights              bool
	HighlightsQuery         string
	HighlightsMaxCharacters int

	Summary       bool
	SummaryQuery  string
	SummarySchema any

	MaxAgeHours      *int
	LivecrawlTimeout int
	Subpages         int
	SubpageTarget    []string
	ExtrasLinks      int
	ExtrasImageLinks int
}

const (
	defaultNumResults = 10
	defaultType       = "auto"
)

type searchRequest struct {
	Query              string         `json:"query"`
	Type               string         `json:"type,omitempty"`
	NumResults         int            `json:"numResults,omitempty"`
	Category           string         `json:"category,omitempty"`
	UserLocation       string         `json:"userLocation,omitempty"`
	IncludeDomains     []string       `json:"includeDomains,omitempty"`
	ExcludeDomains     []string       `json:"excludeDomains,omitempty"`
	StartPublishedDate string         `json:"startPublishedDate,omitempty"`
	EndPublishedDate   string         `json:"endPublishedDate,omitempty"`
	Moderation         bool           `json:"moderation,omitempty"`
	SystemPrompt       string         `json:"systemPrompt,omitempty"`
	AdditionalQueries  []string       `json:"additionalQueries,omitempty"`
	Contents           contentsConfig `json:"contents"`
}

type SearchResult struct {
	ID              string    `json:"id"`
	Title           string    `json:"title"`
	URL             string    `json:"url"`
	PublishedDate   string    `json:"publishedDate"`
	Author          string    `json:"author"`
	Image           string    `json:"image"`
	Favicon         string    `json:"favicon"`
	Text            string    `json:"text"`
	Highlights      []string  `json:"highlights"`
	HighlightScores []float64 `json:"highlightScores"`
	Summary         string    `json:"summary"`
	Extras          struct {
		Links []string `json:"links"`
	} `json:"extras"`
	Score float64 `json:"score"`
}

type CostDollars struct {
	Total float64 `json:"total"`
}

type SearchResponse struct {
	RequestID   string         `json:"requestId"`
	SearchType  string         `json:"searchType"`
	Results     []SearchResult `json:"results"`
	CostDollars CostDollars    `json:"costDollars"`
}

func (c *Client) Search(ctx context.Context, p SearchParams) (*SearchResponse, error) {
	query := strings.TrimSpace(p.Query)
	if query == "" {
		return nil, errors.New("query is required")
	}
	numResults := p.NumResults
	if numResults <= 0 {
		numResults = defaultNumResults
	}
	searchType := strings.TrimSpace(p.Type)
	if searchType == "" {
		searchType = defaultType
	}

	contents := buildContents(p)

	reqBody := searchRequest{
		Query:              query,
		Type:               searchType,
		NumResults:         numResults,
		Category:           strings.TrimSpace(p.Category),
		UserLocation:       strings.TrimSpace(p.UserLocation),
		IncludeDomains:     p.IncludeDomains,
		ExcludeDomains:     p.ExcludeDomains,
		StartPublishedDate: strings.TrimSpace(p.StartPublishedDate),
		EndPublishedDate:   strings.TrimSpace(p.EndPublishedDate),
		Moderation:         p.Moderation,
		SystemPrompt:       strings.TrimSpace(p.SystemPrompt),
		AdditionalQueries:  p.AdditionalQueries,
		Contents:           contents,
	}
	payload, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to encode request: %w", err)
	}

	endpoint := c.baseURL + "/search"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to query Exa: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 8*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("failed to read Exa response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("Exa returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}

	var parsed SearchResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("failed to decode Exa response: %w", err)
	}
	return &parsed, nil
}

func buildContents(p SearchParams) contentsConfig {
	var c contentsConfig

	if p.Text {
		tc := &TextConfig{}
		if p.TextMaxCharacters > 0 {
			tc.MaxCharacters = p.TextMaxCharacters
		}
		if p.TextIncludeHtmlTags {
			tc.IncludeHtmlTags = true
		}
		if v := strings.TrimSpace(p.TextVerbosity); v != "" {
			tc.Verbosity = v
		}
		if len(p.TextIncludeSections) > 0 {
			tc.IncludeSections = p.TextIncludeSections
		}
		if len(p.TextExcludeSections) > 0 {
			tc.ExcludeSections = p.TextExcludeSections
		}
		c.Text = tc
	}

	if p.Highlights {
		if p.HighlightsQuery != "" || p.HighlightsMaxCharacters > 0 {
			c.Highlights = &HighlightsConfig{
				Query:         p.HighlightsQuery,
				MaxCharacters: p.HighlightsMaxCharacters,
			}
		} else {
			c.Highlights = true
		}
	}

	if p.Summary {
		c.Summary = &SummaryConfig{
			Query:  p.SummaryQuery,
			Schema: p.SummarySchema,
		}
	}

	if !p.Text && !p.Highlights && !p.Summary {
		c.Highlights = true
	}

	if p.MaxAgeHours != nil {
		c.MaxAgeHours = p.MaxAgeHours
	}
	if p.LivecrawlTimeout > 0 {
		c.LivecrawlTimeout = p.LivecrawlTimeout
	}
	if p.Subpages > 0 {
		c.Subpages = p.Subpages
	}
	if len(p.SubpageTarget) > 0 {
		c.SubpageTarget = p.SubpageTarget
	}
	if p.ExtrasLinks > 0 || p.ExtrasImageLinks > 0 {
		c.Extras = &ExtrasConfig{Links: p.ExtrasLinks, ImageLinks: p.ExtrasImageLinks}
	}
	return c
}

type FetchParams struct {
	URLs []string
	IDs  []string

	Text                bool
	TextMaxCharacters   int
	TextVerbosity       string
	TextIncludeSections []string
	TextExcludeSections []string
	TextIncludeHtmlTags bool

	Highlights              bool
	HighlightsQuery         string
	HighlightsMaxCharacters int

	Summary       bool
	SummaryQuery  string
	SummarySchema any

	MaxAgeHours      *int
	LivecrawlTimeout int
	Subpages         int
	SubpageTarget    []string
	ExtrasLinks      int
	ExtrasImageLinks int
}

type contentsRequest struct {
	URLs             []string      `json:"urls,omitempty"`
	IDs              []string      `json:"ids,omitempty"`
	Text             any           `json:"text,omitempty"`
	Highlights       any           `json:"highlights,omitempty"`
	Summary          any           `json:"summary,omitempty"`
	MaxAgeHours      *int          `json:"maxAgeHours,omitempty"`
	LivecrawlTimeout int           `json:"livecrawlTimeout,omitempty"`
	Subpages         int           `json:"subpages,omitempty"`
	SubpageTarget    any           `json:"subpageTarget,omitempty"`
	Extras           *ExtrasConfig `json:"extras,omitempty"`
}

type ResultStatus struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Error  *struct {
		Tag            string `json:"tag"`
		HTTPStatusCode *int   `json:"httpStatusCode"`
	} `json:"error"`
}

type ContentsResponse struct {
	RequestID   string         `json:"requestId"`
	Results     []SearchResult `json:"results"`
	Statuses    []ResultStatus `json:"statuses"`
	CostDollars CostDollars    `json:"costDollars"`
}

func (c *Client) Fetch(ctx context.Context, p FetchParams) (*ContentsResponse, error) {
	if len(p.URLs) == 0 && len(p.IDs) == 0 {
		return nil, errors.New("at least one url or id is required")
	}

	req := contentsRequest{
		URLs: p.URLs,
		IDs:  p.IDs,
	}

	if p.Text {
		tc := &TextConfig{}
		if p.TextMaxCharacters > 0 {
			tc.MaxCharacters = p.TextMaxCharacters
		}
		if p.TextIncludeHtmlTags {
			tc.IncludeHtmlTags = true
		}
		if v := strings.TrimSpace(p.TextVerbosity); v != "" {
			tc.Verbosity = v
		}
		if len(p.TextIncludeSections) > 0 {
			tc.IncludeSections = p.TextIncludeSections
		}
		if len(p.TextExcludeSections) > 0 {
			tc.ExcludeSections = p.TextExcludeSections
		}
		req.Text = tc
	}

	if p.Highlights {
		if p.HighlightsQuery != "" || p.HighlightsMaxCharacters > 0 {
			req.Highlights = &HighlightsConfig{
				Query:         p.HighlightsQuery,
				MaxCharacters: p.HighlightsMaxCharacters,
			}
		} else {
			req.Highlights = true
		}
	}

	if p.Summary {
		req.Summary = &SummaryConfig{
			Query:  p.SummaryQuery,
			Schema: p.SummarySchema,
		}
	}

	if !p.Text && !p.Highlights && !p.Summary {
		req.Text = &TextConfig{MaxCharacters: 20000}
	}

	if p.MaxAgeHours != nil {
		req.MaxAgeHours = p.MaxAgeHours
	}
	if p.LivecrawlTimeout > 0 {
		req.LivecrawlTimeout = p.LivecrawlTimeout
	}
	if p.Subpages > 0 {
		req.Subpages = p.Subpages
	}
	if len(p.SubpageTarget) > 0 {
		req.SubpageTarget = p.SubpageTarget
	}
	if p.ExtrasLinks > 0 || p.ExtrasImageLinks > 0 {
		req.Extras = &ExtrasConfig{Links: p.ExtrasLinks, ImageLinks: p.ExtrasImageLinks}
	}

	payload, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to encode request: %w", err)
	}

	endpoint := c.baseURL + "/contents"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", c.apiKey)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to query Exa: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 32*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("failed to read Exa response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("Exa returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}

	var parsed ContentsResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("failed to decode Exa response: %w", err)
	}
	return &parsed, nil
}
