# ADR-0010 — Faz 9: Dashboard, EVM ve Yönetim Raporlaması Kararları

Tarih: 08.07.2026 · Durum: Kabul edildi

## 1. EVM hesap çekirdeği SAF fonksiyonlardır, DB derleyicisi ayrıdır

**Karar:** `internal/dashboard/evm.go` yalnızca saf hesap fonksiyonları
içerir (S-eğrisi, SPI/CPI, EAC/ETC, dağılım türetme); DB'den girdi toplama
`evm_db.go`'daki `LoadEVM`'dedir. Dashboard ucu, aylık rapor snapshot'ı ve
uyarı taraması aynı `LoadEVM`'i çağırır.

**Gerekçe:** Kabul kriteri "EVM değerleri elle hesaplanan kontrol setiyle
birebir tutuyor" — saf çekirdek DB'siz birim testlenir (`evm_test.go`,
BAC=1.000.000 senaryosu elle doğrulanmış SPI 0,833 / CPI 1,064 /
EAC 940.000). Tek `LoadEVM` üç tüketicide üç farklı hesap riskini kaldırır
(Faz 3 `payments/calc.go` ile aynı desen).

## 2. EVM tanımları ve PV kaynak önceliği

**Karar (Plan §7):**
- **PV** = planlanan dağılım × BAC. Dağılım öncelik sırası:
  `pv_plan_entries` (manuel aylık giriş) → milestone ağırlıkları
  (planned_date ayına `weight_pct`, Σ'e normalize) → doğrusal dağılım.
  Kaynak, yanıtın `plan_source` alanında şeffaftır.
- **EV** = kesinleşmiş hakediş brütlerinin alt-sözleşme toplamına oranı ×
  BAC (`EVScale = BAC / Σ contracts[Sub,Addendum].amount`; payda veya BAC
  yoksa ölçek 1 — motor sıfıra bölmez, veri geldikçe düzelir).
- **AC** = kesinleşen hakediş `net_payable` toplamı + `Delivered` PO
  tutarları (son teslimat ayına yazılır).
- SPI/CPI paydası 0 iken **0 = tanımsız** döner; arayüz "—" gösterir,
  uyarı taraması tanımsız endeksi ihlal saymaz.

**Gerekçe:** Sistemde halihazırda üretilen, kilitli (immutable) finansal
verilerden türetim — ayrı bir "gerçekleşme girişi" ekranı açılmaz; rakamlar
hakediş/PO kayıtlarına kadar izlenebilir.

## 3. Aylık yönetim raporu haftalık raporun desenini birebir kullanır

**Karar:** `monthly_reports` tablosu + snapshot JSONB + `monthly_report_pdf`
worker işi + stdlib PDF motorunun paket-yerel kopyası (`dashboard/pdf.go`).
Snapshot, POST anında SENKRON derlenip dondurulur; PDF'e giren her sayı
snapshot'tan gelir ve `GET /monthly-reports/{id}` snapshot'ı aynen döner.
EVM kümülatifleri rapor ayının SONUNA göre kesilir (geriye dönük üretimde
o ayın durumu dondurulur).

**Gerekçe:** Faz 6'da kanıtlanmış akış; "tek tıkla üretim" kabul kriteri
ve rakam doğrulanabilirliği hazır gelir. Aylık derleme çok kaynaklı ve
ağırdır — kuyruk doğru yerdir (ADR-0009/1'in tersi durum).

## 4. Rol duyarlılık backend'de, tek uçla

**Karar:** `GET /projects/{id}/dashboard` herkese açılır (projects.view);
EVM/tutar bloğu handler içinde `reports.view_financial_reports` kontrolüyle
serileştirilir, taşeron temsilcisine hakediş/bulgu sayaçları yalnızca kendi
firması için döner ve aktivite akışı/portföy hiç dönmez. Rol başına ayrı
uç veya yalnızca frontend gizleme YOKTUR.

**Gerekçe:** Plan §4 "finansal görünürlük ayrımı" güvenlik gereğidir;
frontend gizleme API'den veri sızdırır. Tek uç, tek DTO — bakım maliyeti düşük.

## 5. Eşik tabanlı uyarılar: konfigürasyon + tekilleştirme defteri

**Karar:** Eşikler `project_control_settings` satırıdır (CPI/SPI alt eşiği,
bulgu yaşlanma günü; satır yoksa 0.90/0.90/14 varsayılanı). Worker 6 saatte
bir tarar; ürettiği her uyarıyı `control_alerts (project, alert_key,
period='YYYY-MM')` tekil kısıtıyla deftere yazar — INSERT başarılıysa PY
üyelerine (rol varsayılanı `progress_payments.finalize` taşıyanlar)
bildirim gider, çakışırsa sessizce geçer. Ay değişince süren ihlal yeniden
hatırlatılır.

**Gerekçe:** Plan §7 "kontrol tanımları veri olarak tutulur" — yeni eşik
kod değişikliği istemez. Ay bazlı tekilleştirme spam'i önlerken aylık
kontrol ritmiyle hizalıdır; EVM sorgusu ağır olduğundan 6 saatlik tarama
yeterlidir (defter zaten tekrarı engeller).

## 6. S-eğrisi grafiği bağımlılıksız satır içi SVG

**Karar:** Frontend'e grafik kütüphanesi eklenmez; `SCurve.tsx` kümülatif
noktaları saf SVG ile çizer.

**Gerekçe:** Faz 0 kararı "minimum bağımlılık" sürer; üç seri + eksen için
kütüphane maliyeti (bundle, sürüm takibi) getirisini aşar. Noktalar
backend'de hazır hesaplandığından bileşen yalnızca ölçekleme yapar.
