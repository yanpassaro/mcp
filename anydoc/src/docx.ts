import { writeDocx } from "@reamkit";
import { jsonToFlowDoc, withUtcDates } from "./flow.ts";
import { saveFile } from "./save.ts";
import { loadImages, loadMermaidImages } from "./images.ts";
import type { ParsedMarkdown } from "./markdown.ts";
import type { CreatedDocument } from "./types.ts";

export async function createDocx(
  path: string,
  parsed: ParsedMarkdown,
): Promise<CreatedDocument> {
  const images = await loadImages(parsed.blocks);
  const mermaid = await loadMermaidImages(parsed.blocks);
  const flow = jsonToFlowDoc(
    {
      title: parsed.title,
      subtitle: parsed.subtitle,
      blocks: parsed.blocks,
    },
    images,
    undefined,
    mermaid,
  );
  const r = withUtcDates(() => writeDocx(flow));
  return saveFile(path, r.bytes, r.losses.length);
}
