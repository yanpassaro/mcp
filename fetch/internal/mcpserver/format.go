package mcpserver

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"ntdsk.com/mcp/fetch/internal/fetch"
)

func formatResponse(r *fetch.Resp, maxBody int64, htmlRaw bool, htmlMaxChars int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "## Fetch `%s %s`\n\n", r.Method, r.URL)
	size := int64(len(r.Body))
	if r.NoBody {
		if cl := r.Header.Get("Content-Length"); cl != "" {
			if n, err := strconv.ParseInt(cl, 10, 64); err == nil && n >= 0 {
				size = n
			}
		}
	}
	fmt.Fprintf(&b, "- **Status:** `%s` · **Tempo total:** %s · **Tamanho:** %s\n",
		r.Status, r.Duration.Round(time.Millisecond), sizeLabel(size))
	if line := timingLine(r); line != "" {
		fmt.Fprintf(&b, "- **Tempo (1º hop):** %s\n", line)
	}
	if len(r.Hops) > 1 {
		fmt.Fprintf(&b, "- **Redirecionamentos (%d):** %s\n", len(r.Hops)-1, strings.Join(r.Hops, " → "))
	}
	b.WriteString("\n### Headers\n\n")
	b.WriteString(formatHeaders(r.Header))
	b.WriteString("\n### Body\n\n")
	b.WriteString(bodyBlock(r, htmlRaw, htmlMaxChars))
	if r.Truncated {
		fmt.Fprintf(&b, "\n\n_(corpo truncado em %s; aumente FETCH_MAX_BODY_KB se precisar)_\n", sizeLabel(maxBody))
	}
	if r.Curl != "" {
		b.WriteString("\n### Reproduzir\n\n```bash\n")
		b.WriteString(r.Curl)
		b.WriteString("\n```\n")
	}
	return strings.TrimSpace(b.String())
}

const defaultHTMLMaxChars = 1200

func timingLine(r *fetch.Resp) string {
	if len(r.Timings) == 0 {
		return ""
	}
	t := r.Timings[0]
	if t.DNS == 0 && t.Connect == 0 && t.TLS == 0 && t.TTFB == 0 {
		return ""
	}
	var parts []string
	if t.DNS > 0 {
		parts = append(parts, "DNS "+t.DNS.Round(time.Millisecond).String())
	}
	if t.Connect > 0 {
		parts = append(parts, "Conectar "+t.Connect.Round(time.Millisecond).String())
	}
	if t.TLS > 0 {
		parts = append(parts, "TLS "+t.TLS.Round(time.Millisecond).String())
	}
	if t.TTFB > 0 {
		parts = append(parts, "Primeiro byte "+t.TTFB.Round(time.Millisecond).String())
	}
	return strings.Join(parts, " · ")
}

func clearMessage(n int, domain, name string) string {
	switch {
	case n == 0:
		return "Nenhum cookie para remover."
	case strings.TrimSpace(domain) != "" && strings.TrimSpace(name) != "":
		return fmt.Sprintf("%d cookie removido: `%s` em `%s`.", n, name, domain)
	case strings.TrimSpace(domain) != "":
		return fmt.Sprintf("%d cookie(s) removido(s) do domínio `%s`.", n, domain)
	default:
		return fmt.Sprintf("%d cookie(s) removido(s) (tudo).", n)
	}
}

func formatCookies(rows []fetch.CookieRow) string {
	if len(rows) == 0 {
		return "Nenhum cookie salvo. Faça requisições com `Set-Cookie` para salvá-los."
	}
	var b strings.Builder
	fmt.Fprintf(&b, "## Cookies (%d)\n\n", len(rows))
	b.WriteString("| Domínio | Nome | Valor | Path | Expira | Secure | HttpOnly |\n")
	b.WriteString("| --- | --- | --- | --- | --- | --- | --- |\n")
	for _, r := range rows {
		val := truncate(r.Value, 32)
		exp := "sessão"
		if !r.Expires.IsZero() {
			exp = r.Expires.Format("2006-01-02")
		}
		secure := "—"
		if r.Secure {
			secure = "✓"
		}
		httpOnly := "—"
		if r.HttpOnly {
			httpOnly = "✓"
		}
		fmt.Fprintf(&b, "| `%s` | `%s` | `%s` | `%s` | %s | %s | %s |\n",
			cell(r.Domain), cell(r.Name), cell(val), cell(r.Path), exp, secure, httpOnly)
	}
	return strings.TrimSpace(b.String())
}

