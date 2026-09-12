package sandbox

import (
	"fmt"
	"sort"
	"strings"

	lua "github.com/Shopify/go-lua"
)

type pipeBuilder struct {
	rows  []any
	group []string
	aggs  []any
}

func buildPipe(L *lua.State) int {
	t := newTable(L)

	setGoFunc(L, t, "rows", func(l *lua.State) int {
		sb := &pipeBuilder{rows: luaArrayAny(l, 1)}
		b := newTable(L)

		self := func(l *lua.State) int {
			l.PushValue(1)
			return 1
		}

		setGoFunc(L, b, "group", func(l *lua.State) int {
			sb.group = stringSlice(luaToAny(l, 2))
			return self(l)
		})
		addAgg := func(name string) func(*lua.State) int {
			return func(l *lua.State) int {
				src := argString(l, 2)
				dest := argString(l, 3)
				if dest == "" {
					dest = name
				}
				sb.aggs = append(sb.aggs, []any{name, src, dest})
				return self(l)
			}
		}
		setGoFunc(L, b, "count", addAgg("count"))
		setGoFunc(L, b, "sum", addAgg("sum"))
		setGoFunc(L, b, "avg", addAgg("avg"))
		setGoFunc(L, b, "min", addAgg("min"))
		setGoFunc(L, b, "max", addAgg("max"))
		setGoFunc(L, b, "median", addAgg("median"))

		setGoFunc(L, b, "select", func(l *lua.State) int {
			sb.rows = pipeSelect(sb.rows, stringSlice(luaToAny(l, 2)))
			return self(l)
		})
		setGoFunc(L, b, "rename", func(l *lua.State) int {
			sb.rows = pipeRename(sb.rows, toAnyMap(l, 2))
			return self(l)
		})
		setGoFunc(L, b, "drop", func(l *lua.State) int {
			sb.rows = pipeDrop(sb.rows, stringSlice(luaToAny(l, 2)))
			return self(l)
		})
		setGoFunc(L, b, "sort", func(l *lua.State) int {
			sb.rows = pipeSort(sb.rows, stringSlice(luaToAny(l, 2)), argBool(l, 3))
			return self(l)
		})
		setGoFunc(L, b, "distinct", func(l *lua.State) int {
			sb.rows = pipeDistinct(sb.rows, stringSlice(luaToAny(l, 2)))
			return self(l)
		})
		setGoFunc(L, b, "take", func(l *lua.State) int {
			sb.rows = pipeTake(sb.rows, int(argNum(l, 2)))
			return self(l)
		})
		setGoFunc(L, b, "filter", func(l *lua.State) int {
			sb.rows = pipeFilter(l, sb.rows, l.AbsIndex(2))
			return self(l)
		})
		setGoFunc(L, b, "map", func(l *lua.State) int {
			sb.rows = pipeMap(l, sb.rows, l.AbsIndex(2))
			return self(l)
		})
		setGoFunc(L, b, "join", func(l *lua.State) int {
			sb.rows = pipeJoin(sb.rows, luaArrayAny(l, 2), toAnyMap(l, 3))
			return self(l)
		})
		setGoFunc(L, b, "coerce", func(l *lua.State) int {
			sb.rows = pipeCoerce(sb.rows, argString(l, 2), argString(l, 3))
			return self(l)
		})
		setGoFunc(L, b, "infer", func(l *lua.State) int {
			sb.rows = pipeInfer(sb.rows)
			return self(l)
		})
		setGoFunc(L, b, "pivot", func(l *lua.State) int {
			sb.rows = pipePivot(sb.rows, toAnyMap(l, 2))
			return self(l)
		})
		setGoFunc(L, b, "unpivot", func(l *lua.State) int {
			sb.rows = pipeUnpivot(sb.rows, toAnyMap(l, 2))
			return self(l)
		})
		setGoFunc(L, b, "run", func(l *lua.State) int {
			if len(sb.group) > 0 || len(sb.aggs) > 0 {
				pushAny(l, pipeGroup(sb.rows, sb.group, sb.aggs))
			} else {
				pushAny(l, sb.rows)
			}
			return 1
		})

		return 1
	})

	setGoFunc(L, t, "map", func(l *lua.State) int {
		pushAny(l, pipeMap(l, luaArrayAny(l, 1), l.AbsIndex(2)))
		return 1
	})
	setGoFunc(L, t, "filter", func(l *lua.State) int {
		pushAny(l, pipeFilter(l, luaArrayAny(l, 1), l.AbsIndex(2)))
		return 1
	})
	setGoFunc(L, t, "rename", func(l *lua.State) int {
		pushAny(l, pipeRename(luaArrayAny(l, 1), toAnyMap(l, 2)))
		return 1
	})
	setGoFunc(L, t, "select", func(l *lua.State) int {
		pushAny(l, pipeSelect(luaArrayAny(l, 1), stringSlice(luaToAny(l, 2))))
		return 1
	})
	setGoFunc(L, t, "drop", func(l *lua.State) int {
		pushAny(l, pipeDrop(luaArrayAny(l, 1), stringSlice(luaToAny(l, 2))))
		return 1
	})
	setGoFunc(L, t, "sort", func(l *lua.State) int {
		pushAny(l, pipeSort(luaArrayAny(l, 1), stringSlice(luaToAny(l, 2)), argBool(l, 3)))
		return 1
	})
	setGoFunc(L, t, "distinct", func(l *lua.State) int {
		pushAny(l, pipeDistinct(luaArrayAny(l, 1), stringSlice(luaToAny(l, 2))))
		return 1
	})
	setGoFunc(L, t, "take", func(l *lua.State) int {
		pushAny(l, pipeTake(luaArrayAny(l, 1), int(argNum(l, 2))))
		return 1
	})
	setGoFunc(L, t, "group", func(l *lua.State) int {
		pushAny(l, pipeGroup(luaArrayAny(l, 1), stringSlice(luaToAny(l, 2)), luaArrayAny(l, 3)))
		return 1
	})
	setGoFunc(L, t, "join", func(l *lua.State) int {
		pushAny(l, pipeJoin(luaArrayAny(l, 1), luaArrayAny(l, 2), toAnyMap(l, 3)))
		return 1
	})
	setGoFunc(L, t, "coerce", func(l *lua.State) int {
		pushAny(l, pipeCoerce(luaArrayAny(l, 1), argString(l, 2), argString(l, 3)))
		return 1
	})
	setGoFunc(L, t, "infer", func(l *lua.State) int {
		pushAny(l, pipeInfer(luaArrayAny(l, 1)))
		return 1
	})
	setGoFunc(L, t, "pivot", func(l *lua.State) int {
		pushAny(l, pipePivot(luaArrayAny(l, 1), toAnyMap(l, 2)))
		return 1
	})
	setGoFunc(L, t, "unpivot", func(l *lua.State) int {
		pushAny(l, pipeUnpivot(luaArrayAny(l, 1), toAnyMap(l, 2)))
		return 1
	})

	return t
}

