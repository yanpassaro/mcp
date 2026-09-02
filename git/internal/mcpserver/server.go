package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/plumbing"
	"github.com/go-git/go-git/v6/plumbing/object"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	gitmcp "ntdsk.com/mcp/git/internal/git"
)

type Server struct {
	client *gitmcp.Client
}

func New() *Server {
	return &Server{client: gitmcp.NewClient()}
}

func (s *Server) Register(server *mcp.Server) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "git_repo_info",
		Description: "Repository info: worktree root, HEAD (branch/SHA), commit count, branches, tags and remotes. Engine: go-git.",
	}, s.repoInfo)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "git_status",
		Description: "Working tree state: modified, added, removed, untracked and conflicting files (index vs working tree). Markdown table.",
	}, s.status)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "git_log",
		Description: "Commit history (newest first) as a Markdown table. Filters: maxCount, author, path (follows renames), since/until (YYYY-MM-DD). With 'stat'=true shows changed files (+add/-del) per commit (git log --stat style).",
	}, s.log)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "git_show",
		Description: "Details of a commit (author, date, message, changed files and its diff against the parent). 'ref' defaults to HEAD.",
	}, s.show)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "git_diff",
		Description: "Flexible diff: working tree vs HEAD (default), index vs HEAD (staged=true), or two refs (base+head). 'path' filters by prefix; 'context' sets lines of context (0 = only +/- lines, 3 = classic diff). 'stat'=true shows only the per-file summary. Always Markdown.",
	}, s.diff)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "git_refs",
		Description: "List refs of a repository: branches (with all=true includes remotes), remotes (name + URLs) or tags (SHA + date). 'type' selects which: branch, remote or tag.",
	}, s.refs)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "git_blame",
		Description: "Line-by-line blame (SHA + author) of a file, rebuilt from history. 'path' is required; 'maxLines' caps the size.",
	}, s.blame)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "git_tree",
		Description: "List tracked files at a ref (default HEAD). 'path' filters by prefix. Markdown list.",
	}, s.tree)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "git_read_file",
		Description: "Read a file's content: current working tree (no 'ref') or at a revision (branch/tag/SHA).",
	}, s.readFile)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "git_find_commits",
		Description: "Find commits by message text ('query', case-insensitive) with optional author and file filters. Markdown table.",
	}, s.findCommits)
}

func (s *Server) repoInfo(ctx context.Context, _ *mcp.CallToolRequest, in repoInfoInput) (*mcp.CallToolResult, any, error) {
	repo, err := s.client.Open(in.Repo)
	if err != nil {
		return nil, nil, err
	}
	info := repoInfoData{Root: in.Repo, Engine: goGitVersion}
	if head, e := repo.Head(); e == nil {
		info.Head = shortSHA(head.Hash())
		info.HeadFull = head.Hash().String()
		if head.Name().IsBranch() {
			info.Branch = head.Name().Short()
		}
		info.Detached = !head.Name().IsBranch()
	}
	if bIter, e := repo.Branches(); e == nil {
		info.BranchCount = countIter(bIter)
	}
	if tIter, e := repo.Tags(); e == nil {
		info.TagCount = countIter(tIter)
	}
	info.CommitCount = countCommitsAll(repo)
	if cs, total, e := topContributors(repo, 5); e == nil {
		info.Contributors = cs
		info.ContribTotal = total
	}
	if langs, total, e := repoLanguages(repo); e == nil {
		info.Languages = langs
		info.LangTotal = total
	}
	return textResult(formatRepoInfo(info))
}

func (s *Server) status(ctx context.Context, _ *mcp.CallToolRequest, in statusInput) (*mcp.CallToolResult, any, error) {
	repo, err := s.client.Open(in.Repo)
	if err != nil {
		return nil, nil, err
	}
	wt, err := repo.Worktree()
	if err != nil {
		return nil, nil, err
	}
	st, err := wt.Status()
	if err != nil {
		return nil, nil, err
	}
	return textResult(formatStatus(st))
}

