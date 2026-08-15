// Standart ad/tip kataloğu — "Ad / İsim" alanı bu listeden seçilir (spesifik
// makine kimliği plaka/marka/model/seri no ile ayrışır, "Ad" ortak tipi verir).
// Aynı zamanda gruplu galeri görünümünün (MachinePage.tsx) grup başlıklarını
// ve temsili fotoğraf yollarını besler.

export type Tip = "arac" | "is_makinesi" | "ekipman";

export const AD_KATALOG: Record<Tip, string[]> = {
  arac: [
    "Binek Otomobil",
    "Panelvan",
    "Kamyonet (Pickup)",
    "Minibüs / Servis Aracı",
  ],
  is_makinesi: [
    "Kule Vinç",
    "Mobil Vinç",
    "Paletli Ekskavatör",
    "Lastikli Ekskavatör",
    "Yükleyici (Loder)",
    "Kazıcı Yükleyici (Beko Loder)",
    "Dozer",
    "Greyder",
    "Silindir (Sıkıştırma)",
    "Damperli Kamyon",
    "Çekici (Tır Başı)",
    "Su Tankeri - Arazöz",
    "Yakıt Tankeri",
    "Vidanjör",
    "Beton Pompası (mobil)",
    "Beton Pompası (sabit / yer pompası)",
    "Transmikser (Beton Mikseri)",
    "Forklift",
    "Teleskopik Forklift",
    "Vinçli Kamyon",
    "Asfalt Finişeri",
    "Sondaj / Delici Makinesi",
    "Hidrolik Kırıcı",
    "Mobil Platform (Sepetli/Makaslı)",
    "Mini Ekskavatör",
  ],
  ekipman: [
    "Jeneratör",
    "Kompresör",
    "Transpalet",
    "Basınçlı Yıkama Makinesi",
    "Su Pompası (Dalgıç Motor dahil)",
    "Kaynak Makinesi",
    "Beton Vibratörü",
    "Beton Kesme Makinesi",
    "Beton Perdah Makinesi",
    "Demir Kesme/Bükme Makinesi",
    "Kırıcı Delici (Hammer & Breakers)",
    "İskele Sistemi",
    "Kalıp Sistemi",
    "Aydınlatma Kulesi",
    "El Aletleri Seti",
    "El Kompaktörü (Plate Compactor)",
    "Drone",
    "Isıtıcı (ISIMAK vb.)",
  ],
};

export const AD_DIGER = "__diger__";

// trLower — Türkçe İ/I çiftini JS'in varsayılan toLowerCase()'inin
// bozduğu (İ → birleşik noktalı i̇) yerlerde ekran metni için kullan.
export function trLower(s: string): string {
  return s.replace(/İ/g, "I").toLowerCase();
}

// slugify — katalog adını dosya adına çevirir (bkz. public/ekipman-foto/).
// Türkçe karakterleri ASCII'ye indirger; İ/ı çifti özel ele alınır (JS'in
// varsayılan toLowerCase()'i "İ"yi "i̇" — birleşik noktalı — yapar, tek
// karakter "i" değil).
export function slugify(s: string): string {
  return s
    .replace(/İ/g, "I")
    .toLowerCase()
    .replace(/ç/g, "c").replace(/ğ/g, "g").replace(/ı/g, "i")
    .replace(/ö/g, "o").replace(/ş/g, "s").replace(/ü/g, "u")
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "");
}

// photoUrl — katalog tipi için temsili fotoğraf yolu (public/ekipman-foto/
// altında statik olarak paketlenir — S3/R2 bekleyen dosya yükleme motorundan
// bağımsız, bkz. memory: project_fotograflar_s3_bekliyor). Katalogda
// olmayan ("Diğer", elle girilmiş) adlar için undefined döner — çağıran
// taraf jenerik bir yer tutucu gösterir.
export function photoUrl(tip: Tip, ad: string): string | undefined {
  if (!AD_KATALOG[tip].includes(ad)) return undefined;
  return `/ekipman-foto/${tip}/${slugify(ad)}.jpg`;
}
