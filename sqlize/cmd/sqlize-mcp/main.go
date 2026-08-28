package main

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"ntdsk.com/mcp/sqlize/internal/sqlize"
)

func main() {
	setupLog("sqlize")

	stateDir := strings.TrimSpace(os.Getenv("SQLIZE_STATE_DIR"))

	s, err := sqlize.New(stateDir)
	if err != nil {
		log.Fatalf("inicializar sqlize-mcp: %v", err)
	}
	defer s.Close()

	server := mcp.NewServer(&mcp.Implementation{Name: "sqlize-mcp", Version: "0.1.0"}, nil)
	s.Register(server)
	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Printf("sqlize-mcp encerrou: %v", err)
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
