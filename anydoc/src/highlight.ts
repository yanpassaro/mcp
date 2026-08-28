export type Run = { text?: string; color?: string };

const COMMON = new Set([
  "const",
  "let",
  "var",
  "function",
  "return",
  "if",
  "else",
  "elif",
  "for",
  "while",
  "do",
  "switch",
  "case",
  "break",
  "continue",
  "class",
  "new",
  "import",
  "from",
  "export",
  "default",
  "async",
  "await",
  "try",
  "catch",
  "finally",
  "throw",
  "null",
  "true",
  "false",
  "undefined",
  "None",
  "True",
  "False",
  "print",
  "fn",
  "func",
  "pub",
  "interface",
  "type",
  "enum",
  "struct",
  "impl",
  "use",
  "mod",
  "package",
  "yield",
  "lambda",
  "pass",
  "raise",
  "with",
  "this",
  "self",
  "super",
  "static",
  "public",
  "private",
  "protected",
  "def",
  "in",
  "of",
  "as",
  "is",
  "not",
  "and",
  "or",
]);

const SQL = new Set([
  "select",
  "insert",
  "update",
  "delete",
  "create",
  "drop",
  "alter",
  "table",
  "view",
  "index",
  "database",
  "from",
  "where",
  "and",
  "or",
  "not",
  "in",
  "is",
  "null",
  "like",
  "between",
  "join",
  "inner",
  "left",
  "right",
  "full",
  "outer",
  "on",
  "group",
  "by",
  "order",
  "having",
  "limit",
  "offset",
  "distinct",
  "as",
  "count",
  "sum",
  "avg",
  "min",
  "max",
  "case",
  "when",
  "then",
  "else",
  "end",
  "into",
  "values",
  "set",
  "primary",
  "key",
  "foreign",
  "references",
  "unique",
  "default",
  "constraint",
  "using",
  "union",
  "all",
  "exists",
  "with",
  "true",
  "false",
  "cast",
  "desc",
  "asc",
  "varchar",
  "integer",
  "int",
  "text",
  "boolean",
  "timestamp",
  "date",
  "numeric",
  "serial",
  "bigint",
]);

