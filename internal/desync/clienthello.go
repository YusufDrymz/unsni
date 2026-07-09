package desync

import "errors"

// errNoSNI means the buffer is not a ClientHello or carries no SNI extension.
var errNoSNI = errors.New("desync: SNI not found in ClientHello")

// sniRange locates the SNI hostname inside a TLS record that begins with a
// ClientHello handshake message. It returns absolute byte offsets [start,end)
// into buf. Every read is bounds-checked; malformed input yields errNoSNI
// rather than a panic (buf comes straight off an untrusted client socket).
func sniRange(buf []byte) (start, end int, err error) {
	// TLS record header: content_type(1) legacy_version(2) length(2).
	if len(buf) < 5 || buf[0] != 0x16 { // 0x16 = handshake
		return 0, 0, errNoSNI
	}

	// Work on a view starting at the handshake message; remember the base
	// offset so we can translate back to absolute positions in buf.
	const base = 5 + 4 // record header + handshake header
	b := buf[5:]

	// Handshake header: msg_type(1) length(3).
	if len(b) < 4 || b[0] != 0x01 { // 0x01 = client_hello
		return 0, 0, errNoSNI
	}
	hsLen := int(b[1])<<16 | int(b[2])<<8 | int(b[3])
	b = b[4:]
	if hsLen < len(b) {
		b = b[:hsLen] // ignore anything past this handshake message
	}

	// ClientHello body: legacy_version(2) random(32).
	if len(b) < 34 {
		return 0, 0, errNoSNI
	}
	p := 34

	// session_id: len(1) + bytes.
	if p >= len(b) {
		return 0, 0, errNoSNI
	}
	p += 1 + int(b[p])

	// cipher_suites: len(2) + bytes.
	if p+2 > len(b) {
		return 0, 0, errNoSNI
	}
	p += 2 + (int(b[p])<<8 | int(b[p+1]))

	// compression_methods: len(1) + bytes.
	if p+1 > len(b) {
		return 0, 0, errNoSNI
	}
	p += 1 + int(b[p])

	// extensions: len(2) + entries.
	if p+2 > len(b) {
		return 0, 0, errNoSNI
	}
	extEnd := p + 2 + (int(b[p])<<8 | int(b[p+1]))
	p += 2
	if extEnd > len(b) {
		extEnd = len(b)
	}

	for p+4 <= extEnd {
		etype := int(b[p])<<8 | int(b[p+1])
		elen := int(b[p+2])<<8 | int(b[p+3])
		p += 4
		if p+elen > extEnd {
			break
		}
		if etype != 0x0000 { // server_name
			p += elen
			continue
		}
		// server_name extension data:
		//   server_name_list: list_len(2) then entries
		//   entry: name_type(1) name_len(2) name
		e := b[p : p+elen]
		if len(e) < 5 { // 2 + 1 + 2
			return 0, 0, errNoSNI
		}
		nameLen := int(e[3])<<8 | int(e[4])
		nameStart := 5
		if nameStart+nameLen > len(e) {
			return 0, 0, errNoSNI
		}
		abs := base + p + nameStart
		return abs, abs + nameLen, nil
	}
	return 0, 0, errNoSNI
}