var headerPriority = map[string]int{
	"location":                    0,
	"www-authenticate":            1,
	"allow":                       2,
	"retry-after":                 3,
	"content-type":                4,
	"content-length":              5,
	"content-disposition":         6,
	"etag":                        7,
	"last-modified":               8,
	"cache-control":               9,
	"expires":                     10,
	"set-cookie":                  11,
	"date":                        12,
	"server":                      13,
	"x-request-id":                14,
	"request-id":                  15,
	"access-control-allow-origin": 16,
	"vary":                        17,
	"accept-ranges":               18,
}

func headerRank(k string) (int, bool) {
	low := strings.ToLower(k)
	if p, ok := headerPriority[low]; ok {
		return p, true
	}
	flat := strings.ReplaceAll(low, "-", "")
	if strings.HasPrefix(flat, "ratelimit") || strings.HasPrefix(flat, "xratelimit") {
		return 20, true
	}
	return 0, false
}

type headerRow struct {
	name  string
	value string
	rank  int
	bold  bool
}

func formatHeaders(h http.Header) string {
	if len(h) == 0 {
		return "_Sem headers._\n"
	}
	var rows []headerRow
	var cookies []string
	for k, vs := range h {
		if strings.EqualFold(k, "Set-Cookie") {
			for _, v := range vs {
				if name := cookieName(v); name != "" {
					cookies = append(cookies, name)
				}
			}
			continue
		}
		rk, known := headerRank(k)
		rows = append(rows, headerRow{name: k, value: strings.Join(vs, ", "), rank: rk, bold: known})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].rank != rows[j].rank {
			return rows[i].rank < rows[j].rank
		}
		return strings.ToLower(rows[i].name) < strings.ToLower(rows[j].name)
	})
	var b strings.Builder
	for _, r := range rows {
		val := r.value
		if strings.EqualFold(r.name, "Date") {
			if d := prettyDate(r.value); d != "" {
				val = d
			}
		}
		name := cell(r.name)
		if r.bold {
			name = "**" + name + "**"
		}
		fmt.Fprintf(&b, "- %s: `%s`\n", name, cell(foldValue(val, 160)))
	}
	if len(cookies) > 0 {
		for i, c := range cookies {
			cookies[i] = "`" + c + "`"
		}
		fmt.Fprintf(&b, "- **Cookies (%d):** %s\n", len(cookies), strings.Join(cookies, ", "))
	}
	return b.String()
}

func cookieName(v string) string {
	// "nome=valor; Path=/; ..." → só o nome do cookie
	return strings.TrimSpace(strings.SplitN(v, "=", 2)[0])
}

func prettyDate(s string) string {
	t, err := time.Parse(http.TimeFormat, s)
	if err != nil {
		return ""
	}
	return t.Local().Format("02/01/2006 15:04:05")
}

func foldValue(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…" + fmt.Sprintf(" (+%d chars)", len(r)-n)
}

