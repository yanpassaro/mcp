package shell

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

const (
	DEFAULT_MAX_LOG  = 512 * 1024
	DEFAULT_LOG_TAIL = 8 * 1024
)

type Result struct {
	Command  string
	Cwd      string
	ExitCode int
	Output   string
}

type Info struct {
	ID      string
	Command string
	Cwd     string
	Started time.Time
	Status  string
	Exit    int
}

type Log struct {
	ID        string
	Command   string
	Started   time.Time
	Status    string
	Exit      int
	Output    string
	Truncated bool
}

type proc struct {
	id      string
	command string
	cwd     string
	started time.Time
	cmd     *exec.Cmd
	out     *ring
	done    chan struct{}
}

type ring struct {
	mu   sync.Mutex
	data []byte
	max  int
	exit int
}

type Registry struct {
	mu    sync.Mutex
	seq   int
	procs map[string]*proc
}

func NewRegistry() *Registry {
	return &Registry{procs: map[string]*proc{}}
}

func (r *Registry) ForegroundFields(ctx context.Context, command string, fields []string, cwd string, env []string) (Result, error) {
	cmd := exec.CommandContext(ctx, fields[0], fields[1:]...)
	applyAttr(cmd)
	out := newRing(DEFAULT_MAX_LOG)
	cmd.Stdout = out
	cmd.Stderr = out
	cmd.Dir = cwd
	if len(env) != 0 {
		cmd.Env = append(os.Environ(), env...)
	}
	err := cmd.Run()
	return Result{Command: command, Cwd: cwd, ExitCode: out.exit, Output: out.text()}, err
}

func (r *Registry) BackgroundFields(command string, fields []string, cwd string, env []string) (string, error) {
	p := &proc{
		command: strings.TrimSpace(command),
		cwd:     cwd,
		started: time.Now(),
		out:     newRing(DEFAULT_MAX_LOG),
		done:    make(chan struct{}),
	}
	p.cmd = exec.Command(fields[0], fields[1:]...)
	applyAttr(p.cmd)
	p.cmd.Stdout = p.out
	p.cmd.Stderr = p.out
	p.cmd.Dir = cwd
	if len(env) != 0 {
		p.cmd.Env = append(os.Environ(), env...)
	}
	if err := p.cmd.Start(); err != nil {
		return "", fmt.Errorf("iniciar %q: %w", p.command, err)
	}
	r.mu.Lock()
	r.seq++
	p.id = fmt.Sprintf("p%d", r.seq)
	r.procs[p.id] = p
	r.mu.Unlock()
	go func() {
		waitErr := p.cmd.Wait()
		p.out.mu.Lock()
		p.out.exit = exitCode(waitErr)
		p.out.mu.Unlock()
		close(p.done)
	}()
	return p.id, nil
}

func (r *Registry) Kill(id string) (Info, error) {
	p, err := r.get(id)
	if err != nil {
		return Info{}, err
	}
	select {
	case <-p.done:
		return info(p), fmt.Errorf("processo %s já terminou (exit %d)", p.id, p.out.exit)
	default:
	}
	if err := killTree(p.cmd.Process); err != nil {
		return info(p), fmt.Errorf("matar %s: %w", p.id, err)
	}
	select {
	case <-p.done:
	case <-time.After(10 * time.Second):
		return info(p), fmt.Errorf("processo %s não terminou após o kill", p.id)
	}
	return info(p), nil
}

func (r *Registry) List() []Info {
	r.mu.Lock()
	ids := make([]string, 0, len(r.procs))
	for _, p := range r.procs {
		ids = append(ids, p.id)
	}
	r.mu.Unlock()
	items := make([]Info, 0, len(ids))
	for _, id := range ids {
		p, err := r.get(id)
		if err != nil {
			continue
		}
		items = append(items, info(p))
	}
	return items
}

func (r *Registry) Log(id string, max int) (Log, error) {
	p, err := r.get(id)
	if err != nil {
		return Log{}, err
	}
	p.out.mu.Lock()
	data := p.out.tail(max)
	exit := p.out.exit
	p.out.mu.Unlock()
	status := "running"
	select {
	case <-p.done:
		status = fmt.Sprintf("exit %d", exit)
	default:
	}
	truncated := false
	if max > 0 {
		truncated = len(data) >= max
	}
	return Log{ID: p.id, Command: p.command, Started: p.started, Status: status, Exit: exit, Output: string(data), Truncated: truncated}, nil
}

func (r *Registry) get(id string) (*proc, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, errors.New("id é obrigatório")
	}
	r.mu.Lock()
	p, ok := r.procs[id]
	r.mu.Unlock()
	if !ok {
		return nil, fmt.Errorf("processo %q não encontrado; use 'list' no shell_read", id)
	}
	return p, nil
}

func info(p *proc) Info {
	status := "running"
	p.out.mu.Lock()
	exit := p.out.exit
	p.out.mu.Unlock()
	select {
	case <-p.done:
		status = fmt.Sprintf("exit %d", exit)
	default:
	}
	return Info{ID: p.id, Command: p.command, Cwd: p.cwd, Started: p.started, Status: status, Exit: exit}
}

func exitCode(err error) int {
	if err == nil {
		return 0
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return ee.ExitCode()
	}
	return -1
}

func newRing(max int) *ring {
	return &ring{max: max}
}

func (r *ring) Write(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.data = append(r.data, p...)
	if len(r.data) > r.max {
		r.data = r.data[len(r.data)-r.max:]
	}
	return len(p), nil
}

func (r *ring) tail(max int) []byte {
	if max <= 0 {
		max = DEFAULT_LOG_TAIL
	}
	if max > len(r.data) {
		max = len(r.data)
	}
	out := make([]byte, max)
	copy(out, r.data[len(r.data)-max:])
	return out
}

func (r *ring) text() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return string(r.data)
}
