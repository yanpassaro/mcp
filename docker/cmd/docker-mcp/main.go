package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"ntdsk.com/mcp/docker/internal/docker"
	"ntdsk.com/mcp/docker/internal/mcpserver"
)

const DEFAULT_TIMEOUT = 60

func main() {
	setupLog("docker")

	timeout := DEFAULT_TIMEOUT
	if v := strings.TrimSpace(os.Getenv("DOCKER_TIMEOUT_SECONDS")); v != "" {
		n, err := strconv.Atoi(v)
		if err == nil {
			if n > 0 {
				timeout = n
			}
		}
	}

	client, err := docker.NewClient(docker.Config{
		Timeout: time.Duration(timeout) * time.Second,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if closeErr := client.Close(); closeErr != nil {
			log.Printf("docker client close: %v", closeErr)
		}
	}()

	server := mcp.NewServer(&mcp.Implementation{Name: "docker-mcp", Version: "0.1.0"}, nil)
	mcpserver.New(client).Register(server)

	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Printf("docker-mcp stopped: %v", err)
	}
}

func setupLog(server string) {
	dir := filepath.Join(userStateDir(), "logs")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		log.Printf("aviso: falha ao criar %s; log segue para stderr: %v", dir, err)
		return
	}
	path := filepath.Join(dir, fmt.Sprintf("%s-%s.log", server, time.Now().Format("2006-01-02_15-04-05")))
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		log.Printf("aviso: falha ao abrir %s; log segue para stderr: %v", path, err)
		return
	}
	log.SetOutput(f)
}

func userStateDir() string {
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".local", "state", "mcp")
	}
	return filepath.Join(os.Getenv("USERPROFILE"), ".local", "state", "mcp")
}
