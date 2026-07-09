// Command unsni is a cross-platform DPI-bypass proxy with an automatic strategy
// finder, a diagnosis mode, and a WARP config generator for UDP/voice.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/YusufDrymz/unsni/internal/desync"
	"github.com/YusufDrymz/unsni/internal/dns"
	"github.com/YusufDrymz/unsni/internal/finder"
	"github.com/YusufDrymz/unsni/internal/metrics"
	"github.com/YusufDrymz/unsni/internal/proxy"
	"github.com/YusufDrymz/unsni/internal/rule"
	"github.com/YusufDrymz/unsni/internal/sysproxy"
	"github.com/YusufDrymz/unsni/internal/warp"
)

var version = "0.1.0-dev"

const defaultDoH = "https://cloudflare-dns.com/dns-query"

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "run":
		cmdRun(os.Args[2:])
	case "find":
		cmdFind(os.Args[2:])
	case "doctor":
		cmdDoctor(os.Args[2:])
	case "warp":
		cmdWarp(os.Args[2:])
	case "strategies":
		cmdStrategies()
	case "version", "-v", "--version":
		fmt.Println("unsni", version)
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `unsni — DPI bypass engine

usage:
  unsni run [--listen addr] [--socks addr] [--strategy s] [--rules file] [--auto] [--system-proxy] [--doh url] [--metrics addr] [--debug]
  unsni find <host> [--doh url] [--timeout d]
  unsni doctor <host> [--doh url] [--timeout d]
  unsni warp [--out file] [--allowed-ips list]
  unsni strategies
  unsni version
`)
}

func cmdRun(args []string) {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	listen := fs.String("listen", "127.0.0.1:8080", "HTTP CONNECT proxy address (empty = off)")
	socks := fs.String("socks", "", "SOCKS5 proxy address, e.g. 127.0.0.1:1080 (empty = off)")
	stratStr := fs.String("strategy", "record:sni", "default desync strategy (see: unsni strategies)")
	rulesPath := fs.String("rules", "", "per-domain rules file")
	auto := fs.Bool("auto", false, "auto-discover a working strategy per host (on a rules miss)")
	doh := fs.String("doh", defaultDoH, "DNS-over-HTTPS endpoint (empty = system resolver)")
	metricsAddr := fs.String("metrics", "127.0.0.1:9090", "Prometheus /metrics address (empty = off)")
	systemProxy := fs.Bool("system-proxy", false, "set the OS system proxy to --listen on start, revert on exit (macOS/Windows)")
	debug := fs.Bool("debug", false, "log per-connection desync detail (SNI, split point)")
	_ = fs.Parse(args)

	strat, err := desync.ParseStrategy(*stratStr)
	if err != nil {
		fatal(err)
	}

	var rules *rule.Set
	if *rulesPath != "" {
		rules, err = rule.Load(*rulesPath)
		if err != nil {
			fatal(err)
		}
	}

	level := slog.LevelInfo
	if *debug {
		level = slog.LevelDebug
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	defer stop()

	if *metricsAddr != "" {
		mux := http.NewServeMux()
		mux.Handle("/metrics", metrics.Handler())
		srv := &http.Server{Addr: *metricsAddr, Handler: mux}
		go func() {
			logger.Info("metrics listening", "addr", *metricsAddr)
			if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				logger.Warn("metrics server", "err", err)
			}
		}()
		go func() { <-ctx.Done(); _ = srv.Close() }()
	}

	var revert func() error
	if *systemProxy {
		if *listen == "" {
			fatal(fmt.Errorf("--system-proxy requires --listen"))
		}
		revert, err = sysproxy.Set(*listen)
		if err != nil {
			fatal(fmt.Errorf("system proxy: %w", err))
		}
		logger.Info("system proxy set", "addr", *listen)
		fmt.Fprintf(os.Stderr, "System proxy is ON. Traffic routes through unsni.\nPress Ctrl+C to stop and restore your settings.\n")
	}

	s := &proxy.Server{
		HTTPListen:  *listen,
		SocksListen: *socks,
		Default:     strat,
		Rules:       rules,
		Auto:        *auto,
		Timeout:     5 * time.Second,
		Resolver:    dns.New(*doh),
		Logger:      logger,
		Debug:       *debug,
	}
	serveErr := s.ListenAndServe(ctx)

	// Revert explicitly (a deferred call would be skipped by fatal's os.Exit).
	if revert != nil {
		if err := revert(); err != nil {
			logger.Error("system proxy revert FAILED — turn it off manually", "err", err)
		} else {
			logger.Info("system proxy reverted")
		}
	}
	if serveErr != nil {
		fatal(serveErr)
	}
}

