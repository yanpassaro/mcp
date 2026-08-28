package vision

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	defaultBaseURL   = "https://api.openai.com/v1"
	defaultModel     = "gpt-4o"
	defaultMaxTokens = 1500
	maxImageBytes    = 25 * 1024 * 1024
	MaxVideoBytes    = 8 * 1024 * 1024
)

type Config struct {
	BaseURL       string
	APIKey        string
	Model         string
	HTTPClient    *http.Client
	MaxImageBytes int64
	MaxTokens     int
}

type Client struct {
	baseURL       string
	apiKey        string
	model         string
	httpClient    *http.Client
	maxImageBytes int64
	maxTokens     int
}

type MediaInput struct {
	Kind string
	MIME string
	Data []byte
}

func NewClient(cfg Config) (*Client, error) {
	base := strings.TrimSpace(cfg.BaseURL)
	if base == "" {
		base = defaultBaseURL
	}
	model := strings.TrimSpace(cfg.Model)
	if model == "" {
		model = defaultModel
	}
	maxImg := cfg.MaxImageBytes
	if maxImg <= 0 {
		maxImg = maxImageBytes
	}
	maxTokens := cfg.MaxTokens
	if maxTokens <= 0 {
		maxTokens = defaultMaxTokens
	}
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 120 * time.Second}
	}
	return &Client{
		baseURL:       strings.TrimRight(base, "/"),
		apiKey:        cfg.APIKey,
		model:         model,
		httpClient:    httpClient,
		maxImageBytes: maxImg,
		maxTokens:     maxTokens,
	}, nil
}

func (c *Client) MaxImageBytes() int64 {
	return c.maxImageBytes
}

const visionSystemPrompt = "You are a precise vision assistant. Always respond in the same language as the user's prompt. Be specific, concise and well-structured."

type textPart struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type imageURLPart struct {
	Type     string        `json:"type"`
	ImageURL imageURLValue `json:"image_url"`
}

type videoPart struct {
	Type  string        `json:"type"`
	Video imageURLValue `json:"video"`
}

type imageURLValue struct {
	URL string `json:"url"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}

type chatRequest struct {
	Model     string        `json:"model"`
	Messages  []chatMessage `json:"messages"`
	MaxTokens int           `json:"max_tokens"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (c *Client) Complete(ctx context.Context, prompt string, media []MediaInput) (string, error) {
	if strings.TrimSpace(prompt) == "" {
		return "", errors.New("empty prompt")
	}
	content := []any{textPart{Type: "text", Text: strings.TrimSpace(prompt)}}
	for _, m := range media {
		dataURL := "data:" + m.MIME + ";base64," + base64.StdEncoding.EncodeToString(m.Data)
		switch m.Kind {
		case "video":
			content = append(content, videoPart{Type: "video", Video: imageURLValue{URL: dataURL}})
		default:
			content = append(content, imageURLPart{Type: "image_url", ImageURL: imageURLValue{URL: dataURL}})
		}
	}
	reqBody := chatRequest{
		Model: c.model,
		Messages: []chatMessage{
			{Role: "system", Content: visionSystemPrompt},
			{Role: "user", Content: content},
		},
		MaxTokens: c.maxTokens,
	}
	payload, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to encode request: %w", err)
	}

	endpoint := c.baseURL + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if strings.TrimSpace(c.apiKey) != "" {
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(c.apiKey))
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to call vision model: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 16*1024*1024))
	if err != nil {
		return "", fmt.Errorf("failed to read model response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("model returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}

	var parsed chatResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", fmt.Errorf("failed to decode model response: %w", err)
	}
	if parsed.Error != nil && parsed.Error.Message != "" {
		return "", errors.New("model returned an error: " + parsed.Error.Message)
	}
	if len(parsed.Choices) == 0 {
		return "", errors.New("model returned no content")
	}
	out := strings.TrimSpace(parsed.Choices[0].Message.Content)
	if out == "" {
		return "", errors.New("model returned empty content")
	}
	return out, nil
}