func (s *Server) log(ctx context.Context, _ *mcp.CallToolRequest, in logInput) (*mcp.CallToolResult, any, error) {
	repo, err := s.client.Open(in.Repo)
	if err != nil {
		return nil, nil, err
	}
	max := in.MaxCount
	if max <= 0 {
		max = 30
	}
	opts := &git.LogOptions{Order: git.LogOrderCommitterTime}
	if in.Path != "" {
		p := in.Path
		opts.FileName = &p
	}
	if in.Since != "" {
		if t, e := time.Parse("2006-01-02", in.Since); e == nil {
			opts.Since = &t
		}
	}
	if in.Until != "" {
		if t, e := time.Parse("2006-01-02", in.Until); e == nil {
			opts.Until = &t
		}
	}
	if head, err := repo.Head(); err == nil {
		opts.From = head.Hash()
	} else {
		opts.All = true
	}
	iter, err := repo.Log(opts)
	if err != nil {
		return nil, nil, err
	}
	author := strings.ToLower(strings.TrimSpace(in.Author))
	var commits []*object.Commit
	for {
		c, e := iter.Next()
		if e == io.EOF {
			break
		}
		if e != nil {
			return nil, nil, e
		}
		if author != "" {
			hay := strings.ToLower(c.Author.Name + " " + c.Author.Email)
			if !strings.Contains(hay, author) {
				continue
			}
		}
		commits = append(commits, c)
		if len(commits) >= max {
			break
		}
	}
	if in.Stat {
		return textResult(formatLogStat(commits))
	}
	return textResult(formatLog(commits))
}

func (s *Server) show(ctx context.Context, _ *mcp.CallToolRequest, in showInput) (*mcp.CallToolResult, any, error) {
	repo, err := s.client.Open(in.Repo)
	if err != nil {
		return nil, nil, err
	}
	c, err := gitmcp.ResolveCommit(repo, in.Ref)
	if err != nil {
		return nil, nil, err
	}
	var patch *object.Patch
	if len(c.ParentHashes) > 0 {
		parent, e := repo.CommitObject(c.ParentHashes[0])
		if e == nil {
			if pt, e2 := parent.Tree(); e2 == nil {
				if ct, e3 := c.Tree(); e3 == nil {
					if pp, e4 := pt.Patch(ct); e4 == nil {
						patch = pp
					}
				}
			}
		}
	} else {
		if ct, e := c.Tree(); e == nil {
			empty := &object.Tree{}
			if pp, e2 := empty.Patch(ct); e2 == nil {
				patch = pp
			}
		}
	}
	return textResult(formatShow(c, patch))
}

func (s *Server) diff(ctx context.Context, _ *mcp.CallToolRequest, in diffInput) (*mcp.CallToolResult, any, error) {
	repo, err := s.client.Open(in.Repo)
	if err != nil {
		return nil, nil, err
	}
	switch {
	case in.Staged:
		return s.diffStaged(repo, in.Path, in.Context, in.Stat)
	case in.Base != "" || in.Head != "":
		base := strings.TrimSpace(in.Base)
		if base == "" {
			base = "HEAD"
		}
		head := strings.TrimSpace(in.Head)
		if head == "" {
			head = "HEAD"
		}
		return s.diffRefs(repo, base, head, in.Path, in.Context, in.Stat)
	default:
		return s.diffWorking(repo, in.Path, in.Context, in.Stat)
	}
}

func (s *Server) diffRefs(repo *git.Repository, base, head, path string, context int, stat bool) (*mcp.CallToolResult, any, error) {
	from, err := gitmcp.ResolveCommit(repo, base)
	if err != nil {
		return nil, nil, err
	}
	to, err := gitmcp.ResolveCommit(repo, head)
	if err != nil {
		return nil, nil, err
	}
	fromTree, err := from.Tree()
	if err != nil {
		return nil, nil, err
	}
	toTree, err := to.Tree()
	if err != nil {
		return nil, nil, err
	}
	changes, err := fromTree.Diff(toTree)
	if err != nil {
		return nil, nil, err
	}
	var filtered object.Changes
	for _, ch := range changes {
		name := ch.To.Name
		if name == "" {
			name = ch.From.Name
		}
		if path == "" || strings.HasPrefix(name, path) {
			filtered = append(filtered, ch)
		}
	}
	patch, err := filtered.Patch()
	if err != nil {
		return nil, nil, err
	}
	if patch == nil || len(patch.FilePatches()) == 0 {
		return textResult("## Diff: " + base + " → " + head + "\n\n_Sem diferenças._\n")
	}
	if stat {
		return textResult(formatPatchStat(patch, "Diff --stat: "+base+" → "+head))
	}
	return textResult(formatPatch(patch, "Diff: "+base+" → "+head, context))
}

