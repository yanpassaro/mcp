package mcpserver

import (
	"fmt"
	"strings"
)

type lunaFn struct {
	Name    string
	Args    string
	Returns string
	Desc    string
}

type lunaMod struct {
	Name    string
	Prefix  string
	Desc    string
	Fns     []lunaFn
	Example string
}

var lunaOrder = []*lunaMod{
	lunaResult, lunaLog, lunaArgs, lunaIO, lunaTmp, lunaFetch, lunaCookies,
	lunaSecrets, lunaSQL, lunaUUID, lunaCSV, lunaXML, lunaExcel, lunaData,
	lunaRegex, lunaJSON, lunaEncode, lunaStr, lunaList, lunaNum, lunaDate,
	lunaRandom, lunaAssert, lunaFake,
}

var lunaModules = map[string]*lunaMod{}

func init() {
	for _, m := range lunaOrder {
		lunaModules[m.Name] = m
	}
}

func lunaTopicNames() []string {
	names := make([]string, 0, len(lunaOrder)+5)
	for _, m := range lunaOrder {
		names = append(names, m.Name)
	}
	names = append(names, "meta", "limits", "env", "tools", "examples")
	return names
}

func renderLunaIndex() string {
	var b strings.Builder
	b.WriteString("# Sandbox Lua — std (lunadoc)\n\n")
	b.WriteString("> Isolated Lua sandbox: no OS/process; files confined to `mnt/`; network only via `std.fetch` (allowlist). Write `function main(std)` and return with `std.result.ok(...)`/`err(...)`. Objects/arrays become JSON in the output.\n\n")
	b.WriteString("## Modules\n\n| Module | Description |\n| --- | --- |\n")
	for _, m := range lunaOrder {
		fn := ""
		if n := len(m.Fns); n > 0 {
			fn = fmt.Sprintf(" · %d function(s)", n)
		}
		fmt.Fprintf(&b, "| `%s` | %s%s |\n", m.Name, m.Desc, fn)
	}
	b.WriteString("\nUse `sandbox_doc topic=<module>` to see signatures + return + example.\n")
	return b.String()
}

func renderLunaModule(m *lunaMod) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# std.%s\n\n", m.Name)
	b.WriteString("> ")
	b.WriteString(m.Desc)
	b.WriteString("\n\n")
	if len(m.Fns) > 0 {
		b.WriteString("### Signatures\n\n")
		for _, f := range m.Fns {
			args := prettyArgs(f.Args)
			if f.Args == "-" {
				args = ""
			}
			fn := f.Name
			if m.Prefix != "" {
				fn = m.Prefix + "." + fn
			}
			sig := fmt.Sprintf("`%s(%s)`", fn, args)
			switch f.Returns {
			case "", "-":
			case "panics":
				sig += " → ⚠️ *fails (panic)*"
			default:
				sig += " → `" + f.Returns + "`"
			}
			fmt.Fprintf(&b, "**%s**\n", sig)
			if f.Desc != "" {
				fmt.Fprintf(&b, "> %s\n", f.Desc)
			}
			b.WriteString("\n")
		}
	}
	if m.Example != "" {
		b.WriteString("### Example\n\n```lua\n")
		b.WriteString(m.Example)
		b.WriteString("\n```\n")
	}
	return b.String()
}

func prettyParam(s string) string {
	if before, after, ok := strings.Cut(s, ":"); ok {
		return before + ": " + after
	}
	return s
}

func prettyArgs(args string) string {
	if args == "" || args == "-" {
		return args
	}
	parts := strings.Split(args, ",")
	for i, p := range parts {
		parts[i] = prettyParam(strings.TrimSpace(p))
	}
	return strings.Join(parts, ", ")
}

func lunaModule(name string) *lunaMod {
	return lunaModules[name]
}

var lunaResult = &lunaMod{
	Name:   "result",
	Prefix: "result",
	Desc:   "sets the script result (success/error); this is the `main` return value",
	Fns: []lunaFn{
		{"ok", "data:any", "any", "set success — `data` becomes the return (objects/arrays → JSON)"},
		{"err", "msg:string", "panics", "set error — `msg` becomes the error message"},
	},
	Example: `local n = std.num.sum({ 1, 2, 3 })
std.result.ok({ total = n, ok = true })`,
}

