// Package finder probes desync strategies against a target to discover which one
// defeats the local DPI, and to diagnose why a host is blocked.
package finder

import (
	"context"
	"crypto/tls"
	"net"
	"time"

	"github.com/YusufDrymz/unsni/internal/desync"
)

// Result is the outcome of one probe. Strategy is empty for the baseline.
type Result struct {
	Strategy string
	OK       bool
	RTT      time.Duration
	Err      string
}

// Report is a full diagnosis for a host.
type Report struct {
	Host     string
	IP       string
	Baseline Result
	Probes   []Result
}

// Best returns the fastest working strategy, or ("", false) if none worked.
func (r Report) Best() (string, bool) {
	var best Result
	found := false
	for _, p := range r.Probes {
		if p.OK && (!found || p.RTT < best.RTT) {
			best, found = p, true
		}
	}
	return best.Strategy, found
}

// Run performs a baseline TLS handshake plus one probe per built-in strategy
// against ip (with SNI set to host) and returns a Report. The caller resolves
// host to ip beforehand.
func Run(ctx context.Context, host, ip string, timeout time.Duration) Report {
	rep := Report{Host: host, IP: ip}
	rep.Baseline = probe(ctx, host, ip, nil, timeout)

	for _, s := range desync.Builtins() {
		s := s
		res := probe(ctx, host, ip, &s, timeout)
		res.Strategy = s.String()
		rep.Probes = append(rep.Probes, res)
	}
	return rep
}

// First probes the built-in strategies concurrently and returns the first one
// that completes a TLS handshake. Used by the proxy's auto mode, where a working
// strategy is wanted fast rather than a full ranked report.
func First(ctx context.Context, host, ip string, timeout time.Duration) (string, bool) {
	strats := desync.Builtins()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	type res struct {
		strat string
		ok    bool
	}
	ch := make(chan res, len(strats))
	for _, s := range strats {
		s := s
		go func() {
			r := probe(ctx, host, ip, &s, timeout)
			ch <- res{s.String(), r.OK}
		}()
	}
	for range strats {
		if r := <-ch; r.ok {
			return r.strat, true
		}
	}
	return "", false
}

// probe performs a single TLS handshake, optionally through a desync strategy.
func probe(ctx context.Context, host, ip string, strat *desync.Strategy, timeout time.Duration) Result {
	d := net.Dialer{Timeout: timeout}
	raw, err := d.DialContext(ctx, "tcp", net.JoinHostPort(ip, "443"))
	if err != nil {
		return Result{Err: err.Error()}
	}
	defer raw.Close()

	var conn net.Conn = raw
	if strat != nil {
		conn = desync.Wrap(raw, *strat)
	}
	_ = raw.SetDeadline(time.Now().Add(timeout))

	start := time.Now()
	tc := tls.Client(conn, &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12})
	err = tc.HandshakeContext(ctx)
	rtt := time.Since(start)
	_ = tc.Close()

	if err != nil {
		return Result{RTT: rtt, Err: err.Error()}
	}
	return Result{OK: true, RTT: rtt}
}
