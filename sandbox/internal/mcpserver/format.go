package mcpserver

import (
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"ntdsk.com/mcp/sandbox/internal/sandbox"
)

func textResult(text string) (*mcp.CallToolResult, any, error) {
	return result(text, false)
}

func result(text string, isError bool) (*mcp.CallToolResult, any, error) {
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: text}},
		IsError: isError,
	}, nil, nil
}

func formatScriptList(entries []sandbox.Entry) string {
	var b strings.Builder
	b.WriteString("## Scripts do sandbox\n\n")
	if len(entries) == 0 {
		b.WriteString("_Nenhum script salvo ainda._")
		return b.String()
	}
	b.WriteString("| Script | Lines | Size |\n|---|---|---|\n")
	for _, e := range entries {
		fmt.Fprintf(&b, "| `%s` | %d | %s |\n", e.Name, e.Lines, humanSize(e.Size))
	}
	return b.String()
}

func formatScriptRead(name, content string) string {
	return fmt.Sprintf("## Script `%s`\n\n```lua\n%s\n```\n", name, content)
}

func formatRunResult(res sandbox.RunResult, runErr error) string {
	var b strings.Builder
	label := strings.TrimSpace(res.Name)
	if label == "" {
		label = "inline"
	}
	label = strings.ReplaceAll(label, "`", "")
	fmt.Fprintf(&b, "## Script `%s` · %v\n\n", label, res.Duration.Round(1_000_000))
	if res.Description != "" {
		b.WriteString("> ")
		b.WriteString(strings.ReplaceAll(res.Description, "\n", " "))
		b.WriteString("\n\n")
	}

	if !res.Ok || runErr != nil {
		msg := res.Error
		if msg == "" && runErr != nil {
			msg = runErr.Error()
		}
		msg = strings.ReplaceAll(msg, "```", "` ` `")
		fmt.Fprintf(&b, "🔴 **Erro:** %s\n", msg)
		if out := strings.TrimRight(res.Output, "\n"); out != "" {
			fmt.Fprintf(&b, "\n```text\n%s\n```\n", out)
		}
	} else {
		content := strings.TrimRight(res.Data, "\n")
		if res.DataJSON && content != "" {
			fmt.Fprintf(&b, "```json\n%s\n```\n", content)
		} else {
			if content == "" {
				content = strings.TrimRight(res.Output, "\n")
			}
			if content != "" {
				fmt.Fprintf(&b, "```text\n%s\n```\n", content)
			} else {
				b.WriteString("_(sem resultado)_\n")
			}
		}
	}

	if res.Truncated {
		b.WriteString("\n⚠️ **Saída truncada** (limite de 256 KiB excedido).\n")
	}
	return b.String()
}

func humanSize(n int64) string {
	switch {
	case n >= 1024*1024:
		return fmt.Sprintf("%.1f MiB", float64(n)/1024/1024)
	case n >= 1024:
		return fmt.Sprintf("%.1f KiB", float64(n)/1024)
	default:
		return fmt.Sprintf("%d B", n)
	}
}
