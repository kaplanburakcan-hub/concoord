import { useEffect, useRef, useState } from "react";
import { useProjects } from "../../projects/ProjectContext";

// ── Tipler ───────────────────────────────────────────────────────────
type TutanakTip =
  | "kaza_yangin_hirsizlik"
  | "ek_imalat"
  | "mesai"
  | "yevmiyeli";

type OnayAdim = {
  rol: string;
  ad: string;
  durum: "bekliyor" | "onaylandi" | "reddedildi";
  tarih?: string;
  not?: string;
};

type Fotograf = {
  id: string;
  ad: string;
  base64: string;
  tarih: string;
};

type Tutanak = {
  id: string;
  tip: TutanakTip;
  baslik: string;
  tarih: string;
  taseron_id?: string;
  taseron_adi?: string;
  kisim?: string;
  aciklama: string;
  tutar?: number;
  birim?: string;
  miktar?: number;
  durum: "taslak" | "onay_sureci" | "onaylandi" | "reddedildi";
  onay_zinciri: OnayAdim[];
  fotograflar: Fotograf[];
  hakedise_eklendi: boolean;
  olusturan: string;
  olusturma_tarihi: string;
};

const TIP_LABEL: Record<TutanakTip, string> = {
  kaza_yangin_hirsizlik: "Kaza / Yangın / Hırsızlık",
  ek_imalat: "Ek İmalat Tutanağı",
  mesai: "Mesai Tutanağı",
  yevmiyeli: "Yevmiyeli Çalışma Tutanağı",
};

const TIP_ICON: Record<TutanakTip, string> = {
  kaza_yangin_hirsizlik: "🚨",
  ek_imalat: "🔨",
  mesai: "⏰",
  yevmiyeli: "👷",
};

const TIP_HAKEDIS: Record<TutanakTip, boolean> = {
  kaza_yangin_hirsizlik: false,
  ek_imalat: true,
  mesai: true,
  yevmiyeli: true,
};

const KISIMLAR = ["İnşaat", "Elektrik", "Mekanik", "Peyzaj", "Altyapı", "Çelik", "Diğer"];

const DURUM_LABEL: Record<string, string> = {
  taslak: "Taslak",
  onay_sureci: "Onay Sürecinde",
  onaylandi: "Onaylandı",
  reddedildi: "Reddedildi",
};

const DURUM_STYLE: Record<string, string> = {
  taslak: "bg-beton-800 text-beton-200 border-beton-700",
  onay_sureci: "bg-blue-500/15 text-blue-300 border-blue-500/40",
  onaylandi: "bg-green-500/15 text-green-300 border-green-500/40",
  reddedildi: "bg-red-500/15 text-red-300 border-red-500/40",
};

function uid() { return Math.random().toString(36).slice(2, 10); }
function storageKey(pid: string) { return `ipks_saha_tutanaklar_${pid}`; }
function load(pid: string): Tutanak[] {
  try { return JSON.parse(localStorage.getItem(storageKey(pid)) || "[]"); } catch { return []; }
}
function save(pid: string, data: Tutanak[]) {
  localStorage.setItem(storageKey(pid), JSON.stringify(data));
}
function onayZinciriOlustur(kisimVar: boolean): OnayAdim[] {
  const zincir: OnayAdim[] = [];
  if (kisimVar) zincir.push({ rol: "Kısım Şefi", ad: "", durum: "bekliyor" });
  zincir.push({ rol: "Şantiye Şefi", ad: "", durum: "bekliyor" });
  zincir.push({ rol: "Proje Müdürü", ad: "", durum: "bekliyor" });
  return zincir;
}

function fileToBase64(file: File): Promise<string> {
  return new Promise((res, rej) => {
    const r = new FileReader();
    r.onload = () => res(r.result as string);
    r.onerror = () => rej(new Error("Dosya okunamadı"));
    r.readAsDataURL(file);
  });
}

const BOŞ_FORM = {
  tip: "ek_imalat" as TutanakTip,
  baslik: "",
  tarih: new Date().toISOString().slice(0, 10),
  taseron_id: "",
  kisim: "",
  aciklama: "",
  tutar: "",
  birim: "adet",
  miktar: "",
  kisim_sefi_var: false,
};

