package mcpserver

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	neturl "net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/config"
	"github.com/go-git/go-git/v6/plumbing"
	"github.com/go-git/go-git/v6/plumbing/client"
	"github.com/go-git/go-git/v6/plumbing/object"
	gitHttp "github.com/go-git/go-git/v6/plumbing/transport/http"
	gitssh "github.com/go-git/go-git/v6/plumbing/transport/ssh"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	gitmcp "ntdsk.com/mcp/git/internal/git"
)

const (
	DEFAULT_TIMEOUT_SECONDS = 60
	NETWORK_TIMEOUT_SECONDS = 300
	GREP_MAX_RESULTS        = 200
)

func (s *Server) act(ctx context.Context, _ *mcp.CallToolRequest, in actInput) (*mcp.CallToolResult, any, error) {
	action := strings.ToLower(strings.TrimSpace(in.Action))
	if action == "" {
		return nil, nil, errors.New("'action' é obrigatório")
	}
	ctx, cancel := context.WithTimeout(ctx, timeoutFor(in.Data, action))
	defer cancel()
	text, err := s.dispatch(ctx, action, in)
	if err != nil {
		if ctx.Err() != nil {
			return nil, nil, fmt.Errorf("%s: %w", action, ctx.Err())
		}
		return nil, nil, err
	}
	return textResult(text)
}

func (s *Server) dispatch(ctx context.Context, action string, in actInput) (string, error) {
	if action == "clone" {
		return cloneAct(ctx, in.Repo, in.Data)
	}
	repo, err := s.client.Open(in.Repo)
	if err != nil {
		return "", err
	}
	switch action {
	case "add":
		return addAct(repo, in.Data)
	case "restore":
		return restoreAct(repo, in.Data)
	case "commit":
		return commitAct(repo, in.Data)
	case "pull":
		return pullAct(ctx, repo, in.Data)
	case "push":
		return pushAct(ctx, repo, in.Data)
	case "fetch":
		return fetchAct(ctx, repo, in.Data)
	case "merge":
		return mergeAct(repo, in.Data)
	case "reset":
		return resetAct(repo, in.Data)
	case "checkout":
		return checkoutAct(repo, in.Data)
	case "branch":
		return branchAct(repo, in.Data)
	case "tag":
		return tagAct(repo, in.Data)
	case "clean":
		return cleanAct(repo, in.Data)
	default:
		return "", errors.New("'action' inválido: add, restore, commit, pull, push, fetch, merge, reset, checkout, branch, tag, clone, clean")
	}
}

func addAct(repo *git.Repository, d actData) (string, error) {
	wt, err := repo.Worktree()
	if err != nil {
		return "", err
	}
	if d.All {
		if err := wt.AddWithOptions(&git.AddOptions{All: true}); err != nil {
			return "", fmt.Errorf("add --all: %w", err)
		}
		return "✅ Todas as alterações foram adicionadas ao index.", nil
	}
	paths := pathsFor(d.Paths)
	if len(paths) == 0 {
		paths = []string{"."}
	}
	for _, p := range paths {
		if _, err := wt.Add(p); err != nil {
			return "", fmt.Errorf("add %q: %w", p, err)
		}
	}
	return fmt.Sprintf("✅ %d caminho(s) adicionados ao index: `%s`.", len(paths), strings.Join(paths, "`, `")), nil
}

func restoreAct(repo *git.Repository, d actData) (string, error) {
	wt, err := repo.Worktree()
	if err != nil {
		return "", err
	}
	paths := pathsFor(d.Paths)
	if len(paths) == 0 {
		return "", errors.New("restore exige 'paths' em 'data'")
	}
	opts := &git.RestoreOptions{Files: paths, Worktree: true}
	target := "working tree"
	if d.Staged {
		opts.Staged = true
		opts.Worktree = false
		target = "index"
	}
	if err := wt.Restore(opts); err != nil {
		return "", fmt.Errorf("restore: %w", err)
	}
	return fmt.Sprintf("✅ %d caminho(s) restaurados no %s.", len(paths), target), nil
}

