package main

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"ntdsk.com/mcp/git/internal/mcpserver"
)

func main() {
	setupLog("git")

	server := mcp.NewServer(&mcp.Implementation{Name: "git-mcp", Version: "0.1.0"}, nil)
	mcpserver.New().Register(server)

	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Printf("git-mcp stopped: %v", err)
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
