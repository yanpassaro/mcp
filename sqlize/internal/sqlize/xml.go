package sqlize

import (
	"encoding/xml"
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
	var root *xmlNode
	var stack []*xmlNode
	for {
		tok, e := dec.Token()
		if e != nil {
			if e == io.EOF {
				break
			}
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
			} else {
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
	if root == nil {
		return nil, nil, fmt.Errorf("XML vazio ou inválido")
	}
	rowNodes := findRows(root)
	if len(rowNodes) == 0 {
		rowNodes = []*xmlNode{root}
	}
	colOrder := map[string]int{}
	var cols []string
	addCol := func(k string) {
		if _, ok := colOrder[k]; !ok {
			colOrder[k] = len(cols)
			cols = append(cols, k)
		}
	}
	for _, rn := range rowNodes {
		attrs := make([]string, 0, len(rn.Attr))
		for k := range rn.Attr {
			attrs = append(attrs, k)
		}
		sort.Strings(attrs)
		for _, k := range attrs {
			addCol("@" + k)
		}
		for _, ch := range rn.Children {
			if len(ch.Children) == 0 {
				addCol(ch.Name)
			}
		}
	}
	rows := make([][]string, 0, len(rowNodes))
	for _, rn := range rowNodes {
		row := make([]string, len(cols))
		vals := map[string]string{}
		for k, v := range rn.Attr {
			vals["@"+k] = v
		}
		for _, ch := range rn.Children {
			if len(ch.Children) == 0 {
				txt := strings.TrimSpace(ch.Text)
				if _, ok := vals[ch.Name]; !ok || vals[ch.Name] == "" {
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

func findRows(n *xmlNode) []*xmlNode {
	var best []*xmlNode
	var walk func(*xmlNode)
	walk = func(node *xmlNode) {
		if best != nil {
			return
		}
		groups := map[string][]*xmlNode{}
		for _, c := range node.Children {
			groups[c.Name] = append(groups[c.Name], c)
		}
		for _, kids := range groups {
			if len(kids) >= 2 {
				best = kids
				return
			}
		}
		for _, c := range node.Children {
			walk(c)
		}
	}
	walk(n)
	return best
}
