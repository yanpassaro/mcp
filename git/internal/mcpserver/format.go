package mcpserver

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/plumbing"
	"github.com/go-git/go-git/v6/plumbing/object"
)

const maxDiffBytes = 200 * 1024

var convRe = regexp.MustCompile(`^(\w+)(?:\([^)]*\))?:\s*(.+)$`)

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

func shortSHA(h plumbing.Hash) string {
	s := h.String()
	if len(s) > 7 {
		return s[:7]
	}
	return s
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	return strings.TrimSpace(s)
}

func statusLabel(code git.StatusCode) string {
	switch code {
	case git.Unmodified:
		return "—"
	case git.Untracked:
		return "?"
	case git.Modified:
		return "M"
	case git.Added:
		return "A"
	case git.Deleted:
		return "D"
	case git.Renamed:
		return "R"
	case git.Copied:
		return "C"
	case git.UpdatedButUnmerged:
		return "U"
	default:
		return "?"
	}
}

func formatStatus(st git.Status) string {
	index := make([]string, 0)
	worktree := make([]string, 0)
	for f := range st {
		fs := st[f]
		if fs.Staging != git.Unmodified {
			index = append(index, fmt.Sprintf("- `%s` — %s", clean(f), statusLabel(fs.Staging)))
		}
		if fs.Worktree != git.Unmodified {
			worktree = append(worktree, fmt.Sprintf("- `%s` — %s", clean(f), statusLabel(fs.Worktree)))
		}
	}
	sort.Strings(index)
	sort.Strings(worktree)
	var b strings.Builder
	b.WriteString("## Status\n\n")
	if len(index) == 0 && len(worktree) == 0 {
		b.WriteString("_Árvore de trabalho limpa (sem alterações)._")
		return b.String()
	}
	if len(index) > 0 {
		b.WriteString("### 📦 Index (staged)\n")
		b.WriteString(strings.Join(index, "\n"))
		b.WriteByte('\n')
	}
	if len(worktree) > 0 {
		if len(index) > 0 {
			b.WriteString("\n")
		}
		b.WriteString("### 🛠 Working tree\n")
		b.WriteString(strings.Join(worktree, "\n"))
		b.WriteByte('\n')
	}
	return b.String()
}

func formatLog(commits []*object.Commit) string {
	if len(commits) == 0 {
		return "## Log\n\n_Nenhum commit encontrado para os filtros informados._"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "## Log (%d commits)\n\n", len(commits))
	for _, c := range commits {
		fmt.Fprintf(&b, "- `%s` **%s** — %s · %s\n", shortSHA(c.Hash), clean(truncate(firstLine(c.Message), 80)), clean(c.Author.Name), c.Author.When.Format("2006-01-02"))
	}
	return b.String()
}

const maxStatFiles = 40

func formatLogStat(commits []*object.Commit) string {
	if len(commits) == 0 {
		return "## Log\n\n_Nenhum commit encontrado para os filtros informados._"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "## Log --stat (%d commits)\n\n", len(commits))
	for _, c := range commits {
		fmt.Fprintf(&b, "- `%s` **%s** — %s · %s\n", shortSHA(c.Hash), clean(truncate(firstLine(c.Message), 80)), clean(c.Author.Name), c.Author.When.Format("2006-01-02"))
		stats, err := c.Stats()
		if err != nil || len(stats) == 0 {
			continue
		}
		if len(stats) > maxStatFiles {
			stats = stats[:maxStatFiles]
		}
		for _, s := range stats {
			fmt.Fprintf(&b, "  - %s — +%d / -%d\n", clean(s.Name), s.Addition, s.Deletion)
		}
	}
	return b.String()
}

func formatShow(c *object.Commit, patch *object.Patch) string {
	var b strings.Builder
	fmt.Fprintf(&b, "## Commit `%s`\n\n", shortSHA(c.Hash))
	fmt.Fprintf(&b, "- **Autor:** %s <%s>\n", c.Author.Name, c.Author.Email)
	fmt.Fprintf(&b, "- **Committer:** %s <%s>\n", c.Committer.Name, c.Committer.Email)
	fmt.Fprintf(&b, "- **Data:** %s\n", c.Author.When.Format(time.RFC1123))
	fmt.Fprintf(&b, "- **Pais:** %d\n\n", len(c.ParentHashes))
	fmt.Fprintf(&b, "%s\n\n", strings.TrimSpace(c.Message))
	if stats, err := c.Stats(); err == nil && len(stats) > 0 {
		totalA, totalD := 0, 0
		for _, s := range stats {
			totalA += s.Addition
			totalD += s.Deletion
		}
		fmt.Fprintf(&b, "📊 **%d** arquivo(s) alterado(s): **+%d / -%d**\n\n", len(stats), totalA, totalD)
	}
	if patch != nil {
		d := patch.String()
		if len(d) > maxDiffBytes {
			d = d[:maxDiffBytes] + "\n…(diff truncado)"
		}
		fmt.Fprintf(&b, "### Diff\n\n```diff\n%s\n```\n", d)
	}
	return b.String()
}

