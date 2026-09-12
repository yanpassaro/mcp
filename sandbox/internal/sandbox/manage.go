package sandbox

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type TreeNode struct {
	Name     string     `json:"name"`
	IsDir    bool       `json:"isDir"`
	Size     int64      `json:"size,omitempty"`
	Lines    int        `json:"lines,omitempty"`
	Children []TreeNode `json:"children,omitempty"`
}

func (s *Store) CopyIn(hostSrc, dest string) (string, error) {
	if strings.TrimSpace(hostSrc) == "" {
		return "", errors.New("caminho de origem no host (path) é obrigatório")
	}
	if strings.TrimSpace(dest) == "" {
		return "", errors.New("caminho de destino no sandbox (dest) é obrigatório")
	}
	if _, err := os.Stat(hostSrc); err != nil {
		return "", fmt.Errorf("origem no host %q: %w", hostSrc, err)
	}
	fullDest, err := s.resolve(dest)
	if err != nil {
		return "", err
	}
	if err := copyPath(hostSrc, fullDest); err != nil {
		return "", fmt.Errorf("copiar %q para o sandbox: %w", hostSrc, err)
	}
	return dest, nil
}

func (s *Store) CopyOut(src, hostDest string) (string, error) {
	if strings.TrimSpace(src) == "" {
		return "", errors.New("caminho de origem no sandbox (path) é obrigatório")
	}
	if strings.TrimSpace(hostDest) == "" {
		return "", errors.New("caminho de destino no host (dest) é obrigatório")
	}
	fullSrc, err := s.resolve(src)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(fullSrc); err != nil {
		return "", fmt.Errorf("origem no sandbox %q: %w", src, err)
	}
	if err := copyPath(fullSrc, hostDest); err != nil {
		return "", fmt.Errorf("copiar %q para o host: %w", src, err)
	}
	return hostDest, nil
}

func (s *Store) DeleteAll(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("informe um caminho no sandbox")
	}
	full, err := s.resolve(name)
	if err != nil {
		return err
	}
	if samePath(full, s.Root) {
		return errors.New("não é permitido remover a raiz do sandbox")
	}
	if _, err := os.Lstat(full); err != nil {
		return err
	}
	return os.RemoveAll(full)
}

func (s *Store) Tree(rel string) (TreeNode, error) {
	fullRoot := s.Root
	if strings.TrimSpace(rel) != "" {
		var err error
		fullRoot, err = s.resolve(rel)
		if err != nil {
			return TreeNode{}, err
		}
	}
	fi, err := os.Lstat(fullRoot)
	if err != nil {
		return TreeNode{}, err
	}
	if !fi.IsDir() {
		return TreeNode{}, fmt.Errorf("%q não é uma pasta", rel)
	}
	rootName := strings.TrimSpace(rel)
	if rootName == "" {
		rootName = filepath.Base(fullRoot)
	}
	return s.buildTree(fullRoot, rootName)
}

func (s *Store) buildTree(full, name string) (TreeNode, error) {
	fi, err := os.Lstat(full)
	if err != nil {
		return TreeNode{}, err
	}
	node := TreeNode{Name: name, IsDir: fi.IsDir(), Size: fi.Size()}
	if !fi.IsDir() {
		if fi.Mode()&os.ModeSymlink == 0 {
			if n, cerr := countLines(full); cerr == nil {
				node.Lines = n
			}
		}
		return node, nil
	}
	entries, err := os.ReadDir(full)
	if err != nil {
		return node, err
	}
	for _, e := range entries {
		en := e.Name()
		if strings.HasPrefix(en, ".") {
			continue
		}
		child, err := s.buildTree(filepath.Join(full, en), en)
		if err != nil {
			continue
		}
		node.Children = append(node.Children, child)
	}
	sort.Slice(node.Children, func(i, j int) bool {
		if node.Children[i].IsDir != node.Children[j].IsDir {
			return node.Children[i].IsDir
		}
		return node.Children[i].Name < node.Children[j].Name
	})
	return node, nil
}

