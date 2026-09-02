
import {
  maskFull,
  redactPII,
  validCNPJ,
  validCPF,
} from "../src/pii.ts";

function check(name: string, got: string, want: string): void {
  if (got !== want) {
    throw new Error(`${name}\n  got:  ${JSON.stringify(got)}\n  want: ${JSON.stringify(want)}`);
  }
}

Deno.test("checksum CPF/CNPJ", () => {
  check("CPF válido", String(validCPF("529.982.247-25")), "true");
  check("CPF inválido", String(validCPF("123.456.789-00")), "false");
  check("CPF repetido", String(validCPF("111.111.111-11")), "false");
  check("CNPJ válido", String(validCNPJ("11.222.333/0001-81")), "true");
  check("CNPJ inválido", String(validCNPJ("11.111.111/1111-11")), "false");
});

Deno.test("CPF só mascarado se válido", () => {
  check("CPF válido", redactPII("cpf: 529.982.247-25"), "cpf: 52*******-25");
  check("CPF inválido inalterado", redactPII("protocolo 123.456.789-00"),
    "protocolo 123.456.789-00");
});

Deno.test("e-mail oculta o domínio", () => {
  check("email simples", redactPII("email: joao@exemplo.com"), "email: jo**@***.com");
  check("email domínio composto", redactPII("thiago@server.com.br"), "th**@***.br");
});

Deno.test("qualquer URL vira PII", () => {
  check("url com esquema", redactPII("veja https://exemplo.com/pagina?x=1"), "veja ***");
  check("dominio puro", redactPII("site exemplo.com.br"), "site ***");
});

Deno.test("datas genéricas não são mascaradas", () => {
  check("data texto", redactPII("criado em 2026-08-24 às 09:37"), "criado em 2026-08-24 às 09:37");
  check("data coluna de nascimento sim", redactPII("data de nascimento: 01/01/1990"),
    "data de nascimento: 01****/90");
});

Deno.test("cartão com Luhn válido", () => {
  check("cartao", redactPII("cartao: 4111 1111 1111 1111"), "cartao: 4111********1111");
});

Deno.test("nomes compostos em texto livre", () => {
  check("nome em frase", redactPII("Falei com Joao da Silva as 14h"), "Falei com [NOME] as 14h");
  check("nome simples", redactPII("nome: Maria Eduarda"), "nome: [NOME]");
  check("nome primeiro termo", redactPII("O Joao Silva saiu"), "O [NOME] saiu");
});

Deno.test("nomes independentes da lista fixa", () => {
  check("nome fora da lista", redactPII("Falei com Tomasz Kowalski ontem"), "Falei com [NOME] ontem");
  check("nome unico com contexto", redactPII("Falei com Maria ontem"), "Falei com [NOME] ontem");
  check("sobrenome de cidade", redactPII("Falei com Joao Santos ontem"), "Falei com [NOME] ontem");
});

Deno.test("não-nomes continuam intactos", () => {
  check("produto", redactPII("Produto de Limpeza Multiuso"), "Produto de Limpeza Multiuso");
  check("orgao", redactPII("Banco do Brasil"), "Banco do Brasil");
  check("dia da semana", redactPII("Segunda Feira"), "Segunda Feira");
  check("com boleto", redactPII("Pago com Boleto"), "Pago com Boleto");
});

Deno.test("cidades e estados não são mascarados", () => {
  check("cidade famosa", redactPII("Mudança para São Paulo em 2025"), "Mudança para São Paulo em 2025");
  check("capital com nome próprio", redactPII("fui a João Pessoa no fim de semana"), "fui a João Pessoa no fim de semana");
  check("estado", redactPII("moro no Rio de Janeiro"), "moro no Rio de Janeiro");
  check("cidade composta", redactPII("instalou-se em São José dos Campos"), "instalou-se em São José dos Campos");
  check("cidade com nome inicial", redactPII("chácara em Paulo Afonso"), "chácara em Paulo Afonso");
});

Deno.test("código fonte não é mascarado", () => {
  check("chamada de método", redactPII("const x = soma(1, 2); console.log('oi');"),
    "const x = soma(1, 2); console.log('oi');");
  check("bloco fenced", redactPII("```ts\nconst s = soma(a, b);\nconsole.log(s);\n```"),
    "```ts\nconst s = soma(a, b);\nconsole.log(s);\n```");
  check("bloco com url fake", redactPII("```\nconst api = 'https://exemplo.com/x';\n```"),
    "```\nconst api = 'https://exemplo.com/x';\n```");
});

Deno.test("endereços por logradouro", () => {
  check("rua", redactPII("Moro na Rua das Flores, 123 em SP"), "Moro na [ENDEREÇO] em SP");
  check("avenida com bairro", redactPII("Av. Paulista 1000, Bela Vista"), "[ENDEREÇO]");
});

Deno.test("telefone com parênteses", () => {
  check("ddd entre parens", redactPII("Falei com (11) 98765-4321"), "Falei com 11*******-21");
});

Deno.test("colunas de tabela markdown", () => {
  const md = [
    "| nome | cpf | email |",
    "|------|-----|-------|",
    "| Maria Silva | 529.982.247-25 | joao@exemplo.com |",
  ].join("\n");
  const want = [
    "| nome | cpf | email |",
    "|------|-----|-------|",
    "| [NOME] | 52*******-25 | jo**@***.com |",
  ].join("\n");
  check("tabela", redactPII(md), want);
});

Deno.test("segredo vira *** e máscara total", () => {
  check("senha", redactPII("senha: abc123"), "senha: ***");
  check("maskFull", maskFull("x"), "***");
});
