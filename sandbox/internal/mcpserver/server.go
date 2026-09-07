package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"ntdsk.com/mcp/sandbox/internal/sandbox"
)

type Server struct {
	mnt      *sandbox.Store
	dev      *sandbox.Store
	tmp      *sandbox.Store
	tscripts *sandbox.Store
}

func New(mntDir, devDir, tmpDir, tscriptsDir string) *Server {
	dev := sandbox.NewStore(devDir)
	dev.MaxTotalBytes = 16 * 1024 * 1024
	dev.MaxFiles = 2000

	tmp := sandbox.NewStore(tmpDir)
	tmp.MaxTotalBytes = int64(envInt("SANDBOX_TMP_SPACE_MB", 64)) * 1024 * 1024
	tmp.MaxFiles = 1000

	mnt := sandbox.NewStore(mntDir)
	mnt.MaxTotalBytes = int64(envInt("SANDBOX_MNT_SPACE_MB", 256)) * 1024 * 1024
	mnt.MaxFiles = 5000

	tscripts := sandbox.NewStore(tscriptsDir)
	tscripts.MaxTotalBytes = 16 * 1024 * 1024
	tscripts.MaxFiles = 2000

	return &Server{mnt: mnt, dev: dev, tmp: tmp, tscripts: tscripts}
}

func envInt(name string, def int) int {
	if v := os.Getenv(name); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return def
}

func (s *Server) Register(server *mcp.Server) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "sandbox_read",
		Description: "Read a saved Lua script by name, or list saved scripts when 'name' is omitted. 'name' may be '.' or a glob (e.g. '*.lua') to list matching scripts.",
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
		Description: "Run a saved Lua script by 'name', with optional 'args'. Sandboxed: no OS/process; filesystem confined to mnt/; network only via std.fetch (allowlist). Fixed 30s timeout.",
	}, s.runScript)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "sandbox_manage",
		Description: "Manage the sandbox filesystem: copy (host→sandbox), mount (sandbox→host), del, stat, and list (directory tree).",
	}, s.manage)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "sandbox_doc",
		Description: "Returns the sandbox API documentation (std.* modules, tools, limits, env vars). Pass 'topic' to get a specific section (io, fetch, secrets, ...).",
	}, s.doc)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "sandbox_diagnostics",
		Description: "Diagnose a Lua script: compile (syntax), meta, main, and std.* usage. Pass 'name' (saved script) or 'code' (inline).",
	}, s.diagnostics)
}

type readScriptInput struct {
	Name string `json:"name,omitempty" jsonschema:"Script name to read (.lua extension optional). If omitted, lists all saved scripts."`
}

type writeScriptInput struct {
	Name        string `json:"name" jsonschema:"Script file name (inside the scripts folder). The .lua extension is optional."`
	Description string `json:"description,omitempty" jsonschema:"One-line description of what the script does (optional)."`
	Code        string `json:"code" jsonschema:"Body of main(std) (wrapped automatically)."`
}

type delScriptInput struct {
	Name string `json:"name" jsonschema:"Script name to delete (.lua extension optional)."`
}

type runScriptInput struct {
	Name string `json:"name" jsonschema:"Saved script name (in the scripts folder) to run (.lua extension optional)."`
	Args string `json:"args,omitempty" jsonschema:"Script args (JSON parsed, else string)."`
}

type manageInput struct {
	Action string `json:"action" jsonschema:"Action to perform: copy (host→sandbox), mount (sandbox→host), del, stat, list."`
	Path   string `json:"path,omitempty" jsonschema:"Host source path for copy, or sandbox-relative path for mount/del/stat/list."`
	Dest   string `json:"dest,omitempty" jsonschema:"Destination: sandbox-relative path for copy, host path for mount."`
}

