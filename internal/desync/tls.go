package desync

import "errors"

var (
	errNotRecord = errors.New("desync: not a TLS handshake record")
	errBadOffset = errors.New("desync: split offset out of range")
)

// fragmentRecord splits a single TLS record into two records at payload offset
// `at` (0 < at < payloadLen). Both records carry the original content type and
// version; their lengths are fixed up. Any bytes after the first record in buf
// are appended unchanged. The result reassembles, byte-for-byte, to the same
// handshake payload the server would otherwise have received.
func fragmentRecord(buf []byte, at int) ([]byte, error) {
	if len(buf) < 6 || buf[0] != 0x16 {
		return nil, errNotRecord
	}
	recLen := int(buf[3])<<8 | int(buf[4])
	total := 5 + recLen
	if total > len(buf) {
		return nil, errNotRecord
	}
	payload := buf[5:total]
	rest := buf[total:] // trailing bytes beyond this record, if any
	if at <= 0 || at >= len(payload) {
		return nil, errBadOffset
	}

	ct, v0, v1 := buf[0], buf[1], buf[2]
	p1, p2 := payload[:at], payload[at:]

	out := make([]byte, 0, len(buf)+5)
	out = append(out, ct, v0, v1, byte(len(p1)>>8), byte(len(p1)))
	out = append(out, p1...)
	out = append(out, ct, v0, v1, byte(len(p2)>>8), byte(len(p2)))
	out = append(out, p2...)
	out = append(out, rest...)
	return out, nil
}