// ── Ana Bileşen ───────────────────────────────────────────────────────
export default function SahaTutanaklariPage() {
  const { current } = useProjects();
  const pid = current?.id ?? "demo";

  const [tutanaklar, setTutanaklar] = useState<Tutanak[]>([]);
  const [formAcik, setFormAcik] = useState(false);
  const [form, setForm] = useState({ ...BOŞ_FORM });
  const [formFotograflar, setFormFotograflar] = useState<Fotograf[]>([]);
  const [secili, setSecili] = useState<Tutanak | null>(null);
  const [onayModal, setOnayModal] = useState(false);
  const [onayNot, setOnayNot] = useState("");
  const [filtre, setFiltre] = useState<TutanakTip | "hepsi">("hepsi");
  const [fotografBuyuk, setFotografBuyuk] = useState<Fotograf | null>(null);
  const [fotYukleniyor, setFotYukleniyor] = useState(false);

  const formFotoRef = useRef<HTMLInputElement>(null);
  const detayFotoRef = useRef<HTMLInputElement>(null);

  useEffect(() => { setTutanaklar(load(pid)); }, [pid]);

  function kaydet(data: Tutanak[]) {
    setTutanaklar(data);
    save(pid, data);
  }

  // Form fotoğraf ekle
  async function formFotoEkle(files: FileList | null) {
    if (!files) return;
    setFotYukleniyor(true);
    const yeniler: Fotograf[] = [];
    for (const file of Array.from(files)) {
      if (file.size > 10 * 1024 * 1024) { alert(`${file.name} 10MB sınırını aşıyor.`); continue; }
      const base64 = await fileToBase64(file);
      yeniler.push({ id: uid(), ad: file.name, base64, tarih: new Date().toISOString() });
    }
    setFormFotograflar(prev => [...prev, ...yeniler]);
    setFotYukleniyor(false);
    if (formFotoRef.current) formFotoRef.current.value = "";
  }

  // Detay panelinden mevcut tutanağa fotoğraf ekle
  async function detayFotoEkle(files: FileList | null) {
    if (!files || !secili) return;
    setFotYukleniyor(true);
    const yeniler: Fotograf[] = [];
    for (const file of Array.from(files)) {
      if (file.size > 10 * 1024 * 1024) { alert(`${file.name} 10MB sınırını aşıyor.`); continue; }
      const base64 = await fileToBase64(file);
      yeniler.push({ id: uid(), ad: file.name, base64, tarih: new Date().toISOString() });
    }
    const guncellendi = { ...secili, fotograflar: [...(secili.fotograflar || []), ...yeniler] };
    kaydet(tutanaklar.map(t => t.id === secili.id ? guncellendi : t));
    setSecili(guncellendi);
    setFotYukleniyor(false);
    if (detayFotoRef.current) detayFotoRef.current.value = "";
  }

  function formFotoSil(id: string) {
    setFormFotograflar(prev => prev.filter(f => f.id !== id));
  }

  function detayFotoSil(fotoId: string) {
    if (!secili) return;
    const guncellendi = { ...secili, fotograflar: secili.fotograflar.filter(f => f.id !== fotoId) };
    kaydet(tutanaklar.map(t => t.id === secili.id ? guncellendi : t));
    setSecili(guncellendi);
  }

  function tutanakOlustur() {
    if (!form.baslik.trim() || !form.aciklama.trim()) return;
    const zincir = onayZinciriOlustur(form.kisim_sefi_var);
    const yeni: Tutanak = {
      id: uid(),
      tip: form.tip,
      baslik: form.baslik.trim(),
      tarih: form.tarih,
      taseron_id: form.taseron_id || undefined,
      taseron_adi: form.taseron_id ? `Taşeron ${form.taseron_id.slice(0, 4)}` : undefined,
      kisim: form.kisim || undefined,
      aciklama: form.aciklama.trim(),
      tutar: form.tutar ? Number(form.tutar) : undefined,
      birim: form.birim || undefined,
      miktar: form.miktar ? Number(form.miktar) : undefined,
      durum: "taslak",
      onay_zinciri: zincir,
      fotograflar: formFotograflar,
      hakedise_eklendi: false,
      olusturan: "Sistem Yöneticisi",
      olusturma_tarihi: new Date().toISOString(),
    };
    kaydet([yeni, ...tutanaklar]);
    setForm({ ...BOŞ_FORM });
    setFormFotograflar([]);
    setFormAcik(false);
  }

  function onayaSuncu(t: Tutanak) {
    const guncellendi = tutanaklar.map(x =>
      x.id === t.id ? { ...x, durum: "onay_sureci" as const } : x
    );
    kaydet(guncellendi);
    setSecili({ ...t, durum: "onay_sureci" });
  }

  function onayVer(t: Tutanak, karar: "onaylandi" | "reddedildi") {
    const bekleyenIdx = t.onay_zinciri.findIndex(a => a.durum === "bekliyor");
    if (bekleyenIdx === -1) return;
    const yeniZincir = t.onay_zinciri.map((a, i) =>
      i === bekleyenIdx ? { ...a, durum: karar, tarih: new Date().toISOString(), not: onayNot } : a
    );
    const hepsiOnaylandi = yeniZincir.every(a => a.durum === "onaylandi");
    const biriReddetti = yeniZincir.some(a => a.durum === "reddedildi");
    const yeniDurum = biriReddetti ? "reddedildi" : hepsiOnaylandi ? "onaylandi" : "onay_sureci";
    const guncellendi: Tutanak = {
      ...t, onay_zinciri: yeniZincir, durum: yeniDurum,
      hakedise_eklendi: hepsiOnaylandi && TIP_HAKEDIS[t.tip],
    };
    kaydet(tutanaklar.map(x => x.id === t.id ? guncellendi : x));
    setSecili(guncellendi);
    setOnayModal(false);
    setOnayNot("");
  }

  function sil(id: string) {
    if (!confirm("Bu tutanağı silmek istediğinize emin misiniz?")) return;
    kaydet(tutanaklar.filter(t => t.id !== id));
    if (secili?.id === id) setSecili(null);
  }

  const filtreliler = filtre === "hepsi" ? tutanaklar : tutanaklar.filter(t => t.tip === filtre);
  const bekleyenOnay = secili ? secili.onay_zinciri.findIndex(a => a.durum === "bekliyor") : -1;

  if (!current) return <p className="text-beton-400 text-sm">Önce üst bardan bir proje seçin.</p>;

  return (
    <div className="max-w-5xl mx-auto space-y-4">
      {/* Başlık */}
      <div className="flex items-center justify-between gap-3 flex-wrap">
        <h1 className="font-display font-extrabold text-xl text-white">Saha Tutanakları</h1>
        <button
          onClick={() => setFormAcik(true)}
          className="rounded-md bg-emniyet-500 px-3 py-2 text-sm font-medium text-beton-950 hover:brightness-110"
        >
          + Yeni Tutanak
        </button>
      </div>

      {/* Filtre */}
      <div className="flex gap-2 flex-wrap">
        {(["hepsi", ...Object.keys(TIP_LABEL)] as const).map((tip) => (
          <button key={tip} onClick={() => setFiltre(tip as typeof filtre)}
            className={`rounded-full px-3 py-1 text-xs border transition ${
              filtre === tip ? "bg-emniyet-500 text-beton-950 border-emniyet-500 font-semibold"
              : "border-beton-700 text-beton-400 hover:border-beton-500"
            }`}
          >
            {tip === "hepsi" ? "Tümü" : `${TIP_ICON[tip as TutanakTip]} ${TIP_LABEL[tip as TutanakTip]}`}
          </button>
        ))}
      </div>

      <div className="grid md:grid-cols-[1fr_380px] gap-4">
        {/* Liste */}
        <div className="space-y-2">
          {filtreliler.length === 0 && <p className="text-beton-400 text-sm">Henüz tutanak yok.</p>}
          {filtreliler.map((t) => (
            <div key={t.id} onClick={() => setSecili(t)}
              className={`rounded-lg border p-4 cursor-pointer transition ${
                secili?.id === t.id ? "border-emniyet-500 bg-beton-800" : "border-beton-800 bg-beton-900 hover:border-beton-600"
              }`}
            >
              <div className="flex items-center justify-between gap-2 flex-wrap">
                <div className="flex items-center gap-2">
                  <span>{TIP_ICON[t.tip]}</span>
                  <span className="font-medium text-white text-sm">{t.baslik}</span>
                  {t.fotograflar?.length > 0 && (
                    <span className="text-xs text-beton-400">📷 {t.fotograflar.length}</span>
                  )}
                </div>
                <div className="flex items-center gap-2">
                  {t.hakedise_eklendi && (
                    <span className="text-xs bg-green-500/20 text-green-300 border border-green-500/40 rounded-full px-2 py-0.5">✓ Hakediş</span>
                  )}
                  <span className={`rounded-full border px-2 py-0.5 text-xs ${DURUM_STYLE[t.durum]}`}>
                    {DURUM_LABEL[t.durum]}
                  </span>
                </div>
              </div>
              <div className="mt-1 text-xs text-beton-400 flex flex-wrap gap-x-4">
                <span>{TIP_LABEL[t.tip]}</span>
                <span>{t.tarih}</span>
                {t.kisim && <span>{t.kisim}</span>}
                {t.tutar && <span>{t.tutar.toLocaleString("tr-TR")} TL</span>}
              </div>
              <div className="mt-2 flex gap-1">
                {t.onay_zinciri.map((a, i) => (
                  <span key={i} className={`text-[10px] rounded px-1.5 py-0.5 border ${
                    a.durum === "onaylandi" ? "bg-green-500/10 text-green-300 border-green-500/30"
                    : a.durum === "reddedildi" ? "bg-red-500/10 text-red-300 border-red-500/30"
                    : "bg-beton-800 text-beton-400 border-beton-700"
                  }`}>{a.rol}</span>
                ))}
              </div>
            </div>
          ))}
        </div>

        {/* Detay Paneli */}
        {secili && (
          <div className="rounded-lg border border-beton-800 bg-beton-900 p-4 space-y-4 h-fit">
            <div className="flex items-start justify-between gap-2">
              <div>
                <div className="text-lg">{TIP_ICON[secili.tip]}</div>
                <h2 className="font-bold text-white mt-1">{secili.baslik}</h2>
                <p className="text-xs text-beton-400">{TIP_LABEL[secili.tip]} · {secili.tarih}</p>
              </div>
              <span className={`rounded-full border px-2 py-0.5 text-xs ${DURUM_STYLE[secili.durum]}`}>
                {DURUM_LABEL[secili.durum]}
              </span>
            </div>

            {secili.kisim && <p className="text-sm text-beton-300">Kısım: <span className="text-white">{secili.kisim}</span></p>}
            {secili.taseron_adi && <p className="text-sm text-beton-300">Taşeron: <span className="text-white">{secili.taseron_adi}</span></p>}
            <p className="text-sm text-beton-200">{secili.aciklama}</p>

            {(secili.tutar || secili.miktar) && (
              <div className="rounded-md bg-beton-800 p-3 space-y-1 text-sm">
                {secili.miktar && <p className="text-beton-300">Miktar: <span className="text-white">{secili.miktar} {secili.birim}</span></p>}
                {secili.tutar && <p className="text-beton-300">Tutar: <span className="text-white font-semibold">{secili.tutar.toLocaleString("tr-TR")} TL</span></p>}
                {TIP_HAKEDIS[secili.tip] && (
                  <p className="text-xs text-emniyet-500 mt-1">
                    {secili.hakedise_eklendi ? "✓ Taşeron hakediş ilave işlerine eklendi" : "Onaylanınca hakediş ilave işlerine eklenecek"}
                  </p>
                )}
              </div>
            )}

            {/* Fotoğraflar */}
            <div>
              <div className="flex items-center justify-between mb-2">
                <p className="text-xs text-beton-400 uppercase tracking-wide">Fotoğraflar ({secili.fotograflar?.length || 0})</p>
                <div className="flex gap-2">
                  <input ref={detayFotoRef} type="file" accept="image/*" multiple capture="environment"
                    className="hidden"
                    onChange={e => detayFotoEkle(e.target.files)}
                  />
                  <button onClick={() => detayFotoRef.current?.click()}
                    disabled={fotYukleniyor}
                    className="text-xs rounded border border-beton-700 px-2 py-1 text-beton-300 hover:border-emniyet-500 disabled:opacity-50"
                  >
                    {fotYukleniyor ? "Yükleniyor..." : "📷 Fotoğraf Ekle"}
                  </button>
                </div>
              </div>
              {secili.fotograflar?.length > 0 ? (
                <div className="grid grid-cols-3 gap-2">
                  {secili.fotograflar.map((f) => (
                    <div key={f.id} className="relative group">
                      <img
                        src={f.base64}
                        alt={f.ad}
                        className="w-full h-20 object-cover rounded-md cursor-pointer border border-beton-700 hover:border-emniyet-500"
                        onClick={() => setFotografBuyuk(f)}
                      />
                      <button
                        onClick={() => detayFotoSil(f.id)}
                        className="absolute top-1 right-1 bg-red-600 text-white rounded-full w-4 h-4 text-[10px] flex items-center justify-center opacity-0 group-hover:opacity-100 transition"
                      >✕</button>
                    </div>
                  ))}
                </div>
              ) : (
                <p className="text-xs text-beton-500">Henüz fotoğraf yok.</p>
              )}
            </div>

            {/* Onay Zinciri */}
            <div>
              <p className="text-xs text-beton-400 mb-2 uppercase tracking-wide">Onay Zinciri</p>
              <div className="space-y-2">
                {secili.onay_zinciri.map((a, i) => (
                  <div key={i} className={`flex items-center gap-3 rounded-md p-2 text-sm ${
                    a.durum === "onaylandi" ? "bg-green-500/10"
                    : a.durum === "reddedildi" ? "bg-red-500/10"
                    : "bg-beton-800"
                  }`}>
                    <span className={`text-lg ${
                      a.durum === "onaylandi" ? "text-green-400"
                      : a.durum === "reddedildi" ? "text-red-400"
                      : "text-beton-500"
                    }`}>
                      {a.durum === "onaylandi" ? "✓" : a.durum === "reddedildi" ? "✕" : "○"}
                    </span>
                    <div className="flex-1">
                      <p className="text-white font-medium">{a.rol}</p>
                      {a.tarih && <p className="text-xs text-beton-400">{new Date(a.tarih).toLocaleString("tr-TR")}</p>}
                      {a.not && <p className="text-xs text-beton-300 italic">{a.not}</p>}
                    </div>
                  </div>
                ))}
              </div>
            </div>

            {/* Aksiyonlar */}
            <div className="flex flex-wrap gap-2 pt-2 border-t border-beton-800">
              {secili.durum === "taslak" && (
                <button onClick={() => onayaSuncu(secili)}
                  className="rounded-md bg-emniyet-500 px-3 py-1.5 text-xs font-medium text-beton-950 hover:brightness-110"
                >Onaya Sun</button>
              )}
              {secili.durum === "onay_sureci" && bekleyenOnay !== -1 && (
                <button onClick={() => setOnayModal(true)}
                  className="rounded-md bg-emniyet-500 px-3 py-1.5 text-xs font-medium text-beton-950 hover:brightness-110"
                >Onay Ver / Reddet</button>
              )}
              {secili.durum === "taslak" && (
                <button onClick={() => sil(secili.id)}
                  className="rounded-md border border-red-500/40 text-red-400 px-3 py-1.5 text-xs hover:border-red-400"
                >Sil</button>
              )}
            </div>
          </div>
        )}
      </div>

      {/* Yeni Tutanak Formu */}
      {formAcik && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60" onClick={() => setFormAcik(false)}>
          <div className="bg-beton-900 border border-beton-700 rounded-xl w-full max-w-lg mx-4 p-6 space-y-4 max-h-[90vh] overflow-y-auto"
            onClick={e => e.stopPropagation()}
          >
            <h2 className="font-display font-bold text-white text-lg">Yeni Tutanak</h2>

            {/* Tip */}
            <div>
              <label className="block text-xs text-beton-400 mb-1">Tutanak Tipi *</label>
              <div className="grid grid-cols-2 gap-2">
                {(Object.keys(TIP_LABEL) as TutanakTip[]).map((tip) => (
                  <button key={tip} onClick={() => setForm({ ...form, tip })}
                    className={`rounded-md border p-2 text-xs text-left transition ${
                      form.tip === tip ? "border-emniyet-500 bg-emniyet-500/10 text-emniyet-500"
                      : "border-beton-700 text-beton-400 hover:border-beton-500"
                    }`}
                  >
                    {TIP_ICON[tip]} {TIP_LABEL[tip]}
                    {TIP_HAKEDIS[tip] && <span className="block text-[10px] text-beton-500 mt-0.5">Hakediş'e etki eder</span>}
                  </button>
                ))}
              </div>
            </div>

            {/* Başlık */}
            <div>
              <label className="block text-xs text-beton-400 mb-1">Başlık *</label>
              <input value={form.baslik} onChange={e => setForm({ ...form, baslik: e.target.value })}
                className="w-full rounded-md bg-beton-950 border border-beton-800 px-3 py-2 text-sm text-beton-100 outline-none focus:border-emniyet-500"
                placeholder="Tutanak başlığı"
              />
            </div>

            {/* Tarih */}
            <div>
              <label className="block text-xs text-beton-400 mb-1">Tarih *</label>
              <input type="date" value={form.tarih} onChange={e => setForm({ ...form, tarih: e.target.value })}
                className="w-full rounded-md bg-beton-950 border border-beton-800 px-3 py-2 text-sm text-beton-100 outline-none focus:border-emniyet-500"
              />
            </div>

            {/* Kısım */}
            <div>
              <label className="block text-xs text-beton-400 mb-1">Kısım</label>
              <select value={form.kisim} onChange={e => setForm({ ...form, kisim: e.target.value })}
                className="w-full rounded-md bg-beton-950 border border-beton-800 px-3 py-2 text-sm text-beton-100 outline-none focus:border-emniyet-500"
              >
                <option value="">Seçin (opsiyonel)</option>
                {KISIMLAR.map(k => <option key={k} value={k}>{k}</option>)}
              </select>
            </div>

            {/* Kısım Şefi */}
            <div className="flex items-center gap-2">
              <input type="checkbox" id="kisim_sefi" checked={form.kisim_sefi_var}
                onChange={e => setForm({ ...form, kisim_sefi_var: e.target.checked })}
                className="rounded"
              />
              <label htmlFor="kisim_sefi" className="text-sm text-beton-300">Kısım Şefi onayı gerekli</label>
            </div>

            {/* Tutar / Miktar */}
            {TIP_HAKEDIS[form.tip] && (
              <div className="grid grid-cols-3 gap-2">
                <div>
                  <label className="block text-xs text-beton-400 mb-1">Miktar</label>
                  <input type="number" value={form.miktar} onChange={e => setForm({ ...form, miktar: e.target.value })}
                    className="w-full rounded-md bg-beton-950 border border-beton-800 px-3 py-2 text-sm text-beton-100 outline-none focus:border-emniyet-500"
                    placeholder="0"
                  />
                </div>
                <div>
                  <label className="block text-xs text-beton-400 mb-1">Birim</label>
                  <input value={form.birim} onChange={e => setForm({ ...form, birim: e.target.value })}
                    className="w-full rounded-md bg-beton-950 border border-beton-800 px-3 py-2 text-sm text-beton-100 outline-none focus:border-emniyet-500"
                    placeholder="adet"
                  />
                </div>
                <div>
                  <label className="block text-xs text-beton-400 mb-1">Tutar (TL)</label>
                  <input type="number" value={form.tutar} onChange={e => setForm({ ...form, tutar: e.target.value })}
                    className="w-full rounded-md bg-beton-950 border border-beton-800 px-3 py-2 text-sm text-beton-100 outline-none focus:border-emniyet-500"
                    placeholder="0"
                  />
                </div>
              </div>
            )}

            {/* Açıklama */}
            <div>
              <label className="block text-xs text-beton-400 mb-1">Açıklama *</label>
              <textarea value={form.aciklama} onChange={e => setForm({ ...form, aciklama: e.target.value })}
                rows={3}
                className="w-full rounded-md bg-beton-950 border border-beton-800 px-3 py-2 text-sm text-beton-100 outline-none focus:border-emniyet-500 resize-none"
                placeholder="Tutanak detayları..."
              />
            </div>

            {/* Fotoğraf Ekle */}
            <div>
              <label className="block text-xs text-beton-400 mb-2">Fotoğraflar</label>
              <input ref={formFotoRef} type="file" accept="image/*" multiple capture="environment"
                className="hidden"
                onChange={e => formFotoEkle(e.target.files)}
              />
              <button onClick={() => formFotoRef.current?.click()}
                disabled={fotYukleniyor}
                className="w-full rounded-md border border-dashed border-beton-700 px-3 py-3 text-sm text-beton-400 hover:border-emniyet-500 hover:text-emniyet-500 transition disabled:opacity-50"
              >
                {fotYukleniyor ? "Yükleniyor..." : "📷 Fotoğraf Seç veya Çek (birden fazla seçilebilir)"}
              </button>
              {formFotograflar.length > 0 && (
                <div className="mt-2 grid grid-cols-3 gap-2">
                  {formFotograflar.map((f) => (
                    <div key={f.id} className="relative group">
                      <img src={f.base64} alt={f.ad}
                        className="w-full h-20 object-cover rounded-md border border-beton-700"
                      />
                      <button onClick={() => formFotoSil(f.id)}
                        className="absolute top-1 right-1 bg-red-600 text-white rounded-full w-4 h-4 text-[10px] flex items-center justify-center opacity-0 group-hover:opacity-100 transition"
                      >✕</button>
                    </div>
                  ))}
                </div>
              )}
            </div>

            {/* Onay zinciri önizleme */}
            <div className="rounded-md bg-beton-800 p-3">
              <p className="text-xs text-beton-400 mb-2">Onay Zinciri:</p>
              <div className="flex items-center gap-2 flex-wrap text-xs text-beton-300">
                {form.kisim_sefi_var && <><span className="bg-beton-700 rounded px-2 py-0.5">Kısım Şefi</span><span>→</span></>}
                <span className="bg-beton-700 rounded px-2 py-0.5">Şantiye Şefi</span>
                <span>→</span>
                <span className="bg-beton-700 rounded px-2 py-0.5">Proje Müdürü</span>
              </div>
            </div>

            <div className="flex justify-end gap-3 pt-2">
              <button onClick={() => { setFormAcik(false); setFormFotograflar([]); }}
                className="rounded-md border border-beton-700 px-4 py-2 text-sm text-beton-300 hover:border-beton-500"
              >İptal</button>
              <button onClick={tutanakOlustur}
                disabled={!form.baslik.trim() || !form.aciklama.trim()}
                className="rounded-md bg-emniyet-500 px-4 py-2 text-sm font-medium text-beton-950 hover:brightness-110 disabled:opacity-50"
              >Oluştur</button>
            </div>
          </div>
        </div>
      )}

      {/* Onay Modal */}
      {onayModal && secili && bekleyenOnay !== -1 && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60">
          <div className="bg-beton-900 border border-beton-700 rounded-xl w-full max-w-md mx-4 p-6 space-y-4">
            <h2 className="font-bold text-white">Onay: {secili.onay_zinciri[bekleyenOnay].rol}</h2>
            <p className="text-sm text-beton-300">Tutanak: <span className="text-white">{secili.baslik}</span></p>
            <div>
              <label className="block text-xs text-beton-400 mb-1">Not (opsiyonel)</label>
              <textarea value={onayNot} onChange={e => setOnayNot(e.target.value)} rows={2}
                className="w-full rounded-md bg-beton-950 border border-beton-800 px-3 py-2 text-sm text-beton-100 outline-none focus:border-emniyet-500 resize-none"
                placeholder="Onay notu..."
              />
            </div>
            <div className="flex gap-3">
              <button onClick={() => onayVer(secili, "onaylandi")}
                className="flex-1 rounded-md bg-green-600 px-4 py-2 text-sm font-medium text-white hover:bg-green-500"
              >✓ Onayla</button>
              <button onClick={() => onayVer(secili, "reddedildi")}
                className="flex-1 rounded-md bg-red-600 px-4 py-2 text-sm font-medium text-white hover:bg-red-500"
              >✕ Reddet</button>
              <button onClick={() => { setOnayModal(false); setOnayNot(""); }}
                className="rounded-md border border-beton-700 px-4 py-2 text-sm text-beton-300"
              >İptal</button>
            </div>
          </div>
        </div>
      )}

      {/* Büyük Fotoğraf Modalı */}
      {fotografBuyuk && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/80"
          onClick={() => setFotografBuyuk(null)}
        >
          <div className="relative max-w-3xl w-full mx-4" onClick={e => e.stopPropagation()}>
            <img src={fotografBuyuk.base64} alt={fotografBuyuk.ad}
              className="w-full max-h-[80vh] object-contain rounded-lg"
            />
            <p className="text-center text-xs text-beton-400 mt-2">{fotografBuyuk.ad}</p>
            <button onClick={() => setFotografBuyuk(null)}
              className="absolute top-2 right-2 bg-black/60 text-white rounded-full w-8 h-8 flex items-center justify-center hover:bg-black"
            >✕</button>
          </div>
        </div>
      )}
    </div>
  );
}
