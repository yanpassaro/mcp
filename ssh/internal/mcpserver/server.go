package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"ntdsk.com/mcp/ssh/internal/ssh"
)

const MAX_TRANSFER_BYTES = 32 * 1024 * 1024

type Server struct {
	conns []ssh.Config
	mnt   string
}

func New(conns []ssh.Config, mnt string) *Server {
	return &Server{conns: conns, mnt: mnt}
}

func (s *Server) Register(server *mcp.Server) {
	targets := s.targets()

	mcp.AddTool(server, &mcp.Tool{
		Name:        "ssh_run",
		Description: fmt.Sprintf("Run a shell command on a remote host over SSH and return exit code, stdout and stderr. 'target' selects the connection (%s).", targets),
	}, s.runTool)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "ssh_read",
		Description: fmt.Sprintf("Read a remote file as text (default 256 KiB, max 32 MiB). .env, *.env and environ files are blocked. 'target' selects the connection (%s).", targets),
	}, s.readTool)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "ssh_write",
		Description: fmt.Sprintf("Write or append text to a remote file. 'target' selects the connection (%s).", targets),
	}, s.writeTool)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "ssh_transfer",
		Description: fmt.Sprintf("Upload a file from the shared mnt dir to the remote host, or download a remote file into it (max 32 MiB). 'target' selects the connection (%s).", targets),
	}, s.transferTool)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "ssh_probe",
		Description: fmt.Sprintf("Remote host overview: kernel, uptime, disk usage and listening ports. 'target' selects the connection (%s).", targets),
	}, s.probeTool)
}

func (s *Server) targets() string {
	names := make([]string, 0, len(s.conns))
	for _, cfg := range s.conns {
		names = append(names, ssh.Label(cfg.Alias))
	}
	return strings.Join(names, ", ")
}

func (s *Server) conn(target string) (ssh.Config, error) {
	name := strings.ToLower(strings.TrimSpace(target))
	if name == "" {
		if len(s.conns) == 1 {
			return s.conns[0], nil
		}
		return ssh.Config{}, fmt.Errorf("target é obrigatório; conexões: %s", s.targets())
	}
	for _, cfg := range s.conns {
		if ssh.Label(cfg.Alias) == name {
			return cfg, nil
		}
	}
	return ssh.Config{}, fmt.Errorf("conexão %q não encontrada; disponíveis: %s", name, s.targets())
}

func (s *Server) ctxWithTimeout(ctx context.Context, cfg ssh.Config, seconds int) (context.Context, context.CancelFunc) {
	if seconds > 0 {
		return context.WithTimeout(ctx, time.Duration(seconds)*time.Second)
	}
	return context.WithTimeout(ctx, cfg.Timeout())
}

func (s *Server) runTool(ctx context.Context, _ *mcp.CallToolRequest, in runInput) (*mcp.CallToolResult, any, error) {
	cfg, err := s.conn(in.Target)
	if err != nil {
		return nil, nil, err
	}
	command := strings.TrimSpace(in.Command)
	if command == "" {
		return nil, nil, errors.New("command é obrigatório")
	}
	ctx, cancel := s.ctxWithTimeout(ctx, cfg, in.TimeoutSeconds)
	defer cancel()
	result, err := ssh.Run(ctx, cfg, command, strings.TrimSpace(in.Workdir))
	if err != nil {
		return nil, nil, err
	}
	return textResult(formatRun(cfg, command, result))
}

func (s *Server) readTool(ctx context.Context, _ *mcp.CallToolRequest, in readInput) (*mcp.CallToolResult, any, error) {
	cfg, err := s.conn(in.Target)
	if err != nil {
		return nil, nil, err
	}
	ctx, cancel := s.ctxWithTimeout(ctx, cfg, in.TimeoutSeconds)
	defer cancel()
	data, truncated, err := ssh.Read(ctx, cfg, in.Path, in.MaxBytes)
	if err != nil {
		return nil, nil, err
	}
	return textResult(formatRead(cfg, in.Path, string(data), truncated))
}

func (s *Server) writeTool(ctx context.Context, _ *mcp.CallToolRequest, in writeInput) (*mcp.CallToolResult, any, error) {
	cfg, err := s.conn(in.Target)
	if err != nil {
		return nil, nil, err
	}
	ctx, cancel := s.ctxWithTimeout(ctx, cfg, in.TimeoutSeconds)
	defer cancel()
	written, err := ssh.Write(ctx, cfg, in.Path, strings.NewReader(in.Content), in.Append)
	if err != nil {
		return nil, nil, err
	}
	return textResult(formatWrite(cfg, in.Path, written))
}

func (s *Server) transferTool(ctx context.Context, _ *mcp.CallToolRequest, in transferInput) (*mcp.CallToolResult, any, error) {
	cfg, err := s.conn(in.Target)
	if err != nil {
		return nil, nil, err
	}
	direction := strings.ToLower(strings.TrimSpace(in.Direction))
	remote := strings.TrimSpace(in.Remote)
	if remote == "" {
		return nil, nil, errors.New("remote é obrigatório")
	}
	local, err := s.localPath(in.Local)
	if err != nil {
		return nil, nil, err
	}
	ctx, cancel := s.ctxWithTimeout(ctx, cfg, in.TimeoutSeconds)
	defer cancel()
	switch direction {
	case "upload":
		return s.upload(ctx, cfg, local, remote)
	case "download":
		return s.download(ctx, cfg, local, remote)
	default:
		return nil, nil, errors.New("'direction' inválido: use upload ou download")
	}
}