func (s *Server) diffWorking(repo *git.Repository, path string, context int, stat bool) (*mcp.CallToolResult, any, error) {
	wt, err := repo.Worktree()
	if err != nil {
		return nil, nil, err
	}
	head, err := repo.Head()
	if err != nil {
		return nil, nil, err
	}
	hc, err := repo.CommitObject(head.Hash())
	if err != nil {
		return nil, nil, err
	}
	ht, err := hc.Tree()
	if err != nil {
		return nil, nil, err
	}
	st, err := wt.Status()
	if err != nil {
		return nil, nil, err
	}
	keys := make([]string, 0, len(st))
	for k := range st {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var diffs []string
	var srows []statRow
	added, deleted := 0, 0
	for _, name := range keys {
		fs := st[name]
		if fs.Worktree == git.Unmodified && fs.Staging == git.Unmodified {
			continue
		}
		if path != "" && !strings.HasPrefix(name, path) {
			continue
		}
		oldC := ""
		if f, e := ht.File(name); e == nil {
			oldC, _ = f.Contents()
		}
		newC := ""
		if c, e := gitmcp.ReadWorkingFile(repo, name); e == nil {
			newC = c
		}
		if oldC == newC {
			continue
		}
		u := unifiedDiff(name, oldC, newC, context)
		diffs = append(diffs, u)
		fa, fd := 0, 0
		countLines(u, &fa, &fd)
		added += fa
		deleted += fd
		srows = append(srows, statRow{name: name, add: fa, del: fd})
	}
	if stat {
		return textResult(formatWorkingDiffStat("Working tree vs HEAD (--stat)", srows, added, deleted))
	}
	return textResult(formatWorkingDiff("Working tree vs HEAD", diffs, added, deleted))
}

func (s *Server) diffStaged(repo *git.Repository, path string, context int, stat bool) (*mcp.CallToolResult, any, error) {
	head, err := repo.Head()
	if err != nil {
		return nil, nil, err
	}
	hc, err := repo.CommitObject(head.Hash())
	if err != nil {
		return nil, nil, err
	}
	ht, err := hc.Tree()
	if err != nil {
		return nil, nil, err
	}
	headFiles := map[string]bool{}
	if iter := ht.Files(); iter != nil {
		for {
			f, e2 := iter.Next()
			if e2 == io.EOF {
				break
			}
			if e2 != nil {
				break
			}
			headFiles[f.Name] = true
		}
	}
	idx, err := repo.Storer.Index()
	if err != nil {
		return nil, nil, err
	}
	indexSet := map[string]bool{}
	for _, e := range idx.Entries {
		indexSet[e.Name] = true
	}
	var diffs []string
	var srows []statRow
	added, deleted := 0, 0
	addRow := func(name, oldC, newC string) {
		u := unifiedDiff(name, oldC, newC, context)
		diffs = append(diffs, u)
		fa, fd := 0, 0
		countLines(u, &fa, &fd)
		added += fa
		deleted += fd
		srows = append(srows, statRow{name: name, add: fa, del: fd})
	}
	for _, e := range idx.Entries {
		if path != "" && !strings.HasPrefix(e.Name, path) {
			continue
		}
		oldC := ""
		if headFiles[e.Name] {
			if f, err2 := ht.File(e.Name); err2 == nil {
				oldC, _ = f.Contents()
			}
		}
		newC := ""
		if b, err2 := gitmcp.BlobContent(repo, e.Hash); err2 == nil {
			newC = b
		}
		if oldC == newC {
			continue
		}
		addRow(e.Name, oldC, newC)
	}
	for name := range headFiles {
		if path != "" && !strings.HasPrefix(name, path) {
			continue
		}
		if indexSet[name] {
			continue
		}
		oldC := ""
		if f, err2 := ht.File(name); err2 == nil {
			oldC, _ = f.Contents()
		}
		if oldC == "" {
			continue
		}
		addRow(name, oldC, "")
	}
	if stat {
		return textResult(formatWorkingDiffStat("Staged (index) vs HEAD (--stat)", srows, added, deleted))
	}
	return textResult(formatWorkingDiff("Staged (index) vs HEAD", diffs, added, deleted))
}

func topContributors(repo *git.Repository, limit int) ([]contributorRow, int, error) {
	opts := &git.LogOptions{Order: git.LogOrderCommitterTime}
	if head, err := repo.Head(); err == nil {
		opts.From = head.Hash()
	} else {
		opts.All = true
	}
	iter, err := repo.Log(opts)
	if err != nil {
		return nil, 0, err
	}
	counts := map[string]int{}
	names := map[string]string{}
	total := 0
	for {
		c, e := iter.Next()
		if e == io.EOF {
			break
		}
		if e != nil {
			return nil, 0, e
		}
		total++
		key := strings.ToLower(strings.TrimSpace(c.Author.Email))
		if key == "" {
			key = strings.ToLower(strings.TrimSpace(c.Author.Name))
		}
		if key == "" {
			continue
		}
		counts[key]++
		if _, ok := names[key]; !ok {
			if name := strings.TrimSpace(c.Author.Name); name != "" {
				names[key] = name
			}
		}
	}
	rows := make([]contributorRow, 0, len(counts))
	for key, n := range counts {
		name := names[key]
		if name == "" {
			name = key
		}
		rows = append(rows, contributorRow{Name: name, Count: n})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Count != rows[j].Count {
			return rows[i].Count > rows[j].Count
		}
		return strings.ToLower(rows[i].Name) < strings.ToLower(rows[j].Name)
	})
	if len(rows) > limit {
		rows = rows[:limit]
	}
	return rows, total, nil
}

