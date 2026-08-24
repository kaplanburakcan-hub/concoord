import { useCallback, useEffect, useRef, useState } from "react";
import { useSearchParams } from "react-router-dom";
import { api, apiFetchBlob, apiUpload } from "../../api/client";
import { useProjects } from "../../projects/ProjectContext";
import { useKesinKabulTarihi } from "../../hooks/useKesinKabulTarihi";

// Saha Tutanakları — önceden tamamen tarayıcı localStorage'ındaydı (hiç
// kullanıcı/cihaz arasında paylaşılmıyordu, fotoğraflar base64 olarak JSON
// içine gömülüydü). Artık gerçek bir backend varlığı: onay zinciri sunucuda
// hesaplanır (Submit/Decide uçları), fotoğraflar mevcut polimorfik documents
// motoru üzerinden bağlanır (entity_type='saha_tutanagi').

// ── Tipler ───────────────────────────────────────────────────────────
type TutanakTip = "kaza_yangin_hirsizlik" | "ek_imalat" | "mesai" | "yevmiyeli" | "zimmet";

type OnayAdim = {
  rol: string;
  ad: string;
  durum: "bekliyor" | "onaylandi" | "reddedildi";
  tarih?: string;
  not?: string;
};

type Tutanak = {
  id: string;
  tip: TutanakTip;
  baslik: string;
  tarih: string;
  taseron_id?: string;
  taseron_adi?: string;
  personel_id?: string;
  personel_ad_soyad?: string;
  personel_firma?: string;
  kisim?: string;
  aciklama: string;
  tutar?: number;
  birim?: string;
  miktar?: number;
  durum: "taslak" | "onay_sureci" | "onaylandi" | "reddedildi";
  onay_zinciri: OnayAdim[];
  hakedise_eklendi: boolean;
  created_by_name: string;
  created_at: string;
  row_version: number;
};

type Sub = { id: string; company_name: string };
type Personel = { id: string; ad_soyad: string; firma: string; is_aktif: boolean };
type DocItem = { id: string; title: string; latest_version?: number };
// Önizleme için çözümlenmiş fotoğraf: yeni-tutanak formunda henüz yüklenmemiş
// (staged) bir File ya da kaydedilmiş bir documents kaydından gelen blob URL.
type Foto = { key: string; ad: string; url: string; docId?: string; file?: File };

const TIP_LABEL: Record<TutanakTip, string> = {
  kaza_yangin_hirsizlik: "Kaza / Yangın / Hırsızlık",
  ek_imalat: "Ek İmalat Tutanağı",
  mesai: "Mesai Tutanağı",
  yevmiyeli: "Yevmiyeli Çalışma Tutanağı",
  zimmet: "Zimmet Tutanağı",
};

const TIP_ICON: Record<TutanakTip, string> = {
  kaza_yangin_hirsizlik: "🚨",
  ek_imalat: "🔨",
  mesai: "⏰",
  yevmiyeli: "👷",
  zimmet: "📦",
};

