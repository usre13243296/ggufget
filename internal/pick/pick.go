// Package pick turns what a user typed (-f 2, -f q4_k_m, -f "Qwen 7B") into
// indexes into the repo's file list. The matching order is fixed by the manual:
// index expression first, then exact name, then case-insensitive, then a unique
// substring. It never guesses between candidates.
package pick

import (
	"path"
	"sort"
	"strconv"
	"strings"
)

// How records which rule matched, so the CLI can explain the choice.
type How string

const (
	HowIndex    How = "index"
	HowExact    How = "exact"
	HowCaseFold How = "casefold"
	HowContains How = "contains"
)

// Hit is one resolved selection.
type Hit struct {
	Index int
	Query string
	How   How
}

// Miss is one query that did not resolve.
type Miss struct {
	Query      string
	Candidates []int
}

// Clean normalises raw user input pasted from a chat window: stray quotes, a
// repeated flag name, Windows separators.
func Clean(s string) string {
	s = strings.TrimSpace(s)
	for {
		trimmed := strings.Trim(strings.TrimSpace(s), `"'“”‘’「」『』`)
		if trimmed == s {
			break
		}
		s = trimmed
	}
	lower := strings.ToLower(s)
	for _, p := range []string{
		"--file=", "--file ", "-f ",
		"file:", "file：", "文件名:", "文件名：", "文件名 ",
		"序号:", "序号：", "index:",
	} {
		if strings.HasPrefix(lower, p) {
			s = strings.TrimSpace(s[len(p):])
			lower = strings.ToLower(s)
		}
	}
	s = strings.ReplaceAll(s, `\`, "/")
	return strings.TrimSpace(s)
}

// IsIndexExpr reports whether q was meant as an index expression: digits,
// commas, dashes and spaces only. Callers use it to keep an out-of-range
// number an index error instead of silently falling back to a keyword or a
// position in another list.
func IsIndexExpr(q string) bool {
	q = strings.TrimSpace(q)
	if q == "" {
		return false
	}
	for _, r := range q {
		if (r < '0' || r > '9') && r != ',' && r != '-' && r != ' ' {
			return false
		}
	}
	return true
}

// Indices parses "2", "1,3" or "3-1" into one-based indexes within [1,n].
// It reports false when the text is not an index expression at all, including
// when it is out of range, so the caller falls through to keyword matching.
func Indices(q string, n int) ([]int, bool) {
	if q == "" || n <= 0 {
		return nil, false
	}
	for _, r := range q {
		if (r < '0' || r > '9') && r != ',' && r != '-' && r != ' ' {
			return nil, false
		}
	}
	seen := make(map[int]bool)
	out := make([]int, 0, 4)
	add := func(k int) bool {
		if k < 1 || k > n {
			return false
		}
		if !seen[k] {
			seen[k] = true
			out = append(out, k)
		}
		return true
	}
	for _, part := range strings.Split(q, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			return nil, false
		}
		if i := strings.IndexByte(part, '-'); i >= 0 {
			a, err1 := strconv.Atoi(strings.TrimSpace(part[:i]))
			b, err2 := strconv.Atoi(strings.TrimSpace(part[i+1:]))
			if err1 != nil || err2 != nil {
				return nil, false
			}
			if a > b {
				a, b = b, a
			}
			for k := a; k <= b; k++ {
				if !add(k) {
					return nil, false
				}
			}
			continue
		}
		k, err := strconv.Atoi(part)
		if err != nil || !add(k) {
			return nil, false
		}
	}
	if len(out) == 0 {
		return nil, false
	}
	return out, true
}

// Resolve maps queries onto file indexes, deduplicating and keeping list order
// in the result so downloads run top to bottom.
func Resolve(files []string, queries []string) ([]Hit, []Miss) {
	chosen := make(map[int]Hit)
	var misses []Miss
	for _, raw := range queries {
		q := Clean(raw)
		if q == "" {
			continue
		}
		if idxs, ok := Indices(q, len(files)); ok {
			for _, k := range idxs {
				if _, dup := chosen[k-1]; !dup {
					chosen[k-1] = Hit{Index: k - 1, Query: raw, How: HowIndex}
				}
			}
			continue
		}
		i, how, candidates := matchOne(files, q)
		if i >= 0 {
			if _, dup := chosen[i]; !dup {
				chosen[i] = Hit{Index: i, Query: raw, How: how}
			}
			continue
		}
		misses = append(misses, Miss{Query: raw, Candidates: candidates})
	}
	hits := make([]Hit, 0, len(chosen))
	for _, h := range chosen {
		hits = append(hits, h)
	}
	sort.Slice(hits, func(a, b int) bool { return hits[a].Index < hits[b].Index })
	return hits, misses
}

func matchOne(files []string, q string) (int, How, []int) {
	base := path.Base(q)
	var exact []int
	for i, f := range files {
		if f == q || path.Base(f) == base {
			exact = append(exact, i)
		}
	}
	if len(exact) == 1 {
		return exact[0], HowExact, nil
	}
	if len(exact) > 1 {
		return -1, HowExact, exact
	}

	lq := strings.ToLower(q)
	lbase := strings.ToLower(base)
	var folded []int
	for i, f := range files {
		if strings.ToLower(f) == lq || strings.ToLower(path.Base(f)) == lbase {
			folded = append(folded, i)
		}
	}
	if len(folded) == 1 {
		return folded[0], HowCaseFold, nil
	}
	if len(folded) > 1 {
		return -1, HowCaseFold, folded
	}

	var contains []int
	for i, f := range files {
		if strings.Contains(strings.ToLower(f), lq) {
			contains = append(contains, i)
		}
	}
	if len(contains) == 1 {
		return contains[0], HowContains, nil
	}
	if len(contains) > 1 {
		return -1, HowContains, contains
	}
	return -1, "", nil
}
