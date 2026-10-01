use std::collections::BTreeSet;

use quick_xml::events::Event;
use quick_xml::Reader;
use regex::Regex;
use serde_json::Value;

pub const TABULAR_EXTS: &[&str] = &[".csv", ".tsv", ".json", ".xml", ".html", ".htm"];

pub fn is_tabular(ext: &str) -> bool {
    TABULAR_EXTS.contains(&ext.to_lowercase().as_str())
}

fn clean_cell(raw: &str) -> String {
    raw.replace('|', "¦")
        .replace("\r\n", " ")
        .replace('\n', " ")
        .replace('\r', " ")
        .trim()
        .to_string()
}

fn md_table(columns: &[String], rows: &[Vec<String>]) -> String {
    let head_cells: Vec<String> = columns.iter().map(|c| clean_cell(c)).collect();
    let mut lines = Vec::new();
    lines.push(format!("| {} |", head_cells.join(" | ")));
    let seps: Vec<&str> = columns.iter().map(|_| "---").collect();
    lines.push(format!("| {} |", seps.join(" | ")));
    for row in rows {
        let cells: Vec<String> = (0..columns.len())
            .map(|i| clean_cell(row.get(i).map(|s| s.as_str()).unwrap_or("")))
            .collect();
        lines.push(format!("| {} |", cells.join(" | ")));
    }
    lines.join("\n")
}

fn parse_csv(text: &str) -> Vec<Vec<String>> {
    let mut reader = csv::ReaderBuilder::new()
        .flexible(true)
        .has_headers(false)
        .from_reader(text.as_bytes());
    let mut rows: Vec<Vec<String>> = Vec::new();
    for record in reader.records() {
        let record = match record {
            Ok(r) => r,
            Err(_) => continue,
        };
        let row: Vec<String> = record.iter().map(|s| s.to_string()).collect();
        if row.iter().any(|c| !c.is_empty()) {
            rows.push(row);
        }
    }
    rows
}

fn parse_tsv(text: &str) -> Vec<Vec<String>> {
    text.lines()
        .map(|l| l.split('\t').map(|s| s.to_string()).collect())
        .filter(|r: &Vec<String>| r.iter().any(|c| !c.is_empty()))
        .collect()
}

fn json_cell(v: &Value) -> String {
    match v {
        Value::Null => String::new(),
        Value::String(s) => s.clone(),
        other => other.to_string(),
    }
}

fn parse_json_table(text: &str) -> String {
    let data: Value = match serde_json::from_str(text) {
        Ok(v) => v,
        Err(_) => return String::new(),
    };
    match &data {
        Value::Array(items) => {
            if items.is_empty() {
                return String::new();
            }
            let first = &items[0];
            if first.is_object() {
                let mut seen = BTreeSet::new();
                for item in items {
                    if let Value::Object(obj) = item {
                        for k in obj.keys() {
                            seen.insert(k.clone());
                        }
                    }
                }
                let columns: Vec<String> = seen.into_iter().collect();
                let rows: Vec<Vec<String>> = items
                    .iter()
                    .map(|item| {
                        columns
                            .iter()
                            .map(|c| match item.get(c) {
                                Some(v) => json_cell(v),
                                None => String::new(),
                            })
                            .collect()
                    })
                    .collect();
                return md_table(&columns, &rows);
            }
            if first.is_array() {
                let width = items
                    .iter()
                    .filter_map(|i| i.as_array().map(|a| a.len()))
                    .max()
                    .unwrap_or(0);
                let columns: Vec<String> = (0..width).map(|i| format!("col{}", i + 1)).collect();
                let rows: Vec<Vec<String>> = items
                    .iter()
                    .map(|item| match item.as_array() {
                        Some(arr) => arr.iter().map(json_cell).collect(),
                        None => vec![json_cell(item)],
                    })
                    .collect();
                return md_table(&columns, &rows);
            }
            let rows: Vec<Vec<String>> = items.iter().map(|v| vec![json_cell(v)]).collect();
            md_table(&["value".to_string()], &rows)
        }
        Value::Object(obj) => {
            let columns: Vec<String> = obj.keys().cloned().collect();
            let row: Vec<String> = columns
                .iter()
                .map(|c| match obj.get(c) {
                    Some(v) => json_cell(v),
                    None => String::new(),
                })
                .collect();
            md_table(&columns, &[row])
        }
        _ => String::new(),
    }
}

fn strip_tags(html: &str) -> String {
    let br = Regex::new(r"(?i)<br\s*/?>").expect("br");
    let tag = Regex::new(r"<[^>]+>").expect("tag");
    let mut s = br.replace_all(html, "\n").into_owned();
    s = tag.replace_all(&s, "").into_owned();
    s = s.replace("&nbsp;", " ");
    s = s.replace("&amp;", "&");
    s = s.replace("&lt;", "<");
    s = s.replace("&gt;", ">");
    s = s.replace("&quot;", "\"");
    s = s.replace("&#39;", "'");
    s.trim().to_string()
}

