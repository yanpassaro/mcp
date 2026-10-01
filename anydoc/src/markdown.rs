use std::collections::HashMap;

use regex::Regex;
use std::sync::OnceLock;

#[derive(Debug, Clone, PartialEq)]
pub enum Align {
    Left,
    Center,
    Right,
}

#[derive(Debug, Clone, Default)]
pub struct Run {
    pub text: String,
    pub bold: bool,
    pub italic: bool,
    pub strike: bool,
    pub code: bool,
    pub link: Option<String>,
    pub superscript: bool,
}

#[derive(Debug, Clone)]
pub enum Block {
    Heading {
        text: String,
        level: usize,
    },
    Paragraph {
        runs: Vec<Run>,
        indent: usize,
        size: Option<u8>,
    },
    List {
        items: Vec<String>,
    },
    Blockquote {
        items: Vec<String>,
    },
    CodeBlock {
        language: Option<String>,
        text: String,
    },
    Table {
        columns: Vec<String>,
        rows: Vec<Vec<String>>,
        align: Vec<Align>,
    },
    Image {
        url: String,
        text: String,
    },
    Break,
    Definition {
        text: String,
        items: Vec<String>,
    },
}

#[derive(Debug, Clone, Default)]
pub struct ParsedMarkdown {
    pub title: Option<String>,
    pub subtitle: Option<String>,
    pub blocks: Vec<Block>,
}

fn heading_re() -> &'static Regex {
    static RE: OnceLock<Regex> = OnceLock::new();
    RE.get_or_init(|| Regex::new(r"^(#{1,6})\s+(.*?)\s*#*\s*$").expect("heading"))
}

fn fence_re() -> &'static Regex {
    static RE: OnceLock<Regex> = OnceLock::new();
    RE.get_or_init(|| Regex::new(r"^```").expect("fence"))
}

fn hr_re() -> &'static Regex {
    static RE: OnceLock<Regex> = OnceLock::new();
    RE.get_or_init(|| Regex::new(r"^([-*_])\1{2,}$").expect("hr"))
}

fn list_re() -> &'static Regex {
    static RE: OnceLock<Regex> = OnceLock::new();
    RE.get_or_init(|| Regex::new(r"^(\s*)(?:[-*+]|\d+[.)])\s+(.*)$").expect("list"))
}

fn ref_def_re() -> &'static Regex {
    static RE: OnceLock<Regex> = OnceLock::new();
    RE.get_or_init(|| {
        Regex::new(r"^\[([^\]]+)\]:\s*(?:<([^>]+)>|(\S+))").expect("ref def")
    })
}

fn footnote_re() -> &'static Regex {
    static RE: OnceLock<Regex> = OnceLock::new();
    RE.get_or_init(|| Regex::new(r"^\[\^([^\]]+)\]:\s*(.+)$").expect("footnote"))
}

fn toc_re() -> &'static Regex {
    static RE: OnceLock<Regex> = OnceLock::new();
    RE.get_or_init(|| {
        Regex::new(r"(?i)^\[(?:toc|sum[aá]rio|índice|indice)\]$").expect("toc")
    })
}

fn table_sep_re() -> &'static Regex {
    static RE: OnceLock<Regex> = OnceLock::new();
    RE.get_or_init(|| Regex::new(r"^\|?[\s:|-]+\|?$").expect("table sep"))
}

fn image_re() -> &'static Regex {
    static RE: OnceLock<Regex> = OnceLock::new();
    RE.get_or_init(|| Regex::new(r"!\[[^\]]*\]\([^)]+\)").expect("image"))
}

fn image_token_re() -> &'static Regex {
    static RE: OnceLock<Regex> = OnceLock::new();
    RE.get_or_init(|| Regex::new(r"!\[([^\]]*)\]\(([^)]+)\)").expect("image token"))
}

fn inline_re() -> &'static Regex {
    static RE: OnceLock<Regex> = OnceLock::new();
    RE.get_or_init(|| {
        Regex::new(
            r"(\*\*[^*]+\*\*|__[^_]+__|\*[^*]+\*|_[^_]+_|`[^`]+`|~~[^~]+~~|\[\^([^\]]+)\]|\[[^\]]+\]\([^)]+\)|\[[^\]]+\]\[[^\]]+\]|<https?://[^>\s]+>)",
        )
        .expect("inline")
    })
}