func cmdFind(args []string) {
	rep := runFinder("find", args)
	best, ok := rep.Best()
	fmt.Println()
	switch {
	case rep.Baseline.OK:
		fmt.Printf("%s is reachable without desync — no bypass needed.\n", rep.Host)
	case ok:
		fmt.Printf("%s appears blocked (baseline handshake failed).\n", rep.Host)
		fmt.Printf("Best strategy: %s\n", best)
		fmt.Printf("Run: unsni run --strategy %s\n", best)
	default:
		fmt.Printf("No working strategy found for %s — likely IP block, or a UDP/QUIC-only path (see: unsni warp).\n", rep.Host)
	}
}

func cmdDoctor(args []string) {
	rep := runFinder("doctor", args)
	fmt.Println()
	fmt.Printf("Diagnosis for %s (%s):\n", rep.Host, rep.IP)
	printResult("baseline (no desync)", rep.Baseline)
	for _, p := range rep.Probes {
		printResult(p.Strategy, p)
	}
	fmt.Println()
	switch best, ok := rep.Best(); {
	case rep.Baseline.OK:
		fmt.Println("Verdict: NOT blocked at the TLS layer (baseline handshake succeeded).")
	case ok:
		fmt.Printf("Verdict: SNI-based DPI block confirmed — baseline fails, %s succeeds.\n", best)
	default:
		fmt.Println("Verdict: baseline fails and no desync strategy helped — likely IP block or UDP/QUIC-only path.")
	}
}

func runFinder(name string, args []string) finder.Report {
	fs := flag.NewFlagSet(name, flag.ExitOnError)
	doh := fs.String("doh", defaultDoH, "DNS-over-HTTPS endpoint (empty = system resolver)")
	timeout := fs.Duration("timeout", 5*time.Second, "per-probe timeout")
	_ = fs.Parse(args)
	if fs.NArg() < 1 {
		fatal(fmt.Errorf("usage: unsni %s <host>", name))
	}
	host := fs.Arg(0)

	ctx := context.Background()
	ips, err := dns.New(*doh).LookupHost(ctx, host)
	if err != nil || len(ips) == 0 {
		fatal(fmt.Errorf("resolve %s: %v", host, err))
	}
	fmt.Printf("Probing %s (%s) ...\n", host, ips[0])
	return finder.Run(ctx, host, ips[0], *timeout)
}

func cmdWarp(args []string) {
	fs := flag.NewFlagSet("warp", flag.ExitOnError)
	out := fs.String("out", "warp.conf", "output WireGuard config path")
	allowed := fs.String("allowed-ips", "0.0.0.0/0, ::/0", "AllowedIPs (narrow this for split-tunnel)")
	_ = fs.Parse(args)

	acc, err := warp.Register(context.Background())
	if err != nil {
		fatal(err)
	}
	if err := os.WriteFile(*out, []byte(acc.WireGuardConf(*allowed)), 0o600); err != nil {
		fatal(err)
	}
	disp := *out
	if !strings.Contains(disp, "/") {
		disp = "./" + disp // wg-quick treats a bare name as an interface, not a file
	}
	fmt.Printf("Wrote %s\n", *out)
	fmt.Printf("Bring the tunnel up to carry UDP/voice:\n  wg-quick up %s\n", disp)
	fmt.Printf("(or import it into the WireGuard app). Take it down with: wg-quick down %s\n", disp)
}

func cmdStrategies() {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "STRATEGY\tMODE\tSPLIT")
	for _, s := range desync.Builtins() {
		mode := "segment"
		if s.Mode == desync.ModeRecord {
			mode = "record"
		}
		at := "inside SNI"
		if s.At == desync.AtFixed {
			at = fmt.Sprintf("fixed offset %d", s.Off)
		}
		fmt.Fprintf(w, "%s\t%s\t%s\n", s.String(), mode, at)
	}
	_ = w.Flush()
}

func printResult(label string, r finder.Result) {
	if r.OK {
		fmt.Printf("  %-22s OK    (%dms)\n", label, r.RTT.Milliseconds())
		return
	}
	msg := r.Err
	if msg == "" {
		msg = "handshake failed"
	}
	fmt.Printf("  %-22s FAIL  (%s)\n", label, msg)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "unsni:", err)
	os.Exit(1)
}
