# SSH MCP

Executa comandos e move arquivos em hosts remotos. Credenciais vêm só do ambiente.

## Conexões

Cada conexão é um conjunto de variáveis com alias: `STAGING_SSH_HOST` ou `SSH_STAGING_HOST`. Sem alias não há conexão. Basta um `HOST` para ela existir.

| Variável | Padrão | Descrição |
| --- | --- | --- |
| `HOST` | — | host ou IP (obrigatório) |
| `USER` | `USER`/`USERNAME` | usuário SSH |
| `PORT` | `22` | porta |
| `KEY_PATH` / `PASSWORD` | — | chave privada ou senha |
| `KEY_PASSPHRASE` | — | passphrase da chave |
| `KNOWN_HOSTS` | `~/.ssh/known_hosts` | host keys |
| `TIMEOUT_SECONDS` | `30` | timeout de conexão e de cada operação |
| `ALLOW` | — | allowlist do `run`, ex. `cat,ls,systemctl` |

```env
LAB_SSH_HOST=203.0.113.10
LAB_SSH_USER=deploy
LAB_SSH_KEY_PATH=C:/Users/yannp/.ssh/id_ed25519

STAGING_SSH_HOST=staging.exemplo.com
STAGING_SSH_KEY_PATH=C:/Users/yannp/.ssh/staging
STAGING_SSH_PORT=2222
```

Aqui as conexões são `lab` e `staging`.

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

## Limites

- **Denylist sempre ativa** no `run` (~38 regras): formatar ou apagar disco (`mkfs`, `wipefs`, `fdisk`, `parted`, `dd`, `shred`, `cryptsetup`, LVM/ZFS/btrfs), desligar/reiniciar, `rm -rf /` ou `~`, `chmod`/`chown` recursivo na raiz, escrita em `/dev`, `/proc`, `/sys` e arquivos críticos de `/etc`, derrubar rede ou `sshd`, apagar logs/histórico, `crontab -r`, `docker system prune -a`, `kubectl delete ns`, pipe de download para shell e fork bomb.
- Regras casam pelo binário: `fdisk -l` também é bloqueado.
- **Allowlist opcional** por conexão: `STAGING_SSH_ALLOW=cat,ls,systemctl`. Quando definida, cada segmento do comando (`;`, `&&`, `|`) precisa começar por um dos nomes.
- **Paths protegidos** em `write` e `remove`: `/etc`, `/boot`, `/usr`, `/bin`, `/sbin`, `/lib`, `/lib64`, `/sys`, `/proc`, `/dev`, `/root` e qualquer `.ssh`.

## Notas

- `target` pode ser omitido quando só há uma conexão.
- `read` e `download` recusam `.env`, `*.env` e `environ`.
- `remove` recusa `/`, caminho vazio e curingas.
- `local` de upload/download é sempre relativo ao mnt compartilhado (`~/.local/state/mcp/mnt`).
- `run` é um shell remoto real; exit code diferente de zero vem no resultado.
- Saída truncada em 128 KB; transferências até 32 MiB.