func pipeMap(l *lua.State, rows []any, fn int) []any {
	out := []any{}
	for _, r := range rows {
		res, err := luaCall(l, fn, r)
		if err != nil {
			panic(err)
		}
		if res == nil {
			continue
		}
		out = append(out, res)
	}
	return out
}

func pipeFilter(l *lua.State, rows []any, fn int) []any {
	out := []any{}
	for _, r := range rows {
		ok, err := luaCall(l, fn, r)
		if err != nil {
			panic(err)
		}
		if truthy(ok) {
			out = append(out, r)
		}
	}
	return out
}

func pipeRename(rows []any, mapping map[string]any) []any {
	out := make([]any, 0, len(rows))
	for _, r := range rows {
		m, ok := r.(map[string]any)
		if !ok {
			out = append(out, r)
			continue
		}
		nm := map[string]any{}
		for k, v := range m {
			if nk, has := mapping[k]; has {
				nm[fmt.Sprint(nk)] = v
			} else {
				nm[k] = v
			}
		}
		out = append(out, nm)
	}
	return out
}

func pipeSelect(rows []any, fields []string) []any {
	out := make([]any, 0, len(rows))
	for _, r := range rows {
		m, ok := r.(map[string]any)
		if !ok {
			continue
		}
		nm := map[string]any{}
		for _, f := range fields {
			if v, ok := m[f]; ok {
				nm[f] = v
			}
		}
		out = append(out, nm)
	}
	return out
}

func pipeDrop(rows []any, fields []string) []any {
	drop := map[string]bool{}
	for _, f := range fields {
		drop[f] = true
	}
	out := make([]any, 0, len(rows))
	for _, r := range rows {
		m, ok := r.(map[string]any)
		if !ok {
			out = append(out, r)
			continue
		}
		nm := map[string]any{}
		for k, v := range m {
			if !drop[k] {
				nm[k] = v
			}
		}
		out = append(out, nm)
	}
	return out
}

func pipeSort(rows []any, fields []string, desc bool) []any {
	cp := append([]any(nil), rows...)
	sort.SliceStable(cp, func(i, j int) bool {
		for _, f := range fields {
			c := cmpAny(itemProp(cp[i], f), itemProp(cp[j], f))
			if c != 0 {
				if desc {
					return c > 0
				}
				return c < 0
			}
		}
		return false
	})
	return cp
}

