package sandbox

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	maxFileBytes  = 2 * 1024 * 1024
	maxWriteBytes = 1 * 1024 * 1024
)

type Entry struct {
	Name  string
	Lines int
	Size  int64
}

type Store struct {
	Root string
}

func NewStore(root string) *Store {
	return &Store{Root: root}
}

func (s *Store) List() ([]Entry, error) {
	return s.ListDir("")
}

func (s *Store) ListDir(rel string) ([]Entry, error) {
	full := s.Root
	if rel = strings.TrimSpace(rel); rel != "" {
		var err error
		full, err = s.resolve(rel)
		if err != nil {
			return nil, err
		}
	}
	entries, err := os.ReadDir(full)
	if err != nil {
		return nil, err
	}
	out := make([]Entry, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		name := e.Name()
		info, err := e.Info()
		if err != nil {
			continue
		}
		n, _ := countLines(filepath.Join(full, name))
		out = append(out, Entry{Name: name, Lines: n, Size: info.Size()})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (s *Store) Read(name string) (string, error) {
	full, err := s.resolve(name)
	if err != nil {
		return "", err
	}
	fi, err := os.Stat(full)
	if err != nil {
		return "", err
	}
	if fi.IsDir() {
		return "", fmt.Errorf("%s é uma pasta, não um arquivo", name)
	}
	if fi.Size() > maxFileBytes {
		return "", fmt.Errorf("arquivo %s excede %d bytes", name, maxFileBytes)
	}
	b, err := os.ReadFile(full)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (s *Store) ReadLines(name string) ([]string, error) {
	full, err := s.resolve(name)
	if err != nil {
		return nil, err
	}
	return readLines(full)
}

func (s *Store) Write(name, content string) (int, error) {
	full, err := s.resolve(name)
	if err != nil {
		return 0, err
	}
	if len(content) > maxWriteBytes {
		return 0, fmt.Errorf("conteúdo excede %d bytes", maxWriteBytes)
	}
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return 0, err
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		return 0, err
	}
	return len(content), nil
}

func (s *Store) Append(name, content string) (int, error) {
	full, err := s.resolve(name)
	if err != nil {
		return 0, err
	}
	var buf []byte
	if fi, err := os.Stat(full); err == nil && !fi.IsDir() {
		if fi.Size() > maxFileBytes {
			return 0, fmt.Errorf("arquivo %s excede %d bytes", name, maxFileBytes)
		}
		b, err := os.ReadFile(full)
		if err != nil {
			return 0, err
		}
		buf = b
	}
	if len(buf)+len(content) > maxWriteBytes {
		return 0, fmt.Errorf("conteúdo excede %d bytes", maxWriteBytes)
	}
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return 0, err
	}
	buf = append(buf, content...)
	if err := os.WriteFile(full, buf, 0o644); err != nil {
		return 0, err
	}
	return len(buf), nil
}

type FileStat struct {
	Name   string
	Exists bool
	IsDir  bool
	Size   int64
	Lines  int
}

func (s *Store) Stat(name string) (FileStat, error) {
	full, err := s.resolve(name)
	if err != nil {
		return FileStat{}, err
	}
	fi, err := os.Stat(full)
	if err != nil {
		if os.IsNotExist(err) {
			return FileStat{Name: name}, nil
		}
		return FileStat{}, err
	}
	st := FileStat{Name: name, Exists: true, IsDir: fi.IsDir(), Size: fi.Size()}
	if !fi.IsDir() {
		if n, cerr := countLines(full); cerr == nil {
			st.Lines = n
		}
	}
	return st, nil
}

func (s *Store) Delete(name string) error {
	full, err := s.resolve(name)
	if err != nil {
		return err
	}
	fi, err := os.Stat(full)
	if err != nil {
		return err
	}
	if fi.IsDir() {
		return fmt.Errorf("%s é uma pasta; só removo arquivos", name)
	}
	return os.Remove(full)
}

func (s *Store) resolve(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", errors.New("nome de arquivo vazio")
	}
	if filepath.IsAbs(name) {
		return "", fmt.Errorf("caminho absoluto fora do sandbox: %s", name)
	}
	full := filepath.Join(s.Root, name)
	rel, err := filepath.Rel(s.Root, full)
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("arquivo fora da pasta do sandbox: %s", name)
	}
	return full, nil
}

func countLines(path string) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 16*1024*1024)
	n := 0
	for sc.Scan() {
		n++
	}
	return n, sc.Err()
}

func readLines(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 16*1024*1024)
	var out []string
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		out = append(out, line)
	}
	return out, sc.Err()
}
