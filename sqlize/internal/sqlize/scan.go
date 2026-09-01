package sqlize

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type scanInput struct {
	Query string `json:"query,omitempty" jsonschema:"Source SELECT/WITH query (alternative to 'table'). This tool has no 'args', so keep it free of WHERE value filters — prefer 'table' for a plain table."`
	Table string `json:"table,omitempty" jsonschema:"Source table name (alternative to 'query')."`
	Limit int    `json:"limit,omitempty" jsonschema:"Max rows scanned (default 1000, cap 10000)."`
}

func (s *Server) scanTool(ctx context.Context, _ *mcp.CallToolRequest, in scanInput) (*mcp.CallToolResult, any, error) {
	src, err := compareSource(in.Table, in.Query, "table/query")
	if err != nil {
		return nil, nil, err
	}
	limit := in.Limit
	if limit <= 0 {
		limit = 1000
	}
	if limit > 10000 {
		limit = 10000
	}
	q := fmt.Sprintf("SELECT * FROM (%s) _src LIMIT %d", src, limit)
	cols, rows, err := s.store.query(ctx, q)
	if err != nil {
		return nil, nil, fmt.Errorf("scan: %w", err)
	}
	return textResult(scanMarkdown(cols, rows, limit))
}

func scanMarkdown(cols []string, rows [][]string, limit int) string {
	if len(cols) == 0 {
		return "Sem colunas."
	}
	total := len(rows)
	var b strings.Builder
	note := ""
	if total >= limit {
		note = fmt.Sprintf(" (limitado a %d linhas)", limit)
	}
	fmt.Fprintf(&b, "## Inventário PII — %d linha(s)%s\n", total, note)

	anyHit := false
	for ci, col := range cols {
		counts := map[string]int{}
		nonBlank := 0
		for _, r := range rows {
			cell := ""
			if ci < len(r) {
				cell = r[ci]
			}
			if cell == "" {
				continue
			}
			nonBlank++
			counted := false
			for _, sp := range analyzeCell(col, cell) {
				if sp.score >= maskThreshold {
					counts[sp.entity]++
					counted = true
				}
			}
			if !counted {
				if ent, ok := columnEntity(col); ok {
					counts[ent]++
				}
			}
		}
		if len(counts) == 0 {
			continue
		}
		anyHit = true
		fmt.Fprintf(&b, "\n### %s (não-vazias: %d)\n", col, nonBlank)
		ents := make([]string, 0, len(counts))
		for e := range counts {
			ents = append(ents, e)
		}
		sort.Strings(ents)
		for _, e := range ents {
			n := counts[e]
			pct := 0.0
			if nonBlank > 0 {
				pct = float64(n) / float64(nonBlank) * 100
			}
			fmt.Fprintf(&b, "- %s: %d (%.0f%%)\n", e, n, pct)
		}
	}
	if !anyHit {
		b.WriteString("\nNenhuma PII detectada nas colunas analisadas.\n")
	}
	return b.String()
}
