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
	Body    string
}

var lunaOrder = []*lunaMod{
	lunaResult, lunaLog, lunaArgs, lunaIO, lunaTmp, lunaFetch, lunaCookies,
	lunaSecrets, lunaSQL, lunaUUID, lunaCSV, lunaXML, lunaExcel, lunaData,
	lunaRegex, lunaJSON, lunaEncode, lunaStr, lunaList, lunaNum, lunaDate,
	lunaRandom, lunaAssert, lunaTemplate, lunaHuman, lunaPath, lunaStats,
	lunaSchema, lunaDiff, lunaInfer, lunaMissing, lunaText, lunaPipe,
	lunaSeq, lunaDuration, lunaUnit,
}

var lunaNative = []string{
	"print", "pairs", "ipairs", "type", "tostring", "tonumber",
	"setmetatable", "getmetatable", "string", "math", "table",
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
	names = append(names, "meta", "limits", "env", "tools", "run", "scripts", "examples")
	return names
}

func renderLunaIndex() string {
	var b strings.Builder
	b.WriteString("# Sandbox Lua — std (lunadoc)\n\n")
	b.WriteString("> Isolated Lua sandbox: no OS/process; files in `mnt/`; network only via `std.fetch` (allowlist). Each script is `function main(std)` ending in `std.result.ok(...)`/`err(...)`. Inline `code` can be just the body — the wrapper is added for you.\n\n")
	var names []string
	for _, m := range lunaOrder {
		names = append(names, "`"+m.Name+"`")
	}
	fmt.Fprintf(&b, "**Modules:** %s\n\n", strings.Join(names, ", "))
	fmt.Fprintf(&b, "**Native (Lua):** %s\n\n", strings.Join(lunaNative, ", "))
	b.WriteString("## How to run\n\n```\nsandbox_run\n  action: run\n  code: std.result.ok({ soma = 2 + 3 })\n```\n\n")
	b.WriteString("## Module details\n\n| Module | Description |\n| --- | --- |\n")
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
	if m.Body != "" {
		b.WriteString(strings.ReplaceAll(m.Body, "~~~", "```"))
		return b.String()
	}
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

var lunaMetaPages = map[string]*lunaMod{
	"meta":     lunaMeta,
	"tools":    lunaTools,
	"run":      lunaRun,
	"examples": lunaExamples,
	"limits":   lunaLimits,
	"env":      lunaEnv,
}

func lunaMetaTopic(name string) *lunaMod {
	return lunaMetaPages[name]
}

var lunaMeta = &lunaMod{
	Name: "meta",
	Desc: "script header and return convention",
	Body: `# Script — header and return

## Submitting code
Pass **only the body** of ~main~ — do not write ~function main(std)~ or ~end~. When run with ~sandbox_run~ (code=...), the wrapper ~function main(std) ... end~ is added automatically.

~~~lua
-- body only
local x = std.io.read("a.txt")
std.result.ok({ n = #x })
~~~

## Return
- ~std.result.ok(data)~ — success. ~data~ can be a string, number, boolean or table; objects/arrays become JSON.
- ~std.result.err(msg)~ — error (~msg~ becomes the error message).
- If ~result~ is not called, the value returned by ~main~ is used, and ~print()~/~std.log.*~ become the output section.

## Example
~~~lua
if std.args == nil then
  std.result.err("pass args")
end
std.result.ok({ received = std.args })
~~~`,
}

var lunaTools = &lunaMod{
	Name: "tools",
	Desc: "the MCP tools exposed by the sandbox server",
	Body: `# MCP tools

| Tool | Action | What it does |
| --- | --- | --- |
| ~sandbox_run~ | run (default) | Run a script by ~path~ (host .lua file) or inline ~code~, with optional ~args~ (array/object → ~std.args~). Bare ~code~ is auto-wrapped in ~function main(std)~. |
| ~sandbox_doc~ | topic | Return the ~std~ API documentation (optional ~topic~; e.g. ~run~, ~io~). |
| ~sandbox_os~ | copy, mount, del, stat, list | Manage the sandbox filesystem: copy (host→sandbox), mount (sandbox→host), delete, stat or tree-list a sandbox path. |

Scripts (via ~path~) live in the repo; ~std.tmp~ is for temporary data files.`,
}

var lunaRun = &lunaMod{
	Name: "run",
	Desc: "how to run scripts (inline code or path)",
	Body: `# sandbox_run — 2 modos de executar scripts

Executa um script Lua isolado (sem SO/processo; arquivos em ~mnt/~; rede via ~std.fetch~). Entre com **~path~** (arquivo .lua no host) ou **~code~** (inline).

Todo script é uma função ~function main(std) ... end~. **Em todos os modos o script precisa declarar ~function main(std)~** e terminar com ~std.result.ok(...)~ ou ~std.result.err(...)~. No modo ~code~ inline, se você mandar só o corpo, o wrapper é adicionado automaticamente — mas os exemplos abaixo já trazem o wrapper completo.

## Modo 1 — code (inline)
Conteúdo do script:

~~~lua
function main(std)
  local args = std.args or {}
  print("Ola do sandbox!", #args)
  std.result.ok({ ok = true, n = #args })
end
~~~

Chamada:
~~~jsonc
{
  "action": "run",
  "code": "function main(std)\n  local args = std.args or {}\n  print(\\"Ola do sandbox!\\", #args)\n  std.result.ok({ ok = true, n = #args })\nend",
  "args": [1, 2, 3]
}
~~~

## Modo 2 — path (arquivo .lua no host)
Crie o script no repositório (com ~function main(std)~):

~~~lua
-- scripts/relatorio.lua
function main(std)
  local mes = std.args and std.args.mes or "hoje"
  std.result.ok({ relatorio = mes, lido = true })
end
~~~

Chamada:
~~~jsonc
{
  "action": "run",
  "path": "scripts/relatorio.lua",
  "args": { "mes": "2026-09" }
}
~~~

## args → std.args
O campo ~args~ vira ~std.args~ dentro do script. Array vira table indexada; objeto vira table com chaves; uma string que é JSON válido também é interpretada; caso contrário permanece string. Sem ~args~, ~std.args~ é ~nil~.

~~~lua
function main(std)
  local nome = std.args and std.args.nome or "anon"
  std.result.ok({ nome = nome, recebido = std.args })
end
~~~

## Retorno
- ~std.result.ok(data)~ — sucesso; o ~data~ vira o JSON de saída.
- ~std.result.err(msg)~ — erro (~msg~ vira a mensagem).
- Sem ~result~, o valor retornado por ~main~ é usado; ~print()~/~std.log.*~ viram a seção de output.`,
}

var lunaExamples = &lunaMod{
	Name: "examples",
	Desc: "cookbook of small examples",
	Body: `# Examples (cookbook)

## Read and aggregate a file
~~~lua
if std.io.exists("data.txt") then
  local lines = std.io.lines("data.txt")
  std.result.ok({ count = #lines, first = lines[1] })
end
~~~

## Authenticated API call via secrets
~~~lua
if std.secrets.has("github_token_api") then
  local token = std.secrets.get("github_token_api")
  local res = std.fetch.json("https://api.github.com/user", {
    headers = { Authorization = "Bearer " .. token },
  })
  std.result.ok({ ok = res.ok, login = res.data and res.data.login })
end
~~~

## SQL round-trip
~~~lua
local sql = std.sql.connect("app.db")
sql.exec("CREATE TABLE IF NOT EXISTS t (id INTEGER PRIMARY KEY, name TEXT)")
sql.exec("INSERT INTO t (name) VALUES (?)", { "Ava" })
local schema = sql.schema()
local rows = sql.query("SELECT * FROM t")
std.result.ok({ tables = schema.tables, rows = rows })
~~~

## CSV
~~~lua
local rows = std.csv.parse("name,age\nAva,30\nNoah,22")
std.result.ok({ count = #rows, names = std.list.map(rows, function(r) return r[1] end) })
~~~

## Directory tree
~~~lua
local tree = std.io.walk("")
std.result.ok({ root = tree.name, children = #(tree.children or {}) })
~~~

## XML
~~~lua
local doc = std.xml.parse('<people><person id="1">Ava</person></people>')
local xml = std.xml.stringify({ name = "people", children = { { name = "person", attrs = { id = "2" }, text = "Noah" } } })
std.result.ok({ first = doc.children[1].text, xml = xml })
~~~

## Excel
~~~lua
std.excel.write("tmp:out.xlsx", { { "nome", "idade" }, { "Ava", 30 } })
local rows = std.data.from("excel", "tmp:out.xlsx")
std.result.ok({ nomes = std.list.map(rows, function(r) return r.nome end) })
~~~

## Data pipeline (CSV → JSON → XLSX)
~~~lua
std.data.convert("tmp:dados.csv", "tmp:dados.json")
std.data.convert("tmp:dados.json", "tmp:dados.xlsx")
~~~`,
}

var lunaLimits = &lunaMod{
	Name: "limits",
	Desc: "execution and storage limits",
	Body: `# Limits

- Execution: up to 180s (3 min) — configurable via ~SANDBOX_EXEC_TIMEOUT_SECONDS~.
- Output: 256 KiB (truncated).
- Result: 256 KiB (truncated with ~... (truncado)~).
- File: 2 MB per file; 1 MB per write.
- Paths are confined to the sandbox (no absolute paths, no ~..~).
- ~std.sql.query~ returns at most 10,000 rows (~SANDBOX_SQL_MAX_ROWS~).
- Secrets (~SECRET_*~) are auto-masked in any output/return.`,
}

var lunaEnv = &lunaMod{
	Name: "env",
	Desc: "environment / secrets configuration",
	Body: `# Environment

Secrets are available via ~std.secrets.get~ using ~SECRET_*~ variables (e.g., ~SECRET_GITHUB_TOKEN_API~ → ~std.secrets.get("github_token_api")~).

Other ~SANDBOX_*~ variables configure the sandbox (folders, limits, network) and are set by the operator — scripts don't need to read them.`,
}

var lunaResult = &lunaMod{
	Name:   "result",
	Prefix: "result",
	Desc:   "sets the script result (success/error); this is the `main` return value",
	Fns: []lunaFn{
		{"ok", "data:any", "any", "set success — `data` becomes the return (objects/arrays → JSON)"},
		{"render", "template:string, vars:table", "string", "renders `template` (`{{x}}`, `{{#if}}`, `{{#each}}`, `{{else}}`) and returns it as Markdown (`.md`)"},
		{"err", "msg:string", "panics", "set error — `msg` becomes the error message"},
	},
	Example: `local n = std.num.sum({ 1, 2, 3 })
std.result.ok({ total = n, ok = true })`,
}

var lunaLog = &lunaMod{
	Name:   "log",
	Prefix: "log",
	Desc:   "writes to the script's captured output (also `print()`)",
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
		{"glob", "pattern:string, opts?:table", "table<string>", "matching relative paths (`recursive`, `files`, `dirs`, `ext`)"},
		{"walk", "path?:string, opts?:table", "table<string>", "flat recursive list of paths (`files`, `dirs`, `ext`)"},
		{"tree", "path?:string", "table", "{ name, isDir, size, lines, children }"},
		{"zip", "dest:string, source:table|string", "table<string>", "creates a zip: array of paths (folders recursed), {name=content} or a single path; returns entry names"},
		{"unzip", "src:string, dest:string", "table<string>", "extracts a zip into a folder; returns the extracted paths (rejects zip-slip)"},
	},
	Example: `if std.io.exists("data.txt") then
  std.io.append("data.txt", "\nfim")
end
local csvs = std.io.glob("*.csv", { recursive = true })
local files = std.io.walk("", { files = true })
local root = std.io.tree("")
std.result.ok({ files = files, csvs = csvs, root = root.name })`,
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
		{"save", "url:string, path:string, opts?:table", "table", "GET and writes the body to a sandbox file; retries on 429/5xx (opts.retries, opts.backoffMs)"},
	},
	Example: `local res = std.fetch.get("http://localhost:8080/api", {
  headers = { Authorization = "Bearer " .. std.secrets.get("TOKEN") },
  timeout = 5000,
})
if res.ok then std.result.ok({ status = res.status, body = res.body }) end`,
}

