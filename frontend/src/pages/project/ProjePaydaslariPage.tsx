import { useEffect, useRef, useState } from "react";
import * as XLSX from "xlsx";
import { useProjects } from "../../projects/ProjectContext";

// ── Kategori & Alt Kırılım Tanımları ──────────────────────────────────────
type AltKirilim = { id: string; label: string };
type Kategori = {
  id: string;
  label: string;
  icon: string;
  altKirilimlar?: AltKirilim[];
};

const KATEGORILER: Kategori[] = [
  { id: "isveren", label: "İşveren ve Personelleri", icon: "🏛️" },
  { id: "musavir", label: "Müşavir ve Personelleri", icon: "📐" },
  {
    id: "yuklenici", label: "Yüklenici ve Personelleri", icon: "🏗️",
    altKirilimlar: [
      { id: "yk_pm", label: "Proje Yönetim Personeli – PM, Ş.Şefi ve Mühendisler" },
      { id: "yk_idari", label: "İdari Personel – Muhasebe, Finans, Depo, Satınalma, Kamp Pers." },
      { id: "yk_mavi", label: "Mavi Yaka, İşçi, Usta" },
    ],
  },
  {
    id: "alt_yuklenici", label: "Alt Yüklenici ve Personelleri", icon: "🔧",
    altKirilimlar: [
      { id: "ay_pm", label: "Proje Yönetim Personeli – PM, Ş.Şefi ve Mühendisler" },
      { id: "ay_idari", label: "İdari Personel – Muhasebe, Finans, Depo, Satınalma, Kamp Pers." },
      { id: "ay_mavi", label: "Mavi Yaka, İşçi, Usta" },
      { id: "ay_isg", label: "İSG Uzmanı" },
      { id: "ay_kefil", label: "Mali Kefil (Şahsi Kefalet Durumunda)" },
    ],
  },
  { id: "yapi_denetim", label: "Yapı Denetim Firması (Resmi Atanmışsa)", icon: "🔍" },
  { id: "muellif", label: "Proje Müellifi", icon: "✏️" },
  { id: "danismanlar", label: "Danışmanlar (Teknik / İdari / Zorunlu)", icon: "💼" },
  { id: "isg_osgb", label: "İSG – OSGB Firmaları", icon: "⛑️" },
  { id: "tedarikciler", label: "Tedarikçiler", icon: "📦" },
];

// ── Tip Tanımları ─────────────────────────────────────────────────────────
type Paydas = {
  id: string;
  kategoriId: string;
  altKirilimId?: string;
  tip: "kisi" | "firma";
  ad: string;
  soyad?: string;
  unvan?: string;
  firmaAdi?: string;
  telefon?: string;
  email?: string;
  notlar?: string;
};

const BOŞ_FORM: Omit<Paydas, "id"> = {
  kategoriId: "",
  altKirilimId: "",
  tip: "kisi",
  ad: "",
  soyad: "",
  unvan: "",
  firmaAdi: "",
  telefon: "",
  email: "",
  notlar: "",
};