const LANGS: Record<string, Set<string>> = {
  sql: SQL,
  postgres: SQL,
  postgresql: SQL,
  mysql: SQL,
  mariadb: SQL,
  sqlite: SQL,
  psql: SQL,
  js: new Set([
    "var",
    "let",
    "const",
    "function",
    "return",
    "if",
    "else",
    "for",
    "while",
    "do",
    "switch",
    "case",
    "break",
    "continue",
    "new",
    "class",
    "extends",
    "super",
    "this",
    "import",
    "export",
    "default",
    "async",
    "await",
    "try",
    "catch",
    "finally",
    "throw",
    "typeof",
    "instanceof",
    "void",
    "delete",
    "in",
    "of",
    "as",
    "yield",
    "interface",
    "type",
    "enum",
    "declare",
    "namespace",
    "public",
    "private",
    "protected",
    "readonly",
    "static",
    "get",
    "set",
    "abstract",
  ]),
  ts: new Set([
    "var",
    "let",
    "const",
    "function",
    "return",
    "if",
    "else",
    "for",
    "while",
    "do",
    "switch",
    "case",
    "break",
    "continue",
    "new",
    "class",
    "extends",
    "super",
    "this",
    "import",
    "export",
    "default",
    "async",
    "await",
    "try",
    "catch",
    "finally",
    "throw",
    "typeof",
    "instanceof",
    "void",
    "delete",
    "in",
    "of",
    "as",
    "yield",
    "interface",
    "type",
    "enum",
    "declare",
    "namespace",
    "public",
    "private",
    "protected",
    "readonly",
    "static",
    "get",
    "set",
    "abstract",
    "implements",
    "keyof",
    "infer",
    "never",
    "unknown",
    "any",
  ]),
  python: new Set([
    "def",
    "class",
    "return",
    "if",
    "elif",
    "else",
    "for",
    "while",
    "break",
    "continue",
    "import",
    "from",
    "as",
    "try",
    "except",
    "finally",
    "raise",
    "with",
    "lambda",
    "pass",
    "yield",
    "global",
    "nonlocal",
    "assert",
    "del",
    "in",
    "is",
    "not",
    "and",
    "or",
    "None",
    "True",
    "False",
    "async",
    "await",
    "self",
  ]),
  bash: new Set([
    "if",
    "then",
    "else",
    "elif",
    "fi",
    "for",
    "while",
    "until",
    "do",
    "done",
    "case",
    "esac",
    "function",
    "return",
    "export",
    "local",
    "in",
    "select",
    "declare",
    "source",
    "echo",
    "cd",
    "ls",
    "cat",
    "sudo",
    "apt",
    "docker",
    "compose",
    "curl",
    "sh",
    "run",
    "exit",
    "set",
    "mkdir",
    "rm",
    "cp",
    "mv",
    "grep",
    "chmod",
    "chown",
    "systemctl",
    "apt-get",
    "wget",
    "break",
    "continue",
    "shift",
    "getopts",
    "eval",
    "exec",
    "trap",
    "unset",
    "readonly",
    "typeset",
    "alias",
    "unalias",
    "command",
    "type",
    "builtin",
    "hash",
    "history",
    "jobs",
    "bg",
    "fg",
    "wait",
    "kill",
    "disown",
    "pushd",
    "popd",
    "dirs",
    "pwd",
    "test",
    "true",
    "false",
    "printf",
    "read",
    "source",
    "shopt",
    "set",
    "unset",
    "tar",
    "gzip",
    "gunzip",
    "zip",
    "unzip",
    "find",
    "sed",
    "awk",
    "cut",
    "sort",
    "uniq",
    "head",
    "tail",
    "tr",
    "wc",
    "xargs",
    "tee",
    "diff",
    "less",
    "more",
    "ps",
    "top",
    "killall",
    "pkill",
    "ssh",
    "scp",
    "sftp",
    "rsync",
    "ip",
    "ping",
    "ss",
    "netstat",
    "nslookup",
    "dig",
    "hostname",
    "uname",
    "whoami",
    "id",
    "env",
    "printenv",
    "which",
    "whereis",
    "date",
    "sleep",
    "timeout",
    "mount",
    "umount",
    "df",
    "du",
    "ln",
    "touch",
    "file",
    "stat",
    "passwd",
    "useradd",
    "usermod",
    "userdel",
    "groupadd",
    "groupdel",
    "crontab",
    "service",
    "journalctl",
    "git",
    "make",
    "gcc",
    "python",
    "python3",
    "node",
    "npm",
    "pip",
    "perl",
    "ruby",
    "php",
    "aptitude",
    "dpkg",
    "snap",
    "yum",
    "dnf",
    "pacman",
    "podman",
    "kubectl",
    "helm",
    "docker-compose",
  ]),
  powershell: new Set([
    "if", "else", "elseif",
    "for", "foreach", "while", "do",
    "switch", "case", "default",
    "function", "return", "param",
    "begin", "process", "end",
    "try", "catch", "finally", "throw",
    "import", "export", "from",
    "class", "enum", "using",
    "var", "let", "const",
    "write", "echo", "exit",
    "New-Item", "Set-Item", "Get-Item", "Remove-Item",
    "Copy-Item", "Move-Item", "Invoke-Command",
    "Write-Host", "Write-Output", "Write-Error",
    "Select-Object", "Where-Object", "ForEach-Object",
    "Sort-Object", "Group-Object", "Measure-Object",
    "Get-Process", "Get-Service", "Start-Service", "Stop-Service",
    "Get-ChildItem", "Set-Location", "Get-Location",
    "Test-Path", "Resolve-Path", "Push-Location", "Pop-Location",
    "Set-ExecutionPolicy", "Get-Help", "Get-Command",
    "Invoke-RestMethod", "Invoke-WebRequest",
    "ConvertTo-Json", "ConvertFrom-Json",
    "ConvertTo-SecureString", "ConvertFrom-SecureString",
    "Compress-Archive", "Expand-Archive",
    "Get-Credential", "Get-Random",
    "Start-Job", "Receive-Job", "Remove-Job",
    "Out-Null", "Out-File", "Out-Printer",
    "Tee-Object", "Format-Table", "Format-List",
    "cd", "ls", "dir", "mkdir", "rm", "cp", "mv",
    "cat", "type", "more", "less",
  ]),
};

// ---------------------------------------------------------------------------
// Data formats get dedicated per-line highlighters (JSON, YAML, TOML, XML).
// They are kept out of LANGS/COMMON because their grammar is not keyword
// driven: colors come from the role each token plays (key, value, tag...).
// ---------------------------------------------------------------------------

