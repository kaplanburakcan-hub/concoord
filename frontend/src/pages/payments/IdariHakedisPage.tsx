import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { api, apiFetchBlob, apiUpload, RequestError } from "../../api/client";
import { useProjects } from "../ProjectContext";
import { useKesinKabulTarihi } from "../../hooks/useKesinKabulTarihi";

// Nakit Akış Faz D — İdari Hakedişler: idare (işveren) tarafından ana
// yükleniciye ödenen hakedişler, nakit akışının tek "in" (giriş) kaynağı.
// İdarenin kendi onay süreci sistem dışında gerçekleşir — burada ayrı bir
// onay zinciri yok, doğrudan onaylanmış kayıt girişi.
//
// Giriş ekranı gerçek "Hakediş Raporu" düzenine göre: A) sözleşme fiyatları
// + B) fiyat farkı = C toplam; C - D(önceki hakediş) = E (bu dönem);
// E × KDV% = F; E+F = G (tahakkuk); Σ kesintiler = H; G-H = Yükleniciye
// Ödenecek Tutar (nakit akışına yansıyan `tutar`). Tüm ara değerler
// backend'de de hesaplanır (bkz. internal/idarihakedis/handler.go calc) —
// burada sadece önizleme için aynı formül tekrarlanır.

type KesintiKalem = { ad: string; tutar: number };

type IdariHakedis = {
  id: string;
  donem_no: number;
  aciklama: string;
  hakedis_tarihi?: string | null;
  sozlesme_fiyatlari_tutari: number;
  fiyat_farki_tutari: number;
  onceki_hakedis_toplami: number;
  kdv_pct: number;
  kesintiler: KesintiKalem[];
  tutar: number; // = Yükleniciye Ödenecek Tutar (G-H)
  fatura_no?: string | null;
  gelen_odeme_tarihi?: string | null;
  created_by_name: string;
  created_at: string;
  row_version: number;
};

type DocItem = { id: string; title: string; latest_version?: number };
type DocLink = { key: string; ad: string; url: string; docId: string };

const KESINTI_VARSAYILAN: KesintiKalem[] = [
  { ad: "Gelir / Kurumlar Vergisi (E x %3)", tutar: 0 },
  { ad: "Damga Vergisi (E x % …)", tutar: 0 },
  { ad: "KDV Tevkifatı (F x ….)", tutar: 0 },
  { ad: "Geçici Kabul Kesintisi", tutar: 0 },
  { ad: "Fiyat Farkı Kesin Teminat Kesintisi", tutar: 0 },
  { ad: "Sosyal Sigortalar Kurumu Kesintisi", tutar: 0 },
  { ad: "İdare Makinesi Kiraları", tutar: 0 },
  { ad: "Gecikme Cezası", tutar: 0 },
  { ad: "Avans Mahsubu", tutar: 0 },
];

const inpBase =
  "rounded-md bg-beton-950 border border-beton-800 px-3 py-2 text-sm text-beton-100 " +
  "outline-none focus:border-emniyet-500 disabled:opacity-50";

function fmt(n: number): string {
  return (n || 0).toLocaleString("tr-TR", { minimumFractionDigits: 2, maximumFractionDigits: 2 });
}
function parseFmt(s: string): number {
  return parseFloat(s.replace(/\./g, "").replace(",", ".")) || 0;
}

function calcAll(a: number, b: number, d: number, kdvPct: number, kesintiler: KesintiKalem[]) {
  const c = a + b;
  const e = c - d;
  const f = (e * kdvPct) / 100;
  const g = e + f;
  const h = kesintiler.reduce((s, k) => s + (k.tutar || 0), 0);
  const odenecek = g - h;
  return { c, e, f, g, h, odenecek };
}

