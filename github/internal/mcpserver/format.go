package mcpserver

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

func clean(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "|", "/")
	s = strings.ReplaceAll(s, "\t", " ")
	for strings.Contains(s, "  ") {
		s = strings.ReplaceAll(s, "  ", " ")
	}
	return strings.TrimSpace(s)
}

func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	r := []rune(s)
	if len(r) > n {
		return string(r[:n]) + "…"
	}
	return s
}

func toStr(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(t)
	case bool:
		if t {
			return "sim"
		}
		return "não"
	case float64:
		if t == float64(int64(t)) {
			return strconv.Itoa(int(t))
		}
		return strconv.FormatFloat(t, 'f', 2, 64)
	case int:
		return strconv.Itoa(t)
	case []any:
		parts := make([]string, 0, len(t))
		for _, x := range t {
			if s := toStr(x); s != "" {
				parts = append(parts, s)
			}
		}
		return strings.Join(parts, ", ")
	case map[string]any:
		for _, k := range []string{"name", "login", "title", "full_name", "message", "description", "path"} {
			if s, ok := t[k].(string); ok && s != "" {
				return strings.TrimSpace(s)
			}
		}
		return ""
	default:
		return fmt.Sprintf("%v", v)
	}
}

func toInt(v any) int {
	switch t := v.(type) {
	case float64:
		return int(t)
	case int:
		return t
	case string:
		if n, err := strconv.Atoi(strings.TrimSpace(t)); err == nil {
			return n
		}
		return 0
	default:
		return 0
	}
}

func userName(m map[string]any) string {
	for _, k := range []string{"login", "name", "email"} {
		if v, ok := m[k].(string); ok && v != "" {
			return v
		}
	}
	return ""
}

func nestedUser(m map[string]any, key string) string {
	if v, ok := m[key].(map[string]any); ok {
		return userName(v)
	}
	return ""
}

func labelsList(m map[string]any, key string) string {
	if arr, ok := m[key].([]any); ok && len(arr) > 0 {
		parts := make([]string, 0, len(arr))
		for _, it := range arr {
			if lm, ok := it.(map[string]any); ok {
				if n := toStr(lm["name"]); n != "" {
					parts = append(parts, n)
				}
			}
		}
		return strings.Join(parts, ", ")
	}
	return ""
}

func usersList(m map[string]any, key string) string {
	if arr, ok := m[key].([]any); ok && len(arr) > 0 {
		parts := make([]string, 0, len(arr))
		for _, it := range arr {
			if um, ok := it.(map[string]any); ok {
				if n := userName(um); n != "" {
					parts = append(parts, n)
				}
			}
		}
		return strings.Join(parts, ", ")
	}
	return ""
}

func firstOf(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			if s := toStr(v); s != "" {
				return s
			}
		}
	}
	return ""
}

func githubDate(value string) string {
	s := strings.TrimSpace(value)
	if s == "" {
		return ""
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02T15:04:05Z07:00", "2006-01-02"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.Format("02/01/2006 15:04")
		}
	}
	return s
}

func shortSHA(sha string) string {
	sha = strings.TrimSpace(sha)
	if len(sha) > 8 {
		return sha[:8]
	}
	return sha
}

