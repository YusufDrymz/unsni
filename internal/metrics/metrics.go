// Package metrics exposes a tiny, dependency-free Prometheus text endpoint.
// Labels are kept deliberately low-cardinality (outcome only) — the per-host
// detail belongs in structured logs, not in metric series.
package metrics

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
)

var (
	mu       sync.Mutex
	counters = map[string]float64{}
	help     = map[string]string{}
	mtype    = map[string]string{}
)

func register(name, htext string) {
	if _, ok := help[name]; !ok {
		help[name] = htext
		mtype[name] = "counter"
	}
}

// IncConn increments the connection counter for a given outcome (e.g. "ok",
// "dial_error").
func IncConn(outcome string) {
	const name = "unsni_connections_total"
	register(name, "Proxied connections by outcome.")
	mu.Lock()
	counters[fmt.Sprintf("%s{outcome=%q}", name, outcome)]++
	mu.Unlock()
}

// IncDialError increments the upstream dial failure counter.
func IncDialError() {
	const name = "unsni_dial_errors_total"
	register(name, "Upstream dial failures.")
	mu.Lock()
	counters[name]++
	mu.Unlock()
}

// Handler renders the current metrics in Prometheus text exposition format.
func Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		defer mu.Unlock()

		w.Header().Set("Content-Type", "text/plain; version=0.0.4")

		names := make([]string, 0, len(help))
		for n := range help {
			names = append(names, n)
		}
		sort.Strings(names)

		keys := make([]string, 0, len(counters))
		for k := range counters {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for _, n := range names {
			fmt.Fprintf(w, "# HELP %s %s\n", n, help[n])
			fmt.Fprintf(w, "# TYPE %s %s\n", n, mtype[n])
			for _, k := range keys {
				if k == n || strings.HasPrefix(k, n+"{") {
					fmt.Fprintf(w, "%s %g\n", k, counters[k])
				}
			}
		}
	})
}
