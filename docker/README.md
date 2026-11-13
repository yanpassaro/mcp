# Docker MCP

Inspeciona o Docker local ou remoto e gerencia containers usando o SDK oficial do Moby (`github.com/moby/moby/client`). O `docker` CLI só é usado no `docker_run` quando você pede build (`buildContext`) ou compose (`compose`).

A conexão usa `client.FromEnv`, então respeita `DOCKER_HOST`, `DOCKER_API_VERSION`, `DOCKER_CERT_PATH` e `DOCKER_TLS_VERIFY` (Docker Desktop, Engine local/remoto e sockets `unix://`/`npipe://`).

São 4 tools: `docker_get`, `docker_inspect`, `docker_run` e `docker_control`.

Variáveis de ambiente **não são expostas**: `docker_inspect` remove recursivamente qualquer chave `Env`, e `docker_run` não aceita `env`.

## Tools

| Tool | O que faz |
| --- | --- |
| `docker_get` | Lista `containers`, `images`, `volumes`, `networks`, `info`, `stats` ou `disk`; aceita `filter` e `limit` |
| `docker_inspect` | JSON de container, imagem, volume ou network; `Env` do container é removido |
| `docker_run` | Sobe algo: container detached de uma imagem, imagem construída de um Dockerfile (`buildContext`) ou projeto compose (`compose`); aceita nome, command, portas, volumes, network, `autoRemove`, política `pull`, `platform` e `timeoutSeconds` |
| `docker_control` | Start/stop/restart/pause/unpause/kill, `logs`, `exec` e remove de container, imagem, volume ou network |

## Inspect

`docker_inspect` aceita `resource`:

- `container` (padrão): remove `Env` recursivamente.
- `image`, `volume`, `network`: JSON completo.

Exemplos:

```json
{ "resource": "image", "target": "nginx:latest" }
```

```json
{ "resource": "volume", "target": "nginx-html" }
```

```json
{ "resource": "network", "target": "app-net" }
```

```json
{ "container": "web" }
```

`target` é o nome/ID; `container` continua aceito como alias antigo.

## Filtros e limite

`docker_get` aceita `filter` no formato `chave=valor`, separado por vírgula, e `limit`:

```json
{ "resource": "containers", "filter": "status=running,name=api", "limit": 50 }
```

```json
{ "resource": "images", "filter": "dangling=true", "limit": 100 }
```

```json
{ "resource": "volumes", "filter": "label=com.docker.compose.project=app" }
```

`limit: 0` (padrão) mostra tudo. Filtros com vírgula dentro do valor não são suportados nesse formato.

## Variáveis de ambiente

| Variável | Padrão | Descrição |
| --- | --- | --- |
| `DOCKER_TIMEOUT_SECONDS` | `60` | Timeout de cada operação |
| `DOCKER_HOST` | — | Endpoint do daemon (`unix://`, `npipe://`, `tcp://` ou `ssh://`) |
| `DOCKER_API_VERSION` | negociação automática | Fixa a versão da API |
| `DOCKER_TLS_VERIFY` | — | Ativa verificação TLS |
| `DOCKER_CERT_PATH` | — | Pasta dos certificados TLS |
| `DOCKER_REGISTRY_AUTH` | — | Auth base64 para pull de imagem privada |
| `DOCKER_SSH_KEY_PATH` | `~/.ssh/id_ed25519` (ou `id_rsa`, `id_ecdsa`) | Chave para `DOCKER_HOST=ssh://` |
| `DOCKER_SSH_KEY_PASSPHRASE` | — | Passphrase da chave |
| `DOCKER_SSH_PASSWORD` | — | Senha SSH (alternativa à chave) |
| `DOCKER_SSH_KNOWN_HOSTS` | `~/.ssh/known_hosts` | Host keys pinadas |

`DOCKER_CONTEXT` não é lido pelo SDK; para usar um contexto, exporte o `DOCKER_HOST` correspondente.

Logs ficam em `~/.local/state/mcp/logs/docker-<data>.log`.

## Daemon remoto por SSH

`DOCKER_HOST=ssh://usuario@host[:porta][/caminho/do/docker.sock]` é suportado nativamente (o SDK não implementa esse esquema, então o servidor abre o túnel com `golang.org/x/crypto/ssh` e fala com o socket remoto, como o CLI faz).

```env
DOCKER_HOST=ssh://ubuntu@203.0.113.10
DOCKER_SSH_KEY_PATH=C:/Users/yannp/.ssh/ubuntu
DOCKER_SSH_KEY_PASSPHRASE=<passphrase>
# opcional
DOCKER_SSH_KNOWN_HOSTS=C:/Users/yannp/.ssh/known_hosts
```

- Chave: `DOCKER_SSH_KEY_PATH`; sem ela, tenta `~/.ssh/id_ed25519`, `id_rsa` e `id_ecdsa`.
- `DOCKER_SSH_PASSWORD` funciona como alternativa à chave.
- Host key é sempre verificada com `knownhosts`; erro sugere `ssh-keyscan -H <host> >> <arquivo>`.
- Socket padrão `/var/run/docker.sock`; um caminho na URL sobrescreve.
- A conexão é reaproveitada entre chamadas; cada requisição HTTP abre um canal SSH próprio.

Atenção: `compose` e `buildContext` do `docker_run` continuam usando o **CLI local**, que resolve `ssh://` do seu próprio jeito (agente/`~/.ssh/config`). Se a chave tiver passphrase e não houver agent, esse caminho pode falhar — as demais tools funcionam pelo túnel do servidor.

## Exemplo (Zed)

```json
{
  "context_servers": {
    "docker": { "command": "~/.local/bin/docker-mcp.exe" }
  }
}
```

