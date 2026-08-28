// Redator de PII do AnyDoc — port do redator do sqlize
// (sqlize/internal/sqlize/redact.go + names_br.go) para manter os dois MCPs
// pareados:
//   - entidades + score + checksum real (CPF/CNPJ/cartão só mascaram se válidos);
//   - contexto de coluna pt + en (tabelas markdown e "campo: valor");
//   - deny-list de nomes BR + nomes compostos em texto livre;
//   - qualquer URL vira PII; endereços por logradouro;
//   - datas genéricas NÃO são mascaradas (só colunas de data pessoal);
//   - e-mail vira "jo**@***.com" (domínio oculto, mantém a extensão).
//
// Datas: removidas do padrão (igual decisão do sqlize). Colunas como
// "data de nascimento"/"birthdate" continuam mascaradas via contexto de coluna.

export const MASK = "***";

// ---------------------------------------------------------------------------
// utilidades
// ---------------------------------------------------------------------------

function digitsOnly(s: string): string {
  return s.replace(/\D/g, "");
}

// normalizeWord baixa para minúsculas e remove acentos ("João" -> "joao",
// "Endereço" -> "endereco").
function normalizeWord(s: string): string {
  return s.toLowerCase().normalize("NFD").replace(/[\u0300-\u036f]/g, "");
}

