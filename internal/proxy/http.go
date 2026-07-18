// Package proxy implements local HTTP CONNECT and SOCKS5 proxies that tunnel TLS
// connections through a desync strategy to defeat SNI-based DPI. The strategy
// per host comes from (in order) a rules file, auto-discovery, or the default.
package proxy

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/YusufDrymz/unsni/internal/desync"
	"github.com/YusufDrymz/unsni/internal/dns"
	"github.com/YusufDrymz/unsni/internal/finder"
	"github.com/YusufDrymz/unsni/internal/metrics"
	"github.com/YusufDrymz/unsni/internal/rule"
)

// Server is a local proxy. HTTPListen and/or SocksListen enable the respective
// front-ends; at least one must be set.
type Server struct {
	HTTPListen  string
	SocksListen string
	Default     desync.Strategy
	Rules       *rule.Set
	Auto        bool
	Timeout     time.Duration // per-strategy probe timeout in auto mode
	Resolver    *dns.Resolver
	Logger      *slog.Logger
	Debug       bool

	mu    sync.Mutex
	cache map[string]cacheEntry // auto-discovered strategy per host
}

type cacheEntry struct {
	on    bool
	strat desync.Strategy
}

// boundListener is a front-end whose port is already bound and ready to accept.
type boundListener struct {
	ln     net.Listener
	kind   string
	handle func(context.Context, net.Conn)
}

// Listen binds every enabled front-end and returns the live listeners. Binding
// up front (before Serve) lets callers enable the OS system proxy only once the
// ports are actually accepting — otherwise a bind failure (e.g. port in use)
// would leave the proxy pointing at a dead port. On any bind error, listeners
// already opened are closed before returning.
func (s *Server) Listen() ([]boundListener, error) {
	if s.Timeout == 0 {
		s.Timeout = 5 * time.Second
	}
	s.cache = map[string]cacheEntry{}

	var specs []struct {
		addr, kind string
		handle     func(context.Context, net.Conn)
	}
	if s.HTTPListen != "" {
		specs = append(specs, struct {
			addr, kind string
			handle     func(context.Context, net.Conn)
		}{s.HTTPListen, "http", s.handleHTTP})
	}
	if s.SocksListen != "" {
		specs = append(specs, struct {
			addr, kind string
			handle     func(context.Context, net.Conn)
		}{s.SocksListen, "socks5", s.handleSocks})
	}
	if len(specs) == 0 {
		return nil, fmt.Errorf("no listeners configured (set --listen and/or --socks)")
	}

	var bound []boundListener
	var lc net.ListenConfig
	for _, sp := range specs {
		ln, err := lc.Listen(context.Background(), "tcp", sp.addr)
		if err != nil {
			for _, b := range bound {
				_ = b.ln.Close()
			}
			return nil, fmt.Errorf("listen %s %s: %w", sp.kind, sp.addr, err)
		}
		s.Logger.Info("listening", "proto", sp.kind, "addr", sp.addr)
		bound = append(bound, boundListener{ln: ln, kind: sp.kind, handle: sp.handle})
	}
	return bound, nil
}

// Serve accepts connections on the given listeners until ctx is cancelled.
func (s *Server) Serve(ctx context.Context, listeners []boundListener) error {
	errc := make(chan error, len(listeners))
	for _, l := range listeners {
		l := l
		go func() { errc <- s.accept(ctx, l) }()
	}
	for range listeners {
		if err := <-errc; err != nil {
			return err
		}
	}
	return nil
}

// ListenAndServe binds the enabled front-ends and serves them until ctx is
// cancelled. Prefer Listen + Serve when you need to act between bind and serve.
func (s *Server) ListenAndServe(ctx context.Context) error {
	listeners, err := s.Listen()
	if err != nil {
		return err
	}
	return s.Serve(ctx, listeners)
}

func (s *Server) accept(ctx context.Context, l boundListener) error {
	go func() {
		<-ctx.Done()
		_ = l.ln.Close()
	}()

	for {
		c, err := l.ln.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return nil
			default:
				return err
			}
		}
		go l.handle(ctx, c)
	}
}

