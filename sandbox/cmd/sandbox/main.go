package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"ntdsk.com/mcp/sandbox/internal/sandbox"
)

func main() {
	if mb := memLimitMB(); mb > 0 {
		debug.SetMemoryLimit(int64(mb) * 1024 * 1024)
	}

	script, positional, argsJSON, timeoutSec, asJSON, help, err := parseCLI(os.Args[1:])
	if help {
		usage()
		os.Exit(0)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "sandbox: %v\n\n", err)
		usage()
		os.Exit(2)
	}
	if script == "" {
		usage()
		os.Exit(2)
	}

	code, err := os.ReadFile(script)
	if err != nil {
		fmt.Fprintf(os.Stderr, "sandbox: ler script %q: %v\n", script, err)
		os.Exit(1)
	}

	args := argsJSON
	if args == "" {
		if len(positional) > 0 {
			if b, err := json.Marshal(positional); err == nil {
				args = string(b)
			}
		} else {
			args = "{}"
		}
	}

	mnt := openStore("mnt", mntDir(), 256, 5000)
	tmp := openStore("tmp", tmpDir(), 64, 1000)
	if n, err := tmp.Clear(); err == nil && n > 0 {
		fmt.Fprintf(os.Stderr, "sandbox: tmp limpo (%d arquivo(s))\n", n)
	}

	res, runErr := sandbox.Run(mnt, tmp, sandbox.RunRequest{
		Code:    string(code),
		Args:    args,
		Timeout: time.Duration(timeoutSec) * time.Second,
	})

	if asJSON {
		dataVal := any(res.Data)
		if res.DataJSON && res.Data != "" {
			var parsed any
			if err := json.Unmarshal([]byte(res.Data), &parsed); err == nil {
				dataVal = parsed
			}
		}
		out := map[string]any{
			"ok":          res.Ok && runErr == nil,
			"name":        res.Name,
			"desc":        res.Description,
			"output":      res.Output,
			"data":        dataVal,
			"data_json":   res.DataJSON,
			"data_md":     res.DataMarkdown,
			"duration_ms": float64(res.Duration.Milliseconds()),
			"truncated":   res.Truncated,
		}
		if res.Error != "" {
			out["error"] = res.Error
		}
		if runErr != nil {
			out["run_error"] = runErr.Error()
		}
		if b, err := json.MarshalIndent(out, "", "  "); err == nil {
			fmt.Println(string(b))
		}
		os.Exit(exitCode(res.Ok && runErr == nil))
	}

	if !res.Ok || runErr != nil {
		msg := res.Error
		if msg == "" && runErr != nil {
			msg = runErr.Error()
		}
		fmt.Fprintf(os.Stderr, "✗ %s\n", msg)
		if out := strings.TrimRight(res.Output, "\n"); out != "" {
			fmt.Fprintf(os.Stderr, "%s\n", out)
		}
		os.Exit(1)
	}

	if out := strings.TrimRight(res.Output, "\n"); out != "" {
		fmt.Println(out)
	}
	if res.Data != "" {
		fmt.Println(res.Data)
	} else {
		fmt.Println("_(sem resultado)_")
	}
}

func parseCLI(args []string) (script string, positional []string, argsJSON string, timeout int, asJSON, help bool, err error) {
	i := 0
	for i < len(args) {
		a := args[i]
		switch {
		case a == "--json" || a == "-json":
			asJSON = true
			i++
		case a == "--help" || a == "-h" || a == "-help":
			help = true
			return
		case strings.HasPrefix(a, "--args="):
			argsJSON = strings.TrimPrefix(a, "--args=")
			i++
		case a == "--args" || a == "-args":
			if i+1 >= len(args) {
				err = fmt.Errorf("--args requer um valor")
				return
			}
			argsJSON = args[i+1]
			i += 2
		case strings.HasPrefix(a, "--timeout="):
			timeout, _ = strconv.Atoi(strings.TrimPrefix(a, "--timeout="))
			i++
		case a == "--timeout" || a == "-timeout":
			if i+1 >= len(args) {
				err = fmt.Errorf("--timeout requer um valor")
				return
			}
			timeout, _ = strconv.Atoi(args[i+1])
			i += 2
		case strings.HasPrefix(a, "-"):
			err = fmt.Errorf("opção desconhecida: %q", a)
			return
		default:
			if script == "" {
				script = a
			} else {
				positional = append(positional, a)
			}
			i++
		}
	}
	return
}

func usage() {
	fmt.Fprintln(os.Stderr, "Usage: sandbox.exe [options] <script.lua> [args...]")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Options:")
	fmt.Fprintln(os.Stderr, "  --args '<json>'   JSON passed to the script as `std.args`")
	fmt.Fprintln(os.Stderr, "  --timeout <sec>   execution timeout (default: env SANDBOX_EXEC_TIMEOUT_SECONDS)")
	fmt.Fprintln(os.Stderr, "  --json            print the whole result as JSON")
	fmt.Fprintln(os.Stderr, "  -h, --help        show this help")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Extra positional arguments are passed to `std.args` as an array.")
}

func openStore(label, dir string, spaceMB, maxFiles int) *sandbox.Store {
	_ = os.MkdirAll(dir, 0o755)
	s := sandbox.NewStore(dir)
	s.MaxTotalBytes = int64(envInt("SANDBOX_"+strings.ToUpper(label)+"_SPACE_MB", spaceMB)) * 1024 * 1024
	s.MaxFiles = envInt("SANDBOX_"+strings.ToUpper(label)+"_FILES", maxFiles)
	return s
}

func envInt(name string, def int) int {
	if v := os.Getenv(name); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return def
}

func memLimitMB() int64 {
	return int64(envInt("SANDBOX_MEM_LIMIT_MB", 512))
}

func exitCode(ok bool) int {
	if ok {
		return 0
	}
	return 1
}

func mntDir() string {
	return filepath.Join(userStateDir(), "mnt")
}

func tmpDir() string {
	return filepath.Join(userStateDir(), "tmp")
}

func userStateDir() string {
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".local", "state", "mcp")
	}
	return filepath.Join(os.Getenv("USERPROFILE"), ".local", "state", "mcp")
}