func pipeDistinct(rows []any, fields []string) []any {
	if len(fields) == 0 {
		fields = rowFields(rows)
	}
	seen := map[string]bool{}
	out := []any{}
	for _, r := range rows {
		m, ok := r.(map[string]any)
		if !ok {
			out = append(out, r)
			continue
		}
		parts := make([]string, len(fields))
		for i, f := range fields {
			parts[i] = fmt.Sprint(itemProp(m, f))
		}
		key := strings.Join(parts, "\x00")
		if !seen[key] {
			seen[key] = true
			out = append(out, m)
		}
	}
	return out
}

func pipeTake(rows []any, n int) []any {
	if n < 0 {
		n = 0
	}
	if n > len(rows) {
		n = len(rows)
	}
	return rows[:n]
}

func pipeCoerce(rows []any, field, typ string) []any {
	if typ == "" {
		vals := make([]any, 0, len(rows))
		for _, r := range rows {
			if m, ok := r.(map[string]any); ok {
				vals = append(vals, m[field])
			}
		}
		typ = inferType(vals)
	}
	out := make([]any, 0, len(rows))
	for _, r := range rows {
		m, ok := r.(map[string]any)
		if !ok {
			out = append(out, r)
			continue
		}
		nm := map[string]any{}
		for k, v := range m {
			nm[k] = v
		}
		nm[field] = coerceValue(m[field], typ)
		out = append(out, nm)
	}
	return out
}

func pipeGroup(rows []any, fields []string, aggs []any) []any {
	type group struct {
		key string
		all []any
	}
	groups := map[string]*group{}
	var order []string
	for _, r := range rows {
		m, ok := r.(map[string]any)
		if !ok {
			continue
		}
		key := ""
		if len(fields) > 0 {
			parts := make([]string, len(fields))
			for i, f := range fields {
				parts[i] = fmt.Sprint(itemProp(m, f))
			}
			key = strings.Join(parts, "\x00")
		}
		g, exists := groups[key]
		if !exists {
			g = &group{key: key}
			groups[key] = g
			order = append(order, key)
		}
		g.all = append(g.all, r)
	}
	out := []any{}
	for _, key := range order {
		g := groups[key]
		row := map[string]any{}
		if len(fields) > 0 {
			first := g.all[0].(map[string]any)
			for _, f := range fields {
				row[f] = itemProp(first, f)
			}
		}
		for _, a := range aggs {
			arr, ok := a.([]any)
			if !ok || len(arr) < 3 {
				continue
			}
			aggFn := fmt.Sprint(arr[0])
			src := fmt.Sprint(arr[1])
			dest := fmt.Sprint(arr[2])
			if aggFn == "count" {
				row[dest] = float64(len(g.all))
				continue
			}
			var nums []float64
			for _, r := range g.all {
				if n, ok := numOrNil(itemProp(r, src)); ok {
					nums = append(nums, n)
				}
			}
			row[dest] = aggregate(aggFn, nums)
		}
		out = append(out, row)
	}
	return out
}

func aggregate(fn string, nums []float64) any {
	switch fn {
	case "sum":
		var s float64
		for _, n := range nums {
			s += n
		}
		return cleanFloat(s)
	case "avg":
		if len(nums) == 0 {
			return nil
		}
		return cleanFloat(mean(nums))
	case "min":
		if len(nums) == 0 {
			return nil
		}
		m := nums[0]
		for _, n := range nums[1:] {
			if n < m {
				m = n
			}
		}
		return cleanFloat(m)
	case "max":
		if len(nums) == 0 {
			return nil
		}
		m := nums[0]
		for _, n := range nums[1:] {
			if n > m {
				m = n
			}
		}
		return cleanFloat(m)
	case "median":
		if len(nums) == 0 {
			return nil
		}
		return cleanFloat(quantile(nums, 0.5))
	}
	return nil
}

func pipeJoin(left, right []any, opts map[string]any) []any {
	leftKey := optString(opts, "left_key")
	rightKey := optString(opts, "right_key")
	if leftKey == "" || rightKey == "" {
		panic("pipe.join: left_key e right_key são obrigatórios")
	}
	how := optString(opts, "how")
	if how == "" {
		how = "inner"
	}
	rightIdx := map[string][]any{}
	for _, r := range right {
		if m, ok := r.(map[string]any); ok {
			k := fmt.Sprint(m[rightKey])
			rightIdx[k] = append(rightIdx[k], m)
		}
	}
	out := []any{}
	for _, l := range left {
		lm, ok := l.(map[string]any)
		if !ok {
			continue
		}
		k := fmt.Sprint(lm[leftKey])
		matches := rightIdx[k]
		if len(matches) == 0 {
			if how == "left" {
				out = append(out, lm)
			}
			continue
		}
		for _, rm := range matches {
			merged := map[string]any{}
			for kk, vv := range lm {
				merged[kk] = vv
			}
			for kk, vv := range rm.(map[string]any) {
				merged[kk] = vv
			}
			out = append(out, merged)
		}
	}
	return out
}

