# GitHub MCP

Consulta a API do GitHub (**somente leitura**), focada em busca de documentação e código. Retorna Markdown para facilitar a leitura pelo agente.

Requer `GITHUB_TOKEN` (escopo `read`; `repo` para repositórios privados).

## Tools

| Tool | O que faz |
| --- | --- |
| `github_search` | Busca unificada; `type`: `code`, `repo`, `issue`, `pr`, `commit`, `user` |
| `github_repo_info` | Info de um repositório (stars, licença, topics, últimas 5 releases) |
| `github_get_tree` | Árvore de arquivos de um repositório |
| `github_read_file` | Conteúdo completo de um arquivo |
| `github_get_item` | Lê uma issue/PR por número (`type`: `issue`/`pr`) |

## Variáveis de ambiente

| Variável | Padrão | Descrição |
| --- | --- | --- |
| `GITHUB_TOKEN` | obrigatório | Personal Access Token |
| `GITHUB_BASE_URL` | `https://api.github.com` | URL da API (GitHub Enterprise) |
| `GITHUB_TIMEOUT_SECONDS` | `60` | Timeout de cada requisição HTTP |
