export interface MermaidDiagram {
  lines: string[];

  runs?: DiagramRun[][];
  width: number;
}

export type MermaidShape =
  | "rect"
  | "rounded"
  | "circle"
  | "diamond"
  | "cylinder";
export interface MermaidNode {
  id: string;
  label: string;
  shape: MermaidShape;
}
export interface MermaidEdge {
  from: string;
  to: string;
  label?: string;
}

export interface ParsedMermaid {
  nodes: Map<string, MermaidNode>;
  edges: MermaidEdge[];
  order: string[];
  dir: string;
}

const SHAPE_SRC = String
  .raw`\[\([^\[\]]*\)\]|\(\([^()]*\)\)|\[[^\[\]]*\]|\{[^}]*\}|\([^()]*\)`;
const NODE_RE = new RegExp(`^\\s*([A-Za-z0-9_-]+)\\s*(?:(${SHAPE_SRC}))\\s*$`);
const HEAD_RE = /^\s*(?:flowchart|graph)\s+(TB|TD|BT|LR|RL)\b/i;

function shapeOf(def: string): { shape: MermaidShape; label: string } {
  const shape: MermaidShape = def.startsWith("[(")
    ? "cylinder"
    : def.startsWith("((")
    ? "circle"
    : def.startsWith("[")
    ? "rect"
    : def.startsWith("{")
    ? "diamond"
    : def.startsWith("(")
    ? "rounded"
    : "rect";
  const body = def.slice(1, -1).replace(/^\(|\)$/g, "").trim();
  return { shape, label: body || def.trim() };
}

export function wrapLabel(label: string, inner: number): string[] {
  const words = label.split(/\s+/).filter(Boolean);
  if (words.length === 0) return [""];
  const out: string[] = [];
  let cur = "";
  for (const w of words) {
    let piece = w;
    while (piece.length > inner) {
      if (cur) {
        out.push(cur);
        cur = "";
      }
      const head = piece.slice(0, inner);
      const hy = Math.max(head.lastIndexOf("-"), head.lastIndexOf("_"));
      const cut = hy >= 1 && hy + 1 >= inner / 2 ? hy + 1 : inner;
      out.push(piece.slice(0, cut));
      piece = piece.slice(cut);
    }
    const cand = cur ? cur + " " + piece : piece;
    if (cand.length <= inner) cur = cand;
    else {
      out.push(cur);
      cur = piece;
    }
  }
  if (cur) out.push(cur);
  return out;
}

function centerPad(text: string, width: number): string {
  const pad = Math.max(0, width - text.length);
  const left = Math.floor(pad / 2);
  return " ".repeat(left) + text + " ".repeat(pad - left);
}

export type DiagramRun = { text: string; color?: string };

const BOX_COLOR = "56B6C2";
const WIRE_COLOR = "E5C07B";
const LABEL_COLOR = "E6EDF3";
const CYL_COLOR = "C678DD";

function runsOf(chars: string, color?: string): DiagramRun[] {
  return chars ? [{ text: chars, color }] : [];
}

function boxRowRuns(
  n: MermaidNode,
  W: number,
  totalHeight: number,
  portCol: number,
  hasOut: boolean,
): DiagramRun[][] {
  const inner = W - 2;
  const stripe = n.shape === "cylinder" ? 1 : 0;
  const labels = wrapLabel(n.label, inner - 1);
  const interiorCount = totalHeight - 2;
  const padCount = Math.max(0, interiorCount - stripe - labels.length);

  const rows: DiagramRun[][] = [];
  rows.push(runsOf("┌" + "─".repeat(inner) + "┐", BOX_COLOR));
  for (const l of labels) {
    rows.push([
      { text: "│", color: BOX_COLOR },
      { text: " " + centerPad(l, inner - 1), color: LABEL_COLOR },
      { text: "│", color: BOX_COLOR },
    ]);
  }
  for (let i = 0; i < padCount; i++) {
    rows.push(runsOf("│" + " ".repeat(inner) + "│", BOX_COLOR));
  }
  if (stripe) rows.push(runsOf("├" + "─".repeat(inner) + "┤", CYL_COLOR));

  let bottom = "└" + "─".repeat(inner) + "┘";
  if (hasOut && portCol >= 1 && portCol <= W - 2) {
    bottom = bottom.slice(0, portCol) + "┬" + bottom.slice(portCol + 1);
  }
  rows.push(runsOf(bottom, BOX_COLOR));
  return rows;
}

