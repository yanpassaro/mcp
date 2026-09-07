package sandbox

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	lua "github.com/Shopify/go-lua"
)

type xmlNode struct {
	name     string
	attrs    map[string]string
	chars    strings.Builder
	children []*xmlNode
}

func buildXML(L *lua.State) int {
	t := newTable(L)

	setGoFunc(L, t, "parse", func(l *lua.State) int {
		root, err := parseXML(argString(l, 1))
		if err != nil {
			panic(fmt.Errorf("XML inválido: %w", err))
		}
		pushAny(l, xmlNodeToAny(root))
		return 1
	})

	setGoFunc(L, t, "stringify", func(l *lua.State) int {
		n, err := xmlNodeFromAny(luaToAny(l, 1))
		if err != nil {
			panic(err)
		}
		var b strings.Builder
		writeXMLNode(&b, n)
		l.PushString(b.String())
		return 1
	})

	return t
}

func parseXML(s string) (*xmlNode, error) {
	dec := xml.NewDecoder(strings.NewReader(s))
	var root *xmlNode
	stack := []*xmlNode{}
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			n := &xmlNode{name: t.Name.Local, attrs: map[string]string{}}
			for _, a := range t.Attr {
				n.attrs[a.Name.Local] = a.Value
			}
			if len(stack) == 0 {
				root = n
			} else {
				parent := stack[len(stack)-1]
				parent.children = append(parent.children, n)
			}
			stack = append(stack, n)
		case xml.CharData:
			if len(stack) > 0 {
				stack[len(stack)-1].chars.Write([]byte(t))
			}
		case xml.EndElement:
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		}
	}
	if root == nil {
		return nil, errors.New("XML sem elemento raiz")
	}
	return root, nil
}

func xmlNodeToAny(n *xmlNode) map[string]any {
	m := map[string]any{"name": n.name}
	if len(n.attrs) > 0 {
		attrs := make(map[string]any, len(n.attrs))
		for k, v := range n.attrs {
			attrs[k] = v
		}
		m["attrs"] = attrs
	}
	if text := strings.TrimSpace(n.chars.String()); text != "" {
		m["text"] = text
	}
	if len(n.children) > 0 {
		ch := make([]any, len(n.children))
		for i, c := range n.children {
			ch[i] = xmlNodeToAny(c)
		}
		m["children"] = ch
	}
	return m
}

func xmlNodeFromAny(v any) (*xmlNode, error) {
	m, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("nó XML deve ser uma tabela (com 'name')")
	}
	name, _ := m["name"].(string)
	if name == "" {
		return nil, fmt.Errorf("nó XML sem 'name'")
	}
	n := &xmlNode{name: name, attrs: map[string]string{}}
	if a, ok := m["attrs"].(map[string]any); ok {
		for k, val := range a {
			n.attrs[k] = fmt.Sprint(val)
		}
	}
	if txt, ok := m["text"]; ok && txt != nil {
		n.chars.WriteString(fmt.Sprint(txt))
	}
	if c, ok := m["children"].([]any); ok {
		for _, item := range c {
			child, err := xmlNodeFromAny(item)
			if err != nil {
				return nil, err
			}
			n.children = append(n.children, child)
		}
	}
	return n, nil
}

func writeXMLNode(b *strings.Builder, n *xmlNode) {
	b.WriteByte('<')
	b.WriteString(n.name)
	for _, k := range sortedKeys(n.attrs) {
		b.WriteByte(' ')
		b.WriteString(k)
		b.WriteString(`="`)
		xmlEscape(b, n.attrs[k])
		b.WriteByte('"')
	}
	if len(n.children) == 0 {
		if text := n.chars.String(); text != "" {
			b.WriteByte('>')
			xmlEscape(b, text)
			b.WriteString("</")
			b.WriteString(n.name)
			b.WriteByte('>')
		} else {
			b.WriteString("/>")
		}
		return
	}
	b.WriteByte('>')
	if text := n.chars.String(); text != "" {
		xmlEscape(b, text)
	}
	for _, c := range n.children {
		writeXMLNode(b, c)
	}
	b.WriteString("</")
	b.WriteString(n.name)
	b.WriteByte('>')
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func xmlEscape(b *strings.Builder, s string) {
	var buf bytes.Buffer
	_ = xml.EscapeText(&buf, []byte(s))
	b.WriteString(buf.String())
}