var lunaHuman = &lunaMod{
	Name:   "human",
	Prefix: "human",
	Desc:   "humanizes numbers: bytes, durations, compact counts, currency, ordinals, plurals and lists (pt-BR)",
	Fns: []lunaFn{
		{"bytes", "n:number", "string", "bytes: `123 B`, `1,2 MB`, `3 GB`"},
		{"duration", "ms:number", "string", "milliseconds: `5 min`, `1 h 30 min`, `3,5 s`"},
		{"compact", "n:number", "string", "large numbers: `999`, `1,2 mil`, `3,4 mi`, `5 bi`"},
		{"money", "n:number, cur?:string", "string", "currency (default `R$`): `R$ 1.234,56`"},
		{"ordinal", "n:number", "string", "ordinal: `1º`, `12º`"},
		{"plural", "n:number, one:string, many:string", "string", "plural: `human.plural(2, 'item', 'itens')`"},
		{"list", "items:table", "string", "list: `a, b e c`"},
	},
	Example: `local b = std.human.bytes(1.2 * 1024 * 1024)
local d = std.human.duration(300000)
local m = std.human.money(1234.56)
std.result.ok({ b = b, d = d, m = m })`,
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
	Desc:   "SQLite in the sandbox — `connect(path)` returns a bound handle; module-level `import`/`export` take the path. The handle exposes exec/query/get/scalar/close/begin/commit/rollback/tables/columns/schema/import/export (all without the path)",
	Fns: []lunaFn{
		{"connect", "path:string", "object", "returns a bound handle with all query methods (no `path` arg)"},
		{"import", "path:string, table:string, rows:table<map>, opts?:table", "table", "{ imported } — bulk inserts rows; opts.create=true creates the table"},
		{"export", "path:string, sql:string", "table<row>", "query → rows as maps"},
	},
	Example: `local sql = std.sql.connect("app.db")
sql.exec("CREATE TABLE IF NOT EXISTS t (id INTEGER PRIMARY KEY, name TEXT)")
sql.exec("INSERT INTO t (name) VALUES (?)", { "Ava" })
local r = sql.query("SELECT * FROM t")
std.result.ok({ total = #r, first = r[1] and r[1].name })`,
}

