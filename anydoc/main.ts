import { McpServer } from "@mcp/server";
import { serveStdio } from "@mcp/server/stdio";
import { dirname, basename, extname, join } from "node:path";
import { exportFromMarkdown, type DocumentFormat } from "./src/index.ts";
import { registerConvertToMarkdown } from "./src/convert_to_markdown.ts";
import * as z from "@zod";

serveStdio(() => {
  const server = new McpServer({ name: "anydoc", version: "1.0.0" });

  registerConvertToMarkdown(server);

  const addExportTool = (
    toolName: string,
    format: DocumentFormat,
    title: string,
    description: string,
  ) => {
    server.registerTool(
      toolName,
      {
        title,
        description,
        inputSchema: z.object({
          path: z.string("Path to the .md file to export."),
        }),
      },
      async ({ path }) => {
        const mdText = new TextDecoder().decode(await Deno.readFile(path));
        const outPath = join(dirname(path), `${basename(path, extname(path))}.${format}`);
        const result = await exportFromMarkdown(outPath, format, mdText);
        return {
          content: [{ type: "text", text: result.path }],
        };
      },
    );
  };

  addExportTool(
    "anydoc_export_to_pdf",
    "pdf",
    "Export to PDF",
    "Read a .md file and export it as a PDF in the same folder (same base name, .pdf extension). Returns the absolute output path.",
  );
  addExportTool(
    "anydoc_export_to_docx",
    "docx",
    "Export to DOCX",
    "Read a .md file and export it as a Word document (.docx) in the same folder (same base name, .docx extension). Returns the absolute output path.",
  );
  addExportTool(
    "anydoc_export_to_xlsx",
    "xlsx",
    "Export to Excel",
    "Read a .md file and export it as an Excel spreadsheet (.xlsx) in the same folder (same base name, .xlsx extension). Returns the absolute output path.",
  );

  return server;
});
