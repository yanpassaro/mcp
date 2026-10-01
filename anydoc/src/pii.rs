use std::collections::HashMap;
use std::sync::OnceLock;

use regex::Regex;

fn label(entity: &str) -> String {
    let name = match entity {
        "CPF" => "CPF",
        "CNPJ" => "CNPJ",
        "ID" => "DOCUMENTO",
        "EMAIL" => "EMAIL",
        "PHONE" => "TELEFONE",
        "CEP" => "CEP",
        "ADDRESS" => "ENDEREÇO",
        "BANK" => "CONTA",
        "CARD" => "CARTÃO",
        "DATE" => "DATA",
        "SECRET" => "SEGREDO",
        "USER" => "USUÁRIO",
        "RG" => "RG",
        "URL" => "URL",
        "IP" => "IP",
        "MAC" => "MAC",
        "JWT" => "JWT",
        "HASH" => "HASH",
        "BTC" => "BTC",
        "CREDURL" => "URL",
        _ => "VALOR",
    };
    format!("[{}]", name)
}

fn digits_only(s: &str) -> String {
    s.chars().filter(|c| c.is_ascii_digit()).collect()
}

fn normalize_word(s: &str) -> String {
    s.to_lowercase()
        .split(|c: char| !c.is_alphanumeric())
        .collect::<Vec<_>>()
        .join("")
}

fn cpf_digit(sum: u32) -> u32 {
    let r = 11 - (sum % 11);
    if r >= 10 { 0 } else { r }
}

pub fn valid_cpf(doc: &str) -> bool {
    let d: Vec<u8> = digits_only(doc).bytes().collect();
    if d.len() != 11 {
        return false;
    }
    if d.iter().all(|&b| b == d[0]) {
        return false;
    }
    let mut sum = 0u32;
    for i in 0..9 {
        sum += u32::from(d[i] - b'0') * (10 - i as u32);
    }
    if cpf_digit(sum) != u32::from(d[9] - b'0') {
        return false;
    }
    let mut sum = 0u32;
    for i in 0..10 {
        sum += u32::from(d[i] - b'0') * (11 - i as u32);
    }
    cpf_digit(sum) == u32::from(d[10] - b'0')
}

fn cnpj_digit(sum: u32) -> u32 {
    let r = sum % 11;
    if r < 2 { 0 } else { 11 - r }
}

pub fn valid_cnpj(doc: &str) -> bool {
    let d: Vec<u8> = digits_only(doc).bytes().collect();
    if d.len() != 14 {
        return false;
    }
    if d.iter().all(|&b| b == d[0]) {
        return false;
    }
    let w1 = [5u32, 4, 3, 2, 9, 8, 7, 6, 5, 4, 3, 2];
    let mut sum = 0u32;
    for i in 0..12 {
        sum += u32::from(d[i] - b'0') * w1[i];
    }
    if cnpj_digit(sum) != u32::from(d[12] - b'0') {
        return false;
    }
    let w2 = [6u32, 5, 4, 3, 2, 9, 8, 7, 6, 5, 4, 3, 2];
    let mut sum = 0u32;
    for i in 0..13 {
        sum += u32::from(d[i] - b'0') * w2[i];
    }
    cnpj_digit(sum) == u32::from(d[13] - b'0')
}

fn luhn(s: &str) -> bool {
    let d: Vec<u8> = digits_only(s).bytes().collect();
    if d.len() < 12 {
        return false;
    }
    let mut sum = 0u32;
    let mut alt = false;
    for &b in d.iter().rev() {
        let mut n = u32::from(b - b'0');
        if alt {
            n *= 2;
            if n > 9 {
                n -= 9;
            }
        }
        sum += n;
        alt = !alt;
    }
    sum % 10 == 0
}

struct Rule {
    entity: &'static str,
    re: Regex,
    score: f32,
}