func mapItems(items []any) []map[string]any {
	out := make([]map[string]any, 0, len(items))
	for _, it := range items {
		if m, ok := it.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

func totalHeader(total int) string {
	return fmt.Sprintf("**Total de resultados:** %d\n", total)
}

func licenseName(v any) string {
	if lm, ok := v.(map[string]any); ok {
		return toStr(lm["name"])
	}
	return toStr(v)
}

func formatRepoDetail(m map[string]any) string {
	var b strings.Builder
	name := toStr(m["full_name"])
	fmt.Fprintf(&b, "### [%s](%s)\n", name, toStr(m["html_url"]))
	pieces := []string{}
	if s := toStr(m["stargazers_count"]); s != "" && s != "0" {
		pieces = append(pieces, s+" stars")
	}
	if f := toStr(m["forks_count"]); f != "" && f != "0" {
		pieces = append(pieces, f+" forks")
	}
	if o := toStr(m["open_issues_count"]); o != "" && o != "0" {
		pieces = append(pieces, o+" issues abertas")
	}
	if l := toStr(m["language"]); l != "" {
		pieces = append(pieces, "Linguagem: "+l)
	}
	if line := metaLine(pieces...); line != "" {
		b.WriteString(line)
	}
	fmt.Fprintf(&b, "- **Privado:** %s\n", toStr(m["private"]))
	fmt.Fprintf(&b, "- **Licença:** %s\n", orMissing(licenseName(m["license"])))
	fmt.Fprintf(&b, "- **Branch padrão:** %s\n", orMissing(toStr(m["default_branch"])))
	fmt.Fprintf(&b, "- **Criado:** %s\n", githubDate(toStr(m["created_at"])))
	if u := toStr(m["updated_at"]); u != "" {
		fmt.Fprintf(&b, "- **Atualizado:** %s\n", githubDate(u))
	}
	if topics, ok := m["topics"].([]any); ok && len(topics) > 0 {
		parts := make([]string, 0, len(topics))
		for _, tp := range topics {
			parts = append(parts, toStr(tp))
		}
		fmt.Fprintf(&b, "- **Topics:** %s\n", strings.Join(parts, ", "))
	}
	if d := toStr(m["description"]); d != "" {
		fmt.Fprintf(&b, "\n%s\n", d)
	}
	return strings.TrimSpace(b.String())
}

func orMissing(s string) string {
	if s == "" {
		return "—"
	}
	return s
}

func formatIssueDetail(m map[string]any) string {
	var b strings.Builder
	fmt.Fprintf(&b, "## Issue #%s — %s\n", toStr(m["number"]), clean(toStr(m["title"])))
	fmt.Fprintf(&b, "[abrir](%s)\n\n", toStr(m["html_url"]))
	fmt.Fprintf(&b, "- **Estado:** %s\n", toStr(m["state"]))
	fmt.Fprintf(&b, "- **Autor:** %s\n", orMissing(nestedUser(m, "user")))
	if a := usersList(m, "assignees"); a != "" {
		fmt.Fprintf(&b, "- **Responsáveis:** %s\n", a)
	}
	if lb := labelsList(m, "labels"); lb != "" {
		fmt.Fprintf(&b, "- **Labels:** %s\n", lb)
	}
	fmt.Fprintf(&b, "- **Criada:** %s\n", githubDate(toStr(m["created_at"])))
	if u := toStr(m["updated_at"]); u != "" {
		fmt.Fprintf(&b, "- **Atualizada:** %s\n", githubDate(u))
	}
	fmt.Fprintf(&b, "- **Comentários:** %s\n", toStr(m["comments"]))
	if body := toStr(m["body"]); body != "" {
		fmt.Fprintf(&b, "\n---\n%s\n", body)
	}
	return strings.TrimSpace(b.String())
}

func formatPullRequestDetail(m map[string]any) string {
	var b strings.Builder
	fmt.Fprintf(&b, "## PR #%s — %s\n", toStr(m["number"]), clean(toStr(m["title"])))
	fmt.Fprintf(&b, "[abrir](%s)\n\n", toStr(m["html_url"]))
	state := toStr(m["state"])
	if state == "closed" && toStr(m["merged_at"]) != "" {
		state = "merged"
	}
	fmt.Fprintf(&b, "- **Estado:** %s\n", state)
	fmt.Fprintf(&b, "- **Autor:** %s\n", orMissing(nestedUser(m, "user")))
	if h, ok := m["head"].(map[string]any); ok {
		if lbl := toStr(h["label"]); lbl != "" {
			fmt.Fprintf(&b, "- **De:** %s", lbl)
		} else if repo, ok := h["repo"].(map[string]any); ok {
			fmt.Fprintf(&b, "- **De:** %s:%s", toStr(repo["full_name"]), toStr(h["ref"]))
		}
	}
	if b2, ok := m["base"].(map[string]any); ok {
		baseRepo := "?"
		if br, ok := b2["repo"].(map[string]any); ok {
			baseRepo = toStr(br["full_name"])
		}
		fmt.Fprintf(&b, " -> %s:%s\n", baseRepo, toStr(b2["ref"]))
	}
	if merg := toStr(m["merged_at"]); merg != "" {
		fmt.Fprintf(&b, "- **Mergeado:** %s\n", githubDate(merg))
	}
	if ma := toStr(m["mergeable"]); ma != "" {
		fmt.Fprintf(&b, "- **Mergeável:** %s\n", ma)
	}
	fmt.Fprintf(&b, "- **Commits:** %s | **+%s / -%s**\n", orMissing(toStr(m["commits"])), orMissing(toStr(m["additions"])), orMissing(toStr(m["deletions"])))
	fmt.Fprintf(&b, "- **Comentários:** %s\n", toStr(m["comments"]))
	if body := toStr(m["body"]); body != "" {
		fmt.Fprintf(&b, "\n---\n%s\n", body)
	}
	return strings.TrimSpace(b.String())
}

func metaLine(pieces ...string) string {
	parts := make([]string, 0, len(pieces))
	for _, p := range pieces {
		if strings.TrimSpace(p) != "" {
			parts = append(parts, strings.TrimSpace(p))
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, " · ") + "\n"
}

func nestedRepoName(m map[string]any) string {
	if r, ok := m["repository"].(map[string]any); ok {
		if n := toStr(r["full_name"]); n != "" {
			return n
		}
	}
	if v, ok := m["repository_url"].(string); ok && v != "" {
		parts := strings.Split(v, "/repos/")
		if len(parts) == 2 {
			return parts[1]
		}
	}
	if v, ok := m["repository"].(map[string]any); ok {
		if n := toStr(v["name"]); n != "" {
			return n
		}
	}
	return ""
}

func firstTextMatch(m map[string]any) string {
	matches, ok := m["text_matches"].([]any)
	if !ok || len(matches) == 0 {
		return ""
	}
	if mm, ok := matches[0].(map[string]any); ok {
		return clean(toStr(mm["fragment"]))
	}
	return ""
}

func formatCodeSearch(items []any, total int) string {
	var b strings.Builder
	b.WriteString(totalHeader(total))
	for _, m := range mapItems(items) {
		repo := nestedRepoName(m)
		path := toStr(m["path"])
		htmlURL := toStr(m["html_url"])
		lang := ""
		if r, ok := m["repository"].(map[string]any); ok {
			lang = toStr(r["language"])
		}
		fmt.Fprintf(&b, "### [%s — %s](%s)\n", repo, path, htmlURL)
		pieces := []string{}
		if lang != "" {
			pieces = append(pieces, "🏷️ `"+lang+"`")
		}
		if s := toStr(m["score"]); s != "" {
			pieces = append(pieces, "📊 "+s)
		}
		b.WriteString(metaLine(pieces...))
		if frag := firstTextMatch(m); frag != "" {
			fmt.Fprintf(&b, "> %s\n", truncate(frag, 400))
		}
		b.WriteString("\n")
	}
	return strings.TrimSpace(b.String())
}

func formatRepoSearch(items []any, total int) string {
	var b strings.Builder
	b.WriteString(totalHeader(total))
	for _, m := range mapItems(items) {
		name := toStr(m["full_name"])
		htmlURL := toStr(m["html_url"])
		fmt.Fprintf(&b, "### [%s](%s)\n", name, htmlURL)
		pieces := []string{}
		if s := toStr(m["stargazers_count"]); s != "" {
			pieces = append(pieces, "⭐ "+s)
		}
		if f := toStr(m["forks_count"]); f != "" {
			pieces = append(pieces, "🍴 "+f)
		}
		if l := toStr(m["language"]); l != "" {
			pieces = append(pieces, "📝 "+l)
		}
		b.WriteString(metaLine(pieces...))
		if d := clean(toStr(m["description"])); d != "" {
			fmt.Fprintf(&b, "%s\n", truncate(d, 400))
		}
		b.WriteString("\n")
	}
	return strings.TrimSpace(b.String())
}

func formatIssueSearch(items []any, total int) string {
	var b strings.Builder
	b.WriteString(totalHeader(total))
	for _, m := range mapItems(items) {
		tipo := "Issue"
		merged := false
		if pr, ok := m["pull_request"].(map[string]any); ok {
			tipo = "PR"
			if _, isMerged := pr["merged_at"]; isMerged {
				merged = true
			}
		}
		num := toStr(m["number"])
		title := clean(toStr(m["title"]))
		htmlURL := toStr(m["html_url"])
		fmt.Fprintf(&b, "### [#%s](%s) %s\n", num, htmlURL, title)
		state := clean(toStr(m["state"]))
		stateIcon := "🟢"
		switch {
		case merged:
			stateIcon = "🟣"
		case state == "closed":
			stateIcon = "🔴"
		}
		pieces := []string{"🏷️ " + tipo, stateIcon + " " + state}
		if a := nestedUser(m, "user"); a != "" {
			pieces = append(pieces, "👤 "+a)
		}
		if repo := nestedRepoName(m); repo != "" {
			pieces = append(pieces, "📦 "+repo)
		}
		b.WriteString(metaLine(pieces...))
		if labels := labelsList(m, "labels"); labels != "" {
			fmt.Fprintf(&b, "🏷️ %s\n", labels)
		}
		sub := []string{}
		if c := toStr(m["comments"]); c != "" && c != "0" {
			sub = append(sub, "💬 "+c+" comentários")
		}
		if a := usersList(m, "assignees"); a != "" {
			sub = append(sub, "👥 "+a)
		}
		if created := githubDate(toStr(m["created_at"])); created != "" {
			sub = append(sub, "🆕 "+created)
		}
		if updated := githubDate(toStr(m["updated_at"])); updated != "" {
			sub = append(sub, "🕓 "+updated)
		}
		b.WriteString(metaLine(sub...))
		if frag := firstTextMatch(m); frag != "" {
			fmt.Fprintf(&b, "> %s\n", truncate(frag, 300))
		}
		b.WriteString("\n")
	}
	return strings.TrimSpace(b.String())
}

func formatCommitSearch(items []any, total int) string {
	var b strings.Builder
	b.WriteString(totalHeader(total))
	for _, m := range mapItems(items) {
		sha := shortSHA(toStr(m["sha"]))
		htmlURL := toStr(m["html_url"])
		repo := nestedRepoName(m)
		author := nestedUser(m, "author")
		committer := nestedUser(m, "committer")
		date := ""
		msg := ""
		verified := false
		if cm, ok := m["commit"].(map[string]any); ok {
			msg = toStr(cm["message"])
			if am, ok := cm["author"].(map[string]any); ok {
				if date == "" {
					date = githubDate(toStr(am["date"]))
				}
				if author == "" {
					author = toStr(am["name"])
				}
			}
			if vm, ok := cm["verification"].(map[string]any); ok {
				verified = toStr(vm["verified"]) == "sim"
			}
		}
		parents := 0
		if ps, ok := m["parents"].([]any); ok {
			parents = len(ps)
		}
		fmt.Fprintf(&b, "### [`%s`](%s) %s\n", sha, htmlURL, clean(truncate(msg, 200)))
		pieces := []string{}
		if author != "" {
			pieces = append(pieces, "👤 "+author)
		}
		if committer != "" && committer != author {
			pieces = append(pieces, "✍️ "+committer)
		}
		if date != "" {
			pieces = append(pieces, "🕓 "+date)
		}
		if repo != "" {
			pieces = append(pieces, "📦 "+repo)
		}
		if parents > 1 {
			pieces = append(pieces, fmt.Sprintf("🔀 merge (%d pais)", parents))
		}
		if verified {
			pieces = append(pieces, "✅ assinado")
		}
		b.WriteString(metaLine(pieces...))
		if frag := firstTextMatch(m); frag != "" {
			fmt.Fprintf(&b, "> %s\n", truncate(frag, 300))
		}
		b.WriteString("\n")
	}
	return strings.TrimSpace(b.String())
}

func formatReleases(items []any) string {
	entries := mapItems(items)
	if len(entries) == 0 {
		return "Nenhum release encontrado."
	}
	var b strings.Builder
	fmt.Fprintf(&b, "**Releases:** %d\n\n", len(entries))
	for _, m := range entries {
		tag := toStr(m["tag_name"])
		name := clean(toStr(m["name"]))
		htmlURL := toStr(m["html_url"])
		title := tag
		if name != "" && name != tag {
			title = tag + " — " + name
		}
		if htmlURL != "" {
			fmt.Fprintf(&b, "### [%s](%s)\n", title, htmlURL)
		} else {
			fmt.Fprintf(&b, "### %s\n", title)
		}
		pieces := []string{}
		if t := toStr(m["target_commitish"]); t != "" {
			pieces = append(pieces, "📦 "+t)
		}
		if a := nestedUser(m, "author"); a != "" {
			pieces = append(pieces, "👤 "+a)
		}
		if p := githubDate(toStr(m["published_at"])); p != "" {
			pieces = append(pieces, "🕓 "+p)
		} else if c := githubDate(toStr(m["created_at"])); c != "" {
			pieces = append(pieces, "🕓 "+c)
		}
		b.WriteString(metaLine(pieces...))
		flags := []string{}
		if toStr(m["draft"]) == "sim" {
			flags = append(flags, "📝 rascunho")
		}
		if toStr(m["prerelease"]) == "sim" {
			flags = append(flags, "🧪 pré-release")
		}
		b.WriteString(metaLine(flags...))
		if body := clean(toStr(m["body"])); body != "" {
			fmt.Fprintf(&b, "%s\n", truncate(body, 400))
		}
		if assets, ok := m["assets"].([]any); ok && len(assets) > 0 {
			names := make([]string, 0, len(assets))
			for _, it := range assets {
				if am, ok := it.(map[string]any); ok {
					if n := toStr(am["name"]); n != "" {
						names = append(names, n)
					}
				}
			}
			if len(names) > 0 {
				fmt.Fprintf(&b, "📎 %d asset(s): %s\n", len(assets), strings.Join(names, ", "))
			}
		}
		b.WriteString("\n")
	}
	return strings.TrimSpace(b.String())
}

func githubDateUnix(sec int) string {
	if sec <= 0 {
		return ""
	}
	return time.Unix(int64(sec), 0).Format("02/01/2006")
}

func sumInts(arr []any) int {
	s := 0
	for _, v := range arr {
		s += toInt(v)
	}
	return s
}

func insightsPending(metric string) string {
	return fmt.Sprintf("GitHub ainda está calculando a métrica **%s** (resposta 202). Tente novamente em alguns segundos.", metric)
}

func formatInsights(metric string, data any) string {
	switch strings.ToLower(strings.TrimSpace(metric)) {
	case "commit_activity":
		return formatCommitActivity(data)
	case "code_frequency":
		return formatCodeFrequency(data)
	case "participation":
		return formatParticipation(data)
	case "punch_card":
		return formatPunchCard(data)
	case "contributors":
		return formatContributors(data)
	default:
		return formatContributors(data)
	}
}

func formatContributors(data any) string {
	items, ok := data.([]any)
	if !ok || len(items) == 0 {
		return "Nenhum contribuidor encontrado (ou métrica ainda sendo calculada — resposta 202; tente novamente)."
	}
	type row struct {
		login string
		total int
	}
	rows := make([]row, 0, len(items))
	for _, it := range items {
		m, ok := it.(map[string]any)
		if !ok {
			continue
		}
		login := userName(m)
		total := toInt(m["contributions"])
		if login == "" {
			continue
		}
		rows = append(rows, row{login, total})
	}
	if len(rows) == 0 {
		return "Nenhum contribuidor encontrado."
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].total > rows[j].total })
	var b strings.Builder
	fmt.Fprintf(&b, "**Contribuidores:** %d\n\n", len(rows))
	for _, r := range rows {
		fmt.Fprintf(&b, "- %s — %d commits\n", r.login, r.total)
	}
	return strings.TrimSpace(b.String())
}

func formatCommitActivity(data any) string {
	items, ok := data.([]any)
	if !ok || len(items) == 0 {
		return insightsPending("commit_activity")
	}
	var b strings.Builder
	fmt.Fprintf(&b, "**Atividade de commits (últimas %d semanas):**\n\n", len(items))
	for _, it := range items {
		m, ok := it.(map[string]any)
		if !ok {
			continue
		}
		week := githubDateUnix(toInt(m["week"]))
		total := toInt(m["total"])
		fmt.Fprintf(&b, "- %s — %d commits\n", week, total)
	}
	return strings.TrimSpace(b.String())
}

func formatCodeFrequency(data any) string {
	items, ok := data.([]any)
	if !ok || len(items) == 0 {
		return insightsPending("code_frequency")
	}
	var b strings.Builder
	b.WriteString("**Frequência de código (adições/remoções por semana):**\n\n")
	for _, it := range items {
		arr, ok := it.([]any)
		if !ok || len(arr) < 3 {
			continue
		}
		week := githubDateUnix(toInt(arr[0]))
		add := toInt(arr[1])
		del := toInt(arr[2])
		fmt.Fprintf(&b, "- %s — +%d / -%d\n", week, add, del)
	}
	return strings.TrimSpace(b.String())
}

func formatParticipation(data any) string {
	m, ok := data.(map[string]any)
	if !ok {
		return insightsPending("participation")
	}
	var b strings.Builder
	b.WriteString("**Participação semanal (últimas 52 semanas):**\n\n")
	if owner, ok := m["owner"].([]any); ok {
		fmt.Fprintf(&b, "👤 Commits do dono: %d (total %d)\n", len(owner), sumInts(owner))
	}
	if all, ok := m["all"].([]any); ok {
		fmt.Fprintf(&b, "👥 Commits de todos: %d (total %d)\n", len(all), sumInts(all))
	}
	return strings.TrimSpace(b.String())
}

func formatPunchCard(data any) string {
	items, ok := data.([]any)
	if !ok || len(items) == 0 {
		return insightsPending("punch_card")
	}
	days := []string{"Dom", "Seg", "Ter", "Qua", "Qui", "Sex", "Sáb"}
	type row struct {
		day         string
		hour, count int
	}
	rows := make([]row, 0, len(items))
	for _, it := range items {
		arr, ok := it.([]any)
		if !ok || len(arr) < 3 {
			continue
		}
		d := toInt(arr[0])
		h := toInt(arr[1])
		c := toInt(arr[2])
		if c <= 0 {
			continue
		}
		day := "?"
		if d >= 0 && d < len(days) {
			day = days[d]
		}
		rows = append(rows, row{day, h, c})
	}
	if len(rows) == 0 {
		return "Nenhum commit registrado no punch card."
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].count > rows[j].count })
	const maxRows = 24
	if len(rows) > maxRows {
		rows = rows[:maxRows]
	}
	var b strings.Builder
	fmt.Fprintf(&b, "**Punch card (top %d dias/horas com mais commits):**\n\n", len(rows))
	for _, r := range rows {
		fmt.Fprintf(&b, "- %s %02d:00 — %d commits\n", r.day, r.hour, r.count)
	}
	return strings.TrimSpace(b.String())
}

