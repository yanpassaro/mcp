package fetch

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"sort"
	"strings"
	"time"
)

type Config struct {
	AllowHosts []string
	Timeout    time.Duration
	CookieFile string
	MaxBody    int64
}

const defaultMaxBody = 1 << 20

type Client struct {
	http    *http.Client
	allowed []string
	store   *cookieStore
	maxBody int64
}

func NewClient(cfg Config) (*Client, error) {
	httpc := &http.Client{
		Transport: &http.Transport{
			Proxy:           http.ProxyFromEnvironment,
			DialContext:     dial,
			DialTLSContext:  dialTLS,
		},
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 30 * time.Second
	}
	httpc.Timeout = cfg.Timeout
	allowed := make([]string, 0, len(cfg.AllowHosts))
	for _, h := range cfg.AllowHosts {
		if h = strings.ToLower(strings.TrimSpace(h)); h != "" {
			allowed = append(allowed, h)
		}
	}
	if len(allowed) == 0 {
		allowed = []string{"localhost", "127.0.0.1", "::1"}
	}
	maxBody := cfg.MaxBody
	if maxBody <= 0 {
		maxBody = defaultMaxBody
	}
	return &Client{http: httpc, allowed: allowed, store: newCookieStore(cfg.CookieFile), maxBody: maxBody}, nil
}

func (c *Client) AllowHosts() []string {
	return append([]string(nil), c.allowed...)
}

func (c *Client) MaxBody() int64 {
	return c.maxBody
}

func (c *Client) Cookies() []CookieRow {
	return c.store.List()
}

func (c *Client) ClearCookies(domain, name string) int {
	return c.store.Clear(domain, name)
}

func (c *Client) allowHost(authority string) bool {
	hostname := strings.ToLower(authority)
	if h, _, err := net.SplitHostPort(authority); err == nil {
		hostname = strings.ToLower(h)
	}
	for _, e := range c.allowed {
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

type Req struct {
	Method          string
	URL             string
	Body            string
	Headers         map[string]string
	NoCookies       bool
	FollowRedirects bool
	NoBody          bool
	Timeout         time.Duration
}

type timingKey struct{}

type hopTiming struct {
	DNS     time.Duration
	Connect time.Duration
	TLS     time.Duration
	TTFB    time.Duration
}

func timingFor(ctx context.Context) *hopTiming {
	ht, _ := ctx.Value(timingKey{}).(*hopTiming)
	return ht
}

func dial(ctx context.Context, network, addr string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}
	ht := timingFor(ctx)
	start := time.Now()
	ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, err
	}
	if ht != nil {
		ht.DNS = time.Since(start)
	}
	d := &net.Dialer{}
	var lastErr error
	for _, ip := range ips {
		connStart := time.Now()
		conn, err := d.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
		if err == nil {
			if ht != nil {
				ht.Connect = time.Since(connStart)
			}
			return conn, nil
		}
		lastErr = err
	}
	return nil, lastErr
}

func dialTLS(ctx context.Context, network, addr string) (net.Conn, error) {
	rawConn, err := dial(ctx, network, addr)
	if err != nil {
		return nil, err
	}
	host, _, _ := net.SplitHostPort(addr)
	tconn := tls.Client(rawConn, &tls.Config{ServerName: host})
	start := time.Now()
	if err := tconn.HandshakeContext(ctx); err != nil {
		rawConn.Close()
		return nil, err
	}
	if ht := timingFor(ctx); ht != nil {
		ht.TLS = time.Since(start)
	}
	return tconn, nil
}