fn rules() -> &'static [Rule] {
    static RULES: OnceLock<Vec<Rule>> = OnceLock::new();
    RULES.get_or_init(|| {
        let mk = |entity: &'static str, pattern: &str, score: f32| Rule {
            entity,
            re: Regex::new(pattern).expect("pii rule"),
            score,
        };
        vec![
            mk("CPF", r"\b\d{3}\.?\d{3}\.?\d{3}-?\d{2}\b", 0.9),
            mk("CNPJ", r"\b\d{2}\.?\d{3}\.?\d{3}/?\d{4}-?\d{2}\b", 0.9),
            mk("EMAIL", r"\b[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}\b", 1.0),
            mk("PHONE", r"(?:\b|(?=\())(?:\+?55[\s.\-]?)?\(?\d{2}\)?[\s.\-]?9?\d{4}[\s.\-]?\d{4}\b", 0.8),
            mk("CEP", r"\b\d{5}-?\d{3}\b", 0.8),
            mk("RG", r"\b\d{1,2}\.?\d{3}\.?\d{3}-?[\dxX]\b", 0.45),
            mk("CARD", r"\b(?:\d{4}[ \-]?){3}\d{4}\b", 0.9),
            mk("IP", r"\b(?:\d{1,3}\.){3}\d{1,3}\b", 0.7),
            mk("MAC", r"\b(?:[0-9a-fA-F]{2}[:-]){5}[0-9a-fA-F]{2}\b", 1.0),
            mk("JWT", r"\beyJ[A-Za-z0-9_\-]+\.eyJ[A-Za-z0-9_\-]+\.[A-Za-z0-9_\-]+\b", 1.0),
            mk("HASH", r"\b0x[a-fA-F0-9]{40}\b", 1.0),
            mk("BTC", r"\b(?:bc1|[13])[a-zA-HJ-NP-Z0-9]{25,39}\b", 1.0),
            mk("URL", r#"\b[a-zA-Z][a-zA-Z0-9+.\-]*://[^"'<>]+"#, 1.0),
            mk("URL", r#"\bwww\.[^"'<>]+"#, 1.0),
            mk("URL", r#"\b(?:[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?\.)+[a-zA-Z]{2,}(?::\d{1,5})?(?:/[^"'<>]*)?(?![a-zA-Z0-9(])"#, 0.95),
        ]
    })
}

fn address_re() -> &'static Regex {
    static RE: OnceLock<Regex> = OnceLock::new();
    RE.get_or_init(|| {
        Regex::new(
            r"(?i)\b(?:rua|r\.|av\.|avenida|travessa|alameda|estrada|rodovia|praça|pca\.|pça\.|beco|largo|viela|condomínio|condominio|conjunto|residencial|loteamento|chácara|chacara|sítio|sitio|fazenda)\s+(?!(?:em|no|na|nos|nas|e|a|o|com|para|por|sobre)\s)[a-z0-9á-ú.\-']+(?:\s+[a-z0-9á-ú.\-']+){0,4}(?:[,\s]+\d{1,6}(?:[-\s/]\d{1,5})?)?(?:,\s+[A-ZÀ-Ú][a-zà-ú]+(?:\s+[A-ZÀ-Ú][a-zà-ú]+)*)?",
        )
        .expect("address re")
    })
}

fn redact_text(md: &str) -> String {
    let mut out = md.to_string();
    for rule in rules() {
        if rule.score < 0.5 {
            continue;
        }
        out = rule
            .re
            .replace_all(&out, |caps: &regex::Captures| {
                let m = &caps[0];
                match rule.entity {
                    "CPF" if !valid_cpf(m) => m.to_string(),
                    "CNPJ" if !valid_cnpj(m) => m.to_string(),
                    "CARD" if !luhn(m) => m.to_string(),
                    _ => label(rule.entity),
                }
            })
            .into_owned();
    }
    out
}

fn redact_addresses(md: &str) -> String {
    address_re()
        .replace_all(md, "[ENDEREÇO]")
        .into_owned()
}

