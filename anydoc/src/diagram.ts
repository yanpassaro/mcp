// Gera um diagrama mermaid como SVG vetorial (visual "de imagem", não ASCII).
//
// Layout por camadas (longest-path — o mesmo do renderizador ASCII), com
// formas de verdade: retângulos arredondados, círculos, losangos de decisão,
// cilindros de banco de dados, setas com ponta e rótulos de aresta. O SVG é
// rasterizado para PNG pelo módulo diagram-png.ts (resvg-wasm); se o SVG não
// for produzido (LR/RL, ciclos, sintaxe complexa), o fluxo cai no ASCII.
import { computeLayers, parseMermaid, wrapLabel, type MermaidNode } from "./mermaid.ts";
import type { GlyphShaper } from "./svg-text.ts";

export interface DiagramSvg {
  svg: string;
  width: number; // dimensões de design (px); rasteriza-se em 2x para nitidez
  height: number;
}

const FONT = 14;
const CHAR_W = 8.2; // largura média de um caractere latino a 14px
const MAX_NODE_W = 250;
const PAD_X = 24;
const LINE_H = 20;
const VPAD = 13;
const RANK_GAP = 56;
const X_GAP = 26;
const PANEL = 20;
const EDGE_LANE_SPACE = 40; // espaçamento entre fios de arestas longas

// Paleta na pegada dos blocos de código do documento (One Dark sóbrio): painel
// chapado igual ao fundo de código (#1A1D23), bordas em tons calmos e texto em
// off-white — sem brilho nem gradientes chamativos.
const COL = {
  panel: "#1A1D23",
  panelBorder: "#2B303B",
  accent: "#56B6C2",
  nodeFill: "#161B23",
  text: "#D8DEE9",
  edge: "#E5C07B",
  cyan: "#56B6C2",
  purple: "#C678DD",
  orange: "#D19A66",
};

function esc(s: string): string {
  return s.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;").replace(/"/g, "&quot;");
}

interface Sized {
  lines: string[];
  w: number;
  h: number;
  r: number; // raio (círculo)
}

function sizeNode(n: MermaidNode): Sized {
  // Quebra o rótulo até caber na largura máxima da caixa.
  const inner = Math.floor((MAX_NODE_W - PAD_X) / CHAR_W);
  const lines = wrapLabel(n.label, Math.max(6, inner));
  const textW = Math.max(...lines.map((l) => l.length)) * CHAR_W;

  if (n.shape === "diamond") {
    // Mais alto que o box comum (46): a proporção ~1.6:1 faz o losango ler
    // como losango (decisão), não como caixa esmagada.
    const w = Math.min(MAX_NODE_W, Math.max(130, textW + PAD_X + 36));
    const h = lines.length * LINE_H + 58;
    return { lines, w, h, r: 0 };
  }
  if (n.shape === "circle") {
    const r = Math.max(36, (textW + 36) / 2, lines.length * LINE_H / 2 + 16);
    return { lines, w: r * 2, h: r * 2, r };
  }
  if (n.shape === "cylinder") {
    // Altura maior: a tampa (ry=8) e a base consomem ~16px.
    const w = Math.min(MAX_NODE_W, Math.max(130, textW + PAD_X));
    const h = lines.length * LINE_H + VPAD * 2 + 18;
    return { lines, w, h, r: 0 };
  }
  const w = Math.min(MAX_NODE_W, Math.max(90, textW + PAD_X));
  const h = lines.length * LINE_H + VPAD * 2;
  return { lines, w, h, r: 0 };
}

interface Placed extends Sized {
  id: string;
  x: number;
  y: number;
}

