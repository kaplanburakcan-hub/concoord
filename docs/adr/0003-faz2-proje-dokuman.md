# ADR-0003 — Faz 2 Proje Çekirdeği ve Doküman Motoru Kararları

**Tarih:** 06.07.2026 · **Durum:** Kabul edildi

## Bağlam
Faz 2, proje CRUD + milestone yönetimi + çoklu proje navigasyonunu ve SHA-256
doğrulamalı, versiyonlu doküman motorunu (MinIO nesne depolama) teslim eder.
`projects`/`project_members` şeması Faz 1'de kuruldu; Faz 2 bunu tüketir ve
`milestones`, `folders`, `documents`, `document_versions`, `files` tablolarını ekler.

## Kararlar

1. **Nesne depolama: harici SDK yerine stdlib + AWS SigV4.** MinIO erişimi
   `internal/storage` içinde yalnızca standart kütüphaneyle imzalanır. Böylece
   go.mod'a yeni bağımlılık **eklenmez** (minimalist yığın korunur) ve imzalama
   davranışı tümüyle denetlenebilir kalır. Hem başlık imzalama (sunucu→MinIO
   Put/Get) hem sorgu imzalama (presign) uygulanır.

2. **Yükleme/indirme API üzerinden (sunucu aracılı), presign yedekte.** MinIO
   dev'de `127.0.0.1`, prod'da sadece iç ağda dinler; doğrudan tarayıcı-presign
   akışı public bir S3 fronting gerektirir. Faz 2, güvenlik ve dağıtım basitliği
   için **her isteği izin+proje kontrolünden geçiren** sunucu aracılı akışı seçer:
   yükleme akışında SHA-256 hesaplanır, indirme akışla yapılır. Depolama anahtarı
   istemciye sızmaz → "URL'i bilse dahi yetkisiz erişemez" kabul kriteri sağlanır.
   `PresignGet` ileride public MinIO fronting için hazırdır (`IPKS_S3_PUBLIC_ENDPOINT`).

3. **Versiyon ve dosya kayıtları immutable / append-only.** `document_versions`
   ve `files` tablolarında `updated_at/deleted_at/row_version` bilinçli olarak
   yoktur (Plan §5.1 değişmezlik ilkesi). Düzeltme = yeni versiyon; eski versiyon
   daima erişilebilir. Nesne DB kaydından önce yazılır; versiyon numarası
   benzersizliği DB kısıtıyla eşzamanlılığa karşı korunur.

4. **`projects.create` / `projects.delete` izinleri sözlüğe eklendi.** Proje
   açma/arşivleme Faz 1 sözlüğünde yoktu. İzin verisi tek kaynaktan (`internal/rbac`)
   büyütüldü: `create` Admin+ProjectManager varsayılanı, `delete` yalnızca Admin.
   Mevcut kurulumlar için idempotent bir seed sync adımı (`0006_faz2_izin_sync`)
   yeni izinleri upsert eder ve bootstrap admin'e global GRANT olarak ekler.
   Kod değişmeden yeni izin = veri ilkesi korunur (Plan #2).

5. **Proje kapsamı URL segmentinden.** Faz 2 uçları `/projects/{projectID}/...`
   altında; `RequirePermission` proje kimliğini URL'den çözer, veri kapsamı da
   aynı kimlikle zorlanır (çift katman). Proje listesi kişiye özel filtrelenir:
   üyelik VEYA global `projects.view` GRANT'i.

6. **Frontend: ProjectProvider ile proje context'i.** Seçili proje localStorage'da
   kalıcıdır ve değişince Auth katmanına bildirilir → izinler o projenin kapsamında
   yeniden çözülür (proje bazlı RBAC). Üst bardaki seçici çoklu proje navigasyonunu
   sağlar; portföy dashboard'u Faz 9'a bırakılır.

## Sonuçlar
- Sözleşme/zeyilname arşivi doküman motorunun ilk tüketicisidir; PDF `v1→v2`
  versiyonlanır, eski versiyon indirilebilir.
- Sonraki modüller (hakediş, İSG, irsaliye) polimorfik `entity_type/entity_id`
  bağıyla aynı motoru kullanır — her modül kendi dosya kodunu yazmaz.
- Bilinen sınır: doğrudan tarayıcı-presign yükleme, public MinIO fronting
  gerektirdiğinden Faz 2'de devrede değil; sunucu aracılı akış yeterli ve daha güvenli.
