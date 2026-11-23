# GitHub MCP

Consulta a API do GitHub (**somente leitura**) e devolve Markdown. Requer `GITHUB_TOKEN` (escopo `read`; `repo` para repositórios privados).

## `github_read`

`owner` e `repo` ficam no topo.

| `action` | `data` |
| --- | --- |
| `info` | — |
| `tree` | `ref`, `recursive` |
| `file` | `path`, `ref` |
| `item` | `number`, `type` (`issue`/`pr`) |
| `releases` | `size`, `page` |
| `insights` | `metric` (`contributors`/`commit_activity`/`code_frequency`/`participation`/`punch_card`) |

## `github_search`

| `action` | `data` |
| --- | --- |
| `code`, `repo`, `issue`, `pr`, `commit`, `user` | `query`, `sort`, `order`, `perPage`, `page` |

```json
{ "owner": "octocat", "repo": "Hello-World", "action": "file", "data": { "path": "README.md" } }
```

```json
{ "action": "code", "data": { "query": "func Validate extension:go repo:owner/name" } }
```

## Env

| Variável | Padrão | Descrição |
| --- | --- | --- |
| `GITHUB_TOKEN` | obrigatório | Personal Access Token |
| `GITHUB_BASE_URL` | `https://api.github.com` | URL da API (GitHub Enterprise) |
| `GITHUB_TIMEOUT_SECONDS` | `60` | timeout por requisição |
