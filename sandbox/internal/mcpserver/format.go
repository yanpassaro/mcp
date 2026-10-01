package mcpserver

import (
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"ntdsk.com/mcp/sandbox/internal/sandbox"
)

const DEFAULT_MAX_RETURN_LINES = 500

func maxReturnLines() int {
	return envInt("SANDBOX_MAX_RETURN_LINES", DEFAULT_MAX_RETURN_LINES)
}

func limitLines(s string, max int) string {
	if max <= 0 {
		return ""
	}
	body := strings.TrimRight(s, "\n")
	if body == "" {
		return s
	}
	lines := strings.Split(body, "\n")
	if len(lines) <= max {
		return s
	}
	n := len(lines) - max
	omitted := fmt.Sprintf("%d linhas omitidas", n)
	if n == 1 {
		omitted = "1 linha omitida"
	}
	kept := strings.Join(lines[:max], "\n")
	return fmt.Sprintf("%s\n… (truncado: %s)", kept, omitted)
}

func textResult(text string) (*mcp.CallToolResult, any, error) {
	return result(text, false)
}

func result(text string, isError bool) (*mcp.CallToolResult, any, error) {
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: limitLines(text, maxReturnLines())}},
		IsError: isError,
	}, nil, nil
}

func formatRunResult(res sandbox.RunResult, runErr error) string {
	b := strings.Builder{}
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

	if failedRun(res.Ok, runErr) {
		writeRunError(&b, res, runErr)
	}
	if !failedRun(res.Ok, runErr) {
		writeRunData(&b, res)
	}

	if res.Truncated {
		b.WriteString("\n⚠️ **Saída truncada** (limite de 256 KiB excedido).\n")
	}
	return b.String()
}

func failedRun(ok bool, runErr error) bool {
	if !ok {
		return true
	}
	return runErr != nil
}

func writeRunError(b *strings.Builder, res sandbox.RunResult, runErr error) {
	msg := res.Error
	if msg == "" {
		if runErr != nil {
			msg = runErr.Error()
		}
	}
	msg = strings.ReplaceAll(msg, "```", "` ` `")
	fmt.Fprintf(b, "🔴 **Erro:** %s\n", msg)
	out := strings.TrimRight(res.Output, "\n")
	if out != "" {
		fmt.Fprintf(b, "\n```text\n%s\n```\n", out)
	}
}

func writeRunData(b *strings.Builder, res sandbox.RunResult) {
	content := strings.TrimRight(res.Data, "\n")
	if res.DataMarkdown {
		if content == "" {
			b.WriteString("_(sem resultado)_\n")
			return
		}
		b.WriteString(content)
		b.WriteString("\n")
		return
	}
	if res.DataJSON {
		if content != "" {
			fmt.Fprintf(b, "```json\n%s\n```\n", content)
			return
		}
	}
	if content == "" {
		content = strings.TrimRight(res.Output, "\n")
	}
	if content == "" {
		b.WriteString("_(sem resultado)_\n")
		return
	}
	fmt.Fprintf(b, "```text\n%s\n```\n", content)
}

func formatManageStat(name string, st sandbox.FileStat) string {
	b := strings.Builder{}
	fmt.Fprintf(&b, "## Stat `%s`\n\n", name)
	fmt.Fprintf(&b, "- **exists:** %v\n", st.Exists)
	if !st.Exists {
		b.WriteString("_não encontrado_")
		return b.String()
	}
	fmt.Fprintf(&b, "- **isDir:** %v\n", st.IsDir)
	fmt.Fprintf(&b, "- **size:** %s\n", humanSize(st.Size))
	if !st.IsDir {
		fmt.Fprintf(&b, "- **lines:** %d\n", st.Lines)
	}
	return b.String()
}

func FormatTree(root sandbox.TreeNode) string {
	b := strings.Builder{}
	writeTreeNode(&b, root, "", true, true)
	return b.String()
}

func writeTreeNode(b *strings.Builder, n sandbox.TreeNode, prefix string, isLast, isRoot bool) {
	name := n.Name
	if n.IsDir {
		name = fmt.Sprintf("%s/", name)
	}
	if !n.IsDir {
		name = fmt.Sprintf("%s  (%s, %d linhas)", name, humanSize(n.Size), n.Lines)
	}
	if isRoot {
		b.WriteString(name)
		b.WriteString("\n")
	}
	if !isRoot {
		branch, next := treeBranch(isLast, prefix)
		b.WriteString(prefix)
		b.WriteString(branch)
		b.WriteString(name)
		b.WriteString("\n")
		prefix = next
	}
	for i, c := range n.Children {
		writeTreeNode(b, c, prefix, i == len(n.Children)-1, false)
	}
}

func treeBranch(isLast bool, prefix string) (string, string) {
	if isLast {
		return "└── ", fmt.Sprintf("%s    ", prefix)
	}
	return "├── ", fmt.Sprintf("%s│   ", prefix)
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
