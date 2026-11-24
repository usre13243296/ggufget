package downloader

import "testing"

func TestSplitSegments(t *testing.T) {
	segs := splitSegments(1000, 4)
	if len(segs) != 4 {
		t.Fatalf("got %d segments", len(segs))
	}
	var off int64
	for i, s := range segs {
		if s.Start != off {
			t.Fatalf("segment %d starts at %d, want %d", i, s.Start, off)
		}
		if s.Done != 0 {
			t.Fatalf("segment %d must start empty", i)
		}
		off += s.Length
	}
	if off != 1000 {
		t.Fatalf("segments cover %d bytes, want 1000", off)
	}
	// the last segment absorbs the remainder
	if segs[3].Length != 250 || segs[0].Length != 250 {
		t.Fatalf("unexpected split: %+v", segs)
	}

	// a remainder that does not divide evenly must all land in the last piece
	odd := splitSegments(1001, 4)
	total := int64(0)
	for _, s := range odd {
		total += s.Length
	}
	if total != 1001 {
		t.Fatalf("odd split covers %d, want 1001", total)
	}
	if odd[3].Length != 251 {
		t.Fatalf("last segment = %d, want 251", odd[3].Length)
	}

	if got := splitSegments(0, 4); got != nil {
		t.Fatalf("zero total should give no segments, got %v", got)
	}
	if got := splitSegments(10, 0); len(got) != 1 {
		t.Fatalf("n<1 should clamp to one segment, got %v", got)
	}
}

func TestParseContentRange(t *testing.T) {
	start, end, total, ok := parseContentRange("bytes 0-0/12345")
	if !ok || start != 0 || end != 0 || total != 12345 {
		t.Fatalf("got %d-%d/%d ok=%v", start, end, total, ok)
	}
	start, end, total, ok = parseContentRange("bytes 100-199/*")
	if !ok || start != 100 || end != 199 || total != 0 {
		t.Fatalf("got %d-%d/%d ok=%v", start, end, total, ok)
	}
	if _, _, _, ok := parseContentRange("nonsense"); ok {
		t.Fatal("expected failure for a non bytes header")
	}
	if _, _, _, ok := parseContentRange("bytes abc"); ok {
		t.Fatal("expected failure without a slash")
	}
}

func TestParseVerify(t *testing.T) {
	cases := map[string]VerifyMode{
		"auto":   VerifyAuto,
		"size":   VerifySize,
		"sha256": VerifySHA256,
		"off":    VerifyOff,
	}
	for in, want := range cases {
		got, ok := ParseVerify(in)
		if !ok || got != want {
			t.Errorf("ParseVerify(%q) = %v,%v", in, got, ok)
		}
	}
	if _, ok := ParseVerify("md5"); ok {
		t.Error("expected md5 to be rejected")
	}
}

func TestWantsHash(t *testing.T) {
	const gib = int64(1) << 30
	if wantsHash(VerifyAuto, 2*gib, "abc", false) != true {
		t.Error("small file with a known hash must verify")
	}
	if wantsHash(VerifyAuto, 20*gib, "abc", false) != false {
		t.Error("huge file without risk should skip the hash")
	}
	if wantsHash(VerifyAuto, 20*gib, "abc", true) != true {
		t.Error("resumed or retried download must verify")
	}
	if wantsHash(VerifyAuto, 2*gib, "", false) != false {
		t.Error("no hash available means nothing to check")
	}
	if wantsHash(VerifyOff, 1, "abc", true) != false {
		t.Error("--verify off must win")
	}
}
