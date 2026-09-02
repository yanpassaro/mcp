import init, { toMarkdownBytes } from "@anydoc";

const wasmPath = new URL("wasm/anydoc_wasm_bg.wasm", import.meta.url);
const wasmInput = await Deno.readFile(wasmPath);
await init(wasmInput);

export { toMarkdownBytes };