func (s *Server) upload(ctx context.Context, cfg ssh.Config, local, remote string) (*mcp.CallToolResult, any, error) {
	file, err := os.Open(local)
	if err != nil {
		return nil, nil, fmt.Errorf("abrir %s: %w", local, err)
	}
	written, writeErr := ssh.Write(ctx, cfg, remote, file, false)
	closeErr := file.Close()
	if writeErr != nil {
		return nil, nil, writeErr
	}
	if closeErr != nil {
		return nil, nil, fmt.Errorf("fechar %s: %w", local, closeErr)
	}
	return textResult(formatTransfer("upload", local, remote, written, false))
}

func (s *Server) download(ctx context.Context, cfg ssh.Config, local, remote string) (*mcp.CallToolResult, any, error) {
	data, truncated, err := ssh.Read(ctx, cfg, remote, MAX_TRANSFER_BYTES)
	if err != nil {
		return nil, nil, err
	}
	dir := filepath.Dir(local)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, nil, fmt.Errorf("criar %s: %w", dir, err)
	}
	if err := os.WriteFile(local, data, 0o644); err != nil {
		return nil, nil, fmt.Errorf("gravar %s: %w", local, err)
	}
	return textResult(formatTransfer("download", local, remote, int64(len(data)), truncated))
}

func (s *Server) probeTool(ctx context.Context, _ *mcp.CallToolRequest, in probeInput) (*mcp.CallToolResult, any, error) {
	cfg, err := s.conn(in.Target)
	if err != nil {
		return nil, nil, err
	}
	ctx, cancel := s.ctxWithTimeout(ctx, cfg, in.TimeoutSeconds)
	defer cancel()
	result, err := ssh.Probe(ctx, cfg)
	if err != nil {
		return nil, nil, err
	}
	if result.ExitCode != 0 {
		return textResult(formatProbe(cfg, fmt.Sprintf("%s\n\n%s", strings.TrimSpace(result.Stdout), strings.TrimSpace(result.Stderr))))
	}
	return textResult(formatProbe(cfg, result.Stdout))
}

func (s *Server) localPath(name string) (string, error) {
	clean := filepath.Clean(strings.TrimSpace(name))
	if clean == "" {
		return "", errors.New("local é obrigatório")
	}
	if clean == "." {
		return "", errors.New("local é obrigatório")
	}
	if filepath.IsAbs(clean) {
		return "", errors.New("local deve ser relativo ao mnt compartilhado")
	}
	if strings.HasPrefix(clean, "..") {
		return "", errors.New("local não pode sair do mnt compartilhado")
	}
	return filepath.Join(s.mnt, clean), nil
}

type runInput struct {
	Target         string `json:"target,omitempty" jsonschema:"SSH connection alias (optional when only one is configured)."`
	Command        string `json:"command" jsonschema:"Shell command to run on the remote host (required)."`
	Workdir        string `json:"workdir,omitempty" jsonschema:"Directory to run the command in (optional)."`
	TimeoutSeconds int    `json:"timeoutSeconds,omitempty" jsonschema:"Optional timeout override, in seconds."`
}

type readInput struct {
	Target         string `json:"target,omitempty" jsonschema:"SSH connection alias (optional when only one is configured)."`
	Path           string `json:"path" jsonschema:"Remote file path (required)."`
	MaxBytes       int    `json:"maxBytes,omitempty" jsonschema:"Maximum bytes to read (default 262144, max 33554432)."`
	TimeoutSeconds int    `json:"timeoutSeconds,omitempty" jsonschema:"Optional timeout override, in seconds."`
}

type writeInput struct {
	Target         string `json:"target,omitempty" jsonschema:"SSH connection alias (optional when only one is configured)."`
	Path           string `json:"path" jsonschema:"Remote file path (required)."`
	Content        string `json:"content" jsonschema:"Text to write (required)."`
	Append         bool   `json:"append,omitempty" jsonschema:"If true, appends instead of overwriting."`
	TimeoutSeconds int    `json:"timeoutSeconds,omitempty" jsonschema:"Optional timeout override, in seconds."`
}

type transferInput struct {
	Target         string `json:"target,omitempty" jsonschema:"SSH connection alias (optional when only one is configured)."`
	Direction      string `json:"direction" jsonschema:"upload (mnt -> remote) or download (remote -> mnt) (required)."`
	Local          string `json:"local" jsonschema:"File name or relative path inside the shared mnt dir (required)."`
	Remote         string `json:"remote" jsonschema:"Remote file path (required)."`
	TimeoutSeconds int    `json:"timeoutSeconds,omitempty" jsonschema:"Optional timeout override, in seconds."`
}

type probeInput struct {
	Target         string `json:"target,omitempty" jsonschema:"SSH connection alias (optional when only one is configured)."`
	TimeoutSeconds int    `json:"timeoutSeconds,omitempty" jsonschema:"Optional timeout override, in seconds."`
}

func textResult(text string) (*mcp.CallToolResult, any, error) {
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: text}},
	}, nil, nil
}
