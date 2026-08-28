import { parseMarkdown } from "./markdown.ts";
import { createDocx } from "./docx.ts";
import { createPdf } from "./pdf.ts";
import { createXlsx } from "./xlsx.ts";
import type { CreatedDocument, DocumentFormat } from "./types.ts";

export type { CreatedDocument, DocumentFormat } from "./types.ts";
export { createDocx } from "./docx.ts";
export { createPdf } from "./pdf.ts";
export { createXlsx } from "./xlsx.ts";
export { parseMarkdown } from "./markdown.ts";

export function exportFromMarkdown(
  path: string,
  format: DocumentFormat,
  markdown: string,
): Promise<CreatedDocument> {
  const parsed = parseMarkdown(markdown);
  if (format === "xlsx") return createXlsx(path, parsed);
  if (format === "docx") return createDocx(path, parsed);
  return createPdf(path, parsed);
}