func (s *Server) refs(ctx context.Context, _ *mcp.CallToolRequest, in refsInput) (*mcp.CallToolResult, any, error) {
	repo, err := s.client.Open(in.Repo)
	if err != nil {
		return nil, nil, err
	}
	switch strings.ToLower(strings.TrimSpace(in.Type)) {
	case "branch", "branches":
		current := ""
		if head, err := repo.Head(); err == nil {
			current = head.Name().String()
		}
		var rows []branchRow
		iter, err := repo.Branches()
		if err != nil {
			return nil, nil, err
		}
		for {
			ref, e := iter.Next()
			if e == io.EOF {
				break
			}
			if e != nil {
				return nil, nil, e
			}
			rows = append(rows, branchRow{Name: ref.Name().Short(), Head: shortSHA(ref.Hash()), Current: ref.Name().String() == current})
		}
		if in.All {
			riter, err := repo.References()
			if err != nil {
				return nil, nil, err
			}
			for {
				ref, e := riter.Next()
				if e == io.EOF {
					break
				}
				if e != nil {
					return nil, nil, e
				}
				if !ref.Name().IsRemote() {
					continue
				}
				rows = append(rows, branchRow{Name: ref.Name().Short(), Head: shortSHA(ref.Hash()), Current: ref.Name().String() == current})
			}
		}
		return textResult(formatBranches(rows))
	case "remote", "remotes":
		rs, err := repo.Remotes()
		if err != nil {
			return nil, nil, err
		}
		var rows []remoteRow
		for _, r := range rs {
			rows = append(rows, remoteRow{Name: r.Config().Name, URLs: strings.Join(r.Config().URLs, ", ")})
		}
		return textResult(formatRemotes(rows))
	case "tag", "tags":
		iter, err := repo.Tags()
		if err != nil {
			return nil, nil, err
		}
		var rows []tagRow
		for {
			ref, e := iter.Next()
			if e == io.EOF {
				break
			}
			if e != nil {
				return nil, nil, err
			}
			ch := ref.Hash()
			if t, e2 := repo.TagObject(ref.Hash()); e2 == nil {
				ch = t.Target
			}
			date := ""
			if c, e3 := repo.CommitObject(ch); e3 == nil {
				date = c.Author.When.Format("2006-01-02")
			}
			rows = append(rows, tagRow{Name: ref.Name().Short(), SHA: shortSHA(ch), Date: date})
		}
		return textResult(formatTags(rows))
	default:
		return nil, nil, fmt.Errorf("'type' inválido: use branch, remote ou tag")
	}
}

