package mcpserver

import (
	"fmt"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"ntdsk.com/mcp/shell/internal/shell"
)

const maxOutputBytes = 128 * 1024

func textResult(text string) (*mcp.CallToolResult, any, error) {
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: text}},
	}, nil, nil
}

func formatRun(result shell.Result) string {
	b := strings.Builder{}
	fmt.Fprintf(&b, "🖥️ `%s`", result.Command)
	if strings.TrimSpace(result.Cwd) != "" {
		fmt.Fprintf(&b, " — `%s`", result.Cwd)
	}
	fmt.Fprintf(&b, "\n\n- Exit code: `%d`", result.ExitCode)
	if strings.TrimSpace(result.Output) != "" {
		fmt.Fprintf(&b, "\n\n```text\n%s```", codeBlock(result.Output))
	}
	return b.String()
}

func formatList(items []shell.Info) string {
	if len(items) == 0 {
		return "## Processos\n\n_Nenhum processo em background nesta sessão._"
	}
	b := strings.Builder{}
	fmt.Fprintf(&b, "## Processos (%d)\n\n", len(items))
	for _, p := range items {
		fmt.Fprintf(&b, "- **`%s`** — `%s`\n", p.ID, p.Command)
		fmt.Fprintf(&b, "  - Status: `%s` · Iniciado: `%s`", p.Status, p.Started.Format(time.RFC3339))
		if strings.TrimSpace(p.Cwd) != "" {
			fmt.Fprintf(&b, " · Cwd: `%s`", p.Cwd)
		}
		b.WriteString("\n")
	}
	return b.String()
}

func formatLog(result shell.Log) string {
	header := fmt.Sprintf("📜 `%s` — `%s`", result.ID, result.Command)
	if result.Truncated {
		header = fmt.Sprintf("%s (tail truncado)", header)
	}
	header = fmt.Sprintf("%s\n\n- Status: `%s` · Iniciado: `%s`", header, result.Status, result.Started.Format(time.RFC3339))
	if strings.TrimSpace(result.Output) == "" {
		return fmt.Sprintf("%s — sem saída ainda.", header)
	}
	return fmt.Sprintf("%s\n\n```text\n%s```", header, codeBlock(result.Output))
}

func formatTools(tools []shell.Tool, path string) string {
	b := strings.Builder{}
	fmt.Fprintf(&b, "## Tools (%s)\n\n", path)
	if len(tools) == 0 {
		b.WriteString("_Vazia (SHELL_TOOLS_FILE ausente ou vazio)._\n")
		return b.String()
	}
	for _, t := range tools {
		fmt.Fprintf(&b, "- **`shell_%s`** — %s\n  - `%s`\n", t.Name, strings.TrimSpace(t.Desc), t.Cmd)
	}
	return b.String()
}

func codeBlock(text string) string {
	block := truncateOutput(text)
	if strings.HasSuffix(block, "\n") {
		return block
	}
	return fmt.Sprintf("%s\n", block)
}

func truncateOutput(text string) string {
	if len(text) <= maxOutputBytes {
		return text
	}
	return fmt.Sprintf("%s...", text[:maxOutputBytes])
}
