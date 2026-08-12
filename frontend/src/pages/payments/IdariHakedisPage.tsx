import { useCallback, useEffect, useRef, useState } from "react";
import { api, apiFetchBlob, apiUpload } from "../../api/client";
import { useProjects } from "../ProjectContext";

// Nakit Akış Faz D — İdari Hakedişler: idare (işveren) tarafından ana
// yükleniciye ödenen hakedişler, nakit akışının tek "in" (giriş) kaynağı.
// İdarenin kendi onay süreci sistem dışında gerçekleşir (idare zaten
// onaylamıştır) — burada ayrı bir onay zinciri yok, doğrudan onaylanmış
// kayıt girişi. Fatura mevcut polimorfik documents motoruyla bağlanır.
// gelen_odeme_tarihi girilince/güncellenince/temizlenince cash_events'e
// (direction='in') karşılık gelen satır yazılır/güncellenir/silinir.

type IdariHakedis = {
  id: string;
  donem_no: number;
  aciklama: string;
  tutar: number; // KDV dahil, fiilen tahsil edilen/edilecek toplam
  kdv_pct: number;
  fatura_no?: string | null;
  gelen_odeme_tarihi?: string | null;
  created_by_name: string;
  created_at: string;
  row_version: number;
};

type DocItem = { id: string; title: string; latest_version?: number };
type Fatura = { key: string; ad: string; url: string; docId: string };

const inpBase =
  "rounded-md bg-beton-950 border border-beton-800 px-3 py-2 text-sm text-beton-100 " +
  "outline-none focus:border-emniyet-500";

function fmt(n: number): string {
  return n.toLocaleString("tr-TR", { minimumFractionDigits: 2, maximumFractionDigits: 2 });
}

function bosForm() {
  return { donem_no: "", aciklama: "", tutar: "", kdv_pct: "20", fatura_no: "", gelen_odeme_tarihi: "" };
}