// ── Demo Veri ────────────────────────────────────────────────────────────
function demoPaydaslar(): Omit<Paydas, "id">[] {
  return [
    { kategoriId:"isveren", tip:"firma", ad:"Ali", soyad:"Çelik", unvan:"Proje Direktörü", firmaAdi:"", telefon:"0312 400 4000", email:"ali.celik@isveren.gov.tr", notlar:"Tüm sözleşme değişikliklerinde yetkili" },
    { kategoriId:"isveren", tip:"kisi",  ad:"Fatma", soyad:"Yıldız", unvan:"Kontrol Mühendisi", firmaAdi:"", telefon:"0312 400 4010", email:"fatma.yildiz@isveren.gov.tr", notlar:"Hakediş onayı ve saha kontrolleri" },
    { kategoriId:"isveren", tip:"kisi",  ad:"Kemal", soyad:"Arslan", unvan:"İdare Amiri", firmaAdi:"", telefon:"0312 400 4020", email:"kemal.arslan@isveren.gov.tr", notlar:"" },
    { kategoriId:"musavir", tip:"firma", ad:"", soyad:"", unvan:"", firmaAdi:"Proje Müşavirlik A.Ş.", telefon:"0212 500 5000", email:"proje@musavirlik.com", notlar:"İnşaat denetimi ana firma" },
    { kategoriId:"musavir", tip:"kisi",  ad:"Seda", soyad:"Koç", unvan:"Başmüşavir", firmaAdi:"", telefon:"0532 501 5001", email:"seda.koc@musavirlik.com", notlar:"" },
    { kategoriId:"musavir", tip:"kisi",  ad:"Tolga", soyad:"Şahin", unvan:"Saha Denetçisi", firmaAdi:"", telefon:"0533 502 5002", email:"tolga.sahin@musavirlik.com", notlar:"Günlük rapor denetimi" },
    { kategoriId:"yuklenici", altKirilimId:"yk_pm", tip:"kisi", ad:"Murat", soyad:"Demir", unvan:"Proje Müdürü", firmaAdi:"", telefon:"0534 600 6000", email:"murat.demir@yuklenici.com", notlar:"" },
    { kategoriId:"yuklenici", altKirilimId:"yk_pm", tip:"kisi", ad:"Ayşe", soyad:"Güneş", unvan:"Şantiye Şefi", firmaAdi:"", telefon:"0535 601 6001", email:"ayse.gunes@yuklenici.com", notlar:"" },
    { kategoriId:"yuklenici", altKirilimId:"yk_pm", tip:"kisi", ad:"Hasan", soyad:"Çelikbaş", unvan:"İnşaat Mühendisi", firmaAdi:"", telefon:"0536 602 6002", email:"hasan.celikbas@yuklenici.com", notlar:"" },
    { kategoriId:"yuklenici", altKirilimId:"yk_idari", tip:"kisi", ad:"Pınar", soyad:"Altun", unvan:"Muhasebe Sorumlusu", firmaAdi:"", telefon:"0537 603 6003", email:"pinar.altun@yuklenici.com", notlar:"" },
    { kategoriId:"yuklenici", altKirilimId:"yk_idari", tip:"kisi", ad:"Orhan", soyad:"Kara", unvan:"Satınalma Uzmanı", firmaAdi:"", telefon:"0538 604 6004", email:"orhan.kara@yuklenici.com", notlar:"" },
    { kategoriId:"yuklenici", altKirilimId:"yk_mavi", tip:"kisi", ad:"İbrahim", soyad:"Yılmaz", unvan:"Ustabaşı (Kalıpçı)", firmaAdi:"", telefon:"0539 605 6005", email:"", notlar:"" },
    { kategoriId:"alt_yuklenici", altKirilimId:"ay_pm", tip:"kisi", ad:"Cengiz", soyad:"Ateş", unvan:"Alt Yüklenici PM", firmaAdi:"", telefon:"0530 700 7000", email:"cengiz.ates@altyk.com", notlar:"Kaba yapı taşeronu proje yöneticisi" },
    { kategoriId:"alt_yuklenici", altKirilimId:"ay_isg", tip:"kisi", ad:"Neslihan", soyad:"Polat", unvan:"İSG Uzmanı (A Sınıfı)", firmaAdi:"", telefon:"0531 701 7001", email:"neslihan.polat@altyk.com", notlar:"" },
    { kategoriId:"alt_yuklenici", altKirilimId:"ay_kefil", tip:"kisi", ad:"Recep", soyad:"Doğan", unvan:"Mali Kefil", firmaAdi:"", telefon:"0532 702 7002", email:"", notlar:"Kişisel kefalet sözleşmesi mevcut" },
    { kategoriId:"yapi_denetim", tip:"firma", ad:"", soyad:"", unvan:"", firmaAdi:"Güven Yapı Denetim Ltd. Şti.", telefon:"0224 800 8000", email:"bilgi@guvenyapidenetim.com", notlar:"Ruhsat numarası: YD-2025-0482" },
    { kategoriId:"yapi_denetim", tip:"kisi", ad:"Ercan", soyad:"Bal", unvan:"Kontrol Mühendisi (İnşaat)", firmaAdi:"", telefon:"0533 801 8001", email:"ercan.bal@guvenyapidenetim.com", notlar:"" },
    { kategoriId:"muellif", tip:"kisi", ad:"Prof. Dr. Zeynep", soyad:"Kurt", unvan:"Mimar (Proje Müellifi)", firmaAdi:"", telefon:"0212 900 9000", email:"zeynep.kurt@mimari.com", notlar:"Revizyon onayları için iletişime geçilecek" },
    { kategoriId:"muellif", tip:"firma", ad:"", soyad:"", unvan:"", firmaAdi:"Kurt & Ortakları Mimarlık", telefon:"0212 900 9001", email:"info@kurtortaklari.com", notlar:"" },
    { kategoriId:"danismanlar", tip:"kisi", ad:"Dr. Serhan", soyad:"Yiğit", unvan:"Jeoteknik Danışman", firmaAdi:"", telefon:"0532 950 9500", email:"serhan.yigit@geo.com", notlar:"Zemin raporu ve kazık dizaynı" },
    { kategoriId:"danismanlar", tip:"firma", ad:"", soyad:"", unvan:"", firmaAdi:"Tekno Çevre Dan. A.Ş.", telefon:"0312 951 9501", email:"proje@teknocevre.com", notlar:"Çevresel izin danışmanı" },
    { kategoriId:"isg_osgb", tip:"firma", ad:"", soyad:"", unvan:"", firmaAdi:"SafeWork OSGB A.Ş.", telefon:"0850 100 0100", email:"saha@safeworkosgb.com", notlar:"Aylık OSGB hizmet sözleşmesi" },
    { kategoriId:"isg_osgb", tip:"kisi", ad:"Ayhan", soyad:"Tunç", unvan:"Sorumlu İSG Uzmanı (A)", firmaAdi:"", telefon:"0532 100 0101", email:"ayhan.tunc@safeworkosgb.com", notlar:"Haftada 3 gün sahada" },
    { kategoriId:"tedarikciler", tip:"firma", ad:"", soyad:"", unvan:"", firmaAdi:"Akçansa Beton A.Ş.", telefon:"0850 200 0200", email:"satis@akcansa.com", notlar:"Hazır beton tedarikçisi — C30/37, C35/45" },
    { kategoriId:"tedarikciler", tip:"firma", ad:"", soyad:"", unvan:"", firmaAdi:"İçdaş Çelik San. A.Ş.", telefon:"0286 300 0300", email:"ticaret@icdas.com.tr", notlar:"Donatı çeliği B500C tedarikçisi" },
    { kategoriId:"tedarikciler", tip:"firma", ad:"", soyad:"", unvan:"", firmaAdi:"Knauf İnşaat Ürünleri", telefon:"0850 400 0400", email:"musteri@knauf.com.tr", notlar:"Alçı sıva ve kuru yapı sistemleri" },
    { kategoriId:"tedarikciler", tip:"kisi", ad:"Gürsel", soyad:"Ünsal", unvan:"Satış Temsilcisi", firmaAdi:"", telefon:"0532 401 0401", email:"gursel.unsal@knauf.com.tr", notlar:"Teknik destek ve malzeme takibi" },
  ];
}

