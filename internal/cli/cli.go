// Package cli parses the command line without any third-party dependency.
// Positional arguments may appear before or after options, "--" ends option
// parsing, and a misspelt option is reported with a suggestion.
package cli

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Kind is the value type of an option.
type Kind int

const (
	// Bool options take no value.
	Bool Kind = iota
	// Str options take one string value.
	Str
	// Int options take one integer value.
	Int
)

// Def describes one option.
type Def struct {
	Name  string // long name without the leading dashes
	Short string // one letter, no dash
	Kind  Kind
	Key   string // i18n key used by the help printer
}

// Parsed is the result of a successful parse.
type Parsed struct {
	Pos   []string
	strs  map[string]string
	lists map[string][]string
	bools map[string]bool
	ints  map[string]int
	given map[string]bool
}

// ErrBadValue marks a malformed option value.
var ErrBadValue = errors.New("bad option value")

// UsageError is a user-facing argument error. Its message is printed as-is, with
// no "bad option value:" prefix, but it still maps to exit code 2.
type UsageError struct{ Msg string }

func (e *UsageError) Error() string { return e.Msg }

// Is makes errors.Is(err, ErrBadValue) true for every UsageError.
func (e *UsageError) Is(target error) bool { return target == ErrBadValue }

// Badf builds a UsageError with a formatted message.
func Badf(format string, a ...any) error {
	return &UsageError{Msg: fmt.Sprintf(format, a...)}
}

// UnknownFlagError names the offending flag and, when close enough, the flag
// the user probably meant.
type UnknownFlagError struct {
	Flag    string
	Suggest string
}

func (e *UnknownFlagError) Error() string { return "unknown option: " + e.Flag }

// Parse walks args against defs.
func Parse(args []string, defs []Def) (*Parsed, error) {
	byLong := make(map[string]*Def, len(defs))
	byShort := make(map[string]*Def, len(defs))
	for i := range defs {
		d := &defs[i]
		byLong[d.Name] = d
		if d.Short != "" {
			byShort[d.Short] = d
		}
	}
	p := &Parsed{
		strs:  make(map[string]string, len(defs)),
		lists: make(map[string][]string, len(defs)),
		bools: make(map[string]bool, len(defs)),
		ints:  make(map[string]int, len(defs)),
		given: make(map[string]bool, len(defs)),
	}

	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			p.Pos = append(p.Pos, args[i+1:]...)
			return p, nil
		}
		if strings.HasPrefix(a, "--") {
			name := a[2:]
			val := ""
			hasVal := false
			if k := strings.IndexByte(name, '='); k >= 0 {
				val, name, hasVal = name[k+1:], name[:k], true
			}
			d, ok := byLong[name]
			if !ok {
				return nil, &UnknownFlagError{Flag: "--" + name, Suggest: suggestLong("--"+name, byLong)}
			}
			if d.Kind == Bool {
				p.bools[d.Name] = true
				p.given[d.Name] = true
				continue
			}
			if !hasVal {
				if i+1 >= len(args) {
					return nil, Badf("--%s needs a value", name)
				}
				i++
				val = args[i]
			}
			if err := p.assign(d, val); err != nil {
				return nil, err
			}
			continue
		}
		if len(a) > 1 && a[0] == '-' && !isNumericDash(a) {
			short := a[1:]
			d, ok := byShort[short[:1]]
			if !ok {
				return nil, &UnknownFlagError{Flag: "-" + short[:1], Suggest: suggestShort("-"+short[:1], byShort)}
			}
			if d.Kind == Bool {
				// Bundled booleans such as -qy.
				allBool := true
				for _, r := range short {
					dd, ok := byShort[string(r)]
					if !ok || dd.Kind != Bool {
						allBool = false
						break
					}
				}
				if allBool {
					for _, r := range short {
						dd := byShort[string(r)]
						p.bools[dd.Name] = true
						p.given[dd.Name] = true
					}
					continue
				}
				p.bools[d.Name] = true
				p.given[d.Name] = true
				continue
			}
			val := strings.TrimPrefix(short[1:], "=")
			if val == "" {
				if i+1 >= len(args) {
					return nil, Badf("-%s needs a value", short[:1])
				}
				i++
				val = args[i]
			}
			if err := p.assign(d, val); err != nil {
				return nil, err
			}
			continue
		}
		p.Pos = append(p.Pos, a)
	}
	return p, nil
}

func (p *Parsed) assign(d *Def, val string) error {
	switch d.Kind {
	case Int:
		n, err := strconv.Atoi(strings.TrimSpace(val))
		if err != nil {
			return Badf("--%s expects a number, got %q", d.Name, val)
		}
		p.ints[d.Name] = n
	default:
		p.strs[d.Name] = val
		p.lists[d.Name] = append(p.lists[d.Name], val)
	}
	p.given[d.Name] = true
	return nil
}

// isNumericDash keeps "-1" usable as a positional value rather than an option.
func isNumericDash(s string) bool {
	if len(s) < 2 || s[0] != '-' {
		return false
	}
	_, err := strconv.ParseFloat(s, 64)
	return err == nil
}

// Has reports whether the option appeared on the command line.
func (p *Parsed) Has(name string) bool { return p.given[name] }

// Bool reads a boolean option.
func (p *Parsed) Bool(name string) bool { return p.bools[name] }

// Str reads a string option.
func (p *Parsed) Str(name string) string { return p.strs[name] }

// Strs reads every value given to a repeatable string option, in order.
func (p *Parsed) Strs(name string) []string { return p.lists[name] }

// Int reads an integer option.
func (p *Parsed) Int(name string) int { return p.ints[name] }

// StrOr returns the value or def when absent.
func (p *Parsed) StrOr(name, def string) string {
	if v, ok := p.strs[name]; ok && v != "" {
		return v
	}
	return def
}

// IntOr returns the value or def when absent.
func (p *Parsed) IntOr(name string, def int) int {
	if v, ok := p.ints[name]; ok {
		return v
	}
	return def
}

// Closest returns the option closest to word, or "" when nothing is close
// enough. It lets a caller offer "did you mean ..." for subcommands too.
func Closest(word string, options []string) string {
	best, bestDist := "", 3
	for _, o := range options {
		d := levenshtein(word, o)
		if d < bestDist {
			best, bestDist = o, d
		}
	}
	return best
}

func suggestLong(bad string, byLong map[string]*Def) string {
	best, bestDist := "", 3
	for name := range byLong {
		d := levenshtein(bad, "--"+name)
		if d < bestDist {
			best, bestDist = "--"+name, d
		}
	}
	return best
}

func suggestShort(bad string, byShort map[string]*Def) string {
	best, bestDist := "", 3
	for name := range byShort {
		d := levenshtein(bad, "-"+name)
		if d < bestDist {
			best, bestDist = "-"+name, d
		}
	}
	return best
}

// levenshtein is the classic edit distance; inputs are short flag names.
func levenshtein(a, b string) int {
	ra, rb := []rune(a), []rune(b)
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
			cur[j] = min3(cur[j-1]+1, prev[j]+1, prev[j-1]+cost)
		}
		prev, cur = cur, prev
	}
	return prev[len(rb)]
}

func min3(a, b, c int) int {
	if b < a {
		a = b
	}
	if c < a {
		a = c
	}
	return a
}