func formatPatch(patch *object.Patch, title string, context int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "## %s\n\n", title)
	stats := patch.Stats()
	if len(stats) > 0 {
		totalA, totalD := 0, 0
		for _, s := range stats {
			totalA += s.Addition
			totalD += s.Deletion
		}
		fmt.Fprintf(&b, "📊 **%d** arquivo(s) · **+%d / -%d**\n\n", len(stats), totalA, totalD)
	}
	text := patch.String()
	if context >= 0 {
		text = compactPatchText(text, context)
	}
	if len(text) > maxDiffBytes {
		text = text[:maxDiffBytes] + "\n…(diff truncado)"
	}
	if strings.TrimSpace(text) == "" {
		b.WriteString("_Sem diferenças._\n")
		return b.String()
	}
	fmt.Fprintf(&b, "```diff\n%s\n```\n", text)
	return b.String()
}

type statRow struct {
	name string
	add  int
	del  int
}

func formatPatchStat(patch *object.Patch, title string) string {
	stats := patch.Stats()
	rows := make([]statRow, 0, len(stats))
	totalA, totalD := 0, 0
	for _, s := range stats {
		rows = append(rows, statRow{name: s.Name, add: s.Addition, del: s.Deletion})
		totalA += s.Addition
		totalD += s.Deletion
	}
	return formatStatRows(title, rows, totalA, totalD)
}

func formatWorkingDiffStat(title string, rows []statRow, added, deleted int) string {
	return formatStatRows(title, rows, added, deleted)
}

func formatStatRows(title string, rows []statRow, added, deleted int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "## %s\n\n", title)
	if len(rows) == 0 {
		b.WriteString("_Sem diferenças._\n")
		return b.String()
	}
	fmt.Fprintf(&b, "📊 **%d** arquivo(s) · **+%d / -%d**\n\n", len(rows), added, deleted)
	for _, r := range rows {
		fmt.Fprintf(&b, "- %s — +%d / -%d\n", clean(r.name), r.add, r.del)
	}
	return b.String()
}

type contributorRow struct {
	Name  string
	Count int
}

func formatContributors(rows []contributorRow, total int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "## 🏆 Maiores contribuidores — %s commits\n\n", formatThousands(total))
	if len(rows) == 0 {
		b.WriteString("_Sem contribuidores._\n")
		return b.String()
	}
	max := rows[0].Count
	if max <= 0 {
		max = 1
	}
	const barWidth = 20
	medals := []string{"🥇", "🥈", "🥉"}
	for i, r := range rows {
		prefix := "  "
		if i < len(medals) {
			prefix = medals[i] + " "
		}
		filled := int(float64(r.Count) / float64(max) * barWidth)
		if r.Count > 0 && filled < 1 {
			filled = 1
		}
		bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)
		pct := int(float64(r.Count)/float64(total)*100 + 0.5)
		commitWord := "commits"
		if r.Count == 1 {
			commitWord = "commit"
		}
		fmt.Fprintf(&b, "%s **%s** — %s %s\n", prefix, clean(r.Name), formatThousands(r.Count), commitWord)
		fmt.Fprintf(&b, "   `%s` %d%%\n\n", bar, pct)
	}
	return b.String()
}

func formatThousands(n int) string {
	s := strconv.Itoa(n)
	if len(s) <= 3 {
		return s
	}
	var parts []string
	for len(s) > 3 {
		parts = append([]string{s[len(s)-3:]}, parts...)
		s = s[:len(s)-3]
	}
	parts = append([]string{s}, parts...)
	return strings.Join(parts, ".")
}

type branchRow struct {
	Name    string
	Head    string
	Current bool
}

func formatBranches(rows []branchRow) string {
	if len(rows) == 0 {
		return "## Branches\n\n_Nenhuma branch encontrada._"
	}
	var b strings.Builder
	b.WriteString("## Branches\n\n")
	for _, r := range rows {
		mark := "  "
		if r.Current {
			mark = "▶ "
		}
		fmt.Fprintf(&b, "- %s**%s** — `%s`\n", mark, clean(r.Name), r.Head)
	}
	return b.String()
}

