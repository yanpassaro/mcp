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
		if res.DataMarkdown {
			if content != "" {
				b.WriteString(content)
				b.WriteString("\n")
			} else {
				b.WriteString("_(sem resultado)_\n")
			}
		} else if res.DataJSON && content != "" {
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

func formatManageStat(name string, st sandbox.FileStat) string {
	var b strings.Builder
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
	var b strings.Builder
	writeTreeNode(&b, root, "", true, true)
	return b.String()
}

func writeTreeNode(b *strings.Builder, n sandbox.TreeNode, prefix string, isLast, isRoot bool) {
	name := n.Name
	if n.IsDir {
		name += "/"
	} else {
		name += fmt.Sprintf("  (%s, %d linhas)", humanSize(n.Size), n.Lines)
	}
	if isRoot {
		b.WriteString(name)
		b.WriteString("\n")
	} else {
		branch, next := "├── ", prefix+"│   "
		if isLast {
			branch, next = "└── ", prefix+"    "
		}
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