// ── Yardımcı ─────────────────────────────────────────────────────────────
function uid() {
  return Math.random().toString(36).slice(2, 10);
}

function storageKey(pid: string) {
  return `ipks_paydaslar_${pid}`;
}

function loadFromStorage(pid: string): Paydas[] {
  try {
    const raw = localStorage.getItem(storageKey(pid));
    return raw ? JSON.parse(raw) : [];
  } catch {
    return [];
  }
}

function saveToStorage(pid: string, data: Paydas[]) {
  localStorage.setItem(storageKey(pid), JSON.stringify(data));
}

function kategoriLabel(id: string) {
  return KATEGORILER.find((k) => k.id === id)?.label ?? id;
}

function altKirilimLabel(katId: string, akId?: string) {
  if (!akId) return "";
  const kat = KATEGORILER.find((k) => k.id === katId);
  return kat?.altKirilimlar?.find((a) => a.id === akId)?.label ?? akId;
}

// ── Excel Export ──────────────────────────────────────────────────────────
function exportExcel(paydaslar: Paydas[], projeAdi: string) {
  const wb = XLSX.utils.book_new();

  KATEGORILER.forEach((kat) => {
    const rows = paydaslar.filter((p) => p.kategoriId === kat.id);
    const data = [
      ["Tip", "Ad", "Soyad", "Ünvan", "Firma Adı", "Alt Kırılım", "Telefon", "E-posta", "Notlar"],
      ...rows.map((p) => [
        p.tip === "kisi" ? "Kişi" : "Firma",
        p.ad,
        p.soyad ?? "",
        p.unvan ?? "",
        p.firmaAdi ?? "",
        altKirilimLabel(p.kategoriId, p.altKirilimId),
        p.telefon ?? "",
        p.email ?? "",
        p.notlar ?? "",
      ]),
    ];
    const ws = XLSX.utils.aoa_to_sheet(data);
    ws["!cols"] = [8, 15, 15, 15, 20, 25, 14, 22, 20].map((w) => ({ wch: w }));
    const sheetName = kat.label.slice(0, 31).replace(/[:\\/?*[\]]/g, "-");
    XLSX.utils.book_append_sheet(wb, ws, sheetName);
  });

  XLSX.writeFile(wb, `${projeAdi}_Paydaslar.xlsx`);
}

