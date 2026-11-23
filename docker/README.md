# Docker MCP

Inspeciona e gerencia o Docker local ou remoto pelo SDK oficial (o CLI `docker` só entra em build e compose). A conexão vem de `DOCKER_HOST`, `DOCKER_API_VERSION`, `DOCKER_CERT_PATH` e `DOCKER_TLS_VERIFY`.

Toda chamada usa `action` e `data`. Valores de `env` nunca voltam nas respostas.

## `docker_read`

| `action` | `data` |
| --- | --- |
| `containers` | `filter`, `limit`, `last`, `runningOnly` |
| `images` | `filter`, `limit`, `all` |
| `volumes` | `filter`, `limit` |
| `networks` | `filter`, `limit` |
| `info` | — |
| `stats` | `name` |
| `disk` | `verbose` |
| `inspect` | `resource` (`container`/`image`/`volume`/`network`), `name` |
| `logs` | `name`, `tail`, `since`, `until`, `timestamps` |

## `docker_act`

| `action` | `data` |
| --- | --- |
| `run` | `image`, `name`, `command`, `ports`, `volumes`, `network`, `env`, `envFile`, `restart`, `memory`, `cpus`, `workdir`, `user`, `autoRemove`, `pull`, `platform`, `context`, `dockerfile`, `buildArgs`, `buildPull`, `noCache`, `buildTarget`, `timeoutSeconds` |
| `up` | `file`, `project`, `services`, `build`, `timeoutSeconds` |
| `down` | `file`, `project`, `removeVolumes` |
| `start` `stop` `restart` `pause` `unpause` `kill` | `name`, `signal`, `timeoutSeconds` |
| `remove` | `name`, `resource`, `force`, `removeVolumes` |
| `exec` | `name`, `command`, `user`, `workdir`, `timeoutSeconds` |

```json
{ "action": "run", "data": { "image": "nginx", "name": "web", "ports": ["8080:80"] } }
```

```json
{ "action": "logs", "data": { "name": "web", "tail": 100 } }
```

## Notas

- `run` com `context` constrói a imagem antes de subir; `up`/`down` usam o CLI `docker` (precisa estar no `PATH`).
- `filter` é `chave=valor`, separado por vírgula; `limit: 0` mostra tudo.
- `inspect` remove `Env` em qualquer nível do JSON.
- `exec` roda como o usuário do container (root por padrão).
- Saídas truncadas em 128 KB; compose e build têm timeout padrão de 300 s.

## Env

| Variável | Padrão | Descrição |
| --- | --- | --- |
| `DOCKER_TIMEOUT_SECONDS` | `60` | timeout por operação |
| `DOCKER_HOST` | — | `unix://`, `npipe://` ou `tcp://` |
| `DOCKER_API_VERSION` | negociação | fixa a versão da API |
| `DOCKER_TLS_VERIFY` / `DOCKER_CERT_PATH` | — | TLS |
| `DOCKER_REGISTRY_AUTH` | — | auth base64 para pull privado |
