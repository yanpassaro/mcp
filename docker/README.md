# Docker MCP

Inspeciona o Docker local ou remoto e gerencia o ciclo de vida básico de containers usando o SDK oficial do Moby (`github.com/moby/moby/client`). Não depende do `docker` CLI.

A conexão usa `client.FromEnv`, então respeita `DOCKER_HOST`, `DOCKER_API_VERSION`, `DOCKER_CERT_PATH` e `DOCKER_TLS_VERIFY` (Docker Desktop, Engine local/remoto e sockets `unix://`/`npipe://`).

São 5 tools: `docker_get`, `docker_inspect`, `docker_logs`, `docker_run` e `docker_remove`.

Variáveis de ambiente **não são expostas**: `docker_inspect` remove recursivamente qualquer chave `Env`, e `docker_run` não aceita `env`.

## Tools

| Tool | O que faz |
| --- | --- |
| `docker_get` | Lista `containers`, `images`, `volumes`, `networks`, `info`, `stats` ou `disk` (read-only) |
| `docker_inspect` | JSON do inspect de um **container**, sem `Env` |
| `docker_logs` | Logs do container com `tail`, `since`/`until` e timestamps opcionais |
| `docker_run` | Cria e inicia um container detached; aceita nome, command, portas, volumes, network, `autoRemove`, política `pull`, `platform` e `timeoutSeconds` |
| `docker_remove` | Remove um container; opcional `force` e remoção de volumes anônimos |

## Variáveis de ambiente

| Variável | Padrão | Descrição |
| --- | --- | --- |
| `DOCKER_TIMEOUT_SECONDS` | `60` | Timeout de cada operação |
| `DOCKER_HOST` | — | Endpoint do daemon (`unix://`, `npipe://`, `tcp://`) |
| `DOCKER_API_VERSION` | negociação automática | Fixa a versão da API |
| `DOCKER_TLS_VERIFY` | — | Ativa verificação TLS |
| `DOCKER_CERT_PATH` | — | Pasta dos certificados TLS |
| `DOCKER_REGISTRY_AUTH` | — | Auth base64 para pull de imagem privada |

`DOCKER_CONTEXT` não é lido pelo SDK; para usar um contexto, exporte o `DOCKER_HOST` correspondente.

Logs ficam em `~/.local/state/mcp/logs/docker-<data>.log`.

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

- `missing` (padrão): baixa se a imagem não existir localmente.
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

## Limites e segurança

- `docker_get`, `docker_inspect` e `docker_logs` são read-only.
- `docker_run` e `docker_remove` escrevem no daemon; não há `exec`, `build`, `pull`, `push` nem `prune`.
- `docker_inspect` remove `Env` em qualquer nível do JSON.
- `docker_run` não tem parâmetro `env`; variáveis só via imagem ou CLI externa.
- `docker_run` não faz pull automático; a imagem precisa existir no daemon.
- Saídas de `docker_logs`, `docker_inspect` e `docker_get` (disk) são truncadas em 128 KB.
