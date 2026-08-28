package sqlize

import "testing"

func redactCellValue(col, val string) string {
	rows := RedactRows([]string{col}, [][]string{{val}})
	return rows[0][0]
}

func TestRedactRows(t *testing.T) {
	cases := []struct {
		name string
		col  string
		val  string
		want string
	}{
		{"cpf valido em coluna cpf", "cpf", "529.982.247-25", "52*******-25"},
		{"cpf valido em coluna generica", "descricao", "529.982.247-25", "52*******-25"},
		{"cpf invalido em coluna generica", "descricao", "123.456.789-00", "123.456.789-00"},
		{"cpf invalido em coluna cpf", "cpf", "123.456.789-00", "12*******-00"},
		{"email", "email", "joao@exemplo.com", "jo**@***.com"},
		{"email dominio composto", "descricao", "fale com thiago@server.com.br", "fale com th**@***.br"},
		{"url com esquema", "descricao", "veja https://exemplo.com/pagina?x=1", "veja ***"},
		{"url www", "descricao", "acesse www.exemplo.com.br hoje", "acesse *** hoje"},
		{"dominio sem esquema", "descricao", "site exemplo.com.br", "site ***"},
		{"endereco em texto livre", "descricao", "Moro na Rua das Flores, 123 em SP", "Moro na R*** em SP"},
		{"endereco com bairro", "descricao", "Av. Paulista 1000, Bela Vista", "A***"},
		{"rua com prep antes do nome", "descricao", "Rua da Consolação, 100 no centro", "R*** no centro"},
		{"chacara com preposicao nao mascara", "descricao", "Chácara em Paulo Afonso", "Chácara em Paulo Afonso"},
		{"nome em coluna nome", "nome", "Maria Eduarda", "M***"},
		{"nome em texto livre", "descricao", "Falei com Joao da Silva as 14h", "Falei com J*** as 14h"},
		{"nome unico maiusculo", "descricao", "MARIA", "M***"},
		{"cartao em coluna cartao", "cartao", "4111 1111 1111 1111", "4111********1111"},
		{"senha por coluna", "senha", "abc123", "***"},
		{"rg em coluna generica", "descricao", "12.345.678-9", "12.345.678-9"},
		{"rg em coluna rg", "rg", "12.345.678-9", "12*****-89"},
		{"phone em coluna inglesa", "phone", "(11) 98765-4321", "11*******-21"},
		{"name em coluna inglesa", "customer", "Joao Barbosa", "J***"},
		{"password em coluna inglesa", "password", "senha123", "***"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := redactCellValue(tc.col, tc.val); got != tc.want {
				t.Errorf("RedactRows(%q, %q) = %q; want %q", tc.col, tc.val, got, tc.want)
			}
		})
	}
}

func TestRedactValue(t *testing.T) {
	if got := RedactValue("123.456.789-00"); got != "123.456.789-00" {
		t.Errorf("RedactValue(CPF inválido) = %q; want inalterado", got)
	}
	if got := RedactValue("529.982.247-25"); got != "52*******-25" {
		t.Errorf("RedactValue(CPF válido) = %q; want %q", got, "52*******-25")
	}
}

func TestValidCPF(t *testing.T) {
	if !validCPF("529.982.247-25") {
		t.Error("529.982.247-25 deveria ser CPF válido")
	}
	if validCPF("123.456.789-00") {
		t.Error("123.456.789-00 não deveria ser CPF válido")
	}
	if validCPF("111.111.111-11") {
		t.Error("111.111.111-11 (dígitos repetidos) não deveria ser CPF válido")
	}
}

func TestValidCNPJ(t *testing.T) {
	if !validCNPJ("11.222.333/0001-81") {
		t.Error("11.222.333/0001-81 deveria ser CNPJ válido")
	}
	if validCNPJ("11.111.111/1111-11") {
		t.Error("CNPJ com dígitos repetidos não deveria ser válido")
	}
}

func TestColumnEntityEnglish(t *testing.T) {
	if ent, ok := columnEntity("customer"); !ok || ent != "PERSON" {
		t.Errorf("columnEntity(customer) = %q, %v; want PERSON", ent, ok)
	}
	if ent, ok := columnEntity("phone"); !ok || ent != "PHONE" {
		t.Errorf("columnEntity(phone) = %q, %v; want PHONE", ent, ok)
	}
	if ent, ok := columnEntity("password"); !ok || ent != "SECRET" {
		t.Errorf("columnEntity(password) = %q, %v; want SECRET", ent, ok)
	}
}