pub fn slug(text: &str) -> String {
    let lower = text.trim().to_lowercase();
    let mut out = String::new();
    let mut prev_space = false;
    for c in lower.chars() {
        if c.is_alphanumeric() || c == '-' {
            out.push(c);
            prev_space = false;
            continue;
        }
        if c.is_whitespace() {
            if !prev_space {
                out.push('-');
                prev_space = true;
            }
        }
    }
    let mut collapsed = String::new();
    let mut prev_dash = false;
    for c in out.chars() {
        if c == '-' {
            if !prev_dash {
                collapsed.push(c);
            }
            prev_dash = true;
            continue;
        }
        prev_dash = false;
        collapsed.push(c);
    }
    collapsed.trim_matches('-').to_string()
}

fn split_row(row: &str) -> Vec<String> {
    let mut r = row.trim();
    if let Some(stripped) = r.strip_prefix('|') {
        r = stripped;
    }
    if let Some(stripped) = r.strip_suffix('|') {
        r = stripped;
    }
    r.split('|').map(|c| c.trim().to_string()).collect()
}

fn is_table_sep(line: &str) -> bool {
    let t = line.trim();
    table_sep_re().is_match(t) && t.contains('-')
}

fn normalize_html(text: &str) -> String {
    static BR: OnceLock<Regex> = OnceLock::new();
    static STRONG: OnceLock<Regex> = OnceLock::new();
    static EM: OnceLock<Regex> = OnceLock::new();
    static CODE: OnceLock<Regex> = OnceLock::new();
    static ANCHOR: OnceLock<Regex> = OnceLock::new();
    static AUTOLINK: OnceLock<Regex> = OnceLock::new();
    static TAG: OnceLock<Regex> = OnceLock::new();

    let br = BR.get_or_init(|| Regex::new(r"(?i)<br\s*/?>").expect("br"));
    let strong = STRONG.get_or_init(|| Regex::new(r"(?is)<(strong|b)>([\s\S]*?)</\1>").expect("strong"));
    let em = EM.get_or_init(|| Regex::new(r"(?is)<(em|i)>([\s\S]*?)</\1>").expect("em"));
    let code = CODE.get_or_init(|| Regex::new(r"(?is)<code>([\s\S]*?)</code>").expect("code"));
    let anchor = ANCHOR.get_or_init(|| {
        Regex::new(r#"(?is)<a\s+[^>]*href=["']([^"']+)["'][^>]*>([\s\S]*?)</a>"#).expect("a")
    });
    let autolink = AUTOLINK.get_or_init(|| {
        Regex::new(r"(?i)<(https?://[^>\s]+)>").expect("autolink")
    });
    let tag = TAG.get_or_init(|| Regex::new(r"<[^>]+>").expect("tag"));

    let s = br.replace_all(text, "\n");
    let s = strong.replace_all(&s, "**$2**");
    let s = em.replace_all(&s, "*$2*");
    let s = code.replace_all(&s, "`$1`");
    let s = anchor.replace_all(&s, "[$2]($1)");
    let s = autolink.replace_all(&s, "[$1]($1)");
    tag.replace_all(&s, "").into_owned()
}

fn push_emph(
    runs: &mut Vec<Run>,
    inner: &str,
    bold: bool,
    italic: bool,
    refs: &HashMap<String, String>,
) {
    for mut r in parse_inline(inner, refs) {
        if bold {
            r.bold = true;
        }
        if italic {
            r.italic = true;
        }
        runs.push(r);
    }
}

pub fn parse_inline(text: &str, refs: &HashMap<String, String>) -> Vec<Run> {
    let src = normalize_html(text);
    let mut runs: Vec<Run> = Vec::new();
    let mut last = 0usize;

    for m in inline_re().find_iter(&src) {
        let start = m.start();
        if start > last {
            runs.push(Run {
                text: src[last..start].to_string(),
                ..Default::default()
            });
        }
        let tok = &src[m.start()..m.end()];
        push_tok(&mut runs, tok, refs);
        last = m.end();
    }
    if last < src.len() {
        runs.push(Run {
            text: src[last..].to_string(),
            ..Default::default()
        });
    }
    if runs.is_empty() {
        runs.push(Run {
            text: src.clone(),
            ..Default::default()
        });
    }
    runs
}

fn push_tok(runs: &mut Vec<Run>, tok: &str, refs: &HashMap<String, String>) {
    if let Some(inner) = strip(tok, "**") {
        push_emph(runs, inner, true, false, refs);
        return;
    }
    if let Some(inner) = strip(tok, "__") {
        push_emph(runs, inner, true, false, refs);
        return;
    }
    if let Some(inner) = strip(tok, "~~") {
        runs.push(Run {
            text: inner.to_string(),
            strike: true,
            ..Default::default()
        });
        return;
    }
    if let Some(inner) = strip(tok, "`") {
        runs.push(Run {
            text: inner.to_string(),
            code: true,
            ..Default::default()
        });
        return;
    }
    if tok.len() >= 2 && tok.starts_with('*') && tok.ends_with('*') {
        push_emph(runs, &tok[1..tok.len() - 1], false, true, refs);
        return;
    }
    if tok.len() >= 2 && tok.starts_with('_') && tok.ends_with('_') {
        push_emph(runs, &tok[1..tok.len() - 1], false, false, refs);
        return;
    }
    if let Some(inner) = tok.strip_prefix("[^") {
        if let Some(name) = inner.strip_suffix(']') {
            runs.push(Run {
                text: name.to_string(),
                superscript: true,
                ..Default::default()
            });
            return;
        }
    }
    if let Some((label, url)) = parse_link(tok) {
        runs.push(Run {
            text: label,
            link: Some(url),
            ..Default::default()
        });
        return;
    }
    if let Some((label, key)) = parse_ref_link(tok) {
        if let Some(url) = refs.get(&key.to_lowercase()) {
            runs.push(Run {
                text: label,
                link: Some(url.clone()),
                ..Default::default()
            });
            return;
        }
    }
    if tok.len() >= 2 && tok.starts_with('<') && tok.ends_with('>') {
        let url = &tok[1..tok.len() - 1];
        runs.push(Run {
            text: url.to_string(),
            link: Some(url.to_string()),
            ..Default::default()
        });
        return;
    }
    runs.push(Run {
        text: tok.to_string(),
        ..Default::default()
    });
}

fn strip<'a>(tok: &'a str, delim: &str) -> Option<&'a str> {
    if tok.len() >= 4 && tok.starts_with(delim) && tok.ends_with(delim) {
        return Some(&tok[delim.len()..tok.len() - delim.len()]);
    }
    None
}

