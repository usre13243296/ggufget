package pick

import "testing"

func TestIndices(t *testing.T) {
	cases := []struct {
		q     string
		n     int
		want  []int
		valid bool
	}{
		{"2", 4, []int{2}, true},
		{"1,3", 4, []int{1, 3}, true},
		{"1-3", 5, []int{1, 2, 3}, true},
		{"3-1", 5, []int{1, 2, 3}, true},
		{" 2 , 4 ", 4, []int{2, 4}, true},
		{"2,2", 4, []int{2}, true},
		{"0", 4, nil, false},
		{"5", 4, nil, false},
		{"1,x", 4, nil, false},
		{"q4_k_m", 4, nil, false},
	}
	for _, c := range cases {
		got, ok := Indices(c.q, c.n)
		if ok != c.valid {
			t.Fatalf("Indices(%q,%d) valid = %v, want %v", c.q, c.n, ok, c.valid)
		}
		if !ok {
			continue
		}
		if len(got) != len(c.want) {
			t.Fatalf("Indices(%q,%d) = %v, want %v", c.q, c.n, got, c.want)
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Fatalf("Indices(%q,%d) = %v, want %v", c.q, c.n, got, c.want)
			}
		}
	}
}

func TestClean(t *testing.T) {
	cases := map[string]string{
		"--file Q4_K_M":       "Q4_K_M",
		"-f q4":               "q4",
		"文件名：q4_k_m":          "q4_k_m",
		"\x22Q4_K_M.gguf\x22": "Q4_K_M.gguf",
		"dir\\file.gguf":      "dir/file.gguf",
	}
	for in, want := range cases {
		if got := Clean(in); got != want {
			t.Errorf("Clean(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestIsIndexExpr(t *testing.T) {
	for _, q := range []string{"2", "1,3", "1-3", " 2 , 4 ", "3-1"} {
		if !IsIndexExpr(q) {
			t.Errorf("IsIndexExpr(%q) = false, want true", q)
		}
	}
	for _, q := range []string{"", "  ", "q4", "Qwen2.5-7B", "1.5", "a1", "-f 2"} {
		if IsIndexExpr(q) {
			t.Errorf("IsIndexExpr(%q) = true, want false", q)
		}
	}
}

func TestResolve(t *testing.T) {
	files := []string{
		"model-Q4_K_M.gguf",
		"model-Q6_K.gguf",
		"model-Q8_0.gguf",
	}
	hits, misses := Resolve(files, []string{"2"})
	if len(misses) != 0 || len(hits) != 1 || hits[0].Index != 1 {
		t.Fatalf("index selection failed: hits=%v misses=%v", hits, misses)
	}
	if hits[0].How != HowIndex {
		t.Fatalf("expected HowIndex, got %v", hits[0].How)
	}

	hits, misses = Resolve(files, []string{"q6"})
	if len(misses) != 0 || len(hits) != 1 || hits[0].Index != 1 {
		t.Fatalf("fragment selection failed: hits=%v misses=%v", hits, misses)
	}

	hits, misses = Resolve(files, []string{"q4", "q8"})
	if len(misses) != 0 || len(hits) != 2 {
		t.Fatalf("multi selection failed: hits=%v misses=%v", hits, misses)
	}
	if hits[0].Index > hits[1].Index {
		t.Fatalf("hits must be sorted by index: %v", hits)
	}

	_, misses = Resolve(files, []string{"zzz"})
	if len(misses) != 1 {
		t.Fatalf("expected one miss, got %v", misses)
	}

	// an ambiguous fragment reports candidates instead of guessing
	_, misses = Resolve([]string{"a-Q4_K_M.gguf", "b-Q4_K_M.gguf"}, []string{"q4_k_m"})
	if len(misses) != 1 || len(misses[0].Candidates) != 2 {
		t.Fatalf("expected ambiguity with 2 candidates, got %v", misses)
	}
}

func TestResolveDedup(t *testing.T) {
	files := []string{"a.gguf", "b.gguf"}
	hits, misses := Resolve(files, []string{"1", "a.gguf"})
	if len(misses) != 0 || len(hits) != 1 {
		t.Fatalf("duplicate queries must collapse: hits=%v misses=%v", hits, misses)
	}
}
