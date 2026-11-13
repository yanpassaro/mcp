package docker

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/moby/moby/client"
)

const DEFAULT_TIMEOUT_SECONDS = 60

type Config struct {
	Timeout      time.Duration
	RegistryAuth string
}

type Client struct {
	api          *client.Client
	timeout      time.Duration
	registryAuth string
}

func NewClient(cfg Config) (*Client, error) {
	api, err := newAPI()
	if err != nil {
		return nil, fmt.Errorf("create docker client: %w", err)
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = DEFAULT_TIMEOUT_SECONDS * time.Second
	}
	return &Client{api: api, timeout: timeout, registryAuth: cfg.RegistryAuth}, nil
}

func newAPI() (*client.Client, error) {
	host := strings.TrimSpace(os.Getenv("DOCKER_HOST"))
	if isSSHHost(host) {
		return newSSHClient(host)
	}
	return client.New(client.FromEnv)
}

func (c *Client) API() *client.Client {
	return c.api
}

func (c *Client) WithTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	return c.WithTimeoutSeconds(ctx, 0)
}

func (c *Client) WithTimeoutSeconds(ctx context.Context, seconds int) (context.Context, context.CancelFunc) {
	timeout := c.timeout
	if seconds > 0 {
		timeout = time.Duration(seconds) * time.Second
	}
	return context.WithTimeout(ctx, timeout)
}

func (c *Client) RegistryAuth() string {
	return c.registryAuth
}

func (c *Client) Close() error {
	return c.api.Close()
}
