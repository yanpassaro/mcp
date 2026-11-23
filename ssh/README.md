# SSH MCP

Executa comandos e move arquivos em hosts remotos. Credenciais vêm só do ambiente.

## Conexões

Cada conexão é um conjunto `SSH_<ATRIBUTO>` (padrão) ou `SSH_<ALIAS>_<ATRIBUTO>` (nomeada). Basta um `HOST` para a conexão existir.

| Atributo | Padrão | Descrição |
| --- | --- | --- |
| `HOST` | — | host ou IP (obrigatório) |
| `USER` | `USER`/`USERNAME` | usuário SSH |
| `PORT` | `22` | porta |
| `KEY_PATH` / `PASSWORD` | — | chave privada ou senha |
| `KEY_PASSPHRASE` | — | passphrase da chave |
| `KNOWN_HOSTS` | `~/.ssh/known_hosts` | host keys |
| `TIMEOUT_SECONDS` | `30` | timeout de conexão e de cada operação |

```env
SSH_HOST=203.0.113.10
SSH_USER=deploy
SSH_KEY_PATH=C:/Users/yannp/.ssh/id_ed25519

SSH_STAGING_HOST=staging.exemplo.com
SSH_STAGING_KEY_PATH=C:/Users/yannp/.ssh/staging
SSH_STAGING_PORT=2222
```

Aqui as conexões são `default` e `staging`.

## `ssh_read`

| `action` | `data` |
| --- | --- |
| `probe` | — |
| `read` | `path`, `max` |
| `list` | `path` |
| `stat` | `path` |

`data.timeout` vale para `read`, `list` e `stat`.

```json
{ "action": "read", "data": { "path": "/var/log/api/error.log", "max": 65536 } }
```

## `ssh_act`

| `action` | `data` |
| --- | --- |
| `run` | `command`, `workdir` |
| `write` | `path`, `content`, `append` |
| `remove` | `path`, `recursive` |

`data.timeout` vale para todas.

```json
{ "target": "staging", "action": "run", "data": { "command": "systemctl status api --no-pager" } }
```

## `ssh_os`

| `action` | `data` |
| --- | --- |
| `upload` | `local`, `remote` |
| `download` | `local`, `remote` |

## Notas

- `target` pode ser omitido quando só há uma conexão.
- `read` e `download` recusam `.env`, `*.env` e `environ`.
- `remove` recusa `/`, caminho vazio e curingas.
- `local` de upload/download é sempre relativo ao mnt compartilhado (`~/.local/state/mcp/mnt`).
- `run` é um shell remoto real; exit code diferente de zero vem no resultado.
- Saída truncada em 128 KB; transferências até 32 MiB.