function wireRuns(line: string): DiagramRun[] {
  const out: DiagramRun[] = [];
  let cur = "";
  let colored = false;
  const flush = () => {
    if (!cur) return;
    out.push(colored ? { text: cur, color: WIRE_COLOR } : { text: cur });
    cur = "";
  };
  for (const ch of line) {
    const isWire = ch !== " ";
    if (isWire !== colored) {
      flush();
      colored = isWire;
    }
    cur += ch;
  }
  flush();
  return out.length ? out : [{ text: line }];
}

function drawGap(
  k: number,
  nxtIds: string[],
  edges: MermaidEdge[],
  x: Map<string, number>,
  layerOf: Map<string, number>,
  threads: Map<string, number>,
  W: number,
  artboard: number,
): DiagramRun[][] {
  const GAP = 6;
  const port = (id: string) => x.get(id)! + Math.floor(W / 2);

  const nxtWidth = nxtIds.length * W + (nxtIds.length - 1) * GAP;
  const nxtLeft = Math.max(0, Math.floor((artboard - nxtWidth) / 2));
  const portNext = (id: string) =>
    nxtLeft + nxtIds.indexOf(id) * (W + GAP) + Math.floor(W / 2);
  const ekey = (e: MermaidEdge) => `${e.from}\u0001${e.to}`;

  const active = edges.filter((e) =>
    layerOf.get(e.from)! <= k && layerOf.get(e.to)! >= k + 1
  );

  const r0 = new Array<string>(artboard).fill(" ");
  const r1 = new Array<string>(artboard).fill(" ");
  const r2 = new Array<string>(artboard).fill(" ");
  const labels = new Map<number, string>();
  const put = (arr: string[], c: number, ch: string) => {
    if (c >= 0 && c < artboard) arr[c] = ch;
  };

  const endings: { su: number; sv: number }[] = [];
  for (const e of active) {
    let su = threads.get(ekey(e));
    if (su === undefined) {
      su = port(e.from);
      threads.set(ekey(e), su);
    }
    const sv = portNext(e.to);
    const finishes = layerOf.get(e.to)! === k + 1;
    if (finishes) threads.delete(ekey(e));

    put(r0, su, "│");
    if (!finishes) {
      if (r1[su] === "─") put(r1, su, "┼");
      else if (r1[su] === " ") put(r1, su, "│");
      put(r2, su, "│");
      continue;
    }

    endings.push({ su, sv });
    if (su === sv) {
      if (r1[su] === "─") put(r1, su, "┼");
      else if (r1[su] === " ") put(r1, su, "│");
      put(r2, su, "▼");
      if (e.label) {
        const col = Math.min(su + 2, artboard - e.label.length - 1);
        if (col >= 0 && !labels.has(col)) labels.set(col, e.label);
      }
      continue;
    }

    const lo = Math.min(su, sv);
    const hi = Math.max(su, sv);
    for (let c = lo + 1; c < hi; c++) {
      if (r1[c] === "│") r1[c] = "┼";
      else if (r1[c] === " ") r1[c] = "─";
    }
    if (e.label) {
      let col = lo + Math.floor((hi - lo) / 2);
      col = Math.min(col, Math.max(lo, artboard - e.label.length));
      if (col >= 0 && !labels.has(col)) labels.set(col, e.label);
    }
  }

  const destInfo = new Map<number, { n: number; rail: boolean }>();
  for (const { su, sv } of endings) {
    let d = destInfo.get(sv);
    if (!d) {
      d = { n: 0, rail: false };
      destInfo.set(sv, d);
    }
    d.n++;
    if (su !== sv) d.rail = true;
  }
  for (const [sv, d] of destInfo) {
    const cur = r1[sv];
    if (d.n >= 2) r1[sv] = "┼";
    else if (d.rail) r1[sv] = cur === "│" ? "┼" : "┬";
    else if (cur === "─") r1[sv] = "┼";
    else if (cur === " ") r1[sv] = "│";
    put(r2, sv, "▼");
  }

  const dirs = new Map<number, { l: boolean; r: boolean; v: boolean }>();
  for (const { su, sv } of endings) {
    let d = dirs.get(su);
    if (!d) {
      d = { l: false, r: false, v: false };
      dirs.set(su, d);
    }
    if (sv < su) d.l = true;
    else if (sv > su) d.r = true;
    else d.v = true;
  }
  for (const [su, d] of dirs) {
    const cur = r1[su];

    if (cur === "┼" || cur === "┬" || cur === "┴") continue;
    const three = d.l && d.r;
    const ch = d.v && !d.l && !d.r
      ? "│"
      : three
      ? "┴"
      : d.l && !d.r
      ? "┤"
      : d.r && !d.l
      ? "├"
      : "┬";
    put(r1, su, ch);
  }

  const rows: DiagramRun[][] = [wireRuns(r0.join(""))];
  if (labels.size > 0) {
    const line = new Array<string>(artboard).fill(" ");
    for (const [col, txt] of labels) {
      for (let i = 0; i < txt.length && col + i < artboard; i++) {
        line[col + i] = txt[i];
      }
    }
    rows.push(wireRuns(line.join("")));
  }
  rows.push(wireRuns(r1.join("")));
  rows.push(wireRuns(r2.join("")));
  return rows;
}

