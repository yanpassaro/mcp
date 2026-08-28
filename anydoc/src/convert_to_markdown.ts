import init, { toMarkdownBytes } from "@anydoc";
import type { McpServer } from "@mcp/server";
import { basename, dirname, extname, join } from "node:path";
import * as z from "@zod";

import { redactPII } from "./pii.ts";

const wasmPath = new URL("wasm/anydoc_wasm_bg.wasm", import.meta.url);
const wasmInput = await Deno.readFile(wasmPath);
await init(wasmInput);


// EXCEL_EXTS: extensões tratadas como planilha Excel pela conversão WASM.
const EXCEL_EXTS = [".xls", ".xlsx", ".xlsm", ".xlsb"];

// MAX_EXCEL_COLS: limite de colunas exibidas no preview de planilhas, para
// evitar tabelas absurdamente largas (planilhas podem ter centenas de colunas).
const MAX_EXCEL_COLS = 100;

// MAX_EXCEL_ROWS: limite de linhas de dados por tabela no preview.
const MAX_EXCEL_ROWS = 50;

function splitRow(line: string): { cells: string[]; starts: boolean; ends: boolean } {
  const starts = line.startsWith("|");
  const ends = line.endsWith("|");
  let cells = line.split("|");
  if (starts && cells[0].trim() === "") cells = cells.slice(1);
  if (ends && cells[cells.length - 1].trim() === "") cells = cells.slice(0, -1);
  return { cells, starts, ends };
}

function joinRow(cells: string[], starts: boolean, ends: boolean): string {
  let s = cells.join("|");
  if (starts) s = "|" + s;
  if (ends) s = s + "|";
  return s;
}

function isSeparator(line: string): boolean {
  if (!line.includes("|")) return false;
  const cells = line.split("|").map((c) => c.trim()).filter((c) => c !== "");
  if (cells.length === 0) return false;
  return cells.every((c) => /^:?-+:?$/.test(c));
}

// limitExcelPreview reduz cada tabela Markdown (delimitada por '|') às primeiras
// MAX_EXCEL_COLS colunas e MAX_EXCEL_ROWS linhas de dados. Preserva o estilo de
// pipes. A ofuscação de nomes de coluna fica em maskNameColumns (todos os
// formatos), não aqui.
function limitExcelPreview(md: string): string {
  const lines = md.split("\n");
  const out: string[] = [];
  let i = 0;
  while (i < lines.length) {
    const line = lines[i];
    const next = lines[i + 1] ?? "";
    if (line.includes("|") && isSeparator(next)) {
      const header = splitRow(line);
      out.push(joinRow(header.cells.slice(0, MAX_EXCEL_COLS), header.starts, header.ends));
      const sep = splitRow(next);
      out.push(joinRow(sep.cells.slice(0, MAX_EXCEL_COLS), sep.starts, sep.ends));
      i += 2;
      let dataCount = 0;
      while (i < lines.length && lines[i].includes("|") && dataCount < MAX_EXCEL_ROWS) {
        const row = splitRow(lines[i]);
        out.push(joinRow(row.cells.slice(0, MAX_EXCEL_COLS), row.starts, row.ends));
        dataCount++;
        i++;
      }
      while (i < lines.length && lines[i].includes("|")) i++; // descarta linhas excedentes
    } else {
      out.push(line);
      i++;
    }
  }
  return out.join("\n");
}


export function registerConvertToMarkdown(server: McpServer): void {
  server.registerTool(
    "anydoc_convert_to_markdown",
    {
      title: "Convert to Markdown",
      description:
        "Read a document and save it as Markdown (.md) next to the source (same base name), returning the absolute output path. Supported formats: Word (.doc, .docx, .docm), PowerPoint (.ppt, .pps, .pot, .pptx, .pptm, .ppsx, .ppsm), Excel (.xls, .xlsx, .xlsm, .xlsb), OpenDocument (.odt, .ods, .odp), RTF (.rtf), EPUB (.epub), CSV (.csv), and PDF (.pdf). For Excel files only, tables are limited to the first 100 columns and 50 rows.",
      inputSchema: z.object({
        path: z.string("Path to the document to convert."),
      }),
    },
    async ({ path }) => {
      const p = path.trim();

      const stat = await Deno.stat(p);

      if (!stat.isFile) {
        throw new Error(`Not a file: ${p}`);
      }

      const bytes = await Deno.readFile(p);
      const markdown = await toMarkdownBytes(bytes);
      let cleaned = redactPII(markdown);

      // Excel: limita a 10 colunas e 50 linhas no preview.
      if (EXCEL_EXTS.includes(extname(p).toLowerCase())) {
        cleaned = limitExcelPreview(cleaned);
      }

      // Grava o .md ao lado do documento de origem (mesmo comportamento das
      // exportações) e devolve o caminho absoluto — não o conteúdo no chat.
      // NUNCA sobrescreve um .md existente (pode ser o fonte original do doc!):
      // se já existir, grava com o sufixo “-extraido”.
      let outPath = join(dirname(p), `${basename(p, extname(p))}.md`);
      if (outPath.toLowerCase() === p.toLowerCase()) {
        throw new Error(`Source is already a .md file: ${p}`);
      }
      let exists = false;
      try {
        exists = (await Deno.stat(outPath)).isFile;
      } catch {
        exists = false;
      }
      if (exists) {
        outPath = join(dirname(p), `${basename(p, extname(p))}-extraido.md`);
      }
      await Deno.writeTextFile(outPath, cleaned);

      return {
        content: [{ type: "text", text: outPath }],
      };
    },
  );
}
