package desync

import (
	"bytes"
	"io"
	"net"
	"testing"
	"time"
)

// buildClientHello constructs a minimal but structurally valid TLS 1.2
// ClientHello record carrying a single SNI extension with the given host.
func buildClientHello(host string) []byte {
	h := []byte(host)

	// server_name entry: name_type(1)=0 + name_len(2) + name
	entry := append([]byte{0x00, byte(len(h) >> 8), byte(len(h))}, h...)
	// server_name_list: list_len(2) + entry
	list := append([]byte{byte(len(entry) >> 8), byte(len(entry))}, entry...)
	// SNI extension: type(2)=0x0000 + ext_len(2) + list
	ext := append([]byte{0x00, 0x00, byte(len(list) >> 8), byte(len(list))}, list...)
	// extensions block: total_len(2) + ext
	exts := append([]byte{byte(len(ext) >> 8), byte(len(ext))}, ext...)

	body := []byte{0x03, 0x03}                  // legacy_version
	body = append(body, make([]byte, 32)...)    // random
	body = append(body, 0x00)                   // session_id length = 0
	body = append(body, 0x00, 0x02, 0x00, 0x2f) // cipher_suites: len 2 + one suite
	body = append(body, 0x01, 0x00)             // compression_methods: len 1 + null
	body = append(body, exts...)

	hs := append([]byte{0x01, byte(len(body) >> 16), byte(len(body) >> 8), byte(len(body))}, body...)
	rec := append([]byte{0x16, 0x03, 0x01, byte(len(hs) >> 8), byte(len(hs))}, hs...)
	return rec
}

// reassemble concatenates the payloads of every TLS record in stream, mimicking
// what a server does. It fails the test on a truncated or trailing stream.
func reassemble(t *testing.T, stream []byte) []byte {
	t.Helper()
	var out []byte
	for len(stream) >= 5 {
		l := int(stream[3])<<8 | int(stream[4])
		if 5+l > len(stream) {
			t.Fatalf("truncated record: need %d, have %d", 5+l, len(stream))
		}
		out = append(out, stream[5:5+l]...)
		stream = stream[5+l:]
	}
	if len(stream) != 0 {
		t.Fatalf("trailing %d bytes not on a record boundary", len(stream))
	}
	return out
}

func countRecords(stream []byte) int {
	n := 0
	for len(stream) >= 5 {
		l := int(stream[3])<<8 | int(stream[4])
		if 5+l > len(stream) {
			break
		}
		stream = stream[5+l:]
		n++
	}
	return n
}

func TestSNIRange(t *testing.T) {
	tests := []string{"discord.com", "a.co", "gateway.discord.gg", "xn--nxasmq6b.example"}
	for _, host := range tests {
		rec := buildClientHello(host)
		start, end, err := sniRange(rec)
		if err != nil {
			t.Fatalf("%s: sniRange: %v", host, err)
		}
		if got := string(rec[start:end]); got != host {
			t.Errorf("%s: sniRange returned %q", host, got)
		}
	}
}

func TestSNIRangeMalformed(t *testing.T) {
	cases := map[string][]byte{
		"empty":         {},
		"not-handshake": {0x17, 0x03, 0x03, 0x00, 0x01, 0x00},
		"truncated":     {0x16, 0x03, 0x01, 0x00, 0x20, 0x01},
		"no-extensions": buildClientHello("")[:45],
	}
	for name, buf := range cases {
		if _, _, err := sniRange(buf); err == nil {
			t.Errorf("%s: expected error, got nil", name)
		}
	}
}

func TestFragmentRecordReassembles(t *testing.T) {
	rec := buildClientHello("discord.com")
	payload := rec[5:]
	for _, at := range []int{1, 3, len(payload) / 2, len(payload) - 1} {
		frag, err := fragmentRecord(rec, at)
		if err != nil {
			t.Fatalf("at=%d: %v", at, err)
		}
		if countRecords(frag) != 2 {
			t.Errorf("at=%d: got %d records, want 2", at, countRecords(frag))
		}
		if got := reassemble(t, frag); !bytes.Equal(got, payload) {
			t.Errorf("at=%d: reassembled payload differs", at)
		}
	}
}