func commitAct(repo *git.Repository, d actData) (string, error) {
	wt, err := repo.Worktree()
	if err != nil {
		return "", err
	}
	message := strings.TrimSpace(d.Message)
	if d.Amend {
		if d.All {
			return "", errors.New("commit com 'amend' não aceita 'all'")
		}
		if message == "" {
			message, err = headMessage(repo)
			if err != nil {
				return "", err
			}
		}
	}
	if message == "" {
		return "", errors.New("commit exige 'message' em 'data'")
	}
	message, err = withCoAuthors(message, d.CoAuthors)
	if err != nil {
		return "", err
	}
	hash, err := wt.Commit(message, &git.CommitOptions{All: d.All, Amend: d.Amend, AllowEmptyCommits: d.AllowEmpty, Author: authorSignature()})
	if err != nil {
		return "", fmt.Errorf("commit: %w", err)
	}
	return fmt.Sprintf("✅ Commit `%s` criado: %s", shortSHA(hash), firstLine(message)), nil
}

func headMessage(repo *git.Repository) (string, error) {
	head, err := repo.Head()
	if err != nil {
		return "", fmt.Errorf("amend sem 'message' em repositório sem HEAD: %w", err)
	}
	commit, err := repo.CommitObject(head.Hash())
	if err != nil {
		return "", fmt.Errorf("amend sem 'message': %w", err)
	}
	return strings.TrimSpace(commit.Message), nil
}

func withCoAuthors(message string, extra []string) (string, error) {
	entries := append(strings.Split(os.Getenv("GIT_COAUTHOR"), ","), extra...)
	lines := strings.Builder{}
	count := 0
	for _, e := range entries {
		e = strings.TrimSpace(e)
		if e == "" {
			continue
		}
		if !strings.Contains(e, "<") {
			return "", fmt.Errorf("coautor inválido %q: use 'Nome <email>'", e)
		}
		if strings.Contains(message, e) {
			continue
		}
		fmt.Fprintf(&lines, "Co-authored-by: %s\n", e)
		count++
	}
	if count == 0 {
		return message, nil
	}
	return fmt.Sprintf("%s\n\n%s", strings.TrimRight(message, "\n"), strings.TrimRight(lines.String(), "\n")), nil
}

func pullAct(ctx context.Context, repo *git.Repository, d actData) (string, error) {
	wt, err := repo.Worktree()
	if err != nil {
		return "", err
	}
	remote := cmp.Or(strings.TrimSpace(d.Remote), "origin")
	auth, err := remoteAuth(repo, remote)
	if err != nil {
		return "", err
	}
	opts := &git.PullOptions{RemoteName: remote, ClientOptions: auth, Force: d.Force}
	branch := strings.TrimSpace(d.Branch)
	if branch != "" {
		opts.ReferenceName = plumbing.NewBranchReferenceName(branch)
	}
	err = wt.PullContext(ctx, opts)
	if errors.Is(err, git.NoErrAlreadyUpToDate) {
		return "✅ Nada a atualizar: já está sincronizado.", nil
	}
	if err != nil {
		return "", fmt.Errorf("pull: %w", err)
	}
	return fmt.Sprintf("✅ Pull concluído de `%s`.", remote), nil
}

func pushAct(ctx context.Context, repo *git.Repository, d actData) (string, error) {
	remote := cmp.Or(strings.TrimSpace(d.Remote), "origin")
	auth, err := remoteAuth(repo, remote)
	if err != nil {
		return "", err
	}
	if d.Force {
		return "", errors.New("push não aceita 'force'")
	}
	branch := strings.TrimSpace(d.Branch)
	if branch == "" {
		head, e := repo.Head()
		if e != nil {
			return "", e
		}
		if !head.Name().IsBranch() {
			return "", errors.New("push exige 'branch' (HEAD está destacado)")
		}
		branch = head.Name().Short()
	}
	spec := config.RefSpec(fmt.Sprintf("refs/heads/%s:refs/heads/%s", branch, branch))
	opts := &git.PushOptions{RemoteName: remote, RefSpecs: []config.RefSpec{spec}, ClientOptions: auth, FollowTags: d.Tags}
	err = repo.PushContext(ctx, opts)
	if errors.Is(err, git.NoErrAlreadyUpToDate) {
		return "✅ Nada para enviar: já está sincronizado.", nil
	}
	if err != nil {
		return "", fmt.Errorf("push %s: %w", branch, err)
	}
	if d.Tags {
		return fmt.Sprintf("✅ Push de `%s` para `%s` com tags.", branch, remote), nil
	}
	return fmt.Sprintf("✅ Push de `%s` para `%s`.", branch, remote), nil
}

