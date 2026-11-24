package provider

import "testing"

func TestNormalizeRepo(t *testing.T) {
	cases := map[string]string{
		"owner/repo":                                    "owner/repo",
		"https://huggingface.co/owner/repo":             "owner/repo",
		"https://hf-mirror.com/owner/repo/tree/main":    "owner/repo",
		"https://www.modelscope.cn/models/owner/repo":   "owner/repo",
		"https://modelscope.cn/models/owner/repo/files": "owner/repo",
		"http://hf.co/owner/repo":                       "owner/repo",
		"  \"owner/repo\"  ":                            "owner/repo",
		"「owner/repo」":                                  "owner/repo",
		"owner/repo?tab=files":                          "owner/repo",
		"owner/repo/blob/main/file.gguf":                "owner/repo",
		"owner/repo.git":                                "owner/repo",
	}
	for in, want := range cases {
		got, err := NormalizeRepo(in)
		if err != nil {
			t.Errorf("NormalizeRepo(%q) error: %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("NormalizeRepo(%q) = %q, want %q", in, got, want)
		}
	}

	if _, err := NormalizeRepo("https://huggingface.co/"); err == nil {
		t.Error("expected an error for a bare host")
	}
}

func TestSplitRepo(t *testing.T) {
	owner, name := SplitRepo("a/b")
	if owner != "a" || name != "b" {
		t.Fatalf("SplitRepo = %q,%q", owner, name)
	}
	owner, name = SplitRepo("noslash")
	if owner != "" || name != "noslash" {
		t.Fatalf("SplitRepo(noslash) = %q,%q", owner, name)
	}
}

func TestSortAndFilter(t *testing.T) {
	files := []File{
		{Path: "b-Q8_0.gguf", Size: 30},
		{Path: "a-Q4_K_M.gguf", Size: 10},
		{Path: "c-Q6_K.gguf", Size: 20},
	}
	bySize := SortFiles(files, SortSize)
	if bySize[0].Size != 10 || bySize[2].Size != 30 {
		t.Fatalf("size sort wrong: %v", bySize)
	}
	byName := SortFiles(files, SortName)
	if byName[0].Path != "a-Q4_K_M.gguf" {
		t.Fatalf("name sort wrong: %v", byName)
	}
	// the input slice must not be reordered in place
	if files[0].Path != "b-Q8_0.gguf" {
		t.Fatalf("SortFiles mutated its input: %v", files)
	}
	got := FilterFiles(files, "Q6")
	if len(got) != 1 || got[0].Path != "c-Q6_K.gguf" {
		t.Fatalf("filter wrong: %v", got)
	}
	if len(FilterFiles(files, "")) != 3 {
		t.Fatal("empty filter must keep every file")
	}
}
