package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
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
		Name:        "sandbox_doc",
		Description: "Returns the sandbox API documentation (std.* modules, tools, limits, env vars). Pass 'topic' to get a specific section (io, fetch, secrets, ...).",
	}, s.doc)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "sandbox_run",
		Description: "Run a Lua script (saved by 'name' or inline 'code'). Action: 'run' (default). Inline 'code' may be just the body — it is auto-wrapped in `function main(std)`. Sandboxed: no OS/process; filesystem confined to mnt/; network only via std.fetch (allowlist). Fixed 30s timeout.",
	}, s.run)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "sandbox_scripts",
		Description: "Manage saved Lua scripts: 'list' (default), 'read', 'write', 'diagnose', 'edit', 'del'. Names may take a 'temp:' prefix for temp scripts.",
	}, s.scripts)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "sandbox_filesystem",
		Description: "Manage the sandbox filesystem: copy (host→sandbox), mount (sandbox→host), del, stat, and list (directory tree).",
	}, s.filesystem)
}

type runInput struct {
	Action string `json:"action,omitempty" jsonschema:"Action: 'run' (default)."`
	Name   string `json:"name,omitempty" jsonschema:"Saved script name (in the scripts folder) to run (.lua extension optional)."`
	Code   string `json:"code,omitempty" jsonschema:"Inline Lua code to run (used when 'name' is not given). Pass only the body — the function main(std) wrapper is added automatically if missing."`
	Args   string `json:"args,omitempty" jsonschema:"Script args (JSON parsed, else string) — used by 'run'."`
}

type scriptsInput struct {
	Action      string `json:"action,omitempty" jsonschema:"Action: 'list' (default), 'read', 'write', 'diagnose', 'edit', or 'del'."`
	Name        string `json:"name,omitempty" jsonschema:"Script name (.lua extension optional; 'temp:' prefix for temp scripts). For 'list' may be a glob (e.g. '*.lua')."`
	Description string `json:"description,omitempty" jsonschema:"One-line description of what the script does (for 'write')."`
	Code        any    `json:"code,omitempty" jsonschema:"For 'write'/'diagnose': body of main(std) (string). For 'edit': array of { line, code } to replace/remove lines."`
}

type lineEdit struct {
	Line int    `json:"line" jsonschema:"1-based line to replace/remove."`
	Code string `json:"code" jsonschema:"Replacement content (multi-line). Empty removes the line."`
}

type filesystemInput struct {
	Action string `json:"action" jsonschema:"Action to perform: copy (host→sandbox), mount (sandbox→host), del, stat, list."`
	Path   string `json:"path,omitempty" jsonschema:"Host source path for copy, or sandbox-relative path for mount/del/stat/list."`
	Dest   string `json:"dest,omitempty" jsonschema:"Destination: sandbox-relative path for copy, host path for mount."`
}

func (s *Server) run(ctx context.Context, _ *mcp.CallToolRequest, in runInput) (*mcp.CallToolResult, any, error) {
	switch action := strings.ToLower(strings.TrimSpace(in.Action)); action {
	case "", "run":
		return s.runScript(in)
	default:
		return nil, nil, fmt.Errorf("ação inválida %q; use 'run'", action)
	}
}

func (s *Server) runScript(in runInput) (*mcp.CallToolResult, any, error) {
	code := strings.TrimSpace(in.Code)
	if code == "" {
		name := strings.TrimSpace(in.Name)
		if name == "" {
			return nil, nil, errors.New("informe 'name' (script salvo) ou 'code' (inline)")
		}
		var err error
		code, err = s.readScriptSource(name)
		if err != nil {
			return nil, nil, err
		}
	}

	res, err := sandbox.Run(s.mnt, s.tmp, sandbox.RunRequest{
		Code: code,
		Args: in.Args,
	})
	if strings.TrimSpace(res.Name) == "" && strings.TrimSpace(in.Name) != "" {
		res.Name = in.Name
	}
	return result(formatRunResult(res, err), false)
}

func (s *Server) diagnostics(in scriptsInput) (*mcp.CallToolResult, any, error) {
	code, err := codeString(in.Code)
	if err != nil {
		return nil, nil, err
	}
	code = strings.TrimSpace(code)
	name := strings.TrimSpace(in.Name)
	if code == "" && name == "" {
		return nil, nil, errors.New("informe 'name' (script salvo) ou 'code' (inline)")
	}
	if code == "" {
		var err error
		code, err = s.readScriptSource(name)
		if err != nil {
			return nil, nil, err
		}
	}
	scrName, desc, diags := sandbox.Diagnose(code)
	return textResult(formatDiagnostics(scrName, desc, diags))
}

