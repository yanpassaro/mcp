package sqlize

import (
	"regexp"
	"sort"
	"strings"
)

const maskThreshold = 0.5

type match struct {
	start  int
	end    int
	entity string
	score  float64
}

type patternRule struct {
	entity string
	re     *regexp.Regexp
	score  float64
}

var reDate = regexp.MustCompile(`(\d{1,2})[/.\-](\d{1,2})[/.\-](\d{4})|(\d{4})[/.\-](\d{1,2})[/.\-](\d{1,2})`)

var patternRules = []patternRule{
	{entity: "CPF", re: regexp.MustCompile(`\b\d{3}\.?\d{3}\.?\d{3}-?\d{2}\b`), score: 0.9},
	{entity: "CNPJ", re: regexp.MustCompile(`\b\d{2}\.?\d{3}\.?\d{3}\/?\d{4}-?\d{2}\b`), score: 0.9},
	{entity: "EMAIL", re: regexp.MustCompile(`(?i)\b[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,}\b`), score: 1.0},
	{entity: "PHONE", re: regexp.MustCompile(`\b(?:\+?55[\s.\-]?)?\(?\d{2}\)?[\s.\-]?9?\d{4}[\s.\-]?\d{4}\b`), score: 0.8},
	{entity: "PHONE", re: regexp.MustCompile(`\(\d{2}\)[\s.\-]?9?\d{4}[\s.\-]?\d{4}\b`), score: 0.8},
	{entity: "CEP", re: regexp.MustCompile(`\b\d{5}-?\d{3}\b`), score: 0.8},
	{entity: "RG", re: regexp.MustCompile(`\b\d{1,2}\.?\d{3}\.?\d{3}-?[\dxX]\b`), score: 0.45},
	{entity: "CARD", re: regexp.MustCompile(`\b(?:\d{4}[ \-]?){3}\d{4}\b`), score: 0.9},
	{entity: "IP", re: regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`), score: 0.7},
	{entity: "MAC", re: regexp.MustCompile(`(?i)\b(?:[0-9a-f]{2}[:-]){5}[0-9a-f]{2}\b`), score: 1.0},
	{entity: "JWT", re: regexp.MustCompile(`\beyJ[A-Za-z0-9_\-]+\.eyJ[A-Za-z0-9_\-]+\.[A-Za-z0-9_\-]+\b`), score: 1.0},
	{entity: "HASH", re: regexp.MustCompile(`\b0x[a-fA-F0-9]{40}\b`), score: 1.0},
	{entity: "BTC", re: regexp.MustCompile(`\b(?:bc1|[13])[a-zA-HJ-NP-Z0-9]{25,39}\b`), score: 1.0},
	{entity: "URL", re: regexp.MustCompile(`(?i)\b[a-z][a-z0-9+.\-]*:\/\/[^\s"'<>]+`), score: 1.0},
	{entity: "URL", re: regexp.MustCompile(`(?i)\bwww\.[^\s"'<>]+`), score: 1.0},
	{entity: "URL", re: regexp.MustCompile(`\b(?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z]{2,}(?::\d{1,5})?(?:/[^\s"'<>]*)?`), score: 0.95},
}

func validCPF(doc string) bool {
	d := digitsOnly(doc)
	if len(d) != 11 {
		return false
	}
	allSame := true
	for i := 1; i < len(d); i++ {
		if d[i] != d[0] {
			allSame = false
			break
		}
	}
	if allSame {
		return false
	}
	sum := 0
	for i := 0; i < 9; i++ {
		sum += int(d[i]-'0') * (10 - i)
	}
	r := 11 - sum%11
	if r >= 10 {
		r = 0
	}
	if r != int(d[9]-'0') {
		return false
	}
	sum = 0
	for i := 0; i < 10; i++ {
		sum += int(d[i]-'0') * (11 - i)
	}
	r = 11 - sum%11
	if r >= 10 {
		r = 0
	}
	return r == int(d[10]-'0')
}

func validCNPJ(doc string) bool {
	d := digitsOnly(doc)
	if len(d) != 14 {
		return false
	}
	allSame := true
	for i := 1; i < len(d); i++ {
		if d[i] != d[0] {
			allSame = false
			break
		}
	}
	if allSame {
		return false
	}
	w1 := []int{5, 4, 3, 2, 9, 8, 7, 6, 5, 4, 3, 2}
	sum := 0
	for i := 0; i < 12; i++ {
		sum += int(d[i]-'0') * w1[i]
	}
	r := sum % 11
	if r < 2 {
		r = 0
	} else {
		r = 11 - r
	}
	if r != int(d[12]-'0') {
		return false
	}
	w2 := []int{6, 5, 4, 3, 2, 9, 8, 7, 6, 5, 4, 3, 2}
	sum = 0
	for i := 0; i < 13; i++ {
		sum += int(d[i]-'0') * w2[i]
	}
	r = sum % 11
	if r < 2 {
		r = 0
	} else {
		r = 11 - r
	}
	return r == int(d[13]-'0')
}

func luhn(s string) bool {
	d := digitsOnly(s)
	if len(d) < 12 {
		return false
	}
	sum, alt := 0, false
	for i := len(d) - 1; i >= 0; i-- {
		n := int(d[i] - '0')
		if alt {
			n *= 2
			if n > 9 {
				n -= 9
			}
		}
		sum += n
		alt = !alt
	}
	return sum%10 == 0
}

var columnEntityMap = buildColumnEntity()

func buildColumnEntity() map[string]string {
	m := map[string]string{}
	add := func(col, ent string) { m[normalizeWord(col)] = ent }

	for _, c := range []string{
		"nome", "nome completo", "nome da mae", "nome do pai", "nome completo do cliente",
		"nome responsavel", "pessoa", "pessoas", "cliente", "consumidor",
		"responsavel", "responsavel legal", "contato", "contato principal",
		"proprietario", "funcionario", "colaborador", "atendente", "vendedor",
		"representante", "diretor", "gerente", "medico", "paciente", "aluno",
		"professor", "candidato", "solicitante", "beneficiario",
		"customer", "client", "full name", "first name", "firstname", "last name",
		"lastname", "surname", "given name", "person", "person name", "name of contact",
		"mothers name", "fathers name", "contact name", "contact person",
		"responsible", "owner", "proprietor", "employee", "staff", "buyer", "seller",
		"supplier", "vendor", "merchant", "agent", "manager", "supervisor",
		"salesperson", "salesman", "consultant", "attorney", "lawyer", "applicant",
		"tenant", "guest", "member", "partner", "recipient", "author", "creator",
		"employer", "contractor", "assistant", "coordinator", "founder", "president",
		"secretary", "advisor", "analyst", "engineer", "developer", "delivery",
		"courier", "driver", "landlord", "spouse", "relative", "guardian",
		"emergency contact", "next of kin", "doctor", "patient", "student", "teacher",
	} {
		add(c, "PERSON")
	}
	for _, c := range []string{"cpf"} {
		add(c, "CPF")
	}
	for _, c := range []string{"cnpj"} {
		add(c, "CNPJ")
	}
	for _, c := range []string{
		"rg", "cnh", "renavam", "nis", "pis", "pasep", "titulo de eleitor",
		"passaporte", "inscricao estadual", "inscricao municipal", "identidade",
		"passport", "ssn", "identity", "tax id", "taxid", "tax payer id",
		"taxpayer id", "voter id", "national id", "id card", "id number",
		"drivers license", "driver license", "license number", "license plate",
		"plate", "doc number", "document number", "enrollment", "registration",
	} {
		add(c, "ID")
	}
	for _, c := range []string{
		"email", "e-mail", "email principal", "email contato", "email corporativo",
		"mail", "mail address", "email address", "contact email", "official email",
	} {
		add(c, "EMAIL")
	}
	for _, c := range []string{
		"telefone", "fone", "fone fixo", "telefone fixo", "telefone principal",
		"celular", "celular principal", "whatsapp",
		"phone", "telephone", "phone number", "contact number", "mobile",
		"mobile number", "cellphone", "cell", "work phone", "home phone",
		"landline", "whatsapp number", "fax",
	} {
		add(c, "PHONE")
	}
	for _, c := range []string{
		"endereco", "endereco completo", "logradouro", "rua", "avenida", "bairro",
		"distrito", "cidade", "estado", "uf", "pais", "cep", "codigo postal",
		"localizacao",
		"address", "street", "avenue", "neighborhood", "district", "city", "state",
		"country", "zip", "zipcode", "zip code", "postal code", "postcode",
		"street address", "postal address", "home address", "work address",
		"residence", "address line", "province", "region", "municipality",
		"county", "quarter", "zone", "ward", "location", "locality", "village",
	} {
		add(c, "ADDRESS")
	}
	for _, c := range []string{
		"banco", "agencia", "conta", "conta corrente", "numero da conta",
		"bank", "bank account", "agency", "branch", "account", "checking account",
		"account number", "routing number", "sort code", "iban", "swift", "bic", "wire",
	} {
		add(c, "BANK")
	}
	for _, c := range []string{
		"cvv", "validade", "cartao", "numero do cartao", "numero cartao",
		"expiry", "card", "card number", "cardholder", "cardholder name",
		"credit card", "creditcard", "debit card", "pan", "cvv2", "cvc",
		"security code", "verification code",
	} {
		add(c, "CARD")
	}
	for _, c := range []string{
		"data de nascimento", "data nascimento", "nascimento", "data de aniversario",
		"birthdate", "birth date", "birth day", "birthday", "dob", "date of birth", "born",
	} {
		add(c, "DATE")
	}
	for _, c := range []string{
		"senha", "chave", "token", "access token", "authorization", "auth",
		"api key", "api-key", "secret", "bearer",
		"password", "passwd", "pwd", "pass", "private key", "secret key",
		"client secret", "credential", "credentials", "session", "session id",
		"session_id", "cookie", "csrf", "otp", "2fa", "pin", "access key",
		"api token", "refresh token", "recovery code",
	} {
		add(c, "SECRET")
	}
	for _, c := range []string{
		"usuario", "login",
		"user", "username", "user id", "userid", "screen name", "handle", "account name",
	} {
		add(c, "USER")
	}
	return m
}

func columnEntity(name string) (string, bool) {
	ent, ok := columnEntityMap[normalizeWord(name)]
	return ent, ok
}

var reNameSpan = regexp.MustCompile(`\p{Lu}\p{Ll}*(?:(?:\s+(?:de|da|do|dos|das|e)\s+|\s+)\p{Lu}\p{Ll}*)+`)

var nameStopwords = map[string]bool{
	"de": true, "da": true, "do": true, "dos": true, "das": true, "e": true, "a": true, "o": true,
}

func findNameMatches(colName, cell string) []match {
	if cell == "" {
		return nil
	}
	var out []match

	for _, idx := range reNameSpan.FindAllStringIndex(cell, -1) {
		span := cell[idx[0]:idx[1]]
		if s := nameScore(span); s > 0 {
			out = append(out, match{start: idx[0], end: idx[1], entity: "PERSON", score: s})
		}
	}

	if len(out) == 0 {
		if m, ok := wholeCellName(colName, cell); ok {
			out = append(out, m)
		}
	}
	return out
}

func wholeCellName(_ string, cell string) (match, bool) {
	norm := normalizeWord(cell)
	tokens := strings.Fields(norm)
	if len(tokens) == 0 || len(tokens) > 3 {
		return match{}, false
	}
	hasPrep := false
	known := 0
	for _, t := range tokens {
		if nameStopwords[t] {
			hasPrep = true
			continue
		}
		if brFirstNames[t] {
			known++
		}
	}
	score := 0.0
	switch {
	case len(tokens) == 1 && known == 1:
		score = 0.85
	case hasPrep && known >= 1:
		score = 0.9
	}
	if score <= 0 {
		return match{}, false
	}
	return match{start: 0, end: len(cell), entity: "PERSON", score: score}, true
}

var geoDeny = map[string]bool{
	"acre": true, "alagoas": true, "amapa": true, "amazonas": true,
	"bahia": true, "ceara": true, "espirito santo": true, "goias": true,
	"maranhao": true, "mato grosso": true, "mato grosso do sul": true,
	"minas gerais": true, "para": true, "paraiba": true, "parana": true,
	"pernambuco": true, "piaui": true, "rio de janeiro": true,
	"rio grande do norte": true, "rio grande do sul": true, "rondonia": true,
	"roraima": true, "santa catarina": true, "sao paulo": true,
	"sergipe": true, "tocantins": true, "distrito federal": true,
	"aracaju": true, "belem": true, "belo horizonte": true, "boa vista": true,
	"brasilia": true, "campina grande": true, "campinas": true,
	"campo grande": true, "cuiaba": true, "curitiba": true,
	"florianopolis": true, "fortaleza": true, "goiania": true,
	"guarulhos": true, "joao pessoa": true, "macapa": true, "maceio": true,
	"manaus": true, "natal": true, "niteroi": true, "palmas": true,
	"paulo afonso": true, "porto alegre": true, "porto velho": true,
	"recife": true, "rio branco": true, "salvador": true, "santos": true,
	"sao bernardo do campo": true, "sao caetano do sul": true,
	"sao goncalo": true, "sao jose": true, "sao jose do rio preto": true,
	"sao jose dos campos": true, "sao luis": true, "sao vicente": true,
	"sorocaba": true, "teresina": true, "vitoria": true,
	"vitoria da conquista": true,
	"africa do sul":        true, "arabia saudita": true, "coreia do norte": true,
	"coreia do sul": true, "costa do marfim": true, "emirados arabes": true,
	"estados unidos": true, "nova zelandia": true, "reino unido": true,
}

func nameScore(span string) float64 {
	norm := normalizeWord(span)
	tokens := strings.Fields(norm)
	known := 0
	first := ""
	for _, t := range tokens {
		if nameStopwords[t] {
			continue
		}
		if first == "" {
			first = t
		}
		if brFirstNames[t] {
			known++
		}
	}
	if known == 0 {
		return 0
	}
	if !brFirstNames[first] {
		return 0
	}
	if geoDeny[norm] {
		return 0
	}
	if known >= 2 || len(tokens) >= 3 {
		return 0.9
	}
	return 0.85
}

func analyzeCell(colName, cell string) []match {
	var ms []match
	for _, r := range patternRules {
		for _, idx := range r.re.FindAllStringIndex(cell, -1) {
			text := cell[idx[0]:idx[1]]
			score := r.score
			switch r.entity {
			case "CPF":
				if validCPF(text) {
					score = 1.0
				} else {
					score = 0.15
				}
			case "CNPJ":
				if validCNPJ(text) {
					score = 1.0
				} else {
					score = 0.15
				}
			case "CARD":
				if luhn(text) {
					score = 1.0
				} else {
					score = 0.15
				}
			}
			if score < 1.0 {
				if ent, ok := columnEntity(colName); ok && ent == r.entity {
					score = 1.0
				}
			}
			ms = append(ms, match{start: idx[0], end: idx[1], entity: r.entity, score: score})
		}
	}
	ms = append(ms, findNameMatches(colName, cell)...)
	ms = append(ms, findAddressMatches(cell)...)
	return ms
}

var reAddressSpan = regexp.MustCompile(`\b(?i:rua|r\.|av\.|avenida|travessa|alameda|estrada|rodovia|praca|praça|p[çc]a\.|beco|largo|viela|condominio|condomínio|conjunto|residencial|loteamento|chacara|chácara|sitio|sítio|fazenda)\s+[A-Za-z0-9á-úÁ-Ú.\-']+(?:\s+[A-Za-z0-9á-úÁ-Ú.\-']+){0,4}(?:[,\s]+\d{1,6}(?:[-\s/]\d{1,5})?)?(?:\s*,\s*[A-ZÀ-Ú]\p{Ll}+(?:\s+[A-ZÀ-Ú]\p{Ll}+)*)?`)

var addressPreps = map[string]bool{
	"em": true, "no": true, "na": true, "nos": true, "nas": true,
	"e": true, "a": true, "o": true, "com": true, "para": true,
	"por": true, "sobre": true,
}

func findAddressMatches(cell string) []match {
	var out []match
	for _, idx := range reAddressSpan.FindAllStringIndex(cell, -1) {
		span := cell[idx[0]:idx[1]]
		if addrStartsWithPrep(span) {
			continue
		}
		out = append(out, match{start: idx[0], end: idx[1], entity: "ADDRESS", score: 0.85})
	}
	return out
}

func addrStartsWithPrep(span string) bool {
	words := strings.Fields(normalizeWord(span))
	return len(words) >= 2 && addressPreps[words[1]]
}

func resolveSpans(spans []match) []match {
	if len(spans) <= 1 {
		return spans
	}
	ss := append([]match(nil), spans...)
	sort.Slice(ss, func(i, j int) bool {
		if ss[i].start != ss[j].start {
			return ss[i].start < ss[j].start
		}
		if ss[i].score != ss[j].score {
			return ss[i].score > ss[j].score
		}
		return ss[i].end > ss[j].end
	})
	out := []match{}
	lastEnd := -1
	for _, m := range ss {
		if m.start < lastEnd {
			continue
		}
		out = append(out, m)
		lastEnd = m.end
	}
	return out
}

var entityOps = map[string]func(string) string{
	"PERSON":  maskByName,
	"EMAIL":   maskEmail,
	"CPF":     keepEdges,
	"CNPJ":    keepEdges,
	"ID":      keepEdges,
	"PHONE":   keepEdges,
	"CEP":     keepEdges,
	"RG":      keepEdges,
	"BANK":    keepEdges,
	"CARD":    maskCard,
	"DATE":    maskDate,
	"IP":      maskFull,
	"MAC":     maskFull,
	"JWT":     maskFull,
	"HASH":    maskFull,
	"BTC":     maskFull,
	"CREDURL": maskFull,
	"URL":     maskFull,
	"ADDRESS": maskByName,
	"SECRET":  maskFull,
}

func maskSpan(entity, text string) string {
	if op, ok := entityOps[entity]; ok {
		return op(text)
	}
	return maskByName(text)
}

func maskByColumn(cell, entity string) string {
	switch entity {
	case "SECRET":
		return maskFull(cell)
	case "CPF", "CNPJ", "ID", "PHONE", "CEP", "BANK", "CARD", "DATE":
		return keepEdges(cell)
	default:
		return maskByName(cell)
	}
}

func applyColumnMask(colName, cell string, spans []match) string {
	if cell == "" {
		return cell
	}
	var usable []match
	for _, m := range spans {
		if m.score >= maskThreshold {
			usable = append(usable, m)
		}
	}
	if len(usable) == 0 {
		if ent, ok := columnEntity(colName); ok {
			return maskByColumn(cell, ent)
		}
		return cell
	}
	usable = resolveSpans(usable)
	var b strings.Builder
	last := 0
	for _, m := range usable {
		b.WriteString(cell[last:m.start])
		b.WriteString(maskSpan(m.entity, cell[m.start:m.end]))
		last = m.end
	}
	b.WriteString(cell[last:])
	return b.String()
}

func RedactValue(s string) string {
	return applyColumnMask("", s, analyzeCell("", s))
}

func RedactRows(cols []string, rows [][]string) [][]string {
	out := make([][]string, len(rows))
	for i, row := range rows {
		out[i] = make([]string, len(row))
		for j, cell := range row {
			col := ""
			if j < len(cols) {
				col = cols[j]
			}
			out[i][j] = redactCell(col, cell)
		}
	}
	return out
}

func redactCell(colName, cell string) string {
	return applyColumnMask(colName, cell, analyzeCell(colName, cell))
}

func keepEdges(s string) string {
	d := digitsOnly(s)
	if len(d) < 4 {
		return s
	}
	stars := strings.Repeat("*", len(d)-4)
	if sep := lastNonDigitSep(s); sep != "" {
		return d[:2] + stars + sep + d[len(d)-2:]
	}
	return d[:2] + stars + d[len(d)-2:]
}

func lastNonDigitSep(s string) string {
	for i := len(s) - 1; i >= 0; i-- {
		c := s[i]
		if (c < '0' || c > '9') && c != ' ' {
			return string(c)
		}
	}
	return ""
}

func digitsOnly(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func maskEmail(s string) string {
	at := strings.Index(s, "@")
	if at <= 0 {
		return s
	}
	local, rest := s[:at], s[at+1:]
	switch {
	case len(local) == 0:
		local = "**"
	case len(local) == 1:
		local = local[:1] + "**"
	default:
		local = local[:2] + "**"
	}
	dm := "***"
	if dot := strings.LastIndex(rest, "."); dot > 0 && dot < len(rest)-1 {
		dm = "***" + rest[dot:]
	}
	return local + "@" + dm
}

func maskCard(s string) string {
	d := digitsOnly(s)
	if len(d) < 12 {
		return s
	}
	return d[:4] + strings.Repeat("*", len(d)-8) + d[len(d)-4:]
}

func maskDate(s string) string {
	return reDate.ReplaceAllStringFunc(s, func(m string) string {
		p := reDate.FindStringSubmatch(m)
		if p == nil {
			return m
		}
		if p[3] != "" {
			return "**/**/" + p[3]
		}
		if p[6] != "" {
			return p[4] + "/**/**"
		}
		return m
	})
}

func maskByName(s string) string {
	t := strings.TrimSpace(s)
	if t == "" {
		return s
	}
	r := []rune(t)
	if hasLetter(t) {
		return string(r[0]) + "***"
	}
	return "***"
}

func hasLetter(s string) bool {
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
			return true
		}
	}
	return false
}

func maskFull(string) string { return "***" }
