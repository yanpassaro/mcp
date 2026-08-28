# AnyDoc MCP

Servidor MCP em **Deno/TypeScript** para conversão e exportação de documentos,
usando a WASM do [Firecrawl anydoc](https://www.npmjs.com/package/@firecrawl/anydoc-wasm).
Diferente dos servidores Go deste repositório, roda via Deno (não gera `.exe`
no `Taskfile`).

## Tools

| Tool | Descrição |
| --- | --- |
| `anydoc_convert_to_markdown` | Lê um documento e salva como Markdown ao lado do original (mesmo nome, `.md`); não sobrescreve `.md` existente (usa sufixo `-extraido`) e devolve o caminho absoluto |
| `anydoc_export_to_pdf` | Exporta um `.md` como PDF na mesma pasta |
| `anydoc_export_to_docx` | Exporta um `.md` como Word (`.docx`) na mesma pasta |
| `anydoc_export_to_xlsx` | Exporta um `.md` como planilha Excel (`.xlsx`) na mesma pasta |

Formatos de conversão: Word (`.doc`/`.docx`/`.docm`), PowerPoint, Excel,
OpenDocument (`.odt`/`.ods`/`.odp`), RTF, EPUB, CSV e PDF. No Excel, o preview
é limitado a 100 colunas e 50 linhas.

O conteúdo convertido passa por um redator de PII (`src/pii.ts`) antes de
retornar.

## Requisitos

- Deno 1.x+;
- o WASM empacotado em `wasm/anydoc_wasm_bg.wasm` (baixado no primeiro build).

## Build e execução

Via Taskfile (na raiz deste repositório de MCPs) — gera `../dist/anydoc.exe`
(a tarefa `prep` cria `dist/` antes):

```powershell
task build:anydoc
```

Direto (o binário vai para `../dist/anydoc.exe` — crie a pasta `dist/` antes
se não existir):

```powershell
cd anydoc
deno task compile
..\dist\anydoc.exe

# ou direto (sem compilar):
deno task start
```

O processo usa **stdio** para o transporte MCP. Logs e erros vão apenas para
**stderr** (nunca stdout, para não quebrar o protocolo).