func formatUserSearch(items []any, total int) string {
	var b strings.Builder
	b.WriteString(totalHeader(total))
	for _, m := range mapItems(items) {
		login := clean(toStr(m["login"]))
		htmlURL := toStr(m["html_url"])
		typ := clean(toStr(m["type"]))
		if htmlURL != "" {
			fmt.Fprintf(&b, "### [%s](%s)\n", login, htmlURL)
		} else {
			fmt.Fprintf(&b, "### %s\n", login)
		}
		if typ != "" {
			fmt.Fprintf(&b, "🏷️ %s\n", typ)
		}
		b.WriteString("\n")
	}
	return strings.TrimSpace(b.String())
}

type fileTreeNode struct {
	name     string
	isDir    bool
	children map[string]*fileTreeNode
}

func newFileTreeRoot() *fileTreeNode {
	return &fileTreeNode{name: "", children: map[string]*fileTreeNode{}}
}

func (n *fileTreeNode) child(name string) *fileTreeNode {
	if c, ok := n.children[name]; ok {
		return c
	}
	c := &fileTreeNode{name: name, children: map[string]*fileTreeNode{}}
	n.children[name] = c
	return c
}

func countNodes(n *fileTreeNode) int {
	c := 0
	for _, ch := range n.children {
		c += 1 + countNodes(ch)
	}
	return c
}

