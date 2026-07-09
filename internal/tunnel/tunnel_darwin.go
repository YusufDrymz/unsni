//go:build darwin

// Package tunnel brings up an in-process WireGuard (WARP) full tunnel so traffic
// from apps that ignore proxies (Discord's desktop updater, voice/UDP) also gets
// past DPI. It embeds wireguard-go, so no external WireGuard/wg-quick is needed.
// It requires root (creating a utun device + editing routes).
//
// Safety: the default route is never deleted. We override it with two /1 routes
// bound to the tunnel interface, so if this process dies the utun disappears, its
// interface routes vanish with it, and normal connectivity returns on its own.
package tunnel

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net"
	"os/exec"
	"strings"

	"github.com/YusufDrymz/unsni/internal/warp"
	"golang.zx2c4.com/wireguard/conn"
	"golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/tun"
)

const mtu = 1280

// Up creates a utun device, drives WireGuard to WARP with wireguard-go, and points
// the default route + DNS at it. The returned function reverses everything.
func Up(acc *warp.Account, logf func(string)) (func() error, error) {
	privHex, err := keyToHex(acc.Key.Private)
	if err != nil {
		return nil, fmt.Errorf("private key: %w", err)
	}
	peerHex, err := keyToHex(acc.PeerPub)
	if err != nil {
		return nil, fmt.Errorf("peer key: %w", err)
	}
	endpoint, err := net.ResolveUDPAddr("udp", acc.Endpoint)
	if err != nil {
		return nil, fmt.Errorf("resolve endpoint %q: %w", acc.Endpoint, err)
	}

	gw, iface, err := defaultRoute()
	if err != nil {
		return nil, err
	}

	tunDev, err := tun.CreateTUN("utun", mtu)
	if err != nil {
		return nil, fmt.Errorf("create utun (run with sudo): %w", err)
	}
	name, _ := tunDev.Name()

	wg := device.NewDevice(tunDev, conn.NewDefaultBind(), device.NewLogger(device.LogLevelError, "unsni-wg: "))
	uapi := fmt.Sprintf(
		"private_key=%s\npublic_key=%s\nendpoint=%s\npersistent_keepalive_interval=25\nallowed_ip=0.0.0.0/0\nallowed_ip=::/0\n",
		privHex, peerHex, endpoint.String(),
	)
	if err := wg.IpcSet(uapi); err != nil {
		wg.Close()
		return nil, fmt.Errorf("configure wireguard: %w", err)
	}
	if err := wg.Up(); err != nil {
		wg.Close()
		return nil, fmt.Errorf("bring wireguard up: %w", err)
	}

	var undo []func()
	cleanup := func() error {
		for i := len(undo) - 1; i >= 0; i-- {
			undo[i]()
		}
		wg.Close() // removes the utun; its interface routes go with it
		return nil
	}
	fail := func(e error) (func() error, error) {
		cleanup()
		return nil, e
	}

	// Interface addresses.
	if err := run("ifconfig", name, "inet", acc.AddrV4, acc.AddrV4, "alias"); err != nil {
		return fail(err)
	}
	if acc.AddrV6 != "" {
		_ = run("ifconfig", name, "inet6", acc.AddrV6, "prefixlen", "128", "alias")
	}
	if err := run("ifconfig", name, "up"); err != nil {
		return fail(err)
	}

	// Keep WARP's own UDP off the tunnel: reach the endpoint via the real gateway.
	epNet := endpoint.IP.String() + "/32"
	if err := run("route", "-q", "-n", "add", "-inet", epNet, gw); err != nil {
		return fail(err)
	}
	undo = append(undo, func() { _ = run("route", "-q", "-n", "delete", "-inet", epNet, gw) })

	// Default route through the tunnel via /1 splits (never touches the real default).
	for _, cidr := range []string{"0.0.0.0/1", "128.0.0.0/1"} {
		if err := run("route", "-q", "-n", "add", "-inet", cidr, "-interface", name); err != nil {
			return fail(err)
		}
		c := cidr
		undo = append(undo, func() { _ = run("route", "-q", "-n", "delete", "-inet", c, "-interface", name) })
	}
	for _, cidr := range []string{"::/1", "8000::/1"} { // IPv6, best-effort
		if run("route", "-q", "-n", "add", "-inet6", cidr, "-interface", name) == nil {
			c := cidr
			undo = append(undo, func() { _ = run("route", "-q", "-n", "delete", "-inet6", c, "-interface", name) })
		}
	}

	// DNS through WARP.
	if svc, err := serviceForIface(iface); err == nil {
		if run("networksetup", "-setdnsservers", svc, "1.1.1.1", "1.0.0.1") == nil {
			undo = append(undo, func() { _ = run("networksetup", "-setdnsservers", svc, "Empty") })
		}
	}

	logf(fmt.Sprintf("tunnel up: %s → WARP %s (default route + DNS switched)", name, endpoint.IP))
	return cleanup, nil
}

func keyToHex(b64 string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return "", err
	}
	if len(raw) != 32 {
		return "", fmt.Errorf("expected 32-byte key, got %d", len(raw))
	}
	return hex.EncodeToString(raw), nil
}

func defaultRoute() (gateway, iface string, err error) {
	out, err := exec.Command("route", "-n", "get", "default").Output()
	if err != nil {
		return "", "", fmt.Errorf("route get default: %w", err)
	}
	for _, line := range strings.Split(string(out), "\n") {
		f := strings.Fields(line)
		if len(f) < 2 {
			continue
		}
		switch f[0] {
		case "gateway:":
			gateway = f[1]
		case "interface:":
			iface = f[1]
		}
	}
	if gateway == "" || iface == "" {
		return "", "", fmt.Errorf("could not determine default route")
	}
	return gateway, iface, nil
}

func serviceForIface(iface string) (string, error) {
	out, err := exec.Command("networksetup", "-listnetworkserviceorder").Output()
	if err != nil {
		return "", err
	}
	var name string
	for _, line := range strings.Split(string(out), "\n") {
		l := strings.TrimSpace(line)
		if strings.HasPrefix(l, "(") && !strings.Contains(l, "Hardware Port") {
			if i := strings.Index(l, ") "); i != -1 {
				name = strings.TrimSpace(l[i+2:])
			}
		}
		if strings.Contains(l, "Device: "+iface+")") && name != "" {
			return name, nil
		}
	}
	return "", fmt.Errorf("no network service for %s", iface)
}

func run(name string, args ...string) error {
	out, err := exec.Command(name, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s: %v: %s", name, strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}