func TestFragmentRecordBadOffset(t *testing.T) {
	rec := buildClientHello("discord.com")
	payload := rec[5:]
	for _, at := range []int{0, -1, len(payload), len(payload) + 5} {
		if _, err := fragmentRecord(rec, at); err == nil {
			t.Errorf("at=%d: expected error", at)
		}
	}
}

// fakeConn captures everything written to it and reads nothing.
type fakeConn struct{ buf bytes.Buffer }

func (f *fakeConn) Read([]byte) (int, error)         { return 0, io.EOF }
func (f *fakeConn) Write(b []byte) (int, error)      { return f.buf.Write(b) }
func (f *fakeConn) Close() error                     { return nil }
func (f *fakeConn) LocalAddr() net.Addr              { return nil }
func (f *fakeConn) RemoteAddr() net.Addr             { return nil }
func (f *fakeConn) SetDeadline(time.Time) error      { return nil }
func (f *fakeConn) SetReadDeadline(time.Time) error  { return nil }
func (f *fakeConn) SetWriteDeadline(time.Time) error { return nil }

func TestConnAppliesStrategy(t *testing.T) {
	rec := buildClientHello("discord.com")
	payload := rec[5:]

	for _, s := range Builtins() {
		f := &fakeConn{}
		c := Wrap(f, s)
		n, err := c.Write(rec)
		if err != nil {
			t.Fatalf("%s: write: %v", s, err)
		}
		if n != len(rec) {
			t.Errorf("%s: wrote n=%d, want %d", s, n, len(rec))
		}

		out := f.buf.Bytes()
		// Whatever the strategy, the server must reassemble the exact payload.
		if got := reassemble(t, out); !bytes.Equal(got, payload) {
			t.Errorf("%s: reassembled payload differs", s)
		}
		switch s.Mode {
		case ModeRecord:
			if countRecords(out) < 2 {
				t.Errorf("%s: expected >=2 records, got %d", s, countRecords(out))
			}
		case ModeSegment:
			if !bytes.Equal(out, rec) {
				t.Errorf("%s: segment mode must preserve raw bytes", s)
			}
		}
	}
}

func TestConnPassThroughNonTLS(t *testing.T) {
	f := &fakeConn{}
	c := Wrap(f, Builtins()[0])
	data := []byte("GET / HTTP/1.1\r\n\r\n")
	if _, err := c.Write(data); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(f.buf.Bytes(), data) {
		t.Errorf("non-TLS first write was modified")
	}
}

// TestConnEmptyFirstWrite guards the proxy path: io.Copy via bufio.Reader.WriteTo
// emits an empty write before the ClientHello. That empty write must not consume
// the first-write flag, or the ClientHello slips through undesynced.
func TestConnEmptyFirstWrite(t *testing.T) {
	rec := buildClientHello("discord.com")
	payload := rec[5:]

	f := &fakeConn{}
	c := Wrap(f, Strategy{Mode: ModeRecord, At: AtSNI, Off: 1})
	if _, err := c.Write(nil); err != nil { // empty flush, like bufio's writeBuf
		t.Fatalf("empty write: %v", err)
	}
	if _, err := c.Write(rec); err != nil { // the real ClientHello
		t.Fatalf("clienthello write: %v", err)
	}

	out := f.buf.Bytes()
	if countRecords(out) < 2 {
		t.Errorf("desync not applied after empty first write: %d records", countRecords(out))
	}
	if got := reassemble(t, out); !bytes.Equal(got, payload) {
		t.Errorf("reassembled payload differs")
	}
}

