// Package rule maps hostnames to desync strategies, so different sites can use
// different bypass recipes (or be excluded from desync entirely).
package rule

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/YusufDrymz/unsni/internal/desync"
)

type entry struct {
	suffix string // matches host == suffix or host ending in "." + suffix
	bypass bool
	strat  desync.Strategy
}

// Set is an ordered list of host rules.
type Set struct {
	entries []entry
}

// Match returns the strategy for host. ok is false when no rule matches (the
// caller should fall back to its default or auto behavior). When bypass is true
// the host must be tunneled without any desync.
func (s *Set) Match(host string) (strat desync.Strategy, bypass bool, ok bool) {
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	for _, e := range s.entries {
		if host == e.suffix || strings.HasSuffix(host, "."+e.suffix) {
			return e.strat, e.bypass, true
		}
	}
	return desync.Strategy{}, false, false
}

// Load reads rules from a file.
func Load(path string) (*Set, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return Parse(f)
}

// Parse reads rules, one per line:
//
//	discord.com        record:sni
//	*.discordapp.com   record:sni
//	example.com        bypass
//
// Blank lines and lines starting with '#' are ignored.
func Parse(r io.Reader) (*Set, error) {
	var set Set
	sc := bufio.NewScanner(r)
	n := 0
	for sc.Scan() {
		n++
		text := strings.TrimSpace(sc.Text())
		if text == "" || strings.HasPrefix(text, "#") {
			continue
		}
		fields := strings.Fields(text)
		if len(fields) != 2 {
			return nil, fmt.Errorf("rule line %d: expected '<host> <strategy|bypass>', got %q", n, text)
		}
		suffix := strings.TrimSuffix(strings.ToLower(strings.TrimPrefix(fields[0], "*.")), ".")
		e := entry{suffix: suffix}
		if fields[1] == "bypass" {
			e.bypass = true
		} else {
			st, err := desync.ParseStrategy(fields[1])
			if err != nil {
				return nil, fmt.Errorf("rule line %d: %w", n, err)
			}
			e.strat = st
		}
		set.entries = append(set.entries, e)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return &set, nil
}
