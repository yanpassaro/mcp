package sandbox

import (
	"strconv"
	"strings"

	lua "github.com/Shopify/go-lua"
)

type diffOp struct {
	kind byte   // ' ', '+', '-'
	a, b string
}

func buildDiff(L *lua.State) int {
	t := newTable(L)

	setGoFunc(L, t, "unified", func(l *lua.State) int {
		a := argString(l, 1)
		b := argString(l, 2)
		opts := toAnyMap(l, 3)
		ctx := int(optUint(opts, "context", 3))
		from := optString(opts, "from")
		if from == "" {
			from = "a"
		}
		to := optString(opts, "to")
		if to == "" {
			to = "b"
		}
		l.PushString(unifiedDiff(a, b, ctx, from, to))
		return 1
	})

	setGoFunc(L, t, "lines", func(l *lua.State) int {
		pushAny(l, lineOpsToArray(computeDiff(splitLines(argString(l, 1)), splitLines(argString(l, 2)))))
		return 1
	})

	setGoFunc(L, t, "ratio", func(l *lua.State) int {
		l.PushNumber(cleanFloat(stringRatio(argString(l, 1), argString(l, 2))))
		return 1
	})

	return t
}

func unifiedDiff(a, b string, ctx int, from, to string) string {
	if a == b {
		return "sem diferenças"
	}
	if ctx < 0 {
		ctx = 0
	}
	ops := computeDiff(splitLines(a), splitLines(b))

	segs := changeSegments(ops)
	if len(segs) == 0 {
		return "sem diferenças"
	}

	type hunk struct{ start, end int }
	var hunks []hunk
	for _, s := range segs {
		st := s[0] - ctx
		if st < 0 {
			st = 0
		}
		en := s[1] + ctx
		if en > len(ops) {
			en = len(ops)
		}
		if len(hunks) > 0 && st <= hunks[len(hunks)-1].end {
			if en > hunks[len(hunks)-1].end {
				hunks[len(hunks)-1].end = en
			}
			continue
		}
		hunks = append(hunks, hunk{st, en})
	}

	aCum := make([]int, len(ops)+1)
	bCum := make([]int, len(ops)+1)
	for k, o := range ops {
		aCum[k+1] = aCum[k]
		bCum[k+1] = bCum[k]
		if o.kind != '+' {
			aCum[k+1]++
		}
		if o.kind != '-' {
			bCum[k+1]++
		}
	}

	var sb strings.Builder
	sb.WriteString("--- ")
	sb.WriteString(from)
	sb.WriteString("\n")
	sb.WriteString("+++ ")
	sb.WriteString(to)
	sb.WriteString("\n")
	for _, h := range hunks {
		aS := aCum[h.start] + 1
		bS := bCum[h.start] + 1
		aC := aCum[h.end] - aCum[h.start]
		bC := bCum[h.end] - bCum[h.start]
		sb.WriteString("@@ ")
		sb.WriteString(rangeHeader(aS, aC))
		sb.WriteString(" ")
		sb.WriteString(rangeHeader(bS, bC))
		sb.WriteString(" @@\n")
		for k := h.start; k < h.end; k++ {
			switch o := ops[k]; o.kind {
			case ' ':
				sb.WriteString(" ")
				sb.WriteString(o.a)
				sb.WriteString("\n")
			case '-':
				sb.WriteString("-")
				sb.WriteString(o.a)
				sb.WriteString("\n")
			case '+':
				sb.WriteString("+")
				sb.WriteString(o.b)
				sb.WriteString("\n")
			}
		}
	}
	return sb.String()
}

func rangeHeader(line, count int) string {
	if count == 1 {
		return strconv.Itoa(line)
	}
	return strconv.Itoa(line) + "," + strconv.Itoa(count)
}

func computeDiff(a, b []string) []diffOp {
	pre := 0
	for pre < len(a) && pre < len(b) && a[pre] == b[pre] {
		pre++
	}
	suf := 0
	for suf < len(a)-pre && suf < len(b)-pre && a[len(a)-1-suf] == b[len(b)-1-suf] {
		suf++
	}
	midA := a[pre : len(a)-suf]
	midB := b[pre : len(b)-suf]

	op := make([]diffOp, 0, len(a)+len(b))
	for i := 0; i < pre; i++ {
		op = append(op, diffOp{' ', a[i], a[i]})
	}
	op = append(op, diffCore(midA, midB)...)
	for i := suf; i > 0; i-- {
		op = append(op, diffOp{' ', a[len(a)-i], b[len(b)-i]})
	}
	return op
}

