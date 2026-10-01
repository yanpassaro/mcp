use crate::markdown::{Block, ParsedMarkdown, Run};

pub fn runs_text(runs: &[Run]) -> String {
    runs.iter().map(|r| r.text.as_str()).collect()
}

pub fn pdf_safe_text(text: &str) -> String {
    let mut s: String = text
        .chars()
        .filter(|c| {
            !matches!(
                c,
                '\u{FE0E}' | '\u{FE0F}' | '\u{200D}' | '\u{1F3FB}'..='\u{1F3FF}'
            )
        })
        .collect();
    for (emoji, glyph) in [
        ("✅", "✔"),
        ("❌", "✗"),
        ("❤", "♥"),
        ("❤️", "♥"),
        ("💜", "♥"),
        ("💙", "♥"),
        ("⭐", "★"),
        ("🌟", "★"),
        ("✨", "★"),
    ] {
        if s.contains(emoji) {
            s = s.replace(emoji, glyph);
        }
    }
    s
}

pub fn pdf_safe_parsed(parsed: &ParsedMarkdown) -> ParsedMarkdown {
    let mut out = parsed.clone();
    out.title = parsed.title.as_deref().map(pdf_safe_text);
    out.subtitle = parsed.subtitle.as_deref().map(pdf_safe_text);
    out.blocks = parsed.blocks.iter().map(pdf_safe_block).collect();
    out
}

fn pdf_safe_block(block: &Block) -> Block {
    match block {
        Block::Image { .. } => block.clone(),
        Block::CodeBlock { language, text } => Block::CodeBlock {
            language: language.clone(),
            text: text.clone(),
        },
        Block::Heading { text, level } => Block::Heading {
            text: pdf_safe_text(text),
            level: *level,
        },
        Block::Paragraph { runs, indent, size } => Block::Paragraph {
            runs: runs
                .iter()
                .map(|r| Run {
                    text: pdf_safe_text(&r.text),
                    ..r.clone()
                })
                .collect(),
            indent: *indent,
            size: *size,
        },
        Block::List { items } => Block::List {
            items: items.iter().map(|i| pdf_safe_text(i)).collect(),
        },
        Block::Blockquote { items } => Block::Blockquote {
            items: items.iter().map(|i| pdf_safe_text(i)).collect(),
        },
        Block::Table { columns, rows, align } => Block::Table {
            columns: columns.iter().map(|c| pdf_safe_text(c)).collect(),
            rows: rows
                .iter()
                .map(|r| r.iter().map(|c| pdf_safe_text(c)).collect())
                .collect(),
            align: align.clone(),
        },
        Block::Definition { text, items } => Block::Definition {
            text: pdf_safe_text(text),
            items: items.iter().map(|i| pdf_safe_text(i)).collect(),
        },
        Block::Break => Block::Break,
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn replaces_emoji_with_printable_glyphs() {
        assert_eq!(pdf_safe_text("ok ✅ fim"), "ok ✔ fim");
        assert_eq!(pdf_safe_text("a\u{FE0F}b"), "ab");
    }
}