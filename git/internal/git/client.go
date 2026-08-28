package git

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/plumbing"
	"github.com/go-git/go-git/v6/plumbing/object"
)

type Client struct{}

func NewClient() *Client {
	return &Client{}
}

func (c *Client) Open(repoPath string) (*git.Repository, error) {
	p := strings.TrimSpace(repoPath)
	if p == "" {
		return nil, errors.New("'repoPath' é obrigatório (informe o caminho do repositório Git)")
	}
	repo, err := git.PlainOpenWithOptions(p, &git.PlainOpenOptions{DetectDotGit: true})
	if err != nil {
		return nil, fmt.Errorf("não foi possível abrir o repositório em %q: %w", p, err)
	}
	return repo, nil
}

func headCommit(repo *git.Repository) (*object.Commit, error) {
	head, err := repo.Head()
	if err != nil {
		return nil, fmt.Errorf("não foi possível obter HEAD (repositório sem commits?): %w", err)
	}
	return repo.CommitObject(head.Hash())
}

func ResolveCommit(repo *git.Repository, rev string) (*object.Commit, error) {
	rev = strings.TrimSpace(rev)
	if rev == "" {
		return headCommit(repo)
	}
	hash, err := repo.ResolveRevision(plumbing.Revision(rev))
	if err != nil {
		if ref, e2 := repo.Reference(plumbing.NewBranchReferenceName(rev), true); e2 == nil {
			h := ref.Hash()
			hash = &h
		} else if ref, e3 := repo.Reference(plumbing.NewTagReferenceName(rev), true); e3 == nil {
			h := ref.Hash()
			hash = &h
		} else {
			return nil, fmt.Errorf("não foi possível resolver a revisão %q: %w", rev, err)
		}
	}
	return repo.CommitObject(*hash)
}

func ResolveDefaultBase(repo *git.Repository) (*object.Commit, error) {
	for _, name := range []string{"origin/main", "origin/master", "main", "master"} {
		if c, err := ResolveCommit(repo, name); err == nil {
			return c, nil
		}
	}
	return nil, fmt.Errorf("não foi possível determinar uma branch base (tente informar 'base')")
}

func BlobContent(repo *git.Repository, h plumbing.Hash) (string, error) {
	blob, err := repo.BlobObject(h)
	if err != nil {
		return "", err
	}
	r, err := blob.Reader()
	if err != nil {
		return "", err
	}
	defer r.Close()
	b, err := io.ReadAll(r)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func TreeAtRef(repo *git.Repository, rev string) (*object.Tree, error) {
	c, err := ResolveCommit(repo, rev)
	if err != nil {
		return nil, err
	}
	return c.Tree()
}

func FileAtRef(repo *git.Repository, rev, path string) (string, error) {
	t, err := TreeAtRef(repo, rev)
	if err != nil {
		return "", err
	}
	f, err := t.File(path)
	if err != nil {
		return "", err
	}
	return f.Contents()
}

func ReadWorkingFile(repo *git.Repository, path string) (string, error) {
	wt, err := repo.Worktree()
	if err != nil {
		return "", err
	}
	f, err := wt.Filesystem().Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	b, err := io.ReadAll(f)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