// ── Sayıyı yazıya çevir (TL/Kuruş) ───────────────────────────────────────────
const BIR = ["", "bir", "iki", "üç", "dört", "beş", "altı", "yedi", "sekiz", "dokuz"];
const ON = ["", "on", "yirmi", "otuz", "kırk", "elli", "altmış", "yetmiş", "seksen", "doksan"];
function ucBasamak(n: number): string {
  const y = Math.floor(n / 100), o = Math.floor((n % 100) / 10), b = n % 10;
  let s = "";
  if (y > 0) s += (y > 1 ? BIR[y] + " " : "") + "yüz ";
  if (o > 0) s += ON[o] + " ";
  if (b > 0) s += BIR[b] + " ";
  return s;
}
function sayiyiYaziyla(nIn: number): string {
  let n = Math.floor(nIn);
  if (n === 0) return "sıfır";
  const gruplar = ["", "bin", "milyon", "milyar", "trilyon"];
  const parcalar: number[] = [];
  while (n > 0) { parcalar.push(n % 1000); n = Math.floor(n / 1000); }
  let out = "";
  for (let idx = parcalar.length - 1; idx >= 0; idx--) {
    const p = parcalar[idx];
    if (p === 0) continue;
    let kelime = ucBasamak(p);
    if (idx === 1 && p === 1) kelime = ""; // "bir bin" değil "bin"
    out += kelime + gruplar[idx] + " ";
  }
  return out.trim();
}
function tutarYaziyla(tutar: number): string {
  const lira = Math.floor(Math.abs(tutar));
  const kurus = Math.round((Math.abs(tutar) - lira) * 100);
  let s = sayiyiYaziyla(lira) + " Türk Lirası";
  if (kurus > 0) s += " " + sayiyiYaziyla(kurus) + " Kuruş";
  s = s.charAt(0).toUpperCase() + s.slice(1);
  return (tutar < 0 ? "eksi " : "") + s;
}

type FormState = {
  id?: string;
  donem_no: string;
  aciklama: string;
  hakedis_tarihi: string;
  a: string;
  b: string;
  d: string;
  kdv_pct: string;
  kesintiler: KesintiKalem[];
  fatura_no: string;
  gelen_odeme_tarihi: string;
  row_version: number;
};

function bosForm(donemNo?: number, oncekiToplam?: number): FormState {
  return {
    donem_no: donemNo ? String(donemNo) : "",
    aciklama: "", hakedis_tarihi: "",
    a: "", b: "", d: oncekiToplam ? fmt(oncekiToplam) : "",
    kdv_pct: "20",
    kesintiler: KESINTI_VARSAYILAN.map((k) => ({ ...k })),
    fatura_no: "", gelen_odeme_tarihi: "", row_version: 0,
  };
}

