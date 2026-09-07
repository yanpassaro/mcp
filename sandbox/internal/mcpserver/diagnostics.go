package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"ntdsk.com/mcp/sandbox/internal/sandbox"
)

type diagnosticsInput struct {
	Name string `json:"name,omitempty" jsonschema:"Script name to diagnose (reads from dev/). If omitted, 'code' is used."`
	Code string `json:"code,omitempty" jsonschema:"Inline Lua code to diagnose (used when 'name' is omitted)."`
}

func (s *Server) diagnostics(ctx context.Context, _ *mcp.CallToolRequest, in diagnosticsInput) (*mcp.CallToolResult, any, error) {
	code := strings.TrimSpace(in.Code)
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

func formatDiagnostics(name, desc string, diags []sandbox.Diag) string {
	var b strings.Builder
	b.WriteString("## Diagnóstico do script\n\n")
	if name != "" {
		fmt.Fprintf(&b, "- **nome:** `%s`\n", name)
	}
	if desc != "" {
		fmt.Fprintf(&b, "- **descrição:** %s\n", strings.ReplaceAll(desc, "\n", " "))
	}
	if len(diags) == 0 {
		b.WriteString("\n✅ Nenhum problema encontrado.\n")
		return b.String()
	}
	b.WriteString("\n")
	for _, d := range diags {
		sym := "⚠️"
		if d.Severity == "erro" {
			sym = "🔴"
		}
		fmt.Fprintf(&b, "- %s **%s:** %s\n", sym, strings.ToUpper(d.Severity), d.Message)
	}
	return b.String()
}