// ── Excel Şablon İndir ────────────────────────────────────────────────────
function downloadTemplate() {
  const wb = XLSX.utils.book_new();

  KATEGORILER.forEach((kat) => {
    const rows: string[][] = [
      ["Tip (Kişi/Firma)", "Ad", "Soyad", "Ünvan", "Firma Adı", "Alt Kırılım", "Telefon", "E-posta", "Notlar"],
    ];
    if (kat.altKirilimlar) {
      kat.altKirilimlar.forEach((ak) => {
        rows.push(["Kişi", "", "", "", "", ak.label, "", "", ""]);
      });
    } else {
      rows.push(["Kişi", "", "", "", "", "", "", "", ""]);
    }
    const ws = XLSX.utils.aoa_to_sheet(rows);
    ws["!cols"] = [12, 15, 15, 15, 20, 30, 14, 22, 20].map((w) => ({ wch: w }));
    const sheetName = kat.label.slice(0, 31).replace(/[:\\/?*[\]]/g, "-");
    XLSX.utils.book_append_sheet(wb, ws, sheetName);
  });

  XLSX.writeFile(wb, "IPKS_Paydaslar_Sablonu.xlsx");
}

// ── Excel Import ──────────────────────────────────────────────────────────
function importExcel(file: File, callback: (data: Paydas[]) => void) {
  const reader = new FileReader();
  reader.onload = (e) => {
    const data = new Uint8Array(e.target?.result as ArrayBuffer);
    const wb = XLSX.read(data, { type: "array" });
    const result: Paydas[] = [];

    wb.SheetNames.forEach((sheetName) => {
      const kat = KATEGORILER.find(
        (k) => k.label.slice(0, 31).replace(/[:\\/?*[\]]/g, "-") === sheetName
      );
      if (!kat) return;

      const ws = wb.Sheets[sheetName];
      const rows = XLSX.utils.sheet_to_json<string[]>(ws, { header: 1 }) as string[][];

      rows.slice(1).forEach((row) => {
        if (!row[1] && !row[4]) return; // Ad veya Firma Adı yoksa atla
        const altKirilimLabel = row[5] ?? "";
        const altKirilim = kat.altKirilimlar?.find((ak) => ak.label === altKirilimLabel);
        result.push({
          id: uid(),
          kategoriId: kat.id,
          altKirilimId: altKirilim?.id,
          tip: (row[0] ?? "").toLowerCase().includes("firma") ? "firma" : "kisi",
          ad: row[1] ?? "",
          soyad: row[2] ?? "",
          unvan: row[3] ?? "",
          firmaAdi: row[4] ?? "",
          telefon: row[6] ?? "",
          email: row[7] ?? "",
          notlar: row[8] ?? "",
        });
      });
    });

    callback(result);
  };
  reader.readAsArrayBuffer(file);
}

