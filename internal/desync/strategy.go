// Package desync implements DPI-bypass strategies that operate on the first
// client write of a TLS connection (the ClientHello). Strategies are pure data
// so they can be enumerated by the finder, pinned by the user, and unit-tested
// without a socket. The actual bytes-on-the-wire manipulation lives in Conn.
package desync

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// SplitMode selects how the ClientHello is broken up on the wire.
type SplitMode int

const (
	// ModeRecord re-fragments the handshake into multiple valid TLS records
	// (RFC 5246 §6.2.1). The server reassembles transparently; a record-scoped
	// DPI misses an SNI that straddles the boundary. Protocol-compliant.
	ModeRecord SplitMode = iota
	// ModeSegment keeps a single TLS record but flushes it to the socket in two
	// writes. Go enables TCP_NODELAY by default, so this yields two TCP
	// segments and the SNI straddles a segment boundary.
	ModeSegment
)

// SplitAt selects where the boundary lands.
type SplitAt int

const (
	// AtSNI splits inside the SNI hostname — the most targeted position.
	AtSNI SplitAt = iota
	// AtFixed splits at a fixed payload byte offset (Strategy.Off).
	AtFixed
)

// Strategy is a single, self-contained desync recipe.
type Strategy struct {
	Mode SplitMode
	At   SplitAt
	// Off is the split position as an offset into the TLS record payload.
	// Used directly when At==AtFixed, and as a fallback when At==AtSNI but no
	// SNI can be located.
	Off int
}

// ErrBadStrategy is returned by ParseStrategy for malformed input.
var ErrBadStrategy = errors.New("desync: invalid strategy")

// Builtins returns the default strategy set, ordered cheapest / most-likely
// first. The finder probes them in this order.
func Builtins() []Strategy {
	return []Strategy{
		{Mode: ModeRecord, At: AtSNI, Off: 1},
		{Mode: ModeSegment, At: AtSNI, Off: 1},
		{Mode: ModeRecord, At: AtFixed, Off: 1},
		{Mode: ModeSegment, At: AtFixed, Off: 1},
		{Mode: ModeRecord, At: AtFixed, Off: 3},
	}
}

// String renders the strategy in the "mode:at[:off]" wire format.
func (s Strategy) String() string {
	mode := "seg"
	if s.Mode == ModeRecord {
		mode = "record"
	}
	if s.At == AtSNI {
		return mode + ":sni"
	}
	return fmt.Sprintf("%s:fixed:%d", mode, s.Off)
}

// ParseStrategy parses the "mode:at[:off]" format, e.g. "record:sni" or
// "seg:fixed:1".
func ParseStrategy(str string) (Strategy, error) {
	parts := strings.Split(strings.TrimSpace(str), ":")
	if len(parts) < 2 {
		return Strategy{}, fmt.Errorf("%w: %q", ErrBadStrategy, str)
	}

	var s Strategy
	switch parts[0] {
	case "record", "rec":
		s.Mode = ModeRecord
	case "seg", "segment":
		s.Mode = ModeSegment
	default:
		return Strategy{}, fmt.Errorf("%w: unknown mode %q", ErrBadStrategy, parts[0])
	}

	switch parts[1] {
	case "sni":
		s.At = AtSNI
		s.Off = 1 // fallback when SNI cannot be located
	case "fixed":
		s.At = AtFixed
		if len(parts) < 3 {
			return Strategy{}, fmt.Errorf("%w: fixed needs an offset", ErrBadStrategy)
		}
		n, err := strconv.Atoi(parts[2])
		if err != nil || n < 1 {
			return Strategy{}, fmt.Errorf("%w: bad offset %q", ErrBadStrategy, parts[2])
		}
		s.Off = n
	default:
		return Strategy{}, fmt.Errorf("%w: unknown split point %q", ErrBadStrategy, parts[1])
	}
	return s, nil
}