type tagRow struct {
	Name string
	SHA  string
	Date string
}

func formatTags(rows []tagRow) string {
	if len(rows) == 0 {
		return "## Tags\n\n_Nenhuma tag encontrada._"
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Date > rows[j].Date })
	var b strings.Builder
	b.WriteString("## Tags\n\n")
	for _, r := range rows {
		fmt.Fprintf(&b, "- **%s** — `%s` · %s\n", clean(r.Name), r.SHA, r.Date)
	}
	return b.String()
}

type remoteRow struct {
	Name string
	URLs string
}

func formatRemotes(rows []remoteRow) string {
	if len(rows) == 0 {
		return "## Remotes\n\n_Nenhum remote configurado._"
	}
	var b strings.Builder
	b.WriteString("## Remotes\n\n")
	for _, r := range rows {
		fmt.Fprintf(&b, "- **%s** — %s\n", clean(r.Name), clean(r.URLs))
	}
	return b.String()
}

func formatBlame(lines []string, blame []blameLine, maxLines int) string {
	limit := len(lines)
	if maxLines > 0 && maxLines < limit {
		limit = maxLines
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "## Blame (%d linhas)\n\n", len(lines))
	for i := 0; i < limit; i++ {
		b := blame[i]
		fmt.Fprintf(&sb, "%s %s | %s\n", b.SHA, b.Author, strings.TrimRight(lines[i], "\n"))
	}
	return sb.String()
}

type blameLine struct {
	SHA    string
	Author string
	Date   string
}

func formatLsFiles(paths []string) string {
	if len(paths) == 0 {
		return "## Arquivos rastreados\n\n_Nenhum arquivo._"
	}
	sort.Strings(paths)
	return fmt.Sprintf("## Arquivos rastreados (%d)\n\n```\n%s\n```\n", len(paths), strings.Join(paths, "\n"))
}

func formatRevParse(rev, full, short, author, msg string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "## Rev-parse: `%s`\n\n", rev)
	fmt.Fprintf(&b, "- **SHA completo:** `%s`\n", full)
	fmt.Fprintf(&b, "- **SHA curto:** `%s`\n", short)
	if author != "" {
		fmt.Fprintf(&b, "- **Autor:** %s\n", clean(author))
	}
	if msg != "" {
		fmt.Fprintf(&b, "- **Mensagem:** %s\n", clean(truncate(msg, 120)))
	}
	return b.String()
}

func formatCheckIgnore(path string, ignored bool, note string) string {
	if note != "" {
		return fmt.Sprintf("## Check-ignore\n\n`%s` — **%s**.", clean(path), clean(note))
	}
	if ignored {
		return fmt.Sprintf("## Check-ignore\n\n`%s` — **ignorado** (`.gitignore`).", clean(path))
	}
	return fmt.Sprintf("## Check-ignore\n\n`%s` — **não ignorado**.\n", clean(path))
}

func emojiTitle(typ string) string {
	switch typ {
	case "feat":
		return "✨ Features"
	case "fix":
		return "🐛 Fixes"
	case "docs":
		return "📝 Docs"
	case "refactor":
		return "♻️ Refactor"
	case "perf":
		return "🚀 Performance"
	case "test":
		return "🧪 Tests"
	case "chore":
		return "🔧 Chore"
	case "build":
		return "📦 Build"
	case "ci":
		return "⚙️ CI"
	case "style":
		return "💄 Style"
	default:
		return "📦 Outros"
	}
}

func unifiedDiff(path, oldText, newText string, context int) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "--- a/%s\n+++ b/%s\n", path, path)
	for _, l := range filterDiffContext(diffLines(splitDiffLines(oldText), splitDiffLines(newText)), context) {
		if l.text == "" {
			continue
		}
		fmt.Fprintf(&sb, "%c%s\n", l.op, l.text)
	}
	return sb.String()
}

func filterDiffContext(lines []diffLine, context int) []diffLine {
	if context < 0 {
		return lines
	}
	out := make([]diffLine, 0, len(lines))
	hold := make([]diffLine, 0, context)
	for _, l := range lines {
		if l.op == ' ' {
			hold = append(hold, l)
			if len(hold) > context {
				hold = hold[1:]
			}
			continue
		}
		out = append(out, hold...)
		hold = hold[:0]
		out = append(out, l)
	}
	return out
}