var lunaLog = &lunaMod{
	Name:   "log",
	Prefix: "log",
	Desc:   "writes to the script's captured output (also `console.*` and `print()`)",
	Fns: []lunaFn{
		{"ok", "...", "nil", "logs a line (without breaking the result)"},
		{"info", "...", "nil", "same as `ok`"},
		{"warn", "...", "nil", "same as `ok`"},
		{"error", "...", "nil", "same as `ok`"},
		{"err", "...", "nil", "same as `ok`"},
		{"log", "...", "nil", "same as `ok`"},
	},
	Example: `std.log.info("processing", n)
print("extra")`,
}

var lunaArgs = &lunaMod{
	Name:   "args",
	Prefix: "args",
	Desc:   "The `args` value passed at run time. Valid JSON becomes an object/array; otherwise a string; `nil` when absent.",
	Example: `local nome = std.args and std.args.nome or "anon"
std.result.ok({ nome = nome })`,
}

var lunaIO = &lunaMod{
	Name:   "io",
	Prefix: "io",
	Desc:   "sandbox files (`mnt/`) — relative paths; rejects absolute paths and `..`",
	Fns: []lunaFn{
		{"read", "name:string", "string", "reads the whole file as text"},
		{"lines", "name:string", "table<string>", "reads non-empty lines"},
		{"json", "name:string", "any", "reads and parses the file as JSON"},
		{"write", "name:string, content:string", "int", "writes/overwrites; returns bytes"},
		{"append", "name:string, content:string", "int", "appends to the end; returns bytes"},
		{"del", "name:string", "true", "deletes a file (not a folder)"},
		{"exists", "name:string", "bool", "does the path exist?"},
		{"stat", "name:string", "table", "{ name, exists, isDir, size, lines }"},
		{"dir", "rel?:string", "table<string>", "lists folder entry names"},
		{"copy", "src:string, dst:string", "true", "copies within the sandbox"},
		{"move", "src:string, dst:string", "true", "renames/moves"},
		{"mkdir", "path:string", "true", "creates folder(s)"},
		{"glob", "pattern:string", "table<string>", "matching relative paths"},
		{"walk", "path?:string", "table", "{ name, isDir, size, lines, children }"},
	},
	Example: `if std.io.exists("data.txt") then
  std.io.append("data.txt", "\nfim")
end
local tree = std.io.walk("")
std.result.ok({ files = std.io.glob("*.lua"), root = tree.name })`,
}

var lunaTmp = &lunaMod{
	Name:   "tmp",
	Prefix: "tmp",
	Desc:   "temporary files (`tmp/`) — same functions as `std.io` + `clear`; cleaned each run",
	Fns: append([]lunaFn{
		{"clear", "-", "int", "empties the temp folder; returns how many items it removed"},
	}, lunaIO.Fns...),
	Example: `std.tmp.write("memo.txt", "temporario")
local n = std.tmp.clear()
std.result.ok({ cleared = n })`,
}

var lunaFetch = &lunaMod{
	Name:   "fetch",
	Prefix: "fetch",
	Desc:   "HTTP via allowlist — `res = { status, ok, headers, body, bytes, ms, ... }`",
	Fns: []lunaFn{
		{"request", "url:string, opts?:table", "table", "generic request (opts.method, body, headers, timeout, ...)"},
		{"get", "url:string, opts?:table", "table", "GET"},
		{"post", "url:string, body?:string, opts?:table", "table", "POST"},
		{"json", "url:string, opts?:table", "table", "GET and parses the body (res.data)"},
	},
	Example: `local res = std.fetch.get("http://localhost:8080/api", {
  headers = { Authorization = "Bearer " .. std.secrets.get("TOKEN") },
  timeout = 5000,
})
if res.ok then std.result.ok({ status = res.status, body = res.body }) end`,
}

