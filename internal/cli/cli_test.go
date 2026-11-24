package cli

import (
	"errors"
	"testing"
)

func defs() []Def {
	return []Def{
		{Name: "help", Short: "h", Kind: Bool, Key: "flag.help"},
		{Name: "quiet", Short: "q", Kind: Bool, Key: "flag.quiet"},
		{Name: "yes", Short: "y", Kind: Bool, Key: "flag.yes"},
		{Name: "file", Short: "f", Kind: Str, Key: "flag.file"},
		{Name: "jobs", Short: "j", Kind: Int, Key: "flag.jobs"},
		{Name: "no-color", Kind: Bool, Key: "flag.nocolor"},
	}
}

func TestParseForms(t *testing.T) {
	p, err := Parse([]string{"get", "owner/repo", "--file=2", "-f", "q4", "--jobs", "3"}, defs())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(p.Pos) != 2 || p.Pos[0] != "get" || p.Pos[1] != "owner/repo" {
		t.Fatalf("pos = %v", p.Pos)
	}
	files := p.Strs("file")
	if len(files) != 2 || files[0] != "2" || files[1] != "q4" {
		t.Fatalf("file = %v", files)
	}
	if p.IntOr("jobs", 1) != 3 {
		t.Fatalf("jobs = %d", p.IntOr("jobs", 1))
	}
}

func TestParsePositionalsAfterOptions(t *testing.T) {
	p, err := Parse([]string{"--quiet", "list", "-f", "x", "owner/repo"}, defs())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !p.Bool("quiet") {
		t.Fatal("quiet not set")
	}
	if len(p.Pos) != 2 || p.Pos[0] != "list" || p.Pos[1] != "owner/repo" {
		t.Fatalf("pos = %v", p.Pos)
	}
}

func TestParseCombinedShorts(t *testing.T) {
	p, err := Parse([]string{"-qy", "list", "r/r"}, defs())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !p.Bool("quiet") || !p.Bool("yes") {
		t.Fatalf("-qy not expanded: %v", p)
	}
}

func TestParseTerminator(t *testing.T) {
	p, err := Parse([]string{"get", "--", "-weird-repo-name"}, defs())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(p.Pos) != 2 || p.Pos[1] != "-weird-repo-name" {
		t.Fatalf("pos = %v", p.Pos)
	}
}

func TestParseUnknownFlagSuggests(t *testing.T) {
	_, err := Parse([]string{"list", "--quite"}, defs())
	if err == nil {
		t.Fatal("expected an error")
	}
	var uf *UnknownFlagError
	if !errors.As(err, &uf) {
		t.Fatalf("wrong error type: %T", err)
	}
	if uf.Suggest != "--quiet" {
		t.Fatalf("suggest = %q", uf.Suggest)
	}
}

func TestParseBadValue(t *testing.T) {
	_, err := Parse([]string{"--jobs", "many"}, defs())
	if !errors.Is(err, ErrBadValue) {
		t.Fatalf("expected ErrBadValue, got %v", err)
	}
	if err.Error() == "bad option value" {
		t.Fatal("UsageError should carry its own message")
	}
}

func TestParseMissingValue(t *testing.T) {
	_, err := Parse([]string{"--file"}, defs())
	if !errors.Is(err, ErrBadValue) {
		t.Fatalf("expected ErrBadValue, got %v", err)
	}
}
