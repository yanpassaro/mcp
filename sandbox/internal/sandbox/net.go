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

type netConfig struct {
	allow      []string
	timeout    time.Duration
	maxBody    int64
	cookieFile string
	store      *sandboxCookieStore
}

func defaultNetConfig() netConfig {
	cfg := netConfig{
		timeout:    30 * time.Second,
		maxBody:    1 << 20,
		cookieFile: filepath.Join(userLocalShare(), "mcp", "sandbox", "cookies.json"),
	}
	v, err := strconv.Atoi(os.Getenv("SANDBOX_FETCH_TIMEOUT_SECONDS"))
	if err == nil {
		if v > 0 {
			cfg.timeout = time.Duration(v) * time.Second
		}
	}
	kb, err := strconv.Atoi(os.Getenv("SANDBOX_FETCH_MAX_BODY_KB"))
	if err == nil {
		if kb > 0 {
			cfg.maxBody = int64(kb) * 1024
		}
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
	out := []string{}
	for h := range strings.SplitSeq(s, ",") {
		h = strings.ToLower(strings.TrimSpace(h))
		if h == "" {
			continue
		}
		out = append(out, h)
	}
	return out
}

func (c *netConfig) allowHost(authority string) bool {
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

func buildNet(L *lua.State, store *Store) int {
	cfg := defaultNetConfig()
	t := newTable(L)

	setGoFunc(L, t, "request", func(l *lua.State) int {
		res, err := doNet(&cfg, argString(l, 1), toAnyMap(l, 2))
		if err != nil {
			panic(err)
		}
		pushAny(l, res)
		return 1
	})
	setGoFunc(L, t, "get", func(l *lua.State) int {
		opts := toAnyMap(l, 2)
		opts["method"] = "GET"
		res, err := doNet(&cfg, argString(l, 1), opts)
		if err != nil {
			panic(err)
		}
		pushAny(l, res)
		return 1
	})
	setGoFunc(L, t, "post", func(l *lua.State) int {
		opts := toAnyMap(l, 3)
		opts["method"] = "POST"
		s, ok := l.ToValue(2).(string)
		if ok {
			opts["body"] = s
		}
		if !ok {
			if l.Top() >= 2 {
				if l.ToValue(2) != nil {
					opts = toAnyMap(l, 2)
					opts["method"] = "POST"
				}
			}
		}
		res, err := doNet(&cfg, argString(l, 1), opts)
		if err != nil {
			panic(err)
		}
		pushAny(l, res)
		return 1
	})
	setGoFunc(L, t, "json", func(l *lua.State) int {
		opts := toAnyMap(l, 2)
		opts["method"] = "GET"
		res, err := doNet(&cfg, argString(l, 1), opts)
		if err != nil {
			panic(err)
		}
		body, _ := res["body"].(string)
		parsed := any(nil)
		if err := json.Unmarshal([]byte(body), &parsed); err != nil {
			panic(fmt.Errorf("resposta não é JSON: %w", err))
		}
		res["data"] = parsed
		delete(res, "body")
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

	setGoFunc(L, t, "save", func(l *lua.State) int {
		url := argString(l, 1)
		path := argString(l, 2)
		if strings.TrimSpace(path) == "" {
			panic("informe o caminho do arquivo de destino")
		}
		meta, err := netSave(&cfg, store, url, path, toAnyMap(l, 3))
		if err != nil {
			panic(err)
		}
		pushAny(l, meta)
		return 1
	})
	return t
}

func doNet(cfg *netConfig, urlStr string, opts map[string]any) (map[string]any, error) {
	body, meta, err := netAttempts(cfg, urlStr, opts, cfg.maxBody)
	if err != nil {
		return nil, err
	}
	meta["body"] = string(body)
	meta["bytes"] = len(body)
	return meta, nil
}

func netSave(cfg *netConfig, store *Store, urlStr, path string, opts map[string]any) (map[string]any, error) {
	body, meta, err := netAttempts(cfg, urlStr, opts, MAX_FILE_BYTES)
	if err != nil {
		return nil, err
	}
	if _, err := store.WriteBytes(path, body); err != nil {
		return nil, err
	}
	meta["bytes"] = len(body)
	meta["path"] = path
	return meta, nil
}

func netAttempts(cfg *netConfig, urlStr string, opts map[string]any, limit int64) ([]byte, map[string]any, error) {
	retries := 0
	d, ok := numOpt(opts["retries"])
	if ok {
		if d > 0 {
			retries = int(d)
		}
	}
	backoff := 250 * time.Millisecond
	d, ok = numOpt(opts["backoffMs"])
	if ok {
		if d >= 0 {
			backoff = time.Duration(d) * time.Millisecond
		}
	}
	lastErr := error(nil)
	lastMeta := map[string]any(nil)
	lastBody := []byte{}
	for attempt := 0; attempt <= retries; attempt++ {
		body, meta, err := doAttempt(cfg, urlStr, opts, limit)
		if err != nil {
			lastErr = err
			lastMeta = nil
		}
		if err == nil {
			lastMeta = meta
			lastBody = body
			okFlag, _ := meta["ok"].(bool)
			status, _ := meta["status"].(int)
			if okFlag {
				return body, meta, nil
			}
			if !isRetryableStatus(status) {
				return body, meta, nil
			}
			lastErr = fmt.Errorf("status %d", status)
		}
		if attempt < retries {
			time.Sleep(backoff * time.Duration(1<<attempt))
		}
	}
	if lastMeta != nil {
		return lastBody, lastMeta, nil
	}
	return nil, nil, lastErr
}

func isRetryableStatus(status int) bool {
	if status == 429 {
		return true
	}
	if status < 500 {
		return false
	}
	return status < 600
}

func isHTTPScheme(scheme string) bool {
	if scheme == "http" {
		return true
	}
	return scheme == "https"
}

func hasPayload(method, body string) bool {
	if method == "GET" {
		return false
	}
	if method == "HEAD" {
		return false
	}
	return body != ""
}

func doAttempt(cfg *netConfig, urlStr string, opts map[string]any, limit int64) ([]byte, map[string]any, error) {
	method, _ := opts["method"].(string)
	method = strings.ToUpper(strings.TrimSpace(method))
	if method == "" {
		method = "GET"
	}

	u, err := url.Parse(strings.TrimSpace(urlStr))
	if err != nil {
		return nil, nil, fmt.Errorf("URL inválida: %w", err)
	}
	if !isHTTPScheme(u.Scheme) {
		return nil, nil, fmt.Errorf("apenas http/https são permitidos (recebi %q)", u.Scheme)
	}
	if u.Host == "" {
		return nil, nil, errors.New("URL sem host")
	}
	if !cfg.allowHost(u.Host) {
		return nil, nil, fmt.Errorf("host %q não está na allowlist (SANDBOX_FETCH_ALLOW_HOST: %s)", u.Host, strings.Join(cfg.allow, ", "))
	}

	timeout := cfg.timeout
	d, ok := numOpt(opts["timeout"])
	if ok {
		if d > 0 {
			timeout = time.Duration(d) * time.Millisecond
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	body := ""
	if b, ok := opts["body"].(string); ok {
		body = b
	}
	rd := io.Reader(nil)
	if hasPayload(method, body) {
		rd = strings.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, u.String(), rd)
	if err != nil {
		return nil, nil, fmt.Errorf("montar requisição: %w", err)
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
	if hasPayload(method, body) {
		if req.Header.Get("Content-Type") == "" {
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
	}

client := &http.Client{
		Timeout:   timeout,
		Transport: &http.Transport{Proxy: nil},
	}
	follow, _ := opts["followRedirects"].(bool)
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	if follow {
		client.CheckRedirect = nil
	}

	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	if !noCookies {
		if cs := resp.Cookies(); len(cs) > 0 {
			cfg.store.Save(u.Hostname(), cs)
		}
	}

	raw, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, nil, err
	}
	trunc := int64(len(raw)) > limit
	if trunc {
		raw = raw[:limit]
	}

	hdr := map[string]any{}
	for k, vv := range resp.Header {
		hdr[k] = strings.Join(vv, ", ")
	}
	return raw, map[string]any{
		"status":     resp.StatusCode,
		"statusText": resp.Status,
		"ok":         isOKStatus(resp.StatusCode),
		"headers":    hdr,
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

func isOKStatus(status int) bool {
	if status < 200 {
		return false
	}
	return status < 300
}

func (s *sandboxCookieStore) load() {
	b, err := os.ReadFile(s.path)
	if err != nil {
		return
	}
	raw := map[string]map[string]cookieRec{}
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
	if !c.Expires.IsZero() {
		if time.Now().After(c.Expires) {
			return false
		}
	}
	dom := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(c.Domain), "."))
	h := strings.ToLower(host)
	if dom == "" {
		dom = h
	}
	if dom != h {
		if !strings.HasSuffix(h, fmt.Sprintf(".%s", dom)) {
			return false
		}
	}
	p := c.Path
	if p == "" {
		p = "/"
	}
	if !strings.HasPrefix(reqPath, p) {
		return false
	}
	if c.Secure {
		if !secure {
			return false
		}
	}
	return true
}

func (s *sandboxCookieStore) Header(host, reqPath string, secure bool) []string {
	out := []string{}
	for _, cs := range s.jar {
		for _, c := range cs {
			if s.matches(host, reqPath, secure, c) {
				out = append(out, fmt.Sprintf("%s=%s", c.Name, c.Value))
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
	}
	if key != "" {
		cs, ok := s.jar[key]
		if ok {
			n = len(cs)
			delete(s.jar, key)
		}
	}
	if n > 0 {
		s.save()
	}
	return n
}

func (s *sandboxCookieStore) Set(domain, name, value string, opts map[string]any) bool {
	key := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(domain), "."))
	if key == "" {
		return false
	}
	if strings.TrimSpace(name) == "" {
		return false
	}
	if s.jar[key] == nil {
		s.jar[key] = map[string]cookieRec{}
	}
	rec := cookieRec{Name: name, Value: value, Domain: strings.TrimPrefix(strings.TrimSpace(domain), "."), Path: "/"}
	if p, ok := opts["path"].(string); ok {
		if p != "" {
			rec.Path = p
		}
	}
	if sec, ok := opts["secure"].(bool); ok {
		rec.Secure = sec
	}
	s.jar[key][name] = rec
	s.save()
	return true
}
