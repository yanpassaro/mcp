package shell

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

type Pattern struct {
	re *regexp.Regexp
}

func (p Pattern) String() string {
	return p.re.String()
}

func LoadPatterns(path string) ([]Pattern, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, fmt.Errorf("SHELL_ALLOW_FILE não definido: 'run' exige um allowlist de comandos")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("allowlist %q: %w", path, err)
	}
	patterns := make([]Pattern, 0)
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		re, err := regexp.Compile(line)
		if err != nil {
			return nil, fmt.Errorf("allowlist %q: padrão inválido %q: %w", path, line, err)
		}
		patterns = append(patterns, Pattern{re: re})
	}
	return patterns, nil
}

func Match(patterns []Pattern, command string) (string, bool) {
	command = strings.TrimSpace(command)
	for _, p := range patterns {
		if p.re.MatchString(command) {
			return p.re.String(), true
		}
	}
	return "", false
}
