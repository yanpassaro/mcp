package sandbox

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"

	lua "github.com/Shopify/go-lua"
)

const (
	tokText = iota
	tokVar
	tokIf
	tokEach
	tokUnless
	tokElse
	tokEnd
)

type tmplTok struct {
	kind int
	text string
	path string
}

type tmplNode struct {
	kind int
	text string
	path string
	then []*tmplNode
	els  []*tmplNode
}

func buildTemplate(L *lua.State) int {
	t := newTable(L)
	setGoFunc(L, t, "render", func(l *lua.State) int {
		l.PushString(renderTemplate(argString(l, 1), toAnyMap(l, 2)))
		return 1
	})
	return t
}

func renderTemplate(s string, vars map[string]any) string {
	return renderNodes(parseTemplate(tokenizeTemplate(s)), vars)
}

func tokenizeTemplate(s string) []tmplTok {
	var toks []tmplTok
	var lit strings.Builder
	flush := func() {
		if lit.Len() > 0 {
			toks = append(toks, tmplTok{kind: tokText, text: lit.String()})
			lit.Reset()
		}
	}
	addBlock := func(inner string) {
		inner = strings.TrimSpace(inner)
		switch {
		case inner == "else":
			toks = append(toks, tmplTok{kind: tokElse})
		case strings.HasPrefix(inner, "/"):
			toks = append(toks, tmplTok{kind: tokEnd, text: strings.TrimSpace(inner[1:])})
		case strings.HasPrefix(inner, "#each"):
			toks = append(toks, tmplTok{kind: tokEach, path: strings.TrimSpace(strings.TrimPrefix(inner, "#each"))})
		case strings.HasPrefix(inner, "#if"):
			toks = append(toks, tmplTok{kind: tokIf, path: strings.TrimSpace(strings.TrimPrefix(inner, "#if"))})
		case strings.HasPrefix(inner, "#unless"):
			toks = append(toks, tmplTok{kind: tokUnless, path: strings.TrimSpace(strings.TrimPrefix(inner, "#unless"))})
		case inner != "":
			toks = append(toks, tmplTok{kind: tokVar, path: inner})
		}
	}
	i := 0
	for i < len(s) {
		if s[i] == '{' {
			if i+1 < len(s) {
				if s[i+1] == '{' {
					end := strings.Index(s[i+2:], "}}")
					if end >= 0 {
						flush()
						addBlock(s[i+2 : i+2+end])
						i += 2 + end + 2
						continue
					}
				}
			}
			if end := strings.Index(s[i+1:], "}"); end >= 0 {
				flush()
				addBlock(s[i+1 : i+1+end])
				i += 1 + end + 1
				continue
			}
		}
		lit.WriteByte(s[i])
		i++
	}
	flush()
	return toks
}

func parseTemplate(toks []tmplTok) []*tmplNode {
	root := []*tmplNode{}
	type open struct {
		node   *tmplNode
		branch int
	}
	stack := []open{}
	cur := &root
	for _, tok := range toks {
		switch tok.kind {
		case tokText:
			*cur = append(*cur, &tmplNode{kind: tokText, text: tok.text})
		case tokVar:
			*cur = append(*cur, &tmplNode{kind: tokVar, path: tok.path})
		case tokIf, tokEach, tokUnless:
			n := &tmplNode{kind: tok.kind, path: tok.path}
			*cur = append(*cur, n)
			stack = append(stack, open{node: n, branch: 0})
			cur = &n.then
		case tokElse:
			if len(stack) > 0 {
				top := &stack[len(stack)-1]
				top.branch = 1
				cur = &top.node.els
			}
		case tokEnd:
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
			if len(stack) == 0 {
				cur = &root
			}
			if len(stack) != 0 {
				top := stack[len(stack)-1]
				cur = &top.node.then
				if top.branch != 0 {
					cur = &top.node.els
				}
			}
		}
	}
	return root
}

func renderNodes(nodes []*tmplNode, ctx map[string]any) string {
	b := strings.Builder{}
	for _, n := range nodes {
		switch n.kind {
		case tokText:
			b.WriteString(n.text)
		case tokVar:
			if v, ok := jsonPath(ctx, n.path); ok {
				b.WriteString(tmplText(v))
			}
		case tokIf:
			v, ok := jsonPath(ctx, n.path)
			if ok {
				if truthyVal(v) {
					b.WriteString(renderNodes(n.then, ctx))
					break
				}
			}
			b.WriteString(renderNodes(n.els, ctx))
		case tokUnless:
			v, ok := jsonPath(ctx, n.path)
			if !ok {
				b.WriteString(renderNodes(n.then, ctx))
				break
			}
			if !truthyVal(v) {
				b.WriteString(renderNodes(n.then, ctx))
				break
			}
			b.WriteString(renderNodes(n.els, ctx))
		case tokEach:
			v, ok := jsonPath(ctx, n.path)
			arr, isArr := v.([]any)
			if !ok {
				b.WriteString(renderNodes(n.els, ctx))
				break
			}
			if !isArr {
				b.WriteString(renderNodes(n.els, ctx))
				break
			}
			if len(arr) == 0 {
				b.WriteString(renderNodes(n.els, ctx))
				break
			}
			for i, item := range arr {
				m := map[string]any{}
				for k, v := range ctx {
					m[k] = v
				}
				if im, ok := item.(map[string]any); ok {
					for k, v := range im {
						m[k] = v
					}
				}
				m["this"] = item
				m["@index"] = float64(i)
				b.WriteString(renderNodes(n.then, m))
			}
		}
	}
	return b.String()
}

func truthyVal(v any) bool {
	if v == nil {
		return false
	}
	if b, ok := v.(bool); ok {
		return b
	}
	return true
}

func tmplText(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case bool:
		if x {
			return "true"
		}
		return "false"
	case float64:
		if math.Abs(x) < 1e15 {
			if x == math.Trunc(x) {
				return strconv.FormatInt(int64(x), 10)
			}
		}
		return strconv.FormatFloat(x, 'f', -1, 64)
	case map[string]any, []any:
		if b, err := json.Marshal(x); err == nil {
			return string(b)
		}
	}
	return fmt.Sprint(v)
}