function treeText(
  nodes: Map<string, MermaidNode>,
  edges: MermaidEdge[],
  reverse: boolean,
): MermaidDiagram {
  const rows: string[] = [];
  const seen = new Set<string>();
  const src = (e: MermaidEdge) => (reverse ? e.to : e.from);
  const tgt = (e: MermaidEdge) => (reverse ? e.from : e.to);
  const ids = [...nodes.keys()];
  const referenced = new Set(edges.map((e) => tgt(e)));
  const roots = ids.filter((id) => !referenced.has(id));
  const suffix = (e: MermaidEdge) => (e.label ? ` — ${e.label}` : "");

  const emit = (id: string, depth: number) => {
    const out = edges.filter((e) => src(e) === id);
    out.forEach((e, i) => {
      const last = i === out.length - 1;
      const t = nodes.get(tgt(e));
      rows.push(
        `  `.repeat(depth) +
          `${last ? "└" : "├"}──> [${t?.label ?? tgt(e)}]${suffix(e)}`,
      );

      if (!seen.has(tgt(e))) {
        seen.add(tgt(e));
        emit(tgt(e), depth + 1);
      }
    });
  };
  for (const id of roots.length ? roots : ids) {
    if (seen.has(id)) continue;
    seen.add(id);
    rows.push(`[${nodes.get(id)?.label ?? id}]`);
    emit(id, 1);
  }

  for (const id of ids) {
    if (!seen.has(id)) {
      seen.add(id);
      rows.push(`[${nodes.get(id)?.label ?? id}]`);
    }
  }
  return { lines: rows, width: Math.max(...rows.map((r) => r.length), 0) };
}

