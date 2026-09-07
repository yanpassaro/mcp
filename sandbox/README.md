# sandbox-mcp

Sandbox **não-destrutivo** de scripts Lua para a IA: a IA escreve, lê, apaga e roda scripts isolados — sem SO/processo, arquivos confinados à pasta `mnt/`, rede apenas via `std.fetch` (allowlist).

Cada script tem `name`, `description` e uma função `function main(std)` que retorna `std.result.ok(...)`/`err(...)`. A saída vira Markdown (objetos/arrays viram JSON).

## Tools

| Tool | O que faz |
| --- | --- |
| `sandbox_read` | Lê um script por `name`; sem `name`, lista todos |
| `sandbox_write` | Cria/sobrescreve um script (`name` + `description` + `code`) |
| `sandbox_del` | Apaga um script por `name` |
| `sandbox_run` | Roda um script salvo, com `args` opcional |
| `sandbox_manage` | Ações no filesystem do sandbox: `copy` (host→sandbox), `mount` (sandbox→host), `del`, `stat`, `list` (árvore) |
| `sandbox_doc` | Devolve a documentação da API `std` (topic opcional: `io`, `fetch`, `secrets`, ...) |

`std` fornece: `result`, `log`, `args`, `io` (`read`/`lines`/`json`/`write`/`append`/`del`/`exists`/`stat`/`dir`/`copy`/`move`/`mkdir`/`glob`/`walk`), `tmp` (mesmas funções do `io` + `clear`), `sql` (`exec`/`query`/`get`/`scalar`/`close`/`begin`/`commit`/`rollback`/`tables`/`columns`/`schema`), `uuid` (`v4`/`v7`), `csv` (`parse`/`stringify`), `xml` (`parse`/`stringify`), `excel` (`sheets`/`read`/`write`), `data` (`fromCSV`/`toCSV`/`fromJSON`/`toJSON`/`toXML`/`fromExcel`/`toExcel`/`sqlImport`/`sqlExport`/`convert`), `regex` (`match`/`find`/`findAll`/`replace`/`split`/`groups`/`findAllGroups`), `fake` (`seed`/`name`/`email`/`username`/`phone`/`int`/`float`/`bool`/`uuid`/`date`/`words`/`sentence`/`paragraph`), `pii` (`has`/`detect`/`mask`/`maskRows`), `random`, `date`, `str`, `list`, `num`, `json`, `assert` (`ok`/`equal`/`throws`/`type`/`notNil`/`number`/`string`/`boolean`/`table`/`contains`/`matches`/`between`/`length`), `fetch` (`get`/`post`/`json`), `encode`, `secrets` (`get`/`has`).

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

## sandbox_manage

Opera sobre a pasta `mnt/` do sandbox. `path` é sempre relativo ao sandbox (exceto a origem do `copy`, que é no host).

| action | args | o que faz |
| --- | --- | --- |
| `copy` | `path` = origem no host, `dest` = destino no sandbox | copia para dentro do sandbox |
| `mount` | `path` = origem no sandbox, `dest` = destino no host | copia para fora |
| `del` | `path` | apaga arquivo ou pasta (recursivo), dentro do sandbox |
| `stat` | `path` | status (existe, é pasta, tamanho, linhas) |
| `list` | `path` (opcional; raiz se vazio) | lista diretórios em árvore |

## Variáveis de ambiente

| Variável | Padrão | Descrição |
| --- | --- | --- |
| `SANDBOX_MNT_DIR` | `~/.local/share/mcp/sandbox/mnt` | filesystem do script (leitura e escrita) |
| `SANDBOX_DEV_DIR` | `~/.local/share/mcp/sandbox/dev` | scripts do agente |
| `SANDBOX_TMP_DIR` | `~/.local/share/mcp/sandbox/tmp` | pasta temporária do `std.tmp` |
| `SANDBOX_TMP_TTL_SECONDS` | `3600` | idade limite p/ limpar arquivos de `tmp/` na inicialização |
| `SANDBOX_TMP_SPACE_MB` | `64` | teto de espaço de `tmp/` |
| `SANDBOX_MNT_SPACE_MB` | `256` | teto de espaço de `mnt/` |
| `SANDBOX_MEM_LIMIT_MB` | `512` | teto de RAM do processo sandbox |
| `SANDBOX_FETCH_ALLOW_HOST` | `localhost,127.0.0.1,::1` | hosts permitidos para `std.fetch` (vírgulas; `.domínio` libera subdomínios) |
| `SANDBOX_FETCH_TIMEOUT_SECONDS` | `30` | timeout do `std.fetch` |
| `SANDBOX_FETCH_MAX_BODY_KB` | `1024` | teto do corpo da resposta do `std.fetch` |
| `SANDBOX_FETCH_COOKIE_FILE` | `~/.local/share/mcp/sandbox/cookies.json` | persistência de cookies do `std.fetch` |
