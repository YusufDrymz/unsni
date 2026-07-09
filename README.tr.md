# unsni

[English](README.md) · **Türkçe**

> Go ile yazılmış cross-platform DPI bypass motoru. **Otomatik strateji bulucu**
> ve **gerçek observability** ile. SNI tabanlı sansürü (engelli siteler, Discord
> giriş/yazışma) elle kernel kuralı yazmadan aşar.

[![CI](https://github.com/YusufDrymz/unsni/actions/workflows/ci.yml/badge.svg)](https://github.com/YusufDrymz/unsni/actions)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

## Neden

Çoğu DPI bypass aracı (SpoofDPI, byedpi, zapret) ya tek bir desync numarasını
sabit kullanır ya da sana kriptik bir `blockcheck` çalıştırıp parametreleri elle
kopyalatır. Bir sitenin **neden** engelli olduğuna veya **hangi** numaranın
işe yaradığına dair sıfır bilgi verir.

`unsni` tam bu boşluğu doldurur:

- **`unsni find <host>`** ISP'ne karşı strateji uzayını prob eder ve çalışan en
  hızlı desync stratejisini basar. Sıfır konfig.
- **`unsni doctor <host>`** bir bağlantının **neden** koptuğunu söyler (SNI-block mu?
  DNS poisoning mi?) ve hangi stratejinin çözdüğünü gösterir.
- **`unsni run`** stratejiyi uygulayan yerel bir proxy başlatır; Prometheus
  `/metrics` ucuyla gerçek izleme.

Saf Go, tek binary, **cgo yok**, cross-platform.

## Kolay kurulum (terminal yok)

1. [Releases](https://github.com/YusufDrymz/unsni/releases/latest) sayfasından
   sistemine uygun dosyayı indir: `unsni_..._darwin_arm64.tar.gz` (Apple Silicon
   Mac), `..._darwin_amd64` (Intel Mac), veya `..._windows_amd64.zip` (Windows).
2. Çıkar. İçinde şunları görürsün:

   | dosya | ne |
   |-------|-----|
   | **`start-macos.command`** / **`start-windows.bat`** | **← çalıştırmak için BUNA çift tıkla** |
   | `unsni` / `unsni.exe` | programın kendisi (çift tıklama; sadece yardım basar) |
   | `README.md`, `docs/`, `LICENSE` | belgeler |

3. Sistemine uygun launcher'a çift tıkla. Sistem proxy'sini açar; Discord'u /
   tarayıcını normal kullan. **İşin bitince pencereyi kapat**, ayarların otomatik
   geri döner.

> macOS ilk açılış: "geliştirici doğrulanamadı" derse launcher'a **sağ tık → Aç**
> de (bir kez). Admin gerekmez.

## Hızlı başlangıç (CLI)

```bash
# 1. Engelli bir host için çalışan stratejiyi bul
unsni find discord.com
# -> best: record:sni (handshake 84ms)

# 2. Yerel proxy'yi çalıştır (HTTP CONNECT + opsiyonel SOCKS5, per-domain rules, auto)
unsni run --strategy record:sni --socks 127.0.0.1:1080 --rules rules.txt --auto

# 3. Trafiği yönlendir. macOS'ta en güvenilir yol sistem proxy'si:
#    networksetup -setsecurewebproxy "Wi-Fi" 127.0.0.1 8080
#    networksetup -setwebproxy       "Wi-Fi" 127.0.0.1 8080
#    (bitince ...state off ile geri al — bkz. docs/usage.tr.md)
```

En kolayı `unsni run --system-proxy`: başlarken sistem proxy'sini kurar, Ctrl+C /
pencere kapanınca otomatik geri alır.

**Discord sesi (UDP)** için (hiçbir proxy taşıyamaz) WireGuard/WARP tüneli üret:

```bash
unsni warp --out warp.conf && wg-quick up ./warp.conf
```

Tam rehber: [`docs/usage.tr.md`](docs/usage.tr.md) · English: [`docs/usage.md`](docs/usage.md)

Engeli teşhis et:

```bash
unsni doctor discord.com
# baseline (no desync): FAILED at TLS handshake  -> SNI-based block confirmed
# record:sni          : OK (84ms)
# seg:sni             : OK (91ms)
# seg:fixed:1         : FAILED
```

## Stratejiler

`mode:at[:off]` — `mode`: `record` (RFC uyumlu TLS record fragmentation) veya
`seg` (TCP segment split); `at`: `sni` (SNI hostname içinde böl) veya
`fixed:<n>` (sabit payload offset).

```bash
unsni strategies   # yerleşikleri listele
```

## Kapsam (oku)

**Proxy**, **HTTPS/TLS (TCP)** trafiğini çözer — site erişimi ve Discord
**giriş + yazışma + gateway**. Bu, gerçek Türkiye DPI'ına karşı doğrulandı.

**Discord sesi UDP'dir** ve proxy bunu taşıyamaz. Ses için `unsni warp` bir
WireGuard/WARP tüneli üretir; proxy'yle birlikte çalıştırırsın (proxy TCP'yi
desync eder, tünel UDP'yi taşır). Gömülü transparent-capture tüneli (harici
WireGuard'sız) ileri iş. Yanlış vaat yok: ses için WARP tünelinin açık olması gerekir.

## Geliştirme

```bash
make test    # go test -race ./...
make vet
make cross   # cross-compile smoke check (linux/windows/darwin, cgo off)
```

## Lisans

MIT — bkz. [LICENSE](LICENSE).