const COLUMN_GROUPS: &[(&[&str], &str)] = &[
    (&["cpf"], "CPF"),
    (&["cnpj"], "CNPJ"),
    (
        &[
            "rg", "cnh", "renavam", "nis", "pis", "pasep", "titulo de eleitor", "passaporte",
            "inscricao estadual", "inscricao municipal", "identidade", "passport", "ssn", "identity",
            "tax id", "taxid", "tax payer id", "taxpayer id", "voter id", "national id", "id card",
            "id number", "drivers license", "driver license", "license number", "license plate",
            "plate", "doc number", "document number", "enrollment", "registration",
        ],
        "ID",
    ),
    (
        &[
            "email", "e-mail", "email principal", "email contato", "email corporativo", "mail",
            "mail address", "email address", "contact email", "official email",
        ],
        "EMAIL",
    ),
    (
        &[
            "telefone", "fone", "fone fixo", "telefone fixo", "telefone principal", "celular",
            "celular principal", "whatsapp", "phone", "telephone", "phone number", "contact number",
            "mobile", "mobile number", "cellphone", "cell", "work phone", "home phone", "landline",
            "whatsapp number", "fax",
        ],
        "PHONE",
    ),
    (
        &[
            "endereco", "endereco completo", "logradouro", "rua", "avenida", "bairro", "distrito",
            "cidade", "estado", "uf", "pais", "cep", "codigo postal", "localizacao", "address",
            "street", "avenue", "neighborhood", "district", "city", "state", "country", "zip",
            "zipcode", "zip code", "postal code", "postcode", "street address", "postal address",
            "home address", "work address", "residence", "address line", "province", "region",
            "municipality", "county", "quarter", "zone", "ward", "location", "locality", "village",
        ],
        "ADDRESS",
    ),
    (
        &[
            "banco", "agencia", "conta", "conta corrente", "numero da conta", "bank", "bank account",
            "agency", "branch", "account", "checking account", "account number", "routing number",
            "sort code", "iban", "swift", "bic", "wire",
        ],
        "BANK",
    ),
    (
        &[
            "cvv", "validade", "cartao", "numero do cartao", "numero cartao", "expiry", "card",
            "card number", "cardholder", "cardholder name", "credit card", "creditcard",
            "debit card", "pan", "cvv2", "cvc", "security code", "verification code",
        ],
        "CARD",
    ),
    (
        &[
            "data de nascimento", "data nascimento", "nascimento", "data de aniversario",
            "birthdate", "birth date", "birth day", "birthday", "dob", "date of birth", "born",
        ],
        "DATE",
    ),
    (
        &[
            "senha", "chave", "token", "access token", "authorization", "auth", "api key", "api-key",
            "secret", "bearer", "password", "passwd", "pwd", "pass", "private key", "secret key",
            "client secret", "credential", "credentials", "session", "session id", "session_id",
            "cookie", "csrf", "otp", "2fa", "pin", "access key", "api token", "refresh token",
            "recovery code",
        ],
        "SECRET",
    ),
    (
        &[
            "usuario", "login", "user", "username", "user id", "userid", "screen name", "handle",
            "account name",
        ],
        "USER",
    ),
];

fn column_entity() -> &'static HashMap<String, &'static str> {
    static MAP: OnceLock<HashMap<String, &'static str>> = OnceLock::new();
    MAP.get_or_init(|| {
        let mut m = HashMap::new();
        for (names, entity) in COLUMN_GROUPS {
            for name in *names {
                m.insert(normalize_word(name), *entity);
            }
        }
        m
    })
}

fn field_re() -> &'static Regex {
    static RE: OnceLock<Regex> = OnceLock::new();
    RE.get_or_init(|| {
        let mut names: Vec<&str> = Vec::new();
        for (group, _) in COLUMN_GROUPS {
            names.extend(*group);
        }
        names.sort_by(|a, b| b.len().cmp(&a.len()));
        let pattern = format!(
            r"(?i)\b({})\s*[:=]\s*([^\n|,;]+)",
            names.join("|")
        );
        Regex::new(&pattern).expect("field re")
    })
}

fn is_separator(line: &str) -> bool {
    if !line.contains('|') {
        return false;
    }
    let cells: Vec<&str> = line
        .split('|')
        .map(|c| c.trim())
        .filter(|c| !c.is_empty())
        .collect();
    if cells.is_empty() {
        return false;
    }
    cells.iter().all(|c| {
        let core = c.trim_start_matches(':').trim_end_matches(':');
        !core.is_empty() && core.chars().all(|ch| ch == '-')
    })
}