func (s *Server) scripts(ctx context.Context, _ *mcp.CallToolRequest, in scriptsInput) (*mcp.CallToolResult, any, error) {
	switch action := strings.ToLower(strings.TrimSpace(in.Action)); action {
	case "", "list", "read":
		return s.listOrReadScript(in)
	case "write":
		return s.writeScript(in)
	case "diagnose":
		return s.diagnostics(in)
	case "edit":
		return s.editScript(in)
	case "del":
		return s.delScript(in)
	default:
		return nil, nil, fmt.Errorf("ação inválida %q; use 'list', 'read', 'write', 'diagnose', 'edit' ou 'del'", action)
	}
}

func (s *Server) listOrReadScript(in scriptsInput) (*mcp.CallToolResult, any, error) {
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

func (s *Server) writeScript(in scriptsInput) (*mcp.CallToolResult, any, error) {
	ref := strings.TrimSpace(in.Name)
	if ref == "" {
		return nil, nil, errors.New("'name' é obrigatório")
	}
	code, err := codeString(in.Code)
	if err != nil {
		return nil, nil, err
	}
	if strings.TrimSpace(code) == "" {
		return nil, nil, errors.New("'code' é obrigatório")
	}
	isTemp := strings.HasPrefix(ref, "temp:")
	store, clean := s.scriptStore(ref)
	name := withLuaExt(clean)
	wrapped := sandbox.WrapScript(name, in.Description, code)
	if _, err := store.Write(name, wrapped); err != nil {
		return nil, nil, err
	}
	display := name
	if isTemp {
		display = "temp:" + name
	}
	return textResult(formatScriptWrite(display, len(wrapped), wrapped))
}

func (s *Server) delScript(in scriptsInput) (*mcp.CallToolResult, any, error) {
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

func (s *Server) editScript(in scriptsInput) (*mcp.CallToolResult, any, error) {
	ref := strings.TrimSpace(in.Name)
	if ref == "" {
		return nil, nil, errors.New("'name' é obrigatório")
	}
	edits, err := parseLineEdits(in.Code)
	if err != nil {
		return nil, nil, err
	}
	content, err := s.readScriptSource(ref)
	if err != nil {
		return nil, nil, err
	}
	lines := strings.Split(strings.TrimRight(content, "\n"), "\n")

	sort.SliceStable(edits, func(i, j int) bool { return edits[i].Line > edits[j].Line })
	for _, e := range edits {
		l := e.Line
		if l < 1 {
			continue
		}
		repl := []string{}
		if strings.TrimSuffix(e.Code, "\n") != "" {
			repl = strings.Split(strings.TrimSuffix(e.Code, "\n"), "\n")
		}
		if l > len(lines) {
			if len(repl) > 0 {
				lines = append(lines, repl...)
			}
			continue
		}
		idx := l - 1
		if len(repl) == 0 {
			lines = append(lines[:idx], lines[idx+1:]...)
			continue
		}
		out := append([]string{}, lines[:idx]...)
		out = append(out, repl...)
		out = append(out, lines[idx+1:]...)
		lines = out
	}
	updated := strings.Join(lines, "\n")

	store, clean := s.scriptStore(ref)
	name := withLuaExt(clean)
	if _, err := store.Write(name, updated); err != nil {
		return nil, nil, err
	}
	display := name
	if strings.HasPrefix(ref, "temp:") {
		display = "temp:" + name
	}
	return textResult(formatScriptWrite(display, len(updated), updated))
}

func codeString(v any) (string, error) {
	switch x := v.(type) {
	case nil:
		return "", nil
	case string:
		return x, nil
	default:
		return "", fmt.Errorf("'code' deve ser uma string")
	}
}

func parseLineEdits(v any) ([]lineEdit, error) {
	if v == nil {
		return nil, nil
	}
	arr, ok := v.([]any)
	if !ok {
		return nil, fmt.Errorf("'code' para 'edit' deve ser um array de { line, code }")
	}
	out := make([]lineEdit, 0, len(arr))
	for _, e := range arr {
		m, ok := e.(map[string]any)
		if !ok {
			continue
		}
		le := lineEdit{}
		if f, ok := m["line"].(float64); ok {
			le.Line = int(f)
		}
		if s, ok := m["code"].(string); ok {
			le.Code = s
		}
		out = append(out, le)
	}
	return out, nil
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
	return "", fmt.Errorf("script %q não encontrado. Crie-o com sandbox_scripts (write) ou veja o que existe com sandbox_scripts (list/read).", name)
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
