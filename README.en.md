# unsni

[Türkçe](README.md) · **English**

> A small tool that lets you reach blocked sites and **get Discord working again**.
> No install, no config — download, double-click, use.

[![CI](https://github.com/YusufDrymz/unsni/actions/workflows/ci.yml/badge.svg)](https://github.com/YusufDrymz/unsni/actions)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

## What does it do?

If your internet provider (ISP) blocks some sites and Discord, `unsni` gets past
that block. It quietly "cleans up" your traffic so blocked sites open again. You
don't need to sign up, pay, or configure anything like a VPN.

**Works for:** blocked websites, **Discord login + chat**. Verified against real
Turkish DPI censorship.

---

## Install (no terminal needed)

### 1. Download the file

From the [**Releases page**](https://github.com/YusufDrymz/unsni/releases/latest),
grab the one for your system:

| Your system | File to download |
|-------------|------------------|
| **Windows** | `unsni_..._windows_amd64.zip` |
| **Mac (M1/M2/M3 — newer Macs)** | `unsni_..._darwin_arm64.tar.gz` |
| **Mac (Intel — older Macs)** | `unsni_..._darwin_amd64.tar.gz` |

> Not sure which Mac you have?  menu → "About This Mac". "Apple" chip = arm64,
> "Intel" = amd64.

### 2. Unzip it

Double-click the downloaded file to extract a folder. Inside you'll find
`start-windows.bat` / `start-macos.command`, `unsni` (the program itself), and a
few help files.

### 3. Double-click the right file

| What you want | File to double-click | Note |
|---------------|----------------------|------|
| Blocked sites + **Discord in your browser** | **Windows:** `start-windows.bat`<br>**Mac:** `start-macos.command` | No admin/password |
| **Discord desktop app + voice chat** | **Mac:** `start-macos-full.command` | Asks for your Mac password |

A terminal window opens ("routing your traffic"). **Keep this window open.** Now
open your browser or Discord and use it normally.

**When you're done, close the window** — all settings revert automatically.

> ℹ️ Don't double-click `unsni.exe` / `unsni` itself — that's just the program,
> it prints help and exits. Use the launcher file above.

---

## Warnings you might see on first run (all normal)

- **Windows — "Windows protected your PC" (blue screen):**
  Appears because the app isn't code-signed; it's not dangerous.
  → **"More info"** → **"Run anyway"**.
- **Windows — Firewall asks "allow access?":**
  → **"Allow access"**. (If you don't, sites won't open.)
- **Mac — "cannot verify developer":**
  Right-click the launcher → **Open** (just the first time).

---

## If something isn't working

- **Sites won't open / internet seems broken:**
  Close the window, **wait 10 seconds**, and double-click the launcher again.
- **Missed the Windows Firewall prompt:**
  Close the window, start again, and this time click **"Allow access"**.
- **Still stuck:** In Task Manager, kill any leftover `unsni.exe` running in the
  background, then start again.

---

## Discord desktop app and voice

The light mode above (`start-windows.bat` / `start-macos.command`) works for the
**browser**: sites + Discord in the browser (login + chat).

For the **Discord desktop app and voice chat** you need full tunnel mode. Right
now that mode is **Mac-only**: `start-macos-full.command` (asks for your Mac
password, routes all traffic through a secure tunnel). A Windows full-tunnel mode
doesn't exist yet — on Windows, use Discord **in your browser**.

---

<details>
<summary><b>Advanced usage (command line / CLI)</b></summary>

For technical users. Pure Go, single binary, **no cgo**, cross-platform.

### Install

```bash
go install github.com/YusufDrymz/unsni/cmd/unsni@latest
```

### Core commands

```bash
# 1. Find the fastest working strategy for a blocked host
unsni find discord.com
# -> best: record:sni (handshake in 84ms)

# 2. Run the local proxy (HTTP CONNECT + optional SOCKS5, per-domain rules, auto)
unsni run --strategy record:sni --socks 127.0.0.1:1080 --rules rules.txt --auto

# Easiest: sets the system proxy on start, reverts on exit
unsni run --system-proxy
```

### Diagnose a block

```bash
unsni doctor discord.com
# baseline (no desync): FAILED at TLS handshake  -> SNI-based block confirmed
# record:sni          : OK (84ms)
# seg:sni             : OK (91ms)
```

### Strategies

`mode:at[:off]` — `mode` is `record` (RFC-compliant TLS record fragmentation) or
`seg` (TCP segment split); `at` is `sni` (split inside the SNI hostname) or
`fixed:<n>` (fixed payload offset).

```bash
unsni strategies   # list the built-ins
```

### Discord voice (UDP)

A proxy can't carry UDP. For voice, generate a WireGuard/WARP tunnel and run it
alongside the proxy (proxy desyncs TCP, tunnel carries UDP):

```bash
unsni warp --out warp.conf && wg-quick up ./warp.conf
```

Full guide: [`docs/usage.md`](docs/usage.md) · Türkçe: [`docs/usage.tr.md`](docs/usage.tr.md)

### Why it's different from other tools

Most DPI-bypass tools (SpoofDPI, byedpi, zapret) either hard-code a single desync
trick or make you run a cryptic `blockcheck` and copy-paste parameters by hand —
with zero insight into *why* a site is blocked. `unsni` probes the strategy space
against your ISP automatically, finds the fastest working one, and gives you real
observability (`/metrics`).

### Development

```bash
make test    # go test -race ./...
make vet
make cross   # cross-compile smoke check (linux/windows/darwin, cgo off)
```

</details>

## License

MIT — see [LICENSE](LICENSE).
