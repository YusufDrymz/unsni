package rule

import (
	"strings"
	"testing"

	"github.com/YusufDrymz/unsni/internal/desync"
)

func TestParseAndMatch(t *testing.T) {
	src := `
# comment
discord.com        record:sni
*.discordapp.com   seg:fixed:1
example.com        bypass
`
	set, err := Parse(strings.NewReader(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	tests := []struct {
		host      string
		wantOK    bool
		wantByp   bool
		wantStrat string
	}{
		{"discord.com", true, false, "record:sni"},
		{"DISCORD.COM.", true, false, "record:sni"}, // case + trailing dot
		{"cdn.discordapp.com", true, false, "seg:fixed:1"},
		{"discordapp.com", true, false, "seg:fixed:1"}, // a domain rule also covers its apex
		{"example.com", true, true, ""},
		{"google.com", false, false, ""},
	}
	for _, tt := range tests {
		st, byp, ok := set.Match(tt.host)
		if ok != tt.wantOK || byp != tt.wantByp {
			t.Errorf("%s: ok=%v bypass=%v, want ok=%v bypass=%v", tt.host, ok, byp, tt.wantOK, tt.wantByp)
			continue
		}
		if ok && !byp && st.String() != tt.wantStrat {
			t.Errorf("%s: strat=%s, want %s", tt.host, st.String(), tt.wantStrat)
		}
	}
}

func TestParseErrors(t *testing.T) {
	for _, src := range []string{
		"discord.com",             // missing field
		"discord.com bogus:thing", // bad strategy
		"a b c",                   // too many fields
	} {
		if _, err := Parse(strings.NewReader(src)); err == nil {
			t.Errorf("%q: expected error", src)
		}
	}
}

func TestMatchEmptySet(t *testing.T) {
	var s Set
	if _, _, ok := s.Match("discord.com"); ok {
		t.Error("empty set should not match")
	}
	_ = desync.Strategy{}
}
