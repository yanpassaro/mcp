package main

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"ntdsk.com/mcp/sandbox/internal/mcpserver"
	"ntdsk.com/mcp/sandbox/internal/sandbox"
)

func main() {
	setupLog("sandbox")

	if mb := memLimitMB(); mb > 0 {
		debug.SetMemoryLimit(int64(mb) * 1024 * 1024)
		log.Printf("sandbox-mcp: RAM limit = %d MiB", mb)
	}

	mntDir := mntDir()
	tmpDir := tmpDir()

	clearStore := func(label, dir string) {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			log.Printf("sandbox-mcp: %s clear error (mkdir): %v", label, err)
			return
		}
		n, err := sandbox.NewStore(dir).Clear()
		if err != nil {
			log.Printf("sandbox-mcp: %s clear error: %v", label, err)
			return
		}
		if n > 0 {
			log.Printf("sandbox-mcp: %s clear removed %d file(s)", label, n)
		}
	}
	clearStore("tmp", tmpDir)

	log.Printf("sandbox-mcp: mnt=%s tmp=%s", mntDir, tmpDir)

	server := mcp.NewServer(&mcp.Implementation{Name: "sandbox-mcp", Version: "0.1.0"}, nil)
	mcpserver.New(mntDir, tmpDir).Register(server)

	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Printf("sandbox-mcp stopped: %v", err)
	}
}

func mntDir() string {
	dir := filepath.Join(userStateDir(), "mnt")
	seedMntDir(dir)
	return dir
}


func memLimitMB() int64 {
	v := strings.TrimSpace(os.Getenv("SANDBOX_MEM_LIMIT_MB"))
	if v == "" {
		return 512
	}
	if n, err := strconv.Atoi(v); err == nil && n > 0 {
		return int64(n)
	}
	return 512
}

func tmpDir() string {
	dir := filepath.Join(userStateDir(), "tmp")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		log.Printf("warning: failed to create %s: %v", dir, err)
	}
	return dir
}



func seedMntDir(dir string) {
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

func userStateDir() string {
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".local", "state", "mcp")
	}
	return filepath.Join(os.Getenv("USERPROFILE"), ".local", "state", "mcp")
}

func userLocalDir() string {
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".local", "share")
	}
	return filepath.Join(os.Getenv("USERPROFILE"), ".local", "share")
}
