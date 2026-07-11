# ADR 0005 — Faz 4: Görev Yönetimi + Bildirim Motoru

Tarih: 2026-07-07 · Durum: Kabul edildi · Plan referansı: §6.5, §8 (Faz 4), §9

## Bağlam
Bildirim motoru bilinçli olarak erkene alındı (Plan §9): MAR, günlük rapor,
satınalma ve İSG modüllerinin tamamı bildirim üretir. Merkezi motor olmadan
her modül kendi gönderim kodunu yazar ve mimari borç doğar. Görev yönetimi bu
motorun ilk tüketicisidir ve kabul kriterlerini uçtan uca doğrular.

## Kararlar

### 1. Kuyruk: PostgreSQL tablosu (`job_queue`), river kütüphanesi değil
Plan §2 "Go worker + PostgreSQL kuyruk (river)" der. river'ın kendisi yerine
aynı deseni uygulayan yalın bir tablo + `FOR UPDATE SKIP LOCKED` seçildi:
- Faz 4 iş yükü (e-posta/SMS) için river'ın migration seti ve API yüzeyi
  gereksiz karmaşıklık; bağımlılık sıfır tutuldu.
- Desen birebir aynıdır (atomik iş alma, attempts, üstel geri çekilme);
  Faz 6 PDF işleri hacmi büyütürse river'a geçiş, Enqueue/Dequeue arayüzünün
  arkasında tek dosyalık değişikliktir.

### 2. Gönderim asenkron; API isteği hiçbir kanalı beklemez
`notify.Service.Send` in-app kaydı yazar ve Email/SMS için kuyruk işi üretir.
SMTP/SMS gecikmesi veya kesintisi kullanıcı isteğini yavaşlatmaz/düşürmez.
Bildirim, iş değişikliğinin yan etkisidir: gönderim hatası iş kaydını geri
almaz, loglanır ve kuyrue yeniden denenir.

### 3. Kanal tercihleri veridir; varsayılan satırsızdır
`notification_preferences` yalnızca varsayılandan sapmaları tutar
(InApp+Email açık, SMS kapalı). Yeni kanal eklemek şema değişikliği değildir.
SMS adaptörü `SMSSender` arayüzü arkasındadır (Plan §2): Netgsm hazır,
İleti Merkezi vb. tek dosyalık adaptörle eklenir; sağlayıcı yapılandırılmamışsa
log adaptörü çalışır (dev ortamı ve sağlayıcısız kurulum güvenli).

### 4. Kanban sırası: float `kanban_order`
Araya kart eklemek komşu iki değerin ortalamasıdır — tek satır UPDATE, kolon
geneli yeniden numaralandırma yok. Uzun vadede hassasiyet daralırsa kolon
bazlı normalize job'ı eklenir (backlog).

### 5. edit_own kapsamı: oluşturan **veya** atanan
Plan izin sözlüğündeki `tasks.edit_own` "kendi görevleri" ifadesi, saha
pratiğine göre "oluşturduğu ya da kendisine atanan" olarak yorumlandı: atanan
kişi kartını Kanban'da ilerletebilmelidir. Silme daha dar: `edit_all` ya da
(`edit_own` + oluşturan). Kontroller handler içindedir; rota izni `tasks.view`
kalır çünkü middleware tek izin denetler.

### 6. Deadline hatırlatması: görev başına tek işaret
`tasks.deadline_notified_at` tekrar bildirimi engeller; termin değiştirilirse
API alanı NULL'lar ve hatırlatma yeni termine göre yeniden üretilir. Kural:
termin ≤ yarın ve statü ≠ Done. Atanan yoksa oluşturan bilgilendirilir.

### 7. @mention çözümü sunucuda, proje üyeleriyle sınırlı
`@kullanıcıadı` yalnızca ilgili projenin aktif üyeleri arasında çözülür;
eşleşmeyen adlar sessizce yoksayılır (yanlış yazım bildirim üretmez). Çözülen
kullanıcı id'leri `task_comments.mentions` (JSONB) alanında saklanır — plan
şemasıyla uyumlu ve denetlenebilir.

## Kabul kriterleri karşılığı (Plan, Faz 4)
- "Görev atanan kullanıcı in-app + e-posta bildirimi alıyor" → atama anında
  `task_assigned` in-app kaydı + `send_email` kuyruk işi; worker SMTP'den yollar.
- "Deadline yaklaşınca otomatik hatırlatma üretiliyor" → worker 15 dakikada bir
  (ve açılışta) tarar; `task_deadline` bildirimi tüm kanallara tercihle dağılır.

## Sonuçlar
- Faz 5+ modülleri `notify.Service.Send` çağırır; yeni tür = yeni sabit.
- Worker artık gerçek iş işler; Faz 6 PDF üretimi aynı kuyruğa `kind` ekler.
- SMS masrafı varsayılan kapalı; sağlayıcı anlaşması sonrası kullanıcı bazında açılır.
