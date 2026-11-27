package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"ntdsk.com/mcp/sandbox/internal/sandbox"
)

type Server struct {
	mnt  *sandbox.Store
	tmp  *sandbox.Store
	jobs *sandbox.Jobs
}

func New(mntDir, tmpDir string) *Server {
	tmp := sandbox.NewStore(tmpDir)
	tmp.MaxTotalBytes = int64(envInt("SANDBOX_TMP_SPACE_MB", 64)) * 1024 * 1024
	tmp.MaxFiles = 1000

	mnt := sandbox.NewStore(mntDir)
	mnt.MaxTotalBytes = int64(envInt("SANDBOX_MNT_SPACE_MB", 256)) * 1024 * 1024
	mnt.MaxFiles = 5000

	return &Server{mnt: mnt, tmp: tmp, jobs: sandbox.NewJobs()}
}

func envInt(name string, def int) int {
	v := os.Getenv(name)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	if n <= 0 {
		return def
	}
	return n
}

func (s *Server) Register(server *mcp.Server) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "sandbox_doc",
		Description: "Returns the sandbox API documentation (std.* modules, tools, limits, env vars, global scripts). Pass 'topic' to get a specific section (io, net, secrets, run, scripts, ...).",
	}, s.doc)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "sandbox_act",
		Description: "Run a Lua script by 'path' (host .lua file) or inline 'code'. Action: 'run' (default). Inline 'code' may be just the body — it is auto-wrapped in `function main(std)`. 'args' (array/object) becomes std.args. Set 'background' to run async and get a job id back (poll with sandbox_job). Sandboxed: no OS/process; filesystem confined to mnt/; network only via std.net (allowlist). Timeout of 180s (3 min) by default, configurable via SANDBOX_EXEC_TIMEOUT_SECONDS.",
	}, s.act)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "sandbox_job",
		Description: "Manage background sandbox jobs started with sandbox_act background=true. Actions: 'list' (default), 'status' (id), 'logs' (id, optional tail), 'kill' (id).",
	}, s.job)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "sandbox_os",
		Description: "Manage the sandbox filesystem: copy (host→sandbox), mount (sandbox→host), del, stat, and list (directory tree).",
	}, s.filesystem)
}

type actInput struct {
	Action     string `json:"action,omitempty" jsonschema:"Action: 'run' (default)."`
	Path       string `json:"path,omitempty" jsonschema:"Host/project path to a .lua file to run (reads the file)."`
	Code       string `json:"code,omitempty" jsonschema:"Inline Lua code to run. Pass only the body — the function main(std) wrapper is added automatically if missing."`
	Args       any    `json:"args,omitempty" jsonschema:"Script arguments (array or object; becomes std.args)."`
	Background bool   `json:"background,omitempty" jsonschema:"Run asynchronously and return a job id immediately; poll with sandbox_job (status/logs/kill)."`
}

type filesystemInput struct {
	Action string `json:"action" jsonschema:"Action to perform: copy (host→sandbox), mount (sandbox→host), del, stat, list."`
	Path   string `json:"path,omitempty" jsonschema:"Host source path for copy, or sandbox-relative path for mount/del/stat/list."`
	Dest   string `json:"dest,omitempty" jsonschema:"Destination: sandbox-relative path for copy, host path for mount."`
}

func (s *Server) act(ctx context.Context, _ *mcp.CallToolRequest, in actInput) (*mcp.CallToolResult, any, error) {
	switch action := strings.ToLower(strings.TrimSpace(in.Action)); action {
	case "", "run":
		return s.runScript(in)
	default:
		return nil, nil, fmt.Errorf("ação inválida %q; use 'run'", action)
	}
}

func (s *Server) runScript(in actInput) (*mcp.CallToolResult, any, error) {
	code := ""
	switch {
	case strings.TrimSpace(in.Path) != "":
		b, err := os.ReadFile(strings.TrimSpace(in.Path))
		if err != nil {
			return nil, nil, fmt.Errorf("ler script %q: %w", in.Path, err)
		}
		code = string(b)
	case strings.TrimSpace(in.Code) != "":
		code = in.Code
	default:
		return nil, nil, errors.New("informe 'path' (arquivo .lua) ou 'code' (inline)")
	}

	argsStr, err := marshalArgs(in.Args)
	if err != nil {
		return nil, nil, err
	}
	request := sandbox.RunRequest{Code: code, Args: argsStr}
	if in.Background {
		info := s.jobs.Start(s.mnt, s.tmp, request)
		return textResult(formatJobStarted(info))
	}
	res, err := sandbox.Run(s.mnt, s.tmp, request)
	return result(formatRunResult(res, err), false)
}

func marshalArgs(v any) (string, error) {
	switch x := v.(type) {
	case nil:
		return "", nil
	case string:
		return x, nil
	default:
		b, err := json.Marshal(x)
		if err != nil {
			return "", err
		}
		return string(b), nil
	}
}

func (s *Server) filesystem(ctx context.Context, _ *mcp.CallToolRequest, in filesystemInput) (*mcp.CallToolResult, any, error) {
	switch action := strings.ToLower(strings.TrimSpace(in.Action)); action {
	case "copy":
		dest, err := s.mnt.CopyIn(in.Path, in.Dest)
		if err != nil {
			return nil, nil, err
		}
		return textResult(fmt.Sprintf("Copiado `%s` → `%s` (sandbox).\n", in.Path, dest))
	case "mount":
		dest, err := s.mnt.CopyOut(in.Path, in.Dest)
		if err != nil {
			return nil, nil, err
		}
		return textResult(fmt.Sprintf("Copiado `%s` (sandbox) → `%s` (host).\n", in.Path, dest))
	case "del":
		if err := s.mnt.DeleteAll(in.Path); err != nil {
			return nil, nil, err
		}
		return textResult(fmt.Sprintf("Removido `%s` (sandbox).\n", in.Path))
	case "stat":
		st, err := s.mnt.Stat(in.Path)
		if err != nil {
			return nil, nil, err
		}
		return textResult(formatManageStat(in.Path, st))
	case "list":
		t, err := s.mnt.Tree(in.Path)
		if err != nil {
			return nil, nil, err
		}
		return textResult(fmt.Sprintf("```text\n%s\n```", FormatTree(t)))
	default:
		return nil, nil, fmt.Errorf("ação inválida %q; use copy, mount, del, stat ou list", action)
	}
}