var lunaCookies = &lunaMod{
	Name:   "cookies",
	Prefix: "fetch.cookies",
	Desc:   "`std.fetch` cookies, persisted in `SANDBOX_FETCH_COOKIE_FILE`",
	Fns: []lunaFn{
		{"list", "-", "table", "lists all cookies { domain, name, value, path, secure, httpOnly }"},
		{"clear", "domain?:string", "int", "removes cookies (without a domain, clears all); returns how many"},
		{"set", "domain:string, name:string, value:string, opts?:table", "bool", "sets a cookie (opts.path, opts.secure)"},
	},
}

var lunaSecrets = &lunaMod{
	Name:   "secrets",
	Prefix: "secrets",
	Desc:   "operator `SECRET_*` variables (read-only; values are redacted in output)",
	Fns: []lunaFn{
		{"get", "key:string", "string|nil", "reads a secret (case-insensitive; without the SECRET_ prefix)"},
		{"has", "key:string", "bool", "does the secret exist?"},
	},
	Example: `if std.secrets.has("github_token_api") then
  local t = std.secrets.get("github_token_api")
  std.result.ok({ ok = true })  -- never return the token
else
  std.result.err("secret missing")
end`,
}

var lunaSQL = &lunaMod{
	Name:   "sql",
	Prefix: "sql",
	Desc:   "SQLite databases in the sandbox — the same path reuses its connection during the run; `tmp:` for temp; `:memory:` for in-memory",
	Fns: []lunaFn{
		{"exec", "path:string, sql:string, params?:table", "table", "{ lastId, rows } — executes and returns affected rows"},
		{"query", "path:string, sql:string, params?:table", "table<row>", "rows as maps (max SANDBOX_SQL_MAX_ROWS)"},
		{"get", "path:string, sql:string, params?:table", "row|nil", "first row or nil"},
		{"scalar", "path:string, sql:string, params?:table", "any", "first column of the first row"},
		{"close", "path:string", "bool", "closes the connection before the run ends"},
		{"begin", "path:string", "true", "begins a transaction"},
		{"commit", "path:string", "true", "commits"},
		{"rollback", "path:string", "true", "rolls back"},
		{"tables", "path:string", "table<string>", "table names"},
		{"columns", "path:string, table:string", "table", "{ name, type, notnull, pk, dflt }"},
		{"schema", "path:string", "table", "{ tables, columns = { [table] = {...} } }"},
	},
	Example: `std.sql.exec("app.db", "CREATE TABLE IF NOT EXISTS t (id INTEGER PRIMARY KEY, name TEXT)")
std.sql.exec("app.db", "INSERT INTO t (name) VALUES (?)", { "Ava" })
local r = std.sql.query("app.db", "SELECT * FROM t")
std.result.ok({ total = #r, first = r[1] and r[1].name })`,
}

var lunaUUID = &lunaMod{
	Name:   "uuid",
	Prefix: "uuid",
	Desc:   "UUID generation",
	Fns: []lunaFn{
		{"v4", "-", "string", "random UUID (RFC 4122)"},
		{"v7", "-", "string", "time-ordered UUID (RFC 9562)"},
	},
	Example: `std.result.ok({ id = std.uuid.v4(), ordered = std.uuid.v7() })`,
}

var lunaCSV = &lunaMod{
	Name:   "csv",
	Prefix: "csv",
	Desc:   "CSV parse/serialize",
	Fns: []lunaFn{
		{"parse", "s:string, sep?:string", "table<row>", "rows of cells (sep defaults `,`)"},
		{"stringify", "rows:table, sep?:string", "string", "serializes to CSV"},
	},
	Example: `local rows = std.csv.parse("a,b\n1,2")
local csv = std.csv.stringify(rows)
std.result.ok({ rows = rows, csv = csv })`,
}

var lunaXML = &lunaMod{
	Name:   "xml",
	Prefix: "xml",
	Desc:   "XML parse/serialize (nodes = { name, attrs, text, children })",
	Fns: []lunaFn{
		{"parse", "s:string", "node", "converts the document into the root node"},
		{"stringify", "node:table", "string", "serializes a node"},
	},
	Example: `local doc = std.xml.parse('<root><item id="1">Ava</item></root>')
std.result.ok({ name = doc.name, first = doc.children[1].text })`,
}