func bodyBlock(r *fetch.Resp, htmlRaw bool, htmlMaxChars int) string {
	if r.NoBody {
		return "_(corpo omitido — noBody: true)_\n"
	}
	if isBinary(r.Body, r.Header.Get("Content-Type")) {
		cl := r.Header.Get("Content-Length")
		if cl == "" {
			cl = sizeLabel(int64(len(r.Body)))
		}
		return fmt.Sprintf("_(resposta binária — %s; corpo não exibido)_\n", cl)
	}
	trimmed := strings.TrimSpace(string(r.Body))
	if trimmed == "" {
		return "_(resposta vazia)_\n"
	}
	ct := strings.ToLower(r.Header.Get("Content-Type"))
	trim := strings.ToLower(trimmed)
	switch {
	case strings.Contains(ct, "html") || strings.HasPrefix(trim, "<!doctype html") || strings.HasPrefix(trim, "<html"):
		if htmlRaw {
			return "```html\n" + string(r.Body) + "\n```\n"
		}
		if htmlMaxChars < 100 {
			htmlMaxChars = defaultHTMLMaxChars
		}
		return formatExtractedHTML(r.Body, r.URL, htmlMaxChars)
	case strings.Contains(ct, "json") || strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "["):
		if pretty, err := prettyJSON(r.Body); err == nil {
			return "```json\n" + pretty + "\n```\n"
		}
	case strings.Contains(ct, "xml") || (strings.HasPrefix(trimmed, "<") && !strings.HasPrefix(trim, "<!doctype html")):
		if pretty, err := prettyXML(r.Body); err == nil {
			return "```xml\n" + pretty + "\n```\n"
		}
	}
	return "```\n" + string(r.Body) + "\n```\n"
}

var (
	reHTMLComment = regexp.MustCompile(`(?s)<!--.*?-->`)
	reHTMLScript  = regexp.MustCompile(`(?is)<(?:script|style|noscript|template)[^>]*>.*?</(?:script|style|noscript|template)>`)
	reHTMLTitle   = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)
	reHTMLLink    = regexp.MustCompile(`(?is)<a[^>]*href\s*=\s*["']([^"']+)["'][^>]*>(.*?)</a>`)
	reHTMLBreak   = regexp.MustCompile(`(?i)</?(?:p|div|h[1-6]|li|tr|br|section|article|header|footer|blockquote)[^>]*>`)
	reHTMLTag     = regexp.MustCompile(`(?s)<[^>]+>`)
	reHTMLSpace   = regexp.MustCompile(`[\p{Zs}\t\r]+`)
	reHTMLLines   = regexp.MustCompile(`(?m)^[ \t]+|[ \t]+$|(\n)\n+`)
)

type htmlLink struct {
	Text string
	Href string
}

func formatExtractedHTML(raw []byte, base string, maxText int) string {
	title, text, links := extractHTML(raw, base, maxText)
	var b strings.Builder
	fmt.Fprintf(&b, "_HTML extraído — %s originais; use `htmlRaw` para ver o markup._", sizeLabel(int64(len(raw))))
	if title != "" {
		fmt.Fprintf(&b, "\n\n**Título:** %s", cell(title))
	}
	if text != "" {
		b.WriteString("\n\n")
		b.WriteString(text)
	}
	if len(links) > 0 {
		fmt.Fprintf(&b, "\n\n**Links (%d):**\n", len(links))
		for _, l := range links {
			fmt.Fprintf(&b, "- [%s](%s)\n", cell(l.Text), cell(l.Href))
		}
	}
	return strings.TrimSpace(b.String()) + "\n"
}

func extractHTML(raw []byte, base string, maxText int) (title, text string, links []htmlLink) {
	s := string(raw)
	links = linksFromHTML(s, base)
	if m := reHTMLTitle.FindStringSubmatch(s); len(m) == 2 {
		title = strings.TrimSpace(reHTMLSpace.ReplaceAllString(html.UnescapeString(m[1]), " "))
	}
	text = textFromHTML(s)
	if r := []rune(text); len(r) > maxText {
		text = string(r[:maxText]) + "…\n\n_(texto truncado em " + strconv.Itoa(maxText) + " chars; use `htmlRaw` para o HTML completo)_"
	}
	return title, text, links
}

func textFromHTML(s string) string {
	s = reHTMLComment.ReplaceAllString(s, " ")
	s = reHTMLScript.ReplaceAllString(s, " ")
	s = reHTMLBreak.ReplaceAllString(s, "\n")
	s = reHTMLTag.ReplaceAllString(s, " ")
	s = html.UnescapeString(s)
	s = reHTMLSpace.ReplaceAllString(s, " ")
	s = strings.ReplaceAll(s, " \n", "\n")
	s = strings.ReplaceAll(s, "\n ", "\n")
	s = reHTMLLines.ReplaceAllString(s, "$1")
	return strings.TrimSpace(s)
}