func traceTTFB(ht *hopTiming, start time.Time) *httptrace.ClientTrace {
	return &httptrace.ClientTrace{
		GotFirstResponseByte: func() { ht.TTFB = time.Since(start) },
	}
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func curlCommand(method, u string, header http.Header, body string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "curl -X %s %s", method, shellQuote(u))
	keys := make([]string, 0, len(header))
	for k := range header {
		if strings.EqualFold(k, "Accept-Encoding") || strings.EqualFold(k, "Content-Length") {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		for _, v := range header.Values(k) {
			fmt.Fprintf(&b, " \\\n  -H %s", shellQuote(k+": "+v))
		}
	}
	if body != "" {
		fmt.Fprintf(&b, " \\\n  --data-raw %s", shellQuote(body))
	}
	return b.String()
}

type Resp struct {
	Method    string
	URL       string
	Code      int
	Status    string
	Header    http.Header
	Body      []byte
	NoBody    bool
	Truncated bool
	Duration  time.Duration
	Hops      []string
	Timings   []hopTiming
	Curl      string
}

func (c *Client) Do(ctx context.Context, in Req) (*Resp, error) {
	u, err := url.Parse(strings.TrimSpace(in.URL))
	if err != nil {
		return nil, fmt.Errorf("URL inválida: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("apenas http/https são permitidos (recebi %q)", u.Scheme)
	}
	if u.Host == "" {
		return nil, errors.New("URL sem host")
	}
	if !c.allowHost(u.Host) {
		return nil, fmt.Errorf("host %q não está na allowlist (FETCH_ALLOW_HOST: %s)", u.Host, strings.Join(c.allowed, ", "))
	}
	method := strings.ToUpper(strings.TrimSpace(in.Method))
	if method == "" {
		method = "GET"
	}

	timeout := in.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	start := time.Now()
	hops := []string{u.String()}
	cur := u
	const maxHops = 10
	var timings []hopTiming
	var curl string
	for hop := 0; ; hop++ {
		var rd io.Reader
		if hop == 0 && in.Body != "" {
			rd = strings.NewReader(in.Body)
		}
		req, err := http.NewRequestWithContext(ctx, method, cur.String(), rd)
		if err != nil {
			return nil, fmt.Errorf("montar requisição: %w", err)
		}
		if !in.NoCookies {
			secure := cur.Scheme == "https"
			reqPath := cur.Path
			if reqPath == "" {
				reqPath = "/"
			}
			if cv := c.store.Header(cur.Hostname(), reqPath, secure); len(cv) > 0 {
				req.Header.Set("Cookie", strings.Join(cv, "; "))
			}
		}
		for k, v := range in.Headers {
			req.Header.Set(k, v)
		}
		if hop == 0 && in.Body != "" {
			if req.Header.Get("Content-Type") == "" {
				trimmed := strings.TrimSpace(in.Body)
				switch {
				case strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "["):
					req.Header.Set("Content-Type", "application/json")
				case strings.HasPrefix(trimmed, "<"):
					req.Header.Set("Content-Type", "application/xml")
				default:
					req.Header.Set("Content-Type", "text/plain; charset=utf-8")
				}
			}
		}
		if hop == 0 {
			curl = curlCommand(method, cur.String(), req.Header, in.Body)
		}
		hopStart := time.Now()
		var ht hopTiming
		hopCtx := context.WithValue(ctx, timingKey{}, &ht)
		req = req.WithContext(httptrace.WithClientTrace(hopCtx, traceTTFB(&ht, hopStart)))
		r, err := c.http.Do(req)
		if err != nil {
			return nil, fmt.Errorf("%s %s: %w", method, cur.String(), err)
		}
		if !in.NoCookies {
			if sc := r.Header.Values("Set-Cookie"); len(sc) > 0 {
				var cs []*http.Cookie
				for _, v := range sc {
					if ck, e := http.ParseSetCookie(v); e == nil {
						cs = append(cs, ck)
					}
				}
				c.store.Save(cur.Hostname(), cs)
			}
		}
		if loc := r.Header.Get("Location"); loc != "" && r.StatusCode >= 300 && r.StatusCode < 400 && in.FollowRedirects && hop < maxHops {
			ref, perr := cur.Parse(loc)
			r.Body.Close()
			if perr != nil {
				return nil, fmt.Errorf("Location inválido %q: %w", loc, perr)
			}
			cur = ref
			hops = append(hops, ref.String())
			timings = append(timings, ht)
			continue
		}
		var raw []byte
		trunc := false
		if !in.NoBody {
			lim := c.maxBody + 1
			raw, _ = io.ReadAll(io.LimitReader(r.Body, lim))
			trunc = int64(len(raw)) > c.maxBody
			if trunc {
				raw = raw[:c.maxBody]
			}
		}
		r.Body.Close()
		timings = append(timings, ht)
		return &Resp{
			Method:    method,
			URL:       r.Request.URL.String(),
			Code:      r.StatusCode,
			Status:    r.Status,
			Header:    r.Header.Clone(),
			Body:      raw,
			NoBody:    in.NoBody,
			Truncated: trunc,
			Duration:  time.Since(start),
			Hops:      hops,
			Timings:   timings,
			Curl:      curl,
		}, nil
	}
}