func fetchAct(ctx context.Context, repo *git.Repository, d actData) (string, error) {
	if d.All {
		remotes, err := repo.Remotes()
		if err != nil {
			return "", fmt.Errorf("fetch: %w", err)
		}
		for _, r := range remotes {
			cfg := r.Config()
			if len(cfg.URLs) == 0 {
				return "", fmt.Errorf("remote %q sem URL", cfg.Name)
			}
			auth, err := authForURL(cfg.URLs[0])
			if err != nil {
				return "", err
			}
			if err := r.FetchContext(ctx, fetchOptions(d, auth)); err != nil {
				if errors.Is(err, git.NoErrAlreadyUpToDate) {
					continue
				}
				return "", fmt.Errorf("fetch %s: %w", cfg.Name, err)
			}
		}
		return fmt.Sprintf("✅ Fetch concluído em %d remoto(s).", len(remotes)), nil
	}
	remote := cmp.Or(strings.TrimSpace(d.Remote), "origin")
	auth, err := remoteAuth(repo, remote)
	if err != nil {
		return "", err
	}
	opts := fetchOptions(d, auth)
	if err := repo.FetchContext(ctx, opts); err != nil {
		if errors.Is(err, git.NoErrAlreadyUpToDate) {
			return "✅ Nada a buscar: já está atualizado.", nil
		}
		return "", fmt.Errorf("fetch: %w", err)
	}
	return fmt.Sprintf("✅ Fetch concluído de `%s`.", remote), nil
}

func fetchOptions(d actData, auth []client.Option) *git.FetchOptions {
	opts := &git.FetchOptions{ClientOptions: auth, Prune: d.Prune, Depth: d.Depth}
	if d.Tags {
		opts.Tags = plumbing.AllTags
	}
	return opts
}

func mergeAct(repo *git.Repository, d actData) (string, error) {
	wt, err := repo.Worktree()
	if err != nil {
		return "", err
	}
	status, err := wt.Status()
	if err != nil {
		return "", err
	}
	if hasTracked(status) {
		return "", errors.New("merge exige working tree limpo (sem alterações em arquivos rastreados)")
	}
	ref, err := lookupRef(repo, d.Ref)
	if err != nil {
		return "", err
	}
	name := ref.Name().Short()
	err = repo.Merge(ref, git.MergeOptions{Strategy: git.FastForwardMerge})
	if errors.Is(err, git.ErrFastForwardMergeNotPossible) {
		return "", fmt.Errorf("merge %s: apenas fast-forward é suportado; branches divergentes precisam ser resolvidas manualmente", name)
	}
	if err != nil {
		return "", fmt.Errorf("merge %s: %w", name, err)
	}
	if err := wt.Reset(&git.ResetOptions{Commit: ref.Hash(), Mode: git.HardReset}); err != nil {
		return "", fmt.Errorf("merge %s: sincronizar working tree: %w", name, err)
	}
	return fmt.Sprintf("✅ Fast-forward de `%s`.", name), nil
}

func hasTracked(status git.Status) bool {
	for _, s := range status {
		if s.Staging == git.Untracked {
			continue
		}
		if s.Worktree == git.Untracked {
			continue
		}
		return true
	}
	return false
}

func resetAct(repo *git.Repository, d actData) (string, error) {
	mode := strings.ToLower(strings.TrimSpace(d.Mode))
	if mode == "" {
		mode = "mixed"
	}
	hash, err := resolveHash(repo, cmp.Or(strings.TrimSpace(d.Ref), "HEAD"))
	if err != nil {
		return "", err
	}
	wt, err := repo.Worktree()
	if err != nil {
		return "", err
	}
	if err := wt.Reset(&git.ResetOptions{Commit: hash, Mode: resetMode(mode)}); err != nil {
		return "", fmt.Errorf("reset --%s: %w", mode, err)
	}
	return fmt.Sprintf("✅ Reset `--%s` para `%s`.", mode, shortSHA(hash)), nil
}

func resetMode(mode string) git.ResetMode {
	switch mode {
	case "soft":
		return git.SoftReset
	case "hard":
		return git.HardReset
	case "keep":
		return git.KeepReset
	default:
		return git.MixedReset
	}
}

