package mcpserver

import (
	"fmt"
	"strings"

	"ntdsk.com/mcp/search/internal/search"
)

func tableCell(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "|", "\\|")
	return strings.TrimSpace(s)
}

func formatResultBody(r search.SearchResult) string {
	var parts []string
	if len(r.Highlights) > 0 {
		for _, h := range r.Highlights {
			if strings.TrimSpace(h) == "" {
				continue
			}
			parts = append(parts, "- "+tableCell(h))
		}
	}
	if len(parts) == 0 {
		if r.Text != "" {
			parts = append(parts, r.Text)
		} else if r.Summary != "" {
			parts = append(parts, r.Summary)
		}
	}
	return strings.Join(parts, "\n")
}

func formatSearch(resp search.SearchResponse) string {
	var b strings.Builder
	if len(resp.Results) == 0 {
		return "Nenhum resultado encontrado."
	}
	fmt.Fprintf(&b, "**Resultados** (%d)\n\n", len(resp.Results))
	for i, r := range resp.Results {
		fmt.Fprintf(&b, "%d. **%s**\n", i+1, tableCell(r.Title))
		if r.URL != "" {
			fmt.Fprintf(&b, "   %s\n", r.URL)
		}
		if r.Favicon != "" {
			fmt.Fprintf(&b, "   🌐 %s\n", r.Favicon)
		}
		if r.PublishedDate != "" {
			fmt.Fprintf(&b, "   📅 %s\n", r.PublishedDate)
		}
		if r.Author != "" {
			fmt.Fprintf(&b, "   ✍️ %s\n", r.Author)
		}
		if r.Score > 0 {
			fmt.Fprintf(&b, "   🎯 %.2f\n", r.Score)
		}
		if body := formatResultBody(r); body != "" {
			fmt.Fprintf(&b, "\n%s\n", body)
		}
		if len(r.Extras.Links) > 0 {
			fmt.Fprintf(&b, "   🔗 %s\n", strings.Join(r.Extras.Links, ", "))
		}
		b.WriteString("\n")
	}
	return strings.TrimSpace(b.String())
}

func formatContents(resp search.ContentsResponse) string {
	var b strings.Builder
	if len(resp.Results) == 0 && len(resp.Statuses) == 0 {
		return "Nenhum conteúdo encontrado."
	}

	var failures []string
	for _, st := range resp.Statuses {
		if st.Status != "success" {
			tag := ""
			if st.Error != nil {
				tag = st.Error.Tag
			}
			failures = append(failures, fmt.Sprintf("- %s (%s)", st.ID, tag))
		}
	}
	if len(failures) > 0 {
		fmt.Fprintf(&b, "**Falhas** (%d):\n%s\n\n", len(failures), strings.Join(failures, "\n"))
	}

	if len(resp.Results) == 0 {
		return strings.TrimSpace(b.String())
	}

	fmt.Fprintf(&b, "**Conteúdo** (%d)\n\n", len(resp.Results))
	for i, r := range resp.Results {
		fmt.Fprintf(&b, "%d. **%s**\n", i+1, tableCell(r.Title))
		if r.URL != "" {
			fmt.Fprintf(&b, "%s\n\n", r.URL)
		}
		if body := formatResultBody(r); body != "" {
			b.WriteString(body)
			b.WriteString("\n\n")
		} else {
			b.WriteString("_Sem conteúdo disponível._\n\n")
		}
	}
	return strings.TrimSpace(b.String())
}
