package desync

import (
	"fmt"
	"net"
)

// Conn wraps an upstream net.Conn and applies a Strategy to the first TLS record
// written by the client (the ClientHello). It buffers the initial writes until
// the complete record is available, so a ClientHello that arrives in several
// writes (e.g. a large post-quantum one from a browser) is still desynced as a
// whole. Once the first record is handled, writes pass through untouched; reads
// are never modified. Conn is not safe for concurrent writes, which matches how
// a proxy uses it (one writer per direction).
type Conn struct {
	net.Conn
	strat Strategy
	done  bool
	buf   []byte

	// Trace, if set, is called once with a human-readable description of how the
	// first record was handled. Used by the proxy's --debug mode.
	Trace func(string)
}

// Wrap returns a Conn that applies strat to the first TLS record on c.
func Wrap(c net.Conn, strat Strategy) *Conn {
	return &Conn{Conn: c, strat: strat}
}

// Write intercepts the first TLS record and applies the desync strategy to it.
// It always reports len(b) consumed while buffering, so io.Copy is satisfied.
func (c *Conn) Write(b []byte) (int, error) {
	if c.done {
		return c.Conn.Write(b)
	}
	if len(b) == 0 {
		// Never consume the first-record state on an empty write (bufio's
		// WriteTo flushes an empty buffer before the real data).
		return 0, nil
	}

	c.buf = append(c.buf, b...)

	// Not a TLS handshake record → flush and stop intercepting.
	if c.buf[0] != 0x16 {
		c.trace(fmt.Sprintf("first write is not a TLS handshake (0x%02x); passed through", c.buf[0]))
		return len(b), c.flush()
	}
	// Wait until the full record (5-byte header + payload) is buffered.
	if len(c.buf) < 5 {
		return len(b), nil
	}
	total := 5 + (int(c.buf[3])<<8 | int(c.buf[4]))
	if len(c.buf) < total {
		return len(b), nil // record not complete yet; keep buffering
	}
	return len(b), c.desync(total)
}

// flush writes everything buffered as-is and marks the first record handled.
func (c *Conn) flush() error {
	c.done = true
	p := c.buf
	c.buf = nil
	_, err := c.Conn.Write(p)
	return err
}

// desync applies the strategy to the first complete record (c.buf[:total]) and
// writes it, followed by any trailing bytes, then marks the record handled.
func (c *Conn) desync(total int) error {
	c.done = true
	record := c.buf[:total]
	rest := c.buf[total:]
	c.buf = nil

	if c.strat.At == AtSNI {
		if s, e, err := sniRange(record); err == nil {
			c.trace(fmt.Sprintf("SNI %q at [%d,%d) of %d-byte ClientHello", record[s:e], s, e, total))
		} else {
			c.trace(fmt.Sprintf("SNI not found (%v); falling back to fixed offset %d", err, c.strat.Off))
		}
	}

	p := c.payloadOffset(record)
	if p <= 0 {
		c.trace("no split point; ClientHello sent unmodified")
		return c.writeRaw(record, rest)
	}

	switch c.strat.Mode {
	case ModeRecord:
		frag, err := fragmentRecord(record, p)
		if err != nil {
			c.trace(fmt.Sprintf("fragment failed (%v); ClientHello sent unmodified", err))
			return c.writeRaw(record, rest)
		}
		if _, err := c.Conn.Write(frag); err != nil {
			return err
		}
		c.trace(fmt.Sprintf("record-split at payload offset %d -> records of %d + %d bytes", p, p, total-5-p))
	case ModeSegment:
		raw := 5 + p
		if raw <= 0 || raw >= len(record) {
			c.trace("segment offset out of range; ClientHello sent unmodified")
			return c.writeRaw(record, rest)
		}
		if _, err := c.Conn.Write(record[:raw]); err != nil {
			return err
		}
		if _, err := c.Conn.Write(record[raw:]); err != nil {
			return err
		}
		c.trace(fmt.Sprintf("segment-split at raw offset %d -> %d + %d bytes", raw, raw, len(record)-raw))
	default:
		return c.writeRaw(record, rest)
	}

	if len(rest) > 0 {
		if _, err := c.Conn.Write(rest); err != nil {
			return err
		}
	}
	return nil
}

func (c *Conn) writeRaw(record, rest []byte) error {
	if _, err := c.Conn.Write(record); err != nil {
		return err
	}
	if len(rest) > 0 {
		if _, err := c.Conn.Write(rest); err != nil {
			return err
		}
	}
	return nil
}

func (c *Conn) trace(msg string) {
	if c.Trace != nil {
		c.Trace(msg)
	}
}

// payloadOffset returns the split point as an offset into the record payload,
// or 0 if the strategy cannot be applied to this record.
func (c *Conn) payloadOffset(b []byte) int {
	switch c.strat.At {
	case AtSNI:
		s, e, err := sniRange(b)
		if err != nil {
			return c.strat.Off // fall back to the fixed offset
		}
		mid := (s + e) / 2 // absolute offset in b, inside the hostname
		return mid - 5     // translate to a payload offset
	case AtFixed:
		return c.strat.Off
	default:
		return 0
	}
}