## Dependências

O módulo usa o SDK do Moby. Depois de clonar/alterar o `go.mod`:

```bash
cd docker
go mod tidy
```

## Pull de imagens

`docker_run` tem a opção `pull`:

- `missing` (padrão): baixa só se a imagem não existir localmente.
- `always`: baixa sempre.
- `never`: não baixa; falha se não existir.

Exemplo:

```json
{
  "image": "nginx:latest",
  "name": "web",
  "ports": ["8080:80"],
  "pull": "missing",
  "timeoutSeconds": 300
}
```

`platform` permite escolher `linux/amd64`, `linux/arm64`, etc. Para registry privado, defina `DOCKER_REGISTRY_AUTH` com o auth base64 no ambiente do servidor; ele não é exposto ao modelo.

## Controlar containers

`docker_control` aceita `action`:

- `start`, `stop`, `restart`, `pause`, `unpause`, `kill`
- `logs` para ver stdout/stderr do container
- `exec` para rodar um comando dentro do container
- `remove` para container, imagem, volume ou network

Exemplos:

```json
{ "action": "restart", "target": "web", "timeoutSeconds": 30 }
```

```json
{ "action": "stop", "target": "web", "signal": "SIGTERM" }
```

```json
{ "action": "kill", "target": "web", "signal": "SIGKILL" }
```

```json
{ "action": "remove", "resource": "container", "target": "web", "force": true, "volumes": true }
```

```json
{ "action": "remove", "resource": "image", "target": "nginx:latest", "force": true }
```

```json
{ "action": "remove", "resource": "volume", "target": "nginx-html", "force": true }
```

```json
{ "action": "remove", "resource": "network", "target": "app-net" }
```

`timeoutSeconds` vale como timeout da operação e como timeout de parada em `stop`/`restart`. `signal` vale para `stop`, `restart` e `kill`. O campo antigo `container` continua aceito como alias de `target`.

## Logs

`docker_control` com `action: "logs"` devolve stdout e stderr do container, com `tail` (padrão 200, máximo 10000), `since`/`until` e `timestamps` opcionais.

```json
{ "action": "logs", "target": "web", "tail": 200 }
```

```json
{ "action": "logs", "target": "web", "since": "10m", "timestamps": true }
```

## Exec

`docker_control` com `action: "exec"` roda um comando dentro do container em execução e devolve `exit code`, stdout e stderr.

```json
{ "action": "exec", "target": "web", "command": ["sh", "-lc", "nginx -v"] }
```

```json
{ "action": "exec", "target": "web", "command": ["ls", "-la"], "workdir": "/app", "user": "1000:1000", "timeoutSeconds": 30 }
```

O `command` é enviado como argv (`["sh", "-lc", "..."]`), sem shell implícito. `user` e `workdir` são opcionais. Exit code diferente de zero não é erro da tool: vem no resultado para o modelo decidir.

Saída de stdout e stderr é truncada em 128 KB. `exec` roda como o usuário do container (root por padrão); use com cuidado em daemons compartilhados.

## Compose e build

Os dois entram pelo `docker_run`, que usa o binário `docker` (precisa estar no `PATH`) nesses dois casos.

**Compose** — informe `compose` com o caminho do arquivo; `image` é ignorada:

```json
{ "compose": "C:/dev/app/compose.yaml" }
```

```json
{ "compose": "C:/dev/app/compose.yaml", "project": "app", "build": true }
```

```json
{ "compose": "C:/dev/app/compose.yaml", "services": ["api", "db"], "timeoutSeconds": 600 }
```

Roda `docker compose --ansi never -f <arquivo> [-p <projeto>] up -d [--build] [serviços...]` com `cmd.Dir` na pasta do arquivo, então caminhos relativos e bind mounts do compose continuam funcionando.

**Dockerfile** — informe `buildContext` com a pasta de contexto; a imagem é construída com a tag do `image` e usada na hora:

```json
{ "image": "api:local", "buildContext": "C:/dev/app", "ports": ["8080:3000"] }
```

```json
{ "image": "api:local", "buildContext": ".", "dockerfile": "docker/Dockerfile", "buildArgs": ["NODE_ENV=production"], "noCache": true }
```

`buildContext` também aceita `target` (estágio de build), `buildPull` (baixar bases novas) e `noCache`. Depois do build, o pull da imagem é pulado — o `pull` só vale para o caminho sem build.

Caminhos relativos de `compose` e `buildContext` são resolvidos a partir do diretório de trabalho do servidor MCP.

Quando `compose` ou `buildContext` estão preenchidos, o timeout padrão do `docker_run` passa de 60 s para **300 s** (`timeoutSeconds` sobrescreve).

Saída de compose e de build é truncada em 128 KB; em caso de erro, o stderr do CLI volta no próprio erro da tool.

## Limites e segurança

- `docker_get` e `docker_inspect` são read-only.
- `docker_run` e `docker_control` escrevem no daemon; não há `push` nem `prune`.
- Build e compose executam o `docker` CLI sem interpolar shell; o servidor não devolve `docker compose config` (que expõe env).
- `exec` equivale a acesso root dentro do container.
- `docker_inspect` remove `Env` em qualquer nível do JSON.
- `docker_run` não tem parâmetro `env`; variáveis só via imagem, compose, Dockerfile ou CLI externa.
- `docker_run` faz pull conforme `pull` (`missing` padrão); com `pull: "never"` a imagem precisa existir no daemon.
- Saídas de `docker_get` (disk), `docker_inspect`, `docker_control` (logs e exec) e `docker_run` (compose/build) são truncadas em 128 KB.
