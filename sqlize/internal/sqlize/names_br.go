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

func init() {
	path := strings.TrimSpace(os.Getenv("SQLIZE_PII_NAMES"))
	if path == "" {
		return
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(b), "\n") {
		if w := normalizeWord(strings.TrimSpace(line)); w != "" {
			brFirstNames[w] = true
		}
	}
}
