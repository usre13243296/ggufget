package app

import (
	"io"
	"strings"
	"testing"

	"ggufget/internal/provider"
	"ggufget/internal/termui"
)

func testApp() *App {
	return &App{ErrOut: io.Discard, St: termui.Style{}}
}

func testFile(path string, size int64) provider.File {
	return provider.File{Path: path, Size: size}
}

func selectedPaths(files []provider.File) []string {
	out := make([]string, len(files))
	for i, f := range files {
		out[i] = f.Path
	}
	return out
}

func TestResolveSelectionUsesTheView(t *testing.T) {
	files := []provider.File{
		testFile("a.gguf", 30),
		testFile("b.gguf", 10),
		testFile("c.gguf", 20),
	}
	a := testApp()

	cases := []struct {
		name string
		q    string
		view viewOpts
		want string
	}{
		{"default by name", "2", viewOpts{sort: provider.SortName}, "b.gguf"},
		{"sorted by size", "1", viewOpts{sort: provider.SortSize}, "b.gguf"},
		{"filtered", "1", viewOpts{filter: "c", sort: provider.SortName}, "c.gguf"},
		{"limited", "2", viewOpts{limit: 2, sort: provider.SortName}, "b.gguf"},
	}
	for _, c := range cases {
		got, err := a.resolveSelection(files, []string{c.q}, c.view)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if paths := selectedPaths(got); len(paths) != 1 || paths[0] != c.want {
			t.Fatalf("%s: selected %v, want %s", c.name, paths, c.want)
		}
	}
}

func TestResolveSelectionNonGGUFFilename(t *testing.T) {
	files := []provider.File{
		testFile("a.gguf", 1),
		testFile("README.md", 2),
	}
	a := testApp()

	// A full filename matches the whole repository even though the default
	// view only contains gguf files.
	got, err := a.resolveSelection(files, []string{"README.md"}, viewOpts{sort: provider.SortName})
	if err != nil {
		t.Fatalf("README.md: %v", err)
	}
	if paths := selectedPaths(got); len(paths) != 1 || paths[0] != "README.md" {
		t.Fatalf("README.md selected %v", paths)
	}

	// An out-of-range index must not fall back to the second file of the full
	// list (which used to make "-f 2" download README.md here).
	got, err = a.resolveSelection(files, []string{"2"}, viewOpts{sort: provider.SortName})
	if err == nil {
		t.Fatal("expected an out-of-range error")
	}
	if len(got) != 0 {
		t.Fatalf("out-of-range index selected %v", selectedPaths(got))
	}
	if !strings.Contains(err.Error(), "2") {
		t.Fatalf("error should name the index: %v", err)
	}

	// With --any the index view includes the non-gguf file. Path order puts
	// README.md first (uppercase sorts before lowercase).
	got, err = a.resolveSelection(files, []string{"1"}, viewOpts{any: true, sort: provider.SortName})
	if err != nil {
		t.Fatalf("--any index 1: %v", err)
	}
	if paths := selectedPaths(got); len(paths) != 1 || paths[0] != "README.md" {
		t.Fatalf("--any index 1 selected %v", paths)
	}
}

func TestResolveSelectionAmbiguousKeyword(t *testing.T) {
	files := []provider.File{
		testFile("a-Q4_K_M.gguf", 1),
		testFile("b-Q4_K_M.gguf", 2),
	}
	a := testApp()
	got, err := a.resolveSelection(files, []string{"q4_k_m"}, viewOpts{sort: provider.SortName})
	if err == nil {
		t.Fatal("expected an ambiguity error")
	}
	if len(got) != 0 {
		t.Fatalf("ambiguous keyword selected %v", selectedPaths(got))
	}
	if !strings.Contains(err.Error(), "Q4_K_M") {
		t.Fatalf("error should list candidates: %v", err)
	}
}

func TestGetHelpListsTheIndexViewOptions(t *testing.T) {
	names := map[string]bool{}
	for _, d := range commandDefs("get") {
		names[d.Name] = true
	}
	for _, want := range []string{"any", "filter", "sort", "limit", "file", "all"} {
		if !names[want] {
			t.Errorf("get help is missing --%s", want)
		}
	}
}

func TestPickLogPath(t *testing.T) {
	cases := []struct {
		flag, env, cfg, want string
	}{
		{"flag.log", "env.log", "cfg.log", "flag.log"},
		{"", "env.log", "cfg.log", "env.log"},
		{"", "", "cfg.log", "cfg.log"},
		{"", "", "", ""},
		{"  ", "  ", "  ", ""},
	}
	for _, c := range cases {
		if got := pickLogPath(c.flag, c.env, c.cfg); got != c.want {
			t.Errorf("pickLogPath(%q,%q,%q) = %q, want %q", c.flag, c.env, c.cfg, got, c.want)
		}
	}
}