// Mistura uma cor hex em direção a preto (mesmo critério do shadeHex do flow).
function shadeHex(color: string, factor: number): string {
  const n = parseInt(color.replace(/^#/, ""), 16);
  const m = (shift: number) => Math.round(((n >> shift) & 255) * factor);
  return `#${((m(16) << 16) | (m(8) << 8) | m(0)).toString(16).padStart(6, "0")}`;
}

const HEADER_H = 28; // barra de título no topo, igual ao badge dos code blocks

export function mermaidToSvg(source: string, shaper?: GlyphShaper): DiagramSvg | null {
  const parsed = parseMermaid(source);
  if (!parsed) return null;
  const { nodes, edges, order, dir } = parsed;
  // Apenas fluxos de cima para baixo viram SVG; LR/RL/ciclos ficam no ASCII.
  if (dir !== "TB" && dir !== "TD" && dir !== "BT") return null;
  const es = dir === "BT" ? edges.map((e) => ({ from: e.to, to: e.from, label: e.label })) : edges;
  const layers = computeLayers(es, order);
  if (!layers) return null;

  const sizes = new Map<string, Sized>();
  for (const [id, n] of nodes) sizes.set(id, sizeNode(n));

  // Largura total das camadas (base do diagrama) e largura dos "fios" longos
  // que descem pela margem direita.
  const layerW = layers.map((ids) => {
    let w = 0;
    ids.forEach((id, i) => w += (sizes.get(id)!.w) + (i ? X_GAP : 0));
    return w;
  });
  const maxLayerW = Math.max(...layerW, 240);
  const layerIdx = new Map<string, number>();
  layers.forEach((ids, i) => ids.forEach((id) => layerIdx.set(id, i)));
  // Bordas direitas por camada já com a centralização do grafo aplicada — é a
  // partir delas que os fios da margem direita partem (coords de grafo).
  const graphRight = Math.max(...layerW.map((lw) => (maxLayerW - lw) / 2 + lw));
  const long = es.filter((e) => layerIdx.get(e.to)! - layerIdx.get(e.from)! > 1);
  const lanes = long.map((_, i) => graphRight + 56 + i * EDGE_LANE_SPACE);

  // Posições das camadas (y) e dos nós (x centralizado por camada).
  const rankY: number[] = [];
  let y = 0;
  for (let i = 0; i < layers.length; i++) {
    rankY.push(y);
    const maxH = Math.max(...layers[i].map((id) => sizes.get(id)!.h));
    y += maxH + RANK_GAP;
  }
  const contentH = rankY[layers.length - 1] + Math.max(...layers[layers.length - 1].map((id) => sizes.get(id)!.h));
  const artW = Math.max(maxLayerW, lanes.length ? lanes[lanes.length - 1] + EDGE_LANE_SPACE : graphRight);

  const W = Math.max(artW + PANEL * 2, 260);
  const H = contentH + PANEL * 2 + HEADER_H;
  const ox = PANEL + (W - PANEL * 2 - artW) / 2; // centraliza o grafo no painel

  const placed = new Map<string, Placed>();
  layers.forEach((ids, li) => {
    let x = ox + (artW - layerW[li]) / 2;
    // Nós da camada com alturas diferentes (losango alto vs box comum) ficam
    // centralizados verticalmente — o desenho deles parte do mesmo topo.
    const maxH = Math.max(...ids.map((id) => sizes.get(id)!.h));
    for (const id of ids) {
      const s = sizes.get(id)!;
      placed.set(id, { id, ...s, x, y: rankY[li] + (maxH - s.h) / 2 + PANEL + HEADER_H });
      x += s.w + X_GAP;
    }
  });

  // ---- Elementos: fundo, formas, arestas, rótulos -------------------------
  const parts: string[] = [];
  parts.push(`<svg xmlns="http://www.w3.org/2000/svg" width="${W}" height="${H}">`);
  // Painel chapado (mesmo fundo dos blocos de código).
  parts.push(`<rect x="1" y="1" width="${W - 2}" height="${H - 2}" rx="12" fill="${COL.panel}" stroke="${COL.panelBorder}" stroke-width="1"/>`);
  // Header estilo code block: fundo tingido com o acento, rótulo DIAGRAMA,
  // linha inferior de acento e a borda esquerda grossa — igual à casca de código.
  const headerTint = shadeHex(COL.accent, 0.14);
  parts.push(`<rect x="1" y="1" width="${W - 2}" height="${HEADER_H}" fill="${headerTint}"/>`);
  parts.push(`<rect x="1" y="${HEADER_H - 1}" width="${W - 2}" height="1" fill="${COL.accent}"/>`);
  parts.push(`<rect x="1" y="1" width="2.5" height="${H - 2}" fill="${COL.accent}"/>`);
  if (shaper) {
    parts.push(shaper({ text: "DIAGRAMA", x: 16, y: HEADER_H / 2 + 4, size: 10.5, anchor: "start", color: COL.accent }));
  } else {
    parts.push(`<text x="16" y="${HEADER_H / 2 + 4}" font-size="10.5" fill="${COL.accent}">DIAGRAMA</text>`);
  }

  const shapes: string[] = [];
  const labels: string[] = [];
  for (const p of placed.values()) {
    const n = nodes.get(p.id)!;
    const cx = p.x + p.w / 2;
    const tx = (line: string, i: number, cy: number) => {
      const base = cy - ((p.lines.length - 1) / 2) * LINE_H + i * LINE_H + 5;
      if (shaper) {
        labels.push(shaper({ text: line, x: cx, y: base, size: FONT, anchor: "middle", color: COL.text }));
      } else {
        labels.push(`<text x="${cx}" y="${base}" text-anchor="middle" font-size="${FONT}" fill="${COL.text}">${esc(line)}</text>`);
      }
    };
    if (n.shape === "diamond") {
      const cy = p.y + p.h / 2;
      shapes.push(`<polygon points="${cx},${p.y} ${p.x + p.w},${cy} ${cx},${p.y + p.h} ${p.x},${cy}" fill="${COL.nodeFill}" stroke="${COL.orange}" stroke-width="2"/>`);
      p.lines.forEach((l, i) => tx(l, i, cy));
    } else if (n.shape === "circle") {
      shapes.push(`<circle cx="${cx}" cy="${p.y + p.r}" r="${p.r - 2}" fill="${COL.nodeFill}" stroke="${COL.cyan}" stroke-width="2"/>`);
      p.lines.forEach((l, i) => tx(l, i, p.y + p.r));
    } else if (n.shape === "cylinder") {
      // Cilindro clássico de banco de dados: tampa superior curva (Q), laterais
      // retas, base arredondada e a “boca” interna do topo.
      const ry = 8;
      const yTop = p.y + ry;
      const yBot = p.y + p.h - ry;
      shapes.push(
        `<path d="M ${p.x} ${yTop} Q ${cx} ${p.y} ${p.x + p.w} ${yTop} L ${p.x + p.w} ${yBot} Q ${cx} ${p.y + p.h} ${p.x} ${yBot} Z" fill="${COL.nodeFill}" stroke="${COL.purple}" stroke-width="2"/>`,
      );
      // Boca interna: arco do topo desenhado por cima do corpo.
      shapes.push(
        `<path d="M ${p.x + 3} ${yTop} Q ${cx} ${p.y - 1} ${p.x + p.w - 3} ${yTop}" fill="none" stroke="${COL.purple}" stroke-width="2"/>`,
      );
      p.lines.forEach((l, i) => tx(l, i, p.y + p.h / 2));
    } else {
      const rx = n.shape === "rounded" ? 16 : 8;
      shapes.push(`<rect x="${p.x}" y="${p.y}" width="${p.w}" height="${p.h}" rx="${rx}" fill="${COL.nodeFill}" stroke="${COL.cyan}" stroke-width="2"/>`);
      p.lines.forEach((l, i) => tx(l, i, p.y + p.h / 2));
    }
  }

  // ---- Arestas -------------------------------------------------------------
  const paths: string[] = [];
  const edgeLabels: string[] = [];
  let laneIdx = 0;
  for (const e of es) {
    const f = placed.get(e.from)!;
    const t = placed.get(e.to)!;
    const sx = f.x + f.w / 2;
    const sy = f.y + f.h;
    const tx2 = t.x + t.w / 2;
    const ty = t.y;
    const span = layerIdx.get(e.to)! - layerIdx.get(e.from)!;

    let d: string;
    let ly = 0;
    let laneX = 0;
    if (span === 1) {
      const my = (sy + ty) / 2;
      d = sx === tx2 ? `M ${sx} ${sy} L ${sx} ${ty}` : `M ${sx} ${sy} L ${sx} ${my} L ${tx2} ${my} L ${tx2} ${ty}`;
      ly = my - 11;
    } else {
      laneX = ox + lanes[laneIdx++];
      // Margens generosas: o fio longo não encosta nos cards (nem nos painéis).
      const y1 = sy + 18;
      const y2 = ty - 18;
      d = `M ${sx} ${sy} L ${sx} ${y1} L ${laneX} ${y1} L ${laneX} ${y2} L ${tx2} ${y2} L ${tx2} ${ty}`;
      ly = y2 - 11;
    }
    parts.push(`<path d="${d}" fill="none" stroke="${COL.edge}" stroke-width="2" stroke-linejoin="round"/>`);
    // Ponta de seta explícita (triângulo) no topo do alvo — sem <marker>, para
    // não depender da orientação automática do resvg no final do caminho.
    paths.push(`<path d="M ${tx2 - 4.5} ${ty - 9.5} L ${tx2 + 4.5} ${ty - 9.5} L ${tx2} ${ty} z" fill="${COL.edge}"/>`);
    if (e.label) {
      // Posiciona o rótulo sem “brocar” os fios verticais: vão curto desloca o
      // texto para o lado do fio da origem (start/end em vez de centro).
      const labelW = e.label.length * 5.8;
      let anchor: "start" | "middle" | "end";
      let lx2: number;
      if (span > 1) {
        anchor = "end";
        lx2 = laneX - 8;
      } else if (sx === tx2) {
        anchor = "start";
        lx2 = sx + 16; // folga extra do fio vertical
      } else if (Math.abs(tx2 - sx) >= labelW + 16) {
        anchor = "middle";
        lx2 = (sx + tx2) / 2;
      } else if (tx2 > sx) {
        anchor = "start";
        lx2 = sx + 14;
      } else {
        anchor = "end";
        lx2 = sx - 14;
      }
      if (shaper) {
        edgeLabels.push(shaper({ text: e.label, x: lx2, y: ly, size: 11.5, anchor, color: COL.edge }));
      } else {
        edgeLabels.push(`<text x="${lx2}" y="${ly}" text-anchor="${anchor}" font-size="11.5" fill="${COL.edge}" font-style="italic">${esc(e.label)}</text>`);
      }
    }
  }

  parts.push(...shapes, ...paths, ...edgeLabels, ...labels, `</svg>`);
  return { svg: parts.join("\n"), width: W, height: H };
}