// Palette (One Dark, consistent with the generic highlighter):
//   comment 338B1A · string 98C379 · number D19A66 · keyword C678DD
//   key E5C07B · tag/attr 61AFEF · punctuation 7F848E

// ---- JSON ----
const JSON_RE =
  /("(?:[^"\\]|\\.)*")(\s*:)|("(?:[^"\\]|\\.)*")|(\b(?:true|false|null)\b)|([+-]?(?:\d+(?:\.\d+)?|\.\d+)(?:[eE][+-]?\d+)?)|([{}\[\],:])/g;

function highlightJson(line: string): Run[] {
  const out: Run[] = [];
  let last = 0;
  let m: RegExpExecArray | null;
  while ((m = JSON_RE.exec(line)) !== null) {
    if (m.index > last) out.push({ text: line.slice(last, m.index) });
    if (m[1] !== undefined) {
      // "key" + ":"
      out.push({ text: m[1], color: "E5C07B" });
      out.push({ text: m[2], color: "7F848E" });
    } else if (m[3] !== undefined) {
      out.push({ text: m[3], color: "98C379" });
    } else if (m[4] !== undefined) {
      out.push({ text: m[4], color: "C678DD" });
    } else if (m[5] !== undefined) {
      out.push({ text: m[5], color: "D19A66" });
    } else {
      out.push({ text: m[6], color: "7F848E" });
    }
    last = JSON_RE.lastIndex;
  }
  if (last < line.length) out.push({ text: line.slice(last) });
  return out.length ? out : [{ text: line }];
}

// ---- YAML / YML ----
const YAML_VALUE_RE =
  /("(?:[^"\\]|\\.)*"|'(?:[^']|'')*')|(&[A-Za-z0-9_-]+)|(\*[A-Za-z0-9_-]+)|(!{1,2}[A-Za-z0-9_./-]+)|(\b(?:[Tt]rue|[Ff]alse|[Yy]es|[Nn]o|[Oo]n|[Oo]ff|[Nn]ull|[Nn]one|~)\b)|(\d{4}-\d{2}-\d{2}(?:[Tt ]\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:[Zz]|[+-]\d{2}:\d{2})?)?)|([+-]?(?:\d+(?:\.\d+)?|\.\d+)(?:[eE][+-]?\d+)?)|(\|[+-]?\d*)|(>[+-]?\d*)/g;

function highlightYaml(line: string): Run[] {
  // Trailing comment first (outside quotes), so "#" never leaks into tokens.
  let cut = -1;
  let inS = false, inD = false;
  for (let i = 0; i < line.length; i++) {
    const c = line[i];
    if (c === "\\" && (inS || inD)) { i++; continue; }
    if (c === "'" && !inD) { inS = !inS; continue; }
    if (c === '"' && !inS) { inD = !inD; continue; }
    if (c === "#" && !inS && !inD) { cut = i; break; }
  }
  const code = cut < 0 ? line : line.slice(0, cut);
  const comment = cut < 0 ? "" : line.slice(cut);
  const out: Run[] = [];
  const push = (text: string, color?: string) => {
    if (text) out.push(color ? { text, color } : { text });
  };

  const ws = /^\s*/.exec(code)?.[0] ?? "";
  push(ws);
  let rest = code.slice(ws.length);

  // "- item" → bullet
  const dash = /^(-)(\s+)/.exec(rest);
  if (dash) {
    push("-", "C678DD");
    push(dash[2]);
    rest = rest.slice(dash[0].length);
  }

  // leading anchor / alias ("&a", "*a")
  const ref = /^([&*][A-Za-z0-9_-]+)(\s*)/.exec(rest);
  if (ref) {
    push(ref[1], "61AFEF");
    push(ref[2]);
    rest = rest.slice(ref[0].length);
  }

  // "key: value"
  const key = /^([A-Za-z_][A-Za-z0-9_.-]*|"(?:[^"\\]|\\.)*"|'(?:[^']|'')*')(\s*)(:)(?=\s|$)/.exec(rest);
  if (key) {
    push(key[1], "E5C07B");
    push(key[2]);
    push(":", "7F848E");
    rest = rest.slice(key[0].length);
  }

  // Remaining value tokens
  YAML_VALUE_RE.lastIndex = 0;
  let last = 0;
  let m: RegExpExecArray | null;
  while ((m = YAML_VALUE_RE.exec(rest)) !== null) {
    if (m.index > last) push(rest.slice(last, m.index));
    if (m[1] !== undefined) push(m[1], "98C379");
    else if (m[2] !== undefined || m[3] !== undefined || m[4] !== undefined) push(m[0], "61AFEF");
    else if (m[5] !== undefined) push(m[5], "C678DD");
    else if (m[6] !== undefined || m[7] !== undefined) push(m[0], "D19A66");
    else push(m[0], "C678DD");
    last = YAML_VALUE_RE.lastIndex;
  }
  if (last < rest.length) push(rest.slice(last));
  if (comment) push(comment, "338B1A");
  return out.length ? out : [{ text: line }];
}

