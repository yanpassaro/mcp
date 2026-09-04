package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"os"
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
		Name:        "sandbox_read",
		Description: "Read a saved Lua script by name, or list all saved scripts when 'name' is omitted.",
	}, s.readScript)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "sandbox_write",
		Description: "Create/overwrite a Lua script. Pass 'name', optional 'description', and 'code' (the body, auto-wrapped in function main(std)).",
	}, s.writeScript)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "sandbox_del",
		Description: "Delete a saved Lua script by name (.lua extension optional).",
	}, s.delScript)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "sandbox_run",
		Description: "Run a saved Lua script by 'name', with optional 'args'. Works on the std namespace: result.ok/err, log.ok/err (alias console.ok/err), args, fs.read/lines/json/write/append/del/exists/stat/dir, date.now/iso/format/parse/add/unix/diff, random.pick/shuffle/int/seed, str, list, num, json, assert, encode, fetch. No network/process/OS. Fixed 30s timeout.",
	}, s.runScript)
}

type readScriptInput struct {
	Name string `json:"name,omitempty" jsonschema:"Script name to read (.lua extension optional). If omitted, lists all saved scripts."`
}

type writeScriptInput struct {
	Name        string `json:"name" jsonschema:"Script file name (inside the scripts folder). The .lua extension is optional."`
	Description string `json:"description,omitempty" jsonschema:"One-line description of what the script does (optional)."`
	Code        string `json:"code" jsonschema:"The body of main(std) - the Lua code that runs. It is wrapped automatically."`
}

type delScriptInput struct {
	Name string `json:"name" jsonschema:"Script name to delete (.lua extension optional)."`
}

type runScriptInput struct {
	Name string `json:"name" jsonschema:"Saved script name (in the scripts folder) to run (.lua extension optional)."`
	Args string `json:"args,omitempty" jsonschema:"Arguments for the script, accessed via std.args. If valid JSON it is parsed (object/array); otherwise it stays a string."`
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

func (s *Server) writeScript(ctx context.Context, _ *mcp.CallToolRequest, in writeScriptInput) (*mcp.CallToolResult, any, error) {
	name := withLuaExt(strings.TrimSpace(in.Name))
	if name == "" {
		return nil, nil, errors.New("'name' é obrigatório")
	}
	if strings.TrimSpace(in.Code) == "" {
		return nil, nil, errors.New("'code' é obrigatório")
	}
	wrapped := sandbox.WrapScript(name, in.Description, in.Code)
	if _, err := s.scripts.Write(name, wrapped); err != nil {
		return nil, nil, err
	}
	return textResult(fmt.Sprintf("Script `%s` gravado (%d bytes).\n", name, len(wrapped)))
}

func (s *Server) delScript(ctx context.Context, _ *mcp.CallToolRequest, in delScriptInput) (*mcp.CallToolResult, any, error) {
	name := withLuaExt(strings.TrimSpace(in.Name))
	if name == "" {
		return nil, nil, errors.New("'name' é obrigatório")
	}
	for _, cand := range scriptNameVariants(name) {
		err := s.scripts.Delete(cand)
		if err == nil {
			return textResult(fmt.Sprintf("Script `%s` removido.\n", cand))
		}
		if !os.IsNotExist(err) {
			return nil, nil, err
		}
	}
	return nil, nil, fmt.Errorf("script %q não encontrado na pasta de scripts.", name)
}

func (s *Server) runScript(ctx context.Context, _ *mcp.CallToolRequest, in runScriptInput) (*mcp.CallToolResult, any, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return nil, nil, errors.New("informe 'name' (script salvo)")
	}
	code, err := s.readScriptSource(name)
	if err != nil {
		return nil, nil, err
	}

	res, err := sandbox.Run(s.fs, sandbox.RunRequest{
		Code: code,
		Args: in.Args,
	})
	if strings.TrimSpace(res.Name) == "" {
		res.Name = name
	}
	return result(formatRunResult(res, err), false)
}

func (s *Server) readScriptSource(name string) (string, error) {
	for _, cand := range scriptNameVariants(name) {
		if c, err := s.scripts.Read(cand); err == nil {
			return c, nil
		}
	}
	return "", fmt.Errorf("script %q não encontrado na pasta de scripts. Crie-o com sandbox_write (ou veja o que existe com sandbox_read).", name)
}

func withLuaExt(name string) string {
	if name == "" || strings.HasSuffix(strings.ToLower(name), ".lua") {
		return name
	}
	return name + ".lua"
}

func scriptNameVariants(name string) []string {
	name = strings.TrimSpace(name)
	if name == "" {
		return []string{""}
	}
	if strings.HasSuffix(strings.ToLower(name), ".lua") {
		return []string{name, name[:len(name)-len(".lua")]}
	}
	return []string{name, name + ".lua"}
}