export function computeLayers(
  edges: MermaidEdge[],
  order: string[],
): string[][] | null {
  const layer = new Map<string, number>();
  for (const id of order) layer.set(id, 0);
  let changed = true;
  let guard = 0;
  while (changed && guard++ < order.length + 1) {
    changed = false;
    for (const e of edges) {
      const a = layer.get(e.from)!;
      const b = layer.get(e.to)!;
      if (b < a + 1) {
        layer.set(e.to, a + 1);
        changed = true;
      }
    }
  }
  if (guard > order.length) return null;

  const layers: string[][] = [];
  for (const id of order) {
    const l = layer.get(id)!;
    if (!layers[l]) layers[l] = [];
    layers[l].push(id);
  }
  return layers;
}

function layoutTD(
  nodes: Map<string, MermaidNode>,
  edges: MermaidEdge[],
  order: string[],
  maxCols: number,
): MermaidDiagram | null {
  const layers = computeLayers(edges, order);
  if (!layers) return null;

  const layer = new Map<string, number>();
  layers.forEach((ids, li) => ids.forEach((id) => layer.set(id, li)));

  const maxLabel = Math.max(
    ...[...nodes.values()].map((n) => n.label.length),
    1,
  );
  const GAP = 6;
  let W = Math.max(14, Math.min(34, maxLabel + 2));
  const fit = () =>
    Math.max(...layers.map((l) => l.length * W + (l.length - 1) * GAP), W);
  while (fit() > maxCols && W > 14) W -= 2;
  const artboard = fit();
  if (artboard > maxCols + 8) return null;

  const outEdges = new Set(edges.map((e) => e.from));
  const threads = new Map<string, number>();
  const allLines: string[] = [];
  const allRuns: DiagramRun[][] = [];
  layers.forEach((ids, li) => {
    const layerWidth = ids.length * W + (ids.length - 1) * GAP;
    const left = Math.max(0, Math.floor((artboard - layerWidth) / 2));
    const x = new Map<string, number>();
    ids.forEach((id, i) => x.set(id, left + i * (W + GAP)));

    const heights = ids.map((id) => {
      const n = nodes.get(id)!;
      return wrapLabel(n.label, W - 3).length +
        (n.shape === "cylinder" ? 3 : 2);
    });
    const h = Math.max(...heights);

    const layerLines: DiagramRun[][] = [];
    const colOf: number[] = [];
    ids.forEach((id) => {
      const n = nodes.get(id)!;
      const portCol = Math.floor(W / 2);
      const rows = boxRowRuns(n, W, h, portCol, outEdges.has(id));
      const start = x.get(id)!;
      rows.forEach((runRow, j) => {
        const cur = layerLines[j] ?? [];
        const curW = colOf[j] ?? 0;
        if (start > curW) {
          cur.push({ text: " ".repeat(start - curW) });
          colOf[j] = start;
        }
        for (const r of runRow) {
          cur.push(r);
          colOf[j] = (colOf[j] ?? 0) + r.text.length;
        }
        layerLines[j] = cur;
      });
    });
    for (const rr of layerLines) {
      allLines.push(rr.map((r) => r.text).join(""));
      allRuns.push(rr);
    }

    if (li < layers.length - 1) {
      for (
        const rr of drawGap(
          li,
          layers[li + 1],
          edges,
          x,
          layer,
          threads,
          W,
          artboard,
        )
      ) {
        allLines.push(rr.map((r) => r.text).join(""));
        allRuns.push(rr);
      }
    }
  });

  return {
    lines: allLines,
    runs: allRuns,
    width: Math.max(...allLines.map((r) => r.length), 0),
  };
}

export function colorDiagram(line: string): DiagramRun[] {
  const boxChars = new Set([
    "│",
    "─",
    "┌",
    "┐",
    "└",
    "┘",
    "├",
    "┤",
    "┬",
    "┴",
    "┼",
  ]);
  const arrowChars = new Set(["▼", "►", ">"]);
  const out: DiagramRun[] = [];
  let cur = "";
  let color: string | undefined;
  const flush = () => {
    if (!cur) return;
    out.push(color ? { text: cur, color } : { text: cur });
    cur = "";
  };
  for (const ch of line) {
    const c = boxChars.has(ch)
      ? BOX_COLOR
      : arrowChars.has(ch)
      ? WIRE_COLOR
      : undefined;
    if (c !== color) {
      flush();
      color = c;
    }
    cur += ch;
  }
  flush();
  return out.length ? out : [{ text: line }];
}

