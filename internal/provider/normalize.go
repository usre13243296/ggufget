package provider

import (
	"errors"
	"strings"
)

// quoteChars are stripped from both ends of a pasted repository reference.
const quoteChars = "\"'` \t\r\n「」『』“”‘’"

// hostPrefixes maps the URL shapes people paste onto a bare owner/name. Longer
// prefixes come first so that "modelscope.cn/models/" wins over
// "modelscope.cn/".
var hostPrefixes = []string{
	"www.modelscope.cn/models/",
	"modelscope.cn/models/",
	"www.modelscope.cn/",
	"modelscope.cn/",
	"www.huggingface.co/",
	"huggingface.co/",
	"www.hf-mirror.com/",
	"hf-mirror.com/",
	"hf.co/",
	"www.modelscope.ai/models/",
	"modelscope.ai/models/",
}

// urlTailKeywords end a repository path: everything after them is a revision or
// a file path the user does not need to retype.
var urlTailKeywords = map[string]bool{
	"tree": true, "resolve": true, "blob": true, "raw": true,
	"commit": true, "commits": true, "blame": true, "src": true,
}

// NormalizeRepo turns whatever the user pasted into "owner/name".
//
// Accepted inputs include bare ids, full hub URLs, URLs with a /tree/<rev> or
// /resolve/<rev>/<file> tail, a trailing .git, and the same wrapped in quotes.
func NormalizeRepo(raw string) (string, error) {
	s := strings.Trim(raw, quoteChars)
	if s == "" {
		return "", errors.New("empty repository")
	}
	if i := strings.IndexAny(s, "?#"); i >= 0 {
		s = s[:i]
	}
	for _, scheme := range []string{"https://", "http://"} {
		s = strings.TrimPrefix(s, scheme)
	}
	s = strings.TrimSuffix(s, "/")

	lower := strings.ToLower(s)
	for _, prefix := range hostPrefixes {
		if strings.HasPrefix(lower, prefix) {
			s = s[len(prefix):]
			break
		}
	}
	s = strings.TrimSuffix(s, ".git")

	parts := make([]string, 0, 2)
	for _, seg := range strings.Split(s, "/") {
		if seg == "" {
			continue
		}
		if urlTailKeywords[strings.ToLower(seg)] {
			break
		}
		parts = append(parts, seg)
		if len(parts) == 2 {
			break
		}
	}
	if len(parts) < 2 {
		return "", errors.New("repository must look like owner/name")
	}
	owner, name := parts[0], parts[1]
	if strings.ContainsAny(owner+name, " \t\r\n") || name == "." || name == ".." {
		return "", errors.New("repository must look like owner/name")
	}
	return owner + "/" + name, nil
}

// SplitRepo returns the two parts of a normalized repository id.
func SplitRepo(repo string) (owner, name string) {
	if i := strings.IndexByte(repo, '/'); i >= 0 {
		return repo[:i], repo[i+1:]
	}
	return "", repo
}
