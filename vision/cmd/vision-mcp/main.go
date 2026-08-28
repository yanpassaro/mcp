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
	"ntdsk.com/mcp/vision/internal/mcpserver"
	"ntdsk.com/mcp/vision/internal/vision"
)

func main() {
	setupLog("vision")

	baseURL := getenvDefault("VISION_BASE_URL", "https://api.openai.com/v1")
	apiKey := strings.TrimSpace(os.Getenv("VISION_API_KEY"))
	model := getenvDefault("VISION_MODEL", "gpt-4o")

	timeout, _ := envInt("VISION_TIMEOUT_SECONDS", 120)
	if timeout < 1 {
		timeout = 120
	}
	maxTokens, _ := envInt("VISION_MAX_TOKENS", 1500)
	if maxTokens < 1 {
		maxTokens = 1500
	}
	maxImageMB, _ := envInt("VISION_MAX_IMAGE_MB", 25)
	if maxImageMB < 1 {
		maxImageMB = 25
	}

	client, err := vision.NewClient(vision.Config{
		BaseURL:       baseURL,
		APIKey:        apiKey,
		Model:         model,
		HTTPClient:    &http.Client{Timeout: time.Duration(timeout) * time.Second},
		MaxImageBytes: int64(maxImageMB) * 1024 * 1024,
		MaxTokens:     maxTokens,
	})
	if err != nil {
		log.Fatal(err)
	}

	server := mcp.NewServer(&mcp.Implementation{Name: "vision-mcp", Version: "0.1.0"}, nil)
	mcpserver.New(client).Register(server)

	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Printf("vision-mcp stopped: %v", err)
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

func envInt(name string, fallback int) (int, error) {
	v := strings.TrimSpace(os.Getenv(name))
	if v == "" {
		return fallback, nil
	}
	return strconv.Atoi(v)
}

func getenvDefault(name, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(name)); v != "" {
		return v
	}
	return fallback
}
