package ssh

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	xssh "golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

const (
	DEFAULT_PORT            = 22
	DEFAULT_TIMEOUT_SECONDS = 30
)

var attrNames = []string{
	"key_passphrase",
	"timeout_seconds",
	"known_hosts",
	"key_path",
	"password",
	"host",
	"port",
	"user",
}

type Config struct {
	Alias          string
	Host           string
	Port           string
	User           string
	KeyPath        string
	KeyPassphrase  string
	Password       string
	KnownHosts     string
	TimeoutSeconds string

	port    int
	timeout time.Duration
}

func (c Config) PortNumber() int {
	return c.port
}

func (c Config) Timeout() time.Duration {
	return c.timeout
}

func (c Config) Addr() string {
	return net.JoinHostPort(c.Host, strconv.Itoa(c.port))
}

func Discover() ([]Config, error) {
	byAlias := map[string]*Config{}
	for _, entry := range os.Environ() {
		key, value, _ := strings.Cut(entry, "=")
		alias, attr, ok := parseEnvKey(key)
		if !ok {
			continue
		}
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		cfg := byAlias[alias]
		if cfg == nil {
			cfg = &Config{Alias: alias}
			byAlias[alias] = cfg
		}
		cfg.set(attr, trimmed)
	}

	out := make([]Config, 0, len(byAlias))
	for _, cfg := range byAlias {
		if strings.TrimSpace(cfg.Host) == "" {
			continue
		}
		normalized, err := cfg.normalize()
		if err != nil {
			return nil, err
		}
		out = append(out, normalized)
	}
	slices.SortFunc(out, func(a, b Config) int { return cmp.Compare(a.Alias, b.Alias) })
	if len(out) == 0 {
		return nil, errors.New("nenhuma conexão SSH configurada: defina SSH_HOST ou SSH_{ALIAS}_HOST")
	}
	return out, nil
}

func (c *Config) set(attr, value string) {
	switch attr {
	case "host":
		c.Host = value
	case "user":
		c.User = value
	case "port":
		c.Port = value
	case "key_path":
		c.KeyPath = value
	case "key_passphrase":
		c.KeyPassphrase = value
	case "password":
		c.Password = value
	case "known_hosts":
		c.KnownHosts = value
	case "timeout_seconds":
		c.TimeoutSeconds = value
	}
}

func (c *Config) normalize() (Config, error) {
	out := *c
	out.Alias = normalizeAlias(c.Alias)
	out.Host = strings.TrimSpace(c.Host)
	out.User = strings.TrimSpace(c.User)
	if out.User == "" {
		out.User = cmp.Or(strings.TrimSpace(os.Getenv("USER")), strings.TrimSpace(os.Getenv("USERNAME")))
	}
	if out.User == "" {
		return Config{}, fmt.Errorf("conexão %q sem usuário: defina %s", Label(out.Alias), envName(out.Alias, "USER"))
	}
	port, err := parsePort(c.Port, out.Alias)
	if err != nil {
		return Config{}, err
	}
	out.port = port
	timeout, err := parseTimeout(c.TimeoutSeconds, out.Alias)
	if err != nil {
		return Config{}, err
	}
	out.timeout = timeout
	out.KeyPath = expandHome(strings.TrimSpace(c.KeyPath))
	out.KnownHosts = expandHome(strings.TrimSpace(c.KnownHosts))
	return out, nil
}

func parsePort(value, alias string) (int, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return DEFAULT_PORT, nil
	}
	port, err := strconv.Atoi(trimmed)
	if err != nil {
		return 0, fmt.Errorf("%s inválida: %w", envName(alias, "PORT"), err)
	}
	if port <= 0 {
		return 0, fmt.Errorf("%s inválida: %d", envName(alias, "PORT"), port)
	}
	return port, nil
}

func parseTimeout(value, alias string) (time.Duration, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return DEFAULT_TIMEOUT_SECONDS * time.Second, nil
	}
	seconds, err := strconv.Atoi(trimmed)
	if err != nil {
		return 0, fmt.Errorf("%s inválido: %w", envName(alias, "TIMEOUT_SECONDS"), err)
	}
	if seconds <= 0 {
		return 0, fmt.Errorf("%s inválido: %d", envName(alias, "TIMEOUT_SECONDS"), seconds)
	}
	return time.Duration(seconds) * time.Second, nil
}

func parseEnvKey(key string) (alias, attr string, ok bool) {
	up := strings.ToUpper(strings.TrimSpace(key))
	if !strings.HasPrefix(up, "SSH_") {
		return "", "", false
	}
	rest := strings.TrimPrefix(up, "SSH_")
	for _, attr := range attrNames {
		name := strings.ToUpper(attr)
		if rest == name {
			return "", attr, true
		}
		suffix := fmt.Sprintf("_%s", name)
		if strings.HasSuffix(rest, suffix) {
			return normalizeAlias(strings.TrimSuffix(rest, suffix)), attr, true
		}
	}
	return "", "", false
}