// ── Ana Bileşen ───────────────────────────────────────────────────────────
export default function ProjePaydaslariPage() {
  const { current } = useProjects();
  const pid = current?.id ?? "demo";

  const [paydaslar, setPaydaslar] = useState<Paydas[]>([]);
  const [aktifKat, setAktifKat] = useState<string | null>(null);
  const [formAcik, setFormAcik] = useState(false);
  const [form, setForm] = useState<Omit<Paydas, "id">>({ ...BOŞ_FORM });
  const [duzenleId, setDuzenleId] = useState<string | null>(null);
  const [arama, setArama] = useState("");
  const fileRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    setPaydaslar(loadFromStorage(pid));
  }, [pid]);

  function kaydet(data: Paydas[]) {
    setPaydaslar(data);
    saveToStorage(pid, data);
  }

  function formGonder() {
    if (!form.ad && !form.firmaAdi) return;
    if (duzenleId) {
      kaydet(paydaslar.map((p) => (p.id === duzenleId ? { ...form, id: duzenleId } : p)));
      setDuzenleId(null);
    } else {
      kaydet([...paydaslar, { ...form, id: uid() }]);
    }
    setForm({ ...BOŞ_FORM, kategoriId: form.kategoriId });
    setFormAcik(false);
  }

  function sil(id: string) {
    if (!confirm("Bu paydaşı silmek istediğinize emin misiniz?")) return;
    kaydet(paydaslar.filter((p) => p.id !== id));
  }

  function duzenle(p: Paydas) {
    setForm({ ...p });
    setDuzenleId(p.id);
    setAktifKat(p.kategoriId);
    setFormAcik(true);
  }

  function yeniEkle(katId: string) {
    setForm({ ...BOŞ_FORM, kategoriId: katId });
    setDuzenleId(null);
    setAktifKat(katId);
    setFormAcik(true);
  }

  const filtreliPaydaslar = paydaslar.filter((p) => {
    const q = arama.toLowerCase();
    if (!q) return true;
    return (
      p.ad.toLowerCase().includes(q) ||
      (p.soyad ?? "").toLowerCase().includes(q) ||
      (p.firmaAdi ?? "").toLowerCase().includes(q) ||
      (p.email ?? "").toLowerCase().includes(q)
    );
  });

  const secilenKat = KATEGORILER.find((k) => k.id === form.kategoriId);

  return (
    <div className="max-w-5xl mx-auto space-y-4">
      {/* Başlık */}
      <div className="flex items-center justify-between gap-3 flex-wrap">
        <h1 className="font-display font-extrabold text-xl text-white">Proje Paydaşları</h1>
        <div className="flex items-center gap-2 flex-wrap">
          <button
            onClick={downloadTemplate}
            className="rounded-md border border-beton-700 px-3 py-2 text-sm text-beton-200 hover:border-emniyet-500"
          >
            📥 Şablon İndir
          </button>
          <button
            onClick={() => fileRef.current?.click()}
            className="rounded-md border border-beton-700 px-3 py-2 text-sm text-beton-200 hover:border-emniyet-500"
          >
            📤 Excel Yükle
          </button>
          <input
            ref={fileRef}
            type="file"
            accept=".xlsx,.xls"
            className="hidden"
            onChange={(e) => {
              const file = e.target.files?.[0];
              if (!file) return;
              importExcel(file, (data) => {
                if (confirm(`${data.length} paydaş bulundu. Mevcut verilerle birleştirilsin mi?`)) {
                  kaydet([...paydaslar, ...data]);
                } else {
                  kaydet(data);
                }
              });
              e.target.value = "";
            }}
          />
          {paydaslar.length > 0 && (
            <button
              onClick={() => exportExcel(paydaslar, current?.name ?? "Proje")}
              className="rounded-md border border-beton-700 px-3 py-2 text-sm text-beton-200 hover:border-emniyet-500"
            >
              📊 Excel Dışa Aktar
            </button>
          )}
        </div>
      </div>

      {/* Demo Veri Yükle — yalnızca liste boşken */}
      {paydaslar.length === 0 && (
        <div className="rounded-lg border border-beton-700 bg-beton-900/50 px-4 py-3 flex items-center justify-between gap-3">
          <p className="text-sm text-beton-400">Henüz paydaş eklenmemiş. Demo veriyle başlamak ister misiniz?</p>
          <button
            onClick={() => {
              if (!confirm("Bu proje için 27 demo paydaş yüklenecek. Devam edilsin mi?")) return;
              const data = demoPaydaslar().map((p) => ({ ...p, id: uid() }));
              kaydet(data);
            }}
            className="shrink-0 rounded-md bg-beton-700 px-3 py-1.5 text-xs font-medium text-beton-100 hover:bg-beton-600"
          >
            Demo Veri Yükle
          </button>
        </div>
      )}

      {/* Arama */}
      {paydaslar.length > 0 && (
        <input
          type="text"
          placeholder="Paydaş ara..."
          value={arama}
          onChange={(e) => setArama(e.target.value)}
          className="w-full rounded-md bg-beton-950 border border-beton-800 px-3 py-2 text-sm text-beton-100 outline-none focus:border-emniyet-500"
        />
      )}

      {/* Kategori Kartları */}
      <div className="space-y-3">
        {KATEGORILER.map((kat) => {
          const katPaydaslar = filtreliPaydaslar.filter((p) => p.kategoriId === kat.id);
          const acik = aktifKat === kat.id;

          return (
            <div key={kat.id} className="rounded-lg border border-beton-800 bg-beton-900 overflow-hidden">
              {/* Kategori Başlığı */}
              <div
                className="flex items-center justify-between px-4 py-3 cursor-pointer hover:bg-beton-800/50"
                onClick={() => setAktifKat(acik ? null : kat.id)}
              >
                <div className="flex items-center gap-3">
                  <span className="text-lg">{kat.icon}</span>
                  <span className="font-medium text-white text-sm">{kat.label}</span>
                  {katPaydaslar.length > 0 && (
                    <span className="rounded-full bg-emniyet-500/20 text-emniyet-500 text-xs px-2 py-0.5">
                      {katPaydaslar.length}
                    </span>
                  )}
                </div>
                <div className="flex items-center gap-2">
                  <button
                    onClick={(e) => { e.stopPropagation(); yeniEkle(kat.id); }}
                    className="rounded-md bg-emniyet-500 px-3 py-1 text-xs font-medium text-beton-950 hover:brightness-110"
                  >
                    + Ekle
                  </button>
                  <span className="text-beton-400 text-xs">{acik ? "▲" : "▼"}</span>
                </div>
              </div>

              {/* Paydaş Listesi */}
              {acik && (
                <div className="border-t border-beton-800">
                  {katPaydaslar.length === 0 ? (
                    <p className="px-4 py-3 text-beton-400 text-sm">Henüz paydaş eklenmemiş.</p>
                  ) : (
                    <table className="w-full text-sm">
                      <thead>
                        <tr className="text-beton-400 text-left border-b border-beton-800 text-xs">
                          <th className="px-4 py-2">Ad Soyad / Firma</th>
                          <th className="px-4 py-2">Ünvan</th>
                          {kat.altKirilimlar && <th className="px-4 py-2">Alt Kırılım</th>}
                          <th className="px-4 py-2">Telefon</th>
                          <th className="px-4 py-2">E-posta</th>
                          <th className="px-4 py-2"></th>
                        </tr>
                      </thead>
                      <tbody>
                        {katPaydaslar.map((p) => (
                          <tr key={p.id} className="border-b border-beton-800/50 text-beton-200">
                            <td className="px-4 py-2">
                              {p.tip === "firma" ? (
                                <span className="font-medium">{p.firmaAdi}</span>
                              ) : (
                                <span>{p.ad} {p.soyad}</span>
                              )}
                            </td>
                            <td className="px-4 py-2 text-beton-400">{p.unvan}</td>
                            {kat.altKirilimlar && (
                              <td className="px-4 py-2 text-beton-400 text-xs">
                                {altKirilimLabel(p.kategoriId, p.altKirilimId)}
                              </td>
                            )}
                            <td className="px-4 py-2 text-beton-400">{p.telefon}</td>
                            <td className="px-4 py-2 text-beton-400">{p.email}</td>
                            <td className="px-4 py-2 text-right">
                              <button onClick={() => duzenle(p)} className="text-emniyet-500 hover:underline text-xs mr-3">Düzenle</button>
                              <button onClick={() => sil(p.id)} className="text-red-400 hover:underline text-xs">Sil</button>
                            </td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  )}
                </div>
              )}
            </div>
          );
        })}
      </div>

      {/* Form Modal */}
      {formAcik && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60" onClick={() => setFormAcik(false)}>
          <div
            className="bg-beton-900 border border-beton-700 rounded-xl w-full max-w-lg mx-4 p-6 space-y-4 max-h-[90vh] overflow-y-auto"
            onClick={(e) => e.stopPropagation()}
          >
            <h2 className="font-display font-bold text-white text-lg">
              {duzenleId ? "Paydaş Düzenle" : "Paydaş Ekle"}
            </h2>

            {/* Kategori Seç */}
            <div>
              <label className="block text-xs text-beton-400 mb-1">Kategori</label>
              <select
                value={form.kategoriId}
                onChange={(e) => setForm({ ...form, kategoriId: e.target.value, altKirilimId: "" })}
                className="w-full rounded-md bg-beton-950 border border-beton-800 px-3 py-2 text-sm text-beton-100 outline-none focus:border-emniyet-500"
              >
                <option value="">Kategori seçin...</option>
                {KATEGORILER.map((k) => (
                  <option key={k.id} value={k.id}>{k.icon} {k.label}</option>
                ))}
              </select>
            </div>

            {/* Alt Kırılım (varsa) */}
            {secilenKat?.altKirilimlar && (
              <div>
                <label className="block text-xs text-beton-400 mb-1">Alt Kırılım</label>
                <select
                  value={form.altKirilimId}
                  onChange={(e) => setForm({ ...form, altKirilimId: e.target.value })}
                  className="w-full rounded-md bg-beton-950 border border-beton-800 px-3 py-2 text-sm text-beton-100 outline-none focus:border-emniyet-500"
                >
                  <option value="">Alt kırılım seçin...</option>
                  {secilenKat.altKirilimlar.map((ak) => (
                    <option key={ak.id} value={ak.id}>{ak.label}</option>
                  ))}
                </select>
              </div>
            )}

            {/* Tip */}
            <div className="flex gap-3">
              {(["kisi", "firma"] as const).map((t) => (
                <button
                  key={t}
                  onClick={() => setForm({ ...form, tip: t })}
                  className={`flex-1 rounded-md border py-2 text-sm font-medium transition ${
                    form.tip === t
                      ? "border-emniyet-500 bg-emniyet-500/10 text-emniyet-500"
                      : "border-beton-700 text-beton-400 hover:border-beton-500"
                  }`}
                >
                  {t === "kisi" ? "👤 Kişi" : "🏢 Firma"}
                </button>
              ))}
            </div>

            {/* Firma Adı (firma ise) */}
            {form.tip === "firma" && (
              <div>
                <label className="block text-xs text-beton-400 mb-1">Firma Adı *</label>
                <input
                  value={form.firmaAdi}
                  onChange={(e) => setForm({ ...form, firmaAdi: e.target.value })}
                  className="w-full rounded-md bg-beton-950 border border-beton-800 px-3 py-2 text-sm text-beton-100 outline-none focus:border-emniyet-500"
                  placeholder="Firma adı"
                />
              </div>
            )}

            {/* Ad / Soyad */}
            <div className="grid grid-cols-2 gap-3">
              <div>
                <label className="block text-xs text-beton-400 mb-1">
                  {form.tip === "firma" ? "Yetkili Adı" : "Ad *"}
                </label>
                <input
                  value={form.ad}
                  onChange={(e) => setForm({ ...form, ad: e.target.value })}
                  className="w-full rounded-md bg-beton-950 border border-beton-800 px-3 py-2 text-sm text-beton-100 outline-none focus:border-emniyet-500"
                  placeholder="Ad"
                />
              </div>
              <div>
                <label className="block text-xs text-beton-400 mb-1">Soyad</label>
                <input
                  value={form.soyad}
                  onChange={(e) => setForm({ ...form, soyad: e.target.value })}
                  className="w-full rounded-md bg-beton-950 border border-beton-800 px-3 py-2 text-sm text-beton-100 outline-none focus:border-emniyet-500"
                  placeholder="Soyad"
                />
              </div>
            </div>

            {/* Ünvan */}
            <div>
              <label className="block text-xs text-beton-400 mb-1">Ünvan / Görev</label>
              <input
                value={form.unvan}
                onChange={(e) => setForm({ ...form, unvan: e.target.value })}
                className="w-full rounded-md bg-beton-950 border border-beton-800 px-3 py-2 text-sm text-beton-100 outline-none focus:border-emniyet-500"
                placeholder="Proje Müdürü, Şantiye Şefi..."
              />
            </div>

            {/* Telefon / E-posta */}
            <div className="grid grid-cols-2 gap-3">
              <div>
                <label className="block text-xs text-beton-400 mb-1">Telefon</label>
                <input
                  value={form.telefon}
                  onChange={(e) => setForm({ ...form, telefon: e.target.value })}
                  className="w-full rounded-md bg-beton-950 border border-beton-800 px-3 py-2 text-sm text-beton-100 outline-none focus:border-emniyet-500"
                  placeholder="0532 xxx xx xx"
                />
              </div>
              <div>
                <label className="block text-xs text-beton-400 mb-1">E-posta</label>
                <input
                  value={form.email}
                  onChange={(e) => setForm({ ...form, email: e.target.value })}
                  className="w-full rounded-md bg-beton-950 border border-beton-800 px-3 py-2 text-sm text-beton-100 outline-none focus:border-emniyet-500"
                  placeholder="ornek@firma.com"
                />
              </div>
            </div>

            {/* Notlar */}
            <div>
              <label className="block text-xs text-beton-400 mb-1">Notlar</label>
              <textarea
                value={form.notlar}
                onChange={(e) => setForm({ ...form, notlar: e.target.value })}
                rows={2}
                className="w-full rounded-md bg-beton-950 border border-beton-800 px-3 py-2 text-sm text-beton-100 outline-none focus:border-emniyet-500 resize-none"
                placeholder="Ek bilgi..."
              />
            </div>

            {/* Butonlar */}
            <div className="flex justify-end gap-3 pt-2">
              <button
                onClick={() => { setFormAcik(false); setDuzenleId(null); }}
                className="rounded-md border border-beton-700 px-4 py-2 text-sm text-beton-300 hover:border-beton-500"
              >
                İptal
              </button>
              <button
                onClick={formGonder}
                disabled={!form.kategoriId || (!form.ad && !form.firmaAdi)}
                className="rounded-md bg-emniyet-500 px-4 py-2 text-sm font-medium text-beton-950 hover:brightness-110 disabled:opacity-50"
              >
                {duzenleId ? "Güncelle" : "Kaydet"}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
