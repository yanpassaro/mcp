# Shell MCP

Executa comandos locais com **allowlist** — autonomia pra IA sem virar terminal livre. Comandos em foreground (espera e devolve output) ou background (id + logs + kill).

## Env

| Variável | Padrão | Descrição |
| --- | --- | --- |
| `SHELL_ALLOW_ACT` | desligado | habilita `shell_act` (`1`, `true`, `yes`, `on`) |
| `SHELL_ALLOW_FILE` | — | caminho do txt de padrões permitidos (obrigatório pro `run`) |
| `SHELL_TIMEOUT_SECONDS` | `60` | timeout default do foreground |

## Allowlist

```text
# uma linha = padrão; termina em * = prefixo livre
php artisan serve
php artisan *
task *
go test *
cargo run *
docker compose *
```

- Linhas `#` são comentários; vazias são ignoradas.
- Casamento por linha inteira, insensível a maiúsculas: igualdade exata **ou** prefixo quando termina em `*`.
- Sem shell: o comando vira `argv` (`strings.Fields`) e roda via `exec.Command` — não há `&&`, `|`, `>`, `$()` nem expansão de variável.
- Comando sem padrão que case → erro claro apontando o `SHELL_ALLOW_FILE`.

## `shell_act`

| `action` | `data` |
| --- | --- |
| `run` | `command`, `cwd`, `env` (`KEY=VALUE`), `background`, `timeout` |
| `kill` | `id` |

```json
{ "action": "run", "data": { "command": "php artisan serve", "cwd": "C:/dev/api", "background": true, "env": ["APP_PORT=8000"] } }
{ "action": "run", "data": { "command": "go test ./...", "cwd": "C:/dev/api", "timeout": 120 } }
{ "action": "kill", "data": { "id": "p1" } }
```

- `background: true` inicia e devolve `p1`, `p2`…; logs via `shell_read`.
- Foreground espera até sair (ou timeout) e devolve output + exit code; timeout mata o processo.
- `kill` mata a árvore do processo (`taskkill /T /F` no Windows, `SIGKILL` no grupo de processo no Unix).

## `shell_read`

| `action` | `data` |
| --- | --- |
| `list` | — |
| `log` | `id`, `max` (bytes do fim; default `8192`) |
| `allow` | — |

## Notas

- O registry é **em memória**: se o MCP reiniciar, os processos continuam rodando, mas os ids/log se perdem.
- Buffer de log por processo: 512 KB (tail).
- Saída truncada em 128 KB por resposta.
- Requer o binário de cada comando no `PATH` do processo do MCP.
