const LABELS: Record<string, string> = {
  CPF: "CPF",
  CNPJ: "CNPJ",
  ID: "DOCUMENTO",
  EMAIL: "EMAIL",
  PHONE: "TELEFONE",
  CEP: "CEP",
  ADDRESS: "ENDEREÇO",
  BANK: "CONTA",
  CARD: "CARTÃO",
  DATE: "DATA",
  SECRET: "SEGREDO",
  USER: "USUÁRIO",
  RG: "RG",
  URL: "URL",
  IP: "IP",
  MAC: "MAC",
  JWT: "JWT",
  HASH: "HASH",
  BTC: "BTC",
  CREDURL: "URL",
};

function label(entity: string): string {
  return LABELS[entity] ? `[${LABELS[entity]}]` : "[VALOR]";
}

function digitsOnly(s: string): string {
  return s.replace(/\D/g, "");
}

function normalizeWord(s: string): string {
  return s.toLowerCase().normalize("NFD").replace(/[\u0300-\u036f]/g, "");
}

function escapeRe(s: string): string {
  return s.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

function cpfDigit(sum: number): number {
  const r = 11 - (sum % 11);
  return r >= 10 ? 0 : r;
}

export function validCPF(doc: string): boolean {
  const d = digitsOnly(doc);
  if (d.length !== 11 || new Set(d).size === 1) return false;
  let sum = 0;
  for (let i = 0; i < 9; i++) sum += (d.charCodeAt(i) - 48) * (10 - i);
  if (cpfDigit(sum) !== d.charCodeAt(9) - 48) return false;
  sum = 0;
  for (let i = 0; i < 10; i++) sum += (d.charCodeAt(i) - 48) * (11 - i);
  return cpfDigit(sum) === d.charCodeAt(10) - 48;
}

function cnpjDigit(sum: number): number {
  const r = sum % 11;
  return r < 2 ? 0 : 11 - r;
}

export function validCNPJ(doc: string): boolean {
  const d = digitsOnly(doc);
  if (d.length !== 14 || new Set(d).size === 1) return false;
  const w1 = [5, 4, 3, 2, 9, 8, 7, 6, 5, 4, 3, 2];
  let sum = 0;
  for (let i = 0; i < 12; i++) sum += (d.charCodeAt(i) - 48) * w1[i];
  if (cnpjDigit(sum) !== d.charCodeAt(12) - 48) return false;
  const w2 = [6, 5, 4, 3, 2, 9, 8, 7, 6, 5, 4, 3, 2];
  sum = 0;
  for (let i = 0; i < 13; i++) sum += (d.charCodeAt(i) - 48) * w2[i];
  return cnpjDigit(sum) === d.charCodeAt(13) - 48;
}

function luhn(s: string): boolean {
  const d = digitsOnly(s);
  if (d.length < 12) return false;
  let sum = 0;
  let alt = false;
  for (let i = d.length - 1; i >= 0; i--) {
    let n = d.charCodeAt(i) - 48;
    if (alt) {
      n *= 2;
      if (n > 9) n -= 9;
    }
    sum += n;
    alt = !alt;
  }
  return sum % 10 === 0;
}

function maskAddress(_s: string): string {
  return "[ENDEREÇO]";
}

export const maskFull = (_: string) => "[VALOR]";

const RE_ADDRESS =
  /\b(?:rua|r\.|av\.|avenida|travessa|alameda|estrada|rodovia|praça|pca\.|pça\.|beco|largo|viela|condomínio|condominio|conjunto|residencial|loteamento|chácara|chacara|sítio|sitio|fazenda)\s+(?!(?:em|no|na|nos|nas|e|a|o|com|para|por|sobre)\s)[a-z0-9á-ú.\-']+(?:\s+[a-z0-9á-ú.\-']+){0,4}(?:[,\s]+\d{1,6}(?:[-\s/]\d{1,5})?)?(?:,\s+[A-ZÀ-Ú][a-zà-ú]+(?:\s+[A-ZÀ-Ú][a-zà-ú]+)*)?/gi;

interface Rule {
  entity: string;
  re: RegExp;
  score: number;
}

