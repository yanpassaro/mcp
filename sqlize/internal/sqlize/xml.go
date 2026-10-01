package sqlize

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

type xmlNode struct {
	Name     string
	Attr     map[string]string
	Children []*xmlNode
	Text     string
}

func parseXML(path string) ([]string, [][]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	dec := xml.NewDecoder(strings.NewReader(string(data)))
	dec.Strict = false
	root := &xmlNode{}
	stack := []*xmlNode{}
	for tok, e := dec.Token(); !errors.Is(e, io.EOF); tok, e = dec.Token() {
		if e != nil {
			return nil, nil, fmt.Errorf("analisar XML: %w", e)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			n := &xmlNode{Name: t.Name.Local, Attr: map[string]string{}}
			for _, a := range t.Attr {
				n.Attr[a.Name.Local] = a.Value
			}
			if len(stack) > 0 {
				stack[len(stack)-1].Children = append(stack[len(stack)-1].Children, n)
			}
			if len(stack) == 0 {
				root = n
			}
			stack = append(stack, n)
		case xml.EndElement:
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		case xml.CharData:
			if len(stack) > 0 {
				stack[len(stack)-1].Text += string(t)
			}
		}
	}
	if root.Name == "" {
		return nil, nil, fmt.Errorf("XML vazio ou inválido")
	}
	rowNodes := findRows(root)
	if len(rowNodes) == 0 {
		rowNodes = []*xmlNode{root}
	}
	colOrder := map[string]int{}
	cols := []string{}
	col := colBuilder{order: colOrder, cols: &cols}
	for _, rn := range rowNodes {
		attrs := make([]string, 0, len(rn.Attr))
		for k := range rn.Attr {
			attrs = append(attrs, k)
		}
		sort.Strings(attrs)
		for _, k := range attrs {
			col.add(fmt.Sprintf("@%s", k))
		}
		for _, ch := range rn.Children {
			if len(ch.Children) == 0 {
				col.add(ch.Name)
			}
		}
	}
	rows := make([][]string, 0, len(rowNodes))
	for _, rn := range rowNodes {
		row := make([]string, len(cols))
		vals := map[string]string{}
		for k, v := range rn.Attr {
			vals[fmt.Sprintf("@%s", k)] = v
		}
		for _, ch := range rn.Children {
			if len(ch.Children) == 0 {
				txt := strings.TrimSpace(ch.Text)
				if _, ok := vals[ch.Name]; !ok {
					vals[ch.Name] = txt
					continue
				}
				if vals[ch.Name] == "" {
					vals[ch.Name] = txt
				}
			}
		}
		for i, c := range cols {
			row[i] = vals[c]
		}
		rows = append(rows, row)
	}
	if len(cols) == 0 {
		cols = []string{"value"}
	}
	return cols, rows, nil
}

type colBuilder struct {
	order map[string]int
	cols  *[]string
}

func (b *colBuilder) add(k string) {
	if _, ok := b.order[k]; ok {
		return
	}
	b.order[k] = len(*b.cols)
	*b.cols = append(*b.cols, k)
}

func findRows(n *xmlNode) []*xmlNode {
	f := rowFinder{}
	return f.walk(n)
}

type rowFinder struct {
	best []*xmlNode
}

func (f *rowFinder) walk(node *xmlNode) []*xmlNode {
	if f.best != nil {
		return f.best
	}
	groups := map[string][]*xmlNode{}
	for _, c := range node.Children {
		groups[c.Name] = append(groups[c.Name], c)
	}
	for _, kids := range groups {
		if len(kids) >= 2 {
			f.best = kids
			return f.best
		}
	}
	for _, c := range node.Children {
		f.walk(c)
		if f.best != nil {
			return f.best
		}
	}
	return nil
}
