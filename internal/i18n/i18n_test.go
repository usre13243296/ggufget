package i18n

import (
	"os"
	"strings"
	"testing"
)

// langsForTest narrows the checks to one language when GGUFGET_TEST_LANG is
// set, so a translator can verify their own file before the others land.
func langsForTest() []Lang {
	if v := os.Getenv("GGUFGET_TEST_LANG"); v != "" {
		if l, ok := Parse(v); ok {
			return []Lang{l}
		}
	}
	return Langs()
}

// TestLanguagesComplete fails when a language is missing a key, so a
// half-translated menu can never ship.
func TestLanguagesComplete(t *testing.T) {
	for _, l := range langsForTest() {
		if missing := Missing(l); len(missing) > 0 {
			t.Errorf("%s is missing %d string(s): %s", l, len(missing), strings.Join(missing, ", "))
		}
	}
}

// TestFormatsMatchChinese catches a translation that drops or invents a format
// verb, which would print "%!s(MISSING)" at runtime.
func TestFormatsMatchChinese(t *testing.T) {
	for _, l := range langsForTest() {
		for _, k := range Keys() {
			want, got := verbs(tables[Zh][k]), verbs(tables[l][k])
			if got != want {
				t.Errorf("%s %q: %d format verb(s), want %d", l, k, got, want)
			}
		}
	}
}

func verbs(s string) int {
	n := 0
	for i := 0; i < len(s); i++ {
		if s[i] != '%' || i+1 >= len(s) {
			continue
		}
		if s[i+1] == '%' {
			i++
			continue
		}
		n++
	}
	return n
}

func TestParse(t *testing.T) {
	cases := map[string]Lang{
		"zh":          Zh,
		"zh-CN":       Zh,
		"中文":          Zh,
		"en":          En,
		"en_US.UTF-8": En,
		"fr_FR.UTF-8": Fr,
		"Français":    Fr,
		"es-ES":       Es,
		"Español":     Es,
		"ru_RU":       Ru,
		"Русский":     Ru,
	}
	for in, want := range cases {
		got, ok := Parse(in)
		if !ok || got != want {
			t.Errorf("Parse(%q) = %q, %v; want %q", in, got, ok, want)
		}
	}
	for _, in := range []string{"", "  ", "de", "xx-YY"} {
		if _, ok := Parse(in); ok {
			t.Errorf("Parse(%q) unexpectedly succeeded", in)
		}
	}
}

func TestSetAndNames(t *testing.T) {
	defer Set(string(Zh))
	for _, l := range Langs() {
		if !Set(string(l)) {
			t.Fatalf("Set(%q) failed", l)
		}
		if Cur() != l {
			t.Fatalf("Cur() = %q, want %q", Cur(), l)
		}
		if Name(l) == "" {
			t.Errorf("Name(%q) is empty", l)
		}
	}
	if Set("nope") {
		t.Error("Set accepted an unknown language")
	}
}

func TestFallbackToEnglish(t *testing.T) {
	defer Set(string(Zh))
	Set("fr")
	if got := T("ui.bye"); got == "ui.bye" || got == "" {
		t.Errorf("T(ui.bye) in French = %q", got)
	}
	if got := T("no.such.key"); got != "no.such.key" {
		t.Errorf("unknown key = %q, want the key itself", got)
	}
}
