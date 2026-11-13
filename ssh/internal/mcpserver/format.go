package mcpserver

import (
	"fmt"
	"strings"

	"ntdsk.com/mcp/ssh/internal/ssh"
)

const maxOutputBytes = 128 * 1024

func formatRun(cfg ssh.Config, command string, result ssh.Result) string {
	b := strings.Builder{}
	fmt.Fprintf(&b, "🖥️ `%s` — `%s`\n\n- Exit code: `%d`", ssh.Label(cfg.Alias), command, result.ExitCode)
	if strings.TrimSpace(result.Stdout) != "" {
		fmt.Fprintf(&b, "\n\n**stdout**\n\n```text\n%s```", codeBlock(result.Stdout))
	}
	if strings.TrimSpace(result.Stderr) != "" {
		fmt.Fprintf(&b, "\n\n**stderr**\n\n```text\n%s```", codeBlock(result.Stderr))
	}
	return b.String()
}

func formatRead(cfg ssh.Config, path, text string, truncated bool) string {
	header := fmt.Sprintf("📄 `%s:%s`", ssh.Label(cfg.Alias), path)
	if truncated {
		header = fmt.Sprintf("%s (truncado)", header)
	}
	if strings.TrimSpace(text) == "" {
		return fmt.Sprintf("%s — arquivo vazio.", header)
	}
	return fmt.Sprintf("%s\n\n```text\n%s```", header, codeBlock(text))
}

func formatWrite(cfg ssh.Config, path string, written int64) string {
	return fmt.Sprintf("💾 `%s:%s` — %d bytes gravados.", ssh.Label(cfg.Alias), path, written)
}

func formatTransfer(direction, local, remote string, size int64, truncated bool) string {
	header := fmt.Sprintf("🔁 `%s` — %s (`%s` ↔ `%s`)", direction, humanBytes(size), local, remote)
	if truncated {
		header = fmt.Sprintf("%s (truncado)", header)
	}
	return header
}

func formatProbe(cfg ssh.Config, text string) string {
	header := fmt.Sprintf("🔎 `%s`", ssh.Label(cfg.Alias))
	if strings.TrimSpace(text) == "" {
		return fmt.Sprintf("%s — sem saída.", header)
	}
	return fmt.Sprintf("%s\n\n```text\n%s```", header, codeBlock(text))
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

func humanBytes(value int64) string {
	units := []string{"B", "KiB", "MiB", "GiB"}
	scaled := float64(value)
	unit := 0
	for scaled >= 1024 {
		if unit >= len(units)-1 {
			break
		}
		scaled /= 1024
		unit++
	}
	return fmt.Sprintf("%.1f %s", scaled, units[unit])
}
