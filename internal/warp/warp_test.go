package warp

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestGenerateKey(t *testing.T) {
	k, err := GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	for name, s := range map[string]string{"private": k.Private, "public": k.Public} {
		raw, err := base64.StdEncoding.DecodeString(s)
		if err != nil {
			t.Errorf("%s key not valid base64: %v", name, err)
		}
		if len(raw) != 32 {
			t.Errorf("%s key is %d bytes, want 32", name, len(raw))
		}
	}
	if k.Private == k.Public {
		t.Error("private and public keys are identical")
	}
}

func TestKeysAreUnique(t *testing.T) {
	a, _ := GenerateKey()
	b, _ := GenerateKey()
	if a.Private == b.Private {
		t.Error("two generated keys collided")
	}
}

func TestWireGuardConf(t *testing.T) {
	acc := &Account{
		AddrV4:   "172.16.0.2",
		AddrV6:   "2606:4700:110::1",
		PeerPub:  "peerpubkey",
		Endpoint: "engage.cloudflareclient.com:2408",
		Key:      Key{Private: "privkey", Public: "pubkey"},
	}
	conf := acc.WireGuardConf("")
	for _, want := range []string{
		"[Interface]",
		"PrivateKey = privkey",
		"Address = 172.16.0.2/32, 2606:4700:110::1/128",
		"[Peer]",
		"PublicKey = peerpubkey",
		"AllowedIPs = 0.0.0.0/0, ::/0", // default full tunnel
		"Endpoint = engage.cloudflareclient.com:2408",
	} {
		if !strings.Contains(conf, want) {
			t.Errorf("conf missing %q\n---\n%s", want, conf)
		}
	}

	split := acc.WireGuardConf("162.159.0.0/16")
	if !strings.Contains(split, "AllowedIPs = 162.159.0.0/16") {
		t.Error("split-tunnel AllowedIPs not honored")
	}
}