fn parse_html_tables(text: &str) -> Vec<String> {
    let table_re = Regex::new(r"(?is)<table[\s\S]*?</table>").expect("table re");
    let tr_re = Regex::new(r"(?is)<tr[\s\S]*?</tr>").expect("tr re");
    let cell_re = Regex::new(r#"(?is)<(th|td)[^>]*>([\s\S]*?)</\1>"#).expect("cell re");
    let th_re = Regex::new(r"(?i)<th").expect("th re");

    let mut tables = Vec::new();
    for table in table_re.find_iter(text) {
        let block = table.as_str();
        let mut rows: Vec<Vec<String>> = Vec::new();
        let mut header: Option<Vec<String>> = None;
        for tr in tr_re.find_iter(block) {
            let tr_text = tr.as_str();
            let cells: Vec<String> = cell_re
                .captures_iter(tr_text)
                .map(|c| strip_tags(&c[2]))
                .collect();
            if cells.is_empty() {
                continue;
            }
            if th_re.is_match(tr_text) && header.is_none() {
                header = Some(cells);
                continue;
            }
            rows.push(cells);
        }
        match header {
            Some(h) if !h.is_empty() => tables.push(md_table(&h, &rows)),
            _ => {
                if let Some(first) = rows.first() {
                    let head = first.clone();
                    let body: Vec<Vec<String>> = rows[1..].to_vec();
                    tables.push(md_table(&head, &body));
                }
            }
        }
    }
    tables
}

fn parse_xml_table(text: &str) -> String {
    let tag_re = Regex::new(r"<(/?)([A-Za-z_][\w.\-]*)[^>]*>").expect("xml tag re");
    let mut counts: std::collections::HashMap<String, usize> = std::collections::HashMap::new();
    for caps in tag_re.captures_iter(text) {
        if &caps[1] == "/" {
            continue;
        }
        *counts.entry(caps[2].to_string()).or_insert(0) += 1;
    }
    let mut row_tag = String::new();
    let mut best = 1usize;
    for (tag, n) in counts {
        if n > best {
            best = n;
            row_tag = tag;
        }
    }
    if row_tag.is_empty() {
        return String::new();
    }
    match parse_xml_rows(text, &row_tag) {
        Some((columns, rows)) => md_table(&columns, &rows),
        None => String::new(),
    }
}

fn parse_xml_rows(text: &str, row_tag: &str) -> Option<(Vec<String>, Vec<Vec<String>>)> {
    let mut reader = Reader::from_str(text);
    reader.config_mut().trim_text(true);

    let mut columns: BTreeSet<String> = BTreeSet::new();
    let mut rows: Vec<Vec<String>> = Vec::new();
    let mut in_row = false;
    let mut depth_in_row = 0usize;
    let mut current: Vec<(String, String)> = Vec::new();
    let mut stack: Vec<String> = Vec::new();
    let mut key: Option<String> = None;
    let mut buf = String::new();

    loop {
        match reader.read_event() {
            Ok(Event::Eof) => break,
            Ok(Event::Start(e)) => {
                let name = String::from_utf8_lossy(e.name().as_ref()).to_string();
                if name == row_tag && depth_in_row == 0 {
                    in_row = true;
                    current.clear();
                }
                if in_row {
                    depth_in_row += 1;
                    if depth_in_row == 2 {
                        key = Some(name.clone());
                        buf.clear();
                        let attrs = e.attributes();
                        for a in attrs.flatten() {
                            if let Ok(attr_name) = std::str::from_utf8(a.key.as_ref()) {
                                if attr_name != "xmlns" {
                                    columns.insert(attr_name.to_string());
                                    let value = a
                                        .unescape_value()
                                        .map(|v| v.to_string())
                                        .unwrap_or_default();
                                    current.push((attr_name.to_string(), value));
                                }
                            }
                        }
                    }
                }
                stack.push(name);
            }
            Ok(Event::Text(t)) => {
                if in_row && depth_in_row >= 2 {
                    buf.push_str(&t.unescape().unwrap_or_default());
                }
            }
            Ok(Event::End(e)) => {
                let name = String::from_utf8_lossy(e.name().as_ref()).to_string();
                stack.pop();
                if in_row {
                    if depth_in_row == 2 {
                        if let Some(k) = key.take() {
                            let value = strip_tags(buf.trim());
                            columns.insert(k.clone());
                            current.push((k, value));
                        }
                        buf.clear();
                    }
                    depth_in_row -= 1;
                    if depth_in_row == 0 && name == row_tag {
                        in_row = false;
                        let col_list: Vec<String> = columns.iter().cloned().collect();
                        if !col_list.is_empty() {
                            let row: Vec<String> = col_list
                                .iter()
                                .map(|c| {
                                    current
                                        .iter()
                                        .find(|(k, _)| k == c)
                                        .map(|(_, v)| v.clone())
                                        .unwrap_or_default()
                                })
                                .collect();
                            rows.push(row);
                        }
                    }
                }
            }
            Err(_) => break,
            _ => {}
        }
    }

    if rows.is_empty() || columns.is_empty() {
        return None;
    }
    Some((columns.into_iter().collect(), rows))
}

pub fn tabular_to_markdown(bytes: &[u8], ext: &str) -> String {
    let text = String::from_utf8_lossy(bytes);
    let e = ext.to_lowercase();
    match e.as_str() {
        ".csv" => {
            let rows = parse_csv(&text);
            match rows.split_first() {
                Some((head, body)) => md_table(head, body),
                None => String::new(),
            }
        }
        ".tsv" => {
            let rows = parse_tsv(&text);
            match rows.split_first() {
                Some((head, body)) => md_table(head, body),
                None => String::new(),
            }
        }
        ".json" => parse_json_table(&text),
        ".html" | ".htm" => parse_html_tables(&text).join("\n\n"),
        ".xml" => parse_xml_table(&text),
        _ => String::new(),
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn csv_becomes_table() {
        let md = tabular_to_markdown(b"a,b\n1,2\n", ".csv");
        assert!(md.starts_with("| a | b |"));
        assert!(md.contains("| 1 | 2 |"));
    }

    #[test]
    fn json_array_of_objects_becomes_table() {
        let md = tabular_to_markdown(br#"[{"a":1,"b":2}]"#, ".json");
        assert!(md.contains("a"));
        assert!(md.contains("b"));
    }

    #[test]
    fn jsonl_extension_is_tabular() {
        assert!(is_tabular(".tsv"));
        assert!(!is_tabular(".docx"));
    }
}
