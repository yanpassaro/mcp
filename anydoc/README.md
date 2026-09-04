# AnyDoc MCP

Converte e exporta documentos (**Deno/TS**), passando por Markdown. Devolve o caminho absoluto do arquivo gerado.

## Tools

| Tool | O que faz |
| --- | --- |
| `anydoc_import` | Converte um documento para Markdown ao lado do original (mesmo nome, `.md`) |
| `anydoc_export` | Exporta para `pdf`/`docx`/`xlsx` na mesma pasta (via Markdown) |

Formatos suportados: Word (`.doc`/`.docx`/`.docm`), PowerPoint, Excel, OpenDocument (`.odt`/`.ods`/`.odp`), RTF, EPUB, CSV e PDF. CSV/TSV/JSON/XML/HTML de tabela/XLS viram XLSX.

Sem variáveis de ambiente (usa stderr).