const RULES: Rule[] = [
  { entity: "CPF", re: /\b\d{3}\.?\d{3}\.?\d{3}-?\d{2}\b/g, score: 0.9 },
  {
    entity: "CNPJ",
    re: /\b\d{2}\.?\d{3}\.?\d{3}\/?\d{4}-?\d{2}\b/g,
    score: 0.9,
  },
  {
    entity: "EMAIL",
    re: /\b[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,}\b/gi,
    score: 1,
  },

  {
    entity: "PHONE",
    re:
      /(?:\b|(?=\())(?:\+?55[\s.\-]?)?\(?\d{2}\)?[\s.\-]?9?\d{4}[\s.\-]?\d{4}\b/g,
    score: 0.8,
  },
  { entity: "CEP", re: /\b\d{5}-?\d{3}\b/g, score: 0.8 },
  { entity: "RG", re: /\b\d{1,2}\.?\d{3}\.?\d{3}-?[\dxX]\b/gi, score: 0.45 },
  { entity: "CARD", re: /\b(?:\d{4}[ \-]?){3}\d{4}\b/g, score: 0.9 },

  { entity: "IP", re: /\b(?:\d{1,3}\.){3}\d{1,3}\b/g, score: 0.7 },
  { entity: "MAC", re: /\b(?:[0-9a-f]{2}[:-]){5}[0-9a-f]{2}\b/gi, score: 1 },
  {
    entity: "JWT",
    re: /\beyJ[A-Za-z0-9_\-]+\.eyJ[A-Za-z0-9_\-]+\.[A-Za-z0-9_\-]+\b/g,
    score: 1,
  },
  { entity: "HASH", re: /\b0x[a-fA-F0-9]{40}\b/g, score: 1 },
  { entity: "BTC", re: /\b(?:bc1|[13])[a-zA-HJ-NP-Z0-9]{25,39}\b/g, score: 1 },

  { entity: "URL", re: /\b[a-z][a-z0-9+.\-]*:\/\/[^\s"'<>]+/gi, score: 1 },
  { entity: "URL", re: /\bwww\.[^\s"'<>]+/gi, score: 1 },

  {
    entity: "URL",
    re:
      /\b(?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z]{2,}(?::\d{1,5})?(?:\/[^\s"'<>]*)?(?![a-z0-9(])/gi,
    score: 0.95,
  },
];

function maskSpan(entity: string, _value: string): string {
  return label(entity);
}

function redactText(md: string): string {
  let out = md;
  for (const rule of RULES) {
    if (rule.score < 0.5) continue;
    out = out.replace(rule.re, (m) => {
      if (rule.entity === "CPF" && !validCPF(m)) return m;
      if (rule.entity === "CNPJ" && !validCNPJ(m)) return m;
      if (rule.entity === "CARD" && !luhn(m)) return m;
      return maskSpan(rule.entity, m);
    });
  }
  return out;
}

function redactAddresses(md: string): string {
  return md.replace(RE_ADDRESS, (m) => maskAddress(m));
}

const COLUMN_GROUPS: Array<[string[], string]> = [
  [["cpf"], "CPF"],
  [["cnpj"], "CNPJ"],
  [[
    "rg",
    "cnh",
    "renavam",
    "nis",
    "pis",
    "pasep",
    "titulo de eleitor",
    "passaporte",
    "inscricao estadual",
    "inscricao municipal",
    "identidade",
    "passport",
    "ssn",
    "identity",
    "tax id",
    "taxid",
    "tax payer id",
    "taxpayer id",
    "voter id",
    "national id",
    "id card",
    "id number",
    "drivers license",
    "driver license",
    "license number",
    "license plate",
    "plate",
    "doc number",
    "document number",
    "enrollment",
    "registration",
  ], "ID"],
  [[
    "email",
    "e-mail",
    "email principal",
    "email contato",
    "email corporativo",
    "mail",
    "mail address",
    "email address",
    "contact email",
    "official email",
  ], "EMAIL"],
  [[
    "telefone",
    "fone",
    "fone fixo",
    "telefone fixo",
    "telefone principal",
    "celular",
    "celular principal",
    "whatsapp",
    "phone",
    "telephone",
    "phone number",
    "contact number",
    "mobile",
    "mobile number",
    "cellphone",
    "cell",
    "work phone",
    "home phone",
    "landline",
    "whatsapp number",
    "fax",
  ], "PHONE"],
  [[
    "endereco",
    "endereco completo",
    "logradouro",
    "rua",
    "avenida",
    "bairro",
    "distrito",
    "cidade",
    "estado",
    "uf",
    "pais",
    "cep",
    "codigo postal",
    "localizacao",
    "address",
    "street",
    "avenue",
    "neighborhood",
    "district",
    "city",
    "state",
    "country",
    "zip",
    "zipcode",
    "zip code",
    "postal code",
    "postcode",
    "street address",
    "postal address",
    "home address",
    "work address",
    "residence",
    "address line",
    "province",
    "region",
    "municipality",
    "county",
    "quarter",
    "zone",
    "ward",
    "location",
    "locality",
    "village",
  ], "ADDRESS"],
  [[
    "banco",
    "agencia",
    "conta",
    "conta corrente",
    "numero da conta",
    "bank",
    "bank account",
    "agency",
    "branch",
    "account",
    "checking account",
    "account number",
    "routing number",
    "sort code",
    "iban",
    "swift",
    "bic",
    "wire",
  ], "BANK"],
  [[
    "cvv",
    "validade",
    "cartao",
    "numero do cartao",
    "numero cartao",
    "expiry",
    "card",
    "card number",
    "cardholder",
    "cardholder name",
    "credit card",
    "creditcard",
    "debit card",
    "pan",
    "cvv2",
    "cvc",
    "security code",
    "verification code",
  ], "CARD"],
  [[
    "data de nascimento",
    "data nascimento",
    "nascimento",
    "data de aniversario",
    "birthdate",
    "birth date",
    "birth day",
    "birthday",
    "dob",
    "date of birth",
    "born",
  ], "DATE"],
  [[
    "senha",
    "chave",
    "token",
    "access token",
    "authorization",
    "auth",
    "api key",
    "api-key",
    "secret",
    "bearer",
    "password",
    "passwd",
    "pwd",
    "pass",
    "private key",
    "secret key",
    "client secret",
    "credential",
    "credentials",
    "session",
    "session id",
    "session_id",
    "cookie",
    "csrf",
    "otp",
    "2fa",
    "pin",
    "access key",
    "api token",
    "refresh token",
    "recovery code",
  ], "SECRET"],
  [[
    "usuario",
    "login",
    "user",
    "username",
    "user id",
    "userid",
    "screen name",
    "handle",
    "account name",
  ], "USER"],
];

const COLUMN_ENTITY = new Map<string, string>();
const FIELD_KEYS: string[] = [];
for (const [names, entity] of COLUMN_GROUPS) {
  for (const name of names) {
    COLUMN_ENTITY.set(normalizeWord(name), entity);
    FIELD_KEYS.push(name);
  }
}
FIELD_KEYS.sort((a, b) => b.length - a.length);

function maskForEntity(
  entity: string | null | undefined,
  value: string,
): string {
  if (!entity) return value;
  return label(entity);
}

function splitRow(
  line: string,
): { cells: string[]; starts: boolean; ends: boolean } {
  const starts = line.startsWith("|");
  const ends = line.endsWith("|");
  let cells = line.split("|");
  if (starts && cells[0].trim() === "") cells = cells.slice(1);
  if (ends && cells[cells.length - 1].trim() === "") cells = cells.slice(0, -1);
  return { cells, starts, ends };
}

function isSeparator(line: string): boolean {
  if (!line.includes("|")) return false;
  const cells = line.split("|").map((c) => c.trim()).filter((c) => c !== "");
  if (cells.length === 0) return false;
  return cells.every((c) => /^:?-+:?$/.test(c));
}

function joinRow(cells: string[], starts: boolean, ends: boolean): string {
  const body = cells.join("|");
  return starts ? `|${body}|` : ends ? `${body}|` : body;
}

function redactTables(md: string): string {
  const lines = md.split("\n");
  const out: string[] = [];
  let i = 0;
  while (i < lines.length) {
    const line = lines[i];
    const next = lines[i + 1] ?? "";
    if (line.includes("|") && isSeparator(next)) {
      const header = splitRow(line);
      const entityByCol = header.cells.map((c) =>
        COLUMN_ENTITY.get(normalizeWord(c.trim())) ?? null
      );
      out.push(line);
      out.push(next);
      i += 2;
      while (i < lines.length && lines[i].includes("|")) {
        const row = splitRow(lines[i]);
        const cells = row.cells.map((orig, idx) => {
          const ent = entityByCol[idx] ?? null;
          if (!ent) return orig;

          const left = orig.match(/^\s*/)?.[0] ?? "";
          const right = orig.match(/\s*$/)?.[0] ?? "";
          return left + maskForEntity(ent, orig.trim()) + right;
        });
        out.push(joinRow(cells, row.starts, row.ends));
        i++;
      }
    } else {
      out.push(line);
      i++;
    }
  }
  return out.join("\n");
}

let FIELD_RE: RegExp | null = null;

function redactFields(md: string): string {
  if (FIELD_RE === null) {
    const keys = FIELD_KEYS.map(escapeRe).join("|");
    FIELD_RE = new RegExp(`\\b(${keys})\\s*[:=]\\s*([^\\n|,;]+)`, "gi");
  }
  return md.replace(FIELD_RE, (full, key: string, value: string) => {
    const entity = COLUMN_ENTITY.get(normalizeWord(key));
    const masked = maskForEntity(entity, value.trim());
    return full.slice(0, full.length - value.length) + masked;
  });
}

function redactBody(md: string): string {
  let out = redactTables(md);
  out = redactFields(out);
  out = redactText(out);
  out = redactAddresses(out);
  return out;
}

export function redactPII(markdown: string): string {
  const lines = markdown.split("\n");
  const chunks: string[] = [];
  const out: string[] = [];
  let inCode = false;
  let fence = "";
  const flush = (): void => {
    if (chunks.length > 0) {
      out.push(redactBody(chunks.join("\n")));
      chunks.length = 0;
    }
  };
  for (const line of lines) {
    const m = line.match(/^\s*(```+|~~~+)/);
    if (m && (!inCode || m[1][0] === fence)) {
      if (!inCode) flush();
      out.push(line);
      inCode = !inCode;
      fence = inCode ? m[1][0] : "";
      continue;
    }
    if (inCode) out.push(line);
    else chunks.push(line);
  }
  flush();
  return out.join("\n");
}
