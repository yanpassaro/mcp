package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"ntdsk.com/mcp/sandbox/internal/sandbox"
)

type Server struct {
	fs      *sandbox.Store
	scripts *sandbox.Store
}

func New(fsDir, scriptsDir string) *Server {
	return &Server{
		fs:      sandbox.NewStore(fsDir),
		scripts: sandbox.NewStore(scriptsDir),
	}
}

func (s *Server) Register(server *mcp.Server) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "sandbox_write_script",
		Description: "Write/overwrite a JavaScript script in the sandbox scripts folder, or remove an existing script (delete=true). The script runs isolated and non-destructively.",
	}, s.writeScript)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "sandbox_read_script",
		Description: "List all saved scripts (if 'name' is omitted) or read one script's content (if 'name' is provided).",
	}, s.readScript)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "sandbox_run_script",
		Description: "Run a JavaScript script in the isolated sandbox. Format: `const name = ...; const desc = ...; function main(std) { ... return std.return.ok(data) | std.return.err(message); }`. Namespace std: return.ok/err, log.ok/err, args, fs.read/lines/json/write/append/del/exists/stat/dir, date.now/iso/format/parse/add, random, str, list, num, json, assert, encode. No network/process/OS. Fixed 30s timeout.",
	}, s.runScript)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "sandbox_help",
		Description: "Documentation in English: the script format (main/std), the SDK, the sandbox filesystem, the tools, and a runnable example.",
	}, s.help)
}

type writeScriptInput struct {
	Name        string `json:"name" jsonschema:"Script file name (inside the scripts folder)."`
	Description string `json:"description,omitempty" jsonschema:"One-line description of what the script does (optional)."`
	Code        string `json:"code" jsonschema:"The inner body of main(std) - the code that runs inside the function. It is wrapped automatically."`
	Delete      bool   `json:"delete,omitempty" jsonschema:"If true, deletes the script instead of writing it."`
}

func (s *Server) writeScript(ctx context.Context, _ *mcp.CallToolRequest, in writeScriptInput) (*mcp.CallToolResult, any, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return nil, nil, errors.New("'name' é obrigatório")
	}

	if in.Delete {
		if err := s.scripts.Delete(name); err != nil {
			return nil, nil, err
		}
		return textResult(fmt.Sprintf("Script `%s` removido.\n", name))
	}

	if strings.TrimSpace(in.Code) == "" {
		return nil, nil, errors.New("'code' é obrigatório para gravar (ou use 'delete'=true para remover)")
	}
	wrapped := sandbox.WrapScript(name, in.Description, in.Code)
	if _, err := s.scripts.Write(name, wrapped); err != nil {
		return nil, nil, err
	}
	return textResult(fmt.Sprintf("Script `%s` gravado (%d bytes).\n", name, len(wrapped)))
}

type helpInput struct {
	Topic string `json:"topic,omitempty" jsonschema:"Optional topic to focus on (e.g. sdk, write, run)."`
}

func (s *Server) help(ctx context.Context, _ *mcp.CallToolRequest, in helpInput) (*mcp.CallToolResult, any, error) {
	return textResult(sandboxDocs)
}

type readScriptInput struct {
	Name string `json:"name,omitempty" jsonschema:"Script name to read. If omitted, lists all saved scripts."`
}

func (s *Server) readScript(ctx context.Context, _ *mcp.CallToolRequest, in readScriptInput) (*mcp.CallToolResult, any, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		entries, err := s.scripts.List()
		if err != nil {
			return nil, nil, err
		}
		return textResult(formatScriptList(entries))
	}
	content, err := s.readScriptSource(name)
	if err != nil {
		return nil, nil, err
	}
	return textResult(formatScriptRead(name, content))
}

type runScriptInput struct {
	Name string `json:"name,omitempty" jsonschema:"Saved script name (in the scripts folder) to run. If provided, ignores 'code'."`
	Code string `json:"code,omitempty" jsonschema:"Inline JavaScript to run (used when 'name' is not provided)."`
	Args string `json:"args,omitempty" jsonschema:"Arguments for the script, accessed via std.args. If valid JSON it is parsed (object/array); otherwise it stays a string."`
}

func (s *Server) runScript(ctx context.Context, _ *mcp.CallToolRequest, in runScriptInput) (*mcp.CallToolResult, any, error) {
	code := in.Code
	if strings.TrimSpace(in.Name) != "" {
		c, err := s.readScriptSource(in.Name)
		if err != nil {
			return nil, nil, err
		}
		code = c
	}
	if strings.TrimSpace(code) == "" {
		return nil, nil, errors.New("informe 'name' (script salvo) ou 'code' (inline)")
	}

	res, err := sandbox.Run(s.fs, sandbox.RunRequest{
		Code: code,
		Args: in.Args,
	})
	if strings.TrimSpace(res.Name) == "" && strings.TrimSpace(in.Name) != "" {
		res.Name = strings.TrimSpace(in.Name)
	}
	return result(formatRunResult(res, err), err != nil || !res.Ok)
}

func (s *Server) readScriptSource(name string) (string, error) {
	name = strings.TrimSpace(name)
	c, err := s.scripts.Read(name)
	if err == nil {
		return c, nil
	}
	if os.IsNotExist(err) && filepath.Ext(name) == "" {
		if c2, err2 := s.scripts.Read(name + ".js"); err2 == nil {
			return c2, nil
		}
	}
	return "", fmt.Errorf("script %q não encontrado na pasta de scripts. Crie-o com sandbox_write_script (ou veja o que existe com sandbox_read_script).", name)
}
