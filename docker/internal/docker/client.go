package docker

import (
	"context"
	"fmt"
	"time"

	"github.com/moby/moby/client"
)

const DEFAULT_TIMEOUT_SECONDS = 60

type Config struct {
	Timeout time.Duration
}

type Client struct {
	api     *client.Client
	timeout time.Duration
}

func NewClient(cfg Config) (*Client, error) {
	api, err := client.New(client.FromEnv)
	if err != nil {
		return nil, fmt.Errorf("create docker client: %w", err)
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = DEFAULT_TIMEOUT_SECONDS * time.Second
	}
	return &Client{api: api, timeout: timeout}, nil
}

func (c *Client) API() *client.Client {
	return c.api
}

func (c *Client) WithTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, c.timeout)
}

func (c *Client) Close() error {
	return c.api.Close()
}