export function splitEdge(
  line: string,
): { fromPart: string; toPart: string; label?: string } | null {
  let idx = line.indexOf("-->");
  let arrowLen = 3;
  if (idx < 0) {
    idx = line.indexOf("->");
    if (idx < 0) return null;
    arrowLen = 2;
  }
  let fromPart = line.slice(0, idx).trim();
  let toPart = line.slice(idx + arrowLen).trim();
  let label: string | undefined;

  let m = /^\|([^|]*)\|\s*/.exec(toPart);
  if (m) {
    label = m[1].trim();
    toPart = toPart.slice(m[0].length);
  } else {
    m = /\s*\|([^|]*)\|\s*$/.exec(fromPart);
    if (m) {
      label = m[1].trim();
      fromPart = fromPart.slice(0, m.index);
    } else {
      m = /^(.*?)\s*--\s+(.+?)\s*$/.exec(fromPart);
      if (m && !m[2].startsWith("-")) {
        label = m[2].trim();
        fromPart = m[1];
      }
    }
  }
  return { fromPart, toPart, label };
}

export function parseMermaid(text: string): ParsedMermaid | null {
  const lines = text.split("\n");
  let dir = "";
  let li = 0;
  while (li < lines.length && !dir) {
    const m = HEAD_RE.exec(lines[li]);
    if (m) dir = m[1].toUpperCase();
    li++;
  }
  if (!dir) return null;

  const nodes = new Map<string, MermaidNode>();
  const edges: MermaidEdge[] = [];
  const order: string[] = [];
  const defNode = (id: string, def?: string) => {
    if (!nodes.has(id)) {
      order.push(id);
      nodes.set(id, { id, label: id, shape: "rect" });
    }
    if (def) {
      const s = shapeOf(def);
      nodes.set(id, { id, label: s.label || id, shape: s.shape });
    }
  };

  for (const raw of lines) {
    const line = raw.trim();
    if (!line || line.startsWith("%%")) continue;
    if (
      /^\s*(?:subgraph|style|classDef|class\b|linkStyle|direction|click|end)\b/
        .test(line)
    ) continue;
    if (line.includes("-->") || line.includes("->")) {
      const side = splitEdge(line);
      if (!side) continue;
      const lm = new RegExp(`^\\s*([A-Za-z0-9_-]+)\\s*(?:(${SHAPE_SRC}))?\\s*$`)
        .exec(side.fromPart);
      const rm = new RegExp(`^\\s*([A-Za-z0-9_-]+)\\s*(?:(${SHAPE_SRC}))?\\s*$`)
        .exec(side.toPart);
      if (!lm || !rm) continue;
      defNode(lm[1], lm[2]);
      defNode(rm[1], rm[2]);
      edges.push({ from: lm[1], to: rm[1], label: side.label });
    } else {
      const nm = NODE_RE.exec(line);
      if (nm) defNode(nm[1], nm[2]);
    }
  }
  if (nodes.size === 0) return null;
  return { nodes, edges, order, dir };
}

export function renderMermaidDiagram(text: string): MermaidDiagram | null {
  const parsed = parseMermaid(text);
  if (!parsed) return null;
  const { nodes, edges, order, dir } = parsed;

  const es = dir === "BT"
    ? edges.map((e) => ({ from: e.to, to: e.from, label: e.label }))
    : edges;
  if (dir === "LR" || dir === "RL") return treeText(nodes, edges, dir === "RL");
  return layoutTD(nodes, es, order, 76) ?? treeText(nodes, es, false);
}
