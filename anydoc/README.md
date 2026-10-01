# AnyDoc MCP

Lê documentos e devolve o conteúdo como Markdown.
Escrito em **Rust**, usando o crate [`anydoc`](https://github.com/firecrawl/anydoc) (nativo, sem WASM).

## Tools

| Tool | O que faz |
| --- | --- |
| `anydoc_read` | Lê um documento e grava o Markdown ao lado do original (mesmo nome, `.md`; `-extraido.md` se já existir) |

Devolve o caminho absoluto do arquivo gerado. A redação de PII é aplicada antes de gravar.

## Formatos de entrada

A conversão usa o crate `anydoc`, que detecta o formato pelo conteúdo dos bytes
(assinatura do PDF, grupo de abertura do RTF, streams OLE, mimetype do pacote ZIP),
e não pela extensão — arquivos mal nomeados convertem certo.

| Formato | Extensões |
| --- | --- |
| Word | `.doc`, `.docx`, `.docm` |
| PowerPoint | `.ppt`, `.pps`, `.pot`, `.pptx`, `.pptm`, `.ppsx`, `.ppsm` |
| Excel | `.xls`, `.xlsx`, `.xlsm`, `.xlsb` |
| OpenDocument | `.odt`, `.ods`, `.odp` |
| Rich Text | `.rtf` |
| EPUB | `.epub` |
| CSV | `.csv` |
| PDF | `.pdf` |

CSV/TSV/JSON/XML/HTML-de-tabela passam por um conversor próprio (`src/tabular.rs`):
são formatos tabulares simples e não vale o custo de abrir o modelo de documento
completo.

## Redação de PII

Todo Markdown gerado passa por `src/pii.rs` antes de ser devolvido:

- padrões com validação real (CPF, CNPJ, cartão via Luhn) só mascaram se o dígito
  verificador fecha;
- tabelas mascaram célula a célula pela entidade do cabeçalho;
- campos no formato `chave: valor` são mascarados pelo nome da chave;
- endereços, e-mails, telefones, IPs, MAC, JWT, hashes e URLs viram rótulos;
- blocos de código são preservados intactos.

## Estrutura

```
src/
├── main.rs      # servidor MCP (stdio) e a tool anydoc_read
├── pii.rs       # redação de PII
└── tabular.rs   # CSV/TSV/JSON/XML/HTML -> tabela Markdown
```

## Build

```bash
cargo build --release        # anydoc/target/release/anydoc-mcp
```

Ou pelo Taskfile, junto com os outros módulos:

```bash
task build:anydoc            # gera dist/anydoc-mcp.exe
```

## Limitações

- **Sem OCR.** PDF digitalizado falha com `NeedsOcr`; a versão em Rust do crate
  `anydoc` não manda o arquivo para nenhum serviço externo.
- Sem variáveis de ambiente (saída de log vai para stderr).