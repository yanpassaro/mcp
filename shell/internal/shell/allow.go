package shell

import (
	"fmt"
	"os"
	"strings"
)

func LoadPatterns(path string) ([]string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, fmt.Errorf("SHELL_ALLOW_FILE não definido: 'run' exige um allowlist de comandos")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("allowlist %q: %w", path, err)
	}
	patterns := make([]string, 0)
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		patterns = append(patterns, strings.ToLower(line))
	}
	return patterns, nil
}

func Match(patterns []string, command string) (string, bool) {
	command = strings.ToLower(strings.TrimSpace(command))
	for _, p := range patterns {
		if strings.HasSuffix(p, "*") {
			prefix := strings.TrimSuffix(p, "*")
			prefix = strings.TrimRight(prefix, " ")
			if strings.EqualFold(command, prefix) {
				return p, true
			}
			if strings.HasPrefix(command, prefix+" ") {
				return p, true
			}
			continue
		}
		if strings.EqualFold(command, p) {
			return p, true
		}
	}
	return "", false
}
