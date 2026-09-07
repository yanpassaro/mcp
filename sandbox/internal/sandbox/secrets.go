package sandbox

import (
	"os"
	"sort"
	"strings"

	lua "github.com/Shopify/go-lua"
)

const (
	secretEnvPrefix = "SECRET_"
	redactedMarker  = "[REDACTED]"
)

// Secrets exposes registered secrets (environment variables prefixed with
// SECRET_) to scripts, and redacts their values from any output/return so they
// never reach the AI.
type Secrets struct {
	byKey map[string]string
	vals  []string
}

// LoadSecrets builds a Secret registry by inspecting SECRET_* environment
// variables. Keys are normalized to lowercase with the prefix stripped, so
// SECRET_GITHUB_TOKEN_API is reachable as "github_token_api".
func LoadSecrets() *Secrets {
	s := &Secrets{byKey: map[string]string{}}
	seen := map[string]bool{}
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(strings.ToUpper(kv), secretEnvPrefix) {
			continue
		}
		i := strings.IndexByte(kv, '=')
		if i < 0 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(kv[len(secretEnvPrefix):i]))
		val := kv[i+1:]
		if key == "" || val == "" {
			continue
		}
		if !seen[val] {
			seen[val] = true
			s.vals = append(s.vals, val)
		}
		s.byKey[key] = val
	}
	// Replace longest values first so overlapping secrets are fully hidden.
	sort.Slice(s.vals, func(i, j int) bool { return len(s.vals[i]) > len(s.vals[j]) })
	return s
}

// Get returns the secret registered under key (case-insensitive).
func (s *Secrets) Get(key string) (string, bool) {
	if s == nil {
		return "", false
	}
	v, ok := s.byKey[strings.ToLower(strings.TrimSpace(key))]
	return v, ok
}

// Has reports whether a secret key exists (without revealing its value).
func (s *Secrets) Has(key string) bool {
	if s == nil {
		return false
	}
	_, ok := s.byKey[strings.ToLower(strings.TrimSpace(key))]
	return ok
}

// Redact replaces every known secret value in text with a redaction marker.
func (s *Secrets) Redact(text string) string {
	if s == nil || text == "" {
		return text
	}
	for _, v := range s.vals {
		if v != "" && strings.Contains(text, v) {
			text = strings.ReplaceAll(text, v, redactedMarker)
		}
	}
	return text
}

// RedactValue recursively redacts strings inside a Lua-derived value (maps,
// slices, strings) so that secrets are hidden even after JSON marshaling.
func (s *Secrets) RedactValue(v any) any {
	if s == nil {
		return v
	}
	switch x := v.(type) {
	case string:
		return s.Redact(x)
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, val := range x {
			out[s.Redact(k)] = s.RedactValue(val)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, val := range x {
			out[i] = s.RedactValue(val)
		}
		return out
	default:
		return v
	}
}

func buildSecrets(L *lua.State, s *Secrets) int {
	t := newTable(L)
	setGoFunc(L, t, "get", func(l *lua.State) int {
		if v, ok := s.Get(argString(l, 1)); ok {
			l.PushString(v)
		} else {
			l.PushNil()
		}
		return 1
	})
	setGoFunc(L, t, "has", func(l *lua.State) int {
		l.PushBoolean(s.Has(argString(l, 1)))
		return 1
	})
	return t
}
