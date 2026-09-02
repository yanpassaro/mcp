import type { DocumentBlock, RunSpec } from "./flow.ts";

export interface ParsedMarkdown {
  title?: string;
  subtitle?: string;
  blocks: DocumentBlock[];
  tables: { columns: string[]; rows: string[][] }[];
}


export function slug(text: string): string {
  return text
    .trim()
    .toLowerCase()
    .replace(/[^\p{L}\p{N}\s-]/gu, "")
    .replace(/\s+/g, "-")
    .replace(/-+/g, "-")
    .replace(/^-+|-+$/g, "");
}

function splitRow(row: string): string[] {
  let r = row.trim();
  if (r.startsWith("|")) r = r.slice(1);
  if (r.endsWith("|")) r = r.slice(0, -1);
  return r.split("|").map((c) => c.trim());
}

function isTableSep(line: string): boolean {
  const t = line.trim();
  return /^\|?[\s:|-]+\|?$/.test(t) && t.includes("-");
}

function normalizeHtml(text: string): string {
  return text
    .replace(/<br\s*\/?>/gi, "\n")
    .replace(/<(strong|b)>([\s\S]*?)<\/\1>/gi, "**$2**")
    .replace(/<(em|i)>([\s\S]*?)<\/\1>/gi, "*$2*")
    .replace(/<code>([\s\S]*?)<\/code>/gi, "`$1`")
    .replace(/<a\s+[^>]*href=["']([^"']+)["'][^>]*>([\s\S]*?)<\/a>/gi, "[$2]($1)")
    .replace(/<(https?:\/\/[^\s>]+)>/gi, "[$1]($1)")
    .replace(/<[^>]+>/g, "");
}

function pushEmph(runs: RunSpec[], inner: string, flags: Partial<RunSpec>, refs: Record<string, string>): void {
  for (const r of parseInline(inner, refs)) {
    runs.push({ ...r, ...flags });
  }
}