func checkoutAct(repo *git.Repository, d actData) (string, error) {
	branch := strings.TrimSpace(d.Branch)
	if branch == "" {
		return "", errors.New("checkout exige 'branch' em 'data'")
	}
	wt, err := repo.Worktree()
	if err != nil {
		return "", err
	}
	status, err := wt.Status()
	if err != nil {
		return "", fmt.Errorf("status: %w", err)
	}
	if !status.IsClean() {
		if !d.Force {
			if !d.Keep {
				return "", errors.New("worktree tem alterações não commitadas: commite, use 'keep' ou 'force'")
			}
		}
	}
	opts := &git.CheckoutOptions{Branch: plumbing.NewBranchReferenceName(branch), Create: d.Create, Force: d.Force, Keep: d.Keep}
	if d.Create {
		if strings.TrimSpace(d.Ref) != "" {
			hash, err := resolveHash(repo, d.Ref)
			if err != nil {
				return "", err
			}
			opts.Hash = hash
		}
	}
	if err := wt.Checkout(opts); err != nil {
		return "", fmt.Errorf("checkout %s: %w", branch, err)
	}
	return fmt.Sprintf("✅ Agora em `%s`.", branch), nil
}

func branchAct(repo *git.Repository, d actData) (string, error) {
	branch := strings.TrimSpace(d.Branch)
	if branch == "" {
		return "", errors.New("branch exige 'branch' em 'data'")
	}
	if d.Delete {
		if err := repo.Storer.RemoveReference(plumbing.NewBranchReferenceName(branch)); err != nil {
			return "", fmt.Errorf("delete branch %s: %w", branch, err)
		}
		cfg, err := repo.Config()
		if err != nil {
			return "", fmt.Errorf("config: %w", err)
		}
		if _, ok := cfg.Branches[branch]; ok {
			delete(cfg.Branches, branch)
			if err := repo.SetConfig(cfg); err != nil {
				return "", fmt.Errorf("config: %w", err)
			}
		}
		return fmt.Sprintf("✅ Branch `%s` removida.", branch), nil
	}
	hash, err := resolveHash(repo, cmp.Or(strings.TrimSpace(d.Ref), "HEAD"))
	if err != nil {
		return "", err
	}
	ref := plumbing.NewHashReference(plumbing.NewBranchReferenceName(branch), hash)
	if _, err := repo.Reference(ref.Name(), false); err == nil {
		return "", fmt.Errorf("branch %q já existe", branch)
	}
	if err := repo.Storer.SetReference(ref); err != nil {
		return "", fmt.Errorf("criar branch %s: %w", branch, err)
	}
	return fmt.Sprintf("✅ Branch `%s` criada em `%s`.", branch, shortSHA(hash)), nil
}

func tagAct(repo *git.Repository, d actData) (string, error) {
	name := strings.TrimSpace(d.Name)
	if name == "" {
		return "", errors.New("tag exige 'name' em 'data'")
	}
	if d.Delete {
		if err := repo.DeleteTag(name); err != nil {
			return "", fmt.Errorf("delete tag %s: %w", name, err)
		}
		return fmt.Sprintf("✅ Tag `%s` removida.", name), nil
	}
	hash, err := resolveHash(repo, cmp.Or(strings.TrimSpace(d.Ref), "HEAD"))
	if err != nil {
		return "", err
	}
	message := strings.TrimSpace(d.Message)
	if message == "" {
		if _, err := repo.CreateTag(name, hash, nil); err != nil {
			return "", fmt.Errorf("tag %s: %w", name, err)
		}
		return fmt.Sprintf("✅ Tag leve `%s` criada em `%s`.", name, shortSHA(hash)), nil
	}
	if _, err := repo.CreateTag(name, hash, &git.CreateTagOptions{Message: message}); err != nil {
		return "", fmt.Errorf("tag %s: %w", name, err)
	}
	return fmt.Sprintf("✅ Tag anotada `%s` criada em `%s`.", name, shortSHA(hash)), nil
}

func cleanAct(repo *git.Repository, _ actData) (string, error) {
	wt, err := repo.Worktree()
	if err != nil {
		return "", err
	}
	if err := wt.Clean(&git.CleanOptions{Dir: true}); err != nil {
		return "", fmt.Errorf("clean: %w", err)
	}
	return "✅ Arquivos não rastreados removidos.", nil
}

