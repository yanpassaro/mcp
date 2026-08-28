package fetch

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type cookieRec struct {
	Name     string    `json:"name"`
	Value    string    `json:"value"`
	Path     string    `json:"path,omitempty"`
	Domain   string    `json:"domain,omitempty"`
	Expires  time.Time `json:"expires,omitempty"`
	MaxAge   int       `json:"maxAge,omitempty"`
	HttpOnly bool      `json:"httpOnly,omitempty"`
	Secure   bool      `json:"secure,omitempty"`
}

type cookieStore struct {
	path string
	jar  map[string]map[string]cookieRec
}

func newCookieStore(path string) *cookieStore {
	s := &cookieStore{path: path, jar: map[string]map[string]cookieRec{}}
	s.load()
	return s
}

func (s *cookieStore) load() {
	b, err := os.ReadFile(s.path)
	if err != nil {
		return
	}
	var raw map[string]map[string]cookieRec
	if err := json.Unmarshal(b, &raw); err != nil {
		return
	}
	s.jar = raw
}

func (s *cookieStore) save() {
	if s.path == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return
	}
	b, err := json.MarshalIndent(s.jar, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(s.path, b, 0o600)
}

func (s *cookieStore) matches(host, reqPath string, secure bool, c cookieRec) bool {
	if !c.Expires.IsZero() && time.Now().After(c.Expires) {
		return false
	}
	dom := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(c.Domain), "."))
	h := strings.ToLower(host)
	if dom == "" {
		dom = h
	}
	if dom != h && !strings.HasSuffix(h, "."+dom) {
		return false
	}
	p := c.Path
	if p == "" {
		p = "/"
	}
	if !strings.HasPrefix(reqPath, p) {
		return false
	}
	if c.Secure && !secure {
		return false
	}
	return true
}

func (s *cookieStore) Header(host, reqPath string, secure bool) []string {
	var out []string
	for _, cs := range s.jar {
		for _, c := range cs {
			if s.matches(host, reqPath, secure, c) {
				out = append(out, c.Name+"="+c.Value)
			}
		}
	}
	return out
}

func (s *cookieStore) Save(host string, cookies []*http.Cookie) bool {
	if len(cookies) == 0 {
		return false
	}
	changed := false
	for _, c := range cookies {
		if c == nil {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(c.Domain))
		if key == "" {
			key = strings.ToLower(host)
		}
		if s.jar[key] == nil {
			s.jar[key] = map[string]cookieRec{}
		}
		if c.MaxAge < 0 {
			if _, ok := s.jar[key][c.Name]; ok {
				delete(s.jar[key], c.Name)
				changed = true
			}
			continue
		}
		s.jar[key][c.Name] = cookieRec{
			Name: c.Name, Value: c.Value, Path: c.Path, Domain: c.Domain,
			Expires: c.Expires, MaxAge: c.MaxAge, HttpOnly: c.HttpOnly, Secure: c.Secure,
		}
		changed = true
	}
	if changed {
		s.save()
	}
	return changed
}

type CookieRow struct {
	Domain   string
	Name     string
	Value    string
	Path     string
	Expires  time.Time
	HttpOnly bool
	Secure   bool
}

func (s *cookieStore) List() []CookieRow {
	var rows []CookieRow
	for dom, cs := range s.jar {
		for _, c := range cs {
			rows = append(rows, CookieRow{
				Domain: dom, Name: c.Name, Value: c.Value, Path: c.Path,
				Expires: c.Expires, HttpOnly: c.HttpOnly, Secure: c.Secure,
			})
		}
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Domain != rows[j].Domain {
			return rows[i].Domain < rows[j].Domain
		}
		return rows[i].Name < rows[j].Name
	})
	return rows
}

func (s *cookieStore) Clear(domain, name string) int {
	n := 0
	if strings.TrimSpace(domain) == "" {
		for _, cs := range s.jar {
			n += len(cs)
		}
		s.jar = map[string]map[string]cookieRec{}
	} else {
		key := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(domain), "."))
		if strings.TrimSpace(name) == "" {
			n = len(s.jar[key])
			delete(s.jar, key)
		} else if cs, ok := s.jar[key]; ok {
			if _, ok := cs[name]; ok {
				n = 1
				delete(cs, name)
				if len(cs) == 0 {
					delete(s.jar, key)
				}
			}
		}
	}
	s.save()
	return n
}
