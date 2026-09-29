package domain

import "strings"

type edit struct {
	kind string
	text string
}

// myers shares one bounded work budget across line and codepoint refinement.
func myers(a, b []string, budget *int) ([]edit, bool) {
	n, m := len(a), len(b)
	maxDistance := n + m
	v := map[int]int{1: 0}
	trace := make([]map[int]int, 0)
	for d := 0; d <= maxDistance; d++ {
		if *budget <= 0 {
			return nil, false
		}
		snapshot := make(map[int]int, len(v))
		for k, x := range v {
			snapshot[k] = x
			*budget--
		}
		trace = append(trace, snapshot)
		for k := -d; k <= d; k += 2 {
			*budget--
			if *budget < 0 {
				return nil, false
			}
			var x int
			if k == -d || k != d && v[k-1] < v[k+1] {
				x = v[k+1]
			} else {
				x = v[k-1] + 1
			}
			y := x - k
			for x < n && y < m && a[x] == b[y] {
				x++
				y++
				*budget--
				if *budget < 0 {
					return nil, false
				}
			}
			v[k] = x
			if x >= n && y >= m {
				edits := make([]edit, 0, n+m)
				x, y = n, m
				for dd := d; dd >= 0; dd-- {
					vv := trace[dd]
					kk := x - y
					var pk int
					if kk == -dd || kk != dd && vv[kk-1] < vv[kk+1] {
						pk = kk + 1
					} else {
						pk = kk - 1
					}
					px := vv[pk]
					py := px - pk
					for x > px && y > py {
						x--
						y--
						edits = append(edits, edit{"equal", a[x]})
					}
					if dd == 0 {
						break
					}
					if x == px {
						y--
						edits = append(edits, edit{"insert", b[y]})
					} else {
						x--
						edits = append(edits, edit{"delete", a[x]})
					}
				}
				for i, j := 0, len(edits)-1; i < j; i, j = i+1, j-1 {
					edits[i], edits[j] = edits[j], edits[i]
				}
				return edits, true
			}
		}
	}
	return nil, false
}
func tokens(s string) []string {
	out := make([]string, 0, len(s))
	for _, r := range s {
		out = append(out, string(r))
	}
	return out
}
func DiffBodies(from, to string) Diff {
	budget := 5000000
	line, ok := myers(strings.SplitAfter(from, "\n"), strings.SplitAfter(to, "\n"), &budget)
	result := Diff{OffsetUnit: "unicode_code_point", Hunks: []DiffHunk{}}
	edits := []edit{}
	if !ok {
		edits = append(edits, edit{"delete", from}, edit{"insert", to})
		result.Coarse = true
	} else {
		for i := 0; i < len(line); {
			if line[i].kind == "equal" {
				edits = append(edits, line[i])
				i++
				continue
			}
			var a, b strings.Builder
			for i < len(line) && line[i].kind != "equal" {
				if line[i].kind == "delete" {
					a.WriteString(line[i].text)
				} else {
					b.WriteString(line[i].text)
				}
				i++
			}
			refined, good := myers(tokens(a.String()), tokens(b.String()), &budget)
			if good {
				edits = append(edits, refined...)
			} else {
				edits = append(edits, edit{"delete", a.String()}, edit{"insert", b.String()})
				result.Coarse = true
			}
		}
	}
	// Merge before constructing strings, avoiding quadratic concatenation for long runs.
	var old, newOffset int64
	for i := 0; i < len(edits); {
		kind := edits[i].kind
		var text strings.Builder
		for i < len(edits) && edits[i].kind == kind {
			text.WriteString(edits[i].text)
			i++
		}
		s := text.String()
		if s == "" {
			continue
		}
		n := int64(len([]rune(s)))
		h := DiffHunk{Kind: kind, OldStart: old, NewStart: newOffset, Text: s}
		if kind != "insert" {
			h.OldLength = n
			old += n
		}
		if kind != "delete" {
			h.NewLength = n
			newOffset += n
		}
		result.Hunks = append(result.Hunks, h)
	}
	return result
}
func WritingDelta(before, after string) (added, removed int64) {
	for _, h := range DiffBodies(before, after).Hunks {
		if h.Kind == "insert" {
			added += Words(h.Text)
		} else if h.Kind == "delete" {
			removed += Words(h.Text)
		}
	}
	return
}