func diffCore(a, b []string) []diffOp {
	n, m := len(a), len(b)
	if n*m > 2_000_000 {
		op := make([]diffOp, 0, n+m)
		for _, x := range a {
			op = append(op, diffOp{'-', x, ""})
		}
		for _, y := range b {
			op = append(op, diffOp{'+', "", y})
		}
		return op
	}
	dp := make([][]int, n+1)
	for i := range dp {
		dp[i] = make([]int, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if a[i] == b[j] {
				dp[i][j] = dp[i+1][j+1] + 1
			} else if dp[i+1][j] >= dp[i][j+1] {
				dp[i][j] = dp[i+1][j]
			} else {
				dp[i][j] = dp[i][j+1]
			}
		}
	}
	op := make([]diffOp, 0, n+m)
	i, j := 0, 0
	for i < n && j < m {
		if a[i] == b[j] {
			op = append(op, diffOp{' ', a[i], b[j]})
			i++
			j++
		} else if dp[i+1][j] >= dp[i][j+1] {
			op = append(op, diffOp{'-', a[i], ""})
			i++
		} else {
			op = append(op, diffOp{'+', "", b[j]})
			j++
		}
	}
	for i < n {
		op = append(op, diffOp{'-', a[i], ""})
		i++
	}
	for j < m {
		op = append(op, diffOp{'+', "", b[j]})
		j++
	}
	return op
}

func changeSegments(ops []diffOp) [][2]int {
	var segs [][2]int
	i := 0
	for i < len(ops) {
		if ops[i].kind != ' ' {
			j := i
			for j < len(ops) && ops[j].kind != ' ' {
				j++
			}
			segs = append(segs, [2]int{i, j})
			i = j
		} else {
			i++
		}
	}
	return segs
}

func lineOpsToArray(ops []diffOp) []any {
	out := make([]any, 0, len(ops))
	for _, o := range ops {
		opName, text := "equal", o.a
		switch o.kind {
		case '-':
			opName = "del"
		case '+':
			opName, text = "add", o.b
		}
		out = append(out, map[string]any{"op": opName, "text": text})
	}
	return out
}

func stringRatio(a, b string) float64 {
	ra, rb := []rune(a), []rune(b)
	if len(ra) == 0 && len(rb) == 0 {
		return 1
	}
	if len(ra) == 0 || len(rb) == 0 {
		return 0
	}
	if len(ra)*len(rb) > 2_000_000 {
		return prefixSuffixRatio(ra, rb)
	}
	prev := make([]int, len(rb)+1)
	cur := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		cur[0] = i
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			best := prev[j] + 1
			if cur[j-1]+1 < best {
				best = cur[j-1] + 1
			}
			if prev[j-1]+cost < best {
				best = prev[j-1] + cost
			}
			cur[j] = best
		}
		prev, cur = cur, prev
	}
	maxLen := len(ra)
	if len(rb) > maxLen {
		maxLen = len(rb)
	}
	return 1 - float64(prev[len(rb)])/float64(maxLen)
}

func prefixSuffixRatio(a, b []rune) float64 {
	pre := 0
	for pre < len(a) && pre < len(b) && a[pre] == b[pre] {
		pre++
	}
	suf := 0
	for suf < len(a)-pre && suf < len(b)-pre && a[len(a)-1-suf] == b[len(b)-1-suf] {
		suf++
	}
	denom := float64(len(a) + len(b))
	if denom == 0 {
		return 1
	}
	return 2 * float64(pre+suf) / denom
}

func splitLines(s string) []string {
	if s == "" {
		return []string{}
	}
	parts := strings.Split(s, "\n")
	if len(parts) > 0 && parts[len(parts)-1] == "" {
		parts = parts[:len(parts)-1]
	}
	return parts
}