// buildBigClientHello mimics a browser ClientHello: SNI is not the first
// extension (a GREASE ext precedes it) and a large padding extension pushes the
// record well past a typical MSS, so it would arrive in several writes.
func buildBigClientHello(host string) []byte {
	h := []byte(host)
	entry := append([]byte{0x00, byte(len(h) >> 8), byte(len(h))}, h...)
	list := append([]byte{byte(len(entry) >> 8), byte(len(entry))}, entry...)
	sniExt := append([]byte{0x00, 0x00, byte(len(list) >> 8), byte(len(list))}, list...)

	greaseData := []byte{0x00}
	grease := append([]byte{0x0a, 0x0a, byte(len(greaseData) >> 8), byte(len(greaseData))}, greaseData...)

	pad := make([]byte, 1600)
	padExt := append([]byte{0x00, 0x15, byte(len(pad) >> 8), byte(len(pad))}, pad...)

	exts := append([]byte{}, grease...)
	exts = append(exts, sniExt...)
	exts = append(exts, padExt...)
	extsBlock := append([]byte{byte(len(exts) >> 8), byte(len(exts))}, exts...)

	body := []byte{0x03, 0x03}
	body = append(body, make([]byte, 32)...)
	body = append(body, 0x00)
	body = append(body, 0x00, 0x02, 0x00, 0x2f)
	body = append(body, 0x01, 0x00)
	body = append(body, extsBlock...)

	hs := append([]byte{0x01, byte(len(body) >> 16), byte(len(body) >> 8), byte(len(body))}, body...)
	return append([]byte{0x16, 0x03, 0x01, byte(len(hs) >> 8), byte(len(hs))}, hs...)
}

func firstRecordPayload(stream []byte) []byte {
	if len(stream) < 5 {
		return nil
	}
	l := int(stream[3])<<8 | int(stream[4])
	if 5+l > len(stream) {
		return nil
	}
	return stream[5 : 5+l]
}

func TestSNIRangeBigClientHello(t *testing.T) {
	host := "discord.com"
	rec := buildBigClientHello(host)
	start, end, err := sniRange(rec)
	if err != nil {
		t.Fatalf("sniRange: %v", err)
	}
	if got := string(rec[start:end]); got != host {
		t.Errorf("sniRange returned %q", got)
	}
}

// TestConnSplitAcrossWrites is the regression guard for large browser
// ClientHellos delivered in multiple writes: desync must still fire, and the SNI
// must straddle the record boundary (not appear whole in the first record).
func TestConnSplitAcrossWrites(t *testing.T) {
	host := "discord.com"
	rec := buildBigClientHello(host)
	payload := rec[5:]

	f := &fakeConn{}
	c := Wrap(f, Strategy{Mode: ModeRecord, At: AtSNI, Off: 1})
	for _, ch := range [][]byte{rec[:2], rec[2:10], rec[10:50], rec[50:]} {
		if _, err := c.Write(ch); err != nil {
			t.Fatalf("write: %v", err)
		}
	}

	out := f.buf.Bytes()
	if countRecords(out) < 2 {
		t.Errorf("desync not applied across writes: %d records", countRecords(out))
	}
	if got := reassemble(t, out); !bytes.Equal(got, payload) {
		t.Errorf("reassembled payload differs")
	}
	if bytes.Contains(firstRecordPayload(out), []byte(host)) {
		t.Errorf("full SNI present in first record — DPI would still match it")
	}
}

func TestStrategyParseRoundTrip(t *testing.T) {
	for _, s := range Builtins() {
		got, err := ParseStrategy(s.String())
		if err != nil {
			t.Fatalf("%s: %v", s, err)
		}
		if got != s {
			t.Errorf("round trip: %s -> %+v", s, got)
		}
	}
}

func TestParseStrategyErrors(t *testing.T) {
	for _, in := range []string{"", "record", "bogus:sni", "record:bogus", "seg:fixed", "seg:fixed:x", "record:fixed:0"} {
		if _, err := ParseStrategy(in); err == nil {
			t.Errorf("%q: expected error", in)
		}
	}
}
