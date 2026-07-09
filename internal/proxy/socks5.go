package proxy

import (
	"bufio"
	"context"
	"io"
	"net"
	"strconv"
)

// SOCKS5 constants (RFC 1928).
const (
	socksVersion  = 0x05
	socksNoAuth   = 0x00
	socksConnect  = 0x01
	atypIPv4      = 0x01
	atypDomain    = 0x03
	atypIPv6      = 0x04
	repSuccess    = 0x00
	repGeneral    = 0x01
	repCmdNotSup  = 0x07
	repAtypNotSup = 0x08
)

// handleSocks implements the SOCKS5 CONNECT command (no authentication) and then
// hands off to the shared tunnel.
func (s *Server) handleSocks(ctx context.Context, client net.Conn) {
	defer client.Close()
	br := bufio.NewReader(client)

	// Greeting: version, nmethods, methods[nmethods].
	ver, err := br.ReadByte()
	if err != nil || ver != socksVersion {
		return
	}
	nMethods, err := br.ReadByte()
	if err != nil {
		return
	}
	if _, err := io.CopyN(io.Discard, br, int64(nMethods)); err != nil {
		return
	}
	// Choose no-auth.
	if _, err := client.Write([]byte{socksVersion, socksNoAuth}); err != nil {
		return
	}

	// Request: version, cmd, rsv, atyp, dst.addr, dst.port.
	hdr := make([]byte, 4)
	if _, err := io.ReadFull(br, hdr); err != nil || hdr[0] != socksVersion {
		return
	}
	if hdr[1] != socksConnect {
		s.socksReply(client, repCmdNotSup)
		return
	}

	var host string
	switch hdr[3] {
	case atypIPv4:
		b := make([]byte, 4)
		if _, err := io.ReadFull(br, b); err != nil {
			return
		}
		host = net.IP(b).String()
	case atypIPv6:
		b := make([]byte, 16)
		if _, err := io.ReadFull(br, b); err != nil {
			return
		}
		host = net.IP(b).String()
	case atypDomain:
		l, err := br.ReadByte()
		if err != nil {
			return
		}
		b := make([]byte, l)
		if _, err := io.ReadFull(br, b); err != nil {
			return
		}
		host = string(b)
	default:
		s.socksReply(client, repAtypNotSup)
		return
	}

	pb := make([]byte, 2)
	if _, err := io.ReadFull(br, pb); err != nil {
		return
	}
	port := strconv.Itoa(int(pb[0])<<8 | int(pb[1]))

	up, strat, on, err := s.open(ctx, host, port)
	if err != nil {
		s.socksReply(client, repGeneral)
		s.Logger.Warn("dial failed", "host", host, "err", err)
		return
	}
	if err := s.socksReply(client, repSuccess); err != nil {
		_ = up.Close()
		return
	}
	s.pipe(client, br, up, strat, on, host, port)
}

// socksReply sends a SOCKS5 reply with a zero BND.ADDR/BND.PORT.
func (s *Server) socksReply(client net.Conn, rep byte) error {
	_, err := client.Write([]byte{socksVersion, rep, 0x00, atypIPv4, 0, 0, 0, 0, 0, 0})
	return err
}
