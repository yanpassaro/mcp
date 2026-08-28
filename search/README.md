# Search MCP

Servidor MCP em Go para **busca web** e **fetch de páginas** usando a
[Exa Search API](https://exa.ai). As tools retornam o conteúdo formatado em
Markdown (destaques de resultados e texto extraído de páginas). Módulo
independente (`module ntdsk.com/mcp/search`).

## Tools

| Tool | Descrição |
| --- | --- |
| `exa_search` | Busca web via Exa Search API, retornando destaques em Markdown |
| `exa_fetch` | Busca o conteúdo completo de uma ou mais páginas (por URL ou ID Exa) |

Boas práticas com o agente:

- `exa_search` aceita apenas `query`, `category`, `numResults` e
  `maxCharacters` (teto de caracteres dos destaques). O resto (tipo de busca,
  formatos de conteúdo) é fixo no operador.
- `exa_fetch` aceita `urls` (ou `ids`) e `maxCharacters` (texto da página).

## Build

Via Taskfile (na raiz deste repositório de MCPs):

```powershell
task build:search
```

Direto:

```powershell
cd search
go mod tidy
go build -o ../dist/search-mcp.exe ./cmd/search-mcp
```

## Variáveis de ambiente

| Variável | Padrão | Descrição |
| --- | --- | --- |
| `EXA_API_KEY` | obrigatório | Chave da API do Exa |
| `EXA_SEARCH_TYPE` | `auto` | Tipo de busca padrão usado pelo `exa_search` |

## Logs

```text
~/.local/share/mcp/search/logs/search-<AAAA-MM-DD_HH-MM-SS>.log
```