func linksFromHTML(s, base string) []htmlLink {
	s = reHTMLComment.ReplaceAllString(s, " ")
	var out []htmlLink
	seen := map[string]bool{}
	baseURL, _ := url.Parse(base)
	for _, m := range reHTMLLink.FindAllStringSubmatch(s, -1) {
		href := strings.TrimSpace(m[1])
		if href == "" || strings.HasPrefix(strings.ToLower(href), "javascript:") {
			continue
		}
		text := strings.TrimSpace(reHTMLTag.ReplaceAllString(m[2], " "))
		text = strings.TrimSpace(reHTMLSpace.ReplaceAllString(html.UnescapeString(text), " "))
		if baseURL != nil {
			if ref, err := url.Parse(href); err == nil {
				if resolved := baseURL.ResolveReference(ref); resolved != nil {
					href = resolved.String()
				}
			}
		}
		if seen[href] {
			continue
		}
		seen[href] = true
		out = append(out, htmlLink{Text: truncate(text, 80), Href: href})
		if len(out) >= 20 {
			break
		}
	}
	return out
}

func isBinary(body []byte, contentType string) bool {
	ct := strings.ToLower(contentType)
	if ct != "" {
		if strings.HasPrefix(ct, "image/") || strings.HasPrefix(ct, "audio/") ||
			strings.HasPrefix(ct, "video/") || strings.HasPrefix(ct, "font/") ||
			strings.Contains(ct, "octet-stream") || strings.Contains(ct, "binary") ||
			ct == "application/pdf" || ct == "application/zip" || ct == "application/gzip" ||
			ct == "application/x-tar" || strings.Contains(ct, "application/vnd.openxmlformats") {
			return true
		}
	}
	head := body
	if len(head) > 1024 {
		head = head[:1024]
	}
	return bytes.IndexByte(head, 0) >= 0
}

func prettyJSON(b []byte) (string, error) {
	var buf bytes.Buffer
	if err := json.Indent(&buf, b, "", "  "); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func prettyXML(b []byte) (string, error) {
	dec := xml.NewDecoder(bytes.NewReader(b))
	var buf bytes.Buffer
	enc := xml.NewEncoder(&buf)
	enc.Indent("", "  ")
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
		if err := enc.EncodeToken(tok); err != nil {
			return "", err
		}
	}
	if err := enc.Flush(); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func sizeLabel(n int64) string {
	if n < 1024 {
		return strconv.FormatInt(n, 10) + " B"
	}
	return strconv.FormatFloat(float64(n)/1024, 'f', 1, 64) + " KB"
}

func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	r := []rune(s)
	if len(r) > n {
		return string(r[:n]) + "…"
	}
	return s
}

func formatSummary(r *fetch.Resp) string {
	var b strings.Builder
	fmt.Fprintf(&b, "## Fetch `%s %s`\n\n", r.Method, r.URL)
	size := int64(len(r.Body))
	if r.NoBody {
		if cl := r.Header.Get("Content-Length"); cl != "" {
			if n, err := strconv.ParseInt(cl, 10, 64); err == nil && n >= 0 {
				size = n
			}
		}
	}
	fmt.Fprintf(&b, "- **Status:** `%s` · **Tempo total:** %s · **Tamanho:** %s\n", r.Status, r.Duration.Round(time.Millisecond), sizeLabel(size))
	if ct := r.Header.Get("Content-Type"); ct != "" {
		fmt.Fprintf(&b, "- **Content-Type:** %s\n", ct)
	}
	if line := timingLine(r); line != "" {
		fmt.Fprintf(&b, "- **Tempo (1º hop):** %s\n", line)
	}
	if r.Curl != "" {
		b.WriteString("\n### Reproduzir\n\n```bash\n")
		b.WriteString(r.Curl)
		b.WriteString("\n```\n")
	}
	return strings.TrimSpace(b.String())
}

func cell(s string) string {
	s = strings.ReplaceAll(s, "|", "\\|")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\t", " ")
	return s
}
