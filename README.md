# unsni

**English** · [Türkçe](README.tr.md)

> Cross-platform DPI bypass engine in Go — with an **automatic strategy finder**
> and **real observability**. Neutralize SNI-based censorship (blocked sites,
> Discord login/chat) without editing kernel rules by hand.

[![CI](https://github.com/YusufDrymz/unsni/actions/workflows/ci.yml/badge.svg)](https://github.com/YusufDrymz/unsni/actions)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

## Why

Most DPI-bypass tools (SpoofDPI, byedpi, zapret) either hard-code a single
desync trick or make you run a cryptic `blockcheck` and copy-paste parameters
by hand — with zero insight into *why* a site is blocked or *which* trick worked.

`unsni` fills that gap:

- **`unsni find <host>`** probes the strategy space against your ISP and prints
  the fastest working desync strategy — zero config.
- **`unsni doctor <host>`** tells you *why* a connection fails (SNI-based block?
  DNS poisoning?) and which strategy fixes it.
- **`unsni run`** starts a local proxy that applies the strategy, with a
  Prometheus `/metrics` endpoint for real monitoring.

Pure Go, single binary, **no cgo**, cross-platform.

## Install

```bash
go install github.com/YusufDrymz/unsni/cmd/unsni@latest
```

## Easy install (no terminal)

1. Download the file for your system from
   [Releases](https://github.com/YusufDrymz/unsni/releases/latest):
   `unsni_..._darwin_arm64.tar.gz` (Apple Silicon Mac), `..._darwin_amd64` (Intel
   Mac), or `..._windows_amd64.zip` (Windows).
2. Unzip it. Inside you'll see:

   | file | what it is |
   |------|------------|
   | **`start-macos.command`** / **`start-windows.bat`** | **← double-click THIS to run** |
   | `unsni` / `unsni.exe` | the program itself (don't double-click — it just prints help) |
   | `README.md`, `docs/`, `LICENSE` | docs |

3. Double-click the launcher for your OS. It turns the system proxy on; open
   Discord / your browser and use it normally. **Close the window when done** and
   your settings revert automatically.

> macOS first run: if it says "cannot verify developer", **right-click the
> launcher → Open** once. It runs with no admin needed.

## Quick start (CLI)

```bash
# 1. Find a working strategy for a blocked host
unsni find discord.com
# -> best: record:sni (handshake in 84ms)

# 2. Run the local proxy (HTTP CONNECT + optional SOCKS5, per-domain rules, auto-discovery)
unsni run --strategy record:sni --socks 127.0.0.1:1080 --rules rules.txt --auto

# 3. Point your traffic at it. On macOS the system proxy is most reliable:
#    networksetup -setsecurewebproxy "Wi-Fi" 127.0.0.1 8080
#    networksetup -setwebproxy       "Wi-Fi" 127.0.0.1 8080
#    (revert with ...state off when done — see docs/usage.md)
```

For **Discord voice (UDP)**, which no proxy can carry, generate a WireGuard/WARP tunnel:

```bash
unsni warp --out warp.conf && wg-quick up ./warp.conf
```

Full guide: [`docs/usage.md`](docs/usage.md) · Türkçe: [`docs/usage.tr.md`](docs/usage.tr.md)

Diagnose a block:

```bash
unsni doctor discord.com
# baseline (no desync): FAILED at TLS handshake  -> SNI-based block confirmed
# record:sni          : OK (84ms)
# seg:sni             : OK (91ms)
# seg:fixed:1         : FAILED
```

## Strategies

`mode:at[:off]` — `mode` is `record` (RFC-compliant TLS record fragmentation) or
`seg` (TCP segment split); `at` is `sni` (split inside the SNI hostname) or
`fixed:<n>` (fixed payload offset).

```bash
unsni strategies   # list the built-ins
```

## Scope (read this)

The **proxy** handles **HTTPS/TLS (TCP)** — website access and Discord
**login + chat + gateway**. This is verified against real Turkish DPI.

**Discord voice is UDP**, which a proxy cannot carry. For voice, `unsni warp`
generates a WireGuard/WARP tunnel you run alongside the proxy — the proxy desyncs
TCP, the tunnel carries UDP. A built-in transparent-capture tunnel (no external
WireGuard) is future work. No false promises: voice needs the WARP tunnel running.

## Development

```bash
make test    # go test -race ./...
make vet
make cross   # cross-compile smoke check (linux/windows/darwin, cgo off)
```

## License

MIT — see [LICENSE](LICENSE).
