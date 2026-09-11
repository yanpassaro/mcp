package sandbox

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	lua "github.com/Shopify/go-lua"
)

var (
	reEmail = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)
	reURL   = regexp.MustCompile(`^(?:https?|ftps?)://\S+$`)
)

func buildSchema(L *lua.State) int {
	t := newTable(L)

	setGoFunc(L, t, "validate", func(l *lua.State) int {
		val := luaToAny(l, 1)
		spec := luaArrayAny(l, 2)
		var rows []any
		if arr, ok := val.([]any); ok {
			rows = arr
		} else if m, ok := val.(map[string]any); ok {
			rows = []any{m}
		}
		pushAny(l, schemaValidateRows(rows, spec))
		return 1
	})

	setGoFunc(L, t, "validate_one", func(l *lua.State) int {
		val := luaToAny(l, 1)
		spec := luaArrayAny(l, 2)
		rows := []any{val}
		if m, ok := val.(map[string]any); ok {
			rows = []any{m}
		}
		pushAny(l, schemaValidateRows(rows, spec))
		return 1
	})

	setGoFunc(L, t, "coerce", func(l *lua.State) int {
		pushAny(l, coerceValue(luaToAny(l, 1), argString(l, 2)))
		return 1
	})

	return t
}

func schemaValidateRows(rows []any, spec []any) map[string]any {
	errs := []any{}
	for idx, r := range rows {
		m, ok := r.(map[string]any)
		if !ok {
			errs = append(errs, map[string]any{"row": float64(idx + 1), "field": "*", "msg": "linha não é um objeto"})
			continue
		}
		for _, s := range spec {
			rule, ok := s.(map[string]any)
			if !ok {
				continue
			}
			field := optString(rule, "field")
			if field == "" {
				field = "*"
			}
			for _, msg := range checkRule(rule, m[field], field) {
				errs = append(errs, map[string]any{"row": float64(idx + 1), "field": field, "msg": msg})
			}
		}
	}
	return map[string]any{"ok": len(errs) == 0, "errors": errs}
}

func checkRule(rule map[string]any, value any, fieldPath string) []string {
	nullable := asBool(rule["nullable"], false)
	required := asBool(rule["required"], false)
	if value == nil {
		if required && !nullable {
			return []string{fmt.Sprintf("%s é obrigatório", fieldPath)}
		}
		return nil
	}

	var msgs []string
	typ := optString(rule, "type")
	if typ != "" && !typeOK(value, typ) {
		return []string{fmt.Sprintf("%s deve ser %s", fieldPath, typ)}
	}

	switch typ {
	case "string":
		s := fmt.Sprint(value)
		if n, ok := numOpt(rule["min_len"]); ok && float64(len(s)) < n {
			msgs = append(msgs, fmt.Sprintf("%s deve ter tamanho >= %d", fieldPath, int(n)))
		}
		if n, ok := numOpt(rule["max_len"]); ok && float64(len(s)) > n {
			msgs = append(msgs, fmt.Sprintf("%s deve ter tamanho <= %d", fieldPath, int(n)))
		}
		if pat := optString(rule, "pattern"); pat != "" {
			if re, err := regexp.Compile(pat); err == nil && !re.MatchString(s) {
				msgs = append(msgs, fmt.Sprintf("%s não corresponde ao padrão", fieldPath))
			}
		}
	case "email":
		s := fmt.Sprint(value)
		if !reEmail.MatchString(s) {
			msgs = append(msgs, fmt.Sprintf("%s é um e-mail inválido", fieldPath))
		}
	case "url":
		if !reURL.MatchString(fmt.Sprint(value)) {
			msgs = append(msgs, fmt.Sprintf("%s é uma URL inválida", fieldPath))
		}
	case "date":
		if !isDateStr(fmt.Sprint(value)) {
			msgs = append(msgs, fmt.Sprintf("%s é uma data inválida", fieldPath))
		}
	case "number", "integer":
		n, _ := numOpt(value)
		if min, ok := numOpt(rule["min"]); ok && n < min {
			msgs = append(msgs, fmt.Sprintf("%s deve ser >= %g", fieldPath, min))
		}
		if max, ok := numOpt(rule["max"]); ok && n > max {
			msgs = append(msgs, fmt.Sprintf("%s deve ser <= %g", fieldPath, max))
		}
	case "array":
		if items, ok := rule["items"].(map[string]any); ok {
			for i, el := range value.([]any) {
				for _, msg := range checkRule(items, el, fmt.Sprintf("%s[%d]", fieldPath, i+1)) {
					msgs = append(msgs, msg)
				}
			}
		}
	case "object":
		if fields, ok := rule["fields"].([]any); ok {
			m := value.(map[string]any)
			for _, s := range fields {
				if sub, ok := s.(map[string]any); ok {
					subField := optString(sub, "field")
					if subField == "" {
						continue
					}
					for _, msg := range checkRule(sub, m[subField], fieldPath+"."+subField) {
						msgs = append(msgs, msg)
					}
				}
			}
		}
	}

	if one, ok := rule["one_of"].([]any); ok {
		sv := fmt.Sprint(value)
		found := false
		for _, o := range one {
			if fmt.Sprint(o) == sv {
				found = true
				break
			}
		}
		if !found {
			msgs = append(msgs, fmt.Sprintf("%s deve ser um dos valores permitidos", fieldPath))
		}
	}

	return msgs
}

func typeOK(value any, typ string) bool {
	switch typ {
	case "string", "email", "url", "date":
		_, ok := value.(string)
		return ok
	case "number":
		_, ok := numOpt(value)
		return ok
	case "integer":
		n, ok := numOpt(value)
		return ok && n == math.Trunc(n)
	case "boolean":
		_, ok := value.(bool)
		return ok
	case "array":
		_, ok := value.([]any)
		return ok
	case "object":
		_, ok := value.(map[string]any)
		return ok
	}
	return true
}

func isDateStr(s string) bool {
	for _, layout := range []string{
		time.RFC3339, "2006-01-02", "2006-01-02 15:04:05", "2006/01/02", "02/01/2006",
	} {
		if _, err := time.Parse(layout, strings.TrimSpace(s)); err == nil {
			return true
		}
	}
	return false
}

func coerceValue(v any, typ string) any {
	s := strings.TrimSpace(fmt.Sprint(v))
	switch strings.ToLower(strings.TrimSpace(typ)) {
	case "string":
		return s
	case "number", "integer", "int", "float":
		if n, err := strconv.ParseFloat(s, 64); err == nil {
			return n
		}
	case "boolean", "bool":
		switch strings.ToLower(s) {
		case "true", "1", "yes", "on", "y":
			return true
		case "false", "0", "no", "off", "n", "":
			return false
		}
		return v
	}
	return v
}
