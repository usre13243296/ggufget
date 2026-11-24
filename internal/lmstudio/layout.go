// Layout decides where a downloaded .gguf goes inside the models directory.
//
// This is not cosmetic: LM Studio only shows models whose folder it
// recognises. The layout LM Studio itself produces is
//
//	<models>/<file-stem>/lmstudio-ai/<file>.gguf
//
// (first level named after the file without its extension, second level the
// vendor folder, then the file). A plain Hugging Face tree
// (<publisher>/<repo>/<file>.gguf) is kept as an alternative because it is
// useful for sharing a folder with other tools.
package lmstudio

import (
	"path"
	"path/filepath"
	"strings"
)

// Layout names a destination arrangement.
type Layout string

const (
	// LayoutLMStudio is the layout LM Studio indexes:
	//   <stem>/lmstudio-ai/<file>.gguf
	LayoutLMStudio Layout = "lmstudio"
	// LayoutRepo mirrors the Hugging Face hub tree:
	//   <publisher>/<repo>/<file>.gguf
	LayoutRepo Layout = "repo"
	// LayoutFlat drops the file straight into the target directory.
	LayoutFlat Layout = "flat"
)

// VendorDir is the second level of the LM Studio layout.
const VendorDir = "lmstudio-ai"

// ParseLayout accepts the layout names plus a few friendly aliases.
func ParseLayout(s string) (Layout, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", string(LayoutLMStudio), "lms", "hub", "lm-studio":
		return LayoutLMStudio, true
	case string(LayoutRepo), "hf", "huggingface":
		return LayoutRepo, true
	case string(LayoutFlat), "none", "plain":
		return LayoutFlat, true
	}
	return LayoutLMStudio, false
}

// Stem returns the file name without its extension, which is what LM Studio
// uses as the first directory level.
func Stem(file string) string {
	base := path.Base(filepath.ToSlash(file))
	if i := strings.LastIndex(strings.ToLower(base), ".gguf"); i > 0 {
		return base[:i]
	}
	if i := strings.LastIndex(base, "."); i > 0 {
		return base[:i]
	}
	return base
}

// Rel returns the slash-separated path of file relative to the models
// directory for the given layout. owner may be empty.
func Rel(layout Layout, owner, repo, file string) string {
	base := path.Base(filepath.ToSlash(file))
	switch layout {
	case LayoutRepo:
		switch {
		case owner != "" && repo != "":
			return owner + "/" + repo + "/" + base
		case repo != "":
			return repo + "/" + base
		}
		return base
	case LayoutFlat:
		return base
	default:
		return Stem(base) + "/" + VendorDir + "/" + base
	}
}
