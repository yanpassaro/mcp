# sandbox-mcp

Sandbox **não-destrutivo** de Lua para a IA: rode scripts isolados — sem SO/processo, arquivos só em `mnt/`, rede só via `std.fetch` (allowlist).

## Quick start

O caminho mais curto — mande só o corpo, sem função:

```
sandbox_run
  action: run
  code: std.result.ok({ soma = 2 + 3 })
```

Saída:

```
{ "soma": 5 }
```

Todo script é `function main(std)` que termina com `std.result.ok(...)` ou `std.result.err(...)`. No `code` inline você pode mandar **só o corpo** — o wrapper `function main(std) ... end` é adicionado sozinho. Por `path`, basta um arquivo `.lua` no host.

Um script típico:

```
sandbox_run
  action: run
  code: |
    local args = std.args or {}
    print("oi", #args)
    std.result.ok({ n = #args })
  args: [1, 2, 3]
```

Opcional: `-- name=meu_script` e `-- desc=faz algo` no topo do script aparecem no cabeçalho da saída.

## As 3 tools

| Tool | O que faz |
| --- | --- |
| `sandbox_run` | Roda um script por `path` (`.lua` no host) ou `code` inline, com `args` opcional (vira `std.args`). Ação: `run` (default). |
| `sandbox_doc` | Documentação da API `std`. `sandbox_doc topic=<módulo>` traz assinaturas + retorno + exemplo (ex.: `io`, `run`, `fetch`, `limits`). |
| `sandbox_os` | Gerencia o filesystem: `copy` (host→sandbox), `mount` (sandbox→host), `del`, `stat`, `list`. |

## A API `std`

Cada módulo agrupa funções; detalhes em `sandbox_doc topic=<módulo>`.

| Módulo | O que tem |
| --- | --- |
| `result` | `ok(data)` / `err(msg)`; `render(template, vars)` → devolve Markdown renderizado |
| `log` | escreve na saída (`print()` também) |
| `args` | o valor `args` da execução (`std.args`) |
| `io` | arquivos em `mnt/`: `read`, `write`, `append`, `lines`, `json`, `del`, `exists`, `stat`, `dir`, `copy`, `move`, `mkdir`, `glob`, `walk`, `zip`, `unzip` |
| `tmp` | arquivos temporários (`tmp/`): mesmas funções do `io` + `clear` (limpa a cada execução) |
| `fetch` | HTTP: `get`, `post`, `json`, `save`, `request` (+ `fetch.cookies`) |
| `secrets` | lê `SECRET_*`: `get`, `has` (só leitura) |
| `sql` | SQLite: `connect(path)` → `exec`, `query`, `get`, `schema`… + `import`/`export` |
| `uuid` | `v4`, `v7`, `valid` |
| `csv` | `parse`, `stringify` |
| `xml` | `parse`, `stringify` |
| `excel` | `.xlsx`: `sheets`, `read`, `write` |
| `data` | pipeline CSV/JSON/XML/HTML/Excel/SQLite/SQL: `from`, `to`, `convert` |
| `regex` | Go RE2: `match`, `find`, `findAll`, `replace`, `split`, `groups`, `findAllGroups` |
| `json` | `parse`, `stringify`, `format`, `minify`, `path` |
| `encode` | `crc32`, `md5`, `sha256`, `base64`, `base32`, `hex` |
| `str` | `normalize`, `slug`, `title`, `camel`, `pascal`, `snake`, `kebab`, `wrap`, `summarize`, `format`, `count`, `split` |
| `list` | `chunk`, `groupBy`, `unique`, `flatten`, `sortBy`, `countBy`, `first`, `last`, `map`, `filter`, `reduce`, `find`, `some`, `every` |
| `num` | `round`, `clamp`, `percent`, `sum`, `avg`, `parse`, `fmt` |
| `date` | `now`, `iso`, `format`, `parse`, `add`, `unix`, `diff` |
| `random` | `seed`, `int`, `float`, `pick`, `shuffle`, `bool`, `name`, `email`, `username`, `phone`, `date`, `sentence`, `paragraph` |
| `assert` | `ok`, `equal`, `throws`, `type`, `number`, `string`, `boolean`, `table`, `contains`, `matches`, `between`, `length` |
| `template` | `render` — placeholders `{nome}`/`{{nome}}` + `{{#if}}`, `{{#unless}}`, `{{#each}}`, `{{else}}` |
| `human` | `bytes`, `duration`, `compact`, `money`, `ordinal`, `plural`, `list` (pt-BR) |

