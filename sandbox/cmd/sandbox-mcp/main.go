package main

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"ntdsk.com/mcp/sandbox/internal/mcpserver"
)

func main() {
	setupLog("sandbox")

	fsDir := fsDir()
	scriptsDir := scriptsDir(fsDir)
	log.Printf("sandbox-mcp: fs=%s scripts=%s", fsDir, scriptsDir)

	server := mcp.NewServer(&mcp.Implementation{Name: "sandbox-mcp", Version: "0.1.0"}, nil)
	mcpserver.New(fsDir, scriptsDir).Register(server)

	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Printf("sandbox-mcp stopped: %v", err)
	}
}

func fsDir() string {
	if d := strings.TrimSpace(os.Getenv("SANDBOX_FS_DIR")); d != "" {
		return d
	}
	dir := filepath.Join(userLocalDir(), "mcp", "sandbox", "fs")
	seedFsDir(dir)
	return dir
}

func scriptsDir(fsDir string) string {
	if d := strings.TrimSpace(os.Getenv("SANDBOX_SCRIPTS_DIR")); d != "" {
		return d
	}
	dir := filepath.Join(filepath.Dir(fsDir), "scripts")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		log.Printf("warning: failed to create %s: %v", dir, err)
	}
	return dir
}

func seedFsDir(dir string) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		log.Printf("warning: failed to create %s: %v", dir, err)
		return
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) > 0 {
		return
	}
	seeds := map[string]string{
		"first.txt":  "Ava\nLiam\nMaya\nNoah\nZoe\nKai\nElena\nTheo\nIsla\nHugo\nNora\nFinn\nLena\nOmar\nChloe\n",
		"last.txt":   "Silva\nSantos\nOliveira\nSouza\nCosta\nPereira\nAlmeida\nFerreira\nRodrigues\nGomes\nMartins\nBarbosa\n",
		"prefix.txt": "Ae\nBa\nCa\nDe\nEo\nFa\nGa\nHe\nIo\nJo\nKa\nLe\n",
		"suffix.txt": "ron\nal\nin\neth\nara\nova\niel\no\nys\n",
	}
	for name, content := range seeds {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			log.Printf("warning: failed to create %s: %v", name, err)
		}
	}
	log.Printf("sandbox-mcp: folder %s empty; samples created", dir)
}

func setupLog(server string) {
	dir := filepath.Join(userLocalDir(), "mcp", server, "logs")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		log.Printf("warning: failed to create %s; log goes to stderr: %v", dir, err)
		return
	}
	path := filepath.Join(dir, server+"-"+time.Now().Format("2006-01-02_15-04-05")+".log")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		log.Printf("warning: failed to open %s; log goes to stderr: %v", path, err)
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
