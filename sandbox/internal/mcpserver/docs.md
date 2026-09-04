# Sandbox scripts

The sandbox runs **JavaScript** scripts in an isolated VM (goja). A script is a
small tool: it declares metadata and a `main(std)` function that returns
`std.return.ok(...)` or `std.return.err(...)`.

## Script format

```js
const name = "names.js";
const desc = "Generates names from the wordlists.";

export function main(std) {
  // your code here (the "body" that sandbox_write_script wraps for you)
  return std.return.ok(data);   // success
}
```

There is **no** network, process, filesystem access outside the sandbox, or OS
access. `main(std)` runs for up to 30s (fixed).

## The `std` API

### return
| | Description |
| --- | --- |
| `std.return.ok(data)` | Success. Object/array → Markdown; string → text. |
| `std.return.err(message)` | Failure (the tool marks `IsError`). |

### log
| | Description |
| --- | --- |
| `std.log.ok(...)` | Append a line to the output. |
| `std.log.err(...)` | Append a line to the output. |

`console.log` / `console.error` are aliases.

### args
| | Description |
| --- | --- |
| `std.args` | The `args` passed to the tool. If valid JSON it is parsed; otherwise a string. |

### fs (single sandbox filesystem — read/write)
| | Description |
| --- | --- |
| `std.fs.read(name)` | Content of a file. |
| `std.fs.lines(name)` | Non-empty lines (JS array). |
| `std.fs.json(name)` | Read a file and parse it as JSON. |
| `std.fs.write(name, content)` | Overwrite a file (returns bytes). |
| `std.fs.append(name, content)` | Append to a file (returns bytes). |
| `std.fs.del(name)` | Delete a file. |
| `std.fs.exists(name)` | `true`/`false`. |
| `std.fs.stat(name)` | `{name, exists, isDir, size, lines}`. |
| `std.fs.dir([path])` | List files (or a subfolder). |

### random
| | Description |
| --- | --- |
| `std.random.pick(arr)` | Random element of an array. |
| `std.random.shuffle(arr)` | A shuffled copy. |
| `std.random.int(min, max)` | Random integer in `[min, max]`. |
| `std.random.seed(v)` | Make `std.random` deterministic (and `Math.random` when the native lib is enabled). |

### date
| | Description |
| --- | --- |
| `std.date.now()` | Current timestamp (ms). |
| `std.date.iso([date])` | ISO 8601 string (default now). `date` may be ms, ISO, or `YYYY-MM-DD`. |
| `std.date.format("YYYY-MM-DD"[, date])` | Format a timestamp; tokens `YYYY`/`MM`/`DD`/`HH`/`mm`/`ss`. |
| `std.date.parse(str)` | Parse to timestamp (ms). |
| `std.date.add(date, amount, unit)` | Add `days`/`hours`/`months`/`years` (unit like `"day"`, `"month"`, `"hour"`). |
| `std.date.unix([date])` | Seconds since epoch (default now). |
| `std.date.diff(a, b, unit)` | `a - b` in `unit` (`"day"`, `"hour"`, `"minute"`, `"second"`, `"week"` ...). |

### str
| | Description |
| --- | --- |
| `std.str.normalize(s)` | Trim + strip accents. |
| `std.str.slug(s)` | `"João Silva" -> "joao-silva"` (kebab-case identifier). |
| `std.str.title(s)` | Title case each word. |
| `std.str.camel(s)` | `"hello_world" -> "helloWorld"`. |
| `std.str.pascal(s)` | `"hello_world" -> "HelloWorld"`. |
| `std.str.snake(s)` | `"Hello World" -> "hello_world"`. |
| `std.str.kebab(s)` | `"Hello World" -> "hello-world"`. |
| `std.str.wrap(s, n)` | Word-wrap to `n` columns (returns array of lines). |
| `std.str.summarize(s, max)` | Truncate to `max` chars with `...`. |
| `std.str.format(tpl, ctx)` | Replace `{{ key }}` in `tpl` using `ctx`. |
| `std.str.count(s, sub)` | Number of (non-overlapping) occurrences of `sub`. |
| `std.str.split(s, sep[, n])` | Split into array (`n` limits parts). |

### list
| | Description |
| --- | --- |
| `std.list.chunk(arr, n)` | Split into `n`-sized chunks. |
| `std.list.groupBy(arr, key\|fn)` | Object mapping key -> array of items. |
| `std.list.unique(arr)` | Dedupe keeping order. |
| `std.list.flatten(arr)` | Recursively flatten nested arrays. |
| `std.list.sortBy(arr, key\|fn)` | Stable sort by key (string property or function). |
| `std.list.countBy(arr, key\|fn)` | Object mapping key -> count. |
| `std.list.first(arr[, n])` | First element (or first `n` elements). |
| `std.list.last(arr[, n])` | Last element (or last `n` elements). |

### num
| | Description |
| --- | --- |
| `std.num.round(n, d)` | Round to `d` decimals. |
| `std.num.clamp(n, a, b)` | Clamp between `a` and `b`. |
| `std.num.percent(a, b)` | `a / b * 100` (`0` if `b == 0`). |
| `std.num.sum(arr)` | Sum of numbers. |
| `std.num.avg(arr)` | Average (`0` for empty). |
| `std.num.parse(s)` | Parse string to number (`NaN` on invalid). |
| `std.num.fmt(n[, loc][, dec])` | Formatted with thousand separators; `loc` `"pt-BR"` swaps separators; `dec` decimals (default 2). |

