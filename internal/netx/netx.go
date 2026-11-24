// Package netx builds the HTTP clients ggufget uses. One rule matters above
// all: never set http.Client.Timeout, because it would abort a multi-gigabyte
// download mid-flight. Timeouts live on the transport and on request contexts.
package netx

import (
	"context"
	"net"
	"net/http"
	"net/url"
	"sort"
	"time"
)

// Version is stamped into the User-Agent.
const Version = "1.2.1"

// UserAgent identifies the tool to mirrors that rate-limit per client.
const UserAgent = "ggufget/" + Version

// Options configures a client.
type Options struct {
	// HeaderTimeout bounds connect + response-header time, not the body.
	HeaderTimeout time.Duration
	// Proxy is an explicit proxy URL; empty means "use the environment".
	Proxy string
}

// New returns a client tuned for many parallel range requests against a small
// number of hosts.
func New(opts Options) *http.Client {
	if opts.HeaderTimeout <= 0 {
		opts.HeaderTimeout = 30 * time.Second
	}
	dialer := &net.Dialer{Timeout: 15 * time.Second, KeepAlive: 30 * time.Second}
	tr := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           dialContext(dialer),
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          128,
		MaxIdleConnsPerHost:   64,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   15 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ResponseHeaderTimeout: opts.HeaderTimeout,
	}
	if opts.Proxy != "" {
		if pu, err := url.Parse(opts.Proxy); err == nil {
			tr.Proxy = http.ProxyURL(pu)
		}
	}
	return &http.Client{Transport: tr}
}

// dialContext tries IPv4 addresses first. On networks where IPv6 routes go
// nowhere but AAAA records still resolve, Go would otherwise sit on a v6
// handshake until the dial timeout on every single request.
func dialContext(d *net.Dialer) func(ctx context.Context, network, addr string) (net.Conn, error) {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(addr)
		if err != nil || net.ParseIP(host) != nil {
			return d.DialContext(ctx, network, addr)
		}
		ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil || len(ips) < 2 {
			return d.DialContext(ctx, network, addr)
		}
		sort.SliceStable(ips, func(i, j int) bool {
			return ips[i].IP.To4() != nil && ips[j].IP.To4() == nil
		})
		var lastErr error
		for _, ip := range ips {
			c, err := d.DialContext(ctx, network, net.JoinHostPort(ip.IP.String(), port))
			if err == nil {
				return c, nil
			}
			lastErr = err
			if ctx.Err() != nil {
				break
			}
		}
		if lastErr == nil {
			lastErr = &net.OpError{Op: "dial", Net: network, Err: err}
		}
		return nil, lastErr
	}
}

// NewRequest builds a GET with the shared User-Agent and optional headers.
func NewRequest(url string, headers map[string]string) (*http.Request, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", UserAgent)
	req.Header.Set("Accept-Encoding", "identity")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	return req, nil
}
