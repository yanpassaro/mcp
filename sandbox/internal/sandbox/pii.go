package sandbox

import (
	"regexp"
	"strings"

	lua "github.com/Shopify/go-lua"
)

var piiPatterns = []struct {
	label string
	re    *regexp.Regexp
}{
	{"EMAIL", regexp.MustCompile(`[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`)},
	{"URL", regexp.MustCompile(`https?://[^\s"'<>]+`)},
	{"IP", regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`)},
	{"MAC", regexp.MustCompile(`\b[0-9a-fA-F]{2}(?::[0-9a-fA-F]{2}){5}\b`)},
	{"JWT", regexp.MustCompile(`eyJ[A-Za-z0-9_\-]+\.[A-Za-z0-9_\-]+\.[A-Za-z0-9_\-]+`)},
	{"BTC", regexp.MustCompile(`\b(?:1|3|bc1)[A-Za-z0-9]{25,39}\b`)},
	{"CNPJ", regexp.MustCompile(`\b\d{2}\.\d{3}\.\d{3}/\d{4}-\d{2}\b|\b\d{14}\b`)},
	{"CPF", regexp.MustCompile(`\b\d{3}\.\d{3}\.\d{3}-\d{2}\b|\b\d{11}\b`)},
	{"CEP", regexp.MustCompile(`\b\d{5}-\d{3}\b|\b\d{8}\b`)},
	{"PHONE", regexp.MustCompile(`\+?\d{1,3}\s?\(?\d{2,3}\)?\s?\d{4,5}-?\d{4}`)},
	{"CARD", regexp.MustCompile(`\b(?:\d{4}[ -]?){3}\d{2,4}\b`)},
	{"DATE", regexp.MustCompile(`\b\d{4}-\d{2}-\d{2}\b|\b\d{2}/\d{2}/\d{4}\b`)},
}

var piiColumnKeywords = map[string][]string{
	"CPF":    {"cpf", "documento", "doc", "inscricao", "document"},
	"CNPJ":   {"cnpj", "inscricao", "juridica", "company"},
	"ID":     {"rg", "identidade", "passaporte", "cnh", "passport", "ssn", "id number", "document number", "identity", "license", "taxid", "voter id"},
	"EMAIL":  {"email", "e-mail", "mail", "correio", "contact"},
	"PHONE":  {"telefone", "fone", "celular", "whatsapp", "phone", "mobile", "tel"},
	"CEP":    {"cep", "codigo postal", "postal", "zip"},
	"ADDRESS": {"endereco", "rua", "logradouro", "address", "street", "local", "bairro", "cidade", "estado", "uf"},
	"BANK":   {"banco", "agencia", "conta", "bank", "account", "iban", "swift"},
	"CARD":   {"cartao", "cvv", "validade", "card", "credit", "expiry", "pan"},
	"DATE":   {"nascimento", "nascido", "aniversario", "birth", "born", "dob", "validade"},
	"SECRET": {"senha", "chave", "token", "secret", "password", "api key", "credential", "session", "bearer", "auth"},
	"USER":   {"usuario", "login", "user", "username", "account"},
	"URL":    {"url", "website", "site"},
	"IP":     {"ip"},
	"MAC":    {"mac"},
	"JWT":    {"jwt"},
	"BTC":    {"btc", "bitcoin", "wallet"},
	"HASH":   {"hash", "sha", "md5", "crc"},
}

var piiSensitiveMask = func() []struct {
	label string
	re    *regexp.Regexp
} {
	var out []struct {
		label string
		re    *regexp.Regexp
	}
	for _, p := range piiPatterns {
		switch p.label {
		case "EMAIL", "CNPJ", "CPF", "CARD", "PHONE", "JWT", "BTC":
			out = append(out, p)
		}
	}
	return out
}()

func piiMaskText(s string) string {
	for _, p := range piiSensitiveMask {
		s = p.re.ReplaceAllString(s, "["+p.label+"]")
	}
	return s
}

func buildPII(L *lua.State) int {
	t := newTable(L)

	setGoFunc(L, t, "has", func(l *lua.State) int {
		l.PushBoolean(piiHas(argString(l, 1)))
		return 1
	})

	setGoFunc(L, t, "detect", func(l *lua.State) int {
		pushAny(l, piiDetect(argString(l, 1)))
		return 1
	})

	setGoFunc(L, t, "mask", func(l *lua.State) int {
		l.PushString(piiMask(argString(l, 1)))
		return 1
	})

	setGoFunc(L, t, "maskRows", func(l *lua.State) int {
		pushAny(l, piiMaskRows(luaArrayAny(l, 1)))
		return 1
	})

	return t
}

func piiHas(s string) bool {
	for _, p := range piiPatterns {
		if p.re.MatchString(s) {
			return true
		}
	}
	return false
}

func piiDetect(s string) []any {
	out := []any{}
	for _, p := range piiPatterns {
		for _, m := range p.re.FindAllString(s, -1) {
			out = append(out, map[string]any{"type": p.label, "value": m})
		}
	}
	return out
}

func piiMask(s string) string {
	for _, p := range piiPatterns {
		s = p.re.ReplaceAllString(s, "["+p.label+"]")
	}
	return s
}

func piiMaskRows(rows []any) []any {
	piiCols := map[string]string{}
	for _, r := range rows {
		if m, ok := r.(map[string]any); ok {
			for k := range m {
				if _, done := piiCols[k]; done {
					continue
				}
				if c := piiColumnType(k); c != "" {
					piiCols[k] = c
				}
			}
		}
	}
	out := make([]any, len(rows))
	for i, r := range rows {
		m, _ := r.(map[string]any)
		nm := map[string]any{}
		for k, v := range m {
			if t, ok := piiCols[k]; ok {
				nm[k] = maskPIICell(v, t)
			} else {
				nm[k] = v
			}
		}
		out[i] = nm
	}
	return out
}

func piiColumnType(name string) string {
	n := strings.ToLower(name)
	for ct, keys := range piiColumnKeywords {
		for _, k := range keys {
			if strings.Contains(n, k) {
				return ct
			}
		}
	}
	return ""
}

func maskPIICell(v any, typ string) any {
	s, ok := v.(string)
	if !ok {
		return v
	}
	if strings.TrimSpace(s) == "" {
		return v
	}
	if typ == "SECRET" {
		return "[REDACTED]"
	}
	return "[" + typ + "]"
}
