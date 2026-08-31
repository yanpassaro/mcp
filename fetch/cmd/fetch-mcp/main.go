package main

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"ntdsk.com/mcp/fetch/internal/fetch"
	"ntdsk.com/mcp/fetch/internal/mcpserver"
)

func main() {
	logFile := setupLog("fetch")

	var hosts []string
	if allowRaw := strings.TrimSpace(os.Getenv("FETCH_ALLOW_HOST")); allowRaw != "" {
		hosts = strings.Split(allowRaw, ",")
	}

	timeout := 30
	if v := strings.TrimSpace(os.Getenv("FETCH_TIMEOUT_SECONDS")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			timeout = n
		}
	}

	maxBody := int64(1 << 20)
	if v := strings.TrimSpace(os.Getenv("FETCH_MAX_BODY_KB")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			maxBody = int64(n) * 1024
		}
	}

	cookieFile := strings.TrimSpace(os.Getenv("FETCH_COOKIE_FILE"))
	if cookieFile == "" {
		cookieFile = filepath.Join(userLocalDir(), "mcp", "fetch", "cookies.json")
	}

	client, err := fetch.NewClient(fetch.Config{
		AllowHosts: hosts,
		Timeout:    time.Duration(timeout) * time.Second,
		CookieFile: cookieFile,
		MaxBody:    maxBody,
	})
	if err != nil {
		log.Fatalf("inicializar fetch-mcp: %v", err)
	}

	server := mcp.NewServer(&mcp.Implementation{Name: "fetch-mcp", Version: "0.1.0"}, nil)
	mcpserver.New(client, logFile).Register(server)

	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Printf("fetch-mcp stopped: %v", err)
	}
}

func setupLog(server string) *os.File {
	dir := filepath.Join(userLocalDir(), "mcp", server, "logs")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		log.Printf("aviso: falha ao criar %s; log segue para stderr: %v", dir, err)
		return nil
	}
	path := filepath.Join(dir, server+"-"+time.Now().Format("2006-01-02_15-04-05")+".log")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		log.Printf("aviso: falha ao abrir %s; log segue para stderr: %v", path, err)
		return nil
	}
	log.SetOutput(f)
	return f
}

func userLocalDir() string {
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".local", "share")
	}
	return filepath.Join(os.Getenv("USERPROFILE"), ".local", "share")
}