var lunaExcel = &lunaMod{
	Name:   "excel",
	Prefix: "excel",
	Desc:   ".xlsx spreadsheets — paths relative to `mnt/` or `tmp:`",
	Fns: []lunaFn{
		{"sheets", "path:string", "table<string>", "sheet names"},
		{"read", "path:string, sheet?:string", "rows", "rows (array of arrays); uses the first sheet if omitted"},
		{"write", "path:string, rows:table, sheet?:string", "true", "writes the spreadsheet"},
	},
	Example: `local sheets = std.excel.sheets("dados.xlsx")
local rows = std.excel.read("dados.xlsx", sheets[1])
std.result.ok({ total = #rows, first = rows[1] })`,
}

var lunaData = &lunaMod{
	Name:   "data",
	Prefix: "data",
	Desc:   "data pipeline (sqlize-style): moves between CSV/JSON/XML/Excel/SQLite (rows = array of maps)",
	Fns: []lunaFn{
		{"fromCSV", "s:string, sep?:string", "rows", "CSV → rows"},
		{"toCSV", "rows:table, sep?:string", "string", "rows → CSV"},
		{"fromJSON", "s:string", "any", "JSON → value"},
		{"toJSON", "v:any", "string", "value → JSON"},
		{"toXML", "rows:table, root?:string, row?:string", "string", "rows → XML"},
		{"fromExcel", "path:string, sheet?:string", "rows", "spreadsheet → rows"},
		{"toExcel", "rows:table, path:string, sheet?:string", "true", "rows → spreadsheet"},
		{"sqlImport", "db:string, table:string, rows:table, opts?:table", "table", "{ imported } — optional opts.create=true"},
		{"sqlExport", "db:string, query:string", "rows", "query → rows"},
		{"convert", "src:string, dst:string", "true", "converts by extension"},
	},
	Example: `local rows = std.data.fromCSV("nome,idade\nAva,30")
std.data.sqlImport("app.db", "pessoas", rows, { create = true })
std.result.ok({ total = #std.data.sqlExport("app.db", "SELECT * FROM pessoas") })`,
}

var lunaRegex = &lunaMod{
	Name:   "regex",
	Prefix: "regex",
	Desc:   "regular expressions — Go RE2 syntax (differs from Lua patterns)",
	Fns: []lunaFn{
		{"match", "s:string, pattern:string", "bool", "does it match?"},
		{"find", "s:string, pattern:string", "string|nil", "first match"},
		{"findAll", "s:string, pattern:string, limit?:number", "table<string>", "all matches"},
		{"replace", "s:string, pattern:string, repl:string", "string", "replaces (refs `$1`)"},
		{"split", "s:string, pattern:string, limit?:number", "table<string>", "splits"},
		{"groups", "s:string, pattern:string", "table|nil", "{ match, groups = {...} }"},
		{"findAllGroups", "s:string, pattern:string, limit?:number", "table", "groups of all matches"},
	},
	Example: `local xs = std.regex.findAll("a1 b22", "[0-9]+")
std.result.ok({ xs = xs, s = std.regex.replace("join 2024", "[0-9]+", "2025") })`,
}

var lunaJSON = &lunaMod{
	Name:   "json",
	Prefix: "json",
	Desc:   "JSON: parse, stringify and utilities",
	Fns: []lunaFn{
		{"parse", "s:string", "any", "JSON → value"},
		{"stringify", "v:any, indent?:number", "string", "value → JSON"},
		{"format", "v:any", "string", "indented JSON"},
		{"minify", "s:string", "string", "JSON without spaces"},
		{"path", "v:any, path:string", "any", "accesses a dot path, e.g. `a.1`"},
	},
	Example: `local v = std.json.parse('{"a": [1,2]}')
std.result.ok({ x = std.json.path(v, "a.1"), s = std.json.stringify({ a = 1 }, 2) })`,
}

