# Concord — 5 Maddelik Geliştirme Planı

**Repository:** `kaplanburakcan-hub/concoord`  
**Branch:** `main`  
**Hazırlanma tarihi:** 2026-10-08

## Mevcut Durum Özeti

Concord; Go + PostgreSQL tabanlı backend, React/Vite/TypeScript frontend, Docker Compose, MinIO doküman altyapısı, JWT/RBAC, audit log, görev/bildirim, MAR, satınalma, hakediş, EVM/dashboard, PWA/offline ve DR/load-test altyapısını aynı monorepo içinde barındırıyor.

Mevcut mimarinin güçlü tarafı modüllerin proje kapsamı ve yetki motoru etrafında organize edilmiş olması. Son geliştirmelerde özellikle doküman taksonomisi, Excel round-trip, saha konumu, hata mesajları ve finansal hesap doğruluğu üzerinde yoğunlaşılmış.

Aşağıdaki plan, mevcut fonksiyonları tekrar etmek yerine Concord'u daha ölçeklenebilir, ölçülebilir ve ticari olarak güçlü bir proje kontrol platformuna dönüştürmeye odaklanır.

---

# 1. Ürün Kalitesi ve Otomatik Kalite Kapısı

**Öncelik:** P0 — İlk yapılmalı

### Amaç

Yeni özellik ekleme hızını korurken regresyon riskini azaltmak ve `main` branch'ini sürekli deploy edilebilir durumda tutmak.

### Önerilen geliştirmeler

- GitHub Actions tabanlı CI pipeline oluşturulması.
- Backend için:
  - `go test ./...`
  - race detection
  - migration smoke test
  - kritik RBAC test matrisi
- Frontend için:
  - TypeScript build
  - lint/type-check
  - kritik ekranlar için component/E2E testleri
- Docker image build doğrulaması.
- Migration up/down ve temiz PostgreSQL üzerinde smoke test.
- Kritik API endpoint'leri için otomatik health/API contract testleri.
- PR açıldığında zorunlu quality gate.
- `main` için doğrudan hatalı deploy'u engelleyen branch protection.

### Neden şimdi?

Repo'da backend test altyapısı güçlü olmasına rağmen frontend `package.json` seviyesinde test/lint scriptleri görünmüyor. Ayrıca ürünün modül sayısı arttıkça manuel regresyon maliyeti hızla büyüyecek.

### Kabul kriteri

> Bir PR; backend testleri, frontend build'i, migration testi ve kritik smoke testler geçmeden `main`'e merge edilememeli.

---

# 2. Concord Design System + Gerçek Dark Mode / UX Standardizasyonu

**Öncelik:** P0

### Amaç

Çok sayıda modül büyüdüğü için ekranların aynı ürünün parçaları gibi davranmasını sağlamak.

### Önerilen geliştirmeler

- Merkezi design token sistemi:
  - renkler
  - spacing
  - typography
  - radius
  - shadow
  - status renkleri
- `Light / Dark / System` tema tercihi.
- Dark mode'un yalnızca CSS renk değişimi değil, tüm grafiklerin ve veri tablolarının kontrast kurallarına göre ele alınması.
- Ortak:
  - DataTable
  - FilterBar
  - Modal
  - Drawer
  - StatusBadge
  - KPI Card
  - Empty State
  - Error State
  - Loading/Skeleton
  bileşenlerinin standartlaştırılması.
- Mobil saha kullanımında minimum 44px dokunma hedefi.
- Finansal değerler, tarihler ve durumların tüm modüllerde tek formatta gösterilmesi.
- Dashboard, hakediş, satınalma ve doküman ekranlarında ortak görsel hiyerarşi.

### Yeni ürün fikri

Kullanıcıya `Yönetici / Saha / Finans / Taşeron` gibi çalışma profiline göre yoğunluğu değişen bir **Workspace Mode** sunulabilir.

Örneğin yönetici ekranı KPI ve riskleri öne çıkarırken saha modu görev, fotoğraf, MAR ve teslimat işlemlerini öne çıkarır.

### Kabul kriteri

> Kullanıcı tema tercihi kalıcı olmalı; tüm ana sayfalar aynı component/token sistemini kullanmalı ve dark mode'da okunabilirlik kaybı olmamalı.

---

# 3. Proje Kontrol "Risk Engine" — Concord'un Proaktif Karar Destek Katmanı

**Öncelik:** P1 — En yüksek ticari değer potansiyeli

### Amaç

Concord'un yalnızca “ne oldu?” sorusunu cevaplayan bir kayıt sistemi olmaktan çıkıp “ne olacak ve nerede müdahale etmeliyim?” sorusunu cevaplayan bir proje kontrol sistemine dönüşmesi.

### Önerilen geliştirmeler

Mevcut EVM, milestone, görev, MAR, satınalma, hakediş ve İSG verilerinin üzerinde merkezi bir risk motoru kurulmalı.

Motor aşağıdaki sinyalleri birlikte değerlendirebilir:

- SPI/CPI trendi
- geciken milestone
- kritik görevlerin yaşlanması
- geciken PO
- bekleyen MAR
- hakediş sapmaları
- taşeron performansı
- satınalma lead-time
- açık İSG bulguları
- sözleşme tarihi / süre yaklaşımı
- planlanan ve gerçekleşen ilerleme farkı

### Örnek risk üretimi

```text
RISK-024
Kategori: Procurement
Seviye: HIGH

PO-018'in ihtiyaç tarihi 12 gün geçmiş.
İlgili malzeme kritik milestone M-07'yi etkiliyor.
PO gecikmesi + milestone float < 7 gün olduğundan
proje program riski oluşturuyor.

Önerilen aksiyon:
Tedarikçiden recovery plan iste.
Alternatif tedarikçi araştır.
Milestone etkisini yeniden hesapla.
```

