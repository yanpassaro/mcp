package mcpserver

import (
	"context"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type docInput struct {
	Topic string `json:"topic,omitempty" jsonschema:"Documentation topic (module or section). If omitted, returns the index/overview."`
}

var docAliases = map[string]string{
	"":         "index",
	"all":      "index",
	"overview": "index",
	"help":     "index",
	"fs":       "io",
	"file":     "io",
	"files":    "io",
	"enc":      "encode",
	"strings":  "str",
	"text":     "str",
	"arrays":   "list",
	"array":    "list",
	"numbers":  "num",
	"number":   "num",
	"números":  "num",
	"dates":    "date",
	"date":     "date",
	"cookies":   "cookies",
	"sqlite":    "sql",
	"csv":       "csv",
	"regex":     "regex",
	"regexp":    "regex",
	"fake":      "fake",
	"faker":     "fake",
	"pii":       "pii",
	"mask":      "pii",
	"redact":    "pii",
	"xml":       "xml",
	"excel":     "excel",
	"data":      "data",
	"pipeline":  "data",
	"xlsx":      "excel",
	"spreadsheet": "excel",
	"examples":  "examples",
	"cookbook":  "examples",
	"recipes":   "examples",
	"limits":    "limits",
	"env":      "env",
	"meta":     "meta",
	"tools":    "tools",
}

var docTopics = []string{
	"index", "meta", "tools",
	"io", "tmp", "fetch", "cookies", "secrets", "sql", "result", "log", "args",
	"json", "encode", "str", "list", "num", "date", "random", "uuid", "assert", "fake", "pii", "csv", "regex", "xml", "excel", "data",
	"limits", "env", "examples",
}

func (s *Server) doc(ctx context.Context, _ *mcp.CallToolRequest, in docInput) (*mcp.CallToolResult, any, error) {
	topic := strings.ToLower(strings.TrimSpace(in.Topic))
	if alias, ok := docAliases[topic]; ok {
		topic = alias
	}
	doc, ok := sandboxDocs[topic]
	if !ok {
		return textResult(renderDoc(sandboxDocs["index"]) + "\n\n⚠️ Topic `" + strings.TrimSpace(in.Topic) + "` not found. Available topics: " + strings.Join(docTopics, ", ") + ".\n")
	}
	return textResult(renderDoc(doc))
}

func renderDoc(doc string) string {
	return strings.ReplaceAll(doc, "~~~", "```")
}

var sandboxDocs = map[string]string{
	"index": `# Sandbox Lua — Documentation

The sandbox runs isolated Lua scripts (no OS access; network only via ~std.fetch~). Each script defines ~function main(std)~ and finishes with ~std.result.ok(...)~ or ~std.result.err(...)~. Output becomes Markdown (objects/arrays become JSON).

## How to write a script

When using ~sandbox_write~, pass **only the body** of ~main~. The ~-- name=~/~-- desc=~ header and the ~function main(std) ... end~ wrapper are added automatically from the ~name~/~description~ arguments.

~~~lua
local data = std.io.read("data.txt")
local n = std.num.parse(data)
std.result.ok({ lines = std.io.lines("data.txt"), total = n })
~~~

Write it with ~sandbox_write~ (name + code) and run it with ~sandbox_run~. Tools: ~sandbox_read~, ~sandbox_write~, ~sandbox_del~, ~sandbox_run~, ~sandbox_manage~, ~sandbox_doc~.

## ~std~ modules

| Module | Functions |
| --- | --- |
| ~result~ | ~ok~, ~err~ |
| ~log~ | ~ok~, ~info~, ~warn~, ~error~, ~err~, ~log~ |
| ~args~ | the ~args~ value (JSON or string) |
| ~io~ | ~read~, ~lines~, ~json~, ~write~, ~append~, ~del~, ~exists~, ~stat~, ~dir~, ~copy~, ~move~, ~mkdir~, ~glob~, ~walk~ |
| ~tmp~ | same as ~io~ + ~clear~ |
| ~fetch~ | ~request~, ~get~, ~post~, ~json~, ~cookies~ |
| ~secrets~ | ~get~, ~has~ |
| ~sql~ | ~exec~, ~query~, ~get~, ~scalar~, ~close~, ~begin~, ~commit~, ~rollback~, ~tables~, ~columns~, ~schema~ |
| ~uuid~ | ~v4~, ~v7~ |
| ~csv~ | ~parse~, ~stringify~ |
| ~xml~ | ~parse~, ~stringify~ |
| ~excel~ | ~sheets~, ~read~, ~write~ |
| ~data~ | ~fromCSV~, ~toCSV~, ~fromJSON~, ~toJSON~, ~toXML~, ~fromExcel~, ~toExcel~, ~sqlImport~, ~sqlExport~, ~convert~ |
| ~regex~ | ~match~, ~find~, ~findAll~, ~replace~, ~split~, ~groups~, ~findAllGroups~ |
| ~json~ | ~parse~, ~stringify~, ~format~, ~minify~, ~path~ |
| ~encode~ | ~crc32~, ~md5~, ~sha256~, ~base64~, ~hex~ |
| ~str~ | ~normalize~, ~slug~, ~title~, ~camel~, ~pascal~, ~snake~, ~kebab~, ~wrap~, ~summarize~, ~format~, ~count~, ~split~ |
| ~list~ | ~chunk~, ~groupBy~, ~unique~, ~flatten~, ~sortBy~, ~countBy~, ~first~, ~last~, ~map~, ~filter~, ~reduce~, ~find~, ~some~, ~every~ |
| ~num~ | ~round~, ~clamp~, ~percent~, ~sum~, ~avg~, ~parse~, ~fmt~ |
| ~date~ | ~now~, ~iso~, ~format~, ~parse~, ~add~, ~unix~, ~diff~ |
| ~random~ | ~seed~, ~int~, ~pick~, ~shuffle~ |
| ~assert~ | ~ok~, ~equal~, ~throws~, ~type~, ~notNil~, ~number~, ~string~, ~boolean~, ~table~, ~contains~, ~matches~, ~between~, ~length~ |
| ~fake~ | ~seed~, ~name~, ~email~, ~username~, ~phone~, ~int~, ~float~, ~bool~, ~uuid~, ~date~, ~words~, ~sentence~, ~paragraph~ |
| ~pii~ | ~has~, ~detect~, ~mask~, ~maskRows~ |

Call ~sandbox_doc~ with a topic for details (e.g., ~sandbox_doc~ topic=~io~). Topics: ~meta~, ~io~, ~tmp~, ~fetch~, ~cookies~, ~secrets~, ~sql~, ~uuid~, ~csv~, ~regex~, ~xml~, ~excel~, ~data~, ~result~, ~log~, ~json~, ~encode~, ~str~, ~list~, ~num~, ~date~, ~random~, ~assert~, ~fake~, ~limits~, ~env~, ~tools~, ~examples~.

## Limits

- Execution: up to 30s.
- Output: 256 KiB. File: 2 MB.

## Secrets example

~~~lua
-- SECRET_GITHUB_TOKEN_API
local token = std.secrets.get("github_token_api")
if token == nil then std.result.err("secret not configured") end
local res = std.fetch.json("https://api.github.com/user", { headers = { Authorization = "Bearer " .. token } })
std.result.ok({ login = res.data.login })
~~~`,
	"meta": `# Script — header and return

## Submitting code
When you call ~sandbox_write~, pass **only the body** of ~main~ — do not write ~function main(std)~ or ~end~. The header (~-- name=~/~-- desc=~) and the wrapper are added automatically from the ~name~/~description~ arguments.

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
	"tools": `# MCP tools

| Tool | What it does |
| --- | --- |
| ~sandbox_read~ | Read a saved script by ~name~, or list all; ~name~ may be ~.~ or a glob (e.g. ~*.lua~). |
| ~sandbox_write~ | Create/overwrite a script (~name~ + ~code~; ~description~ optional). Pass **only the body** — the ~function main(std)~ wrapper is added automatically. Returns the stored script. |
| ~sandbox_del~ | Delete a saved script by ~name~. |
| ~sandbox_run~ | Run a saved script by ~name~, with optional ~args~ (JSON or string). |
| ~sandbox_manage~ | Filesystem actions: ~copy~ (host→mnt), ~mount~ (mnt→host), ~del~, ~stat~, ~list~ (tree). |
| ~sandbox_doc~ | Return the ~std~ API documentation (optional ~topic~). |

Scripts and data files are stored in separate sandbox folders; ~std.tmp~ is for temporary files.`,
	"io": `# std.io — sandbox files (~mnt/~)

Operates on paths relative to ~mnt/~. Rejects absolute paths and ~..~ (always confined to the sandbox).

| Function | Signature | Returns |
| --- | --- | --- |
| ~read~ | ~read(name)~ | string |
| ~lines~ | ~lines(name)~ | table of (non-empty) lines |
| ~json~ | ~json(name)~ | any (parses the file) |
| ~write~ | ~write(name, content)~ | int (bytes) |
| ~append~ | ~append(name, content)~ | int (bytes) |
| ~del~ | ~del(name)~ | true (files only) |
| ~exists~ | ~exists(name)~ | bool |
| ~stat~ | ~stat(name)~ | { name, exists, isDir, size, lines } |
| ~dir~ | ~dir(rel?)~ | table of file names |
| ~copy~ | ~copy(src, dst)~ | true (within the sandbox) |
| ~move~ | ~move(src, dst)~ | true (rename) |
| ~mkdir~ | ~mkdir(path)~ | true (creates folders) |
| ~glob~ | ~glob(pattern)~ | table (paths relative to the sandbox) |
| ~walk~ | ~walk(path?)~ | { name, isDir, size, lines, children } |

## Examples
~~~lua
if std.io.exists("a.txt") then
  local t = std.io.read("a.txt")
  std.io.append("a.txt", "\nnew line")
end
std.io.write("dir/x.json", std.json.stringify({ ok = 1 }))
local files = std.io.glob("*.lua")
local tree = std.io.walk("")
std.result.ok({ files = files, tree = tree.children })
~~~`,
	"tmp": `# std.tmp — temporary files (~tmp/~)

Same functions as ~std.io~ (~read~, ~write~, ~append~, ~del~, ~exists~, ~stat~, ~dir~, ~copy~, ~move~, ~mkdir~, ~glob~, ~walk~), but rooted in ~tmp/~.

Extra:
- ~clear()~ → number (how many items were removed; empties the temp folder).

Example:
~~~lua
local tmp = std.tmp.write("t.json", std.json.stringify({ a = 1 }))
std.result.ok({ bytes = tmp })
~~~
~tmp/~ files are temporary and cleaned automatically.`,
	"fetch": `# std.fetch — HTTP (network via allowlist)

| Function | Signature | Returns |
| --- | --- | --- |
| ~request~ | ~request(url, opts?)~ | table |
| ~get~ | ~get(url, opts?)~ | table |
| ~post~ | ~post(url, body?, opts?)~ | table |
| ~json~ | ~json(url, opts?)~ | table (parses the body) |

~opts~: ~method~, ~body~ (string), ~headers~ (table), ~timeout~ (ms), ~noCookies~ (bool), ~followRedirects~ (bool).

~res~: { ~status~, ~statusText~, ~ok~, ~headers~, ~body~, ~truncated~, ~bytes~, ~ms~ }. For ~json~: { ~status~, ~ok~, ~headers~, ~data~ (parsed), ~truncated~, ~bytes~, ~ms~ } (no raw ~body~).

## Example
~~~lua
local res = std.fetch.get("http://localhost:8080/api", {
  headers = { Authorization = "Bearer " .. std.secrets.get("TOKEN") },
  timeout = 5000,
})
if res.ok then std.result.ok({ status = res.status, body = res.body }) end
~~~

## Cookies (~std.fetch.cookies~)
- ~list()~ → table of cookies.
- ~clear(domain?)~ → number (removes).
- ~set(domain, name, value, opts?)~ → bool.

Only allowlisted hosts can be reached.`,
	"cookies": `# std.fetch.cookies

- ~list()~ → table of { domain, name, value, path, secure, httpOnly }.
- ~clear(domain?)~ → number (without a domain, clears everything).
- ~set(domain, name, value, opts?)~ → bool.

~opts~: ~path~ (string), ~secure~ (bool).

Persisted in ~SANDBOX_FETCH_COOKIE_FILE~.`,
	"secrets": `# std.secrets

- ~get(key)~ → string | nil.
- ~has(key)~ → bool.

Reads the ~SECRET_<KEY>~ environment variable (case-insensitive). E.g., ~SECRET_GITHUB_TOKEN_API~ → ~std.secrets.get("github_token_api")~. There is no ~set~.

Values from any ~SECRET_*~ are automatically replaced with ~[REDACTED]~ in all output/returns — they never reach the AI, but can still be used inside the script.

~~~lua
if std.secrets.has("github_token_api") then
  local t = std.secrets.get("github_token_api")
  std.result.ok({ ok = true, present = t ~= nil })  -- NEVER return the token
else
  std.result.err("not configured")
end
~~~`,
	"sql": `# std.sql — SQLite databases

Access SQLite databases inside the sandbox. Within a script run, the same path reuses the same connection (so tables/transactions persist across calls); all connections are closed automatically when the run ends. Use ~close(path)~ to release early.

| Function | Signature | Returns |
| --- | --- | --- |
| ~exec~ | ~exec(path, sql, params?)~ | { lastId, rows } |
| ~query~ | ~query(path, sql, params?)~ | table of row maps |
| ~get~ | ~get(path, sql, params?)~ | first row map or nil |
| ~scalar~ | ~scalar(path, sql, params?)~ | first column of the first row |
| ~close~ | ~close(path)~ | bool (released early) |
| ~begin~ | ~begin(path)~ | starts a transaction (true) |
| ~commit~ | ~commit(path)~ | commits (true) |
| ~rollback~ | ~rollback(path)~ | rolls back (true) |
| ~tables~ | ~tables(path)~ | table of table names |
| ~columns~ | ~columns(path, table)~ | table of { name, type, notnull, pk, dflt } |
| ~schema~ | ~schema(path)~ | { tables, columns = { [table] = {...} } } |

~params~ is an array bound to the ~?~ placeholders. Paths are relative to ~mnt/~; prefix with ~tmp:~ to store in the temp folder (e.g., ~tmp:scratch.db~). Use ~:memory:~ for an in-memory DB (shared within the run). ~query~ returns at most ~SANDBOX_SQL_MAX_ROWS~ rows (default 10,000).

While a transaction is open for a path, ~exec~/~query~ use it automatically.

## Example
~~~lua
std.sql.exec("app.db", "CREATE TABLE IF NOT EXISTS t (id INTEGER PRIMARY KEY, name TEXT)")
std.sql.exec("app.db", "INSERT INTO t (name) VALUES (?)", { "Ava" })
local r = std.sql.query("app.db", "SELECT * FROM t")  -- same DB as above
std.result.ok({ total = #r, first = r[1] and r[1].name })
~~~`,
	"result": `# std.result

- ~ok(data)~ → sets success (~data~ becomes the return value; objects/arrays become JSON).
- ~err(msg)~ → sets error (~msg~ becomes the error message).

If nothing is called, the return value of ~main~ is used. ~std.result.ok({ ... })~ is the common way to return data.

~~~lua
std.result.ok({ total = 10, names = { "Ava", "Lua" } })
-- becomes formatted JSON in the result
~~~`,
	"log": `# std.log and console

- ~std.log.ok(...)~, ~std.log.info(...)~, ~std.log.warn(...)~, ~std.log.error(...)~, ~std.log.err(...)~, ~std.log.log(...)~
- ~console.*~ (same names) and the built-in ~print()~.

All write to the captured output of the script (shown in the result). Secret values are redacted in that output.

~~~lua
std.log.info("processing", n)
print("extra line")
~~~`,
	"args": `# std.args

~std.args~ is the ~args~ value passed when running the script (~sandbox_run~ args=...). If it is valid JSON it becomes an object/array; otherwise it stays a string. It can be ~nil~/~false~ when no args are passed.

~~~lua
local name = std.args and std.args.name or "anonymous"
std.result.ok({ name = name })
~~~`,
	"json": `# std.json

| Function | Signature | Returns |
| --- | --- | --- |
| ~parse~ | ~parse(s)~ | any |
| ~stringify~ | ~stringify(v, indent?)~ | string |
| ~format~ | ~format(v)~ | string (indented) |
| ~minify~ | ~minify(s)~ | string |
| ~path~ | ~path(v, "a.b")~ | any (dot path; index arrays by number) |

~~~lua
local v = std.json.parse('{"a": [1,2]}')
local s = std.json.stringify({ a = 1 }, 2)
local x = std.json.path(v, "a.1")  -- 2
~~~`,
	"encode": `# std.encode

| Function | Signature | Returns |
| --- | --- | --- |
| ~crc32~ | ~crc32(s)~ | hex |
| ~md5~ | ~md5(s)~ | hex |
| ~sha256~ | ~sha256(s)~ | hex |
| ~base64~ | ~base64(s, mode?)~ | string (encode/decode/url) |
| ~hex~ | ~hex(s, mode?)~ | string (encode/decode) |

~base64~ ~mode~: ~""~/~encode~/~std~/~standard~, ~decode~/~dec~, ~url~/~urlsafe~.
~hex~: ~encode~/~enc~ or ~decode~/~dec~.

~~~lua
local h = std.encode.sha256("abc")
local b = std.encode.base64("abc")
local back = std.encode.base64(b, "decode")
~~~`,
	"str": `# std.str

| Function | Signature | Returns |
| --- | --- | --- |
| ~normalize~ | ~normalize(s)~ | string (strips accents) |
| ~slug~ | ~slug(s)~ | string |
| ~title~ | ~title(s)~ | string |
| ~camel~ | ~camel(s)~ | string |
| ~pascal~ | ~pascal(s)~ | string |
| ~snake~ | ~snake(s)~ | string |
| ~kebab~ | ~kebab(s)~ | string |
| ~wrap~ | ~wrap(s, width?)~ | table (lines) |
| ~summarize~ | ~summarize(s, max)~ | string |
| ~format~ | ~format(fmt, ...)~ | string (printf-style) |
| ~count~ | ~count(s, sub)~ | number |
| ~split~ | ~split(s, sep, limit?)~ | table |

~~~lua
std.result.ok({
  slug = std.str.slug("Hello World"),
  title = std.str.title("hello world"),
  fmt = std.str.format("Hi %s / %d", "Ava", 42),
})
~~~`,
	"list": `# std.list

| Function | Signature | Returns |
| --- | --- | --- |
| ~chunk~ | ~chunk(arr, n)~ | table of tables |
| ~groupBy~ | ~groupBy(arr, prop)~ | table (key=prop, value=list) |
| ~unique~ | ~unique(arr)~ | table |
| ~flatten~ | ~flatten(arr)~ | table |
| ~sortBy~ | ~sortBy(arr, prop)~ | table (sorted copy) |
| ~countBy~ | ~countBy(arr, prop)~ | table (key=prop, value=count) |
| ~first~ | ~first(arr, n?)~ | table |
| ~last~ | ~last(arr, n?)~ | table |
| ~map~ | ~map(arr, fn)~ | table |
| ~filter~ | ~filter(arr, fn)~ | table |
| ~reduce~ | ~reduce(arr, fn, init?)~ | any |
| ~find~ | ~find(arr, fn)~ | any (or nil) |
| ~some~ | ~some(arr, fn)~ | bool |
| ~every~ | ~every(arr, fn)~ | bool |

The functional ones take a Lua callback ~fn~ (e.g., ~function(x) return x*2 end~). ~reduce~ uses ~fn(acc, x)~; the others use ~fn(x)~.

~~~lua
local names = { "Ava", "Noah", "Ava" }
local un = std.list.unique(names)  -- { "Ava", "Noah" }
local dbl = std.list.map({ 1, 2, 3 }, function(x) return x * 2 end)  -- { 2, 4, 6 }
local evens = std.list.filter({ 1, 2, 3, 4 }, function(x) return x % 2 == 0 end)
local sum = std.list.reduce({ 1, 2, 3 }, function(acc, x) return acc + x end, 0)
local has2 = std.list.some({ 1, 2, 3 }, function(x) return x == 2 end)  -- true
std.result.ok({ un = un, dbl = dbl, evens = evens, sum = sum, has2 = has2 })
~~~`,
	"num": `# std.num

| Function | Signature | Returns |
| --- | --- | --- |
| ~round~ | ~round(v, digits?)~ | number |
| ~clamp~ | ~clamp(v, lo, hi)~ | number |
| ~percent~ | ~percent(a, b)~ | number (a/b*100) |
| ~sum~ | ~sum(arr)~ | number |
| ~avg~ | ~avg(arr)~ | number |
| ~parse~ | ~parse(s)~ | number (NaN if invalid) |
| ~fmt~ | ~fmt(n, dec?, loc?)~ | string (loc=pt-BR → 1.234,56) |

~~~lua
std.result.ok({
  total = std.num.sum({ 1, 2, 3 }),
  pct = std.num.percent(2, 4),      -- 50
  fmt = std.num.fmt(1234.5, 2, "pt-BR"),
})
~~~`,
	"date": `# std.date

| Function | Signature | Returns |
| --- | --- | --- |
| ~now~ | ~now()~ | number (ms) |
| ~iso~ | ~iso(ts?)~ | string RFC3339 |
| ~format~ | ~format(layout, ts?)~ | string (YYYY,MM,DD,HH,mm,ss) |
| ~parse~ | ~parse(s)~ | number (ms) |
| ~add~ | ~add(ts, amount, unit)~ | number (ms) |
| ~unix~ | ~unix(ts?)~ | number (s) |
| ~diff~ | ~diff(a, b, unit)~ | number |

~unit~: ~day~, ~hour~, ~minute~, ~second~, ~week~, ~month~, ~year~.

~~~lua
local now = std.date.now()
local later = std.date.add(now, 2, "hour")
std.result.ok({ iso = std.date.iso(later) })
~~~`,
	"random": `# std.random

| Function | Signature | Returns |
| --- | --- | --- |
| ~seed~ | ~seed(n)~ | — |
| ~int~ | ~int(min, max)~ | number (inclusive) |
| ~pick~ | ~pick(arr)~ | any (random element) |
| ~shuffle~ | ~shuffle(arr)~ | table (new order) |

~~~lua
std.random.seed(123)
local n = std.random.int(1, 6)
local s = std.random.shuffle({ "a", "b", "c" })
~~~`,
	"assert": `# std.assert

| Function | Signature | What it does |
| --- | --- | --- |
| ~ok~ | ~ok(v, msg?)~ | panics if ~v~ is falsy/nil |
| ~equal~ | ~equal(a, b)~ | panics if not deep-equal |
| ~throws~ | ~throws(fn)~ | panics if the function does not raise |
| ~type~ | ~type(v, kind)~ | panics if ~v~ is not the given kind |
| ~notNil~ | ~notNil(v, msg?)~ | panics if ~v~ is nil |
| ~number~/~string~/~boolean~/~table~ | ~fn(v, msg?)~ | panics if type is wrong |
| ~contains~ | ~contains(s, sub)~ | panics unless ~s~ contains ~sub~ |
| ~matches~ | ~matches(s, pattern)~ | panics unless ~s~ matches (Go regex) |
| ~between~ | ~between(v, lo, hi)~ | panics if ~v~ outside [lo, hi] |
| ~length~ | ~length(v, n)~ | panics if length != n |

~~~lua
std.assert.type({}, "table")
std.assert.number(42)
std.assert.contains("sandbox", "sand")
std.assert.matches("abc123", "[0-9]+")
std.assert.length("abcd", 4)
std.result.ok({ checked = true })
~~~`,
	"uuid": `# std.uuid

| Function | Signature | Returns |
| --- | --- | --- |
| ~v4~ | ~v4()~ | random UUID (RFC 4122) |
| ~v7~ | ~v7()~ | time-ordered UUID (RFC 9562, unix ms + random) |

~~~lua
local id = std.uuid.v4()
local ordered = std.uuid.v7()
std.result.ok({ id = id, ordered = ordered })
~~~`,
	"csv": `# std.csv

| Function | Signature | Returns |
| --- | --- | --- |
| ~parse~ | ~parse(s, sep?)~ | table of rows (each row = table of cells) |
| ~stringify~ | ~stringify(rows, sep?)~ | string |

~sep~ defaults to ~,~.

~~~lua
local rows = std.csv.parse("a,b\n1,2\n3,4")
local s = std.csv.stringify(rows)
std.result.ok({ rows = rows, csv = s })
~~~`,
	"xml": `# std.xml

| Function | Signature | Returns |
| --- | --- | --- |
| ~parse~ | ~parse(s)~ | root node table |
| ~stringify~ | ~stringify(node)~ | string |

A node is a table: ~{ name, attrs = {...}, text = "...", children = {...} }~. ~attrs~ and ~children~ are optional; ~children~ is an array of node tables.

~~~lua
local doc = std.xml.parse('<root><item id="1">Ava</item></root>')
local xml = std.xml.stringify({ name = "root", children = { { name = "item", attrs = { id = "1" }, text = "Ava" } } })
std.result.ok({ name = doc.name, first = doc.children[1] })
~~~`,
	"excel": `# std.excel — .xlsx spreadsheets

| Function | Signature | Returns |
| --- | --- | --- |
| ~sheets~ | ~sheets(path)~ | table of sheet names |
| ~read~ | ~read(path, sheet?)~ | rows (array of arrays) |
| ~write~ | ~write(path, rows, sheet?)~ | true |

Paths are relative to ~mnt/~ (or prefixed with ~tmp:~). ~read~ uses the first sheet if ~sheet~ is omitted. ~rows~ is an array of arrays (cells).

~~~lua
local sheets = std.excel.sheets("dados.xlsx")
local rows = std.excel.read("dados.xlsx", sheets[1])
std.excel.write("tmp:out.xlsx", { { "nome", "idade" }, { "Ava", 30 } })
std.result.ok({ total = #rows, first = rows[1] })
~~~`,
	"data": `# std.data — data pipeline (sqlize-style)

	Operates on rows = array of maps. Moves data between CSV, JSON, XML, Excel and SQLite.

	| Function | Signature | Returns |
	| --- | --- | --- |
	| ~fromCSV~ | ~fromCSV(s, sep?)~ | rows |
	| ~toCSV~ | ~toCSV(rows, sep?)~ | string |
	| ~fromJSON~ | ~fromJSON(s)~ | any |
	| ~toJSON~ | ~toJSON(v)~ | string |
	| ~toXML~ | ~toXML(rows, root?, row?)~ | string |
	| ~fromExcel~ | ~fromExcel(path, sheet?)~ | rows |
	| ~toExcel~ | ~toExcel(rows, path, sheet?)~ | true |
	| ~sqlImport~ | ~sqlImport(db, table, rows, opts?)~ | { imported } |
	| ~sqlExport~ | ~sqlExport(db, query)~ | rows |
	| ~convert~ | ~convert(src, dst)~ | true (by extension) |

	Paths are relative to ~mnt/~ (or ~tmp:~). ~sqlImport~ creates the table if it doesn't exist (~opts.create=true~ to force), with TEXT columns.

	## Example
	~~~lua
	local rows = std.data.fromCSV("nome,idade\nAva,30\nNoah,22")
	std.data.sqlImport("app.db", "pessoas", rows, { create = true })
	local back = std.data.sqlExport("app.db", "SELECT * FROM pessoas")
	std.data.convert("tmp:dados.csv", "tmp:dados.xlsx")
	std.result.ok({ total = #back })
	~~~`,
		"regex": `# std.regex

		Uses Go's ~regexp~ (RE2 syntax — references and escapes differ from Lua patterns).

		| Function | Signature | Returns |
		| --- | --- | --- |
		| ~match~ | ~match(s, pattern)~ | bool |
		| ~find~ | ~find(s, pattern)~ | first match or nil |
		| ~findAll~ | ~findAll(s, pattern, limit?)~ | table of matches |
		| ~replace~ | ~replace(s, pattern, repl)~ | string (~$1~ refs) |
		| ~split~ | ~split(s, pattern, limit?)~ | table |
		| ~groups~ | ~groups(s, pattern)~ | { match, groups = {...} } or nil |
		| ~findAllGroups~ | ~findAllGroups(s, pattern, limit?)~ | table of the above |

		~~~lua
		local m  = std.regex.match("abc123", "[0-9]+")                -- true
		local xs = std.regex.findAll("a1 b22", "[0-9]+")              -- { "1", "22" }
		local s  = std.regex.replace("join 2024", "[0-9]+", "2025")    -- "join 2025"
		local g  = std.regex.groups("id=42", "id=([0-9]+)")             -- { match="id=42", groups={ "42" } }
		std.result.ok({ m = m, xs = xs, s = s, id = g.groups[1] })
		~~~
	`,
	"examples": `# Examples (cookbook)

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
std.sql.exec("app.db", "CREATE TABLE IF NOT EXISTS t (id INTEGER PRIMARY KEY, name TEXT)")
std.sql.exec("app.db", "INSERT INTO t (name) VALUES (?)", { "Ava" })
local schema = std.sql.schema("app.db")
local rows = std.sql.query("app.db", "SELECT * FROM t")
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
local rows = std.data.fromExcel("tmp:out.xlsx")
std.result.ok({ nomes = std.list.map(rows, function(r) return r.nome end) })
~~~

## Data pipeline (CSV → JSON → XLSX)
~~~lua
std.data.convert("tmp:dados.csv", "tmp:dados.json")
std.data.convert("tmp:dados.json", "tmp:dados.xlsx")
~~~`,
	"fake": `# std.fake — fake data (Faker-style)

	Seeded (deterministic) fake data for tests/demos.

	| Function | Signature | Returns |
	| --- | --- | --- |
	| ~seed~ | ~seed(n)~ | — |
	| ~name~ | ~name()~ | string |
	| ~firstName~ / ~lastName~ | ~fn()~ | string |
	| ~email~ | ~email()~ | string |
	| ~username~ | ~username()~ | string |
	| ~phone~ | ~phone()~ | string |
	| ~int~ | ~int(min, max)~ | number |
	| ~float~ | ~float(min, max, dec?)~ | number |
	| ~bool~ | ~bool()~ | bool |
	| ~uuid~ | ~uuid()~ | string (v4) |
	| ~date~ | ~date(from?, to?)~ | RFC3339 string |
	| ~words~ / ~sentence~ / ~paragraph~ | ~fn(n?)~ | string |

	~~~lua
	std.fake.seed(42)
	local p = { nome = std.fake.name(), email = std.fake.email(), n = std.fake.int(1, 100) }
	std.result.ok(p)
	~~~`,
		"pii": `# std.pii — detect and mask PII

		Detects and masks personally identifiable information (CPF, CNPJ, email, phone, card, address, secrets, ...).

		**Auto-mask:** sensitive PII (CPF, CNPJ, email, phone, card, JWT, BTC) is automatically replaced in all outputs/returns (like secrets). Dates, IPs and URLs are NOT auto-masked, to avoid breaking normal data — use ~std.pii.mask~ for a full mask.

		| Function | Signature | Returns |
		| --- | --- | --- |
		| ~has~ | ~has(s)~ | bool |
		| ~detect~ | ~detect(s)~ | table of { type, value } |
		| ~mask~ | ~mask(s)~ | string (replaces with [TYPE]) |
		| ~maskRows~ | ~maskRows(rows)~ | rows with PII columns masked |

		~maskRows~ detects PII columns by header name (e.g. ~cpf~, ~email~, ~telefone~, ~senha~...) and masks their cells (secrets become ~[REDACTED]~).

		~~~lua
		local s = "Contato: joao@ex.com, CPF 123.456.789-01"
		local masked = std.pii.mask(s)              -- "Contato: [EMAIL], CPF [CPF]"
		local rows = std.pii.maskRows({ { cpf = "123.456.789-01", nome = "Ava" } })
		std.result.ok({ has = std.pii.has(s), masked = masked, rows = rows })
		~~~`,
			"limits": `# Limits

- Execution: up to 30s.
- Output: 256 KiB (truncated).
- File: 2 MB per file; 1 MB per write.
- Paths are confined to the sandbox (no absolute paths, no ~..~).
- ~std.sql.query~ returns at most 10,000 rows (~SANDBOX_SQL_MAX_ROWS~).
- Secrets (~SECRET_*~) and PII (CPF, CNPJ, email, phone, card, JWT/BTC) are auto-masked in any output/return.`,
	"env": `# Environment

Secrets are available via ~std.secrets.get~ using ~SECRET_*~ variables (e.g., ~SECRET_GITHUB_TOKEN_API~ → ~std.secrets.get("github_token_api")~).

Other ~SANDBOX_*~ variables configure the sandbox (folders, limits, network) and are set by the operator — scripts don't need to read them.`,
}