func (s *Server) readScript(ctx context.Context, _ *mcp.CallToolRequest, in readScriptInput) (*mcp.CallToolResult, any, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, "*?") {
		pattern := ""
		if name != "" && name != "." && name != ".." {
			pattern = name
		}
		devEntries := filterScripts(s.dev, pattern)
		tmpEntries := filterScripts(s.tscripts, pattern)
		if len(devEntries)+len(tmpEntries) == 0 {
			if pattern != "" {
				return textResult(fmt.Sprintf("Nenhum script correspondeu a `%s`.\n", pattern))
			}
			return textResult("_Nenhum script salvo ainda._")
		}
		var b strings.Builder
		if len(devEntries) > 0 {
			b.WriteString(formatScriptList("Scripts do sandbox", devEntries, scriptDescs(s.dev, devEntries)))
		}
		if len(tmpEntries) > 0 {
			if b.Len() > 0 {
				b.WriteString("\n")
			}
			b.WriteString(formatScriptList("Scripts temporários (`temp:`)", tmpEntries, scriptDescs(s.tscripts, tmpEntries)))
		}
		return textResult(b.String())
	}
	content, err := s.readScriptSource(name)
	if err != nil {
		return nil, nil, err
	}
	return textResult(formatScriptRead(name, content))
}

func (s *Server) writeScript(ctx context.Context, _ *mcp.CallToolRequest, in writeScriptInput) (*mcp.CallToolResult, any, error) {
	ref := strings.TrimSpace(in.Name)
	if ref == "" {
		return nil, nil, errors.New("'name' é obrigatório")
	}
	if strings.TrimSpace(in.Code) == "" {
		return nil, nil, errors.New("'code' é obrigatório")
	}
	isTemp := strings.HasPrefix(ref, "temp:")
	store, clean := s.scriptStore(ref)
	name := withLuaExt(clean)
	wrapped := sandbox.WrapScript(name, in.Description, in.Code)
	if _, err := store.Write(name, wrapped); err != nil {
		return nil, nil, err
	}
	display := name
	if isTemp {
		display = "temp:" + name
	}
	return textResult(formatScriptWrite(display, len(wrapped), wrapped))
}

func (s *Server) delScript(ctx context.Context, _ *mcp.CallToolRequest, in delScriptInput) (*mcp.CallToolResult, any, error) {
	ref := strings.TrimSpace(in.Name)
	if ref == "" {
		return nil, nil, errors.New("'name' é obrigatório")
	}
	store, clean := s.scriptStore(ref)
	name := withLuaExt(clean)
	for _, cand := range scriptNameVariants(name) {
		err := store.Delete(cand)
		if err == nil {
			display := cand
			if strings.HasPrefix(ref, "temp:") {
				display = "temp:" + cand
			}
			return textResult(fmt.Sprintf("Script `%s` removido.\n", display))
		}
		if !os.IsNotExist(err) {
			return nil, nil, err
		}
	}
	return nil, nil, fmt.Errorf("script %q não encontrado.", ref)
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

	res, err := sandbox.Run(s.mnt, s.tmp, sandbox.RunRequest{
		Code: code,
		Args: in.Args,
	})
	if strings.TrimSpace(res.Name) == "" {
		res.Name = name
	}
	return result(formatRunResult(res, err), false)
}

func (s *Server) manage(ctx context.Context, _ *mcp.CallToolRequest, in manageInput) (*mcp.CallToolResult, any, error) {
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
		return textResult("```text\n" + FormatTree(t) + "\n```")
	default:
		return nil, nil, fmt.Errorf("ação inválida %q; use copy, mount, del, stat ou list", action)
	}
}

func scriptDescs(store *sandbox.Store, entries []sandbox.Entry) map[string]string {
	descs := map[string]string{}
	for _, e := range entries {
		if content, err := store.Read(e.Name); err == nil {
			if _, d := sandbox.ParseMeta(content); d != "" {
				descs[e.Name] = d
			}
		}
	}
	return descs
}

func (s *Server) readScriptSource(name string) (string, error) {
	store, clean := s.scriptStore(name)
	for _, cand := range scriptNameVariants(clean) {
		if c, err := store.Read(cand); err == nil {
			return c, nil
		}
	}
	return "", fmt.Errorf("script %q não encontrado. Crie-o com sandbox_write (ou veja o que existe com sandbox_read).", name)
}

func (s *Server) scriptStore(ref string) (*sandbox.Store, string) {
	if strings.HasPrefix(ref, "temp:") {
		return s.tscripts, strings.TrimPrefix(ref, "temp:")
	}
	return s.dev, ref
}

func filterScripts(store *sandbox.Store, pattern string) []sandbox.Entry {
	entries, err := store.List()
	if err != nil {
		return nil
	}
	if pattern == "" {
		return entries
	}
	var out []sandbox.Entry
	for _, e := range entries {
		if ok, _ := filepath.Match(pattern, e.Name); ok {
			out = append(out, e)
		}
	}
	return out
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
