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
	tools     []shell.Tool
	toolsFile string
	actOn     bool
	timeout   time.Duration
}

func New(registry *shell.Registry, tools []shell.Tool, toolsFile string) *Server {
	return &Server{registry: registry, tools: tools, toolsFile: toolsFile, actOn: actAllowed(), timeout: time.Duration(envTimeout()) * time.Second}
}

func (s *Server) Register(server *mcp.Server) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "shell_read",
		Description: "Local processes: list (running/exited), log (tail of output by id) or tools (registered command tools).",
	}, s.read)

	if !s.actOn {
		return
	}
	s.registerTools(server)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "shell_kill",
		Description: "Kill a background process started by a shell tool, by id.",
	}, s.killTool)
}

func (s *Server) registerTools(server *mcp.Server) {
	for _, t := range s.tools {
		tool := t
		mcp.AddTool(server, &mcp.Tool{
			Name:        fmt.Sprintf("shell_%s", tool.Name),
			Description: fmt.Sprintf("%s Runs: `%s`", strings.TrimSpace(tool.Desc), tool.Cmd),
		}, func(ctx context.Context, req *mcp.CallToolRequest, in map[string]string) (*mcp.CallToolResult, any, error) {
			return s.runTool(ctx, tool, in)
		})
	}
}

func (s *Server) runTool(ctx context.Context, tool shell.Tool, in map[string]string) (*mcp.CallToolResult, any, error) {
	values := make(map[string]string, len(in))
	for name, v := range in {
		values[name] = strings.TrimSpace(v)
	}
	fields, err := shell.Render(tool.Cmd, values)
	if err != nil {
		return nil, nil, fmt.Errorf("shell_%s: %w", tool.Name, err)
	}
	command := strings.Join(fields, " ")
	cwd := strings.TrimSpace(values["cwd"])
	if tool.Background {
		id, err := s.registry.BackgroundFields(command, fields, cwd, nil)
		if err != nil {
			return nil, nil, fmt.Errorf("shell_%s: %w", tool.Name, err)
		}
		return textResult(fmt.Sprintf("✅ Processo `%s` iniciado em background:\n\n- **Comando:** `%s`\n- **Log:** `shell_read` → `{ \"action\": \"log\", \"data\": { \"id\": \"%s\" } }`\n- **Parar:** `shell_kill` → `{ \"id\": \"%s\" }`", id, command, id, id))
	}
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	result, err := s.registry.ForegroundFields(ctx, command, fields, cwd, nil)
	if err != nil {
		if ctx.Err() != nil {
			return nil, nil, fmt.Errorf("shell_%s: %w", tool.Name, ctx.Err())
		}
		return nil, nil, fmt.Errorf("shell_%s: %w\n\n%s", tool.Name, err, result.Output)
	}
	return textResult(formatRun(result))
}

func (s *Server) killTool(ctx context.Context, req *mcp.CallToolRequest, in map[string]string) (*mcp.CallToolResult, any, error) {
	info, err := s.registry.Kill(strings.TrimSpace(in["id"]))
	if err != nil {
		return nil, nil, err
	}
	return textResult(fmt.Sprintf("🛑 Processo `%s` encerrado: `%s` (exit %d).", info.ID, info.Command, info.Exit))
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
	case "tools":
		return textResult(formatTools(s.tools, s.toolsFile))
	case "":
		return nil, nil, errors.New("'action' é obrigatório")
	default:
		return nil, nil, errors.New("'action' inválido: list, log, tools")
	}
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
