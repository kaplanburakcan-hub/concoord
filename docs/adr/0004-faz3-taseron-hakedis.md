# ADR-0004 — Faz 3: Taşeron ve Hakediş Yönetimi (Finansal Çekirdek)

Durum: Kabul edildi · Tarih: Faz 3 · Bağlam: Plan §8, §6.3, §6.4, §5.1

Faz 3 platformun finansal kayıt sistemidir: taşeron kartları, alt sözleşme
arşivi, birim fiyat cetveli (BOQ) ve kümülatif hakediş iş akışı. Aşağıdaki
kararlar bu fazın "değişmezlik + birebir doğruluk" hedefini korur.

## 1. Hesap çekirdeği saf ve birim testli (Plan §6.4)

Kümülatif hesap (`internal/payments/calc.go`) tamamen DB'siz/I/O'suz saf bir
fonksiyondur (`Compute`). Böylece Plan'daki kabul kriteri — *"kümülatif ikinci
dönemde doğru taşınıyor; avans mahsubu ve teminat kesintisi sentetik doğrulama
setiyle birebir tutuyor"* — deterministik birim testlerle (`calc_test.go`)
kanıtlanır. Handler yalnızca girdi toplar, sonucu yazar.

- Kolonlar: A brüt kümülatif, B önceki dönem, C bu dönem brüt, D avans mahsubu,
  E teminat, F/G/H ekstra (İSG/vergi/diğer), I net (KDV hariç), + KDV ayrı satır.
- Avans mahsubu **kalan avansı aşamaz** (ikinci dönemde sınırlama testi mevcut).
- Yuvarlama: kuruş (2 ondalık), sıfırdan uzağa; her ara ve nihai tutara uygulanır.
- Oranlar yüzde olarak taşınır (3.0 = %3); core kesire çevirir.

## 2. Finansal değişmezlik DB seviyesinde (Plan §5.1)

`Finalized` hakediş satırı ve kalemleri PostgreSQL trigger'larıyla UPDATE/DELETE'e
kapatılır (`lock_finalized_payment`, `lock_finalized_children`, ERRCODE
`restrict_violation`). Kesinleşmeye **geçiş** serbesttir; kesinleştikten sonra her
değişiklik reddedilir. Uygulama katmanı da geçiş grafiğini (`CanTransition`)
uygular, ama son söz DB'dedir → uygulama hatası veri bozamaz. Handler kilit
ihlalini (`23001`) 409'a çevirir. Düzeltme = `revision_of` referanslı yeni kayıt.

## 3. Excel/CSV içe aktarma — yeni bağımlılık yok

ADR-0003 ethosuyla (harici kütüphane yerine stdlib) tutarlı olarak `.xlsx`
yalnızca `archive/zip` + `encoding/xml` ile okunur (`sharedStrings.xml` + ilk
çalışma sayfası); `.csv` de desteklenir (`import.go`). `go.mod`'a Excel bağımlılığı
eklenmez. Sütun düzeni A–E: poz_no, açıklama, birim, sözleşme miktarı, birim fiyat;
başlık satırı sezilir ve atlanır. Upsert `(subcontractor_id, poz_no)` ile.

## 4. Hakediş özet PDF'i — stdlib

Rich HTML→PDF (gotenberg/chromedp) Faz 3'te devreye alınmaz; özet tek/çok sayfalık
metin PDF'i olarak stdlib ile elle kurulur (`pdf.go`) — yeni bağımlılık yok, çıktı
deterministik. Base-14 Helvetica WinAnsi Türkçe karakter taşımadığından etiketler
ASCII'ye çevrilir (`asciiTR`); sayısal veri zaten ASCII'dir. Zengin şablon Faz 6/9
rapor motoruna ertelenir.

## 5. İzin haritalama — yeni izin yok

Faz 3 mevcut izin sözlüğünü kullanır; yeni modül izni eklenmez:
- Taşeron / sözleşme / birim fiyat CRUD → `contracts.{view,upload,delete}`
  (ayrı "subcontractors" modülü yok; bunlar finansal/sözleşme alanının parçası).
- Hakediş iş akışı → `progress_payments.{view,create_draft,edit_draft,submit,approve,finalize}`.
- Finansal görünürlük → `progress_payments.view_financials` (aşağıya bakınız).

## 6. Satır seviyesi güvenlik + view_financials ayrımı (Plan §4)

- **Satır seviyesi**: `project_members.subcontractor_id` (Faz 1'de forward-declare,
  Faz 3'te FK bağlandı) bir kullanıcıyı bir taşerona bağlar (SubcontractorRep).
  Bağlıysa listeler/detaylar backend'de **zorunlu** o taşerona filtrelenir; başka
  taşerona erişim 403. Bağlı değilse (PM/Admin/SiteEngineer) kısıtsız.
- **view_financials**: `progress_payments.view` metrajı (miktar) gösterir; birim
  fiyat/tutar/net yalnızca `view_financials` ile döner. Maskeleme sözleşme, birim
  fiyat cetveli, hakediş ve PDF genelinde tutarlı uygulanır (SiteEngineer varsayılan
  olarak onay verir ama tutarları görmez).

## 7. Sözleşme şartlarının çözümü

Bir hakediş için avans/teminat/avans-mahsup oranları, taşeronun **yürürlükteki**
sözleşmesinden okunur (en yeni `Main`/`Sub`, `sign_date` → `created_at` sırasıyla).
Önceki dönemlere dek mahsup edilmiş avans, geçmiş `Finalized` hakedişlerin
`AdvanceOffset` kesintilerinin toplamıdır. KDV oranı hakediş kaydında tutulur
(varsayılan %20). Kesinleştirmede kayıtlı metraj + güncel birim fiyat + kayıtlı
manuel kesintilerle yeniden hesaplanır (otomatik D/E tazelenir), sonra kilitlenir.
