package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"ntdsk.com/mcp/github/internal/github"
	"ntdsk.com/mcp/github/internal/mcpserver"
)

func main() {
	setupLog("github")

	token := strings.TrimSpace(os.Getenv("GITHUB_TOKEN"))
	baseURL := os.Getenv("GITHUB_BASE_URL")

	timeout := 60
	if v := strings.TrimSpace(os.Getenv("GITHUB_TIMEOUT_SECONDS")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			timeout = n
		}
	}

	client, err := github.NewClient(github.Config{
		BaseURL:    baseURL,
		Token:      token,
		HTTPClient: &http.Client{Timeout: time.Duration(timeout) * time.Second},
	})
	if err != nil {
		log.Fatal(err)
	}

	server := mcp.NewServer(&mcp.Implementation{Name: "github-mcp", Version: "0.1.0"}, nil)
	mcpserver.New(client).Register(server)

	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Printf("github-mcp stopped: %v", err)
	}
}

func setupLog(server string) {
	dir := filepath.Join(userLocalDir(), "mcp", server, "logs")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		log.Printf("aviso: falha ao criar %s; log segue para stderr: %v", dir, err)
		return
	}
	path := filepath.Join(dir, server+"-"+time.Now().Format("2006-01-02_15-04-05")+".log")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		log.Printf("aviso: falha ao abrir %s; log segue para stderr: %v", path, err)
		return
	}
	log.SetOutput(f)
}

func userLocalDir() string {
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".local", "share")
	}
	return filepath.Join(os.Getenv("USERPROFILE"), ".local", "share")
}