fn parse_link(tok: &str) -> Option<(String, String)> {
    let close = tok.find("](")?;
    if !tok.ends_with(')') {
        return None;
    }
    let label = &tok[1..close];
    let url = &tok[close + 2..tok.len() - 1];
    if url.is_empty() {
        return None;
    }
    Some((label.to_string(), url.to_string()))
}

fn parse_ref_link(tok: &str) -> Option<(String, String)> {
    let close = tok.find("][")?;
    let label = &tok[1..close];
    let key = &tok[close + 2..tok.len() - 1];
    if key.is_empty() {
        return None;
    }
    Some((label.to_string(), key.to_string()))
}

fn emit_paragraph(lines: &[String], out: &mut Vec<Block>, refs: &HashMap<String, String>) {
    let has_image = lines.iter().any(|l| image_re().is_match(l));
    if !has_image {
        out.push(Block::Paragraph {
            runs: parse_inline(&lines.join(" "), refs),
            indent: 0,
            size: None,
        });
        return;
    }
    for line in lines {
        let matches: Vec<_> = image_token_re().captures_iter(line).collect();
        if matches.is_empty() {
            out.push(Block::Paragraph {
                runs: parse_inline(line, refs),
                indent: 0,
                size: None,
            });
            continue;
        }
        let mut last = 0usize;
        let mut pending = String::new();
        for m in matches {
            let start = m.get(0).map(|g| g.start()).unwrap_or(0);
            let before = line[last..start].trim();
            if !before.is_empty() {
                if !pending.is_empty() {
                    pending.push(' ');
                }
                pending.push_str(before);
            }
            if !pending.is_empty() {
                out.push(Block::Paragraph {
                    runs: parse_inline(&pending, refs),
                    indent: 0,
                    size: None,
                });
                pending.clear();
            }
            let url = m.get(2).map(|g| g.as_str().to_string()).unwrap_or_default();
            let text = m.get(1).map(|g| g.as_str().to_string()).unwrap_or_default();
            out.push(Block::Image {
                url,
                text: if text.is_empty() {
                    "image".to_string()
                } else {
                    text
                },
            });
            last = m.get(0).map(|g| g.end()).unwrap_or(0);
        }
        let after = line[last..].trim();
        if !after.is_empty() {
            out.push(Block::Paragraph {
                runs: parse_inline(after, refs),
                indent: 0,
                size: None,
            });
        }
    }
}

fn parse_align(cell: &str) -> Align {
    let left = cell.starts_with(':');
    let right = cell.ends_with(':');
    if left && right {
        return Align::Center;
    }
    if right {
        return Align::Right;
    }
    Align::Left
}

