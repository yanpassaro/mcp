package sqlize

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type profileInput struct {
	Query  string `json:"query,omitempty" jsonschema:"Source SELECT/WITH query (alternative to 'table'). This tool has no 'args', so keep it free of WHERE value filters — prefer 'table' for a plain table."`
	Table  string `json:"table,omitempty" jsonschema:"Source table name (alternative to 'query')."`
	Limit  int    `json:"limit,omitempty" jsonschema:"Max rows profiled (default 1000, cap 10000)."`
	Top    int    `json:"top,omitempty" jsonschema:"Max most-frequent values shown per column (default 5, cap 20)."`
	Redact *bool  `json:"redact,omitempty" jsonschema:"Mask the sample values shown in the output. Default: true."`
}

func (s *Server) profileTool(ctx context.Context, _ *mcp.CallToolRequest, in profileInput) (*mcp.CallToolResult, any, error) {
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
	top := in.Top
	if top <= 0 {
		top = 5
	}
	if top > 20 {
		top = 20
	}

	q := fmt.Sprintf("SELECT * FROM (%s) _src LIMIT %d", src, limit)
	cols, rows, err := s.store.query(ctx, q)
	if err != nil {
		return nil, nil, fmt.Errorf("perfil: %w", err)
	}
	doRedact := in.Redact == nil || *in.Redact
	return textResult(profileMarkdown(cols, rows, doRedact, top, limit))
}

func profileMarkdown(cols []string, rows [][]string, redact bool, top, limit int) string {
	if len(cols) == 0 {
		return "Sem colunas."
	}
	total := len(rows)
	var b strings.Builder
	note := ""
	if total >= limit {
		note = fmt.Sprintf(" (limitado a %d linhas)", limit)
	}
	fmt.Fprintf(&b, "## Perfil — %d linha(s)%s\n", total, note)
	for ci, col := range cols {
		vals := make([]string, len(rows))
		for i, r := range rows {
			if ci < len(r) {
				vals[i] = r[ci]
			}
		}
		fmt.Fprintf(&b, "\n### %s\n", col)
		b.WriteString(profileColumn(col, vals, redact, top))
	}
	return b.String()
}

func profileColumn(col string, vals []string, redact bool, top int) string {
	nonBlank := 0
	distinct := map[string]struct{}{}
	freq := map[string]int{}
	allNum := true
	hasNum := false
	var minF, maxF, sum float64
	pii := 0
	for _, v := range vals {
		if v == "" {
			continue
		}
		nonBlank++
		distinct[v] = struct{}{}
		freq[v]++
		if redactCell(col, v) != v {
			pii++
		}
		if allNum {
			f, err := strconv.ParseFloat(v, 64)
			if err != nil {
				allNum = false
			} else {
				if !hasNum {
					minF, maxF = f, f
					hasNum = true
				} else {
					if f < minF {
						minF = f
					}
					if f > maxF {
						maxF = f
					}
				}
				sum += f
			}
		}
	}

	var b strings.Builder
	pct := func(n int) float64 {
		if nonBlank == 0 {
			return 0
		}
		return float64(n) / float64(nonBlank) * 100
	}
	fmt.Fprintf(&b, "- nulos/vazios: %d\n", len(vals)-nonBlank)
	fmt.Fprintf(&b, "- distintos: %d\n", len(distinct))
	if allNum && nonBlank > 0 {
		fmt.Fprintf(&b, "- min/max/média: %s / %s / %s\n",
			strconv.FormatFloat(minF, 'f', -1, 64),
			strconv.FormatFloat(maxF, 'f', -1, 64),
			strconv.FormatFloat(sum/float64(nonBlank), 'f', -1, 64))
	} else if nonBlank > 0 {
		mins, maxs := vals[0], vals[0]
		for _, v := range vals {
			if v == "" {
				continue
			}
			if v < mins {
				mins = v
			}
			if v > maxs {
				maxs = v
			}
		}
		fmt.Fprintf(&b, "- min/max: %s / %s\n", mins, maxs)
	}
	fmt.Fprintf(&b, "- PII (valor seria mascarado): %d (%.0f%%)\n", pii, pct(pii))

	if top > 0 && len(freq) > 0 {
		type kv struct {
			val string
			n   int
		}
		lst := make([]kv, 0, len(freq))
		for v, n := range freq {
			lst = append(lst, kv{v, n})
		}
		sort.Slice(lst, func(i, j int) bool {
			if lst[i].n != lst[j].n {
				return lst[i].n > lst[j].n
			}
			return lst[i].val > lst[j].val
		})
		if len(lst) > top {
			lst = lst[:top]
		}
		b.WriteString("- valores mais comuns:\n")
		for _, e := range lst {
			shown := e.val
			if redact {
				shown = redactCell(col, e.val)
			}
			fmt.Fprintf(&b, "  - %s (%d)\n", shown, e.n)
		}
	}
	return b.String()
}
