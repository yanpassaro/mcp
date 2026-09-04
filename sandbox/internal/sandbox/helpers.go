package sandbox

import (
	"bytes"
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
		return nil
	}
	var v any
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
		if pretty, ok := prettyJSON(exp); ok {
			return pretty, true
		}
		return exp, false
	case map[string]any, []any:
		if b, err := json.MarshalIndent(exp, "", "  "); err == nil {
			return string(b), true
		}
		return fmt.Sprint(exp), false
	default:
		return fmt.Sprint(exp), false
	}
}

func prettyJSON(s string) (string, bool) {
	s = strings.TrimSpace(s)
	if s == "" || (s[0] != '{' && s[0] != '[') {
		return "", false
	}
	var buf bytes.Buffer
	if err := json.Indent(&buf, []byte(s), "", "  "); err != nil {
		return "", false
	}
	return buf.String(), true
}
