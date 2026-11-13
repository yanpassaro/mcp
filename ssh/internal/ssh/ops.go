package ssh

import (
	"bytes"
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	xssh "golang.org/x/crypto/ssh"
)

const (
	DEFAULT_READ_BYTES = 256 * 1024
	MAX_READ_BYTES     = 32 * 1024 * 1024
	PROBE_COMMAND      = "uname -srm; echo; uptime; echo; df -h /; echo; (ss -ltn 2>/dev/null || netstat -ltn 2>/dev/null) || true"
)

type Result struct {
	ExitCode int
	Stdout   string
	Stderr   string
}

func Run(ctx context.Context, cfg Config, command, workdir string) (Result, error) {
	client, err := Dial(ctx, cfg)
	if err != nil {
		return Result{}, err
	}
	result, runErr := runCommand(ctx, client, command, workdir, nil)
	closeErr := client.Close()
	if runErr != nil {
		return result, runErr
	}
	if closeErr != nil {
		return result, fmt.Errorf("fechar conexão: %w", closeErr)
	}
	return result, nil
}

func runCommand(ctx context.Context, client *xssh.Client, command, workdir string, stdin io.Reader) (Result, error) {
	session, err := client.NewSession()
	if err != nil {
		return Result{}, fmt.Errorf("abrir sessão: %w", err)
	}
	full := command
	if workdir != "" {
		full = fmt.Sprintf("cd %s && %s", shellQuote(workdir), command)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr
	if stdin != nil {
		session.Stdin = stdin
	}

	done := make(chan error, 1)
	go func() { done <- session.Run(full) }()
	select {
	case runErr := <-done:
		result := Result{Stdout: stdout.String(), Stderr: stderr.String()}
		if runErr == nil {
			return result, nil
		}
		var exitErr *xssh.ExitError
		if errors.As(runErr, &exitErr) {
			result.ExitCode = exitErr.ExitStatus()
			return result, nil
		}
		result.ExitCode = -1
		return result, fmt.Errorf("executar comando: %w", runErr)
	case <-ctx.Done():
		closeErr := session.Close()
		<-done
		result := Result{Stdout: stdout.String(), Stderr: stderr.String()}
		if closeErr != nil {
			return result, fmt.Errorf("%w (encerrar sessão: %v)", ctx.Err(), closeErr)
		}
		return result, ctx.Err()
	}
}

func Read(ctx context.Context, cfg Config, path string, maxBytes int) ([]byte, bool, error) {
	err := checkRead(path)
	if err != nil {
		return nil, false, err
	}
	limit := clampBytes(maxBytes)
	command := fmt.Sprintf("head -c %d -- %s", limit+1, shellQuote(strings.TrimSpace(path)))
	result, err := Run(ctx, cfg, command, "")
	if err != nil {
		return nil, false, err
	}
	if result.ExitCode != 0 {
		return nil, false, fmt.Errorf("ler %s: %s", path, firstLine(result))
	}
	data := []byte(result.Stdout)
	if len(data) > limit {
		return data[:limit], true, nil
	}
	return data, false, nil
}

func Write(ctx context.Context, cfg Config, path string, src io.Reader, appendMode bool) (int64, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return 0, errors.New("path é obrigatório")
	}
	operator := ">"
	if appendMode {
		operator = ">>"
	}
	command := fmt.Sprintf("cat %s %s", operator, shellQuote(trimmed))
	client, err := Dial(ctx, cfg)
	if err != nil {
		return 0, err
	}
	counter := &countingReader{reader: src}
	result, runErr := runCommand(ctx, client, command, "", counter)
	closeErr := client.Close()
	if runErr != nil {
		return counter.count, runErr
	}
	if result.ExitCode != 0 {
		return counter.count, fmt.Errorf("escrever %s: %s", trimmed, firstLine(result))
	}
	if closeErr != nil {
		return counter.count, fmt.Errorf("fechar conexão: %w", closeErr)
	}
	return counter.count, nil
}

func Probe(ctx context.Context, cfg Config) (Result, error) {
	return Run(ctx, cfg, PROBE_COMMAND, "")
}

func checkRead(path string) error {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return errors.New("path é obrigatório")
	}
	base := strings.ToLower(filepath.Base(trimmed))
	if base == ".env" {
		return errors.New("leitura de .env bloqueada")
	}
	if strings.HasPrefix(base, ".env.") {
		return errors.New("leitura de .env bloqueada")
	}
	if strings.HasSuffix(base, ".env") {
		return errors.New("leitura de .env bloqueada")
	}
	if strings.HasSuffix(base, "environ") {
		return errors.New("leitura de environ bloqueada")
	}
	return nil
}

func shellQuote(value string) string {
	escaped := strings.ReplaceAll(value, "'", `'\''`)
	return fmt.Sprintf("'%s'", escaped)
}

func clampBytes(value int) int {
	if value <= 0 {
		return DEFAULT_READ_BYTES
	}
	if value > MAX_READ_BYTES {
		return MAX_READ_BYTES
	}
	return value
}

func firstLine(result Result) string {
	text := cmp.Or(strings.TrimSpace(result.Stderr), strings.TrimSpace(result.Stdout))
	line, _, _ := strings.Cut(text, "\n")
	return line
}

type countingReader struct {
	reader io.Reader
	count  int64
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.reader.Read(p)
	c.count += int64(n)
	return n, err
}
