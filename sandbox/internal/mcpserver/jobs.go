package mcpserver

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"ntdsk.com/mcp/sandbox/internal/sandbox"
)

type jobInput struct {
	Action string `json:"action,omitempty" jsonschema:"Action: 'list' (default), 'status', 'logs', 'kill'."`
	ID     string `json:"id,omitempty" jsonschema:"Background job id (e.g. job-1), for status/logs/kill."`
	Tail   int    `json:"tail,omitempty" jsonschema:"logs: return only the last N lines (0 = all, truncated at the tool limit)."`
}

func (s *Server) job(ctx context.Context, _ *mcp.CallToolRequest, in jobInput) (*mcp.CallToolResult, any, error) {
	switch action := strings.ToLower(strings.TrimSpace(in.Action)); action {
	case "", "list":
		return textResult(formatJobList(s.jobs.List()))
	case "status", "get":
		info, ok := s.jobs.Get(strings.TrimSpace(in.ID))
		if !ok {
			return nil, nil, fmt.Errorf("job %q não encontrado", in.ID)
		}
		return textResult(formatJobStatus(info))
	case "logs":
		info, ok := s.jobs.Get(strings.TrimSpace(in.ID))
		if !ok {
			return nil, nil, fmt.Errorf("job %q não encontrado", in.ID)
		}
		return textResult(formatJobLogs(info, in.Tail))
	case "kill", "cancel":
		info, ok := s.jobs.Kill(strings.TrimSpace(in.ID))
		if !ok {
			return nil, nil, fmt.Errorf("job %q não encontrado", in.ID)
		}
		return textResult(formatJobStatus(info))
	default:
		return nil, nil, fmt.Errorf("ação inválida %q; use list, status, logs ou kill", action)
	}
}

func formatJobStarted(info sandbox.JobInfo) string {
	b := strings.Builder{}
	label := strings.TrimSpace(info.Name)
	if label == "" {
		label = "inline"
	}
	fmt.Fprintf(&b, "## Job `%s` iniciado · `%s`\n\n", info.ID, label)
	fmt.Fprintf(&b, "Acompanhe com `sandbox_job action=status id=%s`, `action=logs id=%s` ou `action=kill id=%s`.\n", info.ID, info.ID, info.ID)
	return b.String()
}

func formatJobList(infos []sandbox.JobInfo) string {
	b := strings.Builder{}
	fmt.Fprintf(&b, "## Jobs (%d)\n\n", len(infos))
	if len(infos) == 0 {
		b.WriteString("_(nenhum job)_\n")
		return b.String()
	}
	b.WriteString("| ID | Status | Duração | Nome | Início |\n")
	b.WriteString("| --- | --- | --- | --- | --- |\n")
	for _, info := range infos {
		label := strings.TrimSpace(info.Name)
		if label == "" {
			label = "inline"
		}
		label = strings.ReplaceAll(label, "|", "\\|")
		fmt.Fprintf(&b, "| `%s` | %s | %v | %s | %s |\n", info.ID, info.Status, jobDuration(info).Round(1_000_000), label, info.Started.Format("15:04:05"))
	}
	return b.String()
}

func formatJobStatus(info sandbox.JobInfo) string {
	b := strings.Builder{}
	fmt.Fprintf(&b, "## Job `%s` · %s\n\n", info.ID, info.Status)
	if info.Description != "" {
		fmt.Fprintf(&b, "> %s\n\n", strings.ReplaceAll(info.Description, "\n", " "))
	}
	fmt.Fprintf(&b, "- **duração:** %v\n", jobDuration(info).Round(1_000_000))
	if info.Status == sandbox.JOB_RUNNING {
		b.WriteString("_(em execução — use `sandbox_job action=logs` para acompanhar a saída)_\n")
		return b.String()
	}
	if info.Status == sandbox.JOB_CANCELLED {
		b.WriteString("_(cancelado)_\n")
		return b.String()
	}
	if info.Err != nil {
		writeRunError(&b, info.Result, info.Err)
		return b.String()
	}
	if !info.Result.Ok {
		writeRunError(&b, info.Result, nil)
		return b.String()
	}
	writeRunData(&b, info.Result)
	if info.Truncated {
		b.WriteString("\n⚠️ **Saída truncada** (limite de 256 KiB excedido).\n")
	}
	return b.String()
}

func formatJobLogs(info sandbox.JobInfo, tail int) string {
	b := strings.Builder{}
	fmt.Fprintf(&b, "## Logs `%s` · %s\n\n", info.ID, info.Status)
	out := strings.TrimRight(info.Output, "\n")
	if out == "" {
		b.WriteString("_(sem saída ainda)_\n")
		return b.String()
	}
	out = tailLines(out, tail)
	fmt.Fprintf(&b, "```text\n%s\n```\n", out)
	if info.Truncated {
		b.WriteString("\n⚠️ **Saída truncada** (limite de 256 KiB excedido).\n")
	}
	return b.String()
}

func jobDuration(info sandbox.JobInfo) time.Duration {
	if info.Status == sandbox.JOB_RUNNING {
		return time.Since(info.Started)
	}
	if info.Ended.IsZero() {
		return 0
	}
	return info.Ended.Sub(info.Started)
}

func tailLines(s string, n int) string {
	if n <= 0 {
		return s
	}
	lines := strings.Split(s, "\n")
	if len(lines) <= n {
		return s
	}
	return strings.Join(lines[len(lines)-n:], "\n")
}