pub fn parse_markdown(md: &str) -> ParsedMarkdown {
    let normalized = md.replace("\r\n", "\n").replace('\t', "    ");
    let lines: Vec<&str> = normalized.split('\n').collect();

    let mut refs: HashMap<String, String> = HashMap::new();
    for line in &lines {
        if let Some(c) = ref_def_re().captures(line.trim()) {
            let url = c
                .get(2)
                .or_else(|| c.get(3))
                .map(|g| g.as_str().to_string())
                .unwrap_or_default();
            refs.insert(c[1].to_lowercase(), url);
        }
    }

    let outline = build_outline(&lines);

    let mut parsed = ParsedMarkdown::default();
    let mut list_items: Vec<String> = Vec::new();
    let mut i = 0usize;

    while i < lines.len() {
        let trimmed = lines[i].trim();

        if let Some(c) = ref_def_re().captures(trimmed) {
            flush_list(&mut list_items, &mut parsed.blocks);
            let url = c
                .get(2)
                .or_else(|| c.get(3))
                .map(|g| g.as_str().to_string())
                .unwrap_or_default();
            refs.insert(c[1].to_lowercase(), url);
            i += 1;
            continue;
        }

        if let Some(c) = footnote_re().captures(trimmed) {
            flush_list(&mut list_items, &mut parsed.blocks);
            parsed.blocks.push(Block::Paragraph {
                runs: vec![
                    Run {
                        text: format!("{}. ", &c[1]),
                        bold: true,
                        ..Default::default()
                    },
                    Run {
                        text: c[2].to_string(),
                        ..Default::default()
                    },
                ],
                indent: 0,
                size: Some(9),
            });
            i += 1;
            continue;
        }

        if toc_re().is_match(trimmed) {
            flush_list(&mut list_items, &mut parsed.blocks);
            if !outline.is_empty() {
                parsed.blocks.push(Block::Heading {
                    text: "Sumário".to_string(),
                    level: 1,
                });
                for (text, level) in &outline {
                    parsed.blocks.push(Block::Paragraph {
                        runs: vec![Run {
                            text: text.clone(),
                            link: Some(format!("#{}", slug(text))),
                            ..Default::default()
                        }],
                        indent: level.saturating_sub(1),
                        size: None,
                    });
                }
            }
            i += 1;
            continue;
        }

        if fence_re().is_match(trimmed) {
            flush_list(&mut list_items, &mut parsed.blocks);
            let language = trimmed
                .trim_start_matches('`')
                .trim()
                .split_whitespace()
                .next()
                .map(|s| s.to_string());
            let mut code_lines: Vec<String> = Vec::new();
            i += 1;
            while i < lines.len() && lines[i].trim() != "```" {
                code_lines.push(lines[i].to_string());
                i += 1;
            }
            i += 1;
            parsed.blocks.push(Block::CodeBlock {
                language,
                text: code_lines.join("\n"),
            });
            continue;
        }

        if let Some(c) = heading_re().captures(trimmed) {
            flush_list(&mut list_items, &mut parsed.blocks);
            let level = c[1].chars().count();
            let text = c[2].trim().to_string();
            if level == 1 && parsed.title.is_none() {
                parsed.title = Some(text);
                i += 1;
                continue;
            }
            if level == 2 && parsed.subtitle.is_none() && parsed.title.is_none() {
                parsed.subtitle = Some(text);
                i += 1;
                continue;
            }
            parsed.blocks.push(Block::Heading { text, level });
            i += 1;
            continue;
        }

        if hr_re().is_match(trimmed) {
            flush_list(&mut list_items, &mut parsed.blocks);
            parsed.blocks.push(Block::Break);
            i += 1;
            continue;
        }

        let next_line = lines.get(i + 1).copied().unwrap_or("");
        if trimmed.contains('|') && is_table_sep(next_line) {
            let columns = split_row(trimmed);
            let aligns: Vec<Align> = split_row(next_line.trim())
                .iter()
                .map(|s| parse_align(s))
                .collect();
            i += 2;
            let mut rows: Vec<Vec<String>> = Vec::new();
            while i < lines.len() && lines[i].trim().contains('|') && !fence_re().is_match(lines[i].trim()) {
                rows.push(split_row(lines[i].trim()));
                i += 1;
            }
            parsed.blocks.push(Block::Table {
                columns,
                rows,
                align: aligns,
            });
            continue;
        }

        if let Some(c) = list_re().captures(trimmed) {
            list_items.push(c[2].to_string());
            i += 1;
            continue;
        }

        if trimmed.starts_with('>') {
            flush_list(&mut list_items, &mut parsed.blocks);
            let mut quote_lines: Vec<String> = Vec::new();
            while i < lines.len() && lines[i].trim().starts_with('>') {
                let l = lines[i].trim().trim_start_matches('>').trim_start();
                quote_lines.push(l.to_string());
                i += 1;
            }
            parsed.blocks.push(Block::Blockquote { items: quote_lines });
            continue;
        }

        if let Some(rest) = trimmed.strip_prefix(": ") {
            flush_list(&mut list_items, &mut parsed.blocks);
            parsed.blocks.push(Block::Paragraph {
                runs: parse_inline(rest, &refs),
                indent: 0,
                size: Some(10),
            });
            i += 1;
            continue;
        }

        if i + 1 < lines.len()
            && !trimmed.starts_with(['#', ':', '>', '`', '-', '*', '+'])
            && !trimmed.starts_with(|c: char| c.is_ascii_digit())
            && lines[i + 1].trim().starts_with(": ")
        {
            flush_list(&mut list_items, &mut parsed.blocks);
            let term = trimmed.to_string();
            i += 1;
            let mut defs: Vec<String> = Vec::new();
            while i < lines.len() && lines[i].trim().starts_with(": ") {
                defs.push(lines[i].trim()[2..].to_string());
                i += 1;
            }
            parsed.blocks.push(Block::Definition {
                text: term,
                items: defs,
            });
            continue;
        }

        if trimmed.is_empty() {
            flush_list(&mut list_items, &mut parsed.blocks);
            i += 1;
            continue;
        }

        flush_list(&mut list_items, &mut parsed.blocks);
        let mut para_lines: Vec<String> = Vec::new();
        while i < lines.len() {
            let t = lines[i].trim();
            if t.is_empty() {
                break;
            }
            if heading_re().is_match(t) {
                break;
            }
            if fence_re().is_match(t) {
                break;
            }
            if hr_re().is_match(t) {
                break;
            }
            if list_re().is_match(t) {
                break;
            }
            let nxt = lines.get(i + 1).copied().unwrap_or("");
            if t.contains('|') && is_table_sep(nxt) {
                break;
            }
            para_lines.push(t.to_string());
            i += 1;
        }
        emit_paragraph(&para_lines, &mut parsed.blocks, &refs);
    }

    flush_list(&mut list_items, &mut parsed.blocks);
    parsed
}

