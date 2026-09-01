package mcpserver

import (
	"bufio"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/plumbing/format/gitignore"
	"github.com/go-git/go-git/v6/plumbing/object"
)

// extLanguages mapeia extensão de arquivo → linguagem (heurística simplificada
// no estilo do GitHub linguist, sem dependência externa).
var extLanguages = map[string]string{
	".go":         "Go",
	".ts":         "TypeScript",
	".tsx":        "TypeScript",
	".mts":        "TypeScript",
	".cts":        "TypeScript",
	".js":         "JavaScript",
	".jsx":        "JavaScript",
	".mjs":        "JavaScript",
	".cjs":        "JavaScript",
	".py":         "Python",
	".rs":         "Rust",
	".java":       "Java",
	".kt":         "Kotlin",
	".kts":        "Kotlin",
	".swift":      "Swift",
	".c":          "C",
	".h":          "C",
	".cpp":        "C++",
	".cc":         "C++",
	".cxx":        "C++",
	".hpp":        "C++",
	".hh":         "C++",
	".cs":         "C#",
	".rb":         "Ruby",
	".php":        "PHP",
	".sh":         "Shell",
	".bash":       "Shell",
	".zsh":        "Shell",
	".ps1":        "PowerShell",
	".bat":        "Batch",
	".cmd":        "Batch",
	".sql":        "SQL",
	".proto":      "Protocol Buffers",
	".lua":        "Lua",
	".pl":         "Perl",
	".r":          "R",
	".dart":       "Dart",
	".ex":         "Elixir",
	".exs":        "Elixir",
	".erl":        "Erlang",
	".hrl":        "Erlang",
	".hs":         "Haskell",
	".clj":        "Clojure",
	".cljs":       "Clojure",
	".scala":      "Scala",
	".groovy":     "Groovy",
	".gradle":     "Groovy",
	".vue":        "Vue",
	".svelte":     "Svelte",
	".astro":      "Astro",
	".html":       "HTML",
	".htm":        "HTML",
	".css":        "CSS",
	".scss":       "SCSS",
	".sass":       "SCSS",
	".less":       "Less",
	".styl":       "Stylus",
	".md":         "Markdown",
	".markdown":   "Markdown",
	".rst":        "reStructuredText",
	".json":       "JSON",
	".json5":      "JSON5",
	".yaml":       "YAML",
	".yml":        "YAML",
	".toml":       "TOML",
	".xml":        "XML",
	".svg":        "SVG",
	".csv":        "CSV",
	".txt":        "Text",
	".ini":        "INI",
	".cfg":        "INI",
	".env":        "Env",
	".tf":         "Terraform",
	".tfvars":     "Terraform",
	".graphql":    "GraphQL",
	".gql":        "GraphQL",
	".tex":        "TeX",
	".sol":        "Solidity",
	".zig":        "Zig",
	".nim":        "Nim",
	".coffee":     "CoffeeScript",
	".pug":        "Pug",
	".ejs":        "EJS",
	".hbs":        "Handlebars",
	".mustache":   "Mustache",
	".dockerfile": "Dockerfile",
}

// binaryExts arquivos binários que nunca entram na contagem de linhas.
var binaryExts = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".webp": true,
	".bmp": true, ".ico": true, ".tiff": true, ".tif": true,
	".exe": true, ".dll": true, ".so": true, ".dylib": true, ".o": true, ".a": true,
	".zip": true, ".gz": true, ".tar": true, ".7z": true, ".rar": true, ".xz": true,
	".pdf": true, ".psd": true, ".ai": true, ".eps": true,
	".woff": true, ".woff2": true, ".ttf": true, ".otf": true, ".eot": true,
	".mp3": true, ".mp4": true, ".avi": true, ".mov": true, ".mkv": true,
	".webm": true, ".wav": true,
	".wasm": true, ".class": true, ".jar": true, ".pyc": true, ".pyo": true,
}

