//go:build darwin

// Package sysproxy sets and reverts the OS-level HTTP(S) proxy, so an end user
// gets one command instead of editing network settings by hand.
package sysproxy

import (
	"fmt"
	"net"
	"os/exec"
	"strings"
)

// Set points the active network service's web + secure-web proxy at addr
// (host:port) and returns a function that turns them back off.
func Set(addr string) (func() error, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, fmt.Errorf("bad address %q: %w", addr, err)
	}
	if host == "" || host == "0.0.0.0" {
		host = "127.0.0.1"
	}

	svc, err := activeService()
	if err != nil {
		return nil, err
	}

	for _, kind := range []string{"-setwebproxy", "-setsecurewebproxy"} {
		if out, err := exec.Command("networksetup", kind, svc, host, port).CombinedOutput(); err != nil {
			return nil, fmt.Errorf("networksetup %s: %v: %s", kind, err, strings.TrimSpace(string(out)))
		}
	}

	revert := func() error {
		var firstErr error
		for _, kind := range []string{"-setwebproxystate", "-setsecurewebproxystate"} {
			if out, err := exec.Command("networksetup", kind, svc, "off").CombinedOutput(); err != nil && firstErr == nil {
				firstErr = fmt.Errorf("networksetup %s: %v: %s", kind, err, strings.TrimSpace(string(out)))
			}
		}
		return firstErr
	}
	return revert, nil
}

// activeService maps the default route's interface to a networksetup service name
// (e.g. en0 -> "Wi-Fi").
func activeService() (string, error) {
	out, err := exec.Command("route", "get", "default").Output()
	if err != nil {
		return "", fmt.Errorf("route get default: %w", err)
	}
	var iface string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "interface:") {
			iface = strings.TrimSpace(strings.TrimPrefix(line, "interface:"))
			break
		}
	}
	if iface == "" {
		return "", fmt.Errorf("could not determine the active network interface")
	}

	out, err = exec.Command("networksetup", "-listnetworkserviceorder").Output()
	if err != nil {
		return "", err
	}
	// Blocks look like:
	//   (1) Wi-Fi
	//   (Hardware Port: Wi-Fi, Device: en0)
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
	return "", fmt.Errorf("no network service found for interface %s", iface)
}
