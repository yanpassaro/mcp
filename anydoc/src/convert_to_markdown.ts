import type { McpServer } from "@mcp/server";
import { basename, dirname, extname, join } from "node:path";
import * as z from "@zod";

import { toMarkdownBytes } from "./wasm.ts";
import { redactPII } from "./pii.ts";

export function registerConvertToMarkdown(server: McpServer): void {
  server.registerTool(
    "anydoc_import",
    {
      title: "Convert to Markdown",
      description:
        "Read a document and save it as Markdown next to the source (same base name). Supported formats: Word, PowerPoint, Excel, OpenDocument, RTF, CSV, and PDF.",
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
      const markdown = toMarkdownBytes(bytes);
      const cleaned = redactPII(markdown);

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
