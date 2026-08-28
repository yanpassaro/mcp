package sqlize

import (
	"os"
	"strings"
)

func normalizeWord(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		switch r {
		case 'á', 'à', 'â', 'ã', 'ä':
			b.WriteRune('a')
		case 'é', 'è', 'ê', 'ë':
			b.WriteRune('e')
		case 'í', 'ì', 'î', 'ï':
			b.WriteRune('i')
		case 'ó', 'ò', 'ô', 'õ', 'ö':
			b.WriteRune('o')
		case 'ú', 'ù', 'û', 'ü':
			b.WriteRune('u')
		case 'ç':
			b.WriteRune('c')
		case 'ñ', 'ý':
			b.WriteRune('n')
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

var brFirstNames = map[string]bool{
	"joao": true, "jose": true, "antonio": true, "francisco": true, "carlos": true,
	"paulo": true, "pedro": true, "lucas": true, "gabriel": true, "matheus": true,
	"rafael": true, "marcos": true, "marcelo": true, "bruno": true, "thiago": true,
	"felipe": true, "gustavo": true, "rodrigo": true, "fernando": true, "eduardo": true,
	"daniel": true, "leonardo": true, "ricardo": true, "andre": true, "henrique": true,
	"guilherme": true, "diego": true, "vinicius": true, "leandro": true, "renato": true,
	"alexandre": true, "fabio": true, "sergio": true, "rogerio": true, "mauricio": true,
	"jorge": true, "marcio": true, "everton": true, "anderson": true, "douglas": true,
	"wesley": true, "davi": true, "miguel": true, "arthur": true, "bernardo": true,
	"heitor": true, "theo": true, "enzo": true, "nicolas": true, "samuel": true,
	"benjamin": true, "joaquim": true, "lucca": true, "lorenzo": true, "anthony": true,
	"caua": true, "murilo": true, "pietro": true, "alan": true, "caio": true,
	"igor": true, "alex": true, "emerson": true, "elias": true, "gilberto": true,
	"hugo": true, "ivan": true, "julio": true, "kleber": true, "nilton": true,
	"roberto": true, "romulo": true, "sandro": true, "sebastiao": true, "valdir": true,
	"vitor": true, "wagner": true, "william": true, "yuri": true, "flavio": true,
	"gilmar": true, "gerson": true, "osmar": true, "valter": true, "rubens": true,
	"joel": true, "nilson": true, "edson": true, "edvaldo": true, "jefferson": true,
	"jairo": true, "jaime": true, "jeferson": true, "evandro": true, "eder": true,
	"elton": true, "ezequiel": true, "sidnei": true, "sidney": true, "olavo": true,
	"oswaldo": true, "otavio": true, "raimundo": true, "reinaldo": true, "saulo": true,
	"tadeu": true, "ulisses": true, "vanderlei": true, "washington": true,
	"maria": true, "ana": true, "juliana": true, "mariana": true, "fernanda": true,
	"camila": true, "larissa": true, "beatriz": true, "amanda": true, "patricia": true,
	"leticia": true, "vanessa": true, "carolina": true, "gabriela": true, "thais": true,
	"aline": true, "bruna": true, "jessica": true, "natalia": true, "bianca": true,
	"raquel": true, "renata": true, "sabrina": true, "luciana": true, "viviane": true,
	"sandra": true, "claudia": true, "adriana": true, "cintia": true, "daniela": true,
	"priscila": true, "taina": true, "yasmin": true, "isadora": true, "manuela": true,
	"helena": true, "alice": true, "sophia": true, "laura": true, "valentina": true,
	"rafaela": true, "milena": true, "isabela": true, "luana": true, "melissa": true,
	"nicole": true, "sarah": true, "karina": true, "monica": true, "simone": true,
	"eliane": true, "regina": true, "tereza": true, "aparecida": true, "deise": true,
	"elen": true, "heloisa": true, "ingrid": true, "joana": true, "katia": true,
	"lucia": true, "paula": true, "roberta": true, "sonia": true, "sueli": true,
	"valquiria": true, "vera": true, "vitoria": true, "fabiana": true, "nayara": true,
	"gabrielly": true, "gabriele": true, "julia": true, "giovanna": true, "melina": true,
	"michele": true, "michelle": true, "silvia": true, "solange": true, "tatiana": true,
	"tania": true, "tamires": true, "vania": true, "veronica": true, "zelia": true,
	"junior": true, "neto": true, "kevin": true, "emily": true,
}

var wordDeny = map[string]bool{
	"bom": true, "boa": true, "inicio": true, "fim": true, "topo": true,
	"rodape": true, "anexo": true, "atencao": true, "aviso": true,
	"erro": true, "sucesso": true, "falha": true, "alerta": true,
	"pendente": true, "cancelado": true, "concluido": true, "ativo": true,
	"inativo": true, "bloqueado": true, "liberado": true, "sim": true,
	"nao": true, "novo": true, "usado": true, "seminovo": true,
	"obrigatorio": true, "opcional": true, "padrao": true,
	"janeiro": true, "fevereiro": true, "abril": true, "maio": true,
	"junho": true, "julho": true, "agosto": true, "setembro": true,
	"outubro": true, "novembro": true, "dezembro": true,
	"segunda": true, "terca": true, "quarta": true, "quinta": true,
	"sexta": true, "sabado": true, "domingo": true,
	"monday": true, "tuesday": true, "wednesday": true, "thursday": true,
	"friday": true, "saturday": true, "sunday": true,
	"january": true, "february": true, "march": true, "april": true,
	"may": true, "june": true, "july": true, "august": true,
	"september": true, "october": true, "november": true, "december": true,
	"total": true, "valor": true, "valores": true, "quantidade": true,
	"status": true, "situacao": true, "pedido": true, "venda": true,
	"vendas": true, "compra": true, "compras": true, "item": true,
	"itens": true, "lote": true, "serie": true, "numero": true,
	"codigo": true, "nota": true, "relatorio": true, "orcamento": true,
	"contrato": true, "fatura": true, "cobranca": true, "desconto": true,
	"imposto": true, "taxa": true, "frete": true, "entrega": true,
	"prazo": true, "garantia": true, "modelo": true, "marca": true,
	"categoria": true, "tipo": true, "descricao": true, "observacao": true,
	"detalhe": true, "motivo": true, "origem": true, "destino": true,
	"forma": true, "meio": true, "canal": true, "campanha": true,
	"promocao": true, "produto": true, "produtos": true, "servico": true,
	"servicos": true, "limpeza": true, "manutencao": true, "instalacao": true,
	"montagem": true, "reparo": true, "seguro": true, "aluguel": true,
	"assinatura": true, "mensalidade": true, "projeto": true, "processo": true,
	"atividade": true, "tarefa": true, "reuniao": true, "treinamento": true,
	"suporte": true, "equipe": true, "time": true, "departamento": true,
	"setor": true, "unidade": true, "filial": true, "matriz": true,
	"escritorio": true, "loja": true, "estoque": true, "documento": true,
	"pagamento": true, "confirmado": true, "emitida": true, "emitido": true,
	"recebido": true, "faturamento": true, "mensagem": true, "historico": true,
	"boleto": true, "descarte": true, "devolucao": true, "cancelamento": true,
	"retirada": true, "coleta": true, "aprovacao": true, "analise": true,
	"revisao": true, "auditoria": true, "inspecao": true, "validacao": true,
	"emissao": true, "impressao": true, "digitalizacao": true,
	"arquivamento": true, "armazenamento": true, "movimentacao": true,
	"remessa": true, "separacao": true, "embalagem": true, "expedicao": true,
	"financeiro": true, "contabil": true, "fiscal": true, "comercial": true,
	"administrativo": true, "atualizacao": true, "sincronizacao": true,
	"integracao": true, "migracao": true, "backup": true, "restauracao": true,
	"exclusao": true, "inclusao": true, "alteracao": true, "consulta": true,
	"registro": true, "cadastro": true, "login": true, "acesso": true,
	"permissao": true, "usuario": true, "sessao": true, "conexao": true,
	"gerente": true, "diretor": true, "coordenador": true, "supervisor": true,
	"analista": true, "assistente": true, "consultor": true, "vendedor": true,
	"atendente": true, "tecnico": true, "engenheiro": true, "advogado": true,
	"arquiteto": true, "motorista": true, "recepcionista": true, "caixa": true,
	"porteiro": true, "auxiliar": true, "estagiario": true, "aprendiz": true,
	"presidente": true, "secretaria": true, "cozinheiro": true, "conferente": true,
	"rua": true, "avenida": true, "travessa": true, "alameda": true,
	"estrada": true, "rodovia": true, "praca": true, "beco": true,
	"largo": true, "viela": true, "condominio": true, "conjunto": true,
	"residencial": true, "loteamento": true, "chacara": true, "sitio": true,
	"fazenda": true, "quadra": true, "bloco": true, "andar": true, "sala": true,
	"apto": true, "apartamento": true, "casa": true, "predio": true,
	"edificio": true, "torre": true, "terreno": true,
	"banco": true, "empresa": true, "grupo": true, "fundacao": true,
	"associacao": true, "cooperativa": true, "sindicato": true,
	"universidade": true, "faculdade": true, "colegio": true, "escola": true,
	"hospital": true, "clinica": true, "laboratorio": true, "instituto": true,
	"centro": true, "prefeitura": true, "ministerio": true, "governo": true,
	"autarquia": true, "agencia": true, "sociedade": true, "companhia": true,
	"industria": true, "fabrica": true, "comercio": true, "distribuidora": true,
	"transportadora": true, "construtora": true, "imobiliaria": true,
	"product": true, "service": true, "services": true, "amount": true,
	"value": true, "quantity": true, "order": true,
	"sale": true, "purchase": true, "items": true,
	"number": true, "code": true, "report": true, "budget": true,
	"contract": true, "invoice": true, "discount": true, "tax": true,
	"fee": true, "shipping": true, "delivery": true, "deadline": true,
	"warranty": true, "model": true, "brand": true, "category": true,
	"type": true, "description": true, "reason": true, "origin": true,
	"destination": true, "payment": true, "plan": true, "project": true,
	"task": true, "meeting": true, "support": true, "team": true,
	"department": true, "unit": true, "branch": true, "store": true,
	"stock": true, "title": true, "document": true, "message": true,
	"history": true, "manager": true, "director": true, "coordinator": true,
	"analyst": true, "assistant": true, "consultant": true,
	"salesperson": true, "technician": true, "engineer": true, "lawyer": true,
	"driver": true, "receptionist": true, "intern": true, "president": true,
	"secretary": true, "street": true, "avenue": true, "road": true,
	"highway": true, "lane": true, "square": true, "building": true,
	"floor": true, "suite": true, "apartment": true, "house": true,
	"bank": true, "company": true, "group": true, "foundation": true,
	"association": true, "cooperative": true, "university": true,
	"college": true, "school": true, "clinic": true,
	"laboratory": true, "institute": true, "center": true, "government": true,
	"ministry": true, "agency": true, "society": true, "industry": true,
	"factory": true,
	"review": true, "approval": true, "analysis": true, "inspection": true,
	"validation": true, "printing": true, "removal": true, "testing": true,
	"processing": true, "handling": true, "storage": true, "packing": true,
	"dispatch": true, "update": true, "query": true, "record": true,
	"registration": true, "access": true, "permission": true,
	"user": true, "session": true, "connection": true, "sync": true,
	"integration": true, "migration": true, "restore": true,
	"deletion": true, "insertion": true, "alteration": true, "exclusion": true,
	"inclusion": true,
}

var orgSuffix = map[string]bool{
	"ltda": true, "eireli": true, "epp": true,
}

func init() {
	loadWordList(os.Getenv("SQLIZE_PII_NAMES"), brFirstNames)
	loadWordList(os.Getenv("SQLIZE_PII_WORDS"), wordDeny)
}

func loadWordList(path string, into map[string]bool) {
	path = strings.TrimSpace(path)
	if path == "" {
		return
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return
	}
	for line := range strings.SplitSeq(string(b), "\n") {
		if w := normalizeWord(strings.TrimSpace(line)); w != "" {
			into[w] = true
		}
	}
}
