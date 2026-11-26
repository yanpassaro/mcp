package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"ntdsk.com/mcp/shell/internal/shell"
)

const (
	DEFAULT_TIMEOUT_SECONDS = 60
)

type Server struct {
	registry  *shell.Registry
	patterns  []string
	allowFile string
	actOn     bool
}

func New(registry *shell.Registry, patterns []string, allowFile string) *Server {
	return &Server{registry: registry, patterns: patterns, allowFile: allowFile, actOn: actAllowed()}
}

func (s *Server) Register(server *mcp.Server) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "shell_read",
		Description: "Local processes: list (running/exited), log (tail of output by id) or allow (show the command allowlist).",
	}, s.read)

	if s.actOn {
		mcp.AddTool(server, &mcp.Tool{
			Name:        "shell_act",
			Description: "Run allowlisted commands locally (no shell: plain argv) in foreground or background, or kill a background process by id.",
		}, s.act)
	}
}

func (s *Server) read(ctx context.Context, _ *mcp.CallToolRequest, in input) (*mcp.CallToolResult, any, error) {
	action := strings.ToLower(strings.TrimSpace(in.Action))
	switch action {
	case "list":
		return textResult(formatList(s.registry.List()))
	case "log":
		if strings.TrimSpace(in.Data.ID) == "" {
			return nil, nil, errors.New("log exige 'id' em 'data'")
		}
		result, err := s.registry.Log(in.Data.ID, in.Data.Max)
		if err != nil {
			return nil, nil, err
		}
		return textResult(formatLog(result))
	case "allow":
		return textResult(formatAllow(s.patterns, s.allowFile))
	case "":
		return nil, nil, errors.New("'action' é obrigatório")
	default:
		return nil, nil, errors.New("'action' inválido: list, log, allow")
	}
}

func (s *Server) act(ctx context.Context, _ *mcp.CallToolRequest, in actInput) (*mcp.CallToolResult, any, error) {
	action := strings.ToLower(strings.TrimSpace(in.Action))
	if action == "" {
		return nil, nil, errors.New("'action' é obrigatório")
	}
	ctx, cancel := context.WithTimeout(ctx, timeoutFor(in.Data))
	defer cancel()
	switch action {
	case "run":
		return s.run(ctx, in.Data)
	case "kill":
		return s.kill(in.Data)
	default:
		return nil, nil, errors.New("'action' inválido: run, kill")
	}
}

func (s *Server) run(ctx context.Context, d actData) (*mcp.CallToolResult, any, error) {
	command := strings.TrimSpace(d.Command)
	if command == "" {
		return nil, nil, errors.New("run exige 'command' em 'data'")
	}
	pattern, ok := shell.Match(s.patterns, command)
	if !ok {
		return nil, nil, fmt.Errorf("comando não permitido: %q (allowlist: %s)", command, s.allowFile)
	}
	env, err := parseEnv(d.Env)
	if err != nil {
		return nil, nil, err
	}
	if d.Background {
		id, err := s.registry.Background(command, strings.TrimSpace(d.Cwd), env)
		if err != nil {
			return nil, nil, err
		}
		return textResult(fmt.Sprintf("✅ Processo `%s` iniciado em background:\n\n- **Comando:** `%s`\n- **Padrão:** `%s`\n- **Log:** `shell_read` → `{ \"action\": \"log\", \"data\": { \"id\": \"%s\" } }`\n- **Parar:** `shell_act` → `{ \"action\": \"kill\", \"data\": { \"id\": \"%s\" } }`", id, command, pattern, id, id))
	}
	result, err := s.registry.Foreground(ctx, command, strings.TrimSpace(d.Cwd), env)
	if err != nil {
		if ctx.Err() != nil {
			return nil, nil, fmt.Errorf("run %q: %w", command, ctx.Err())
		}
		return nil, nil, fmt.Errorf("run %q: %w\n\n%s", command, err, result.Output)
	}
	return textResult(formatRun(result))
}

func (s *Server) kill(d actData) (*mcp.CallToolResult, any, error) {
	info, err := s.registry.Kill(d.ID)
	if err != nil {
		return nil, nil, err
	}
	return textResult(fmt.Sprintf("🛑 Processo `%s` encerrado: `%s` (exit %d).", info.ID, info.Command, info.Exit))
}

func timeoutFor(d actData) time.Duration {
	if d.Timeout > 0 {
		return time.Duration(d.Timeout) * time.Second
	}
	return time.Duration(envTimeout()) * time.Second
}

func envTimeout() int {
	n, err := strconv.Atoi(strings.TrimSpace(os.Getenv("SHELL_TIMEOUT_SECONDS")))
	if err != nil {
		return DEFAULT_TIMEOUT_SECONDS
	}
	if n <= 0 {
		return DEFAULT_TIMEOUT_SECONDS
	}
	return n
}

func actAllowed() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("SHELL_ALLOW_ACT"))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func parseEnv(env []string) ([]string, error) {
	cleaned := make([]string, 0, len(env))
	for _, e := range env {
		e = strings.TrimSpace(e)
		if e == "" {
			continue
		}
		if !strings.Contains(e, "=") {
			return nil, fmt.Errorf("env inválida %q: use KEY=VALUE", e)
		}
		cleaned = append(cleaned, e)
	}
	return cleaned, nil
}