// ---- TOML ----
const TOML_RE =
  /(\[\[[^\[\]]*\]\]|\[[^\[\]]*\])|([A-Za-z0-9_.-]+)(\s*)(=)|("(?:[^"\\]|\\.)*"|'(?:[^']|'')*')|(\b(?:true|false)\b)|(\d{4}-\d{2}-\d{2}(?:[Tt ]\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:[Zz]|[+-]\d{2}:\d{2})?)?)|([+-]?(?:\d+(?:_\d+)*(?:\.\d+(?:_\d+)*)?(?:[eE][+-]?\d+)?|0x[0-9A-Fa-f_]+|0o[0-7_]+|0b[01_]+))|(^|\s)(#.*)/g;

function highlightToml(line: string): Run[] {
  const out: Run[] = [];
  let last = 0;
  let m: RegExpExecArray | null;
  while ((m = TOML_RE.exec(line)) !== null) {
    if (m.index > last) out.push({ text: line.slice(last, m.index) });
    if (m[1] !== undefined) {
      // [section] / [[array-of-tables]]
      out.push({ text: m[1], color: "61AFEF" });
    } else if (m[2] !== undefined) {
      out.push({ text: m[2], color: "E5C07B" });
      out.push({ text: m[3] });
      out.push({ text: m[4], color: "7F848E" });
    } else if (m[5] !== undefined) {
      out.push({ text: m[5], color: "98C379" });
    } else if (m[6] !== undefined) {
      out.push({ text: m[6], color: "C678DD" });
    } else if (m[7] !== undefined || m[8] !== undefined) {
      out.push({ text: m[0], color: "D19A66" });
    } else {
      // comment: group 9 is the leading whitespace (possibly empty)
      if (m[9]) out.push({ text: m[9] });
      out.push({ text: m[10], color: "338B1A" });
    }
    last = TOML_RE.lastIndex;
  }
  if (last < line.length) out.push({ text: line.slice(last) });
  return out.length ? out : [{ text: line }];
}

// ---- XML ----
const XML_RE =
  /(<!--[\s\S]*?-->)|(<!\[CDATA\[[\s\S]*?\]\]>)|(<\?[\s\S]*?\?>)|(<![A-Za-z][^>]*>)|(<\/?[A-Za-z][A-Za-z0-9:_-]*)|([A-Za-z_:][A-Za-z0-9_:.-]*)(\s*=\s*)("(?:[^"\\]|\\.)*"|'[^']*')|(>|\/>)/g;

function highlightXml(line: string): Run[] {
  const out: Run[] = [];
  let last = 0;
  let m: RegExpExecArray | null;
  while ((m = XML_RE.exec(line)) !== null) {
    if (m.index > last) out.push({ text: line.slice(last, m.index) });
    if (m[1] !== undefined) {
      out.push({ text: m[1], color: "338B1A" });
    } else if (m[2] !== undefined || m[3] !== undefined || m[4] !== undefined) {
      out.push({ text: m[0], color: "61AFEF" });
    } else if (m[5] !== undefined) {
      // <tag / </tag
      out.push({ text: m[5], color: "61AFEF" });
    } else if (m[6] !== undefined) {
      out.push({ text: m[6], color: "E5C07B" });
      out.push({ text: m[7], color: "7F848E" });
      out.push({ text: m[8], color: "98C379" });
    } else {
      out.push({ text: m[9], color: "7F848E" });
    }
    last = XML_RE.lastIndex;
  }
  if (last < line.length) out.push({ text: line.slice(last) });
  return out.length ? out : [{ text: line }];
}