// ── Rapor formu (yeni + düzenle ortak) ──────────────────────────────────────
function HakedisForm({
  initial, projectName, projectId, onSave, onCancel, canEdit,
}: {
  initial: FormState;
  projectName: string;
  projectId: string;
  onSave: (f: FormState) => Promise<void>;
  onCancel: () => void;
  canEdit: boolean;
}) {
  const kesinKabul = useKesinKabulTarihi(projectId);
  const [f, setF] = useState<FormState>(initial);
  const [saving, setSaving] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  function set<K extends keyof FormState>(k: K, v: FormState[K]) {
    setF((p) => ({ ...p, [k]: v }));
  }
  function setKesinti(i: number, patch: Partial<KesintiKalem>) {
    setF((p) => ({ ...p, kesintiler: p.kesintiler.map((k, idx) => (idx === i ? { ...k, ...patch } : k)) }));
  }
  function addKesinti() {
    setF((p) => ({ ...p, kesintiler: [...p.kesintiler, { ad: "", tutar: 0 }] }));
  }
  function removeKesinti(i: number) {
    setF((p) => ({ ...p, kesintiler: p.kesintiler.filter((_, idx) => idx !== i) }));
  }

  const a = parseFmt(f.a), b = parseFmt(f.b), d = parseFmt(f.d), kdvPct = Number(f.kdv_pct) || 0;
  const { c, e, fVal, g, h, odenecek } = useMemo(() => {
    const r = calcAll(a, b, d, kdvPct, f.kesintiler);
    return { c: r.c, e: r.e, fVal: r.f, g: r.g, h: r.h, odenecek: r.odenecek };
  }, [a, b, d, kdvPct, f.kesintiler]);

  async function submit() {
    setSaving(true);
    setErr(null);
    try {
      await onSave(f);
    } catch (e) {
      setErr(e instanceof RequestError ? e.message : "Kaydedilemedi.");
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="rounded-lg border border-beton-800 bg-beton-900 overflow-hidden mb-4">
      <div className="p-4 border-b border-beton-800 grid sm:grid-cols-[1fr_120px] gap-3">
        <div>
          <label className="block text-xs text-beton-500 mb-1">İşin Adı</label>
          <input disabled value={projectName} className={`${inpBase} w-full`} />
        </div>
        <div>
          <label className="block text-xs text-beton-500 mb-1">Hakediş No</label>
          <input disabled={!canEdit} value={f.donem_no} inputMode="numeric"
            onChange={(e) => set("donem_no", e.target.value)} className={`${inpBase} w-full`} />
        </div>
        <div className="sm:col-span-2">
          <label className="block text-xs text-beton-500 mb-1">…Tarihine Kadar Yapılan İşin Hakedişi</label>
          <input disabled={!canEdit} type="date" value={f.hakedis_tarihi} max={kesinKabul}
            onChange={(e) => set("hakedis_tarihi", e.target.value)} className={`${inpBase} w-full sm:w-56`} />
        </div>
      </div>

      <div className="p-4 border-b border-beton-800 space-y-2">
        <Satir harf="A" etiket="Sözleşme Fiyatları İle Yapılan İş Tutarı">
          <input disabled={!canEdit} value={f.a} inputMode="decimal" onChange={(e) => set("a", e.target.value)}
            onBlur={(e) => set("a", fmt(parseFmt(e.target.value)))} className={`${inpBase} w-44 text-right font-mono`} />
        </Satir>
        <Satir harf="B" etiket="Fiyat Farkı Tutarı">
          <input disabled={!canEdit} value={f.b} inputMode="decimal" onChange={(e) => set("b", e.target.value)}
            onBlur={(e) => set("b", fmt(parseFmt(e.target.value)))} className={`${inpBase} w-44 text-right font-mono`} />
        </Satir>
        <SatirHesap harf="C" etiket="Toplam Tutar (A+B)" deger={c} />
      </div>

      <div className="p-4 border-b border-beton-800 space-y-2">
        <Satir harf="D" etiket="Bir Önceki Hakedişin Toplam Tutarı">
          <input disabled={!canEdit} value={f.d} inputMode="decimal" onChange={(e) => set("d", e.target.value)}
            onBlur={(e) => set("d", fmt(parseFmt(e.target.value)))} className={`${inpBase} w-44 text-right font-mono`} />
        </Satir>
        <SatirHesap harf="E" etiket="Bu Hakedişin Tutarı (C−D)" deger={e} />
        <Satir harf="F" etiket={<>KDV (E x %
          <input disabled={!canEdit} value={f.kdv_pct} inputMode="decimal"
            onChange={(ev) => set("kdv_pct", ev.target.value)}
            className="inline-block w-14 mx-1 text-center rounded border border-beton-800 bg-beton-950 px-1 py-0.5 text-beton-100" />)</>}>
          <span className="w-44 text-right font-mono text-beton-100 font-semibold">{fmt(fVal)}</span>
        </Satir>
        <SatirHesap harf="G" etiket="Tahakkuk Tutarı (E+F)" deger={g} vurgu />
      </div>

      <div className="p-4 border-b border-beton-800">
        <div className="flex gap-3">
          <div className="hidden sm:flex items-center justify-center text-[10px] font-bold uppercase tracking-widest text-beton-500 border-r border-beton-800 pr-3"
            style={{ writingMode: "vertical-rl", transform: "rotate(180deg)" }}>
            Kesintiler ve Mahsuplar
          </div>
          <div className="flex-1 space-y-1.5">
            {f.kesintiler.map((k, i) => (
              <div key={i} className="flex items-center gap-2">
                <span className="text-xs text-beton-500 w-5">{String.fromCharCode(97 + i)})</span>
                <input disabled={!canEdit} value={k.ad} onChange={(e) => setKesinti(i, { ad: e.target.value })}
                  className="flex-1 bg-transparent border-none text-sm text-beton-200 focus:bg-beton-950 focus:rounded px-1 py-1 outline-none" />
                <input disabled={!canEdit} value={k.tutar ? fmt(k.tutar) : ""} placeholder="0,00" inputMode="decimal"
                  onChange={(e) => setKesinti(i, { tutar: parseFmt(e.target.value) })}
                  onBlur={(e) => setKesinti(i, { tutar: parseFmt(e.target.value) })}
                  className={`${inpBase} w-36 text-right font-mono py-1`} />
                {canEdit && (
                  <button onClick={() => removeKesinti(i)} className="text-beton-600 hover:text-red-400 w-5 text-sm">✕</button>
                )}
              </div>
            ))}
          </div>
        </div>
        {canEdit && (
          <button onClick={addKesinti}
            className="mt-2 text-xs border border-dashed border-beton-700 hover:border-emniyet-500 text-emniyet-500 rounded-md px-3 py-1.5">
            + Kesinti / Mahsup Satırı Ekle
          </button>
        )}
        <div className="mt-3">
          <SatirHesap harf="H" etiket="Kesinti ve Mahsuplar Toplamı" deger={h} />
        </div>
        <div className="mt-2 rounded-lg border border-emniyet-500/40 bg-emniyet-500/10 px-4 py-3 flex items-center justify-between gap-3">
          <span className="text-sm font-semibold text-beton-100">Yükleniciye Ödenecek Tutar (G−H)</span>
          <span className="text-lg font-bold text-emniyet-500 font-mono">{fmt(odenecek)} TL</span>
        </div>
        <p className="mt-2 text-[11.5px] italic text-beton-500">
          Yazıyla: <span className="text-beton-300 not-italic">{tutarYaziyla(odenecek)}.</span>
        </p>
      </div>

      <div className="p-4 border-b border-beton-800 grid sm:grid-cols-2 gap-3">
        <div>
          <label className="block text-xs text-beton-500 mb-1">Fatura No</label>
          <input disabled={!canEdit} value={f.fatura_no} onChange={(e) => set("fatura_no", e.target.value)}
            className={`${inpBase} w-full`} placeholder="opsiyonel" />
        </div>
        <div>
          <label className="block text-xs text-beton-500 mb-1">Gelen Ödeme Tarihi</label>
          <input disabled={!canEdit} type="date" value={f.gelen_odeme_tarihi}
            onChange={(e) => set("gelen_odeme_tarihi", e.target.value)} className={`${inpBase} w-full`} />
        </div>
      </div>

      {err && <p className="px-4 pt-3 text-xs text-red-400">{err}</p>}
      {canEdit && (
        <div className="p-4 flex justify-end gap-2">
          <button onClick={onCancel} className="rounded-md border border-beton-700 px-4 py-2 text-sm text-beton-300 hover:border-beton-500">
            İptal
          </button>
          <button onClick={submit} disabled={saving || !f.donem_no}
            className="rounded-md bg-emniyet-500 px-5 py-2 text-sm font-medium text-beton-950 hover:brightness-110 disabled:opacity-50">
            {saving ? "Kaydediliyor…" : "Hakedişi Kaydet"}
          </button>
        </div>
      )}
    </div>
  );
}

