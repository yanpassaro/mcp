package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"ntdsk.com/mcp/search/internal/mcpserver"
	"ntdsk.com/mcp/search/internal/search"
)

func main() {
	setupLog("search")

	apiKey := strings.TrimSpace(os.Getenv("EXA_API_KEY"))
	if apiKey == "" {
		log.Fatal("EXA_API_KEY is required")
	}

	searchType := strings.TrimSpace(os.Getenv("EXA_SEARCH_TYPE"))
	if searchType == "" {
		searchType = "auto"
	}

	client, err := search.NewClient(search.Config{
		BaseURL:    "https://api.exa.ai",
		APIKey:     apiKey,
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
	})
	if err != nil {
		log.Fatal(err)
	}

	cfg := mcpserver.Config{
		NumResults: 10,
		SearchType: searchType,
		Search:     mcpserver.DefaultSearchContent(),
		Fetch:      mcpserver.DefaultFetchContent(),
	}

	server := mcp.NewServer(&mcp.Implementation{Name: "search-mcp", Version: "0.1.0"}, nil)
	mcpserver.New(client, cfg).Register(server)

	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Printf("search-mcp stopped: %v", err)
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
