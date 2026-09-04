# sandbox-mcp

Sandbox **não-destrutivo** de scripts Lua para a IA: a IA escreve, lê, apaga e roda scripts isolados — sem SO/processo, arquivos confinados à pasta `fs/`, rede apenas via `std.fetch` (allowlist).

Cada script tem `name`, `description` e uma função `function main(std)` que retorna `std.result.ok(...)`/`err(...)`. A saída vira Markdown (objetos/arrays viram JSON).

## Tools

| Tool | O que faz |
| --- | --- |
| `sandbox_read` | Lê um script por `name`; sem `name`, lista todos |
| `sandbox_write` | Cria/sobrescreve um script (`name` + `description` + `code`) |
| `sandbox_del` | Apaga um script por `name` |
| `sandbox_run` | Roda um script salvo, com `args` opcional |

`std` fornece: `result`, `log`, `args`, `fs` (`read`/`lines`/`json`/`write`/`append`/`del`/`exists`/`stat`/`dir`), `random`, `date`, `str`, `list`, `num`, `json`, `assert`, `fetch`, `encode`. Limites: execução 30s, saída 256 KiB, arquivo 2 MB.

## Variáveis de ambiente

| Variável | Padrão | Descrição |
| --- | --- | --- |
| `SANDBOX_FS_DIR` | `~/.local/share/mcp/sandbox/fs` | filesystem do script (leitura e escrita) |
| `SANDBOX_SCRIPTS_DIR` | `~/.local/share/mcp/sandbox/scripts` | scripts do agente |
| `SANDBOX_FETCH_ALLOW_HOST` | `localhost,127.0.0.1,::1` | hosts permitidos para `std.fetch` (vírgulas; `.domínio` libera subdomínios) |
| `SANDBOX_FETCH_TIMEOUT_SECONDS` | `30` | timeout do `std.fetch` |
| `SANDBOX_FETCH_MAX_BODY_KB` | `1024` | teto do corpo da resposta do `std.fetch` |
| `SANDBOX_FETCH_COOKIE_FILE` | `~/.local/share/mcp/sandbox/cookies.json` | persistência de cookies do `std.fetch` |
