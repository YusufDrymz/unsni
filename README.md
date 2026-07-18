# unsni

**Türkçe** · [English](README.en.md)

> Engelli sitelere girmeni ve **Discord'un açılmasını** sağlayan küçük bir araç.
> Kurulum yok, ayar yok — indir, çift tıkla, kullan.

[![CI](https://github.com/YusufDrymz/unsni/actions/workflows/ci.yml/badge.svg)](https://github.com/YusufDrymz/unsni/actions)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

## Ne işe yarar?

İnternet sağlayıcın (ISP) bazı siteleri ve Discord'u engelliyorsa, `unsni` bu
engeli aşar. Arka planda trafiğini "temizleyerek" engellenen sitelere yeniden
erişmeni sağlar. VPN gibi hesap açman, para ödemen ya da bir şey ayarlaman
gerekmez.

**Çalıştığı yerler:** engelli web siteleri, **Discord giriş + yazışma**.
Türkiye'deki gerçek engellere karşı test edildi.

---

## Kurulum (terminal bilmene gerek yok)

### 1. Dosyayı indir

[**Releases sayfasından**](https://github.com/YusufDrymz/unsni/releases/latest)
sistemine uygun olanı indir:

| Sistemin | İndireceğin dosya |
|----------|-------------------|
| **Windows** | `unsni_..._windows_amd64.zip` |
| **Mac (M1/M2/M3 — yeni Mac'ler)** | `unsni_..._darwin_arm64.tar.gz` |
| **Mac (Intel — eski Mac'ler)** | `unsni_..._darwin_amd64.tar.gz` |

> Mac'inin hangisi olduğunu bilmiyorsan: sol üstteki  → "Bu Mac Hakkında". "Apple"
> yazıyorsa arm64, "Intel" yazıyorsa amd64.

### 2. Dosyayı çıkar (unzip)

İndirdiğin dosyaya çift tıkla, içinden bir klasör çıkar. İçinde şunlar olur:
`start-windows.bat` / `start-macos.command`, `unsni` (programın kendisi) ve
birkaç yardım dosyası.

### 3. Doğru dosyaya çift tıkla

| Ne istiyorsun? | Çift tıklayacağın dosya | Not |
|----------------|--------------------------|-----|
| Engelli siteler + **Discord'u tarayıcıdan** kullanmak | **Windows:** `start-windows.bat`<br>**Mac:** `start-macos.command` | Yönetici/şifre gerekmez |
| **Discord masaüstü uygulaması + sesli konuşma** | **Mac:** `start-macos-full.command` | Mac şifreni ister |

Çift tıklayınca siyah/beyaz bir pencere açılır ("trafik yönlendiriliyor" yazar).
**Bu pencereyi açık bırak.** Şimdi tarayıcını veya Discord'u aç ve kullan.

**İşin bitince pencereyi kapat** — tüm ayarlar otomatik eski haline döner.

> ℹ️ `unsni.exe` / `unsni` dosyasına çift tıklama — o sadece programın kendisi,
> yardım metni basıp kapanır. Sen yukarıdaki başlatma dosyasını kullan.

---

## İlk açılışta çıkabilecek uyarılar (normaldir)

- **Windows — "Windows bilgisayarınızı korudu" (mavi ekran):**
  Program imzalı olmadığı için çıkar, tehlike değil.
  → **"Ek bilgi"** → **"Yine de çalıştır"**.
- **Windows — Güvenlik Duvarı "erişime izin verilsin mi?" sorarsa:**
  → **"Erişime izin ver"**. (İzin vermezsen siteler açılmaz.)
- **Mac — "geliştirici doğrulanamadı":**
  Başlatma dosyasına **sağ tık → Aç** de (sadece ilk seferde).

---

## Bir şeyler çalışmıyorsa

- **Siteler açılmadı / internet gitti gibi oldu:**
  Pencereyi kapat, **10 saniye bekle**, başlatma dosyasına tekrar çift tıkla.
- **Windows'ta Güvenlik Duvarı uyarısını kaçırdıysan:**
  Pencereyi kapat, tekrar başlat, bu sefer **"Erişime izin ver"** de.
- **Hâlâ olmuyorsa:** Görev Yöneticisi'nde arka planda takılı kalmış `unsni.exe`
  varsa kapat, sonra tekrar başlat.

---

## Discord masaüstü uygulaması ve ses

Yukarıdaki hafif mod (`start-windows.bat` / `start-macos.command`) **tarayıcı**
için çalışır: siteler + tarayıcıdaki Discord (giriş + yazışma).

**Discord'un masaüstü uygulaması ve sesli konuşma** için tam tünel modu gerekir.
Şu an bu mod **sadece Mac'te** hazır: `start-macos-full.command` (Mac şifreni
ister, tüm trafiği güvenli bir tünelden geçirir). Windows için tam tünel modu
henüz yok — Windows'ta Discord'u **tarayıcıdan** kullan.

---

<details>
<summary><b>İleri kullanım (komut satırı / CLI)</b></summary>

Teknik kullanıcılar için. Saf Go, tek binary, **cgo yok**, cross-platform.

### Kurulum

```bash
go install github.com/YusufDrymz/unsni/cmd/unsni@latest
```

### Temel komutlar

```bash
# 1. Engelli bir host için çalışan en hızlı stratejiyi bul
unsni find discord.com
# -> best: record:sni (handshake 84ms)

# 2. Yerel proxy'yi çalıştır (HTTP CONNECT + opsiyonel SOCKS5, per-domain rules, auto)
unsni run --strategy record:sni --socks 127.0.0.1:1080 --rules rules.txt --auto

# En kolayı: sistem proxy'sini otomatik kurar, çıkışta geri alır
unsni run --system-proxy
```

### Engeli teşhis et

```bash
unsni doctor discord.com
# baseline (no desync): FAILED at TLS handshake  -> SNI-based block confirmed
# record:sni          : OK (84ms)
# seg:sni             : OK (91ms)
```

### Stratejiler

`mode:at[:off]` — `mode`: `record` (RFC uyumlu TLS record fragmentation) veya
`seg` (TCP segment split); `at`: `sni` (SNI hostname içinde böl) veya
`fixed:<n>` (sabit payload offset).

```bash
unsni strategies   # yerleşikleri listele
```

### Discord sesi (UDP)

Proxy UDP taşıyamaz. Ses için WireGuard/WARP tüneli üret ve proxy'yle birlikte
çalıştır (proxy TCP'yi desync eder, tünel UDP'yi taşır):

```bash
unsni warp --out warp.conf && wg-quick up ./warp.conf
```

Tam rehber: [`docs/usage.tr.md`](docs/usage.tr.md) · English: [`docs/usage.md`](docs/usage.md)

### Neden başka araçlardan farklı?

Çoğu DPI bypass aracı (SpoofDPI, byedpi, zapret) ya tek bir desync numarasını
sabit kullanır ya da sana kriptik bir `blockcheck` çalıştırıp parametreleri elle
kopyalatır — bir sitenin **neden** engelli olduğuna dair sıfır bilgi verir.
`unsni` ise strateji uzayını senin ISP'ne karşı otomatik prob eder, en hızlı
çalışanı bulur ve gerçek observability (`/metrics`) sunar.

### Geliştirme

```bash
make test    # go test -race ./...
make vet
make cross   # cross-compile smoke check (linux/windows/darwin, cgo off)
```

</details>

## Lisans

MIT — bkz. [LICENSE](LICENSE).
