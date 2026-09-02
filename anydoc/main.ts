import { McpServer } from "@mcp/server";
import { serveStdio } from "@mcp/server/stdio";
import { dirname, basename, extname, join } from "node:path";
import { exportFromMarkdown, type DocumentFormat } from "./src/index.ts";
import { registerConvertToMarkdown } from "./src/convert_to_markdown.ts";
import { toMarkdownBytes } from "./src/wasm.ts";
import { isTabular, tabularToMarkdown } from "./src/tabular.ts";
import { redactPII } from "./src/pii.ts";
import * as z from "@zod";

const EXPORT_FORMATS = ["pdf", "docx", "xlsx"] as const;


function sourceToMarkdown(bytes: Uint8Array, ext: string): string {
  if (ext === ".md") return new TextDecoder().decode(bytes);
  if (isTabular(ext)) return redactPII(tabularToMarkdown(bytes, ext));
  return redactPII(toMarkdownBytes(bytes));
}

serveStdio(() => {
  const server = new McpServer({ name: "anydoc", version: "1.0.0" });

  registerConvertToMarkdown(server);

  server.registerTool(
    "anydoc_export",
    {
      title: "Export document",
      description:
        "Export a file to pdf, docx or xlsx in the same folder (same base name), going through Markdown. Sources: .md, or CSV/TSV/JSON/XML/HTML-table/XLS (and any doc the converter supports: docx, pdf, pptx, odt…) — so CSV/TSV/JSON/XML/HTML/XLS become XLSX, and docx/pdf become PDF/DOCX/XLSX. PII is redacted. Returns the absolute output path.",
      inputSchema: z.object({
        path: z.string("Path to the source file to export."),
        format: z.enum(EXPORT_FORMATS),
      }),
    },
    async ({ path, format }) => {
      const p = path.trim();
      const stat = await Deno.stat(p);
      if (!stat.isFile) throw new Error(`Not a file: ${p}`);
      const bytes = await Deno.readFile(p);
      const ext = extname(p).toLowerCase();
      const md = await sourceToMarkdown(bytes, ext);
      const outPath = join(dirname(p), `${basename(p, extname(p))}.${format}`);
      const result = await exportFromMarkdown(outPath, format as DocumentFormat, md);
      return {
        content: [{ type: "text", text: result.path }],
      };
    },
  );

  return server;
});