## Como o script termina

- `std.result.ok(data)` — sucesso; `data` (string, número, booleano ou tabela) vira a saída; objetos/arrays viram JSON.
- `std.result.render(template, vars)` — renderiza `template` (`{{x}}`, `{{#if}}`, `{{#each}}`) e devolve o resultado como Markdown (`.md`).
- `std.result.err(msg)` — erro; `msg` é a mensagem de erro.
- Sem `result`, o valor retornado por `main` é usado; `print()`/`std.log.*` viram a seção de saída.

## Segurança

As libs nativas perigosas são removidas: `dofile`, `loadfile`, `load`, `loadstring`, `require`, `collectgarbage`, `os`, `io`, `debug`, `package`, `coroutine`, `cjson`. Ficam primitivas seguras (`pairs`, `ipairs`, `type`, `tostring`, `tonumber`, metatables, `string`, `math`, `table`).

## Limites

- Execução: **180s** (3 min; timeout real, configurável via `SANDBOX_EXEC_TIMEOUT_SECONDS`).
- Saída e retorno: **256 KiB** (truncado com `... (truncado)`).
- Arquivo: **2 MB** por arquivo; **1 MB** por escrita.
- RAM do processo: **512 MiB** (padrão, soft limit do Go).
- Espaço: `mnt/` **256 MiB / 5.000 arquivos**; `tmp/` **64 MiB / 1.000 arquivos**.
- `std.sql.query` retorna no máximo **10.000 linhas** (`SANDBOX_SQL_MAX_ROWS`).
- Caminhos confinados ao sandbox: sem caminho absoluto, sem `..`.

## Segredos

Variáveis `SECRET_*` ficam disponíveis por `std.secrets.get`, sem o prefixo e em minúsculo: `SECRET_GITHUB_TOKEN_API` → `std.secrets.get("github_token_api")` (case-insensitive). Só leitura:

```lua
if std.secrets.has("github_token_api") then
  local token = std.secrets.get("github_token_api")
  std.result.ok({ ok = true })
end
```

O valor de qualquer `SECRET_*` é trocado por `[REDACTED]` em toda saída (prints, `std.log`, retorno) — nunca aparece para a IA.

## sandbox_os

Opera sobre `mnt/` do sandbox. `path` é relativo ao sandbox, exceto a origem do `copy` (host).

| action | args | O que faz |
| --- | --- | --- |
| `copy` | `path` (origem no host), `dest` (no sandbox) | copia para dentro do sandbox |
| `mount` | `path` (no sandbox), `dest` (no host) | copia para fora |
| `del` | `path` | apaga arquivo ou pasta (recursivo) |
| `stat` | `path` | status: existe, é pasta, tamanho, linhas |
| `list` | `path` (opcional; raiz se vazio) | lista diretórios em árvore |

## Variáveis de ambiente

O filesystem é fixo (sem env): `mnt` em `~/.local/state/mcp/mnt` (compartilhado com o sqlize) e `tmp` em `~/.local/state/mcp/tmp`.

| Variável | Padrão | Descrição |
| --- | --- | --- |
| `SANDBOX_TMP_SPACE_MB` | `64` | teto de espaço de `tmp/` |
| `SANDBOX_MNT_SPACE_MB` | `256` | teto de espaço de `mnt/` |
| `SANDBOX_MEM_LIMIT_MB` | `512` | teto de RAM do processo sandbox |
| `SANDBOX_EXEC_TIMEOUT_SECONDS` | `180` | timeout de execução de cada script |
| `SANDBOX_FETCH_ALLOW_HOST` | `localhost,127.0.0.1,::1` | hosts permitidos para `std.fetch` (vírgulas; `.domínio` libera subdomínios) |
| `SANDBOX_FETCH_TIMEOUT_SECONDS` | `30` | timeout do `std.fetch` |
| `SANDBOX_FETCH_MAX_BODY_KB` | `1024` | teto do corpo da resposta do `std.fetch` |
| `SANDBOX_FETCH_COOKIE_FILE` | `~/.local/share/mcp/sandbox/cookies.json` | persistência de cookies do `std.fetch` |
