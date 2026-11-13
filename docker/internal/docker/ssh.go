package docker

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/moby/moby/client"
	xssh "golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

const (
	DEFAULT_SSH_PORT      = 22
	DEFAULT_DOCKER_SOCKET = "/var/run/docker.sock"
	SSH_DIAL_TIMEOUT      = 15 * time.Second
	SSH_PLACEHOLDER_HOST  = "tcp://docker.invalid"
)

type sshTarget struct {
	host       string
	port       int
	user       string
	socket     string
	keyPath    string
	passphrase string
	password   string
	knownHosts string
}

func (t sshTarget) addr() string {
	return net.JoinHostPort(t.host, strconv.Itoa(t.port))
}

func (t sshTarget) label() string {
	return fmt.Sprintf("%s@%s", t.user, t.addr())
}

type sshDialer struct {
	target sshTarget

	mu     sync.Mutex
	client *xssh.Client
}

func isSSHHost(raw string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(raw)), "ssh://")
}

func newSSHClient(raw string) (*client.Client, error) {
	target, err := parseSSHTarget(raw)
	if err != nil {
		return nil, err
	}
	dialer := &sshDialer{target: target}
	return client.New(
		client.WithTLSClientConfigFromEnv(),
		client.WithAPIVersionFromEnv(),
		client.WithHost(SSH_PLACEHOLDER_HOST),
		client.WithDialContext(dialer.DialContext),
	)
}

func parseSSHTarget(raw string) (sshTarget, error) {
	trimmed := strings.TrimSpace(raw)
	if !isSSHHost(trimmed) {
		return sshTarget{}, fmt.Errorf("host ssh inválido: %s", raw)
	}
	rest := trimmed[len("ssh://"):]
	socket := ""
	if idx := strings.Index(rest, "/"); idx >= 0 {
		socket = rest[idx:]
		rest = rest[:idx]
	}
	user := ""
	hostPart := rest
	if idx := strings.LastIndex(rest, "@"); idx >= 0 {
		user = rest[:idx]
		hostPart = rest[idx+1:]
	}
	host := hostPart
	port := DEFAULT_SSH_PORT
	if idx := strings.LastIndex(hostPart, ":"); idx >= 0 {
		host = hostPart[:idx]
		parsed, err := strconv.Atoi(hostPart[idx+1:])
		if err != nil {
			return sshTarget{}, fmt.Errorf("porta ssh inválida em %s: %w", raw, err)
		}
		if parsed <= 0 {
			return sshTarget{}, fmt.Errorf("porta ssh inválida em %s: %d", raw, parsed)
		}
		port = parsed
	}
	if host == "" {
		return sshTarget{}, fmt.Errorf("host ssh inválido: %s", raw)
	}
	target := sshTarget{
		host:       host,
		port:       port,
		user:       cmp.Or(strings.TrimSpace(user), currentUser()),
		socket:     cmp.Or(socket, DEFAULT_DOCKER_SOCKET),
		keyPath:    cmp.Or(strings.TrimSpace(os.Getenv("DOCKER_SSH_KEY_PATH")), defaultKeyPath()),
		passphrase: os.Getenv("DOCKER_SSH_KEY_PASSPHRASE"),
		password:   os.Getenv("DOCKER_SSH_PASSWORD"),
		knownHosts: cmp.Or(strings.TrimSpace(os.Getenv("DOCKER_SSH_KNOWN_HOSTS")), defaultKnownHosts()),
	}
	if target.user == "" {
		return sshTarget{}, fmt.Errorf("usuário ssh não definido para %s: use ssh://usuario@host", raw)
	}
	return target, nil
}

func (d *sshDialer) DialContext(ctx context.Context, _, _ string) (net.Conn, error) {
	sshClient, err := d.sshClient(ctx)
	if err != nil {
		return nil, err
	}
	conn, err := sshClient.Dial("unix", d.target.socket)
	if err != nil {
		return nil, fmt.Errorf("abrir %s em %s: %w", d.target.socket, d.target.label(), err)
	}
	return conn, nil
}

