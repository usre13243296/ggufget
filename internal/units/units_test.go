package units

import "testing"

func TestBytes(t *testing.T) {
	cases := []struct {
		in   int64
		want string
	}{
		{0, "0 B"},
		{1023, "1023 B"},
		{1024, "1.0 KiB"},
		{1536, "1.5 KiB"},
		{1024 * 1024, "1.0 MiB"},
		{7300776320, "6.80 GiB"},
	}
	for _, c := range cases {
		if got := Bytes(c.in); got != c.want {
			t.Errorf("Bytes(%d) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestQuant(t *testing.T) {
	cases := map[string]string{
		"Qwen2.5-7B-Instruct-Q4_K_M.gguf": "Q4_K_M",
		"qwen2.5-7b-instruct-q8_0.gguf":   "Q8_0",
		"model-IQ2_XXS.gguf":              "IQ2_XXS",
		"model-Q5_K_S.gguf":               "Q5_K_S",
		"no-quant-here.gguf":              "",
	}
	for in, want := range cases {
		if got := Quant(in); got != want {
			t.Errorf("Quant(%q) = %q, want %q", in, got, want)
		}
	}
}