func pipeInfer(rows []any) []any {
	out := make([]any, 0, len(rows))
	if len(rows) == 0 {
		return out
	}
	fields := rowFields(rows)
	types := map[string]string{}
	for _, f := range fields {
		vals := make([]any, 0, len(rows))
		for _, r := range rows {
			if m, ok := r.(map[string]any); ok {
				vals = append(vals, m[f])
			}
		}
		types[f] = inferType(vals)
	}
	for _, r := range rows {
		m, ok := r.(map[string]any)
		if !ok {
			out = append(out, r)
			continue
		}
		nm := map[string]any{}
		for k, v := range m {
			nm[k] = coerceValue(v, types[k])
		}
		out = append(out, nm)
	}
	return out
}

func pipePivot(rows []any, opts map[string]any) []any {
	index := stringSlice(opts["index"])
	columns := optString(opts, "columns")
	values := optString(opts, "values")
	agg := optString(opts, "agg")
	if agg == "" {
		if values == "" {
			agg = "count"
		} else {
			agg = "sum"
		}
	}
	if columns == "" {
		panic("pipe.pivot: columns é obrigatório")
	}
	idxRows := map[string]map[string]any{}
	var order []string
	colSet := map[string]bool{}
	var colOrder []string
	groups := map[string]map[string][]any{}
	for _, r := range rows {
		m, ok := r.(map[string]any)
		if !ok {
			continue
		}
		parts := make([]string, len(index))
		for i, f := range index {
			parts[i] = fmt.Sprint(itemProp(m, f))
		}
		idxKey := strings.Join(parts, "\x00")
		cKey := fmt.Sprint(itemProp(m, columns))
		if _, ok := idxRows[idxKey]; !ok {
			row := map[string]any{}
			for _, f := range index {
				row[f] = itemProp(m, f)
			}
			idxRows[idxKey] = row
			order = append(order, idxKey)
		}
		if !colSet[cKey] {
			colSet[cKey] = true
			colOrder = append(colOrder, cKey)
		}
		if groups[idxKey] == nil {
			groups[idxKey] = map[string][]any{}
		}
		groups[idxKey][cKey] = append(groups[idxKey][cKey], itemProp(m, values))
	}
	out := []any{}
	for _, idxKey := range order {
		row := idxRows[idxKey]
		for _, cKey := range colOrder {
			row[cKey] = pivotCell(agg, groups[idxKey][cKey])
		}
		out = append(out, row)
	}
	return out
}

func pivotCell(agg string, vals []any) any {
	switch agg {
	case "count":
		return float64(len(vals))
	case "first":
		if len(vals) > 0 {
			return vals[0]
		}
		return nil
	case "last":
		if len(vals) > 0 {
			return vals[len(vals)-1]
		}
		return nil
	default:
		var nums []float64
		for _, v := range vals {
			if n, ok := numOrNil(v); ok {
				nums = append(nums, n)
			}
		}
		return aggregate(agg, nums)
	}
}

func pipeUnpivot(rows []any, opts map[string]any) []any {
	id := stringSlice(opts["id"])
	cols := stringSlice(opts["cols"])
	key := optString(opts, "key")
	if key == "" {
		key = "key"
	}
	value := optString(opts, "value")
	if value == "" {
		value = "value"
	}
	dropNil := asBool(opts["drop_nil"], false)
	if len(cols) == 0 {
		set := map[string]bool{}
		for _, f := range id {
			set[f] = true
		}
		seen := map[string]bool{}
		for _, r := range rows {
			if m, ok := r.(map[string]any); ok {
				for k := range m {
					if !set[k] && !seen[k] {
						cols = append(cols, k)
						seen[k] = true
					}
				}
			}
		}
		sort.Strings(cols)
	}
	out := []any{}
	for _, r := range rows {
		m, ok := r.(map[string]any)
		if !ok {
			continue
		}
		base := map[string]any{}
		for _, f := range id {
			base[f] = itemProp(m, f)
		}
		for _, c := range cols {
			cell := itemProp(m, c)
			if dropNil && isNullVal(cell, map[string]any{}) {
				continue
			}
			nm := map[string]any{}
			for k, v := range base {
				nm[k] = v
			}
			nm[key] = c
			nm[value] = cell
			out = append(out, nm)
		}
	}
	return out
}
