package lmstudio

import "testing"

func TestParseLayout(t *testing.T) {
	cases := []struct {
		in   string
		want Layout
		ok   bool
	}{
		{"", LayoutLMStudio, true},
		{"lmstudio", LayoutLMStudio, true},
		{"LMStudio", LayoutLMStudio, true},
		{"lms", LayoutLMStudio, true},
		{"hub", LayoutLMStudio, true},
		{"repo", LayoutRepo, true},
		{"hf", LayoutRepo, true},
		{"huggingface", LayoutRepo, true},
		{"flat", LayoutFlat, true},
		{"none", LayoutFlat, true},
		{"  flat  ", LayoutFlat, true},
		{"weird", LayoutLMStudio, false},
	}
	for _, c := range cases {
		got, ok := ParseLayout(c.in)
		if ok != c.ok || got != c.want {
			t.Errorf("ParseLayout(%q) = %q,%v; want %q,%v", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestStem(t *testing.T) {
	cases := map[string]string{
		"DeepSeek-Coder-V2-Lite-Instruct-IQ2_S.gguf": "DeepSeek-Coder-V2-Lite-Instruct-IQ2_S",
		"sub/dir/model.Q8_0.gguf":                    "model.Q8_0",
		"model":                                      "model",
		"a/b/plain.txt":                              "plain",
	}
	for in, want := range cases {
		if got := Stem(in); got != want {
			t.Errorf("Stem(%q) = %q; want %q", in, got, want)
		}
	}
}

func TestRel(t *testing.T) {
	const file = "DeepSeek-Coder-V2-Lite-Instruct-IQ2_S.gguf"

	if got, want := Rel(LayoutLMStudio, "owner", "repo", file),
		"DeepSeek-Coder-V2-Lite-Instruct-IQ2_S/lmstudio-ai/"+file; got != want {
		t.Errorf("lmstudio layout = %q; want %q", got, want)
	}
	if got, want := Rel(LayoutRepo, "owner", "repo", file), "owner/repo/"+file; got != want {
		t.Errorf("repo layout = %q; want %q", got, want)
	}
	if got, want := Rel(LayoutFlat, "owner", "repo", file), file; got != want {
		t.Errorf("flat layout = %q; want %q", got, want)
	}
	if got, want := Rel(LayoutRepo, "", "repo", file), "repo/"+file; got != want {
		t.Errorf("repo layout without owner = %q; want %q", got, want)
	}
	// A file stored in a repo subdirectory is flattened: LM Studio only walks a
	// fixed depth, so anything deeper would never be indexed.
	if got, want := Rel(LayoutLMStudio, "", "", "sub/dir/m.gguf"), "m/lmstudio-ai/m.gguf"; got != want {
		t.Errorf("nested file = %q; want %q", got, want)
	}
	if got, want := Rel(LayoutRepo, "", "", "sub/dir/m.gguf"), "m.gguf"; got != want {
		t.Errorf("nested file, repo layout = %q; want %q", got, want)
	}
}