function escapeRe(s: string): string {
  return s.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

// ---------------------------------------------------------------------------
// checksums (equivalente ao validate_result do Presidio)
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// máscaras (operadores por entidade, iguais ao sqlize)
// ---------------------------------------------------------------------------

function keepEdges(s: string): string {
  const d = digitsOnly(s);
  if (d.length < 4) return s;
  let sep = "";
  for (let i = s.length - 1; i >= 0; i--) {
    const c = s[i];
    if (!/\d/.test(c) && c !== " ") {
      sep = c;
      break;
    }
  }
  return d.slice(0, 2) + "*".repeat(d.length - 4) + sep + d.slice(-2);
}

// maskEmail mantém só o início do usuário e a extensão do domínio.
function maskEmail(s: string): string {
  const at = s.indexOf("@");
  if (at <= 0) return s;
  let local = s.slice(0, at);
  const rest = s.slice(at + 1);
  local = local.length === 0 ? "**" : local.length === 1 ? local + "**" : local.slice(0, 2) + "**";
  let dm = "***";
  const dot = rest.lastIndexOf(".");
  if (dot > 0 && dot < rest.length - 1) dm = "***" + rest.slice(dot);
  return local + "@" + dm;
}

function maskCard(s: string): string {
  const d = digitsOnly(s);
  if (d.length < 12) return s;
  return d.slice(0, 4) + "*".repeat(d.length - 8) + d.slice(-4);
}

function maskByName(s: string): string {
  const t = s.trim();
  if (!t) return s;
  const first = [...t][0];
  return /[a-zà-ú]/i.test(first) ? `${first}***` : MASK;
}

export const maskFull = (_: string) => MASK;

// ---------- deny-list de nomes BR (chaves normalizadas, sem acento) ----------

const brFirstNames = new Set<string>([
  "joao", "jose", "antonio", "francisco", "carlos", "paulo", "pedro", "lucas",
  "gabriel", "matheus", "rafael", "marcos", "marcelo", "bruno", "thiago",
  "felipe", "gustavo", "rodrigo", "fernando", "eduardo", "daniel", "leonardo",
  "ricardo", "andre", "henrique", "guilherme", "diego", "vinicius", "leandro",
  "renato", "alexandre", "fabio", "sergio", "rogerio", "mauricio", "jorge",
  "marcio", "everton", "anderson", "douglas", "wesley", "davi", "miguel",
  "arthur", "bernardo", "heitor", "theo", "enzo", "nicolas", "samuel",
  "benjamin", "joaquim", "lucca", "lorenzo", "anthony", "caua", "murilo",
  "pietro", "alan", "caio", "igor", "alex", "emerson", "elias", "gilberto",
  "hugo", "ivan", "julio", "kleber", "nilton", "roberto", "romulo", "sandro",
  "sebastiao", "valdir", "vitor", "wagner", "william", "yuri", "flavio",
  "gilmar", "gerson", "osmar", "valter", "rubens", "joel", "nilson", "edson",
  "edvaldo", "jefferson", "jairo", "jaime", "jeferson", "evandro", "eder",
  "elton", "ezequiel", "sidnei", "sidney", "olavo", "oswaldo", "otavio",
  "raimundo", "reinaldo", "saulo", "tadeu", "ulisses", "vanderlei",
  "washington", "maria", "ana", "juliana", "mariana", "fernanda", "camila",
  "larissa", "beatriz", "amanda", "patricia", "leticia", "vanessa",
  "carolina", "gabriela", "thais", "aline", "bruna", "jessica", "natalia",
  "bianca", "raquel", "renata", "sabrina", "luciana", "viviane", "sandra",
  "claudia", "adriana", "cintia", "daniela", "priscila", "taina", "yasmin",
  "isadora", "manuela", "helena", "alice", "sophia", "laura", "valentina",
  "rafaela", "milena", "isabela", "luana", "melissa", "nicole", "sarah",
  "karina", "monica", "simone", "eliane", "regina", "tereza", "aparecida",
  "deise", "elen", "heloisa", "ingrid", "joana", "katia", "lucia", "paula",
  "roberta", "sonia", "sueli", "valquiria", "vera", "vitoria", "fabiana",
  "nayara", "gabrielly", "gabriele", "julia", "giovanna", "melina",
  "michele", "michelle", "silvia", "solange", "tatiana", "tania", "tamires",
  "vania", "veronica", "zelia", "junior", "neto", "kevin", "emily",
]);

const NAME_STOPWORDS = new Set(["de", "da", "do", "dos", "das", "e", "a", "o"]);

// sequências de 2+ palavras iniciando com maiúscula (com preposições).
const RE_NAME_SPAN = /\p{Lu}\p{Ll}*(?:(?:\s+(?:de|da|do|dos|das|e)\s+|\s+)\p{Lu}\p{Ll}*)+/gu;

// Geográficos comuns (normalizados, sem acento) — estados, capitais e
// municípios BR + países — cujo nome contém palavras que também são nomes
// próprios ("São Paulo", "João Pessoa"...). Usados para NÃO mascarar cidades
// e países em texto livre; a lista cobre antes os casos que começam com nome
// próprio (ex.: "Paulo Afonso", "Vitória da Conquista"), porque spans
// começando por não-nome já são rejeitados pela regra do primeiro termo.
const GEO_DENY = new Set([
  // estados BR (normalizados)
  "acre", "alagoas", "amapa", "amazonas", "bahia", "ceara", "espirito santo",
  "goias", "maranhao", "mato grosso", "mato grosso do sul", "minas gerais",
  "para", "paraiba", "parana", "pernambuco", "piaui", "rio de janeiro",
  "rio grande do norte", "rio grande do sul", "rondonia", "roraima",
  "santa catarina", "sao paulo", "sergipe", "tocantins",
  "distrito federal",
  // capitais e principais municípios BR
  "aracaju", "belem", "belo horizonte", "boa vista", "brasilia",
  "campina grande", "campinas", "campo grande", "cuiaba", "curitiba",
  "florianopolis", "fortaleza", "goiania", "guarulhos", "joao pessoa",
  "macapa", "maceio", "manaus", "natal", "niteroi", "palmas",
  "paulo afonso", "porto alegre", "porto velho", "recife", "rio branco",
  "salvador", "santos", "sao bernardo do campo", "sao caetano do sul",
  "sao goncalo", "sao jose", "sao jose do rio preto", "sao jose dos campos",
  "sao luis", "sao vicente", "sorocaba", "teresina", "vitoria",
  "vitoria da conquista",
  // países/regiões
  "africa do sul", "arabia saudita", "coreia do norte", "coreia do sul",
  "costa do marfim", "emirados arabes", "estados unidos", "nova zelandia",
  "reino unido",
]);

function nameScore(span: string): number {
  const tokens = normalizeWord(span).split(/\s+/);
  let known = 0;
  for (const t of tokens) {
    if (NAME_STOPWORDS.has(t)) continue;
    if (brFirstNames.has(t)) known++;
  }
  if (known === 0) return 0;
  // a 1ª palavra precisa ser um nome conhecido: "São Paulo" (são+paulo),
  // "São José dos Campos" etc. começam com não-nome e não são personas.
  const first = tokens.find((t) => !NAME_STOPWORDS.has(t));
  if (!first || !brFirstNames.has(first)) return 0;
  // geográficos que começam com nome próprio ("João Pessoa", "Paulo Afonso").
  if (GEO_DENY.has(normalizeWord(span))) return 0;
  return known >= 2 || tokens.length >= 3 ? 0.9 : 0.85;
}

// endereços por logradouro ("Rua das Flores, 123"). A 1ª palavra após o
// logradouro não pode ser preposição de localização ("em", "no", "na"...) —
// "chácara em Paulo Afonso" é lugar+localidade, não o nome do logradouro;
// de/da/do/dos/das continuam aceitas ("Rua da Consolação").
// Com o bairro capitalizado opcional: "Av. Paulista 1000, Bela Vista".
const RE_ADDRESS =
  /\b(?:rua|r\.|av\.|avenida|travessa|alameda|estrada|rodovia|praça|pca\.|pça\.|beco|largo|viela|condomínio|condominio|conjunto|residencial|loteamento|chácara|chacara|sítio|sitio|fazenda)\s+(?!(?:em|no|na|nos|nas|e|a|o|com|para|por|sobre)\s)[a-z0-9á-ú.\-']+(?:\s+[a-z0-9á-ú.\-']+){0,4}(?:[,\s]+\d{1,6}(?:[-\s/]\d{1,5})?)?(?:,\s+[A-ZÀ-Ú][a-zà-ú]+(?:\s+[A-ZÀ-Ú][a-zà-ú]+)*)?/gi;