function Satir({ harf, etiket, children }: { harf: string; etiket: React.ReactNode; children: React.ReactNode }) {
  return (
    <div className="flex items-center gap-3">
      <span className="w-5 text-xs font-bold text-beton-500">{harf}</span>
      <span className="flex-1 text-[13px] text-beton-300">{etiket}</span>
      {children}
    </div>
  );
}
function SatirHesap({ harf, etiket, deger, vurgu }: { harf: string; etiket: string; deger: number; vurgu?: boolean }) {
  return (
    <div className={`flex items-center gap-3 rounded-md px-2 py-1.5 ${vurgu ? "bg-emniyet-500/5" : "bg-beton-950/60"}`}>
      <span className="w-5 text-xs font-bold text-beton-500">{harf}</span>
      <span className={`flex-1 text-[13px] ${vurgu ? "font-semibold text-beton-100" : "text-beton-300"}`}>{etiket}</span>
      <span className={`w-44 text-right font-mono ${vurgu ? "font-bold text-beton-100" : "font-semibold text-beton-200"}`}>
        {fmt(deger)} TL
      </span>
    </div>
  );
}

// ── Belge listesi + yükleme (Fatura / Hakediş Belgesi ortak) ───────────────
function BelgePaneli({
  pid, hakedisId, kategori, baslik, ipucu,
}: { pid: string; hakedisId: string; kategori: string; baslik: string; ipucu: string }) {
  const [belgeler, setBelgeler] = useState<DocLink[]>([]);
  const [yukleniyor, setYukleniyor] = useState(false);
  const ref = useRef<HTMLInputElement>(null);

  const load = useCallback(async () => {
    try {
      const d = await api<{ documents: DocItem[] }>(
        `/projects/${pid}/documents?entity_type=idari_hakedis_fatura&entity_id=${hakedisId}&category=${kategori}`,
        { projectId: pid }
      );
      const withUrls = await Promise.all(
        (d.documents ?? []).filter((doc) => doc.latest_version).map(async (doc) => {
          const url = await apiFetchBlob(`/projects/${pid}/documents/${doc.id}/versions/${doc.latest_version}/download`);
          return { key: doc.id, ad: doc.title, url, docId: doc.id } as DocLink;
        })
      );
      setBelgeler(withUrls);
    } catch {
      setBelgeler([]);
    }
  }, [pid, hakedisId, kategori]);

  useEffect(() => { load(); }, [load]);

  async function ekle(files: FileList | null) {
    if (!files) return;
    setYukleniyor(true);
    try {
      for (const file of Array.from(files)) {
        if (file.size > 20 * 1024 * 1024) { alert(`${file.name} 20MB sınırını aşıyor.`); continue; }
        const doc = await api<{ document: { id: string } }>(`/projects/${pid}/documents`, {
          method: "POST", projectId: pid,
          body: { title: file.name, doc_category: kategori, entity_type: "idari_hakedis_fatura", entity_id: hakedisId },
        });
        const fd = new FormData();
        fd.append("file", file);
        await apiUpload(`/projects/${pid}/documents/${doc.document.id}/versions`, fd);
      }
      await load();
    } finally {
      setYukleniyor(false);
      if (ref.current) ref.current.value = "";
    }
  }

  return (
    <div className="rounded-lg border border-beton-800 bg-beton-900 p-4">
      <div className="flex items-center justify-between mb-1.5">
        <p className="text-xs font-semibold text-beton-300">{baslik} ({belgeler.length})</p>
        <input ref={ref} type="file" accept="application/pdf,image/*" multiple className="hidden"
          onChange={(e) => ekle(e.target.files)} />
        <button onClick={() => ref.current?.click()} disabled={yukleniyor}
          className="text-xs rounded border border-beton-700 px-2 py-1 text-beton-300 hover:border-emniyet-500 disabled:opacity-50">
          {yukleniyor ? "Yükleniyor…" : "📎 PDF Ekle"}
        </button>
      </div>
      <p className="text-[10.5px] text-beton-500 mb-2">{ipucu}</p>
      {belgeler.length > 0 ? (
        <ul className="space-y-1">
          {belgeler.map((f) => (
            <li key={f.key}>
              <a href={f.url} download={f.ad} className="text-xs text-emniyet-500 hover:underline">{f.ad}</a>
            </li>
          ))}
        </ul>
      ) : (
        <p className="text-xs text-beton-600">Henüz eklenmedi.</p>
      )}
    </div>
  );
}

