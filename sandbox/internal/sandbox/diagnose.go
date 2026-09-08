package sandbox

import (
	"regexp"
	"slices"
	"strings"

	lua "github.com/Shopify/go-lua"
)

type Diag struct {
	Severity string
	Message  string
}

var stdAPI = map[string][]string{
	"result": {"ok", "err"},
	"log":    {"ok", "info", "warn", "error", "err", "log"},
	"args":   {},
	"io":     {"read", "lines", "json", "write", "append", "del", "exists", "stat", "dir", "copy", "move", "mkdir", "glob", "walk"},
	"tmp":    {"read", "lines", "json", "write", "append", "del", "exists", "stat", "dir", "copy", "move", "mkdir", "glob", "walk", "clear"},
	"sql":    {"exec", "query", "get", "scalar", "close", "begin", "commit", "rollback", "tables", "columns", "schema"},
	"uuid":   {"v4", "v7"},
	"csv":    {"parse", "stringify"},
	"xml":    {"parse", "stringify"},
	"excel":  {"sheets", "read", "write"},
	"data":   {"fromCSV", "toCSV", "fromJSON", "toJSON", "toXML", "fromExcel", "toExcel", "sqlImport", "sqlExport", "convert"},
	"regex":  {"match", "find", "findAll", "replace", "split", "groups", "findAllGroups"},
	"fake":   {"seed", "firstName", "lastName", "name", "email", "username", "phone", "int", "float", "bool", "uuid", "date", "words", "sentence", "paragraph"},
	"json":   {"parse", "stringify", "format", "minify", "path"},
	"encode": {"crc32", "md5", "sha256", "base64", "hex"},
	"str":    {"normalize", "slug", "title", "camel", "pascal", "snake", "kebab", "wrap", "summarize", "format", "count", "split"},
	"list":   {"chunk", "groupBy", "unique", "flatten", "sortBy", "countBy", "first", "last", "map", "filter", "reduce", "find", "some", "every"},
	"num":    {"round", "clamp", "percent", "sum", "avg", "parse", "fmt"},
	"date":   {"now", "iso", "format", "parse", "add", "unix", "diff"},
	"random": {"seed", "int", "pick", "shuffle"},
	"assert": {"ok", "equal", "throws", "type", "notNil", "number", "string", "boolean", "table", "contains", "matches", "between", "length"},
	"fetch":  {"request", "get", "post", "json", "cookies"},
}

var (
	reMain   = regexp.MustCompile(`(?m)\bfunction\s+main\s*\(`)
	reStdUse = regexp.MustCompile(`std\.([A-Za-z_][A-Za-z0-9_]*)\.([A-Za-z_][A-Za-z0-9_]*)`)
)

func Diagnose(code string) (name, desc string, diags []Diag) {
	name, desc = ParseMeta(code)

	L := lua.NewState()
	lua.Require(L, "base", lua.BaseOpen, true)
	if err := L.Load(strings.NewReader(code), "@diag", "t"); err != nil {
		diags = append(diags, Diag{"erro", "sintaxe: " + callError(L, err)})
		return name, desc, diags
	}

	if !reMain.MatchString(code) {
		diags = append(diags, Diag{"aviso", "não foi encontrado `function main(std)`; o script não roda"})
	}

	for _, m := range reStdUse.FindAllStringSubmatch(code, -1) {
		mod, fn := m[1], m[2]
		funcs, ok := stdAPI[mod]
		if !ok {
			diags = append(diags, Diag{"aviso", "módulo `std." + mod + "` não existe"})
			continue
		}
		if !containsStr(funcs, fn) {
			diags = append(diags, Diag{"aviso", "`std." + mod + "." + fn + "` não existe"})
		}
	}
	return name, desc, diags
}

func containsStr(list []string, s string) bool {
	return slices.Contains(list, s)
}
