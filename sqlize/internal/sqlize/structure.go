package sqlize

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

type structColumn struct {
	Name      string
	Type      string
	Default   string
	RefSchema string
	RefTable  string
	RefColumn string
	NotNull   bool
	PK        bool
	Auto      bool
}

type structIdx struct {
	Unique  bool
	IsPK    bool
	Columns []string
}

type structTable struct {
	Schema string
	Name   string
	Engine string
	Cols   []structColumn
	Idx    []structIdx
}

var engineIcon = map[string]string{
	"sqlite":   "🏡",
	"postgres": "🐘",
	"mysql":    "🐬",
}

func displaySchema(schema, engine string) string {
	if schema != "" {
		return schema
	}
	switch engine {
	case "postgres":
		return "public"
	case "mysql":
		return ""
	default:
		return "main"
	}
}

func isNullDefault(v string) bool {
	t := strings.ToLower(strings.TrimSpace(v))
	return t == "null" || strings.HasPrefix(t, "null::")
}

func isPkVal(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "", "0", "false", "f", "no", "n":
		return false
	}
	return true
}

func refTarget(fk structColumn) string {
	col := fk.RefColumn
	if col == "" {
		col = "?"
	}
	target := col
	if fk.RefTable != "" {
		target = fk.RefTable + "." + target
	}
	sc := fk.RefSchema
	if sc != "" && sc != "public" && sc != "main" {
		target = sc + "." + target
	}
	return strings.ToUpper(strings.TrimSpace(target))
}

func structFlags(c structColumn, compositePK bool, unique, indexed map[string]bool) string {
	var flags []string
	if c.PK && !compositePK {
		flags = append(flags, "🔑")
	}
	if c.Auto {
		flags = append(flags, "🔢")
	}
	if c.NotNull || c.PK {
		flags = append(flags, "🔒")
	}
	if d := strings.TrimSpace(c.Default); d != "" && !c.Auto && !isNullDefault(d) {
		flags = append(flags, "🔄 "+short(d))
	}
	if unique[c.Name] {
		flags = append(flags, "⭐")
	}
	if c.RefTable != "" || c.RefColumn != "" {
		flags = append(flags, "🔗 "+refTarget(c))
	}
	if indexed[c.Name] {
		flags = append(flags, "⚡")
	}
	return strings.Join(flags, " ")
}

func renderStructureTable(t structTable) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# 📋 %s\n\n", t.Name)

	schema := displaySchema(t.Schema, t.Engine)
	icon := engineIcon[t.Engine]
	if icon == "" {
		icon = "🏡"
	}
	if schema != "" {
		fmt.Fprintf(&b, "▶️ `%s` %s\n\n", schema, icon)
	} else {
		fmt.Fprintf(&b, "▶️ %s\n\n", icon)
	}

	unique := map[string]bool{}
	indexed := map[string]bool{}
	var compositeUniques [][]string
	for _, ix := range t.Idx {
		if ix.IsPK {
			continue
		}
		if len(ix.Columns) == 0 {
			continue
		}
		if ix.Unique {
			if len(ix.Columns) == 1 {
				unique[ix.Columns[0]] = true
			} else {
				compositeUniques = append(compositeUniques, ix.Columns)
			}
			continue
		}
		for _, c := range ix.Columns {
			indexed[c] = true
		}
	}

	var pkCols []string
	for _, c := range t.Cols {
		if c.PK {
			pkCols = append(pkCols, c.Name)
		}
	}
	compositePK := len(pkCols) > 1

	width := 0
	for _, c := range t.Cols {
		if w := utf8.RuneCountInString(c.Name) + 2; w > width {
			width = w
		}
	}

	for _, c := range t.Cols {
		b.WriteString("- `");
		b.WriteString(c.Name);
		b.WriteString("`")
		pad := width - (utf8.RuneCountInString(c.Name) + 2)
		for range pad {
			b.WriteByte(' ')
		}
		b.WriteByte(' ')
		if strings.TrimSpace(c.Type) != "" {
			b.WriteString(strings.ToUpper(strings.TrimSpace(c.Type)))
		}
		if flags := structFlags(c, compositePK, unique, indexed); flags != "" {
			b.WriteString(" · ")
			b.WriteString(flags)
		}
		b.WriteString("\n")
	}

	if compositePK {
		fmt.Fprintf(&b, "- 🔑 PK → (%s)\n", strings.Join(pkCols, ", "))
	}
	for _, cu := range compositeUniques {
		fmt.Fprintf(&b, "- ⭐ → (%s)\n", strings.Join(cu, ", "))
	}

	b.WriteString("\n---\n")
	b.WriteString("🔑 PK · 🔢 AUTO · 🔒 NOT NULL · 🔄 DEFAULT")
	b.WriteString(" · ⭐ UNIQUE · 🔗 FK · ⚡ ÍNDICE\n")
	return b.String()
}
