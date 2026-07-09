# unsni — Kullanım

`unsni`, SNI tabanlı DPI sansürünü aşar. Yerel bir proxy çalıştırır; TLS
ClientHello'yu böler ki record bazlı DPI SNI'yı eşleştiremesin, isimleri
DNS-over-HTTPS ile çözer (DNS poisoning'i atlar) ve ISP'nde hangi bölmenin
çalıştığını otomatik bulabilir. Proxy'nin taşıyamadığı UDP (ses) için
WireGuard/WARP config üretir.

## Kurulum / derleme

```bash
go install github.com/YusufDrymz/unsni/cmd/unsni@latest
# ya da checkout'tan:
go build -o unsni ./cmd/unsni
```

## 1. Çalışan stratejiyi bul

```bash
unsni find discord.com
# baseline düşer -> "Best strategy: record:sni"
```

`unsni doctor <host>` her strateji için ayrıntı + karar (SNI-block / engelli değil
/ IP-block) basar.

## 2. Proxy'yi çalıştır

```bash
unsni run --strategy record:sni
```

Bayraklar:

| Bayrak | Varsayılan | Anlamı |
|--------|-----------|--------|
| `--listen` | `127.0.0.1:8080` | HTTP CONNECT proxy adresi (boş = kapalı) |
| `--socks` | *(kapalı)* | SOCKS5 proxy adresi, örn. `127.0.0.1:1080` |
| `--strategy` | `record:sni` | varsayılan desync stratejisi |
| `--rules` | *(yok)* | per-domain kural dosyası (aşağıda) |
| `--auto` | `false` | kural yoksa host için stratejiyi otomatik keşfet |
| `--doh` | Cloudflare | DNS-over-HTTPS ucu (boş = sistem resolver) |
| `--metrics` | `127.0.0.1:9090` | Prometheus `/metrics` (boş = kapalı) |
| `--debug` | `false` | bağlantı başına SNI + bölme detayı logla |

## 3. Trafiği proxy'e yönlendir

### En kolay: sistem proxy'sini unsni yönetsin
`--system-proxy` başlarken sistem proxy'sini kurar ve **Ctrl+C'ye basınca otomatik
geri alır** (macOS ve Windows). Tek komut, footgun yok:

```bash
unsni run --system-proxy
# -> "System proxy is ON. Press Ctrl+C to stop and restore your settings."
# Discord / herhangi bir tarayıcıyı normal kullan; bitince Ctrl+C.
```

Her şey (tarayıcılar, Discord masaüstü, updater dahil) unsni çalışırken ondan
geçer. (macOS'ta admin şifresi sorabilir.)

### Elle (eşdeğer, istersen)
```bash
networksetup -setsecurewebproxy "Wi-Fi" 127.0.0.1 8080
networksetup -setwebproxy "Wi-Fi" 127.0.0.1 8080
# ... bitince (ÖNEMLİ: proxy açıkken unsni durursa tüm HTTPS kırılır):
networksetup -setsecurewebproxystate "Wi-Fi" off
networksetup -setwebproxystate "Wi-Fi" off
```

### Sadece Chrome (bayrağın uygulanması için TEMİZ instance)
```bash
open -na "Google Chrome" --args \
  --user-data-dir="/tmp/unsni-chrome" \
  --proxy-server="http://127.0.0.1:8080" \
  --disable-quic
```

## Kural dosyası

Satır başına bir kural: `<host> <strateji|bypass>`. Bir kural host'un
subdomain'lerini de kapsar. Boş satır ve `#` yorumları atlanır.

```
discord.com        record:sni
*.discordapp.com   record:sni
example.com        bypass
```

Bağlantı başına sıra: kural dosyası → `--auto` keşfi → `--strategy` varsayılan.

## Stratejiler

`mode:at[:off]` — `mode` = `record` (RFC uyumlu TLS record fragmentation) veya
`seg` (TCP segment split); `at` = `sni` (SNI içinde böl) veya `fixed:<n>`.
Yerleşikler için `unsni strategies`.

## Ses / UDP + masaüstü uygulamaları (tam tünel)

Proxy Discord **sesini** (doğrudan UDP) taşıyamaz ve **masaüstü uygulamasını**
düzeltemez (updater proxy'yi yok sayar). İkisi de ağ katmanında tünel ister.

**En kolayı — gömülü tünel (WireGuard kurmaya gerek yok):**

```bash
sudo unsni tunnel     # wireguard-go gömülü, WARP'a bağlanır, tüm trafiği geçirir
# Discord'u (masaüstü veya tarayıcı) aç; ses çalışır. Ctrl+C / pencereyi kapat.
```

Şifre ister (route değişikliği admin gerektirir), çıkışta route/DNS'i geri alır.
Default route asla silinmez, bu yüzden çökme durumunda bile bağlantı kendini toparlar.

**Alternatif — config üretip WireGuard ile kendin çalıştır:**

```bash
unsni warp --out warp.conf
wg-quick up ./warp.conf     # WireGuard kurulu olmalı (brew install wireguard-tools)
# ... tünel açıkken ses çalışır ...
wg-quick down ./warp.conf
```

`--allowed-ips` ile split-tunnel için daralt (varsayılan full tunnel — sesin
taşındığını garanti etmenin en basit yolu).

## Gözlemlenebilirlik

`http://127.0.0.1:9090/metrics` bağlantı sayaçlarını verir. `--debug` her
bağlantıda SNI bulundu mu ve ClientHello nasıl bölündü loglar.