### Yeni öneri

Risklerin yalnızca renkli gösterilmesi yerine her risk için:

**Signal → Impact → Owner → Recommended Action → Due Date → Resolution**

zinciri oluşturulmalı.

### Kabul kriteri

> Yönetici dashboard'ında yalnızca KPI değil, önceliklendirilmiş risk listesi ve her risk için atanabilir aksiyon bulunmalı.

---

# 4. AI Document & Contract Intelligence

**Öncelik:** P1

### Amaç

Concord'daki doküman motorunu pasif arşiv olmaktan çıkarıp sözleşme ve proje bilgisinin aktif bilgi katmanına dönüştürmek.

### Önerilen geliştirmeler

Doküman yüklenirken AI destekli metadata çıkarımı:

- doküman türü
- sözleşme numarası
- taraflar
- tarih
- revizyon
- geçerlilik tarihi
- tutar
- para birimi
- ilgili proje
- ilgili taşeron
- ilgili PO/PR/MAR
- kritik tarihler
- notice/deadline ifadeleri

### Sözleşme özelinde

PDF/DOCX içinden:

- ödeme koşulları
- teminat hükümleri
- gecikme cezaları
- süre uzatımı hükümleri
- notice süreleri
- değişiklik/variation hükümleri
- fesih koşulları
- sigorta gereklilikleri

çıkarılarak yapılandırılmış Contract Intelligence kaydı oluşturulabilir.

### Yeni öneri

Bir **Contract Watchlist** oluşturulmalı.

Örneğin:

```text
CONTRACT-014
Notice Deadline       18 Oct 2026
Performance Bond      Valid
Insurance             Expires in 21 days
EOT Notice             9 days remaining
LD Exposure            HIGH
```

Bu alanlar mevcut bildirim motoru ile ilişkilendirilebilir.

### Kabul kriteri

> Kullanıcı bir sözleşme yüklediğinde temel metadata ve kritik tarihlerin önemli kısmı otomatik çıkarılabilmeli; kullanıcı sonuçları onaylayıp düzeltebilmeli.

---

# 5. Saha-Öncelikli Offline Execution + Kanıt Zinciri

**Öncelik:** P1

### Amaç

Concord'un masaüstü proje kontrol yazılımından gerçek saha operasyon platformuna dönüşmesini sağlamak.

Mevcut PWA/offline altyapısı bunun için iyi bir başlangıç noktası.

### Önerilen geliştirmeler

Sahadaki kullanıcı için:

- offline görev listesi
- offline fotoğraf çekimi
- GPS/konum
- timestamp
- kullanıcı kimliği
- ilgili proje
- ilgili görev/MAR/PO
- kısa saha notu
- tekrar bağlantıda otomatik senkronizasyon

### Kanıt zinciri

Her saha kanıtı:

```text
Project
  ↓
Work Item / Task
  ↓
Evidence
  ├── Photo
  ├── Location
  ├── Timestamp
  ├── User
  └── Comment
       ↓
Audit Log
```

şeklinde immutable bir kayıt zincirine bağlanmalı.

### Yeni öneri: Saha Günlüğü

Tek bir **Daily Site Log** ekranı oluşturulabilir:

- hava durumu
- çalışan sayısı
- ekipman
- yapılan işler
- beton/döküm bilgileri
- teslimatlar
- gecikmeler
- İSG olayları
- fotoğraflar
- ziyaretçiler
- müşavir/işveren notları

Gün sonunda otomatik PDF günlük raporu üretilebilir.

### Kabul kriteri

> Saha kullanıcısı internet olmadan kritik veri ve fotoğrafı kaydedebilmeli; bağlantı geldiğinde kayıtlar güvenli şekilde senkronize edilmeli ve her kanıt proje/audit zincirine bağlanmalı.

---

# Önceliklendirme

| Sıra | Geliştirme | Öncelik | Beklenen Etki |
|---|---|---|---|
| 1 | CI/CD + Quality Gates | P0 | Regresyon ve deploy riskini azaltır |
| 2 | Design System + Dark Mode | P0 | Ürün kalitesini ve kullanılabilirliği artırır |
| 3 | Risk Engine | P1 | Yönetici karar desteğini güçlendirir |
| 4 | AI Contract Intelligence | P1 | Concord'u farklılaştırır |
| 5 | Offline Field Execution | P1 | Saha kullanımını ve veri kalitesini artırır |

# Önerilen Uygulama Sırası

```text
PHASE A
CI/CD + Quality Gates
        ↓
PHASE B
Design System + Dark Mode
        ↓
PHASE C
Risk Engine
        ↓
PHASE D
AI Document / Contract Intelligence
        ↓
PHASE E
Offline Field Execution + Daily Site Log
```

## Stratejik Hedef

Concord'un bir sonraki aşamasında yeni ekran sayısını artırmak yerine mevcut modüllerin ürettiği veriyi birbirine bağlamak daha yüksek değer yaratacaktır.

Hedef mimari:

```text
                 ┌───────────────────┐
                 │   CONCORD DATA    │
                 └─────────┬─────────┘
                           │
        ┌──────────────────┼──────────────────┐
        ↓                  ↓                  ↓
   Project Control     Contract Intel     Field Data
        │                  │                  │
        └──────────────────┼──────────────────┘
                           ↓
                    ┌──────────────┐
                    │  RISK ENGINE │
                    └──────┬───────┘
                           ↓
                    ┌──────────────┐
                    │ MANAGEMENT   │
                    │ DECISIONS    │
                    └──────────────┘
```

**Ana hedef:** Concord'u kayıt tutan bir PYS'den, proje verisini yorumlayan ve yöneticinin bir sonraki aksiyonunu öneren bir **Project Intelligence Platform** haline getirmek.