func (s *Server) blame(ctx context.Context, _ *mcp.CallToolRequest, in blameInput) (*mcp.CallToolResult, any, error) {
	repo, err := s.client.Open(in.Repo)
	if err != nil {
		return nil, nil, err
	}
	path := strings.TrimSpace(in.Path)
	if path == "" {
		return nil, nil, errors.New("'path' é obrigatório para blame")
	}
	opts := &git.LogOptions{Order: git.LogOrderCommitterTime, FileName: &path}
	if head, err := repo.Head(); err == nil {
		opts.From = head.Hash()
	} else {
		opts.All = true
	}
	iter, err := repo.Log(opts)
	if err != nil {
		return nil, nil, err
	}
	var commits []*object.Commit
	for {
		c, e := iter.Next()
		if e == io.EOF {
			break
		}
		if e != nil {
			return nil, nil, e
		}
		commits = append(commits, c)
	}
	for i, j := 0, len(commits)-1; i < j; i, j = i+1, j-1 {
		commits[i], commits[j] = commits[j], commits[i]
	}

	var lines []string
	var authors []blameLine
	for _, c := range commits {
		content, e := fileContentAt(repo, c, path)
		if e != nil {
			continue
		}
		newLines := strings.Split(content, "\n")
		if lines == nil {
			authors = make([]blameLine, len(newLines))
			for i := range newLines {
				authors[i] = blameLineOf(c)
			}
			lines = newLines
			continue
		}
		newAuthors := make([]blameLine, len(newLines))
		used := make([]bool, len(lines))
		for i, nl := range newLines {
			idx := -1
			for j := range lines {
				if !used[j] && lines[j] == nl {
					idx = j
					break
				}
			}
			if idx >= 0 {
				newAuthors[i] = authors[idx]
				used[idx] = true
			} else {
				newAuthors[i] = blameLineOf(c)
			}
		}
		lines = newLines
		authors = newAuthors
	}
	return textResult(formatBlame(lines, authors, in.MaxLines))
}


func (s *Server) tree(ctx context.Context, _ *mcp.CallToolRequest, in treeInput) (*mcp.CallToolResult, any, error) {
	repo, err := s.client.Open(in.Repo)
	if err != nil {
		return nil, nil, err
	}
	ref := strings.TrimSpace(in.Ref)
	if ref == "" {
		ref = "HEAD"
	}
	t, err := gitmcp.TreeAtRef(repo, ref)
	if err != nil {
		return nil, nil, err
	}
	iter := t.Files()
	var paths []string
	if iter != nil {
		for {
			f, e := iter.Next()
			if e == io.EOF {
				break
			}
			if e != nil {
				break
			}
			if (in.Path == "" || strings.HasPrefix(f.Name, in.Path)) && (in.Pattern == "" || matchPath(in.Pattern, f.Name)) {
				paths = append(paths, f.Name)
			}
		}
	}
	sort.Strings(paths)
	return textResult(formatTree(paths))
}

func (s *Server) readFile(ctx context.Context, _ *mcp.CallToolRequest, in readFileInput) (*mcp.CallToolResult, any, error) {
	path := strings.TrimSpace(in.Path)
	if path == "" {
		return nil, nil, errors.New("'path' é obrigatório para ler o arquivo")
	}
	repo, err := s.client.Open(in.Repo)
	if err != nil {
		return nil, nil, err
	}
	if strings.TrimSpace(in.Ref) != "" {
		content, e := gitmcp.FileAtRef(repo, in.Ref, path)
		if e != nil {
			return nil, nil, e
		}
		return textResult(formatReadFile(path, in.Ref, content))
	}
	content, e := gitmcp.ReadWorkingFile(repo, path)
	if e != nil {
		return nil, nil, e
	}
	return textResult(formatReadFile(path, "working tree", content))
}


