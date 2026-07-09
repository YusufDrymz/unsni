// Package dns provides a DNS-over-HTTPS resolver with a system fallback, used to
// sidestep DNS-based censorship (poisoned A records).
package dns

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"
)

// Resolver resolves hostnames via DoH, falling back to the system resolver.
type Resolver struct {
	endpoint string
	hc       *http.Client
	sys      *net.Resolver
}

// New returns a Resolver. An empty endpoint disables DoH (system resolver only).
func New(endpoint string) *Resolver {
	return &Resolver{
		endpoint: endpoint,
		hc:       &http.Client{Timeout: 5 * time.Second},
		sys:      net.DefaultResolver,
	}
}

// LookupHost returns IP strings for host. It tries DoH first (when configured),
// then the system resolver. A host that is already an IP literal is returned as-is.
func (r *Resolver) LookupHost(ctx context.Context, host string) ([]string, error) {
	if net.ParseIP(host) != nil {
		return []string{host}, nil
	}
	if r.endpoint != "" {
		if ips, err := r.doh(ctx, host); err == nil && len(ips) > 0 {
			return ips, nil
		}
	}
	return r.sys.LookupHost(ctx, host)
}

type dohResponse struct {
	Answer []struct {
		Type int    `json:"type"`
		Data string `json:"data"`
	} `json:"Answer"`
}

func (r *Resolver) doh(ctx context.Context, host string) ([]string, error) {
	q := url.Values{}
	q.Set("name", host)
	q.Set("type", "A")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.endpoint+"?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/dns-json")

	resp, err := r.hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("doh: status %d", resp.StatusCode)
	}

	var out dohResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	var ips []string
	for _, a := range out.Answer {
		if a.Type == 1 && net.ParseIP(a.Data) != nil { // type 1 = A record
			ips = append(ips, a.Data)
		}
	}
	return ips, nil
}