func (s *Store) CopyWithin(src, dst string) (string, error) {
	fullSrc, err := s.resolve(src)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(fullSrc); err != nil {
		return "", fmt.Errorf("origem %q: %w", src, err)
	}
	fullDst, err := s.resolve(dst)
	if err != nil {
		return "", err
	}
	if err := copyPath(fullSrc, fullDst); err != nil {
		return "", err
	}
	return dst, nil
}

func (s *Store) Move(src, dst string) (string, error) {
	fullSrc, err := s.resolve(src)
	if err != nil {
		return "", err
	}
	fullDst, err := s.resolve(dst)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(fullDst), 0o755); err != nil {
		return "", err
	}
	if err := os.Rename(fullSrc, fullDst); err != nil {
		return "", err
	}
	return dst, nil
}

func (s *Store) Mkdir(path string) (string, error) {
	full, err := s.resolve(path)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(full, 0o755); err != nil {
		return "", err
	}
	return path, nil
}

func (s *Store) Glob(pattern string) ([]string, error) {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return nil, errors.New("padrão de glob vazio")
	}
	full, err := s.resolve(pattern)
	if err != nil {
		return nil, err
	}
	matches, err := filepath.Glob(full)
	if err != nil {
		return nil, err
	}
	root, _ := filepath.Abs(s.Root)
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		rel, err := filepath.Rel(root, m)
		if err != nil {
			continue
		}
		if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
			continue
		}
		out = append(out, rel)
	}
	sort.Strings(out)
	return out, nil
}

func (s *Store) Walk(root string, files, dirs bool, ext string) ([]string, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		root = "."
	}
	full, err := s.resolve(root)
	if err != nil {
		return nil, err
	}
	var out []string
	err = filepath.Walk(full, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if path != full && strings.HasPrefix(info.Name(), ".") {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if info.IsDir() {
			if dirs && path != full {
				if rel, e := filepath.Rel(s.Root, path); e == nil {
					out = append(out, rel)
				}
			}
			return nil
		}
		if !files {
			return nil
		}
		if ext != "" && !strings.EqualFold(filepath.Ext(info.Name()), ext) {
			return nil
		}
		if rel, e := filepath.Rel(s.Root, path); e == nil {
			out = append(out, rel)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(out)
	return out, nil
}

func (s *Store) RecursiveGlob(pattern string, files, dirs bool, ext string) ([]string, error) {
	all, err := s.Walk("", files, dirs, ext)
	if err != nil {
		return nil, err
	}
	pat := strings.TrimSpace(pattern)
	pat = strings.TrimPrefix(pat, "**/")
	out := []string{}
	for _, rel := range all {
		if ok, _ := filepath.Match(pat, filepath.Base(rel)); ok {
			out = append(out, rel)
		}
	}
	return out, nil
}

func filterGlob(store *Store, names []string, files, dirs bool, ext string) ([]string, error) {
	out := []string{}
	for _, n := range names {
		st, err := store.Stat(n)
		if err != nil {
			continue
		}
		if st.IsDir {
			if dirs {
				out = append(out, n)
			}
			continue
		}
		if !files {
			continue
		}
		if ext != "" && !strings.EqualFold(filepath.Ext(n), ext) {
			continue
		}
		out = append(out, n)
	}
	return out, nil
}

func (s *Store) Clear() (int, error) {
	entries, err := os.ReadDir(s.Root)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, e := range entries {
		if err := os.RemoveAll(filepath.Join(s.Root, e.Name())); err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}

func samePath(a, b string) bool {
	ca, errA := filepath.Abs(filepath.Clean(a))
	cb, errB := filepath.Abs(filepath.Clean(b))
	if errA != nil || errB != nil {
		return filepath.Clean(a) == filepath.Clean(b)
	}
	return ca == cb
}

func copyPath(src, dst string) error {
	fi, err := os.Stat(src)
	if err != nil {
		return err
	}
	if fi.IsDir() {
		if err := os.MkdirAll(dst, 0o755); err != nil {
			return err
		}
		entries, err := os.ReadDir(src)
		if err != nil {
			return err
		}
		for _, e := range entries {
			if err := copyPath(filepath.Join(src, e.Name()), filepath.Join(dst, e.Name())); err != nil {
				return err
			}
		}
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}