// ── Ana sayfa ────────────────────────────────────────────────────────────────
export default function IdariHakedisPage() {
  const { current } = useProjects();
  const pid = current?.id;

  const [liste, setListe] = useState<IdariHakedis[]>([]);
  const [aktifId, setAktifId] = useState<string | "yeni" | null>(null);
  const [err, setErr] = useState<string | null>(null);

  const load = useCallback(async () => {
    if (!pid) return;
    setErr(null);
    try {
      const r = await api<{ idari_hakedisler: IdariHakedis[] }>(`/projects/${pid}/idari-hakedisler`, { projectId: pid });
      setListe(r.idari_hakedisler ?? []);
    } catch {
      setErr("İdari hakedişler yüklenemedi ya da erişim yetkiniz yok.");
    }
  }, [pid]);

  useEffect(() => { load(); }, [load]);

  const secili = liste.find((h) => h.id === aktifId) ?? null;

  function baslatYeni() {
    setAktifId("yeni");
  }

  async function kaydet(f: FormState) {
    if (!pid) return;
    const kesintiler = f.kesintiler.filter((k) => k.ad.trim() !== "" || k.tutar !== 0);
    const body = {
      donem_no: Number(f.donem_no),
      aciklama: f.aciklama.trim(),
      hakedis_tarihi: f.hakedis_tarihi || undefined,
      sozlesme_fiyatlari_tutari: parseFmt(f.a),
      fiyat_farki_tutari: parseFmt(f.b),
      onceki_hakedis_toplami: parseFmt(f.d),
      kdv_pct: Number(f.kdv_pct) || 20,
      kesintiler,
      fatura_no: f.fatura_no.trim() || undefined,
      gelen_odeme_tarihi: f.gelen_odeme_tarihi || undefined,
      row_version: f.row_version,
    };
    if (f.id) {
      await api(`/projects/${pid}/idari-hakedisler/${f.id}`, { method: "PATCH", projectId: pid, body });
    } else {
      await api(`/projects/${pid}/idari-hakedisler`, { method: "POST", projectId: pid, body });
    }
    setAktifId(null);
    await load();
  }

  async function sil(h: IdariHakedis) {
    if (!pid || !confirm(`Hakediş No ${h.donem_no} silinsin mi?`)) return;
    try {
      await api(`/projects/${pid}/idari-hakedisler/${h.id}`, { method: "DELETE", projectId: pid });
      if (aktifId === h.id) setAktifId(null);
      await load();
    } catch {
      setErr("Silinemedi.");
    }
  }

  const toplamOdenecek = liste.reduce((s, h) => s + h.tutar, 0);
  const bekleyenTahsil = liste.filter((h) => !h.gelen_odeme_tarihi).reduce((s, h) => s + h.tutar, 0);

  if (!current) return <p className="text-beton-400 text-sm">Önce üst bardan bir proje seçin.</p>;

  const formInitial: FormState | null =
    aktifId === "yeni"
      ? bosForm(
          (liste[0]?.donem_no ?? 0) + 1,
          liste[0] ? liste[0].sozlesme_fiyatlari_tutari + liste[0].fiyat_farki_tutari : undefined
        )
      : secili
      ? {
          id: secili.id,
          donem_no: String(secili.donem_no),
          aciklama: secili.aciklama,
          hakedis_tarihi: secili.hakedis_tarihi ?? "",
          a: fmt(secili.sozlesme_fiyatlari_tutari),
          b: fmt(secili.fiyat_farki_tutari),
          d: fmt(secili.onceki_hakedis_toplami),
          kdv_pct: String(secili.kdv_pct),
          kesintiler: secili.kesintiler.length > 0 ? secili.kesintiler : KESINTI_VARSAYILAN.map((k) => ({ ...k })),
          fatura_no: secili.fatura_no ?? "",
          gelen_odeme_tarihi: secili.gelen_odeme_tarihi ?? "",
          row_version: secili.row_version,
        }
      : null;

  return (
    <div className="max-w-4xl mx-auto space-y-4">
      <div className="flex items-center justify-between gap-3 flex-wrap">
        <div>
          <h1 className="font-display font-extrabold text-xl text-white">İdari Hakedişler</h1>
          <p className="text-xs text-beton-400 mt-0.5">
            İdare (işveren) tarafından ana yükleniciye ödenen hakedişler — nakit akışının giriş kaynağı.
          </p>
        </div>
        {aktifId === null && (
          <button onClick={baslatYeni}
            className="rounded-md bg-emniyet-500 px-3 py-2 text-sm font-medium text-beton-950 hover:brightness-110">
            + Yeni Hakediş
          </button>
        )}
      </div>
      {err && <p className="text-sm text-red-400">{err}</p>}

      <div className="grid grid-cols-2 gap-3">
        <div className="rounded-lg border border-beton-800 bg-beton-900 p-3">
          <p className="text-xs text-beton-500 mb-1">Toplam Yükleniciye Ödenecek Tutar</p>
          <p className="text-lg font-bold text-white">{fmt(toplamOdenecek)} TL</p>
        </div>
        <div className="rounded-lg border border-beton-800 bg-beton-900 p-3">
          <p className="text-xs text-beton-500 mb-1">Ödeme Tarihi Girilmemiş</p>
          <p className={`text-lg font-bold ${bekleyenTahsil > 0 ? "text-amber-400" : "text-beton-400"}`}>
            {fmt(bekleyenTahsil)} TL
          </p>
        </div>
      </div>

      {formInitial && (
        <>
          <HakedisForm
            initial={formInitial}
            projectName={current.name}
            projectId={pid!}
            canEdit
            onSave={kaydet}
            onCancel={() => setAktifId(null)}
          />
          {secili && (
            <div className="grid sm:grid-cols-2 gap-3 -mt-2">
              <BelgePaneli pid={pid!} hakedisId={secili.id} kategori="IdariHakedisFatura" baslik="Fatura"
                ipucu="Hakedişe ait fatura (varsa birden fazla)." />
              <BelgePaneli pid={pid!} hakedisId={secili.id} kategori="IdariHakedisBelgesi" baslik="Hakediş Belgesi"
                ipucu="İmzalı kapak sayfası ya da komple hakediş — hangisi elinizdeyse onu ekleyin." />
            </div>
          )}
          {aktifId === "yeni" && (
            <p className="text-xs text-beton-500 -mt-2">Belgeleri eklemek için önce hakedişi kaydedin.</p>
          )}
        </>
      )}

      {aktifId === null && (
        <div className="space-y-2">
          {liste.length === 0 && <p className="text-beton-400 text-sm">Henüz idari hakediş kaydı yok.</p>}
          {liste.map((h) => (
            <div key={h.id} onClick={() => setAktifId(h.id)}
              className="rounded-lg border border-beton-800 bg-beton-900 hover:border-beton-600 p-4 cursor-pointer transition">
              <div className="flex items-center justify-between gap-2 flex-wrap">
                <span className="font-medium text-white text-sm">
                  Hakediş No {h.donem_no}{h.hakedis_tarihi ? ` — ${h.hakedis_tarihi}` : ""}
                </span>
                <div className="flex items-center gap-2">
                  {h.gelen_odeme_tarihi ? (
                    <span className="text-xs bg-green-500/20 text-green-300 border border-green-500/40 rounded-full px-2 py-0.5">
                      ✓ Tahsil edildi
                    </span>
                  ) : (
                    <span className="text-xs bg-amber-500/15 text-amber-400 border border-amber-500/40 rounded-full px-2 py-0.5">
                      Ödeme bekliyor
                    </span>
                  )}
                  <button onClick={(e) => { e.stopPropagation(); sil(h); }}
                    className="text-xs text-red-400 hover:text-red-300">Sil</button>
                </div>
              </div>
              <div className="mt-1 text-xs text-beton-400 flex flex-wrap gap-x-4">
                <span className="text-beton-200 font-semibold">{fmt(h.tutar)} TL — yükleniciye ödenecek</span>
                <span>KDV %{h.kdv_pct}</span>
                {h.fatura_no && <span>Fatura: {h.fatura_no}</span>}
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
