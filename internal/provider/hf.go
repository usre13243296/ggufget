package provider

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"ggufget/internal/netx"
)

// HF talks to huggingface.co and any compatible mirror (hf-mirror.com).
type HF struct {
	name   string
	base   string
	client *http.Client
	token  string
}

// NewHF builds a Hugging Face style provider. name is the value used by
// --provider ("hf" or "hf-mirror"), base is the site root without a trailing
// slash.
func NewHF(name, base string, c *http.Client, token string) *HF {
	return &HF{name: name, base: strings.TrimSuffix(base, "/"), client: c, token: token}
}

func (p *HF) Name() string { return p.name }
func (p *HF) Host() string { return p.base }

type hfSibling struct {
	RFilename string `json:"rfilename"`
	Size      int64  `json:"size"`
	LFS       *struct {
		SHA256 string `json:"sha256"`
	} `json:"lfs"`
}

type hfModel struct {
	ID       string          `json:"id"`
	SHA      string          `json:"sha"`
	Gated    json.RawMessage `json:"gated"`
	Private  bool            `json:"private"`
	CardData struct {
		License string `json:"license"`
	} `json:"cardData"`
	Tags     []string    `json:"tags"`
	Siblings []hfSibling `json:"siblings"`
}

func (p *HF) headers() map[string]string {
	if p.token == "" {
		return nil
	}
	return map[string]string{"Authorization": "Bearer " + p.token}
}

func (p *HF) fetch(ctx context.Context, repo, rev string) (*hfModel, error) {
	u := p.base + "/api/models/" + EscapePath(repo) + "?blobs=true"
	if rev != "" {
		u += "&revision=" + url.QueryEscape(rev)
	}
	req, err := netx.NewRequest(u, p.headers())
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, MapStatus(resp.StatusCode, u, string(body))
	}
	var m hfModel
	if err := json.NewDecoder(resp.Body).Decode(&m); err != nil {
		return nil, err
	}
	return &m, nil
}

func (p *HF) meta(repo string, m *hfModel) *Meta {
	return &Meta{
		Repo:    repo,
		Commit:  m.SHA,
		Branch:  "main",
		Gated:   isGated(m.Gated),
		Private: m.Private,
		License: licenseOf(m),
	}
}

func (p *HF) Probe(ctx context.Context, repo string) (*Meta, error) {
	m, err := p.fetch(ctx, repo, "")
	if err != nil {
		return nil, err
	}
	return p.meta(repo, m), nil
}

func (p *HF) List(ctx context.Context, repo, rev string) (*Meta, []File, error) {
	m, err := p.fetch(ctx, repo, rev)
	if err != nil {
		return nil, nil, err
	}
	meta := p.meta(repo, m)
	if meta.Commit == "" {
		meta.Commit = rev
	}
	files := make([]File, 0, len(m.Siblings))
	for _, s := range m.Siblings {
		f := File{Path: s.RFilename, Size: s.Size}
		if s.LFS != nil {
			f.LFS = true
			f.SHA256 = s.LFS.SHA256
		}
		files = append(files, f)
	}
	return meta, files, nil
}

func (p *HF) ResolveURL(repo, rev, path string) string {
	if rev == "" {
		rev = "main"
	}
	return p.base + "/" + EscapePath(repo) + "/resolve/" + url.PathEscape(rev) + "/" + EscapePath(path)
}

// isGated handles both the boolean and the string ("auto"/"manual") forms the
// API has used over the years.
func isGated(raw json.RawMessage) bool {
	s := strings.TrimSpace(string(raw))
	switch s {
	case "", "null", "false":
		return false
	}
	return true
}

func licenseOf(m *hfModel) string {
	if m.CardData.License != "" {
		return m.CardData.License
	}
	for _, t := range m.Tags {
		if strings.HasPrefix(t, "license:") {
			return strings.TrimPrefix(t, "license:")
		}
	}
	return ""
}

// Hit is one search result.
type Hit struct {
	ID        string `json:"id"`
	Downloads int64  `json:"downloads"`
}

// Search queries a Hugging Face compatible endpoint. ModelScope exposes no
// public search API that this tool can rely on, so search is HF only.
func Search(ctx context.Context, c *http.Client, endpoint, q string, limit int) ([]Hit, error) {
	if limit <= 0 {
		limit = 10
	}
	u := strings.TrimSuffix(endpoint, "/") + "/api/models?search=" + url.QueryEscape(q) +
		"&limit=" + strconv.Itoa(limit) + "&sort=downloads&direction=-1"
	req, err := netx.NewRequest(u, nil)
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, MapStatus(resp.StatusCode, u, string(body))
	}
	var out []Hit
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out, nil
}
