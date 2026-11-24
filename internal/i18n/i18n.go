// Package i18n holds every user-visible string in ggufget.
//
// Chinese and English live together in messages.go so neither can silently
// drift. French, Spanish and Russian live in their own files and are checked by
// TestLanguagesComplete: a key that exists in one language but not another is a
// test failure, never an empty line at runtime.
package i18n

import (
	"fmt"
	"os"
	"sort"
	"strings"
)

// Lang is a supported UI language.
type Lang string

const (
	Zh Lang = "zh"
	En Lang = "en"
	Fr Lang = "fr"
	Es Lang = "es"
	Ru Lang = "ru"
)

// order is the order languages appear in menus.
var order = []Lang{Zh, En, Fr, Es, Ru}

type table map[string]string

var tables = map[Lang]table{Zh: {}, En: {}, Fr: {}, Es: {}, Ru: {}}

var current = Zh

func init() { Register() }

// def registers one message in Chinese and English.
func def(key, zh, en string) {
	tables[Zh][key] = zh
	tables[En][key] = en
}

// register fills one language table; extra languages call it from their own file.
func register(l Lang, m table) {
	t := tables[l]
	if t == nil {
		t = table{}
		tables[l] = t
	}
	for k, v := range m {
		t[k] = v
	}
}

var aliases = map[string]Lang{
	"zh": Zh, "cn": Zh, "zh-cn": Zh, "zh_cn": Zh, "chinese": Zh, "中文": Zh,
	"en": En, "en-us": En, "en_us": En, "en-gb": En, "english": En,
	"fr": Fr, "fr-fr": Fr, "fr_fr": Fr, "french": Fr, "francais": Fr, "français": Fr,
	"es": Es, "es-es": Es, "es_es": Es, "spanish": Es, "espanol": Es, "español": Es,
	"ru": Ru, "ru-ru": Ru, "ru_ru": Ru, "russian": Ru, "русский": Ru,
}

// Parse maps a user or environment tag onto a supported language.
func Parse(s string) (Lang, bool) {
	k := strings.ToLower(strings.TrimSpace(s))
	if k == "" {
		return "", false
	}
	if l, ok := aliases[k]; ok {
		return l, true
	}
	for _, l := range order {
		if strings.HasPrefix(k, string(l)) {
			return l, true
		}
	}
	return "", false
}

// Set switches the active language. It reports false for an unknown tag.
func Set(s string) bool {
	l, ok := Parse(s)
	if !ok {
		return false
	}
	current = l
	return true
}

// FromEnv guesses the language from GGUFGET_LANG / LC_ALL / LC_MESSAGES / LANG.
func FromEnv() {
	for _, key := range []string{"GGUFGET_LANG", "LC_ALL", "LC_MESSAGES", "LANG"} {
		v := os.Getenv(key)
		if v == "" {
			continue
		}
		if l, ok := Parse(v); ok {
			current = l
			return
		}
	}
}

// Cur returns the active language.
func Cur() Lang { return current }

// Langs returns every supported language in menu order.
func Langs() []Lang { return append([]Lang{}, order...) }

// Name is the native name, shown in language pickers.
func Name(l Lang) string {
	switch l {
	case En:
		return "English"
	case Fr:
		return "Français"
	case Es:
		return "Español"
	case Ru:
		return "Русский"
	}
	return "中文"
}

// T looks up a message. Unknown keys return the key itself, which makes a typo
// visible in tests instead of printing an empty line.
func T(key string, a ...any) string {
	s, ok := tables[current][key]
	if !ok || s == "" {
		s = tables[En][key]
	}
	if s == "" {
		s = tables[Zh][key]
	}
	if s == "" {
		return key
	}
	if len(a) == 0 {
		return s
	}
	return fmt.Sprintf(s, a...)
}

// Missing lists the keys that exist in Chinese but not in l, sorted. Tests use
// it to fail the build instead of shipping a half-translated menu.
func Missing(l Lang) []string {
	var out []string
	for k := range tables[Zh] {
		if tables[l][k] == "" {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

// Keys returns every key, sorted.
func Keys() []string {
	out := make([]string, 0, len(tables[Zh]))
	for k := range tables[Zh] {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
