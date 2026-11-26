# Shell MCP

Tools de comando para o agente local — **uma tool por comando**, definidas em JSON (`SHELL_TOOLS_FILE`). O agente não monta a linha de comando: só preenche os campos do schema. Foreground (espera e devolve output) ou background (id + logs + kill).

## Env

| Variável | Padrão | Descrição |
| --- | --- | --- |
| `SHELL_ALLOW_ACT` | desligado | habilita as tools `shell_<name>` e `shell_kill` (`1`, `true`, `yes`, `on`) |
| `SHELL_TOOLS_FILE` | — | JSON de tools por comando (ver abaixo) |
| `SHELL_TIMEOUT_SECONDS` | `60` | timeout default do foreground |

## Tools por comando (`SHELL_TOOLS_FILE`)

```json
[
  { "name": "test",    "desc": "Testa o pacote Go atual", "cmd": "go test ${args}" },
  { "name": "artisan", "desc": "Artisan do Laravel",      "cmd": "php artisan ${action} ${args?}" },
  { "name": "serve",   "desc": "Sobe o servidor Laravel", "cmd": "php artisan serve", "background": true }
]
```

- `${x}` obrigatório, `${x?}` opcional (omitido do argv se vazio).
- Cada item vira a tool `shell_<name>` com a descrição e um campo string por placeholder.
- `cwd` é placeholder especial: vira diretório de trabalho (não vai pro argv).
- `background: true` inicia e devolve o id; logs via `shell_read`, stop via `shell_kill`.
- Executável é fixo no template (não pode começar por placeholder); sem shell — só argv.
- JSON inválido, name duplicado ou fora de `^[a-z][a-z0-9_-]{0,31}$` recusam o boot.

## Tools

| Tool | input |
| --- | --- |
| `shell_<name>` | um campo string por placeholder do `cmd` (+ `cwd` se houver) |
| `shell_kill` | `id` |

## `shell_read`

| `action` | `data` |
| --- | --- |
| `list` | — |
| `log` | `id`, `max` (bytes do fim; default `8192`) |
| `tools` | — |

## Notas

- O registry é **em memória**: se o MCP reiniciar, os processos continuam rodando, mas os ids/log se perdem.
- Buffer de log por processo: 512 KB (tail).
- Saída truncada em 128 KB por resposta.
- Requer o binário de cada comando no `PATH` do processo do MCP.
