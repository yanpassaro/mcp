# sandbox-mcp

Sandbox **não-destrutivo** de scripts Lua para a IA: a IA escreve, lê, apaga e roda scripts isolados — sem SO/processo, arquivos confinados à pasta `mnt/`, rede apenas via `std.fetch` (allowlist).

Cada script tem `name`, `description` e uma função `function main(std)` que retorna `std.result.ok(...)`/`err(...)`. A saída vira Markdown (objetos/arrays viram JSON).

**Só o corpo (body) basta:** no `sandbox_run` com `code` inline, o wrapper `function main(std) ... end` é adicionado automaticamente quando ausente — a IA pode enviar apenas `print(...)` / `std.log.*` sem escrever a função. Scripts por `path` devem declarar `function main(std)`.

## Tools

| Tool | Action | O que faz |
| --- | --- | --- |
| `sandbox_run` | `run` (default) | Roda um script por `path` (.lua no host) ou `code` inline, com `args` (array/objeto → `std.args`) |
| `sandbox_doc` | `topic` (opcional) | Documentação da API `std` com assinaturas (args + retorno) por módulo; `topic=<módulo>` (ex.: `io`) traz as funções detalhadas + exemplo. `topic=run` tem os 2 modos com exemplos |
| `sandbox_os` | `copy`, `mount`, `del`, `stat`, `list` | Gerencia o filesystem do sandbox: copia (host→sandbox), monta (sandbox→host), apaga, mostra status ou lista (árvore) um caminho do sandbox |

`std` fornece: `result`, `log`, `args`, `io` (`read`/`lines`/`json`/`write`/`append`/`del`/`exists`/`stat`/`dir`/`copy`/`move`/`mkdir`/`glob`/`walk`), `tmp` (mesmas funções do `io` + `clear`), `sql` (`exec`/`query`/`get`/`scalar`/`close`/`begin`/`commit`/`rollback`/`tables`/`columns`/`schema`), `uuid` (`v4`/`v7`), `csv` (`parse`/`stringify`), `xml` (`parse`/`stringify`), `excel` (`sheets`/`read`/`write`), `data` (`fromCSV`/`toCSV`/`fromJSON`/`toJSON`/`fromJSONL`/`toJSONL`/`toXML`/`fromXML`/`fromExcel`/`toExcel`/`toSql`/`fromSql`/`sqlImport`/`sqlExport`/`convert`), `regex` (`match`/`find`/`findAll`/`replace`/`split`/`groups`/`findAllGroups`), `fake` (`seed`/`name`/`email`/`username`/`phone`/`int`/`float`/`bool`/`uuid`/`date`/`words`/`sentence`/`paragraph`), `random`, `date`, `str`, `list`, `num`, `json`, `assert` (`ok`/`equal`/`throws`/`type`/`notNil`/`number`/`string`/`boolean`/`table`/`contains`/`matches`/`between`/`length`), `fetch` (`get`/`post`/`json`), `encode`, `secrets` (`get`/`has`), `template` (`render`/`compile`/`loop`/`cond`).

**Segurança:** as libs nativas perigosas são removidas (`dofile`, `loadfile`, `load`, `require`, `collectgarbage`, `os`, `io`, `debug`, `package`, `coroutine`); só restam primitivas seguras (`pairs`, `ipairs`, `type`, `tostring`, `tonumber`, metatables, `string`, `math`, `table`).

**Limites:** execução 30s (timeout real — a chamada retorna em até 30s; como a lib Lua não interrompe um loop contínuo, esse cálculo em background fica contido pelo limite de 4 execuções simultâneas), saída 256 KiB, arquivo 2 MB, RAM do processo (padrão 512 MiB, soft limit do Go) e espaço em `mnt/` (padrão 256 MiB / 5.000 arquivos).

## Secrets

Secrets são expostos por variáveis de ambiente com o prefixo `SECRET_`. A IA só consegue **ler** (não há `set`):

```lua
-- SECRET_GITHUB_TOKEN_API
local token = std.secrets.get("github_token_api")
if token == nil then
    std.result.err("secret github_token_api não configurado")
end
```

A chave é o nome da variável sem o prefixo `SECRET_`, em minúsculas (`SECRET_GITHUB_TOKEN_API` → `github_token_api`), e é case-insensitive.

**Redação:** os valores de qualquer `SECRET_*` são automaticamente substituídos por `[REDACTED]` em toda saída (prints, `console`, retorno de `std.result`) — nunca aparecem para a IA.

## sandbox_os

Opera sobre a pasta `mnt/` do sandbox. `path` é sempre relativo ao sandbox (exceto a origem do `copy`, que é no host).

| action | args | o que faz |
| --- | --- | --- |
| `copy` | `path` = origem no host, `dest` = destino no sandbox | copia para dentro do sandbox |
| `mount` | `path` = origem no sandbox, `dest` = destino no host | copia para fora |
| `del` | `path` | apaga arquivo ou pasta (recursivo), dentro do sandbox |
| `stat` | `path` | status (existe, é pasta, tamanho, linhas) |
| `list` | `path` (opcional; raiz se vazio) | lista diretórios em árvore |

## Variáveis de ambiente

O filesystem do sandbox é fixo (sem env): `mnt` em `~/.local/state/mcp/mnt` (compartilhado com o sqlize) e `tmp` em `~/.local/state/mcp/tmp`.

| Variável | Padrão | Descrição |
| --- | --- | --- |
| `SANDBOX_TMP_SPACE_MB` | `64` | teto de espaço de `tmp/` |
| `SANDBOX_MNT_SPACE_MB` | `256` | teto de espaço de `mnt/` |
| `SANDBOX_MEM_LIMIT_MB` | `512` | teto de RAM do processo sandbox |
| `SANDBOX_FETCH_ALLOW_HOST` | `localhost,127.0.0.1,::1` | hosts permitidos para `std.fetch` (vírgulas; `.domínio` libera subdomínios) |
| `SANDBOX_FETCH_TIMEOUT_SECONDS` | `30` | timeout do `std.fetch` |
| `SANDBOX_FETCH_MAX_BODY_KB` | `1024` | teto do corpo da resposta do `std.fetch` |
| `SANDBOX_FETCH_COOKIE_FILE` | `~/.local/share/mcp/sandbox/cookies.json` | persistência de cookies do `std.fetch` |