export function parseInline(text: string, refs: Record<string, string> = {}): RunSpec[] {
  const src = normalizeHtml(text);
  const runs: RunSpec[] = [];

  const re =
    /(\*\*[^*]+\*\*|(?<![A-Za-z0-9])__[^_]+__(?![A-Za-z0-9])|\*[^*]+\*|(?<![A-Za-z0-9])_[^_]+_(?![A-Za-z0-9])|`[^`]+`|~~[^~]+~~|\[\^([^\]]+)\]|\[[^\]]+\]\([^)]+\)|\[[^\]]+\]\[[^\]]+\]|<https?:\/\/[^\s>]+>)/g;
  let last = 0;
  let m: RegExpExecArray | null;
  while ((m = re.exec(src)) !== null) {
    if (m.index > last) runs.push({ text: src.slice(last, m.index) });
    const tok = m[0];
    if (tok.startsWith("**") && tok.endsWith("**")) {
      pushEmph(runs, tok.slice(2, -2), { bold: true }, refs);
    } else if (tok.startsWith("__") && tok.endsWith("__")) {
      pushEmph(runs, tok.slice(2, -2), { bold: true }, refs);
    } else if (tok.startsWith("*") && tok.endsWith("*")) {
      pushEmph(runs, tok.slice(1, -1), { italic: true }, refs);
    } else if (tok.startsWith("_") && tok.endsWith("_")) {
      pushEmph(runs, tok.slice(1, -1), {}, refs);
    } else if (tok.startsWith("`") && tok.endsWith("`")) {
      runs.push({ text: tok.slice(1, -1), code: true });
    } else if (tok.startsWith("~~") && tok.endsWith("~~")) {
      runs.push({ text: tok.slice(2, -2), strike: true });
    } else if (tok.startsWith("[^") && tok.endsWith("]")) {
      const fm = /^\[\^([^\]]+)\]$/.exec(tok);
      if (fm) runs.push({ text: fm[1], verticalAlign: "superscript" });
      else runs.push({ text: tok });
    } else if (tok.startsWith("[") && tok.endsWith(")")) {
      const lm = /^\[([^\]]+)\]\(([^)]+)\)$/.exec(tok);
      if (lm) runs.push({ text: lm[1], link: true, url: lm[2] });
      else runs.push({ text: tok });
    } else if (tok.startsWith("[") && tok.includes("][")) {
      const rm = /^\[([^\]]+)\]\[([^\]]+)\]$/.exec(tok);
      if (rm) {
        const url = refs[rm[2].toLowerCase()];
        if (url) runs.push({ text: rm[1], link: true, url });
        else runs.push({ text: tok });
      } else runs.push({ text: tok });
    } else if (tok.startsWith("<") && tok.endsWith(">")) {
      const url = tok.slice(1, -1);
      runs.push({ text: url, link: true, url });
    } else {
      runs.push({ text: tok });
    }
    last = re.lastIndex;
  }
  if (last < src.length) runs.push({ text: src.slice(last) });
  if (runs.length === 0) runs.push({ text: src });
  return runs;
}

function emitParagraph(lines: string[], container: DocumentBlock[], refs: Record<string, string>): void {
  const hasImage = lines.some((l) => /!\[[^\]]*\]\([^)]+\)/.test(l));
  if (!hasImage) {
    container.push({ kind: "paragraph", runs: parseInline(lines.join(" "), refs) });
    return;
  }
  const IMAGE_RE = /!\[([^\]]*)\]\(([^)]+)\)/g;
  for (const line of lines) {
    const matches = [...line.matchAll(IMAGE_RE)];
    if (matches.length === 0) {
      container.push({ kind: "paragraph", runs: parseInline(line, refs) });
      continue;
    }
    let last = 0;
    let pending = "";
    for (const m of matches) {
      const before = line.slice(last, m.index ?? 0).trim();
      if (before) pending += (pending ? " " : "") + before;
      if (pending) {
        container.push({ kind: "paragraph", runs: parseInline(pending, refs) });
        pending = "";
      }
      container.push({ kind: "image", url: m[2], text: m[1] || "image" });
      last = (m.index ?? 0) + m[0].length;
    }
    const after = line.slice(last).trim();
    if (after) container.push({ kind: "paragraph", runs: parseInline(after, refs) });
  }
}

export function parseMarkdown(md: string): ParsedMarkdown {
  const lines = md.replace(/\r\n/g, "\n").replace(/\t/g, "    ").split("\n");
  const blocks: DocumentBlock[] = [];
  const tables: { columns: string[]; rows: string[][] }[] = [];
  const refs: Record<string, string> = {};
  for (const line of lines) {
    const rd = /^\[([^\]]+)\]:\s*(?:<([^>]+)>|(\S+))/.exec(line.trim());
    if (rd) refs[rd[1].toLowerCase()] = rd[2] ?? rd[3] ?? "";
  }


  let outlineCode = false;
  let outlineTitleSeen = false;
  let outlineSubSeen = false;
  const outline: { text: string; level: number }[] = [];
  for (const raw of lines) {
    const tl = raw.trim();
    if (/^```/.test(tl)) { outlineCode = !outlineCode; continue; }
    if (outlineCode) continue;
    const oh = /^(#{1,6})\s+(.*?)\s*#*\s*$/.exec(tl);
    if (oh) {
      const lvl = oh[1].length;
      const txt = oh[2].trim();
      let isTitle = false;
      let isSub = false;
      if (lvl === 1 && !outlineTitleSeen) { isTitle = true; outlineTitleSeen = true; }
      else if (lvl === 2 && !outlineSubSeen && !outlineTitleSeen) { isSub = true; outlineSubSeen = true; }
      if (!isTitle && !isSub) outline.push({ text: txt, level: lvl });
    }
  }

  let title: string | undefined;
  let subtitle: string | undefined;
  let listItems: string[] = [];

  const flushList = () => {
    if (listItems.length) {
      blocks.push({ kind: "list", items: listItems.slice() });
      listItems = [];
    }
  };

  let i = 0;
  while (i < lines.length) {
    const trimmed = lines[i].trim();

    const refDef = /^\[([^\]]+)\]:\s*(?:<([^>]+)>|(\S+))/.exec(trimmed);
    if (refDef) {
      flushList();
      refs[refDef[1].toLowerCase()] = refDef[2] ?? refDef[3] ?? "";
      i++;
      continue;
    }

    const fnDef = /^\[\^([^\]]+)\]:\s*(.+)$/.exec(trimmed);
    if (fnDef) {
      flushList();
      blocks.push({
        kind: "paragraph",
        size: 9,
        runs: [{ text: `${fnDef[1]}. `, bold: true }, { text: fnDef[2] }],
      });
      i++;
      continue;
    }

    if (/^\[(?:(?:toc|sum[aá]rio|índice|indice))\]$/i.test(trimmed)) {
      flushList();
      if (outline.length > 0) {
        blocks.push({ kind: "heading", text: "Sumário" });
        for (const entry of outline) {
          blocks.push({
            kind: "paragraph",
            indent: Math.max(0, entry.level - 1),
            runs: [{ text: entry.text, link: true, url: "#" + slug(entry.text) }],
          });
        }
      }
      i++;
      continue;
    }

    if (/^```/.test(trimmed)) {
      flushList();
      const language = trimmed.slice(3).trim().split(/\s+/)[0] || undefined;
      const codeLines: string[] = [];
      i++;
      while (i < lines.length && !/^```\s*$/.test(lines[i].trim())) {
        codeLines.push(lines[i]);
        i++;
      }
      i++;
      blocks.push({ kind: "codeblock", language, text: codeLines.join("\n") });
      continue;
    }

    const h = /^(#{1,6})\s+(.*?)\s*#*\s*$/.exec(trimmed);
    if (h) {
      flushList();
      const level = h[1].length;
      const text = h[2].trim();
      if (level === 1 && title === undefined) {
        title = text;
        i++;
        continue;
      }
      if (level === 2 && subtitle === undefined && title === undefined) {
        subtitle = text;
        i++;
        continue;
      }
      blocks.push({ kind: "heading", text, level });
      i++;
      continue;
    }

    if (/^([-*_])\1{2,}$/.test(trimmed)) {
      flushList();
      blocks.push({ kind: "break" });
      i++;
      continue;
    }

    if (trimmed.includes("|") && i + 1 < lines.length && isTableSep(lines[i + 1])) {
      const columns = splitRow(trimmed);
      const sep = lines[i + 1].trim();
      const aligns = splitRow(sep).map((s) => {
        const left = s.startsWith(":");
        const right = s.endsWith(":");
        if (left && right) return "center";
        if (right) return "right";
        if (left) return "left";
        return "left";
      });
      i += 2;
      const rows: string[][] = [];
      while (i < lines.length && lines[i].trim().includes("|") && !/^```/.test(lines[i].trim())) {
        rows.push(splitRow(lines[i].trim()));
        i++;
      }
      tables.push({ columns, rows });
      blocks.push({
        kind: "table",
        columns,
        rows,
        alignCols: aligns,
        borders: true,
        striped: true,
        headerBackground: "#17171c",
        headerColor: "#ffffff",
      });
      continue;
    }

    const li = /^(\s*)(?:[-*+]|\d+[.)])\s+(.*)$/.exec(trimmed);
    if (li) {
      listItems.push(li[2]);
      i++;
      continue;
    }

    if (trimmed.startsWith(">")) {
      flushList();
      const quoteLines: string[] = [];
      while (i < lines.length && lines[i].trim().startsWith(">")) {
        const l = lines[i].trim().replace(/^>+/, "").replace(/^\s+/, "");
        quoteLines.push(l);
        i++;
      }
      blocks.push({ kind: "blockquote", items: quoteLines });
      continue;
    }

    if (trimmed.startsWith(": ")) {
      flushList();
      blocks.push({ kind: "paragraph", runs: parseInline(trimmed.slice(2), refs), size: 10 });
      i++;
      continue;
    }

    if (
      i + 1 < lines.length &&
      !/^[:#>`\-\*+\d\s]/.test(trimmed) &&
      lines[i + 1].trim().startsWith(": ")
    ) {
      flushList();
      const term = trimmed;
      i++;
      const defs: string[] = [];
      while (i < lines.length && lines[i].trim().startsWith(": ")) {
        defs.push(lines[i].trim().slice(2));
        i++;
      }
      blocks.push({ kind: "definition", text: term, items: defs });
      continue;
    }

    if (trimmed === "") {
      flushList();
      i++;
      continue;
    }

    flushList();
    const paraLines: string[] = [];
    while (i < lines.length) {
      const t = lines[i].trim();
      if (t === "") break;
      if (/^#{1,6}\s+/.test(t)) break;
      if (/^```/.test(t)) break;
      if (/^([-*_])\1{2,}$/.test(t)) break;
      if (/^(\s*)(?:[-*+]|\d+[.)])\s+/.test(t)) break;
      if (t.includes("|") && i + 1 < lines.length && isTableSep(lines[i + 1])) break;
      paraLines.push(t);
      i++;
    }
    emitParagraph(paraLines, blocks, refs);
  }

  flushList();
  return { title, subtitle, blocks, tables };
}