func cloneAct(ctx context.Context, dest string, d actData) (string, error) {
	url := strings.TrimSpace(d.URL)
	if url == "" {
		return "", errors.New("clone exige 'url' em 'data'")
	}
	path, err := clonePath(dest)
	if err != nil {
		return "", err
	}
	auth, err := authForURL(url)
	if err != nil {
		return "", err
	}
	opts := &git.CloneOptions{URL: url, Depth: d.Depth, ClientOptions: auth}
	if branch := strings.TrimSpace(d.Branch); branch != "" {
		opts.ReferenceName = plumbing.NewBranchReferenceName(branch)
		opts.SingleBranch = true
	}
	if _, err := git.PlainCloneContext(ctx, path, opts); err != nil {
		return "", fmt.Errorf("clone %s: %w", url, err)
	}
	return fmt.Sprintf("✅ Clonado `%s` em `%s`.", url, path), nil
}

func clonePath(dest string) (string, error) {
	p := strings.TrimSpace(dest)
	if p == "" {
		return "", errors.New("'repo' é obrigatório como diretório de destino do clone")
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", fmt.Errorf("caminho do repositório: %w", err)
	}
	info, err := os.Stat(abs)
	if errors.Is(err, os.ErrNotExist) {
		return abs, nil
	}
	if err != nil {
		return "", fmt.Errorf("destino %q: %w", abs, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("destino %q existe e não é um diretório", abs)
	}
	entries, err := os.ReadDir(abs)
	if err != nil {
		return "", fmt.Errorf("destino %q: %w", abs, err)
	}
	if len(entries) != 0 {
		return "", fmt.Errorf("destino %q não está vazio", abs)
	}
	return abs, nil
}

func remoteAuth(repo *git.Repository, name string) ([]client.Option, error) {
	remote, err := repo.Remote(name)
	if err != nil {
		return nil, fmt.Errorf("remote %q: %w", name, err)
	}
	urls := remote.Config().URLs
	if len(urls) == 0 {
		return nil, fmt.Errorf("remote %q sem URL", name)
	}
	return authForURL(urls[0])
}

func authForURL(raw string) ([]client.Option, error) {
	if isSSHURL(raw) {
		auth, err := gitssh.NewSSHAgentAuth(sshUser(raw))
		if err != nil {
			return nil, fmt.Errorf("ssh-agent indisponível: %w", err)
		}
		return []client.Option{client.WithSSHAuth(auth)}, nil
	}
	user, pass := httpCredentials(raw)
	if pass == "" {
		return nil, nil
	}
	return []client.Option{client.WithHTTPAuth(&gitHttp.BasicAuth{Username: user, Password: pass})}, nil
}

func httpCredentials(raw string) (string, string) {
	u, err := neturl.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", ""
	}
	if u.User != nil {
		user := u.User.Username()
		pass, ok := u.User.Password()
		if ok {
			return user, pass
		}
	}
	token := strings.TrimSpace(os.Getenv("GIT_TOKEN"))
	if token == "" {
		return "", ""
	}
	user := strings.TrimSpace(os.Getenv("GIT_HTTP_USER"))
	if user == "" {
		user = "oauth2"
	}
	return user, token
}

func sshUser(url string) string {
	at := strings.Index(url, "@")
	if at < 0 {
		return "git"
	}
	prefix := url[:at]
	if i := strings.LastIndex(prefix, "/"); i >= 0 {
		prefix = prefix[i+1:]
	}
	if prefix == "" {
		return "git"
	}
	return prefix
}

func isSSHURL(url string) bool {
	if strings.HasPrefix(url, "ssh://") {
		return true
	}
	at := strings.Index(url, "@")
	if at < 0 {
		return false
	}
	colon := strings.Index(url, ":")
	if colon < 0 {
		return false
	}
	return at < colon
}

func authorSignature() *object.Signature {
	name := strings.TrimSpace(os.Getenv("GIT_USER"))
	email := strings.TrimSpace(os.Getenv("GIT_EMAIL"))
	if name == "" {
		return nil
	}
	if email == "" {
		return nil
	}
	return &object.Signature{Name: name, Email: email, When: time.Now()}
}