// ---------------------------------------------------------------------------
// regras de formato (entidade + regex + score; checksum decide CPF/CNPJ/cartão)
// ---------------------------------------------------------------------------

interface Rule {
  entity: string;
  re: RegExp;
  score: number;
}

const RULES: Rule[] = [
  { entity: "CPF", re: /\b\d{3}\.?\d{3}\.?\d{3}-?\d{2}\b/g, score: 0.9 },
  { entity: "CNPJ", re: /\b\d{2}\.?\d{3}\.?\d{3}\/?\d{4}-?\d{2}\b/g, score: 0.9 },
  { entity: "EMAIL", re: /\b[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,}\b/gi, score: 1 },
  // (?\b|(?=\() permite começar no "(" — senão "(11) 98765-4321" perderia o
  // parêntese e "(11*******-21" vazaria o "(".
  { entity: "PHONE", re: /(?:\b|(?=\())(?:\+?55[\s.\-]?)?\(?\d{2}\)?[\s.\-]?9?\d{4}[\s.\-]?\d{4}\b/g, score: 0.8 },
  { entity: "CEP", re: /\b\d{5}-?\d{3}\b/g, score: 0.8 },
  { entity: "RG", re: /\b\d{1,2}\.?\d{3}\.?\d{3}-?[\dxX]\b/gi, score: 0.45 },
  { entity: "CARD", re: /\b(?:\d{4}[ \-]?){3}\d{4}\b/g, score: 0.9 },
  // datas genéricas NÃO são mascaradas (igual sqlize); só colunas de data pessoal.
  { entity: "IP", re: /\b(?:\d{1,3}\.){3}\d{1,3}\b/g, score: 0.7 },
  { entity: "MAC", re: /\b(?:[0-9a-f]{2}[:-]){5}[0-9a-f]{2}\b/gi, score: 1 },
  { entity: "JWT", re: /\beyJ[A-Za-z0-9_\-]+\.eyJ[A-Za-z0-9_\-]+\.[A-Za-z0-9_\-]+\b/g, score: 1 },
  { entity: "HASH", re: /\b0x[a-fA-F0-9]{40}\b/g, score: 1 },
  { entity: "BTC", re: /\b(?:bc1|[13])[a-zA-HJ-NP-Z0-9]{25,39}\b/g, score: 1 },
  // qualquer URL vira PII (com esquema, www ou domínio puro)
  { entity: "URL", re: /\b[a-z][a-z0-9+.\-]*:\/\/[^\s"'<>]+/gi, score: 1 },
  { entity: "URL", re: /\bwww\.[^\s"'<>]+/gi, score: 1 },
  // domínio puro: exige TLD real + não seguido de "(", para não confundir
  // chamadas de método de código ("console.log(...)") com URLs.
  { entity: "URL", re: /\b(?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.)+(?:com\.br|net\.br|org\.br|gov\.br|edu\.br|com\.ar|com\.mx|com\.co|com\.uk|co\.uk|org\.uk|com\.au|co\.nz|com|net|org|gov|edu|info|biz|io|dev|app|ai|co|me|tv|site|online|store|tech|cloud|xyz|br|us|uk|ca|de|fr|es|pt|it|nl|ru|jp|cn|in|mx|ar|cl|au|za|kr|se|ch|at|be|pl)(?::\d{1,5})?(?:\/[^\s"'<>]*)?(?![a-z0-9(])/gi, score: 0.95 },      
];

// operadores por entidade nos spans (mesmo mapa do sqlize entityOps).
function maskSpan(entity: string, value: string): string {
  switch (entity) {
    case "CPF":
    case "CNPJ":
    case "ID":
    case "PHONE":
    case "CEP":
    case "RG":
    case "BANK":
      return keepEdges(value);
    case "CARD":
      return maskCard(value);
    case "EMAIL":
      return maskEmail(value);
    case "PERSON":
    case "ADDRESS":
    default:
      return maskByName(value);
  }
}

const FULL_TEXT_ENTITIES = new Set(["URL", "IP", "MAC", "JWT", "HASH", "BTC", "SECRET", "CREDURL"]);

function redactText(md: string): string {
  let out = md;
  for (const rule of RULES) {
    if (rule.score < 0.5) continue; // RG e afins só mascaram com coluna PII
    out = out.replace(rule.re, (m) => {
      if (rule.entity === "CPF" && !validCPF(m)) return m;
      if (rule.entity === "CNPJ" && !validCNPJ(m)) return m;
      if (rule.entity === "CARD" && !luhn(m)) return m;
      return FULL_TEXT_ENTITIES.has(rule.entity) ? MASK : maskSpan(rule.entity, m);
    });
  }
  return out;
}

function redactAddresses(md: string): string {
  return md.replace(RE_ADDRESS, (m) => maskByName(m));
}

function redactNames(md: string): string {
  return md.replace(RE_NAME_SPAN, (m) => {
    if (nameScore(m) <= 0) return m;
    // O regex engloba stopwords no início/fim do span ("O Joao Silva"); a
    // máscara deve preservá-las e cobrir só o núcleo nomeado ("O J***").
    const words = m.split(/\s+/);
    let start = 0;
    let end = words.length;
    while (start < end && NAME_STOPWORDS.has(normalizeWord(words[start]))) start++;
    while (end > start && NAME_STOPWORDS.has(normalizeWord(words[end - 1]))) end--;
    const core = words.slice(start, end).join(" ");
    const masked = maskByName(core);
    const head = words.slice(0, start).join(" ");
    const tail = words.slice(end).join(" ");
    return [head, masked, tail].filter(Boolean).join(" ");
  });
}

// ---------------------------------------------------------------------------
// contexto de coluna (pt + en), igual à buildColumnEntity do sqlize
// ---------------------------------------------------------------------------

const COLUMN_GROUPS: Array<[string[], string]> = [
  [[
    "nome", "nome completo", "nome da mãe", "nome da mae", "nome do pai",
    "nome social", "nome completo do cliente", "nome responsavel", "pessoa",
    "pessoas", "cliente", "consumidor", "responsavel", "responsavel legal",
    "contato", "contato principal", "proprietario", "funcionario",
    "colaborador", "atendente", "vendedor", "representante", "diretor",
    "gerente", "medico", "paciente", "aluno", "professor", "candidato",
    "solicitante", "beneficiario", "customer", "client", "full name",
    "first name", "firstname", "last name", "lastname", "surname",
    "given name", "person", "person name", "name of contact", "mothers name",
    "fathers name", "contact name", "contact person", "responsible", "owner",
    "proprietor", "employee", "staff", "buyer", "seller", "supplier",
    "vendor", "merchant", "agent", "manager", "supervisor", "salesperson",
    "salesman", "consultant", "attorney", "lawyer", "applicant", "tenant",
    "guest", "member", "partner", "recipient", "author", "creator",
    "employer", "contractor", "assistant", "coordinator", "founder",
    "president", "secretary", "advisor", "analyst", "engineer", "developer",
    "delivery", "courier", "driver", "landlord", "spouse", "relative",
    "guardian", "emergency contact", "next of kin", "doctor", "patient",
    "student", "teacher", "candidate",
  ], "PERSON"],
  [["cpf"], "CPF"],
  [["cnpj"], "CNPJ"],
  [[
    "rg", "cnh", "renavam", "nis", "pis", "pasep", "titulo de eleitor",
    "passaporte", "inscricao estadual", "inscricao municipal", "identidade",
    "passport", "ssn", "identity", "tax id", "taxid", "tax payer id",
    "taxpayer id", "voter id", "national id", "id card", "id number",
    "drivers license", "driver license", "license number", "license plate",
    "plate", "doc number", "document number", "enrollment", "registration",
  ], "ID"],
  [[
    "email", "e-mail", "email principal", "email contato", "email corporativo",
    "mail", "mail address", "email address", "contact email", "official email",
  ], "EMAIL"],
  [[
    "telefone", "fone", "fone fixo", "telefone fixo", "telefone principal",
    "celular", "celular principal", "whatsapp", "phone", "telephone",
    "phone number", "contact number", "mobile", "mobile number", "cellphone",
    "cell", "work phone", "home phone", "landline", "whatsapp number", "fax",
  ], "PHONE"],
  [[
    "endereco", "endereco completo", "logradouro", "rua", "avenida", "bairro",
    "distrito", "cidade", "estado", "uf", "pais", "cep", "codigo postal",
    "localizacao", "address", "street", "avenue", "neighborhood", "district",
    "city", "state", "country", "zip", "zipcode", "zip code", "postal code",
    "postcode", "street address", "postal address", "home address",
    "work address", "residence", "address line", "province", "region",
    "municipality", "county", "quarter", "zone", "ward", "location",
    "locality", "village",
  ], "ADDRESS"],
  [[
    "banco", "agencia", "conta", "conta corrente", "numero da conta",
    "bank", "bank account", "agency", "branch", "account", "checking account",
    "account number", "routing number", "sort code", "iban", "swift", "bic",
    "wire",
  ], "BANK"],
  [[
    "cvv", "validade", "cartao", "numero do cartao", "numero cartao",
    "expiry", "card", "card number", "cardholder", "cardholder name",
    "credit card", "creditcard", "debit card", "pan", "cvv2", "cvc",
    "security code", "verification code",
  ], "CARD"],
  [[
    "data de nascimento", "data nascimento", "nascimento", "data de aniversario",
    "birthdate", "birth date", "birth day", "birthday", "dob", "date of birth",
    "born",
  ], "DATE"],
  [[
    "senha", "chave", "token", "access token", "authorization", "auth",
    "api key", "api-key", "secret", "bearer", "password", "passwd", "pwd",
    "pass", "private key", "secret key", "client secret", "credential",
    "credentials", "session", "session id", "session_id", "cookie", "csrf",
    "otp", "2fa", "pin", "access key", "api token", "refresh token",
    "recovery code",
  ], "SECRET"],
  [[
    "usuario", "login", "user", "username", "user id", "userid",
    "screen name", "handle", "account name",
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
FIELD_KEYS.sort((a, b) => b.length - a.length); // "nome completo" antes de "nome"

// maskForEntity: máscara de CÉLULA/valor por coluna (=== maskByColumn do sqlize).
function maskForEntity(entity: string | null | undefined, value: string): string {
  switch (entity) {
    case "SECRET":
      return MASK;
    case "CPF":
    case "CNPJ":
    case "ID":
    case "PHONE":
    case "CEP":
    case "BANK":
    case "DATE":
      return keepEdges(value);
    case "CARD":
      return maskCard(value);
    case "EMAIL":
      return value.includes("@") ? maskEmail(value) : maskByName(value);
    default:
      return maskByName(value);
  }
}

// ---------- tabelas markdown ----------

function splitRow(line: string): { cells: string[]; starts: boolean; ends: boolean } {
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
      const entityByCol = header.cells.map((c) => COLUMN_ENTITY.get(normalizeWord(c.trim())) ?? null);
      out.push(line); // cabeçalho inalterado
      out.push(next); // separador inalterado
      i += 2;
      while (i < lines.length && lines[i].includes("|")) {
        const row = splitRow(lines[i]);
        const cells = row.cells.map((orig, idx) => {
          const ent = entityByCol[idx] ?? null;
          if (!ent) return orig;
          // preserva o espaçamento original da célula ao redor do valor mascarado.
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

// ---------- "campo: valor" / "campo=valor" em texto ----------

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
  // Ordem importa: tabelas e "campo: valor" primeiro (máscaras de célula); em
  // seguida os formatos/endereços/nomes. Valores já mascarados não re-combinam
  // com os padrões (estrelas não casam os regex), evitando mascarar 2x.
  let out = redactTables(md); // colunas de tabelas markdown
  out = redactFields(out); // "campo: valor" em texto
  out = redactText(out); // formatos (regex + checksum)
  out = redactAddresses(out); // endereços por logradouro
  out = redactNames(out); // nomes compostos via deny-list
  return out;
}

// ---------------------------------------------------------------------------

export function redactPII(markdown: string): string {
  // Blocos de código fenced (```/~~~) são protegidos: código fonte não é PII e
  // os padrões de URL/nome geram falsos positivos dentro dele ("console.log",
  // "soma.dev()"). Todo o resto segue o redactBody (tabelas, campos, formatos,
  // endereços, nomes).
  const lines = markdown.split("\n");
  const chunks: string[] = []; // trecho atual fora de código, a redigir junto
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
