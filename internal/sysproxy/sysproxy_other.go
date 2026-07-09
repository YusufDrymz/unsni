//go:build !darwin && !windows

// Package sysproxy sets and reverts the OS-level HTTP(S) proxy.
package sysproxy

import "fmt"

// Set is unsupported on this OS; configure the proxy manually.
func Set(addr string) (func() error, error) {
	return nil, fmt.Errorf("--system-proxy is not supported on this OS; set the proxy manually (see docs/usage.md)")
}
