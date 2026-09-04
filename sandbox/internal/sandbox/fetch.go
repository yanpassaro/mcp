package sandbox

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	lua "github.com/Shopify/go-lua"
)

type fetchConfig struct {
	allow      []string
	timeout    time.Duration
	maxBody    int64
	cookieFile string
	store      *sandboxCookieStore
}

func defaultFetchConfig() fetchConfig {
	cfg := fetchConfig{
		timeout:    30 * time.Second,
		maxBody:    1 << 20,
		cookieFile: filepath.Join(userLocalShare(), "mcp", "sandbox", "cookies.json"),
	}
	if v, err := strconv.Atoi(os.Getenv("SANDBOX_FETCH_TIMEOUT_SECONDS")); err == nil && v > 0 {
		cfg.timeout = time.Duration(v) * time.Second
	}
	if v, err := strconv.Atoi(os.Getenv("SANDBOX_FETCH_MAX_BODY_KB")); err == nil && v > 0 {
		cfg.maxBody = int64(v) * 1024
	}
	if f := strings.TrimSpace(os.Getenv("SANDBOX_FETCH_COOKIE_FILE")); f != "" {
		cfg.cookieFile = f
	}
	cfg.allow = splitHosts(os.Getenv("SANDBOX_FETCH_ALLOW_HOST"))
	if len(cfg.allow) == 0 {
		cfg.allow = []string{"localhost", "127.0.0.1", "::1"}
	}
	cfg.store = newSandboxCookieStore(cfg.cookieFile)
	return cfg
}

func userLocalShare() string {
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".local", "share")
	}
	return filepath.Join(os.Getenv("USERPROFILE"), ".local", "share")
}

func splitHosts(s string) []string {
	var out []string
	for _, h := range strings.Split(s, ",") {
		if h = strings.ToLower(strings.TrimSpace(h)); h != "" {
			out = append(out, h)
		}
	}
	return out
}

func (c *fetchConfig) allowHost(authority string) bool {
	hostname := strings.ToLower(authority)
	if h, _, err := net.SplitHostPort(authority); err == nil {
		hostname = strings.ToLower(h)
	}
	for _, e := range c.allow {
		if strings.HasPrefix(e, ".") {
			if strings.HasSuffix("."+hostname, e) {
				return true
			}
			continue
		}
		if strings.Contains(e, ":") {
			if e == strings.ToLower(authority) {
				return true
			}
			continue
		}
		if e == hostname {
			return true
		}
	}
	return false
}

func buildFetch(L *lua.State) int {
	cfg := defaultFetchConfig()
	t := newTable(L)

	setGoFunc(L, t, "request", func(l *lua.State) int {
		res, err := doFetch(&cfg, argString(l, 1), toAnyMap(l, 2))
		if err != nil {
			panic(err)
		}
		pushAny(l, res)
		return 1
	})
	setGoFunc(L, t, "get", func(l *lua.State) int {
		opts := toAnyMap(l, 2)
		opts["method"] = "GET"
		res, err := doFetch(&cfg, argString(l, 1), opts)
		if err != nil {
			panic(err)
		}
		pushAny(l, res)
		return 1
	})
	setGoFunc(L, t, "post", func(l *lua.State) int {
		opts := toAnyMap(l, 3)
		opts["method"] = "POST"
		if s, ok := l.ToValue(2).(string); ok {
			opts["body"] = s
		} else if l.Top() >= 2 && l.ToValue(2) != nil {
			opts = toAnyMap(l, 2)
			opts["method"] = "POST"
		}
		res, err := doFetch(&cfg, argString(l, 1), opts)
		if err != nil {
			panic(err)
		}
		pushAny(l, res)
		return 1
	})

	cookies := newTable(L)
	setGoFunc(L, cookies, "list", func(l *lua.State) int {
		pushAny(l, cfg.store.List())
		return 1
	})
	setGoFunc(L, cookies, "clear", func(l *lua.State) int {
		l.PushInteger(cfg.store.Clear(argString(l, 1)))
		return 1
	})
	setGoFunc(L, cookies, "set", func(l *lua.State) int {
		ok := cfg.store.Set(argString(l, 1), argString(l, 2), argString(l, 3), toAnyMap(l, 4))
		l.PushBoolean(ok)
		return 1
	})
	setFieldValue(L, t, "cookies")
	return t
}

