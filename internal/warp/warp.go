// Package warp registers an anonymous Cloudflare WARP account and renders a
// WireGuard config. unsni's userspace proxy handles TCP/TLS; WARP covers the
// UDP traffic a proxy cannot carry (e.g. Discord voice). unsni only generates
// the config — the tunnel itself is run by WireGuard (wg-quick / the WireGuard app).
package warp

import (
	"bytes"
	"context"
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// The WARP registration endpoint mirrors what community tools (wgcf) use. The
// API version occasionally changes; if Register starts returning non-200, bump
// these to the current values.
const (
	apiBase   = "https://api.cloudflareclient.com/v0a2158"
	userAgent = "okhttp/3.12.1"
	clientVer = "a-6.30-2158"
)

// Key is a WireGuard (Curve25519) keypair, base64-encoded.
type Key struct {
	Private string
	Public  string
}

// GenerateKey creates a WireGuard-compatible Curve25519 keypair using the
// standard library (no cgo, no external crypto dependency).
func GenerateKey() (Key, error) {
	priv, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return Key{}, err
	}
	return Key{
		Private: base64.StdEncoding.EncodeToString(priv.Bytes()),
		Public:  base64.StdEncoding.EncodeToString(priv.PublicKey().Bytes()),
	}, nil
}

// Account is the subset of a WARP registration we need to build a config.
type Account struct {
	AddrV4   string
	AddrV6   string
	PeerPub  string
	Endpoint string
	Key      Key
}

// Register creates an anonymous WARP account bound to a fresh keypair.
func Register(ctx context.Context) (*Account, error) {
	key, err := GenerateKey()
	if err != nil {
		return nil, err
	}

	body, _ := json.Marshal(map[string]any{
		"install_id": "",
		"fcm_token":  "",
		"tos":        time.Now().UTC().Format("2006-01-02T15:04:05.000-07:00"),
		"key":        key.Public,
		"type":       "Android",
		"model":      "PC",
		"locale":     "en_US",
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiBase+"/reg", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("CF-Client-Version", clientVer)

	resp, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("warp register: status %d (the API version may need updating)", resp.StatusCode)
	}

	var out struct {
		Config struct {
			Peers []struct {
				PublicKey string `json:"public_key"`
				Endpoint  struct {
					Host string `json:"host"`
				} `json:"endpoint"`
			} `json:"peers"`
			Interface struct {
				Addresses struct {
					V4 string `json:"v4"`
					V6 string `json:"v6"`
				} `json:"addresses"`
			} `json:"interface"`
		} `json:"config"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	if len(out.Config.Peers) == 0 {
		return nil, fmt.Errorf("warp register: response had no peers")
	}

	return &Account{
		AddrV4:   out.Config.Interface.Addresses.V4,
		AddrV6:   out.Config.Interface.Addresses.V6,
		PeerPub:  out.Config.Peers[0].PublicKey,
		Endpoint: out.Config.Peers[0].Endpoint.Host,
		Key:      key,
	}, nil
}

// WireGuardConf renders a WireGuard config. allowedIPs controls the tunnel
// scope: "0.0.0.0/0, ::/0" for a full tunnel (definitely carries Discord voice),
// or a narrower list for split-tunnel.
func (a *Account) WireGuardConf(allowedIPs string) string {
	if allowedIPs == "" {
		allowedIPs = "0.0.0.0/0, ::/0"
	}
	endpoint := a.Endpoint
	if endpoint == "" {
		endpoint = "engage.cloudflareclient.com:2408"
	}

	var b strings.Builder
	b.WriteString("[Interface]\n")
	fmt.Fprintf(&b, "PrivateKey = %s\n", a.Key.Private)
	fmt.Fprintf(&b, "Address = %s/32, %s/128\n", a.AddrV4, a.AddrV6)
	b.WriteString("DNS = 1.1.1.1\n")
	b.WriteString("MTU = 1280\n\n")
	b.WriteString("[Peer]\n")
	fmt.Fprintf(&b, "PublicKey = %s\n", a.PeerPub)
	fmt.Fprintf(&b, "AllowedIPs = %s\n", allowedIPs)
	fmt.Fprintf(&b, "Endpoint = %s\n", endpoint)
	return b.String()
}