func compactPatchText(text string, context int) string {
	if context < 0 {
		return text
	}
	lines := strings.Split(text, "\n")
	out := make([]string, 0, len(lines))
	hold := make([]string, 0, context)
	flush := func() {
		out = append(out, hold...)
		hold = hold[:0]
	}
	for _, line := range lines {
		switch {
		case strings.HasPrefix(line, "@@") || isDiffHeader(line):
			flush()
			out = append(out, line)
		case strings.HasPrefix(line, "+") || strings.HasPrefix(line, "-") || strings.HasPrefix(line, "\\"):
			flush()
			out = append(out, line)
		case strings.HasPrefix(line, " "):
			hold = append(hold, line)
			if len(hold) > context {
				hold = hold[1:]
			}
		default:
			flush()
			out = append(out, line)
		}
	}
	return strings.Join(out, "\n")
}

func isDiffHeader(line string) bool {
	for _, prefix := range []string{
		"diff --git ", "index ",
		"new file mode ", "deleted file mode ", "old mode ", "new mode ",
		"similarity index ", "rename from ", "rename to ", "Binary files ",
	} {
		if strings.HasPrefix(line, prefix) {
			return true
		}
	}
	return false
}

type diffLine struct {
	op   byte
	text string
}

func splitDiffLines(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(strings.TrimSuffix(s, "\n"), "\n")
}

