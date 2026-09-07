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

type Secrets struct {
	byKey map[string]string
	vals  []string
}

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
	sort.Slice(s.vals, func(i, j int) bool { return len(s.vals[i]) > len(s.vals[j]) })
	return s
}

func (s *Secrets) Get(key string) (string, bool) {
	if s == nil {
		return "", false
	}
	v, ok := s.byKey[strings.ToLower(strings.TrimSpace(key))]
	return v, ok
}

func (s *Secrets) Has(key string) bool {
	if s == nil {
		return false
	}
	_, ok := s.byKey[strings.ToLower(strings.TrimSpace(key))]
	return ok
}

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