func formatTree(items []any, total int) string {
	entries := mapItems(items)
	if len(entries) == 0 {
		return "Nenhum item encontrado."
	}

	root := newFileTreeRoot()
	dirs, files := 0, 0
	for _, m := range entries {
		path := toStr(m["path"])
		typ := toStr(m["type"])
		if path == "" {
			continue
		}
		parts := strings.Split(path, "/")
		cur := root
		for i, p := range parts {
			cur = cur.child(p)
			if i < len(parts)-1 || typ == "tree" {
				cur.isDir = true
			}
		}
		if typ == "tree" {
			dirs++
		} else {
			files++
		}
	}

	const maxTreeLines = 400
	var lines []string
	counter := 0
	var walk func(n *fileTreeNode, prefix string)
	walk = func(n *fileTreeNode, prefix string) {
		if counter >= maxTreeLines {
			return
		}
		names := make([]string, 0, len(n.children))
		for k := range n.children {
			names = append(names, k)
		}
		sort.Slice(names, func(i, j int) bool {
			ci, cj := n.children[names[i]], n.children[names[j]]
			if ci.isDir != cj.isDir {
				return ci.isDir
			}
			return names[i] < names[j]
		})
		for i, name := range names {
			if counter >= maxTreeLines {
				return
			}
			last := i == len(names)-1
			connector := "├── "
			if last {
				connector = "└── "
			}
			label := name
			if n.children[name].isDir {
				label += "/"
			}
			lines = append(lines, prefix+connector+label)
			counter++
			childPrefix := prefix
			if last {
				childPrefix += "    "
			} else {
				childPrefix += "│   "
			}
			walk(n.children[name], childPrefix)
		}
	}
	walk(root, "")

	var b strings.Builder
	fmt.Fprintf(&b, "📊 %d entradas — %d diretórios, %d arquivos", len(entries), dirs, files)
	if total > 0 && total != len(entries) {
		fmt.Fprintf(&b, " (total relatado pela API: %d)", total)
	}
	b.WriteString("\n\n```\n")
	b.WriteString(strings.Join(lines, "\n"))
	if countNodes(root) > counter {
		fmt.Fprintf(&b, "\n… lista truncada em %d linhas. Use recursive=false ou um ref/path mais específico.\n", maxTreeLines)
	}
	b.WriteString("\n```")
	return strings.TrimSpace(b.String())
}