func normalizeAlias(prefix string) string {
	a := strings.ToLower(strings.TrimSpace(prefix))
	if a == "" {
		return ""
	}
	if a == "ssh" {
		return ""
	}
	b := strings.Builder{}
	for _, r := range a {
		if isLowerAlphaNum(r) {
			b.WriteRune(r)
			continue
		}
		b.WriteRune('_')
	}
	return strings.Trim(b.String(), "_")
}

func isLowerAlphaNum(r rune) bool {
	if r >= 'a' {
		if r <= 'z' {
			return true
		}
	}
	if r >= '0' {
		if r <= '9' {
			return true
		}
	}
	return false
}

func Label(alias string) string {
	return cmp.Or(alias, "default")
}

func envName(alias, attr string) string {
	if alias == "" {
		return fmt.Sprintf("SSH_%s", attr)
	}
	return fmt.Sprintf("SSH_%s_%s", strings.ToUpper(alias), attr)
}

func expandHome(path string) string {
	if !strings.HasPrefix(path, "~") {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	return filepath.Join(home, strings.TrimPrefix(strings.TrimPrefix(path, "~"), string(os.PathSeparator)))
}

func Dial(ctx context.Context, cfg Config) (*xssh.Client, error) {
	auths, err := authMethods(cfg)
	if err != nil {
		return nil, err
	}
	callback, err := hostKeyCallback(cfg)
	if err != nil {
		return nil, err
	}
	clientConfig := &xssh.ClientConfig{
		User:            cfg.User,
		Auth:            auths,
		HostKeyCallback: callback,
		Timeout:         cfg.Timeout(),
	}
	dialer := net.Dialer{Timeout: cfg.Timeout()}
	conn, err := dialer.DialContext(ctx, "tcp", cfg.Addr())
	if err != nil {
		return nil, fmt.Errorf("conectar %s: %w", cfg.Addr(), err)
	}
	clientConn, chans, reqs, err := xssh.NewClientConn(conn, cfg.Addr(), clientConfig)
	if err != nil {
		closeErr := conn.Close()
		if closeErr != nil {
			return nil, fmt.Errorf("handshake %s: %w (fechar conexão: %v)", cfg.Addr(), err, closeErr)
		}
		return nil, fmt.Errorf("handshake %s: %w", cfg.Addr(), err)
	}
	return xssh.NewClient(clientConn, chans, reqs), nil
}

func authMethods(cfg Config) ([]xssh.AuthMethod, error) {
	if cfg.KeyPath != "" {
		key, err := os.ReadFile(cfg.KeyPath)
		if err != nil {
			return nil, fmt.Errorf("ler chave %s: %w", cfg.KeyPath, err)
		}
		if cfg.KeyPassphrase != "" {
			signer, err := xssh.ParsePrivateKeyWithPassphrase(key, []byte(cfg.KeyPassphrase))
			if err != nil {
				return nil, fmt.Errorf("parse chave %s com %s: %w", cfg.KeyPath, envName(cfg.Alias, "KEY_PASSPHRASE"), err)
			}
			return []xssh.AuthMethod{xssh.PublicKeys(signer)}, nil
		}
		signer, err := xssh.ParsePrivateKey(key)
		if err != nil {
			var missing *xssh.PassphraseMissingError
			if errors.As(err, &missing) {
				return nil, fmt.Errorf("chave %s protegida por passphrase: defina %s", cfg.KeyPath, envName(cfg.Alias, "KEY_PASSPHRASE"))
			}
			return nil, fmt.Errorf("parse chave %s: %w", cfg.KeyPath, err)
		}
		return []xssh.AuthMethod{xssh.PublicKeys(signer)}, nil
	}
	if cfg.Password != "" {
		return []xssh.AuthMethod{xssh.Password(cfg.Password)}, nil
	}
	return nil, fmt.Errorf("conexão %q sem credencial: defina %s ou %s", Label(cfg.Alias), envName(cfg.Alias, "KEY_PATH"), envName(cfg.Alias, "PASSWORD"))
}

func hostKeyCallback(cfg Config) (xssh.HostKeyCallback, error) {
	path := cfg.KnownHosts
	if path == "" {
		path = defaultKnownHosts()
	}
	callback, err := knownhosts.New(path)
	if err != nil {
		return nil, fmt.Errorf("known_hosts %s: %w (rode ssh-keyscan -H %s >> %s)", path, err, cfg.Host, path)
	}
	return callback, nil
}

func defaultKnownHosts() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".ssh", "known_hosts")
	}
	return filepath.Join(home, ".ssh", "known_hosts")
}