var lunaEncode = &lunaMod{
	Name:   "encode",
	Prefix: "encode",
	Desc:   "hashes and encodings",
	Fns: []lunaFn{
		{"crc32", "s:string", "hex", "CRC-32"},
		{"md5", "s:string", "hex", "MD5"},
		{"sha256", "s:string", "hex", "SHA-256"},
		{"base64", "s:string, mode?:string", "string", "encode/decode/url-safe"},
		{"hex", "s:string, mode?:string", "string", "encode/decode"},
	},
	Example: `std.result.ok({
  h = std.encode.sha256("abc"),
  b = std.encode.base64("abc"),
})`,
}

var lunaStr = &lunaMod{
	Name:   "str",
	Prefix: "str",
	Desc:   "strings: normalization, casing, formatting",
	Fns: []lunaFn{
		{"normalize", "s:string", "string", "removes accents"},
		{"slug", "s:string", "string", "url-safe slug"},
		{"title", "s:string", "string", "Title Case"},
		{"camel", "s:string", "string", "camelCase"},
		{"pascal", "s:string", "string", "PascalCase"},
		{"snake", "s:string", "string", "snake_case"},
		{"kebab", "s:string", "string", "kebab-case"},
		{"wrap", "s:string, width?:number", "table<string>", "wraps into lines"},
		{"summarize", "s:string, max:number", "string", "summarizes with `...`"},
		{"format", "fmt:string, ...", "string", "printf-style"},
		{"count", "s:string, sub:string", "number", "occurrences"},
		{"split", "s:string, sep:string, limit?:number", "table<string>", "splits"},
	},
	Example: `std.result.ok({
  slug = std.str.slug("Ola Mundo"),
  fmt = std.str.format("Hi %s / %d", "Ava", 42),
})`,
}

var lunaList = &lunaMod{
	Name:   "list",
	Prefix: "list",
	Desc:   "array helpers (functional ones take a `fn` callback)",
	Fns: []lunaFn{
		{"chunk", "arr:table, n:number", "table", "chunks into groups of `n`"},
		{"groupBy", "arr:table, prop:string", "table", "{ [value] = items }"},
		{"unique", "arr:table", "table", "removes duplicates"},
		{"flatten", "arr:table", "table", "flattens nested arrays"},
		{"sortBy", "arr:table, prop:string", "table", "sorted copy"},
		{"countBy", "arr:table, prop:string", "table", "{ [value] = count }"},
		{"first", "arr:table, n?:number", "table", "first `n`"},
		{"last", "arr:table, n?:number", "table", "last `n`"},
		{"map", "arr:table, fn:function", "table", "maps each item"},
		{"filter", "arr:table, fn:function", "table", "keeps truthy items"},
		{"reduce", "arr:table, fn:function, init?:any", "any", "accumulates with `fn(acc, x)`"},
		{"find", "arr:table, fn:function", "any|nil", "first matching item"},
		{"some", "arr:table, fn:function", "bool", "does any match?"},
		{"every", "arr:table, fn:function", "bool", "do all match?"},
	},
	Example: `local dbl = std.list.map({ 1, 2, 3 }, function(x) return x * 2 end)
std.result.ok({ dbl = dbl, all = std.list.every(dbl, function(x) return x > 0 end) })`,
}

var lunaNum = &lunaMod{
	Name:   "num",
	Prefix: "num",
	Desc:   "numbers: rounding, aggregation, parse, formatting",
	Fns: []lunaFn{
		{"round", "v:number, digits?:number", "number", "rounds to `digits`"},
		{"clamp", "v:number, lo:number, hi:number", "number", "clamps to the range"},
		{"percent", "a:number, b:number", "number", "a/b*100"},
		{"sum", "arr:table", "number", "sums"},
		{"avg", "arr:table", "number", "average"},
		{"parse", "s:string", "number", "parses; NaN if invalid"},
		{"fmt", "n:number, dec?:number, loc?:string", "string", "formats (pt-BR → 1.234,56)"},
	},
	Example: `std.result.ok({
  t = std.num.sum({ 1, 2, 3 }),
  p = std.num.percent(2, 4),
  f = std.num.fmt(1234.5, 2, "pt-BR"),
})`,
}