func (s *Server) findCommits(ctx context.Context, _ *mcp.CallToolRequest, in findCommitsInput) (*mcp.CallToolResult, any, error) {
	repo, err := s.client.Open(in.Repo)
	if err != nil {
		return nil, nil, err
	}
	q := strings.ToLower(strings.TrimSpace(in.Query))
	if q == "" {
		return nil, nil, errors.New("'query' é obrigatório (texto da mensagem)")
	}
	max := in.MaxCount
	if max <= 0 {
		max = 30
	}
	opts := &git.LogOptions{Order: git.LogOrderCommitterTime}
	if in.Path != "" {
		p := in.Path
		opts.FileName = &p
	}
	if head, err := repo.Head(); err == nil {
		opts.From = head.Hash()
	} else {
		opts.All = true
	}
	iter, err := repo.Log(opts)
	if err != nil {
		return nil, nil, err
	}
	author := strings.ToLower(strings.TrimSpace(in.Author))
	var commits []*object.Commit
	for {
		c, e := iter.Next()
		if e == io.EOF {
			break
		}
		if e != nil {
			return nil, nil, e
		}
		if !strings.Contains(strings.ToLower(firstLine(c.Message)), q) {
			continue
		}
		if author != "" {
			hay := strings.ToLower(c.Author.Name + " " + c.Author.Email)
			if !strings.Contains(hay, author) {
				continue
			}
		}
		commits = append(commits, c)
		if len(commits) >= max {
			break
		}
	}
	return textResult(formatLog(commits))
}


func fileContentAt(repo *git.Repository, c *object.Commit, path string) (string, error) {
	f, err := c.File(path)
	if err != nil {
		return "", err
	}
	return f.Contents()
}

func blameLineOf(c *object.Commit) blameLine {
	return blameLine{SHA: shortSHA(c.Hash), Author: clean(c.Author.Name), Date: c.Author.When.Format("2006-01-02")}
}

func countLines(u string, added, deleted *int) {
	for l := range strings.SplitSeq(u, "\n") {
		if strings.HasPrefix(l, "+") {
			*added++
		} else if strings.HasPrefix(l, "-") {
			*deleted++
		}
	}
}

func countIter(iter interface {
	Next() (*plumbing.Reference, error)
}) int {
	n := 0
	for {
		_, e := iter.Next()
		if e == io.EOF {
			break
		}
		if e != nil {
			break
		}
		n++
	}
	return n
}

func countCommitsAll(repo *git.Repository) int {
	iter, err := repo.Log(&git.LogOptions{All: true, Order: git.LogOrderCommitterTime})
	if err != nil {
		return 0
	}
	n := 0
	for {
		_, e := iter.Next()
		if e == io.EOF {
			break
		}
		if e != nil {
			break
		}
		n++
	}
	return n
}


func matchPath(pattern, name string) bool {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return true
	}
	if !strings.ContainsAny(pattern, "*?") {
		return strings.Contains(name, pattern)
	}
	re, err := globRegexp(pattern)
	if err != nil {
		return strings.Contains(name, pattern)
	}
	return re.MatchString(name)
}

func globRegexp(pattern string) (*regexp.Regexp, error) {
	var b strings.Builder
	b.WriteString("^")
	runes := []rune(pattern)
	for i := 0; i < len(runes); i++ {
		switch runes[i] {
		case '*':
			if i+1 < len(runes) && runes[i+1] == '*' {
				b.WriteString("(?:.*/)?")
				i++
			} else {
				b.WriteString("[^/]*")
			}
		case '?':
			b.WriteString("[^/]")
		default:
			b.WriteString(regexp.QuoteMeta(string(runes[i])))
		}
	}
	b.WriteString("$")
	return regexp.Compile(b.String())
}

type repoInfoInput struct {
	Repo string `json:"repo" jsonschema:"Path to the Git repository (required)."`
}

