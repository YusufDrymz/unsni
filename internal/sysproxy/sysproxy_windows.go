//go:build windows

// Package sysproxy sets and reverts the OS-level HTTP(S) proxy, so an end user
// gets one command instead of editing network settings by hand.
package sysproxy

import (
	"fmt"
	"os/exec"
	"syscall"
)

const regPath = `HKCU\Software\Microsoft\Windows\CurrentVersion\Internet Settings`

// Set enables the WinINET system proxy (used by Edge/Chrome/most apps) pointing
// at addr, and returns a function that disables it again.
func Set(addr string) (func() error, error) {
	if err := regAdd("ProxyServer", "REG_SZ", addr); err != nil {
		return nil, err
	}
	if err := regAdd("ProxyEnable", "REG_DWORD", "1"); err != nil {
		return nil, err
	}
	refresh()

	revert := func() error {
		err := regAdd("ProxyEnable", "REG_DWORD", "0")
		refresh()
		return err
	}
	return revert, nil
}

func regAdd(name, typ, data string) error {
	out, err := exec.Command("reg", "add", regPath, "/v", name, "/t", typ, "/d", data, "/f").CombinedOutput()
	if err != nil {
		return fmt.Errorf("reg add %s: %v: %s", name, err, out)
	}
	return nil
}

// refresh notifies WinINET that the proxy settings changed so running apps pick
// them up without a restart.
func refresh() {
	const (
		optSettingsChanged = 39
		optRefresh         = 37
	)
	proc := syscall.NewLazyDLL("wininet.dll").NewProc("InternetSetOptionW")
	_, _, _ = proc.Call(0, uintptr(optSettingsChanged), 0, 0)
	_, _, _ = proc.Call(0, uintptr(optRefresh), 0, 0)
}