fn redact_tables(md: &str) -> String {
    let lines: Vec<&str> = md.split('\n').collect();
    let mut out: Vec<String> = Vec::new();
    let mut i = 0;
    while i < lines.len() {
        let line = lines[i];
        let next = lines.get(i + 1).copied().unwrap_or("");
        if line.contains('|') && is_separator(next) {
            let header_cells: Vec<String> = line
                .trim()
                .trim_start_matches('|')
                .trim_end_matches('|')
                .split('|')
                .map(|c| c.trim().to_string())
                .collect();
            let entity_by_col: Vec<Option<&'static str>> = header_cells
                .iter()
                .map(|c| column_entity().get(&normalize_word(c)).copied())
                .collect();
            out.push(line.to_string());
            out.push(next.to_string());
            i += 2;
            while i < lines.len() && lines[i].contains('|') {
                let row = lines[i];
                let starts = row.trim_start().starts_with('|');
                let ends = row.trim_end().ends_with('|');
                let mut cells: Vec<&str> = row.trim().trim_matches('|').split('|').collect();
                if !starts {
                    cells.remove(0);
                }
                if !ends {
                    cells.pop();
                }
                let replaced: Vec<String> = cells
                    .iter()
                    .enumerate()
                    .map(|(idx, orig)| match entity_by_col.get(idx).copied().flatten() {
                        None => (*orig).to_string(),
                        Some(ent) => {
                            let left: String =
                                orig.chars().take_while(|c| c.is_whitespace()).collect();
                            let right: String =
                                orig.chars().rev().take_while(|c| c.is_whitespace()).collect();
                            format!("{}{}{}", left, label(ent), right)
                        }
                    })
                    .collect();
                let body = replaced.join("|");
                let line = if starts {
                    format!("|{}|", body)
                } else if ends {
                    format!("{}|", body)
                } else {
                    body
                };
                out.push(line);
                i += 1;
            }
            continue;
        }
        out.push(line.to_string());
        i += 1;
    }
    out.join("\n")
}

fn redact_fields(md: &str) -> String {
    field_re()
        .replace_all(md, |caps: &regex::Captures| {
            let key = &caps[1];
            let value = &caps[2];
            match column_entity().get(&normalize_word(key)).copied() {
                None => caps[0].to_string(),
                Some(ent) => {
                    let head_len = caps[0].len() - value.len();
                    format!("{}{}", &caps[0][..head_len], label(ent))
                }
            }
        })
        .into_owned()
}

fn redact_body(md: &str) -> String {
    let out = redact_tables(md);
    let out = redact_fields(&out);
    let out = redact_text(&out);
    redact_addresses(&out)
}

pub fn redact_pii(markdown: &str) -> String {
    let lines: Vec<&str> = markdown.split('\n').collect();
    let mut out: Vec<String> = Vec::new();
    let mut chunks: Vec<String> = Vec::new();
    let mut in_code = false;
    let mut fence = '\0';

    let flush = |chunks: &mut Vec<String>, out: &mut Vec<String>| {
        if chunks.is_empty() {
            return;
        }
        out.push(redact_body(&chunks.join("\n")));
        chunks.clear();
    };

    for line in lines {
        let trimmed = line.trim_start();
        let fence_char = trimmed.chars().next().filter(|c| *c == '`' || *c == '~');
        if let Some(c) = fence_char {
            let run_len = trimmed.chars().take_while(|ch| *ch == c).count();
            if !in_code || c == fence {
                if !in_code {
                    flush(&mut chunks, &mut out);
                }
                out.push(line.to_string());
                in_code = !in_code;
                fence = if in_code { c } else { '\0' };
                continue;
            }
            let _ = run_len;
        }
        if in_code {
            out.push(line.to_string());
        } else {
            chunks.push(line.to_string());
        }
    }
    flush(&mut chunks, &mut out);
    out.join("\n")
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn masks_cpf() {
        assert!(valid_cpf("529.982.247-25"));
        assert!(!valid_cpf("111.111.111-11"));
        assert!(redact_pii("cpf 529.982.247-25").contains("[CPF]"));
    }

    #[test]
    fn masks_email_and_keeps_code() {
        let out = redact_pii("a@b.com");
        assert!(out.contains("[EMAIL]"));
        let code = "```\na@b.com\n```";
        assert_eq!(redact_pii(code), code);
    }

    #[test]
    fn masks_table_cells_by_column() {
        let md = "| email | nome |\n| --- | --- |\n| a@b.com | Ana |";
        let out = redact_pii(md);
        assert!(out.contains("[EMAIL]"));
        assert!(out.contains("Ana"));
    }
}