type statusInput struct {
	Repo string `json:"repo" jsonschema:"Path to the Git repository (required)."`
}

type logInput struct {
	Repo     string `json:"repo" jsonschema:"Path to the Git repository (required)."`
	MaxCount int    `json:"maxCount,omitempty" jsonschema:"Maximum number of commits (default 30)."`
	Author   string `json:"author,omitempty" jsonschema:"Author filter (name or e-mail, partial, case-insensitive)."`
	Path     string `json:"path,omitempty" jsonschema:"Filters commits that touched this file (follows renames)."`
	Since    string `json:"since,omitempty" jsonschema:"Start date (YYYY-MM-DD) when the commit was authored."`
	Until    string `json:"until,omitempty" jsonschema:"End date (YYYY-MM-DD)."`
	Stat     bool   `json:"stat,omitempty" jsonschema:"If true, shows changed files (+add/-del) per commit (git log --stat)."`
}

type showInput struct {
	Repo string `json:"repo" jsonschema:"Path to the Git repository (required)."`
	Ref  string `json:"ref,omitempty" jsonschema:"Commit revision (default HEAD)."`
}

type diffInput struct {
	Repo    string `json:"repo" jsonschema:"Path to the Git repository (required)."`
	Base    string `json:"base,omitempty" jsonschema:"Base ref (branch/tag/SHA). If 'head' is also given, diffs base to head."`
	Head    string `json:"head,omitempty" jsonschema:"Final ref (branch/tag/SHA)."`
	Staged  bool   `json:"staged,omitempty" jsonschema:"If true, diffs the index (staged) against HEAD."`
	Stat    bool   `json:"stat,omitempty" jsonschema:"If true, shows only the per-file summary (git diff --stat), without content."`
	Path    string `json:"path,omitempty" jsonschema:"Path prefix filter (optional)."`
	Context int    `json:"context,omitempty" jsonschema:"Lines of context around changes (default 0 = only +/- lines; 3 = classic diff)."`
}


type refsInput struct {
	Repo string `json:"repo" jsonschema:"Path to the Git repository (required)."`
	Type string `json:"type" jsonschema:"Type of refs to list: branch, remote or tag (required)."`
	All  bool   `json:"all,omitempty" jsonschema:"For branch only: if true, includes remote branches."`
}

type blameInput struct {
	Repo     string `json:"repo" jsonschema:"Path to the Git repository (required)."`
	Path     string `json:"path" jsonschema:"Path of the file (required)."`
	MaxLines int    `json:"maxLines,omitempty" jsonschema:"Maximum number of lines shown (default 500)."`
}


type treeInput struct {
	Repo    string `json:"repo" jsonschema:"Path to the Git repository (required)."`
	Ref     string `json:"ref,omitempty" jsonschema:"Ref/tag/SHA (default HEAD)."`
	Path    string `json:"path,omitempty" jsonschema:"Path prefix filter (optional)."`
	Pattern string `json:"pattern,omitempty" jsonschema:"Glob (with * and **) or substring to filter tracked files (e.g. '**/*.go', '*test*')."`
}

type readFileInput struct {
	Repo string `json:"repo" jsonschema:"Path to the Git repository (required)."`
	Path string `json:"path" jsonschema:"Path of the file (required)."`
	Ref  string `json:"ref,omitempty" jsonschema:"Optional ref (branch/tag/SHA) to read the file at that revision; if empty, reads the working tree."`
}


type findCommitsInput struct {
	Repo     string `json:"repo" jsonschema:"Path to the Git repository (required)."`
	Query    string `json:"query" jsonschema:"Text searched in the commit message (case-insensitive)."`
	Author   string `json:"author,omitempty" jsonschema:"Optional author filter (partial, case-insensitive)."`
	Path     string `json:"path,omitempty" jsonschema:"Filters commits that touched this file (optional)."`
	MaxCount int    `json:"maxCount,omitempty" jsonschema:"Maximum number of commits (default 30)."`
}


func textResult(text string) (*mcp.CallToolResult, any, error) {
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: text}},
	}, nil, nil
}