var lunaUUID = &lunaMod{
	Name:   "uuid",
	Prefix: "uuid",
	Desc:   "UUID generation and validation",
	Fns: []lunaFn{
		{"v4", "-", "string", "random UUID (RFC 4122)"},
		{"v7", "-", "string", "time-ordered UUID (RFC 9562)"},
		{"valid", "s:string, version?:number", "bool", "is a valid UUID; optional version check (4/7)"},
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
		{"write", "path:string, rows:table, sheet?:string, opts?:table", "true", "writes the spreadsheet; color opts (header/row/alt/zebra)"},
	},
	Example: `local sheets = std.excel.sheets("dados.xlsx")
local rows = std.excel.read("dados.xlsx", sheets[1])
std.result.ok({ total = #rows, first = rows[1] })`,
}

var lunaData = &lunaMod{
	Name:   "data",
	Prefix: "data",
	Desc:   "data pipeline (sqlize-style): moves between CSV/JSON/XML/HTML/Excel/SQLite/SQL (rows = array of maps)",
	Fns: []lunaFn{
		{"from", "fmt:string, s:string, opts?:string", "any", "parse by format: csv, json, jsonl/ndjson, xml, html, sql (text) or excel/xlsx/xls (file path); opts = sep / root / sheet"},
		{"to", "fmt:string, v:any, opts?:string", "string", "serialize by format: csv, json, jsonl/ndjson, xml, html, sql (returns text); excel writes to a file (opts = path + sheet); color opts (header/row/alt/zebra)"},
		{"convert", "src:string, dst:string, opts?:table", "true", "converts by extension (opts = colors)"},
	},
	Example: `local rows = std.data.from("csv", "nome,idade\nAva,30")
std.sql.import("app.db", "pessoas", rows, { create = true })
std.result.ok({ total = #std.sql.export("app.db", "SELECT * FROM pessoas") })`,
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
		{"format", "v:any, opts?:table", "string", "pretty JSON; opts = { indent?, nested?, max? } — `nested` parseia strings que são JSON, `max` trunca o resultado"},
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
		{"crc32", "s:string", "hex", "CRC-32 (checksum)"},
		{"md5", "s:string", "hex", "MD5"},
		{"sha1", "s:string", "hex", "SHA-1"},
		{"sha224", "s:string", "hex", "SHA-224"},
		{"sha256", "s:string", "hex", "SHA-256"},
		{"sha384", "s:string", "hex", "SHA-384"},
		{"sha512", "s:string", "hex", "SHA-512"},
		{"sha3_256", "s:string", "hex", "SHA3-256"},
		{"sha3_512", "s:string", "hex", "SHA3-512"},
		{"argon2", "password:string, opts?:table", "string", "argon2id PHC hash (`memory`, `iterations`, `parallelism`, `salt`, `key_length`)"},
		{"argon2_verify", "password:string, hash:string", "bool", "verifies a password against an argon2id PHC hash"},
		{"base64", "s:string, mode?:string", "string", "encode/decode/url-safe"},
		{"base32", "s:string, mode?:string", "string", "encode/decode/hex/nopad"},
		{"hex", "s:string, mode?:string", "string", "encode/decode"},
	},
	Example: `local hash = std.encode.argon2("s3cret!")
std.result.ok({
  h = std.encode.sha256("abc"),
  b = std.encode.base64("abc"),
  ok = std.encode.argon2_verify("s3cret!", hash),
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
	Desc:   "dates in ms — `unit`: day, hour, minute, second, week, month, year; `tz` = IANA (ex. `America/Sao_Paulo`), resolve DST automaticamente",
	Fns: []lunaFn{
		{"now", "-", "number", "ms now"},
		{"iso", "ts?:number, tz?:string", "string", "RFC3339 (opcional tz)"},
		{"format", "layout:string, ts?:number, tz?:string", "string", "YYYY,MM,DD,HH,mm,ss (opcional tz)"},
		{"parse", "s:string, tz?:string", "number", "string → ms (opcional tz)"},
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
	Desc:   "random values (seeded) — numbers, picks and generated data (names/emails/sentences/passwords)",
	Fns: []lunaFn{
		{"seed", "n:number", "nil", "sets the seed (deterministic)"},
		{"int", "min:number, max:number", "number", "inclusive integer"},
		{"float", "min:number, max:number, dec?:number", "number", "decimal"},
		{"pick", "arr:table", "any", "random element"},
		{"shuffle", "arr:table", "table", "shuffled copy"},
		{"bool", "-", "bool", "random boolean"},
		{"name", "-", "string", "randomly generated full name"},
		{"email", "-", "string", "random email"},
		{"username", "-", "string", "random username"},
		{"phone", "-", "string", "phone"},
		{"date", "from?:string, to?:string", "string", "RFC3339 date"},
		{"sentence", "n?:number", "string", "n random sentences (default 1)"},
		{"paragraph", "n?:number", "string", "n random sentences (default 3)"},
		{"password", "length?:number, opts?:table", "string", "random password (`lower/upper/digits/symbols`, `exclude`, `allow`)"},
		{"token", "bytes?:number, opts?:table", "string", "crypto-safe token (base64url; `alphabet` custom)"},
		{"cpf", "-", "string", "CPF válido (dígito verificador)"},
		{"cnpj", "-", "string", "CNPJ válido (dígito verificador)"},
		{"cep", "-", "string", "CEP `NNNNN-NNN`"},
		{"rg", "-", "string", "RG com dígito verificador"},
	},
	Example: `std.random.seed(42)