const TIP_HAKEDIS: Record<TutanakTip, boolean> = {
  kaza_yangin_hirsizlik: false,
  ek_imalat: true,
  mesai: true,
  yevmiyeli: true,
  zimmet: false,
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

const BOŞ_FORM = {
  tip: "ek_imalat" as TutanakTip,
  baslik: "",
  tarih: new Date().toISOString().slice(0, 10),
  taseron_id: "",
  personel_id: "",
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
  const pid = current?.id;
  const kesinKabul = useKesinKabulTarihi(pid);
  const [searchParams, setSearchParams] = useSearchParams();

  const [tutanaklar, setTutanaklar] = useState<Tutanak[]>([]);
  const [subs, setSubs] = useState<Sub[]>([]);
  const [personeller, setPersoneller] = useState<Personel[]>([]);
  const [formAcik, setFormAcik] = useState(false);
  const [form, setForm] = useState({ ...BOŞ_FORM });
  const [formFotograflar, setFormFotograflar] = useState<Foto[]>([]);
  const [secili, setSecili] = useState<Tutanak | null>(null);
  const [seciliFotograflar, setSeciliFotograflar] = useState<Foto[]>([]);
  const [onayModal, setOnayModal] = useState(false);
  const [onayNot, setOnayNot] = useState("");
  const [filtre, setFiltre] = useState<TutanakTip | "hepsi">("hepsi");
  const [fotografBuyuk, setFotografBuyuk] = useState<Foto | null>(null);
  const [fotYukleniyor, setFotYukleniyor] = useState(false);
  const [olusturuluyor, setOlusturuluyor] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  const formFotoRef = useRef<HTMLInputElement>(null);
  const detayFotoRef = useRef<HTMLInputElement>(null);

  const subName = (id?: string) => subs.find((s) => s.id === id)?.company_name;
  const personelFirma = (id?: string) => personeller.find((p) => p.id === id)?.firma;

  const load = useCallback(async () => {
    if (!pid) return;
    setErr(null);
    try {
      const [t, s, p] = await Promise.all([
        api<{ tutanaklar: Tutanak[] }>(`/projects/${pid}/tutanaklar`, { projectId: pid }),
        api<{ subcontractors: Sub[] }>(`/projects/${pid}/subcontractors`, { projectId: pid }),
        api<{ personnel: Personel[] }>(`/projects/${pid}/personnel`, { projectId: pid }),
      ]);
      setTutanaklar(t.tutanaklar ?? []);
      setSubs(s.subcontractors ?? []);
      setPersoneller((p.personnel ?? []).filter((x) => x.is_aktif));
    } catch {
      setErr("Tutanaklar yüklenemedi ya da erişim yetkiniz yok.");
    }
  }, [pid]);

  useEffect(() => { load(); }, [load]);

  // Depo Raporları'ndan "+ Zimmet Tutanağı" ile gelindiğinde formu doğrudan
  // Zimmet tipiyle açar (?tip=zimmet).
  useEffect(() => {
    if (searchParams.get("tip") === "zimmet") {
      setForm((f) => ({ ...f, tip: "zimmet" }));
      setFormAcik(true);
      setFiltre("zimmet");
      searchParams.delete("tip");
      setSearchParams(searchParams, { replace: true });
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // Seçili tutanağın fotoğrafları: documents listesi + her biri için blob URL.
  const loadFotograflar = useCallback(async (t: Tutanak) => {
    if (!pid) return;
    try {
      const d = await api<{ documents: DocItem[] }>(
        `/projects/${pid}/documents?entity_type=saha_tutanagi&entity_id=${t.id}`, { projectId: pid });
      const withUrls = await Promise.all(
        (d.documents ?? []).filter((doc) => doc.latest_version).map(async (doc) => {
          const url = await apiFetchBlob(`/projects/${pid}/documents/${doc.id}/versions/${doc.latest_version}/download`);
          return { key: doc.id, ad: doc.title, url, docId: doc.id } as Foto;
        })
      );
      setSeciliFotograflar(withUrls);
    } catch {
      setSeciliFotograflar([]);
    }
  }, [pid]);

  function sec(t: Tutanak) {
    setSecili(t);
    loadFotograflar(t);
  }

  // Form fotoğraf ekle — henüz sunucuya yüklenmez (tutanak id'si yok), sadece
  // yerel önizleme için staged tutulur; tutanakOlustur() sırasında yüklenir.
  async function formFotoEkle(files: FileList | null) {
    if (!files) return;
    const yeniler: Foto[] = [];
    for (const file of Array.from(files)) {
      if (file.size > 10 * 1024 * 1024) { alert(`${file.name} 10MB sınırını aşıyor.`); continue; }
      yeniler.push({ key: uid(), ad: file.name, url: URL.createObjectURL(file), file });
    }
    setFormFotograflar((prev) => [...prev, ...yeniler]);
    if (formFotoRef.current) formFotoRef.current.value = "";
  }

  // Detay panelinden mevcut (kaydedilmiş) tutanağa fotoğraf ekle — hemen yüklenir.
  async function detayFotoEkle(files: FileList | null) {
    if (!files || !secili || !pid) return;
    setFotYukleniyor(true);
    try {
      for (const file of Array.from(files)) {
        if (file.size > 10 * 1024 * 1024) { alert(`${file.name} 10MB sınırını aşıyor.`); continue; }
        const doc = await api<{ document: { id: string } }>(`/projects/${pid}/documents`, {
          method: "POST", projectId: pid,
          body: { title: file.name, doc_category: "SahaTutanagi", entity_type: "saha_tutanagi", entity_id: secili.id },
        });
        const fd = new FormData();
        fd.append("file", file);
        await apiUpload(`/projects/${pid}/documents/${doc.document.id}/versions`, fd);
      }
      await loadFotograflar(secili);
    } catch {
      setErr("Fotoğraf yüklenemedi.");
    } finally {
      setFotYukleniyor(false);
      if (detayFotoRef.current) detayFotoRef.current.value = "";
    }
  }

  function formFotoSil(key: string) {
    setFormFotograflar((prev) => prev.filter((f) => f.key !== key));
  }

  async function tutanakOlustur() {
    if (!form.baslik.trim() || !form.aciklama.trim() || !pid) return;
    if (form.tip === "zimmet" && !form.personel_id) return;
    setOlusturuluyor(true);
    setErr(null);
    try {
      const res = await api<{ id: string }>(`/projects/${pid}/tutanaklar`, {
        method: "POST", projectId: pid,
        body: {
          tip: form.tip,
          baslik: form.baslik.trim(),
          tarih: form.tarih,
          taseron_id: form.taseron_id || null,
          personel_id: form.personel_id || null,
          kisim: form.kisim || null,
          aciklama: form.aciklama.trim(),
          tutar: form.tutar ? Number(form.tutar) : null,
          birim: form.birim || null,
          miktar: form.miktar ? Number(form.miktar) : null,
          kisim_sefi_var: form.kisim_sefi_var,
        },
      });
      // Staged fotoğrafları şimdi yükle (tutanak id'si artık var).
      for (const f of formFotograflar) {
        if (!f.file) continue;
        const doc = await api<{ document: { id: string } }>(`/projects/${pid}/documents`, {
          method: "POST", projectId: pid,
          body: { title: f.ad, doc_category: "SahaTutanagi", entity_type: "saha_tutanagi", entity_id: res.id },
        });
        const fd = new FormData();
        fd.append("file", f.file);
        await apiUpload(`/projects/${pid}/documents/${doc.document.id}/versions`, fd);
      }
      formFotograflar.forEach((f) => URL.revokeObjectURL(f.url));
      setForm({ ...BOŞ_FORM });
      setFormFotograflar([]);
      setFormAcik(false);
      await load();
    } catch {
      setErr("Tutanak oluşturulamadı.");
    } finally {
      setOlusturuluyor(false);
    }
  }

  async function onayaSuncu(t: Tutanak) {
    if (!pid) return;
    try {
      await api(`/projects/${pid}/tutanaklar/${t.id}/submit`, { method: "POST", projectId: pid });
      await load();
      setSecili({ ...t, durum: "onay_sureci" });
    } catch {
      setErr("Onaya sunulamadı.");
    }
  }

  async function onayVer(t: Tutanak, karar: "onaylandi" | "reddedildi") {
    if (!pid) return;
    try {
      await api(`/projects/${pid}/tutanaklar/${t.id}/decide`, {
        method: "POST", projectId: pid, body: { karar, not: onayNot || null },
      });
      await load();
      setOnayModal(false);
      setOnayNot("");
      const guncel = await api<{ tutanaklar: Tutanak[] }>(`/projects/${pid}/tutanaklar`, { projectId: pid });
      setSecili(guncel.tutanaklar.find((x) => x.id === t.id) ?? null);
    } catch {
      setErr("Karar kaydedilemedi.");
    }
  }

  async function sil(id: string) {
    if (!pid || !confirm("Bu tutanağı silmek istediğinize emin misiniz?")) return;
    try {
      await api(`/projects/${pid}/tutanaklar/${id}`, { method: "DELETE", projectId: pid });
      await load();
      if (secili?.id === id) setSecili(null);
    } catch {
      setErr("Silinemedi.");
    }
  }

  const filtreliler = filtre === "hepsi" ? tutanaklar : tutanaklar.filter((t) => t.tip === filtre);
  const bekleyenOnay = secili ? secili.onay_zinciri.findIndex((a) => a.durum === "bekliyor") : -1;

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
      {err && <p className="text-sm text-red-400">{err}</p>}

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
            <div key={t.id} onClick={() => sec(t)}
              className={`rounded-lg border p-4 cursor-pointer transition ${
                secili?.id === t.id ? "border-emniyet-500 bg-beton-800" : "border-beton-800 bg-beton-900 hover:border-beton-600"
              }`}
            >
              <div className="flex items-center justify-between gap-2 flex-wrap">
                <div className="flex items-center gap-2">
                  <span>{TIP_ICON[t.tip]}</span>
                  <span className="font-medium text-white text-sm">{t.baslik}</span>
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
                {t.taseron_adi && <span>{t.taseron_adi}</span>}
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
            {(secili.taseron_adi || subName(secili.taseron_id)) && (
              <p className="text-sm text-beton-300">Taşeron: <span className="text-white">{secili.taseron_adi ?? subName(secili.taseron_id)}</span></p>
            )}
            {secili.personel_ad_soyad && (
              <p className="text-sm text-beton-300">
                Personel: <span className="text-white">{secili.personel_ad_soyad}</span>
                {secili.personel_firma && <span className="text-beton-400"> ({secili.personel_firma})</span>}
              </p>
            )}
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
                <p className="text-xs text-beton-400 uppercase tracking-wide">Fotoğraflar ({seciliFotograflar.length})</p>
                <div className="flex gap-2">
                  <input ref={detayFotoRef} type="file" accept="image/*" multiple capture="environment"
                    className="hidden"
                    onChange={(e) => detayFotoEkle(e.target.files)}
                  />
                  <button onClick={() => detayFotoRef.current?.click()}
                    disabled={fotYukleniyor}
                    className="text-xs rounded border border-beton-700 px-2 py-1 text-beton-300 hover:border-emniyet-500 disabled:opacity-50"
                  >
                    {fotYukleniyor ? "Yükleniyor..." : "📷 Fotoğraf Ekle"}
                  </button>
                </div>
              </div>
              {seciliFotograflar.length > 0 ? (
                <div className="grid grid-cols-3 gap-2">
                  {seciliFotograflar.map((f) => (
                    <img key={f.key} src={f.url} alt={f.ad}
                      className="w-full h-20 object-cover rounded-md cursor-pointer border border-beton-700 hover:border-emniyet-500"
                      onClick={() => setFotografBuyuk(f)}
                    />
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
            onClick={(e) => e.stopPropagation()}
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
              <input value={form.baslik} onChange={(e) => setForm({ ...form, baslik: e.target.value })}
                className="w-full rounded-md bg-beton-950 border border-beton-800 px-3 py-2 text-sm text-beton-100 outline-none focus:border-emniyet-500"
                placeholder="Tutanak başlığı"
              />
            </div>

            {/* Tarih */}
            <div>
              <label className="block text-xs text-beton-400 mb-1">Tarih *</label>
              <input type="date" value={form.tarih} max={kesinKabul} onChange={(e) => setForm({ ...form, tarih: e.target.value })}
                className="w-full rounded-md bg-beton-950 border border-beton-800 px-3 py-2 text-sm text-beton-100 outline-none focus:border-emniyet-500"
              />
            </div>

            {/* Zimmet: Personel seçimi — firma otomatik belirir */}
            {form.tip === "zimmet" && (
              <div>
                <label className="block text-xs text-beton-400 mb-1">Personel *</label>
                <select value={form.personel_id} onChange={(e) => setForm({ ...form, personel_id: e.target.value })}
                  className="w-full rounded-md bg-beton-950 border border-beton-800 px-3 py-2 text-sm text-beton-100 outline-none focus:border-emniyet-500"
                >
                  <option value="">Seçin</option>
                  {personeller.map((p) => <option key={p.id} value={p.id}>{p.ad_soyad}</option>)}
                </select>
                {form.personel_id && (
                  <p className="mt-1 text-xs text-beton-400">
                    Firma: <span className="text-emniyet-500 font-medium">{personelFirma(form.personel_id) || "—"}</span>
                    {personelFirma(form.personel_id) && (
                      <span className="text-beton-500"> · onay sonrası firmanın tanımlı kullanıcılarına bildirim gider</span>
                    )}
                  </p>
                )}
              </div>
            )}

            {/* Taşeron */}
            {form.tip !== "zimmet" && (
              <div>
                <label className="block text-xs text-beton-400 mb-1">Taşeron</label>
                <select value={form.taseron_id} onChange={(e) => setForm({ ...form, taseron_id: e.target.value })}
                  className="w-full rounded-md bg-beton-950 border border-beton-800 px-3 py-2 text-sm text-beton-100 outline-none focus:border-emniyet-500"
                >
                  <option value="">Seçin (opsiyonel)</option>
                  {subs.map((s) => <option key={s.id} value={s.id}>{s.company_name}</option>)}
                </select>
              </div>
            )}

            {/* Kısım */}
            <div>
              <label className="block text-xs text-beton-400 mb-1">Kısım</label>
              <select value={form.kisim} onChange={(e) => setForm({ ...form, kisim: e.target.value })}
                className="w-full rounded-md bg-beton-950 border border-beton-800 px-3 py-2 text-sm text-beton-100 outline-none focus:border-emniyet-500"
              >
                <option value="">Seçin (opsiyonel)</option>
                {KISIMLAR.map((k) => <option key={k} value={k}>{k}</option>)}
              </select>
            </div>

            {/* Kısım Şefi */}
            <div className="flex items-center gap-2">
              <input type="checkbox" id="kisim_sefi" checked={form.kisim_sefi_var}
                onChange={(e) => setForm({ ...form, kisim_sefi_var: e.target.checked })}
                className="rounded"
              />
              <label htmlFor="kisim_sefi" className="text-sm text-beton-300">Kısım Şefi onayı gerekli</label>
            </div>

            {/* Tutar / Miktar */}
            {(TIP_HAKEDIS[form.tip] || form.tip === "zimmet") && (
              <div className={form.tip === "zimmet" ? "grid grid-cols-2 gap-2" : "grid grid-cols-3 gap-2"}>
                <div>
                  <label className="block text-xs text-beton-400 mb-1">Miktar</label>
                  <input type="number" value={form.miktar} onChange={(e) => setForm({ ...form, miktar: e.target.value })}
                    className="w-full rounded-md bg-beton-950 border border-beton-800 px-3 py-2 text-sm text-beton-100 outline-none focus:border-emniyet-500"
                    placeholder="0"
                  />
                </div>
                <div>
                  <label className="block text-xs text-beton-400 mb-1">Birim</label>
                  <input value={form.birim} onChange={(e) => setForm({ ...form, birim: e.target.value })}
                    className="w-full rounded-md bg-beton-950 border border-beton-800 px-3 py-2 text-sm text-beton-100 outline-none focus:border-emniyet-500"
                    placeholder="adet"
                  />
                </div>
                {TIP_HAKEDIS[form.tip] && (
                  <div>
                    <label className="block text-xs text-beton-400 mb-1">Tutar (TL)</label>
                    <input type="number" value={form.tutar} onChange={(e) => setForm({ ...form, tutar: e.target.value })}
                      className="w-full rounded-md bg-beton-950 border border-beton-800 px-3 py-2 text-sm text-beton-100 outline-none focus:border-emniyet-500"
                      placeholder="0"
                    />
                  </div>
                )}
              </div>
            )}

            {/* Açıklama */}
            <div>
              <label className="block text-xs text-beton-400 mb-1">Açıklama *</label>
              <textarea value={form.aciklama} onChange={(e) => setForm({ ...form, aciklama: e.target.value })}
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
                onChange={(e) => formFotoEkle(e.target.files)}
              />
              <button onClick={() => formFotoRef.current?.click()}
                className="w-full rounded-md border border-dashed border-beton-700 px-3 py-3 text-sm text-beton-400 hover:border-emniyet-500 hover:text-emniyet-500 transition"
              >
                📷 Fotoğraf Seç veya Çek (birden fazla seçilebilir)
              </button>
              {formFotograflar.length > 0 && (
                <div className="mt-2 grid grid-cols-3 gap-2">
                  {formFotograflar.map((f) => (
                    <div key={f.key} className="relative group">
                      <img src={f.url} alt={f.ad}
                        className="w-full h-20 object-cover rounded-md border border-beton-700"
                      />
                      <button onClick={() => formFotoSil(f.key)}
                        className="absolute top-1 right-1 bg-red-600 text-white-solid rounded-full w-4 h-4 text-[10px] flex items-center justify-center opacity-0 group-hover:opacity-100 transition"
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
                disabled={!form.baslik.trim() || !form.aciklama.trim() || olusturuluyor || (form.tip === "zimmet" && !form.personel_id)}
                className="rounded-md bg-emniyet-500 px-4 py-2 text-sm font-medium text-beton-950 hover:brightness-110 disabled:opacity-50"
              >{olusturuluyor ? "Oluşturuluyor…" : "Oluştur"}</button>
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
              <textarea value={onayNot} onChange={(e) => setOnayNot(e.target.value)} rows={2}
                className="w-full rounded-md bg-beton-950 border border-beton-800 px-3 py-2 text-sm text-beton-100 outline-none focus:border-emniyet-500 resize-none"
                placeholder="Onay notu..."
              />
            </div>
            <div className="flex gap-3">
              <button onClick={() => onayVer(secili, "onaylandi")}
                className="flex-1 rounded-md bg-green-600 px-4 py-2 text-sm font-medium text-white-solid hover:bg-green-500"
              >✓ Onayla</button>
              <button onClick={() => onayVer(secili, "reddedildi")}
                className="flex-1 rounded-md bg-red-600 px-4 py-2 text-sm font-medium text-white-solid hover:bg-red-500"
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
          <div className="relative max-w-3xl w-full mx-4" onClick={(e) => e.stopPropagation()}>
            <img src={fotografBuyuk.url} alt={fotografBuyuk.ad}
              className="w-full max-h-[80vh] object-contain rounded-lg"
            />
            <p className="text-center text-xs text-beton-400 mt-2">{fotografBuyuk.ad}</p>
            <button onClick={() => setFotografBuyuk(null)}
              className="absolute top-2 right-2 bg-black/60 text-white-solid rounded-full w-8 h-8 flex items-center justify-center hover:bg-black"
            >✕</button>
          </div>
        </div>
      )}
    </div>
  );
}
