package provider

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"

	"ggufget/internal/netx"
)

// MS talks to modelscope.cn, which is consistently the fastest mirror from
// mainland China but does not carry every repository.
type MS struct {
	name   string
	base   string
	client *http.Client
	token  string
}

// NewMS builds a ModelScope provider.
func NewMS(base string, c *http.Client, token string) *MS {
	return &MS{name: "modelscope", base: strings.TrimSuffix(base, "/"), client: c, token: token}
}

func (p *MS) Name() string { return p.name }
func (p *MS) Host() string { return p.base }

type msFile struct {
	Path     string `json:"Path"`
	Size     int64  `json:"Size"`
	Sha256   string `json:"Sha256"`
	Revision string `json:"Revision"`
	Type     string `json:"Type"`
	IsLFS    bool   `json:"IsLFS"`
}

type msEnvelope struct {
	Code    int    `json:"Code"`
	Message string `json:"Message"`
	Data    *struct {
		Files           []msFile `json:"Files"`
		DefaultRevision string   `json:"DefaultRevision"`
	} `json:"Data"`
}

// defaultRev is ModelScope's branch name; unlike Hugging Face it is "master".
const msDefaultRev = "master"

func (p *MS) headers() map[string]string {
	if p.token == "" {
		return nil
	}
	return map[string]string{"Authorization": "Bearer " + p.token}
}

func (p *MS) fetch(ctx context.Context, repo, rev string) (*msEnvelope, error) {
	if rev == "" {
		rev = msDefaultRev
	}
	u := p.base + "/api/v1/models/" + EscapePath(repo) + "/repo/files?Revision=" + url.QueryEscape(rev)
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
	var env msEnvelope
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		return nil, err
	}
	// ModelScope answers HTTP 200 with a business code for some failures.
	if env.Code != 0 && env.Code != 200 && env.Data == nil {
		if env.Code == http.StatusNotFound {
			return nil, MapStatus(http.StatusNotFound, u, env.Message)
		}
		return nil, &HTTPError{Status: env.Code, URL: u, Body: env.Message}
	}
	return &env, nil
}

func (p *MS) Probe(ctx context.Context, repo string) (*Meta, error) {
	meta, _, err := p.List(ctx, repo, "")
	return meta, err
}

func (p *MS) List(ctx context.Context, repo, rev string) (*Meta, []File, error) {
	env, err := p.fetch(ctx, repo, rev)
	if err != nil {
		return nil, nil, err
	}
	if env.Data == nil {
		return nil, nil, MapStatus(http.StatusNotFound, p.base+"/"+repo, "empty response")
	}
	files := make([]File, 0, len(env.Data.Files))
	commit := ""
	commits := map[string]bool{}
	for _, f := range env.Data.Files {
		if f.Type != "" && f.Type != "blob" {
			continue
		}
		files = append(files, File{Path: f.Path, Size: f.Size, SHA256: f.Sha256, LFS: f.IsLFS})
		if f.Revision != "" {
			commits[f.Revision] = true
			commit = f.Revision
		}
	}
	// Every file in one listing comes from a single snapshot; only pin the
	// commit when they agree, otherwise fall back to the branch name.
	if len(commits) != 1 {
		commit = rev
	}
	if commit == "" {
		commit = msDefaultRev
	}
	return &Meta{Repo: repo, Commit: commit, Branch: msDefaultRev}, files, nil
}

func (p *MS) ResolveURL(repo, rev, path string) string {
	if rev == "" {
		rev = msDefaultRev
	}
	return p.base + "/models/" + EscapePath(repo) + "/resolve/" + url.PathEscape(rev) + "/" + EscapePath(path)
}