func (s *Server) handleHTTP(ctx context.Context, client net.Conn) {
	defer client.Close()

	br := bufio.NewReader(client)
	req, err := http.ReadRequest(br)
	if err != nil {
		return
	}
	if req.Method != http.MethodConnect {
		_, _ = io.WriteString(client, "HTTP/1.1 405 Method Not Allowed\r\n\r\n")
		return
	}

	host, port, err := net.SplitHostPort(req.Host)
	if err != nil {
		host, port = req.Host, "443"
	}

	up, strat, on, err := s.open(ctx, host, port)
	if err != nil {
		_, _ = io.WriteString(client, "HTTP/1.1 502 Bad Gateway\r\n\r\n")
		s.Logger.Warn("dial failed", "host", host, "err", err)
		return
	}
	if _, err := io.WriteString(client, "HTTP/1.1 200 Connection Established\r\n\r\n"); err != nil {
		_ = up.Close()
		return
	}
	s.pipe(client, br, up, strat, on, host, port)
}

// open resolves host, decides the strategy, and dials the upstream. The returned
// bool reports whether desync should be applied (rule/auto/default AND port 443).
func (s *Server) open(ctx context.Context, host, port string) (net.Conn, desync.Strategy, bool, error) {
	ips, err := s.Resolver.LookupHost(ctx, host)
	if err != nil || len(ips) == 0 {
		ips = []string{host}
	}
	strat, on := s.strategyFor(ctx, host, ips)

	up, err := net.DialTimeout("tcp", net.JoinHostPort(ips[0], port), 15*time.Second)
	if err != nil {
		metrics.IncDialError()
		metrics.IncConn("dial_error")
		return nil, strat, on, err
	}
	return up, strat, on && port == "443", nil
}

// pipe wires the client and upstream together, applying desync to the first
// upstream write when on is true. br carries any bytes buffered while parsing the
// proxy request, so the ClientHello is not lost.
func (s *Server) pipe(client net.Conn, br *bufio.Reader, up net.Conn, strat desync.Strategy, on bool, host, port string) {
	defer up.Close()

	upstream := up
	if on {
		dc := desync.Wrap(up, strat)
		if s.Debug {
			h := host
			dc.Trace = func(m string) { s.Logger.Debug("desync", "host", h, "detail", m) }
		}
		upstream = dc
	}

	metrics.IncConn("ok")
	s.Logger.Info("tunnel", "host", host, "port", port, "desync", on)

	go func() {
		_, _ = io.Copy(upstream, br)
		if tc, ok := up.(*net.TCPConn); ok {
			_ = tc.CloseWrite()
		}
	}()
	_, _ = io.Copy(client, up)
}

func (s *Server) strategyFor(ctx context.Context, host string, ips []string) (desync.Strategy, bool) {
	if s.Rules != nil {
		if st, bypass, ok := s.Rules.Match(host); ok {
			return st, !bypass
		}
	}
	if s.Auto && len(ips) > 0 {
		return s.autoStrategy(ctx, host, ips[0])
	}
	return s.Default, true
}

func (s *Server) autoStrategy(ctx context.Context, host, ip string) (desync.Strategy, bool) {
	s.mu.Lock()
	e, ok := s.cache[host]
	s.mu.Unlock()
	if ok {
		return e.strat, e.on
	}

	res := cacheEntry{on: true, strat: s.Default}
	if best, found := finder.First(ctx, host, ip, s.Timeout); found {
		if st, err := desync.ParseStrategy(best); err == nil {
			res.strat = st
			s.Logger.Info("auto: selected strategy", "host", host, "strategy", best)
		}
	} else {
		s.Logger.Warn("auto: no strategy worked, using default", "host", host, "default", s.Default.String())
	}

	s.mu.Lock()
	s.cache[host] = res
	s.mu.Unlock()
	return res.strat, res.on
}