func doFetch(cfg *fetchConfig, urlStr string, opts map[string]any) (map[string]any, error) {
	method, _ := opts["method"].(string)
	method = strings.ToUpper(strings.TrimSpace(method))
	if method == "" {
		method = "GET"
	}

	u, err := url.Parse(strings.TrimSpace(urlStr))
	if err != nil {
		return nil, fmt.Errorf("URL inválida: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("apenas http/https são permitidos (recebi %q)", u.Scheme)
	}
	if u.Host == "" {
		return nil, errors.New("URL sem host")
	}
	if !cfg.allowHost(u.Host) {
		return nil, fmt.Errorf("host %q não está na allowlist (SANDBOX_FETCH_ALLOW_HOST: %s)", u.Host, strings.Join(cfg.allow, ", "))
	}

	timeout := cfg.timeout
	if d, ok := numOpt(opts["timeout"]); ok && d > 0 {
		timeout = time.Duration(d) * time.Millisecond
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	var body string
	if b, ok := opts["body"].(string); ok {
		body = b
	}
	var rd io.Reader
	if method != "GET" && method != "HEAD" && body != "" {
		rd = strings.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, u.String(), rd)
	if err != nil {
		return nil, fmt.Errorf("montar requisição: %w", err)
	}
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", "NTDSK-SANDBOX/1.0")
	}
	noCookies, _ := opts["noCookies"].(bool)
	if !noCookies {
		if cv := cfg.store.Header(u.Hostname(), u.Path, u.Scheme == "https"); len(cv) > 0 {
			req.Header.Set("Cookie", strings.Join(cv, "; "))
		}
	}
	if hdr, ok := opts["headers"].(map[string]any); ok {
		for k, v := range hdr {
			req.Header.Set(k, fmt.Sprint(v))
		}
	}
	if method != "GET" && method != "HEAD" && body != "" && req.Header.Get("Content-Type") == "" {
		t := strings.TrimSpace(body)
		switch {
		case strings.HasPrefix(t, "{"):
			req.Header.Set("Content-Type", "application/json")
		case strings.HasPrefix(t, "["):
			req.Header.Set("Content-Type", "application/json")
		case strings.HasPrefix(t, "<"):
			req.Header.Set("Content-Type", "application/xml")
		default:
			req.Header.Set("Content-Type", "text/plain; charset=utf-8")
		}
	}

	client := &http.Client{
		Timeout:   timeout,
		Transport: &http.Transport{Proxy: nil},
	}
	follow, _ := opts["followRedirects"].(bool)
	if follow {
		client.CheckRedirect = nil
	} else {
		client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	}

	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if !noCookies {
		if cs := resp.Cookies(); len(cs) > 0 {
			cfg.store.Save(u.Hostname(), cs)
		}
	}

	raw, err := io.ReadAll(io.LimitReader(resp.Body, cfg.maxBody+1))
	if err != nil {
		return nil, err
	}
	trunc := int64(len(raw)) > cfg.maxBody
	if trunc {
		raw = raw[:cfg.maxBody]
	}

	hdr := map[string]any{}
	for k, vv := range resp.Header {
		hdr[k] = strings.Join(vv, ", ")
	}
	return map[string]any{
		"status":     resp.StatusCode,
		"statusText": resp.Status,
		"ok":         resp.StatusCode >= 200 && resp.StatusCode < 300,
		"headers":    hdr,
		"body":       string(raw),
		"truncated":  trunc,
		"bytes":      len(raw),
		"ms":         time.Since(start).Milliseconds(),
	}, nil
}


type cookieRec struct {
	Name     string    `json:"name"`
	Value    string    `json:"value"`
	Path     string    `json:"path,omitempty"`
	Domain   string    `json:"domain,omitempty"`
	Expires  time.Time `json:"expires,omitempty"`
	HttpOnly bool      `json:"httpOnly,omitempty"`
	Secure   bool      `json:"secure,omitempty"`
}

type sandboxCookieStore struct {
	path string
	jar  map[string]map[string]cookieRec
}

func newSandboxCookieStore(path string) *sandboxCookieStore {
	s := &sandboxCookieStore{path: path, jar: map[string]map[string]cookieRec{}}
	s.load()
	return s
}

func (s *sandboxCookieStore) load() {
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

func (s *sandboxCookieStore) save() {
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

func (s *sandboxCookieStore) matches(host, reqPath string, secure bool, c cookieRec) bool {
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

func (s *sandboxCookieStore) Header(host, reqPath string, secure bool) []string {
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

func (s *sandboxCookieStore) Save(host string, cookies []*http.Cookie) bool {
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
		rec := cookieRec{
			Name: c.Name, Value: c.Value, Path: c.Path, Domain: c.Domain,
			Expires: c.Expires, HttpOnly: c.HttpOnly, Secure: c.Secure,
		}
		if s.jar[key][c.Name] != rec {
			s.jar[key][c.Name] = rec
			changed = true
		}
	}
	if changed {
		s.save()
	}
	return changed
}

func (s *sandboxCookieStore) List() []any {
	var rows []any
	for dom, cs := range s.jar {
		for _, c := range cs {
			rows = append(rows, map[string]any{
				"domain":   dom,
				"name":     c.Name,
				"value":    c.Value,
				"path":     c.Path,
				"secure":   c.Secure,
				"httpOnly": c.HttpOnly,
			})
		}
	}
	return rows
}

func (s *sandboxCookieStore) Clear(domain string) int {
	n := 0
	key := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(domain), "."))
	if key == "" {
		for _, cs := range s.jar {
			n += len(cs)
		}
		s.jar = map[string]map[string]cookieRec{}
	} else if cs, ok := s.jar[key]; ok {
		n = len(cs)
		delete(s.jar, key)
	}
	if n > 0 {
		s.save()
	}
	return n
}

func (s *sandboxCookieStore) Set(domain, name, value string, opts map[string]any) bool {
	key := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(domain), "."))
	if key == "" || strings.TrimSpace(name) == "" {
		return false
	}
	if s.jar[key] == nil {
		s.jar[key] = map[string]cookieRec{}
	}
	rec := cookieRec{Name: name, Value: value, Domain: strings.TrimPrefix(strings.TrimSpace(domain), "."), Path: "/"}
	if p, ok := opts["path"].(string); ok && p != "" {
		rec.Path = p
	}
	if sec, ok := opts["secure"].(bool); ok {
		rec.Secure = sec
	}
	s.jar[key][name] = rec
	s.save()
	return true
}