### encode
| | Description |
| --- | --- |
| `std.encode.crc32(s)` | CRC-32 checksum (hex). |
| `std.encode.md5(s)` | MD5 digest (hex). |
| `std.encode.sha256(s)` | SHA-256 digest (hex). |
| `std.encode.base64(s[, mode])` | Base64. `mode` (default `"encode"`): `"encode"` (standard), `"decode"`, `"url"`, `"urlDecode"`. |
| `std.encode.hex(s[, mode])` | Hex. `mode`: `"encode"` (default) / `"decode"`. |

### json
| | Description |
| --- | --- |
| `std.json.parse(s)` | Parse a JSON string into a value (replaces native `JSON.parse`). |
| `std.json.stringify(obj[, indent])` | Serialize a value to a JSON string (compact by default; `indent` spaces if > 0) — replaces native `JSON.stringify`. |
| `std.json.format(obj)` | Pretty-print (2-space indent). |
| `std.json.minify(s)` | Compact JSON string (parses then re-serializes). |
| `std.json.path(obj, "a.b.0.c")` | Traverse nested object/array by dotted path (missing -> `undefined`). |

For file I/O use `std.fs.json(name)` (read) and `std.fs.write(name, std.json.stringify(obj))`
(write) on the single sandbox filesystem, instead of the native `JSON` global.

### assert
| | Description |
| --- | --- |
| `std.assert.ok(v[, msg])` | Throws (fails the run) if `v` is falsy. |
| `std.assert.equal(a, b[, msg])` | Throws if `a` is not strictly `===` `b`. |
| `std.assert.throws(fn)` | Throws if `fn` does not throw. |

### fetch
| | Description |
| --- | --- |
| `std.fetch.request(url[, opts])` | HTTP request. `opts`: `method`, `headers`, `body`, `timeout` (ms), `noCookies`, `followRedirects`. Returns `{status, ok, headers, body, bytes, ms, truncated}`. |
| `std.fetch.get(url[, opts])` | Shorthand for `request` with `method: "GET"`. |
| `std.fetch.post(url[, body][, opts])` | Shorthand for `request` with `method: "POST"` (body may be a string or in opts). |
| `std.fetch.cookies.list()` | Cookies saved so far. |
| `std.fetch.cookies.clear([domain])` | Remove all (or one domain) cookies; returns count. |
| `std.fetch.cookies.set(domain, name, value[, opts])` | Set/change a cookie (`opts`: `path`, `secure`). |

`std.fetch` only reaches hosts in `SANDBOX_FETCH_ALLOW_HOST` (default
`localhost,127.0.0.1,::1`; a leading `.` allows subdomains). `Set-Cookie` headers
are saved and re-sent automatically, persisted to `SANDBOX_FETCH_COOKIE_FILE`.

The native JS built-ins (`Math`, `JSON`, `Array`, `Date`, ...) are **disabled by
default** — scripts see only `std` (and `console`). To re-enable them set
`SANDBOX_DISABLE_JS_STDLIB=0` (or `FullStdlib: true`). There is no `require`,
`process`, `fs` or `Buffer`; network is only available via `std.fetch`.

## Data folders

| Folder | Purpose |
| --- | --- |
| `fs/` | The single sandbox filesystem — read/write via `std.fs`. |
| `scripts/` | Your scripts — managed by the tools. |

All paths are confined: absolute paths and `..` are rejected.

## Tools

| Tool | What it does |
| --- | --- |
| `sandbox_write_script` | Write a script. Pass `name`, optional `description`, and `code` (the **body** of `main`). Wraps automatically. `delete: true` removes. |
| `sandbox_read_script` | List all scripts (no `name`) or read one (`name`). |
| `sandbox_run_script` | Run by `name` (saved) or inline `code`; pass `args` for params. |
| `sandbox_help` | This documentation. |

## Example

Write a script (only the function body):

```jsonc
// sandbox_write_script
{
  "name": "names.js",
  "description": "Generates names and saves them to fs/.",
  "code": "const a = std.fs.lines('first.txt');\nconst b = std.fs.lines('last.txt');\nconst nomes = [];\nfor (let i = 0; i < 5; i++)\n  nomes.push(std.random.pick(a) + ' ' + std.random.pick(b));\nstd.fs.write('nomes.txt', nomes.join('\\n'));\nreturn std.return.ok({ nomes, salvos: std.fs.dir() });"
}
```

Run it:

```jsonc
// sandbox_run_script
{ "name": "names.js" }
```

The result comes back as **Markdown**:

```text
## Script `names.js` · 1.2ms

**nomes:** Liam Silva, Maya Santos, Theo Costa, Zoe Almeida, Hugo Barbosa
**salvos:** nomes.txt
```

`std.return.ok(object)` renders as Markdown (key/value lines).
`std.return.err("...")` marks the tool as failed (`IsError`).

## Limits

| Resource | Value |
| --- | --- |
| Runtime | 30 s (fixed) |
| Output | 256 KiB |
| Read (data file) | 2 MB per file |
| Write (script/out) | 1 MiB |