std.result.ok({ n = std.random.int(1, 100), nome = std.random.name(), pw = std.random.password(12) })`,
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



var lunaTemplate = &lunaMod{
	Name:   "template",
	Prefix: "template",
	Desc:   "render text with placeholders and inline blocks — `{name}`/`{{name}}`, `{{#if}}`, `{{#unless}}`, `{{#each}}`, `{{else}}` (dot-path, `{{this}}`, `{{@index}}`)",
	Fns: []lunaFn{
		{"render", "str:string, vars:table", "string", "replaces placeholders; supports blocks `#if`/`#unless`/`#each`/`else` (missing → empty)"},
	},
	Example: `local t = [[
<ul>
{{#each itens}}<li>{{@index}}: {{nome}}{{#if ativo}} (ativo){{else}} (inativo){{/if}}</li>{{/each}}
</ul>
]]
std.result.ok({ html = std.template.render(t, {
  itens = { { nome = "Ava", ativo = true }, { nome = "Noah", ativo = false } },
}) })`,
}

var lunaPath = &lunaMod{
	Name:   "path",
	Prefix: "path",
	Desc:   "path manipulation (slash-based) — helpers for building mnt paths",
	Fns: []lunaFn{
		{"join", "...parts", "string", "joins segments"},
		{"basename", "p:string", "string", "last segment"},
		{"dirname", "p:string", "string", "parent dir"},
		{"ext", "p:string", "string", "extension with dot"},
		{"stem", "p:string", "string", "basename without ext"},
		{"split", "p:string", "table", "{dir, file}"},
		{"normalize", "p:string", "string", "cleans `.`/`..`/`//`"},
		{"is_abs", "p:string", "bool", "absolute?"},
		{"rel", "base:string, p:string", "string", "relative path"},
		{"within", "base:string, p:string", "bool", "is `p` inside `base`?"},
	},
	Example: `std.result.ok({
  j = std.path.join("mnt", "data", "x.csv"),
  e = std.path.ext("a/b/x.csv"),
})`,
}



var lunaStats = &lunaMod{
	Name:   "stats",
	Prefix: "stats",
	Desc:   "descriptive statistics on numeric arrays (or `opts.field`) — ignores non-numeric",
	Fns: []lunaFn{
		{"count", "arr:table, opts?:table", "number", "counts numeric entries"},
		{"min", "arr:table, opts?:table", "number", "smallest"},
		{"max", "arr:table, opts?:table", "number", "largest"},
		{"avg", "arr:table, opts?:table", "number", "mean"},
		{"median", "arr:table, opts?:table", "number", "median"},
		{"quantile", "arr:table, q:number", "number", "q in 0..1"},
		{"percentile", "arr:table, p:number", "number", "p in 0..100"},
		{"variance", "arr:table, sample?:bool", "number", "population or sample"},
		{"stdev", "arr:table, sample?:bool", "number", "standard deviation"},
		{"range", "arr:table, opts?:table", "table", "{min, max}"},
		{"mode", "arr:table, opts?:table", "table<number>", "most frequent values"},
		{"histogram", "arr:table, opts?:table", "table", "bins `{count, lo, hi}`"},
	},
	Example: `local xs = { 1, 2, 2, 3, 4, 5, 5, 5, 6 }
std.result.ok({ med = std.stats.median(xs), q = std.stats.quantile(xs, 0.25) })`,
}

var lunaSchema = &lunaMod{
	Name:   "schema",
	Prefix: "schema",
	Desc:   "validate rows/objects against a spec — returns `{ok, errors}` (no panic)",
	Fns: []lunaFn{
		{"validate", "rows:table, spec:table", "table", "validates a list of maps"},
		{"validate_one", "obj:table, spec:table", "table", "validates one map"},
		{"coerce", "v:any, type:string", "any", "coerces to string/number/bool"},
	},
	Example: `local res = std.schema.validate({
  { name = "Ava", age = 30, email = "a@x.com" },
}, {
  { field = "name",  type = "string", required = true },
  { field = "age",   type = "number", min = 0, max = 120 },
})
std.result.ok(res)`,
}

var lunaDiff = &lunaMod{
	Name:   "diff",
	Prefix: "diff",
	Desc:   "unified diff between strings (stdlib only) — no external library",
	Fns: []lunaFn{
		{"unified", "a:string, b:string, opts?:table", "string", "unified diff (`context`, `from`, `to`)"},
		{"lines", "a:string, b:string", "table", "ops `{op, text}` (equal/add/del)"},
		{"ratio", "a:string, b:string", "number", "similarity 0..1"},
	},
	Example: `std.result.ok({
  u = std.diff.unified("a\nb\nc", "a\nb\nd"),
  r = std.diff.ratio("abc", "abd"),
})`,
}

var lunaInfer = &lunaMod{
	Name:   "infer",
	Prefix: "infer",
	Desc:   "infer column types (integer/number/boolean/date/string) from values",
	Fns: []lunaFn{
		{"type", "values:table, opts?:table", "string", "inferred type of a column"},
		{"types", "rows:table, opts?:table", "table", "{ field = type } (opts.fields)"},
		{"detect", "v:any", "string", "type of one value"},
		{"coerce", "values:table, type?:string", "table", "coerces values (opts.type)"},
	},
	Example: `std.result.ok({
  t1 = std.infer.type({ "1", "2", "3.5" }),
  t2 = std.infer.type({ "2024-01-01", "2023-05-05" }),
})`,
}

var lunaMissing = &lunaMod{
	Name:   "missing",
	Prefix: "missing",
	Desc:   "handle null/missing values (nil, empty, NA/NULL/-) — fill/drop/which",
	Fns: []lunaFn{
		{"is_null", "v:any, opts?:table", "bool", "is it a missing value?"},
		{"count", "values:table, opts?:table", "number", "counts missing (opts.field)"},
		{"which", "values:table, opts?:table", "table<number>", "indices of missing"},
		{"fill", "values:table, opts?:table", "table", "fill missing (value/method)"},
		{"drop", "rows:table, opts?:table", "table", "drops rows with missing"},
	},
	Example: `std.result.ok({
  c = std.missing.count({ 1, nil, "", 3 }),
  f = std.missing.fill({ 1, nil, "", 3 }, { method = "mean" }),
})`,
}

var lunaText = &lunaMod{
	Name:   "text",
	Prefix: "text",
	Desc:   "text processing — regex extraction, tokens, n-grams, similarity/fuzzy",
	Fns: []lunaFn{
		{"extract", "s:string, pattern:string, opts?:table", "table<string>", "matches (opts.group)"},
		{"tokens", "s:string, opts?:table", "table<string>", "words (lower/min_len/stopwords)"},
		{"ngrams", "s:string, n?:number", "table<string>", "sliding n-word windows"},
		{"similarity", "a:string, b:string, opts?:table", "number", "0..1 (method)"},
		{"match", "a:string, b:string, threshold?:number", "bool", "similar enough?"},
		{"keywords", "s:string, n?:number", "table", "top {word, count}"},
	},
	Example: `std.result.ok({
  ex = std.text.extract("email: a@b.com", "[\\w.]+@[\\w.]+"),
  tok = std.text.tokens("Ola mundo, ola!"),
  sim = std.text.similarity("Joao", "João"),
})`,
}

var lunaPipe = &lunaMod{
	Name:   "pipe",
	Prefix: "pipe",
	Desc:   "row processing over a list of maps — select/rename/mutate/filter/sort/group/join (+ fluent `rows` builder)",
	Fns: []lunaFn{
		{"rows", "rows:table", "builder", "fluent builder; chain `:group():count():sum():run()`"},
		{"map", "rows:table, fn:function", "table", "transforms each row (nil drops)"},
		{"filter", "rows:table, fn:function", "table", "keeps rows where fn is truthy"},
		{"rename", "rows:table, mapping:table", "table", "renames columns"},
		{"select", "rows:table, fields:table", "table", "keeps only fields"},
		{"drop", "rows:table, fields:table", "table", "drops fields"},
		{"sort", "rows:table, field:string, desc?:bool", "table", "sorts by field(s)"},
		{"distinct", "rows:table, fields?:table", "table", "removes duplicates"},
		{"take", "rows:table, n:number", "table", "first n rows"},
		{"group", "rows:table, fields:table, aggs:table", "table", "group_by + agg"},
		{"join", "rows:table, right:table, opts:table", "table", "join (inner/left)"},
		{"coerce", "rows:table, field:string, type?:string", "table", "coerces a column"},
		{"infer", "rows:table, opts?:table", "table", "auto-coerces all columns"},
		{"pivot", "rows:table, opts:table", "table", "long→wide (`index`, `columns`, `values`, `agg`)"},
		{"unpivot", "rows:table, opts:table", "table", "wide→long (`id`, `cols`, `key`, `value`, `drop_nil`)"},
	},
	Example: `local out = std.pipe.rows(data)
  :group("regiao")
  :count("*", "n")
  :sum("receita", "total")
  :run()
local wide = std.pipe.pivot(vendas, { index={"ano"}, columns="trim", values="receita", agg="sum" })
std.result.ok({ out = out, wide = wide })`,
}

var lunaSeq = &lunaMod{
	Name:   "seq",
	Prefix: "seq",
	Desc:   "generate ranges/sequences and pack/zips — `range`, `iota`, `arange`, `repeat`, `zip`, `enumerate`",
	Fns: []lunaFn{
		{"range", "n:number", "table<number>", "`{1..n}` (count)"},
		{"iota", "n:number", "table<number>", "`{0..n-1}` (indices)"},
		{"arange", "start:number, stop:number, step?:number", "table<number>", "`[start, stop)` (negative step ok)"},
		{"repeat", "v:any, n:number", "table<any>", "`{v, v, ...}` n times"},
		{"zip", "...arrs", "table<table>", "i-th elements as tuples"},
		{"enumerate", "arr:table, start?:number", "table<table>", "`{ {i, item} }`"},
	},
	Example: `std.result.ok({
  r = std.seq.range(5),
  a = std.seq.arange(0, 10, 2),
})`,
}

var lunaDuration = &lunaMod{
	Name:   "duration",
	Prefix: "duration",
	Desc:   "durations in ms — parse/format/arithmetic (complements `date`/`human`)",
	Fns: []lunaFn{
		{"parse", "s:string", "number|nil", "`\"2h30m\"` → ms"},
		{"format", "ms:number", "string", "ms → `\"2h30m\"`"},
		{"parts", "ms:number", "table", "{w, d, h, m, s, ms}"},
		{"total", "ms:number, unit:string", "number", "ms in a unit (day/hour/minute/second/week)"},
		{"compare", "a, b", "number", "-1/0/1 (accepts ms or string)"},
		{"add", "ts:number, dur", "number", "ms + duration"},
		{"sub", "ts:number, dur", "number", "ms − duration"},
	},
	Example: `local ms = std.duration.parse("2h30m")
std.result.ok({ ms = ms, fmt = std.duration.format(ms), tot = std.duration.total(ms, "minute") })`,
}

var lunaUnit = &lunaMod{
	Name:   "unit",
	Prefix: "unit",
	Desc:   "unit conversion — length/mass/data/time/volume/temperature (aliases incl. pt-BR)",
	Fns: []lunaFn{
		{"convert", "v:number, from:string, to:string", "number", "converts between compatible units"},
		{"factor", "unit:string", "number", "factor to the base unit (linear)"},
		{"list", "-", "table", "{ category = { units } }"},
	},
	Example: `std.result.ok({
  km = std.unit.convert(5, "km", "m"),
  c  = std.unit.convert(0, "c", "f"),
})`,
}
