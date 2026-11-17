# SSH MCP

Executa comandos e move arquivos em hosts remotos via SSH usando `golang.org/x/crypto/ssh`. Não depende do `ssh.exe` nem do `scp`.

São 4 tools: `ssh_run`, `ssh_os`, `ssh_transfer` e `ssh_probe`.

Credenciais vêm **apenas** do ambiente do servidor: nenhuma tool aceita senha, chave ou passphrase, e nada disso aparece na saída.

## Conexões

Cada conexão é um conjunto de variáveis `SSH_<ATRIBUTO>` (conexão padrão) ou `SSH_<ALIAS>_<ATRIBUTO>` (conexão nomeada). Basta uma `HOST` para a conexão existir.

| Atributo | Obrigatório | Descrição |
| --- | --- | --- |
| `HOST` | sim | host ou IP |
| `USER` | não | usuário SSH; padrão vem de `USER`/`USERNAME` |
| `PORT` | não | porta; padrão `22` |
| `KEY_PATH` | um dos dois | caminho da chave privada (`~` é expandido) |
| `PASSWORD` | um dos dois | senha |
| `KEY_PASSPHRASE` | não | passphrase da chave |
| `KNOWN_HOSTS` | não | arquivo de host keys; padrão `~/.ssh/known_hosts` |
| `TIMEOUT_SECONDS` | não | timeout de conexão e de cada operação; padrão `30` |

Exemplo com três conexões:

```env
SSH_HOST=203.0.113.10
SSH_USER=deploy
SSH_KEY_PATH=C:/Users/yannp/.ssh/id_ed25519

SSH_STAGING_HOST=staging.exemplo.com
SSH_STAGING_USER=deploy
SSH_STAGING_KEY_PATH=C:/Users/yannp/.ssh/staging
SSH_STAGING_PORT=2222

SSH_DB_HOST=10.0.0.5
SSH_DB_USER=root
SSH_DB_PASSWORD=segredo
SSH_DB_TIMEOUT_SECONDS=60
```

Aqui as conexões são `default`, `staging` e `db`. O alias é o trecho do meio em minúsculas, com `_` no lugar de caracteres não alfanuméricos.

## Tools

| Tool | O que faz |
| --- | --- |
| `ssh_run` | Roda um comando de shell e devolve exit code, stdout e stderr |
| `ssh_os` | Arquivos remotos: `read`, `write`, `list`, `stat` e `remove` |
| `ssh_transfer` | `upload` do `mnt` compartilhado para o host, ou `download` do host para o `mnt` |
| `ssh_probe` | Kernel, uptime, disco e portas em escuta |

`target` escolhe a conexão. Com uma só conexão configurada, `target` pode ser omitido. Erro de alvo lista as conexões disponíveis.

## Exemplos

```json
{ "target": "staging", "command": "systemctl status api --no-pager" }
```

```json
{ "target": "db", "command": "docker ps --format '{{.Names}}\\t{{.Status}}'", "workdir": "/srv" }
```

```json
{ "target": "default", "action": "read", "path": "/var/log/api/error.log", "maxBytes": 65536 }
```

```json
{ "target": "default", "action": "write", "path": "/etc/myapp/feature.flag", "content": "on\n" }
```

```json
{ "target": "default", "action": "list", "path": "/etc/ssh" }
```

```json
{ "target": "default", "action": "stat", "path": "/var/lib/app.db" }
```

```json
{ "target": "default", "action": "remove", "path": "/tmp/build", "recursive": true }
```

```json
{ "target": "default", "direction": "download", "local": "logs/app.log", "remote": "/var/log/app.log" }
```

```json
{ "target": "default", "direction": "upload", "local": "deploy/nginx.conf", "remote": "/etc/nginx/nginx.conf" }
```

```json
{ "target": "staging" }
```

`command` é uma string enviada inteira ao shell remoto (sem array de argv). `workdir` é aplicado como `cd '<dir>' && <comando>`. Exit code diferente de zero não é erro da tool: vem no resultado.

## Arquivos remotos

`ssh_os` cobre o filesystem do host, com ações:

| `action` | Comando no host | Observação |
| --- | --- | --- |
| `read` | `head -c <maxBytes+1> -- '<path>'` | 256 KiB padrão, 32 MiB máx.; `.env`, `*.env` e `environ` bloqueados |
| `write` | `cat > '<path>'` ou `cat >> '<path>'` | conteúdo via stdin |
| `list` | `ls -lan -- '<path>'` | saída crua em bloco de código |
| `stat` | `stat -c '%n\|%s\|%F\|%a\|%U\|%G\|%y' -- '<path>'` | formato GNU (Linux) |
| `remove` | `rm -f -- '<path>'` ou `rm -rf` | recusa `/`, caminho vazio e curingas |

`ssh_transfer` usa o mesmo caminho de `cat`, com teto de 32 MiB:

- `local` é sempre **relativo** ao `mnt` compartilhado (`~/.local/state/mcp/mnt`); caminhos absolutos e `..` são recusados.
- `upload`: `mnt/<local>` → `remote`.
- `download`: `remote` → `mnt/<local>` (diretórios são criados).

## Segurança

- Chave e senha ficam só no ambiente; não há parâmetro de credencial em nenhuma tool.
- Host key é sempre verificada com `knownhosts`; não existe modo "ignorar". Se o host não estiver no arquivo, o erro sugere `ssh-keyscan -H <host> >> <arquivo>`.
- `ssh_os` com `read` e `download` recusam `.env`, `*.env` e arquivos `environ`; `remove` recusa `/`, caminho vazio e curingas.
- `ssh_run` é um shell remoto de verdade: quem tem a tool tem o acesso do usuário SSH. O bloqueio acima vale só para leitura/transferência.
- Saída das tools é truncada em 128 KB.
- `ssh_run` executa como o usuário SSH configurado; o servidor não interpola shell além de `cd` e do comando enviado.

## Linux

```bash
task build:ssh:linux
```

Ou, no diretório `ssh/`:

```bash
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o ../dist/ssh-mcp-linux-amd64 ./cmd/ssh-mcp
```

## Exemplo (Zed)

```json
{
  "context_servers": {
    "ssh": {
      "command": "~/.local/bin/ssh-mcp.exe",
      "env": {
        "SSH_HOST": "203.0.113.10",
        "SSH_USER": "deploy",
        "SSH_KEY_PATH": "C:/Users/yannp/.ssh/id_ed25519"
      }
    }
  }
}
```

## Dependências

```bash
cd ssh
go mod tidy
```

Logs ficam em `~/.local/state/mcp/logs/ssh-<data>.log`.
