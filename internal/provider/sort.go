package provider

import (
	"sort"
	"strings"

	"ggufget/internal/units"
)

// ParseSort maps the --sort value onto a SortMode.
func ParseSort(s string) (SortMode, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "name":
		return SortName, true
	case "size":
		return SortSize, true
	case "quant":
		return SortQuant, true
	}
	return SortName, false
}

// SortFiles orders a copy of files. The index numbers shown by `list` and
// accepted by `get -f` come from this order, so both commands must call it.
func SortFiles(files []File, mode SortMode) []File {
	out := make([]File, len(files))
	copy(out, files)
	switch mode {
	case SortSize:
		sort.SliceStable(out, func(i, j int) bool {
			if out[i].Size != out[j].Size {
				return out[i].Size < out[j].Size
			}
			return out[i].Path < out[j].Path
		})
	case SortQuant:
		sort.SliceStable(out, func(i, j int) bool {
			qi, qj := units.Quant(out[i].Path), units.Quant(out[j].Path)
			if qi != qj {
				return qi < qj
			}
			if out[i].Size != out[j].Size {
				return out[i].Size < out[j].Size
			}
			return out[i].Path < out[j].Path
		})
	default:
		sort.SliceStable(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	}
	return out
}

// FilterFiles keeps entries whose path contains the keyword, case-insensitively.
func FilterFiles(files []File, keyword string) []File {
	keyword = strings.ToLower(strings.TrimSpace(keyword))
	if keyword == "" {
		return files
	}
	out := make([]File, 0, len(files))
	for _, f := range files {
		if strings.Contains(strings.ToLower(f.Path), keyword) {
			out = append(out, f)
		}
	}
	return out
}