func (d *sshDialer) sshClient(ctx context.Context) (*xssh.Client, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.client != nil {
		return d.client, nil
	}
	client, err := dialSSH(ctx, d.target)
	if err != nil {
		return nil, err
	}
	d.client = client
	return client, nil
}

func dialSSH(ctx context.Context, target sshTarget) (*xssh.Client, error) {
	auths, err := sshAuth(target)
	if err != nil {
		return nil, err
	}
	callback, err := sshHostKey(target)
	if err != nil {
		return nil, err
	}
	config := &xssh.ClientConfig{
		User:            target.user,
		Auth:            auths,
		HostKeyCallback: callback,
		Timeout:         SSH_DIAL_TIMEOUT,
	}
	dialer := net.Dialer{Timeout: SSH_DIAL_TIMEOUT}
	conn, err := dialer.DialContext(ctx, "tcp", target.addr())
	if err != nil {
		return nil, fmt.Errorf("conectar %s: %w", target.addr(), err)
	}
	clientConn, chans, reqs, err := xssh.NewClientConn(conn, target.addr(), config)
	if err != nil {
		closeErr := conn.Close()
		if closeErr != nil {
			return nil, fmt.Errorf("handshake %s: %w (fechar conexão: %v)", target.addr(), err, closeErr)
		}
		return nil, fmt.Errorf("handshake %s: %w", target.addr(), err)
	}
	return xssh.NewClient(clientConn, chans, reqs), nil
}

func sshAuth(target sshTarget) ([]xssh.AuthMethod, error) {
	if target.keyPath != "" {
		key, err := os.ReadFile(target.keyPath)
		if err != nil {
			return nil, fmt.Errorf("ler chave %s: %w (defina DOCKER_SSH_KEY_PATH)", target.keyPath, err)
		}
		if target.passphrase != "" {
			signer, err := xssh.ParsePrivateKeyWithPassphrase(key, []byte(target.passphrase))
			if err != nil {
				return nil, fmt.Errorf("parse chave %s com DOCKER_SSH_KEY_PASSPHRASE: %w", target.keyPath, err)
			}
			return []xssh.AuthMethod{xssh.PublicKeys(signer)}, nil
		}
		signer, err := xssh.ParsePrivateKey(key)
		if err != nil {
			var missing *xssh.PassphraseMissingError
			if errors.As(err, &missing) {
				return nil, fmt.Errorf("chave %s protegida por passphrase: defina DOCKER_SSH_KEY_PASSPHRASE", target.keyPath)
			}
			return nil, fmt.Errorf("parse chave %s: %w", target.keyPath, err)
		}
		return []xssh.AuthMethod{xssh.PublicKeys(signer)}, nil
	}
	if target.password != "" {
		return []xssh.AuthMethod{xssh.Password(target.password)}, nil
	}
	return nil, errors.New("sem credencial ssh: defina DOCKER_SSH_KEY_PATH ou DOCKER_SSH_PASSWORD")
}

func sshHostKey(target sshTarget) (xssh.HostKeyCallback, error) {
	callback, err := knownhosts.New(target.knownHosts)
	if err != nil {
		return nil, fmt.Errorf("known_hosts %s: %w (rode ssh-keyscan -H %s >> %s)", target.knownHosts, err, target.host, target.knownHosts)
	}
	return callback, nil
}

func currentUser() string {
	return cmp.Or(strings.TrimSpace(os.Getenv("USER")), strings.TrimSpace(os.Getenv("USERNAME")))
}

func defaultKeyPath() string {
	for _, name := range []string{"id_ed25519", "id_rsa", "id_ecdsa"} {
		path := filepath.Join(sshDir(), name)
		_, err := os.Stat(path)
		if err == nil {
			return path
		}
	}
	return ""
}

func defaultKnownHosts() string {
	return filepath.Join(sshDir(), "known_hosts")
}

func sshDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".ssh"
	}
	return filepath.Join(home, ".ssh")
}
