import { dirname } from "node:path";
import type { CreatedDocument } from "./types.ts";

export async function saveFile(
  path: string,
  bytes: Uint8Array,
  losses = 0,
): Promise<CreatedDocument> {
  await Deno.mkdir(dirname(path), { recursive: true });
  await Deno.writeFile(path, bytes);
  return { path, bytes: bytes.length, losses };
}