// fallbackDirs só é usado quando o repo NÃO tem .gitignore: diretórios
// gerados/dependências que não devem contar como código.
var fallbackDirs = []string{
	".git", "node_modules", "vendor", "dist", "build", "out", ".next", ".nuxt",
	".venv", "venv", ".tox", "__pycache__", ".cache", "coverage", "target",
	".gradle", ".idea", ".vscode", "bower_components", "Pods", ".terraform",
}

func languageForPath(name string) (string, bool) {
	base := strings.ToLower(filepath.Base(name))
	switch base {
	case "dockerfile":
		return "Dockerfile", true
	case "makefile", "gnumakefile":
		return "Makefile", true
	}
	ext := strings.ToLower(filepath.Ext(name))
	lang, ok := extLanguages[ext]
	return lang, ok
}

func isBinaryFile(name string) bool {
	return binaryExts[strings.ToLower(filepath.Ext(name))]
}

func inIgnoredDir(name string) bool {
	name = filepath.ToSlash(name)
	for _, d := range fallbackDirs {
		if name == d || strings.HasPrefix(name, d+"/") {
			return true
		}
	}
	return false
}

// gitIgnorer reúne os padrões dos .gitignore presentes na árvore do HEAD.
type gitIgnorer struct {
	entries []gitignore.Pattern
}

func (g *gitIgnorer) ignored(name string) bool {
	segs := strings.Split(name, "/")
	for _, p := range g.entries {
		if p.Match(segs, false) == gitignore.Exclude {
			return true
		}
	}
	return false
}

func buildGitIgnorer(tree *object.Tree) *gitIgnorer {
	g := &gitIgnorer{}
	_ = tree.Files().ForEach(func(f *object.File) error {
		if filepath.Base(f.Name) != ".gitignore" {
			return nil
		}
		content, err := f.Contents()
		if err != nil {
			return nil
		}
		var domain []string
		if i := strings.LastIndex(f.Name, "/"); i >= 0 {
			domain = strings.Split(f.Name[:i], "/")
		}
		for _, line := range strings.Split(content, "\n") {
			line = strings.TrimSpace(line)
			// Linhas de negação ("!") re-incluem arquivos: como aqui a decisão é
			// "exclui ou conta", ignorá-las evita marcação incorreta num OR simples.
			if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "!") {
				continue
			}
			g.entries = append(g.entries, gitignore.ParsePattern(line, domain))
		}
		return nil
	})
	return g
}

// repoLanguages conta linhas por linguagem nos arquivos rastreados no HEAD,
// ignorando o que o .gitignore do próprio repo exclui (com fallback de
// diretórios comuns quando não há .gitignore).
func repoLanguages(repo *git.Repository) (map[string]int, int, error) {
	langs := map[string]int{}
	head, err := repo.Head()
	if err != nil {
		return langs, 0, nil // repositório sem commits: nada a contar
	}
	commit, err := repo.CommitObject(head.Hash())
	if err != nil {
		return langs, 0, err
	}
	tree, err := commit.Tree()
	if err != nil {
		return langs, 0, err
	}
	ignorer := buildGitIgnorer(tree)
	useGitIgnore := len(ignorer.entries) > 0
	total := 0
	err = tree.Files().ForEach(func(f *object.File) error {
		if isBinaryFile(f.Name) {
			return nil
		}
		if useGitIgnore {
			if ignorer.ignored(f.Name) {
				return nil
			}
		} else if inIgnoredDir(f.Name) {
			return nil
		}
		lang, ok := languageForPath(f.Name)
		if !ok {
			return nil
		}
		n, lerr := countTextLines(f)
		if lerr != nil {
			return nil
		}
		langs[lang] += n
		total += n
		return nil
	})
	return langs, total, err
}

func countTextLines(f *object.File) (int, error) {
	r, err := f.Reader()
	if err != nil {
		return 0, err
	}
	defer r.Close()
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 4*1024*1024)
	lines := 0
	for sc.Scan() {
		lines++
	}
	return lines, sc.Err()
}
