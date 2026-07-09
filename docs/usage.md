# unsni — Usage

`unsni` neutralizes SNI-based DPI censorship. It runs a local proxy that splits
the TLS ClientHello so a record-scoped DPI can't match the SNI, resolves names
over DNS-over-HTTPS to dodge DNS poisoning, and can auto-discover which split
works on your ISP. For UDP traffic a proxy can't carry (voice), it generates a
WireGuard/WARP config.

## Install / build

```bash
go install github.com/YusufDrymz/unsni/cmd/unsni@latest
# or from a checkout:
go build -o unsni ./cmd/unsni
```

## 1. Find a working strategy

```bash
unsni find discord.com
# baseline fails -> "Best strategy: record:sni"
```

`unsni doctor <host>` prints the full per-strategy breakdown and a verdict
(SNI-block confirmed / not blocked / IP-block).

## 2. Run the proxy

```bash
unsni run --strategy record:sni
```

Flags:

| Flag | Default | Meaning |
|------|---------|---------|
| `--listen` | `127.0.0.1:8080` | HTTP CONNECT proxy address (empty = off) |
| `--socks` | *(off)* | SOCKS5 proxy address, e.g. `127.0.0.1:1080` |
| `--strategy` | `record:sni` | default desync strategy |
| `--rules` | *(none)* | per-domain rules file (see below) |
| `--auto` | `false` | auto-discover the strategy per host on a rules miss |
| `--doh` | Cloudflare | DNS-over-HTTPS endpoint (empty = system resolver) |
| `--metrics` | `127.0.0.1:9090` | Prometheus `/metrics` (empty = off) |
| `--debug` | `false` | log per-connection SNI + split detail |

## 3. Point your traffic at it

### Easiest: let unsni manage the system proxy
`--system-proxy` sets the OS system proxy on start and **reverts it automatically
when you press Ctrl+C** (macOS and Windows). One command, no footgun:

```bash
unsni run --system-proxy
# -> "System proxy is ON. Press Ctrl+C to stop and restore your settings."
# use Discord / any browser normally; Ctrl+C when done.
```

Everything — browsers, the Discord desktop app, even its updater — goes through
unsni while it runs. (On macOS this may prompt for your admin password.)

### Manual (equivalent, if you prefer)
```bash
networksetup -setsecurewebproxy "Wi-Fi" 127.0.0.1 8080
networksetup -setwebproxy "Wi-Fi" 127.0.0.1 8080
# ... when done (IMPORTANT: if unsni stops while this is on, HTTPS breaks):
networksetup -setsecurewebproxystate "Wi-Fi" off
networksetup -setwebproxystate "Wi-Fi" off
```

### Chrome only (fresh instance so the flag actually applies)
```bash
open -na "Google Chrome" --args \
  --user-data-dir="/tmp/unsni-chrome" \
  --proxy-server="http://127.0.0.1:8080" \
  --disable-quic
```

## Rules file

One rule per line: `<host> <strategy|bypass>`. A rule also covers the host's
subdomains. Blank lines and `#` comments are ignored.

```
discord.com        record:sni
*.discordapp.com   record:sni
example.com        bypass
```

Lookup order per connection: rules file → `--auto` discovery → `--strategy` default.

## Strategies

`mode:at[:off]` — `mode` = `record` (RFC-compliant TLS record fragmentation) or
`seg` (TCP segment split); `at` = `sni` (split inside the SNI) or `fixed:<n>`.
See `unsni strategies` for the built-ins.

## Voice / UDP + desktop apps (full tunnel)

A proxy cannot carry Discord **voice** (direct UDP) or fix the **desktop app**
(its updater ignores proxies). Both need a network-layer tunnel.

**Easiest — built-in tunnel (no WireGuard install needed):**

```bash
sudo unsni tunnel     # embeds wireguard-go, connects to WARP, routes everything
# open Discord (desktop or browser); voice works. Ctrl+C / close window to stop.
```

It asks for your password (routing needs admin) and restores routes/DNS on exit.
The default route is never deleted, so even a crash self-heals connectivity.

**Alternative — generate a config and run it with WireGuard yourself:**

```bash
unsni warp --out warp.conf
wg-quick up ./warp.conf     # needs WireGuard installed (brew install wireguard-tools)
# ... voice works while the tunnel is up ...
wg-quick down ./warp.conf
```

`--allowed-ips` narrows the tunnel for split-tunnel (default is a full tunnel,
which is the simplest way to guarantee voice is carried).

## Observability

`http://127.0.0.1:9090/metrics` exposes connection counters. `--debug` logs, per
connection, whether the SNI was found and how the ClientHello was split.