fn build_outline(lines: &[&str]) -> Vec<(String, usize)> {
    let mut outline: Vec<(String, usize)> = Vec::new();
    let mut in_code = false;
    let mut title_seen = false;
    let mut sub_seen = false;
    for raw in lines {
        let tl = raw.trim();
        if fence_re().is_match(tl) {
            in_code = !in_code;
            continue;
        }
        if in_code {
            continue;
        }
        let c = match heading_re().captures(tl) {
            Some(c) => c,
            None => continue,
        };
        let level = c[1].chars().count();
        let text = c[2].trim().to_string();
        let mut is_title = false;
        let mut is_sub = false;
        if level == 1 && !title_seen {
            is_title = true;
            title_seen = true;
        }
        if level == 2 && !sub_seen && !title_seen {
            is_sub = true;
            sub_seen = true;
        }
        if !is_title && !is_sub {
            outline.push((text, level));
        }
    }
    outline
}

fn flush_list(items: &mut Vec<String>, blocks: &mut Vec<Block>) {
    if items.is_empty() {
        return;
    }
    blocks.push(Block::List {
        items: items.clone(),
    });
    items.clear();
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn slugifies() {
        assert_eq!(slug("Olá Mundo!"), "olá-mundo");
    }

    #[test]
    fn parses_title_and_table() {
        let md = parse_markdown("# Título\n\n| a | b |\n| --- | --- |\n| 1 | 2 |");
        assert_eq!(md.title.as_deref(), Some("Título"));
        let table = md
            .blocks
            .iter()
            .find_map(|b| match b {
                Block::Table { columns, rows, .. } => Some((columns, rows)),
                _ => None,
            })
            .expect("table block");
        assert_eq!(table.0, vec!["a", "b"]);
        assert_eq!(table.1, vec![vec!["1", "2"]]);
    }

    #[test]
    fn parses_inline_styles() {
        let refs = HashMap::new();
        let runs = parse_inline("**b** *i* `c` ~~s~~", &refs);
        assert!(runs.iter().any(|r| r.bold));
        assert!(runs.iter().any(|r| r.italic));
        assert!(runs.iter().any(|r| r.code));
        assert!(runs.iter().any(|r| r.strike));
    }
}
