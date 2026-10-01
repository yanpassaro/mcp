package sandbox

import (
	"encoding/json"
	"fmt"
	"strings"
)

func numOpt(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int64:
		return float64(n), true
	case int:
		return float64(n), true
	}
	return 0, false
}

func parseArgs(argStr string) any {
	s := strings.TrimSpace(argStr)
	if s == "" {
		return map[string]any{}
	}
	v := any(nil)
	if err := json.Unmarshal([]byte(s), &v); err == nil {
		return v
	}
	return s
}

func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func renderData(v any) (string, bool) {
	if v == nil {
		return "", false
	}
	switch exp := v.(type) {
	case string:
		parsed := any(nil)
		if err := json.Unmarshal([]byte(strings.TrimSpace(exp)), &parsed); err == nil {
			b, merr := json.MarshalIndent(parseNestedJSON(parsed), "", "  ")
			if merr == nil {
				return string(b), true
			}
		}
		return exp, false
	case map[string]any, []any:
		if b, err := json.MarshalIndent(parseNestedJSON(v), "", "  "); err == nil {
			return string(b), true
		}
		return fmt.Sprint(v), false
	default:
		return fmt.Sprint(v), false
	}
}


