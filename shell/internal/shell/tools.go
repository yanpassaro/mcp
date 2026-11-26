package shell

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
)

type Tool struct {
	Name       string `json:"name"`
	Desc       string `json:"desc"`
	Cmd        string `json:"cmd"`
	Background bool   `json:"background,omitempty"`
}

const TOOL_NAME_RE = `^[a-z][a-z0-9_-]{0,31}$`
const PLACEHOLDER_RE = `\$\{([a-z][a-z0-9_]*)(\?)?\}`

var toolNameRE = regexp.MustCompile(TOOL_NAME_RE)
var placeholderRE = regexp.MustCompile(PLACEHOLDER_RE)

type fieldSpec struct {
	Name       string
	Optional   bool
}

func LoadTools(path string) ([]Tool, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("tools %q: %w", path, err)
	}
	var tools []Tool
	if err := json.Unmarshal(data, &tools); err != nil {
		return nil, fmt.Errorf("tools %q: JSON inválido: %w", path, err)
	}
	seen := make(map[string]bool, len(tools))
	for i, t := range tools {
		name := strings.TrimSpace(t.Name)
		if !toolNameRE.MatchString(name) {
			return nil, fmt.Errorf("tools %q: item %d: name inválido %q (use ^%s)", path, i, t.Name, TOOL_NAME_RE)
		}
		if seen[name] {
			return nil, fmt.Errorf("tools %q: name duplicado %q", path, name)
		}
		seen[name] = true
		if strings.TrimSpace(t.Cmd) == "" {
			return nil, fmt.Errorf("tools %q: item %d (%s): cmd é obrigatório", path, i, name)
		}
		if err := checkTemplate(t.Cmd); err != nil {
			return nil, fmt.Errorf("tools %q: item %d (%s): %w", path, i, name, err)
		}
	}
	return tools, nil
}

func checkTemplate(cmd string) error {
	fields := strings.Fields(cmd)
	if len(fields) == 0 {
		return fmt.Errorf("cmd vazio")
	}
	if len(fields) == 1 && placeholderRE.MatchString(fields[0]) {
		return fmt.Errorf("cmd não pode começar por placeholder; o executável é fixo")
	}
	return nil
}

func placeholders(cmd string) []fieldSpec {
	matches := placeholderRE.FindAllStringSubmatch(cmd, -1)
	specs := make([]fieldSpec, 0, len(matches))
	for _, m := range matches {
		specs = append(specs, fieldSpec{Name: m[1], Optional: m[2] == "?"})
	}
	return specs
}

func Render(cmd string, values map[string]string) ([]string, error) {
	out := make([]string, 0, 8)
	for _, field := range strings.Fields(cmd) {
		if !placeholderRE.MatchString(field) {
			out = append(out, field)
			continue
		}
		m := placeholderRE.FindStringSubmatch(field)
		name, optional := m[1], m[2] == "?"
		value, ok := values[name]
		value = strings.TrimSpace(value)
		if !ok || value == "" {
			if optional {
				continue
			}
			return nil, fmt.Errorf("campo %q é obrigatório", name)
		}
		if strings.Contains(value, "\n") {
			return nil, fmt.Errorf("campo %q não pode ter quebra de linha", name)
		}
		out = append(out, value)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("comando ficou vazio")
	}
	return out, nil
}

func FieldsOf(cmd string) []fieldSpec {
	return placeholders(cmd)
}