export default function IdariHakedisPage() {
  const { current } = useProjects();
  const pid = current?.id;

  const [liste, setListe] = useState<IdariHakedis[]>([]);
  const [formAcik, setFormAcik] = useState(false);
  const [form, setForm] = useState(bosForm());
  const [olusturuluyor, setOlusturuluyor] = useState(false);
  const [secili, setSecili] = useState<IdariHakedis | null>(null);
  const [duzenle, setDuzenle] = useState(bosForm());
  const [faturalar, setFaturalar] = useState<Fatura[]>([]);
  const [faturaYukleniyor, setFaturaYukleniyor] = useState(false);
  const [kaydediliyor, setKaydediliyor] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  const faturaRef = useRef<HTMLInputElement>(null);

  const load = useCallback(async () => {
    if (!pid) return;
    setErr(null);
    try {
      const r = await api<{ idari_hakedisler: IdariHakedis[] }>(
        `/projects/${pid}/idari-hakedisler`, { projectId: pid });
      setListe(r.idari_hakedisler ?? []);
    } catch {
      setErr("İdari hakedişler yüklenemedi ya da erişim yetkiniz yok.");
    }
  }, [pid]);

  useEffect(() => { load(); }, [load]);

  const loadFaturalar = useCallback(async (h: IdariHakedis) => {
    if (!pid) return;
    try {
      const d = await api<{ documents: DocItem[] }>(
        `/projects/${pid}/documents?entity_type=idari_hakedis_fatura&entity_id=${h.id}`, { projectId: pid });
      const withUrls = await Promise.all(
        (d.documents ?? []).filter((doc) => doc.latest_version).map(async (doc) => {
          const url = await apiFetchBlob(`/projects/${pid}/documents/${doc.id}/versions/${doc.latest_version}/download`);
          return { key: doc.id, ad: doc.title, url, docId: doc.id } as Fatura;
        })
      );
      setFaturalar(withUrls);
    } catch {
      setFaturalar([]);
    }
  }, [pid]);

  function sec(h: IdariHakedis) {
    setSecili(h);
    setDuzenle({
      donem_no: String(h.donem_no), aciklama: h.aciklama, tutar: String(h.tutar),
      kdv_pct: String(h.kdv_pct), fatura_no: h.fatura_no ?? "", gelen_odeme_tarihi: h.gelen_odeme_tarihi ?? "",
    });
    loadFaturalar(h);
  }

  async function olustur() {
    if (!pid || !form.aciklama.trim() || !form.donem_no || !form.tutar) return;
    setOlusturuluyor(true);
    setErr(null);
    try {
      await api(`/projects/${pid}/idari-hakedisler`, {
        method: "POST", projectId: pid,
        body: {
          donem_no: Number(form.donem_no),
          aciklama: form.aciklama.trim(),
          tutar: Number(form.tutar),
          kdv_pct: Number(form.kdv_pct) || 20,
          fatura_no: form.fatura_no.trim() || undefined,
          gelen_odeme_tarihi: form.gelen_odeme_tarihi || undefined,
        },
      });
      setForm(bosForm());
      setFormAcik(false);
      await load();
    } catch {
      setErr("Kayıt oluşturulamadı. Dönem numarası bu projede zaten kullanılıyor olabilir.");
    } finally {
      setOlusturuluyor(false);
    }
  }

  async function kaydet() {
    if (!pid || !secili) return;
    setKaydediliyor(true);
    setErr(null);
    try {
      await api(`/projects/${pid}/idari-hakedisler/${secili.id}`, {
        method: "PATCH", projectId: pid,
        body: {
          aciklama: duzenle.aciklama.trim(),
          tutar: Number(duzenle.tutar),
          kdv_pct: Number(duzenle.kdv_pct) || 20,
          fatura_no: duzenle.fatura_no.trim() || undefined,
          gelen_odeme_tarihi: duzenle.gelen_odeme_tarihi || undefined,
          row_version: secili.row_version,
        },
      });
      await load();
      const guncel = await api<{ idari_hakedisler: IdariHakedis[] }>(
        `/projects/${pid}/idari-hakedisler`, { projectId: pid });
      setSecili(guncel.idari_hakedisler.find((x) => x.id === secili.id) ?? null);
    } catch {
      setErr("Kaydedilemedi (sayfa güncel olmayabilir — yeniden seçip deneyin).");
    } finally {
      setKaydediliyor(false);
    }
  }

  async function faturaEkle(files: FileList | null) {
    if (!files || !secili || !pid) return;
    setFaturaYukleniyor(true);
    setErr(null);
    try {
      for (const file of Array.from(files)) {
        if (file.size > 20 * 1024 * 1024) { alert(`${file.name} 20MB sınırını aşıyor.`); continue; }
        const doc = await api<{ document: { id: string } }>(`/projects/${pid}/documents`, {
          method: "POST", projectId: pid,
          body: {
            title: file.name, doc_category: "IdariHakedisFatura",
            entity_type: "idari_hakedis_fatura", entity_id: secili.id,
          },
        });
        const fd = new FormData();
        fd.append("file", file);
        await apiUpload(`/projects/${pid}/documents/${doc.document.id}/versions`, fd);
      }
      await loadFaturalar(secili);
    } catch {
      setErr("Fatura yüklenemedi.");
    } finally {
      setFaturaYukleniyor(false);
      if (faturaRef.current) faturaRef.current.value = "";
    }
  }

  async function sil(h: IdariHakedis) {
    if (!pid || !confirm(`"${h.aciklama}" (Dönem #${h.donem_no}) silinsin mi?`)) return;
    try {
      await api(`/projects/${pid}/idari-hakedisler/${h.id}`, { method: "DELETE", projectId: pid });
      if (secili?.id === h.id) setSecili(null);
      await load();
    } catch {
      setErr("Silinemedi.");
    }
  }

  const toplamTahsil = liste.reduce((s, h) => s + h.tutar, 0);
  const bekleyenTahsil = liste.filter((h) => !h.gelen_odeme_tarihi).reduce((s, h) => s + h.tutar, 0);

  if (!current) return <p className="text-beton-400 text-sm">Önce üst bardan bir proje seçin.</p>;

  return (
    <div className="max-w-5xl mx-auto space-y-4">
      <div className="flex items-center justify-between gap-3 flex-wrap">
        <div>
          <h1 className="font-display font-extrabold text-xl text-white">İdari Hakedişler</h1>
          <p className="text-xs text-beton-400 mt-0.5">
            İdare (işveren) tarafından ana yükleniciye ödenen hakedişler — nakit akışının giriş kaynağı.
          </p>
        </div>
        <button onClick={() => setFormAcik(true)}
          className="rounded-md bg-emniyet-500 px-3 py-2 text-sm font-medium text-beton-950 hover:brightness-110">
          + Yeni Hakediş
        </button>
      </div>
      {err && <p className="text-sm text-red-400">{err}</p>}

      <div className="grid grid-cols-2 gap-3">
        <div className="rounded-lg border border-beton-800 bg-beton-900 p-3">
          <p className="text-xs text-beton-500 mb-1">Toplam Tahakkuk (KDV Dahil)</p>
          <p className="text-lg font-bold text-white">{fmt(toplamTahsil)} TL</p>
        </div>
        <div className="rounded-lg border border-beton-800 bg-beton-900 p-3">
          <p className="text-xs text-beton-500 mb-1">Ödeme Tarihi Girilmemiş</p>
          <p className={`text-lg font-bold ${bekleyenTahsil > 0 ? "text-amber-400" : "text-beton-400"}`}>
            {fmt(bekleyenTahsil)} TL
          </p>
        </div>
      </div>

      <div className="grid md:grid-cols-[1fr_380px] gap-4">
        {/* Liste */}
        <div className="space-y-2">
          {liste.length === 0 && <p className="text-beton-400 text-sm">Henüz idari hakediş kaydı yok.</p>}
          {liste.map((h) => (
            <div key={h.id} onClick={() => sec(h)}
              className={`rounded-lg border p-4 cursor-pointer transition ${
                secili?.id === h.id ? "border-emniyet-500 bg-beton-800" : "border-beton-800 bg-beton-900 hover:border-beton-600"
              }`}
            >
              <div className="flex items-center justify-between gap-2 flex-wrap">
                <span className="font-medium text-white text-sm">Dönem #{h.donem_no} — {h.aciklama}</span>
                {h.gelen_odeme_tarihi ? (
                  <span className="text-xs bg-green-500/20 text-green-300 border border-green-500/40 rounded-full px-2 py-0.5">
                    ✓ Tahsil edildi
                  </span>
                ) : (
                  <span className="text-xs bg-amber-500/15 text-amber-400 border border-amber-500/40 rounded-full px-2 py-0.5">
                    Ödeme bekliyor
                  </span>
                )}
              </div>
              <div className="mt-1 text-xs text-beton-400 flex flex-wrap gap-x-4">
                <span className="text-beton-200 font-semibold">{fmt(h.tutar)} TL</span>
                <span>KDV %{h.kdv_pct}</span>
                {h.fatura_no && <span>Fatura: {h.fatura_no}</span>}
                {h.gelen_odeme_tarihi && <span>Gelen ödeme: {h.gelen_odeme_tarihi}</span>}
              </div>
            </div>
          ))}
        </div>

        {/* Detay Paneli */}
        {secili && (
          <div className="rounded-lg border border-beton-800 bg-beton-900 p-4 space-y-4 h-fit">
            <div className="flex items-start justify-between gap-2">
              <h2 className="font-bold text-white">Dönem #{secili.donem_no}</h2>
              <button onClick={() => sil(secili)}
                className="text-xs text-red-400 hover:text-red-300">Sil</button>
            </div>

            <div>
              <label className="block text-xs text-beton-400 mb-1">Açıklama</label>
              <input value={duzenle.aciklama} onChange={(e) => setDuzenle({ ...duzenle, aciklama: e.target.value })}
                className={`${inpBase} w-full`} />
            </div>

            <div className="grid grid-cols-2 gap-2">
              <div>
                <label className="block text-xs text-beton-400 mb-1">Tutar (KDV Dahil)</label>
                <input value={duzenle.tutar} inputMode="decimal"
                  onChange={(e) => setDuzenle({ ...duzenle, tutar: e.target.value })}
                  className={`${inpBase} w-full text-right`} />
              </div>
              <div>
                <label className="block text-xs text-beton-400 mb-1">KDV %</label>
                <input value={duzenle.kdv_pct} inputMode="decimal"
                  onChange={(e) => setDuzenle({ ...duzenle, kdv_pct: e.target.value })}
                  className={`${inpBase} w-full text-right`} />
              </div>
            </div>

            {Number(duzenle.tutar) > 0 && (
              <p className="text-xs text-beton-500">
                KDV hariç: {fmt(Number(duzenle.tutar) / (1 + (Number(duzenle.kdv_pct) || 0) / 100))} TL ·
                {" "}KDV: {fmt(Number(duzenle.tutar) - Number(duzenle.tutar) / (1 + (Number(duzenle.kdv_pct) || 0) / 100))} TL
              </p>
            )}

            <div>
              <label className="block text-xs text-beton-400 mb-1">Fatura No</label>
              <input value={duzenle.fatura_no} onChange={(e) => setDuzenle({ ...duzenle, fatura_no: e.target.value })}
                className={`${inpBase} w-full`} placeholder="opsiyonel" />
            </div>

            <div>
              <label className="block text-xs text-beton-400 mb-1">Gelen Ödeme Tarihi</label>
              <input type="date" value={duzenle.gelen_odeme_tarihi}
                onChange={(e) => setDuzenle({ ...duzenle, gelen_odeme_tarihi: e.target.value })}
                className={`${inpBase} w-full`} />
              <p className="text-[10px] text-beton-500 mt-1">
                Girilince nakit akışına giriş (in) olarak yansır; temizlenirse kaldırılır.
              </p>
            </div>

            <button onClick={kaydet} disabled={kaydediliyor}
              className="w-full rounded-md bg-emniyet-500 px-3 py-2 text-sm font-medium text-beton-950 hover:brightness-110 disabled:opacity-50">
              {kaydediliyor ? "Kaydediliyor…" : "Kaydet"}
            </button>

            {/* Fatura */}
            <div className="border-t border-beton-800 pt-3">
              <div className="flex items-center justify-between mb-2">
                <p className="text-xs text-beton-400 uppercase tracking-wide">Fatura ({faturalar.length})</p>
                <input ref={faturaRef} type="file" accept="application/pdf,image/*" multiple className="hidden"
                  onChange={(e) => faturaEkle(e.target.files)} />
                <button onClick={() => faturaRef.current?.click()} disabled={faturaYukleniyor}
                  className="text-xs rounded border border-beton-700 px-2 py-1 text-beton-300 hover:border-emniyet-500 disabled:opacity-50">
                  {faturaYukleniyor ? "Yükleniyor…" : "📎 Fatura Yükle"}
                </button>
              </div>
              {faturalar.length > 0 ? (
                <ul className="space-y-1">
                  {faturalar.map((f) => (
                    <li key={f.key}>
                      <a href={f.url} download={f.ad} className="text-xs text-emniyet-500 hover:underline">
                        {f.ad}
                      </a>
                    </li>
                  ))}
                </ul>
              ) : (
                <p className="text-xs text-beton-500">Henüz fatura yüklenmedi.</p>
              )}
            </div>
          </div>
        )}
      </div>

      {/* Yeni Hakediş Formu */}
      {formAcik && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60" onClick={() => setFormAcik(false)}>
          <div className="bg-beton-900 border border-beton-700 rounded-xl w-full max-w-md mx-4 p-6 space-y-4"
            onClick={(e) => e.stopPropagation()}>
            <h2 className="font-display font-bold text-white text-lg">Yeni İdari Hakediş</h2>

            <div>
              <label className="block text-xs text-beton-400 mb-1">Dönem No *</label>
              <input value={form.donem_no} inputMode="numeric"
                onChange={(e) => setForm({ ...form, donem_no: e.target.value })}
                className={`${inpBase} w-full`} placeholder="1" />
            </div>

            <div>
              <label className="block text-xs text-beton-400 mb-1">Açıklama *</label>
              <input value={form.aciklama} onChange={(e) => setForm({ ...form, aciklama: e.target.value })}
                className={`${inpBase} w-full`} placeholder="Ör. 2026 Ocak dönemi hakediş" />
            </div>

            <div className="grid grid-cols-2 gap-2">
              <div>
                <label className="block text-xs text-beton-400 mb-1">Tutar (KDV Dahil) *</label>
                <input value={form.tutar} inputMode="decimal"
                  onChange={(e) => setForm({ ...form, tutar: e.target.value })}
                  className={`${inpBase} w-full text-right`} placeholder="0" />
              </div>
              <div>
                <label className="block text-xs text-beton-400 mb-1">KDV %</label>
                <input value={form.kdv_pct} inputMode="decimal"
                  onChange={(e) => setForm({ ...form, kdv_pct: e.target.value })}
                  className={`${inpBase} w-full text-right`} />
              </div>
            </div>

            <div>
              <label className="block text-xs text-beton-400 mb-1">Fatura No</label>
              <input value={form.fatura_no} onChange={(e) => setForm({ ...form, fatura_no: e.target.value })}
                className={`${inpBase} w-full`} placeholder="opsiyonel" />
            </div>

            <div>
              <label className="block text-xs text-beton-400 mb-1">Gelen Ödeme Tarihi</label>
              <input type="date" value={form.gelen_odeme_tarihi}
                onChange={(e) => setForm({ ...form, gelen_odeme_tarihi: e.target.value })}
                className={`${inpBase} w-full`} />
              <p className="text-[10px] text-beton-500 mt-1">Henüz tahsil edilmediyse boş bırakın; sonra girebilirsiniz.</p>
            </div>

            <div className="flex justify-end gap-3 pt-2">
              <button onClick={() => { setFormAcik(false); setForm(bosForm()); }}
                className="rounded-md border border-beton-700 px-4 py-2 text-sm text-beton-300 hover:border-beton-500">
                İptal
              </button>
              <button onClick={olustur}
                disabled={!form.aciklama.trim() || !form.donem_no || !form.tutar || olusturuluyor}
                className="rounded-md bg-emniyet-500 px-4 py-2 text-sm font-medium text-beton-950 hover:brightness-110 disabled:opacity-50">
                {olusturuluyor ? "Oluşturuluyor…" : "Oluştur"}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