// ---- NGINX / CRON ----
// Tokens compartilhados pelos argumentos de diretivas nginx e pelos comandos
// das linhas de cron: comentário, variável, string, número, pontuação de bloco.
const SEGMENT_RE =
  /(#[^\n]*)|(\$[A-Za-z0-9_]+)|("[^"]*"|'[^']*')|([+-]?(?:\d+(?:\.\d+)?(?:[a-z%]+)?))|([{};=])/g;

// Extrai cor de comentários/variáveis/strings/valores de um trecho livre.
function segmentRuns(text: string): Run[] {
  const out: Run[] = [];
  let last = 0;
  let m: RegExpExecArray | null;
  SEGMENT_RE.lastIndex = 0;
  while ((m = SEGMENT_RE.exec(text)) !== null) {
    if (m.index > last) out.push({ text: text.slice(last, m.index) });
    if (m[1] !== undefined) out.push({ text: m[1], color: "338B1A" });
    else if (m[2] !== undefined) out.push({ text: m[2], color: "E5C07B" });
    else if (m[3] !== undefined) out.push({ text: m[3], color: "98C379" });
    else if (m[4] !== undefined) out.push({ text: m[4], color: "D19A66" });
    else out.push({ text: m[0], color: "7F848E" });
    last = SEGMENT_RE.lastIndex;
  }
  if (last < text.length) out.push({ text: text.slice(last) });
  return out;
}

// Diretivas nginx que abrem um bloco { } — pintadas de azul, como "tags".
const NGINX_BLOCKS = new Set([
  "location", "server", "http", "events", "upstream", "if", "map",
  "geo", "stream", "mail", "types", "split_clients",
]);

function highlightNginx(line: string): Run[] {
  const out: Run[] = [];
  const push = (text: string, color?: string) => { if (text) out.push(color ? { text, color } : { text }); };
  const ws = /^\s*/.exec(line)?.[0] ?? "";
  push(ws);
  let rest = line.slice(ws.length);
  if (rest.startsWith("#")) {
    push(rest, "338B1A");
    return out;
  }
  // Diretiva = primeiro termo seguido de espaço ou `{` (ex.: "location = ...")
  const dw = /^([a-zA-Z_][a-zA-Z0-9_-]*)(?=[ \t{])/.exec(rest);
  if (dw) {
    push(dw[1], NGINX_BLOCKS.has(dw[1]) ? "61AFEF" : "C678DD");
    rest = rest.slice(dw[1].length);
  }
  out.push(...segmentRuns(rest));
  return out.length ? out : [{ text: line }];
}

// Campo de horário de cron: aceita apenas combinações permitidas do crontab
// (*, listas, intervalos, passos, ? e nomes de mês/dia), para não confundir o
// comando com os 5 campos.
const CRON_FIELD_RE =
  /^(?:\*|\?|@[A-Za-z]+|\d+|\d+-\d+|\*\/\d+|\d+-\d+\/\d+|\d+(?:,\d+)+|[A-Za-z]{3}(?:-[A-Za-z]{3})?)$/;

function highlightCron(line: string): Run[] {
  const out: Run[] = [];
  const push = (text: string, color?: string) => { if (text) out.push(color ? { text, color } : { text }); };
  const ws = /^\s*/.exec(line)?.[0] ?? "";
  push(ws);
  const rest = line.slice(ws.length);
  if (rest.startsWith("#")) {
    push(rest, "338B1A");
    return out;
  }
  // Macros (@daily, @reboot, @hourly...)
  const mac = /^(@[A-Za-z]+)(\s+)(.*)$/.exec(rest);
  if (mac) {
    push(mac[1], "C678DD");
    push(mac[2]);
    out.push(...segmentRuns(mac[3]));
    return out;
  }
  // 5 campos de horário + comando
  const fm =
    /^(\S+)(\s+)(\S+)(\s+)(\S+)(\s+)(\S+)(\s+)(\S+)(\s+)(.*)$/.exec(rest);
  if (fm && [1, 3, 5, 7, 9].every((i) => CRON_FIELD_RE.test(fm[i]))) {
    for (let i = 1; i <= 9; i += 2) {
      push(fm[i], "D19A66");
      push(fm[i + 1]);
    }
    out.push(...segmentRuns(fm[11]));
    return out;
  }
  // Linha sem agenda (ex.: continuação do comando)
  out.push(...segmentRuns(rest));
  return out.length ? out : [{ text: line }];
}

const DATA_HIGHLIGHTERS: Record<string, (line: string) => Run[]> = {
  json: highlightJson,
  yaml: highlightYaml,
  yml: highlightYaml,
  toml: highlightToml,
  xml: highlightXml,
  nginx: highlightNginx,
  cron: highlightCron,
};

function keywordsFor(lang?: string): Set<string> {
  if (!lang) return COMMON;
  const l = lang.toLowerCase();
  const extra = LANGS[l] ?? LANGS[l.replace(/[^a-z]/g, "")];
  if (!extra) return COMMON;
  return new Set([...COMMON, ...extra]);
}

function buildRegex(lang?: string): RegExp {
  const l = (lang ?? "").toLowerCase();
  // Support more comment styles
  const commentPatterns: string[] = [];
  if (["python", "py", "bash", "sh", "shell", "zsh", "yaml", "yml", "ruby", "rb", "toml", "ini", "dockerfile", "make", "powershell", "ps1"].some((p) => l === p || l.startsWith(p))) {
    commentPatterns.push("#[^\\n]*"); // hash comments
  }
  if (["javascript", "js", "typescript", "ts", "jsx", "tsx", "c", "cpp", "csharp", "cs", "java", "go", "rust", "php", "swift", "kotlin", "scala", "dart", "zig", "css", "scss", "less"].some((p) => l === p || l.startsWith(p))) {
    commentPatterns.push("//[^\\n]*"); // double-slash comments
  }
  if (["sql", "mariadb", "mysql", "postgresql", "postgres", "sqlite"].some((p) => l === p || l.startsWith(p))) {
    commentPatterns.push("--[^\\n]*"); // sql comments
  }
  // Default to // if no specific pattern matched
  if (commentPatterns.length === 0) commentPatterns.push("//[^\\n]*");
  const comment = commentPatterns.join("|");
  const src = [
    `(?<comment>${comment}|/\\*[\\s\\S]*?\\*/)`,
    "(?<string>\"[^\"]*\"|'[^']*'|`[^`]*`)",
    "(?<number>[0-9]+(?:\\.[0-9]+)?)",
    "(?<flag>-{1,2}[A-Za-z][A-Za-z0-9-]*)",
    "(?<word>[A-Za-z_$][A-Za-z0-9_$]*)",
    "(?<other>[^\\s])",
  ].join("|");
  return new RegExp(src, "g");
}

export function highlight(line: string, lang?: string): Run[] {
  // .env files: highlight only the variable name
  const l = (lang ?? "").toLowerCase();
  if (l === "env") {
    if (line.trimStart().startsWith("#")) return [{ text: line, color: "338B1A" }];
    const v = /^([A-Z][A-Z0-9_]*)(=.*)/.exec(line);
    if (v) return [{ text: v[1], color: "E5C07B" }, { text: v[2] }];
    return [{ text: line }];
  }
  // Other config files: no highlighting
  if (["ini", "properties", "cfg", "conf"].some((p) => l === p || l.endsWith("." + p))) {
    return [{ text: line }];
  }
  const data = DATA_HIGHLIGHTERS[l] ?? DATA_HIGHLIGHTERS[l.replace(/[^a-z]/g, "")];
  if (data) return data(line);
  const kw = keywordsFor(lang);
  const re = buildRegex(lang);
  const out: Run[] = [];
  let last = 0;
  let m: RegExpExecArray | null;

  while ((m = re.exec(line)) !== null) {
    if (m.index === 0 && last > 0) {
      // handled below
    }
    if (m.index > last) out.push({ text: line.slice(last, m.index) });

    const g = m.groups;
    if (g?.comment !== undefined) {
      out.push({ text: m[0], color: "338B1A" });
    } else if (g?.string !== undefined) {
      out.push({ text: m[0], color: "98C379" });
    } else if (g?.number !== undefined) {
      out.push({ text: m[0], color: "D19A66" });
    } else if (g?.flag !== undefined) {
      out.push({ text: m[0], color: "D19A66" });
    } else if (g?.word !== undefined) {
      const w = m[0];
      let color: string | undefined;
      if (kw.has(w) || kw.has(w.toLowerCase())) {
        color = "C678DD";
      } else if (w[0] === "$") {
        color = "E5C07B";
      } else {
        const rest = line.slice(m.index + w.length).replace(/^\s+/, "");
        if (rest[0] === "(") {
          color = "61AFEF";
        }
      }
      out.push({ text: w, color });
    } else {
      out.push({ text: m[0] });
    }

    last = re.lastIndex;
  }

  if (last < line.length) out.push({ text: line.slice(last) });
  if (out.length === 0) out.push({ text: line });
  return out;
}