func diffLines(a, b []string) []diffLine {
	n, m := len(a), len(b)
	if n == 0 && m == 0 {
		return nil
	}
	max := n + m
	offset := max + 1
	v := make([]int, 2*max+3)
	var trace [][]int

	solved := false
	endD := 0
	for d := 0; d <= max; d++ {
		trace = append(trace, append([]int(nil), v...))
		for k := -d; k <= d; k += 2 {
			x := 0
			if k == -d || (k != d && v[offset+k-1] < v[offset+k+1]) {
				x = v[offset+k+1]
			} else {
				x = v[offset+k-1] + 1
			}
			y := x - k
			for x < n && y < m && a[x] == b[y] {
				x++
				y++
			}
			v[offset+k] = x
			if x >= n && y >= m {
				endD = d
				solved = true
				break
			}
		}
		if solved {
			break
		}
	}

	out := make([]diffLine, 0, n+m)
	x, y := n, m
	for d := endD; d > 0; d-- {
		prev := trace[d]
		k := x - y
		prevK := k + 1
		if k != -d && (k == d || prev[offset+k-1] >= prev[offset+k+1]) {
			prevK = k - 1
		}
		prevX := prev[offset+prevK]
		prevY := prevX - prevK
		for x > prevX && y > prevY {
			out = append(out, diffLine{op: ' ', text: a[x-1]})
			x--
			y--
		}
		if x == prevX {
			if y > 0 {
				out = append(out, diffLine{op: '+', text: b[y-1]})
				y--
			}
		} else if x > 0 {
			out = append(out, diffLine{op: '-', text: a[x-1]})
			x--
		}
	}
	for x > 0 && y > 0 {
		out = append(out, diffLine{op: ' ', text: a[x-1]})
		x--
		y--
	}
	for x > 0 {
		out = append(out, diffLine{op: '-', text: a[x-1]})
		x--
	}
	for y > 0 {
		out = append(out, diffLine{op: '+', text: b[y-1]})
		y--
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out
}

func formatWorkingDiff(title string, diffs []string, added, deleted int) string {
	if len(diffs) == 0 {
		return fmt.Sprintf("## %s\n\n_Sem alterações._\n", title)
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "## %s\n\n", title)
	fmt.Fprintf(&sb, "📊 **+%d / -%d** linhas\n\n```diff\n%s\n```\n", added, deleted, strings.Join(diffs, "\n"))
	return sb.String()
}

const goGitVersion = "go-git v6.0.0-alpha.5"

type repoInfoData struct {
	Root        string
	Branch      string
	Detached    bool
	Head        string
	HeadFull    string
	CommitCount int
	BranchCount int
	TagCount    int
	RemoteCount int
	Engine      string
}

func formatRepoInfo(info repoInfoData) string {
	var b strings.Builder
	b.WriteString("## Informações do repositório\n\n")
	if info.Root != "" {
		fmt.Fprintf(&b, "- **Raiz da árvore de trabalho:** `%s`\n", info.Root)
	}
	headLabel := info.Head
	if info.Detached {
		headLabel = info.Head + " (detached)"
	} else if info.Branch != "" {
		headLabel = info.Branch + " (" + info.Head + ")"
	}
	fmt.Fprintf(&b, "- **HEAD:** `%s`\n", headLabel)
	if info.HeadFull != "" {
		fmt.Fprintf(&b, "- **SHA do HEAD:** `%s`\n", info.HeadFull)
	}
	fmt.Fprintf(&b, "- **Commits (total):** %d\n", info.CommitCount)
	fmt.Fprintf(&b, "- **Branches:** %d\n", info.BranchCount)
	fmt.Fprintf(&b, "- **Tags:** %d\n", info.TagCount)
	fmt.Fprintf(&b, "- **Remotes:** %d\n", info.RemoteCount)
	fmt.Fprintf(&b, "- **Engine:** %s (sem git CLI)\n", info.Engine)
	return b.String()
}

func formatBranchCompare(base, head string, ahead, behind int, stats object.FileStats, commits []*object.Commit) string {
	var b strings.Builder
	fmt.Fprintf(&b, "## Comparação: `%s` → `%s`\n\n", base, head)
	fmt.Fprintf(&b, "- 🔼 **Ahead:** %d commit(s) em `%s` ausentes em `%s`\n", ahead, head, base)
	fmt.Fprintf(&b, "- 🔽 **Behind:** %d commit(s) em `%s` ausentes em `%s`\n\n", behind, base, head)
	if len(stats) > 0 {
		totalA, totalD := 0, 0
		for _, s := range stats {
			totalA += s.Addition
			totalD += s.Deletion
		}
		fmt.Fprintf(&b, "📊 **%d** arquivo(s) alterado(s): **+%d / -%d**\n\n", len(stats), totalA, totalD)
	} else {
		b.WriteString("_Sem alterações de arquivos._\n\n")
	}
	fmt.Fprintf(&b, "### Commits de diferença (%d)\n\n", len(commits))
	if len(commits) == 0 {
		b.WriteString("_Nenhum commit de diferença._\n")
		return b.String()
	}
	for _, c := range commits {
		fmt.Fprintf(&b, "- `%s` **%s** — %s · %s\n", shortSHA(c.Hash), clean(truncate(firstLine(c.Message), 80)), clean(c.Author.Name), c.Author.When.Format("2006-01-02"))
	}
	return b.String()
}

func formatTree(paths []string) string {
	if len(paths) == 0 {
		return "## Árvore\n\n_Nenhum arquivo rastreado neste ref/path._"
	}
	return fmt.Sprintf("## Árvore (%d arquivos)\n\n```\n%s\n```\n", len(paths), strings.Join(paths, "\n"))
}

func formatReadFile(path, at string, content string) string {
	lang := ""
	switch {
	case strings.HasSuffix(path, ".go"):
		lang = "go"
	case strings.HasSuffix(path, ".md"):
		lang = "markdown"
	case strings.HasSuffix(path, ".json"):
		lang = "json"
	case strings.HasSuffix(path, ".yml") || strings.HasSuffix(path, ".yaml"):
		lang = "yaml"
	case strings.HasSuffix(path, ".ts"):
		lang = "typescript"
	case strings.HasSuffix(path, ".js"):
		lang = "javascript"
	}
	return fmt.Sprintf("## Arquivo `%s` (%s)\n\n```%s\n%s\n```\n", path, at, lang, content)
}

func formatFindFiles(pattern string, paths []string) string {
	if len(paths) == 0 {
		return "## Busca de arquivos\n\n_Nenhum arquivo encontrado para o padrão informado._"
	}
	label := pattern
	if label == "" {
		label = "(todos)"
	}
	return fmt.Sprintf("## Busca de arquivos: `%s` (%d)\n\n```\n%s\n```\n", label, len(paths), strings.Join(paths, "\n"))
}

type stashRow struct {
	SHA     string
	Branch  string
	Message string
}

func formatStash(stashes []stashRow) string {
	if len(stashes) == 0 {
		return "## Stashes\n\n_Nenhum stash._"
	}
	var b strings.Builder
	b.WriteString("## Stashes\n\n")
	for _, st := range stashes {
		extra := ""
		if st.Branch != "" {
			extra = " _(" + clean(st.Branch) + ")_"
		}
		fmt.Fprintf(&b, "- `%s` %s%s\n", st.SHA, clean(st.Message), extra)
	}
	return b.String()
}

type submoduleRow struct {
	Path     string
	Name     string
	URL      string
	Expected string
	Branch   string
}

func formatSubmodules(rows []submoduleRow) string {
	if len(rows) == 0 {
		return "## Submodules\n\n_Nenhum submodule._"
	}
	var b strings.Builder
	b.WriteString("## Submodules\n\n")
	for _, r := range rows {
		fmt.Fprintf(&b, "- `%s` (**%s**) — %s · esperado `%s` [branch: %s]\n", clean(r.Path), clean(r.Name), clean(r.URL), r.Expected, clean(r.Branch))
	}
	return b.String()
}