var lunaDate = &lunaMod{
	Name:   "date",
	Prefix: "date",
	Desc:   "dates in ms — `unit`: day, hour, minute, second, week, month, year",
	Fns: []lunaFn{
		{"now", "-", "number", "ms now"},
		{"iso", "ts?:number", "string", "RFC3339"},
		{"format", "layout:string, ts?:number", "string", "YYYY,MM,DD,HH,mm,ss"},
		{"parse", "s:string", "number", "string → ms"},
		{"add", "ts:number, amount:number, unit:string", "number", "adds `amount` of `unit`"},
		{"unix", "ts?:number", "number", "seconds"},
		{"diff", "a:number, b:number, unit:string", "number", "difference in `unit`"},
	},
	Example: `local now = std.date.now()
local later = std.date.add(now, 2, "hour")
std.result.ok({ iso = std.date.iso(later) })`,
}

var lunaRandom = &lunaMod{
	Name:   "random",
	Prefix: "random",
	Desc:   "random numbers/picks (seeded)",
	Fns: []lunaFn{
		{"seed", "n:number", "nil", "sets the seed (deterministic)"},
		{"int", "min:number, max:number", "number", "inclusive integer"},
		{"pick", "arr:table", "any", "random element"},
		{"shuffle", "arr:table", "table", "shuffled copy"},
	},
	Example: `std.random.seed(123)
std.result.ok({ n = std.random.int(1, 6), s = std.random.shuffle({ "a", "b", "c" }) })`,
}

var lunaAssert = &lunaMod{
	Name:   "assert",
	Prefix: "assert",
	Desc:   "checks that fail (panic) — useful for tests/validation",
	Fns: []lunaFn{
		{"ok", "v:any, msg?:string", "nil", "fails if `v` is falsy/nil"},
		{"equal", "a:any, b:any", "nil", "fails if not deep-equal"},
		{"throws", "fn:function", "nil", "fails if `fn` does not raise"},
		{"type", "v:any, kind:string", "nil", "fails if the type is not the given one"},
		{"notNil", "v:any, msg?:string", "nil", "fails if `v` is nil"},
		{"number", "v:any, msg?:string", "nil", "fails if not a number"},
		{"string", "v:any, msg?:string", "nil", "fails if not a string"},
		{"boolean", "v:any, msg?:string", "nil", "fails if not a boolean"},
		{"table", "v:any, msg?:string", "nil", "fails if not a table"},
		{"contains", "s:string, sub:string", "nil", "fails if `s` does not contain `sub`"},
		{"matches", "s:string, pattern:string", "nil", "fails if `s` does not match (Go regex)"},
		{"between", "v:number, lo:number, hi:number", "nil", "fails if outside [lo, hi]"},
		{"length", "v:any, n:number", "nil", "fails if length != n"},
	},
	Example: `std.assert.type({}, "table")
std.assert.matches("abc123", "[0-9]+")
std.result.ok({ ok = true })`,
}

var lunaFake = &lunaMod{
	Name:   "fake",
	Prefix: "fake",
	Desc:   "fake data (Faker-style) with deterministic seed",
	Fns: []lunaFn{
		{"seed", "n:number", "nil", "sets the seed"},
		{"name", "-", "string", "full name"},
		{"firstName", "-", "string", "first name"},
		{"lastName", "-", "string", "last name"},
		{"email", "-", "string", "email"},
		{"username", "-", "string", "username"},
		{"phone", "-", "string", "phone"},
		{"int", "min:number, max:number", "number", "integer"},
		{"float", "min:number, max:number, dec?:number", "number", "decimal"},
		{"bool", "-", "bool", "boolean"},
		{"uuid", "-", "string", "UUID v4"},
		{"date", "from?:string, to?:string", "string", "RFC3339 date"},
		{"words", "n?:number", "string", "words"},
		{"sentence", "n?:number", "string", "sentence"},
		{"paragraph", "n?:number", "string", "paragraph"},
	},
	Example: `std.fake.seed(42)
std.result.ok({ nome = std.fake.name(), email = std.fake.email(), n = std.fake.int(1, 100) })`,
}