func lookupRef(repo *git.Repository, rev string) (plumbing.Reference, error) {
	name := strings.TrimSpace(rev)
	if name == "" {
		return plumbing.Reference{}, errors.New("merge exige 'ref' em 'data'")
	}
	candidates := []plumbing.ReferenceName{
		plumbing.ReferenceName(name),
		plumbing.NewBranchReferenceName(name),
		plumbing.NewTagReferenceName(name),
		plumbing.ReferenceName("refs/remotes/" + name),
	}
	for _, candidate := range candidates {
		ref, err := repo.Reference(candidate, true)
		if err == nil {
			return *ref, nil
		}
	}
	hash, err := resolveHash(repo, name)
	if err != nil {
		return plumbing.Reference{}, err
	}
	refName := plumbing.NewBranchReferenceName(name)
	if strings.Contains(name, "/") {
		refName = plumbing.ReferenceName("refs/remotes/" + name)
	}
	return *plumbing.NewHashReference(refName, hash), nil
}

func resolveHash(repo *git.Repository, rev string) (plumbing.Hash, error) {
	commit, err := gitmcp.ResolveCommit(repo, rev)
	if err != nil {
		return plumbing.ZeroHash, err
	}
	return commit.Hash, nil
}

func pathsFor(paths []string) []string {
	cleaned := make([]string, 0, len(paths))
	for _, p := range paths {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		cleaned = append(cleaned, p)
	}
	return cleaned
}

func isNetwork(action string) bool {
	switch action {
	case "pull", "push", "fetch", "clone":
		return true
	default:
		return false
	}
}

func timeoutFor(d actData, action string) time.Duration {
	if d.Timeout > 0 {
		return time.Duration(d.Timeout) * time.Second
	}
	if isNetwork(action) {
		return NETWORK_TIMEOUT_SECONDS * time.Second
	}
	return time.Duration(envTimeout()) * time.Second
}

func envTimeout() int {
	n, err := strconv.Atoi(strings.TrimSpace(os.Getenv("GIT_TIMEOUT_SECONDS")))
	if err != nil {
		return DEFAULT_TIMEOUT_SECONDS
	}
	if n <= 0 {
		return DEFAULT_TIMEOUT_SECONDS
	}
	return n
}

func actAllowed() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("GIT_ALLOW_ACT"))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

type actInput struct {
	Repo   string  `json:"repo" jsonschema:"Path to the Git repository (required); for clone, the destination directory."`
	Action string  `json:"action" jsonschema:"Action: add, restore, commit, pull, push, fetch, merge, reset, checkout, branch, tag, clone or clean (required)."`
	Data   actData `json:"data,omitempty" jsonschema:"Options for the chosen action; fields that do not apply are ignored."`
}

type actData struct {
	Paths          []string `json:"paths,omitempty" jsonschema:"Files or directories (add, restore)."`
	Message        string   `json:"message,omitempty" jsonschema:"Commit message; annotated tag message."`
	Ref            string   `json:"ref,omitempty" jsonschema:"Target ref (merge, reset, branch start point)."`
	Remote         string   `json:"remote,omitempty" jsonschema:"Remote name (default origin)."`
	Branch         string   `json:"branch,omitempty" jsonschema:"Branch name."`
	Name           string   `json:"name,omitempty" jsonschema:"Tag name."`
	Mode           string   `json:"mode,omitempty" jsonschema:"Reset mode: soft, mixed, hard, keep."`
	URL            string   `json:"url,omitempty" jsonschema:"URL to clone."`
	Depth          int      `json:"depth,omitempty" jsonschema:"Shallow depth (clone, fetch)."`
	All            bool     `json:"all,omitempty" jsonschema:"Stage all, or all remotes."`
	Amend          bool     `json:"amend,omitempty" jsonschema:"Amend the previous commit."`
	AllowEmpty     bool     `json:"allowEmpty,omitempty" jsonschema:"Allow an empty commit."`
	CoAuthors      []string `json:"coAuthors,omitempty" jsonschema:"Co-authors as 'Name <email>'."`
	Force          bool     `json:"force,omitempty" jsonschema:"Force pull or checkout (discards local changes)."`
	Tags           bool     `json:"tags,omitempty" jsonschema:"Fetch: all tags. Push: follow reachable annotated tags."`
	Prune          bool     `json:"prune,omitempty" jsonschema:"Prune deleted remote refs."`
	Staged         bool     `json:"staged,omitempty" jsonschema:"Restore the index."`
	Create         bool     `json:"create,omitempty" jsonschema:"checkout: create the branch."`
	Keep           bool     `json:"keep,omitempty" jsonschema:"checkout: keep local changes when switching."`
	Delete         bool     `json:"delete,omitempty" jsonschema:"Delete branch/tag."`
	Timeout        int      `json:"timeout,omitempty" jsonschema:"Timeout in seconds."`
}
