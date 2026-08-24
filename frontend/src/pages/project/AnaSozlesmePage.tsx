import { useCallback, useEffect, useRef, useState } from "react";
import { api, RequestError } from "../../api/client";
import { useAuth } from "../../auth/AuthContext";
import { useProjects } from "../ProjectContext";

// ── Stil sabitleri ────────────────────────────────────────────────────────────
const inpBase =
  "rounded-md bg-beton-950 border border-beton-800 px-3 py-1.5 text-sm text-beton-200 " +
  "outline-none focus:border-emniyet-500 disabled:opacity-50";
const inp = `w-full ${inpBase}`;
const labelSm = "text-xs font-medium text-beton-400";

// ── Tipler ───────────────────────────────────────────────────────────────────

type SozlesmeTuru = "birim_fiyat" | "goturu_bedel" | "karma";

type BirimFiyatKalem = {
  id: string;
  tanim: string;
  birim: string;
  miktar: number | string;
  birim_fiyat: number | string;
  para_birimi: string;
};

type ContractForm = {
  isveren_adi: string;
  yuklenici_proje_sorumlusu: string;
  sozlesme_turu: SozlesmeTuru;
  fiyat_farki_var: boolean;
  fiyat_farki_formulu: string;
  sozlesme_bedeli: string;
  sozlesme_para_birimi: string;
  birim_fiyat_kalemleri: BirimFiyatKalem[];
  sozlesme_tarihi: string;
  yer_teslim_tarihi: string;
  is_suresi_gun: string;
  gecici_kabul_sonrasi_gun: string;
  max_artis_orani: string;
  max_eksilis_orani: string;
  sgk_is_yeri_no: string;
  pdf_dosya_url: string;
  pdf_dosya_adi: string;
};

const PARA_BIRIMLERI = ["TRY", "USD", "EUR", "GBP", "CHF", "JPY"];

const TUR_LABEL: Record<SozlesmeTuru, string> = {
  birim_fiyat: "Birim Fiyatlı",
  goturu_bedel: "Anahtar Teslim Götürü Bedel",
  karma: "Kısmi Birim Fiyatlı – Anahtar Teslim Götürü Bedel",
};

// ── Yardımcılar ──────────────────────────────────────────────────────────────

function isoToDisplay(iso: string | null | undefined): string {
  if (!iso) return "";
  const [y, m, d] = iso.split("-");
  if (!y || !m || !d) return "";
  return `${d}/${m}/${y}`;
}

function displayToISO(display: string): string | null {
  const parts = display.split("/");
  if (parts.length !== 3) return null;
  const [d, m, y] = parts;
  if (d.length !== 2 || m.length !== 2 || y.length !== 4) return null;
  const month = parseInt(m, 10);
  const day = parseInt(d, 10);
  if (month < 1 || month > 12 || day < 1 || day > 31) return null;
  return `${y}-${m}-${d}`;
}

function maskDate(_prev: string, raw: string): string {
  const digits = raw.replace(/\D/g, "").slice(0, 8);
  let result = "";
  for (let i = 0; i < digits.length; i++) {
    if (i === 2 || i === 4) result += "/";
    result += digits[i];
  }
  return result;
}

function formatBigNumber(raw: string): string {
  const clean = raw.replace(/\./g, "").replace(",", ".");
  if (!clean) return "";
  const num = parseFloat(clean);
  if (isNaN(num)) return raw;
  return num.toLocaleString("tr-TR", { minimumFractionDigits: 2, maximumFractionDigits: 2 });
}

function parseBigNumber(formatted: string): number | null {
  const clean = formatted.replace(/\./g, "").replace(",", ".");
  const n = parseFloat(clean);
  return isNaN(n) ? null : n;
}

function newUUID(): string {
  return (crypto as any).randomUUID ? (crypto as any).randomUUID() : Math.random().toString(36).slice(2);
}

// ── Bileşen ──────────────────────────────────────────────────────────────────

export default function AnaSozlesmePage() {
  const { current } = useProjects();
  const { can } = useAuth();
  const pid = current?.id;
  const canEdit = can("projects.edit");

  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [saved, setSaved] = useState(false);
  const [err, setErr] = useState<string | null>(null);
  const [fieldErr, setFieldErr] = useState<Record<string, string>>({});
  const pdfRef = useRef<HTMLInputElement>(null);

  // Kaydet-ve-kilitle akışı: sözleşme kaydedilince otomatik kilitlenir
  // (bkz. backend contracts.Upsert). Kilitliyken "view" — özet gösterilir;
  // "Güncelle / Revize Et" tıklanınca "edit"e geçilir. Şimdilik revizyon
  // da doğrudan kilitlenir, onay hiyerarşisi yok (bkz. ilgili commit notu).
  const [mode, setMode] = useState<"view" | "edit">("edit");
  const [meta, setMeta] = useState<{ isLocked: boolean; updatedAt?: string; updatedByName?: string }>({ isLocked: false });

  const empty = (): ContractForm => ({
    isveren_adi: "",
    yuklenici_proje_sorumlusu: "",
    sozlesme_turu: "goturu_bedel",
    fiyat_farki_var: false,
    fiyat_farki_formulu: "",
    sozlesme_bedeli: "",
    sozlesme_para_birimi: "TRY",
    birim_fiyat_kalemleri: [],
    sozlesme_tarihi: "",
    yer_teslim_tarihi: "",
    is_suresi_gun: "",
    gecici_kabul_sonrasi_gun: "",
    max_artis_orani: "",
    max_eksilis_orani: "",
    sgk_is_yeri_no: "",
    pdf_dosya_url: "",
    pdf_dosya_adi: "",
  });

  const [form, setForm] = useState<ContractForm>(empty());

  const load = useCallback(async (silent = false) => {
    if (!pid) return;
    if (!silent) setLoading(true);
    setErr(null);
    try {
      const res = await api<{ contract: any }>(`/projects/${pid}/main-contract`, { projectId: pid });
      const c = res.contract;
      setMeta({ isLocked: !!c.is_locked, updatedAt: c.updated_at, updatedByName: c.updated_by_name });
      setMode(c.is_locked ? "view" : "edit");
      setForm({
        isveren_adi: c.isveren_adi ?? "",
        yuklenici_proje_sorumlusu: c.yuklenici_proje_sorumlusu ?? "",
        sozlesme_turu: c.sozlesme_turu ?? "goturu_bedel",
        fiyat_farki_var: c.fiyat_farki_var ?? false,
        fiyat_farki_formulu: c.fiyat_farki_formulu ?? "",
        sozlesme_bedeli: c.sozlesme_bedeli != null
          ? formatBigNumber(String(c.sozlesme_bedeli)) : "",
        sozlesme_para_birimi: c.sozlesme_para_birimi ?? "TRY",
        birim_fiyat_kalemleri: (c.birim_fiyat_kalemleri ?? []).map((k: any) => ({
          ...k,
          birim_fiyat: formatBigNumber(String(k.birim_fiyat)),
          miktar: String(k.miktar),
        })),
        sozlesme_tarihi: isoToDisplay(c.sozlesme_tarihi),
        yer_teslim_tarihi: isoToDisplay(c.yer_teslim_tarihi),
        is_suresi_gun: c.is_suresi_gun != null ? String(c.is_suresi_gun) : "",
        gecici_kabul_sonrasi_gun: c.gecici_kabul_sonrasi_gun != null ? String(c.gecici_kabul_sonrasi_gun) : "",
        max_artis_orani: c.max_artis_orani != null ? String(c.max_artis_orani) : "",
        max_eksilis_orani: c.max_eksilis_orani != null ? String(c.max_eksilis_orani) : "",
        sgk_is_yeri_no: c.sgk_is_yeri_no ?? "",
        pdf_dosya_url: c.pdf_dosya_url ?? "",
        pdf_dosya_adi: c.pdf_dosya_adi ?? "",
      });
    } catch (e) {
      if (e instanceof RequestError && e.status === 404) {
        setForm(empty());
        setMeta({ isLocked: false });
        setMode("edit");
      } else {
        setErr("Sözleşme verileri yüklenemedi.");
      }
    } finally {
      if (!silent) setLoading(false);
    }
  }, [pid]);

  useEffect(() => { load(); }, [load]);

  function set<K extends keyof ContractForm>(k: K, v: ContractForm[K]) {
    setForm(f => ({ ...f, [k]: v }));
    setFieldErr(fe => { const n = { ...fe }; delete n[k]; return n; });
  }

  function handleDateChange(key: "sozlesme_tarihi" | "yer_teslim_tarihi", raw: string) {
    set(key, maskDate(form[key], raw));
  }

  function addKalem() {
    const kalem: BirimFiyatKalem = {
      id: newUUID(),
      tanim: "",
      birim: "",
      miktar: "",
      birim_fiyat: "",
      para_birimi: form.sozlesme_para_birimi,
    };
    set("birim_fiyat_kalemleri", [...form.birim_fiyat_kalemleri, kalem]);
  }

  function updateKalem(idx: number, field: string, value: string | number) {
    const next = form.birim_fiyat_kalemleri.map((k, i) =>
      i === idx ? { ...k, [field]: value } : k
    );
    set("birim_fiyat_kalemleri", next);
  }

  function removeKalem(idx: number) {
    set("birim_fiyat_kalemleri", form.birim_fiyat_kalemleri.filter((_, i) => i !== idx));
  }

  function validate(): boolean {
    const errs: Record<string, string> = {};
    if (!form.isveren_adi.trim())
      errs.isveren_adi = "İşveren bilgisi zorunludur";
    if (!form.yuklenici_proje_sorumlusu.trim())
      errs.yuklenici_proje_sorumlusu = "Yüklenici Proje Sorumlusu zorunludur";
    if (form.sozlesme_tarihi && !displayToISO(form.sozlesme_tarihi))
      errs.sozlesme_tarihi = "Geçerli tarih girin (gg/aa/yyyy)";
    if (form.yer_teslim_tarihi && !displayToISO(form.yer_teslim_tarihi))
      errs.yer_teslim_tarihi = "Geçerli tarih girin (gg/aa/yyyy)";
    if (form.is_suresi_gun && (isNaN(+form.is_suresi_gun) || +form.is_suresi_gun <= 0))
      errs.is_suresi_gun = "Pozitif tam sayı girin";
    if (form.gecici_kabul_sonrasi_gun && (isNaN(+form.gecici_kabul_sonrasi_gun) || +form.gecici_kabul_sonrasi_gun < 0))
      errs.gecici_kabul_sonrasi_gun = "0 veya daha büyük tam sayı girin";
    if (form.max_artis_orani && (isNaN(+form.max_artis_orani) || +form.max_artis_orani < 0))
      errs.max_artis_orani = "Geçerli oran girin";
    if (form.max_eksilis_orani && (isNaN(+form.max_eksilis_orani) || +form.max_eksilis_orani < 0))
      errs.max_eksilis_orani = "Geçerli oran girin";
    setFieldErr(errs);
    return Object.keys(errs).length === 0;
  }

  async function handleSave() {
    if (!pid || !validate()) return;
    setSaving(true);
    setSaved(false);
    setErr(null);

    const body: any = {
      isveren_adi: form.isveren_adi.trim(),
      yuklenici_proje_sorumlusu: form.yuklenici_proje_sorumlusu.trim(),
      sozlesme_turu: form.sozlesme_turu,
      fiyat_farki_var: form.fiyat_farki_var,
      fiyat_farki_formulu: form.fiyat_farki_var ? form.fiyat_farki_formulu : "",
      sozlesme_bedeli: (form.sozlesme_turu === "goturu_bedel" || form.sozlesme_turu === "karma")
        ? parseBigNumber(form.sozlesme_bedeli) : null,
      sozlesme_para_birimi: form.sozlesme_para_birimi,
      birim_fiyat_kalemleri: (form.sozlesme_turu === "birim_fiyat" || form.sozlesme_turu === "karma")
        ? form.birim_fiyat_kalemleri.map(k => ({
            ...k,
            miktar: parseFloat(String(k.miktar)) || 0,
            birim_fiyat: parseBigNumber(String(k.birim_fiyat)) ?? 0,
          }))
        : [],
      sozlesme_tarihi: displayToISO(form.sozlesme_tarihi) ?? null,
      yer_teslim_tarihi: displayToISO(form.yer_teslim_tarihi) ?? null,
      is_suresi_gun: form.is_suresi_gun ? parseInt(form.is_suresi_gun, 10) : null,
      gecici_kabul_sonrasi_gun: form.gecici_kabul_sonrasi_gun
        ? parseInt(form.gecici_kabul_sonrasi_gun, 10) : null,
      max_artis_orani: form.max_artis_orani ? parseFloat(form.max_artis_orani) : null,
      max_eksilis_orani: form.max_eksilis_orani ? parseFloat(form.max_eksilis_orani) : null,
      sgk_is_yeri_no: form.sgk_is_yeri_no,
      pdf_dosya_url: form.pdf_dosya_url,
      pdf_dosya_adi: form.pdf_dosya_adi,
    };

    try {
      await api(`/projects/${pid}/main-contract`, { method: "PUT", body, projectId: pid });
      setSaved(true);
      setTimeout(() => setSaved(false), 3000);
      await load(true); // meta (is_locked/updated_by) + mode="view" tazelenir
    } catch (e) {
      if (e instanceof RequestError && e.api?.details) {
        setFieldErr(e.api.details as Record<string, string>);
      } else {
        setErr("Kaydedilemedi. Lütfen tekrar deneyin.");
      }
    } finally {
      setSaving(false);
    }
  }

  function handlePdfSelect(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0];
    if (!file) return;
    if (!file.name.endsWith(".pdf")) {
      setFieldErr(fe => ({ ...fe, pdf: "Yalnızca PDF dosyası kabul edilir." }));
      return;
    }
    set("pdf_dosya_adi", file.name);
    set("pdf_dosya_url", "");
    setFieldErr(fe => { const n = { ...fe }; delete n.pdf; return n; });
  }

  if (!current) {
    return <p className="p-6 text-beton-400">Önce üst bardan bir proje seçin.</p>;
  }
  if (loading) {
    return (
      <div className="p-6 flex items-center gap-2 text-beton-400">
        <span className="animate-spin inline-block">⏳</span> Yükleniyor…
      </div>
    );
  }

  const showBirim = form.sozlesme_turu === "birim_fiyat" || form.sozlesme_turu === "karma";
  const showLump  = form.sozlesme_turu === "goturu_bedel" || form.sozlesme_turu === "karma";

  return (
    <div className="max-w-4xl mx-auto p-6 space-y-8">

      {/* Başlık */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-semibold text-beton-100">Ana Sözleşme</h1>
          <p className="text-sm text-beton-400 mt-0.5">{current.name}</p>
        </div>
        {canEdit && mode === "edit" && (
          <button
            onClick={handleSave}
            disabled={saving}
            className="px-5 py-2 rounded-md bg-emniyet-500 hover:bg-emniyet-600 text-beton-950 text-sm font-medium
                       disabled:opacity-50 transition-colors"
          >
            {saving ? "Kaydediliyor…" : "Kaydet ve Kilitle"}
          </button>
        )}
        {canEdit && mode === "view" && (
          <button
            onClick={() => setMode("edit")}
            className="px-5 py-2 rounded-md bg-[var(--group-accent)] text-white-solid text-sm font-medium
                       hover:brightness-95 transition-[filter]"
          >
            Güncelle / Revize Et
          </button>
        )}
      </div>

      {err && (
        <div className="rounded-md bg-red-500/10 border border-red-500/30 px-4 py-3 text-sm text-red-400">
          {err}
        </div>
      )}
      {saved && (
        <div className="rounded-md bg-green-500/10 border border-green-500/30 px-4 py-3 text-sm text-green-400">
          Sözleşme kaydedildi ve kilitlendi.
        </div>
      )}

      {mode === "view" ? (
        <ContractSummary form={form} meta={meta} />
      ) : (
      <>
      {/* ── 0. Taraflar ───────────────────────────────────────────────────── */}
      <Section title="Taraflar">
        <div className="flex flex-col gap-4">
          <Field label="İşveren *" error={fieldErr.isveren_adi}>
            <input
              type="text"
              value={form.isveren_adi}
              disabled={!canEdit}
              onChange={e => set("isveren_adi", e.target.value)}
              placeholder="Örn: T.C. Sağlık Bakanlığı"
              className={inp}
            />
          </Field>
          <Field label="Yüklenici Proje Sorumlusu *" error={fieldErr.yuklenici_proje_sorumlusu}>
            <input
              type="text"
              value={form.yuklenici_proje_sorumlusu}
              disabled={!canEdit}
              onChange={e => set("yuklenici_proje_sorumlusu", e.target.value)}
              placeholder="Ad Soyad"
              className={inp}
            />
          </Field>
        </div>
      </Section>

      {/* ── 1. Sözleşme Türü ─────────────────────────────────────────────── */}
      <Section title="Sözleşme Türü">
        <div className="flex flex-col gap-3">
          {(Object.keys(TUR_LABEL) as SozlesmeTuru[]).map(t => (
            <label key={t} className="flex items-center gap-3 cursor-pointer group">
              <input
                type="radio"
                name="sozlesme_turu"
                value={t}
                checked={form.sozlesme_turu === t}
                disabled={!canEdit}
                onChange={() => set("sozlesme_turu", t)}
                className="accent-blue-500 w-4 h-4 cursor-pointer"
              />
              <span className="text-sm text-beton-100 group-hover:text-emniyet-500 transition-colors">
                {TUR_LABEL[t]}
              </span>
            </label>
          ))}
        </div>
      </Section>

      {/* ── 2. Fiyat Farkı ───────────────────────────────────────────────── */}
      <Section title="Fiyat Farkı">
        <div className="flex flex-col gap-4">
          <div className="flex items-center gap-6">
            {[true, false].map(v => (
              <label key={String(v)} className="flex items-center gap-2 cursor-pointer">
                <input
                  type="radio"
                  name="fiyat_farki"
                  checked={form.fiyat_farki_var === v}
                  disabled={!canEdit}
                  onChange={() => set("fiyat_farki_var", v)}
                  className="accent-blue-500 w-4 h-4 cursor-pointer"
                />
                <span className="text-sm text-beton-100">{v ? "Var" : "Yok"}</span>
              </label>
            ))}
          </div>
          {form.fiyat_farki_var && (
            <div className="flex flex-col gap-1">
              <label className={labelSm}>Fiyat Farkı Formülü</label>
              <input
                type="text"
                value={form.fiyat_farki_formulu}
                disabled={!canEdit}
                onChange={e => set("fiyat_farki_formulu", e.target.value)}
                placeholder="Örn: TÜİK endeksli, Madde 7.1 hükmüne göre…"
                className={inp}
              />
            </div>
          )}
        </div>
      </Section>

      {/* ── 3. Sözleşme Bedeli ───────────────────────────────────────────── */}
      {showLump && (
        <Section title={form.sozlesme_turu === "karma" ? "Götürü Bedel Bölüm Tutarı" : "Sözleşme Bedeli"}>
          <div className="flex gap-3 items-start">
            <div className="w-28 flex flex-col gap-1">
              <label className={labelSm}>Para Birimi</label>
              <select
                value={form.sozlesme_para_birimi}
                disabled={!canEdit}
                onChange={e => set("sozlesme_para_birimi", e.target.value)}
                className={`${inpBase} w-full`}
              >
                {PARA_BIRIMLERI.map(c => <option key={c}>{c}</option>)}
              </select>
            </div>
            <div className="flex-1 flex flex-col gap-1">
              <label className={labelSm}>Tutar</label>
              <input
                type="text"
                inputMode="numeric"
                value={form.sozlesme_bedeli}
                disabled={!canEdit}
                onChange={e => set("sozlesme_bedeli", e.target.value)}
                onBlur={e => set("sozlesme_bedeli", formatBigNumber(e.target.value))}
                placeholder="350.000.000.000,00"
                className={`${inp} text-right font-mono tabular-nums`}
              />
            </div>
          </div>
        </Section>
      )}

      {/* ── 4. Birim Fiyat Kalemleri ─────────────────────────────────────── */}
      {showBirim && (
        <Section title={form.sozlesme_turu === "karma" ? "Birim Fiyatlı Bölüm Kalemleri" : "Birim Fiyat Kalemleri"}>
          <div className="overflow-x-auto">
            <table className="w-full text-sm border-collapse">
              <thead>
                <tr className="text-beton-500 text-left border-b border-beton-800">
                  <th className="pb-2 pr-3 font-medium w-1/3">Tanım</th>
                  <th className="pb-2 pr-3 font-medium w-20">Birim</th>
                  <th className="pb-2 pr-3 font-medium w-24 text-right">Miktar</th>
                  <th className="pb-2 pr-3 font-medium w-28 text-right">Birim Fiyat</th>
                  <th className="pb-2 pr-3 font-medium w-24">Para Birimi</th>
                  {canEdit && <th className="pb-2 w-10" />}
                </tr>
              </thead>
              <tbody>
                {form.birim_fiyat_kalemleri.map((k, i) => (
                  <tr key={k.id} className="border-b border-beton-800/40">
                    <td className="py-2 pr-3">
                      <input
                        type="text"
                        value={k.tanim as string}
                        disabled={!canEdit}
                        onChange={e => updateKalem(i, "tanim", e.target.value)}
                        placeholder="İş kalemi adı"
                        className={inp}
                      />
                    </td>
                    <td className="py-2 pr-3">
                      <input
                        type="text"
                        value={k.birim as string}
                        disabled={!canEdit}
                        onChange={e => updateKalem(i, "birim", e.target.value)}
                        placeholder="m², ton…"
                        className={inp}
                      />
                    </td>
                    <td className="py-2 pr-3">
                      <input
                        type="number"
                        value={k.miktar as any}
                        disabled={!canEdit}
                        onChange={e => updateKalem(i, "miktar", e.target.value)}
                        className={`${inp} text-right tabular-nums`}
                        min={0}
                      />
                    </td>
                    <td className="py-2 pr-3">
                      <input
                        type="text"
                        inputMode="numeric"
                        value={k.birim_fiyat as any}
                        disabled={!canEdit}
                        onChange={e => updateKalem(i, "birim_fiyat", e.target.value)}
                        onBlur={e => updateKalem(i, "birim_fiyat", formatBigNumber(e.target.value))}
                        className={`${inp} text-right font-mono tabular-nums`}
                      />
                    </td>
                    <td className="py-2 pr-3">
                      <select
                        value={k.para_birimi as string}
                        disabled={!canEdit}
                        onChange={e => updateKalem(i, "para_birimi", e.target.value)}
                        className={inp}
                      >
                        {PARA_BIRIMLERI.map(c => <option key={c}>{c}</option>)}
                      </select>
                    </td>
                    {canEdit && (
                      <td className="py-2 text-center">
                        <button
                          onClick={() => removeKalem(i)}
                          className="text-red-400 hover:text-red-300 text-base leading-none transition-colors"
                          title="Kalemi sil"
                        >
                          ✕
                        </button>
                      </td>
                    )}
                  </tr>
                ))}
                {form.birim_fiyat_kalemleri.length === 0 && (
                  <tr>
                    <td colSpan={6} className="py-4 text-center text-beton-500 text-sm">
                      Henüz kalem eklenmedi.
                    </td>
                  </tr>
                )}
              </tbody>
            </table>
          </div>
          {canEdit && (
            <button
              onClick={addKalem}
              className="mt-3 text-sm text-emniyet-500 hover:text-emniyet-600 transition-colors"
            >
              + Kalem Ekle
            </button>
          )}
        </Section>
      )}

      {/* ── 5. Tarihler ve Süre ──────────────────────────────────────────── */}
      <Section title="Tarihler ve Süre">
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-x-6 gap-y-4">

          <Field label="Sözleşme Tarihi" error={fieldErr.sozlesme_tarihi}>
            <input
              type="text"
              value={form.sozlesme_tarihi}
              disabled={!canEdit}
              onChange={e => handleDateChange("sozlesme_tarihi", e.target.value)}
              placeholder="gg/aa/yyyy"
              maxLength={10}
              className={inp}
            />
          </Field>

          <Field label="Yer Teslim Tarihi" error={fieldErr.yer_teslim_tarihi}>
            <input
              type="text"
              value={form.yer_teslim_tarihi}
              disabled={!canEdit}
              onChange={e => handleDateChange("yer_teslim_tarihi", e.target.value)}
              placeholder="gg/aa/yyyy"
              maxLength={10}
              className={inp}
            />
          </Field>

          <Field label="İşin Süresi" error={fieldErr.is_suresi_gun}>
            <div className="flex items-center gap-2">
              <input
                type="number"
                value={form.is_suresi_gun}
                disabled={!canEdit}
                onChange={e => set("is_suresi_gun", e.target.value)}
                placeholder="365"
                min={1}
                className={`${inpBase} w-32`}
              />
              <span className="text-sm text-beton-400">takvim günü</span>
            </div>
          </Field>

          <Field label="Kesin Kabul Tarihi" error={fieldErr.gecici_kabul_sonrasi_gun}>
            <div className="flex items-center gap-2 flex-wrap">
              <span className="text-sm text-beton-400">Geçici Kabulden</span>
              <input
                type="number"
                value={form.gecici_kabul_sonrasi_gun}
                disabled={!canEdit}
                onChange={e => set("gecici_kabul_sonrasi_gun", e.target.value)}
                placeholder="365"
                min={0}
                className={`${inpBase} w-24`}
              />
              <span className="text-sm text-beton-400">gün sonra</span>
            </div>
          </Field>

        </div>
      </Section>

      {/* ── 6. İş Artışı / Eksilişi ─────────────────────────────────────── */}
      <Section title="İş Artışı / Eksilişi">
        <div className="flex flex-wrap gap-6">
          <Field label="Azami Artış Oranı" error={fieldErr.max_artis_orani}>
            <div className="flex items-center gap-2">
              <span className="text-sm text-beton-400 font-medium">+</span>
              <input
                type="number"
                value={form.max_artis_orani}
                disabled={!canEdit}
                onChange={e => set("max_artis_orani", e.target.value)}
                placeholder="20"
                min={0}
                step={0.01}
                className={`${inpBase} w-24`}
              />
              <span className="text-sm text-beton-400">%</span>
            </div>
          </Field>

          <Field label="Azami Ekiliş Oranı" error={fieldErr.max_eksilis_orani}>
            <div className="flex items-center gap-2">
              <span className="text-sm text-beton-400 font-medium">−</span>
              <input
                type="number"
                value={form.max_eksilis_orani}
                disabled={!canEdit}
                onChange={e => set("max_eksilis_orani", e.target.value)}
                placeholder="20"
                min={0}
                step={0.01}
                className={`${inpBase} w-24`}
              />
              <span className="text-sm text-beton-400">%</span>
            </div>
          </Field>
        </div>
      </Section>

      {/* ── 7. SGK ve PDF ────────────────────────────────────────────────── */}
      <Section title="Diğer Bilgiler">
        <div className="flex flex-col gap-4">

          <Field label="SGK İşyeri Numarası (opsiyonel)">
            <input
              type="text"
              value={form.sgk_is_yeri_no}
              disabled={!canEdit}
              onChange={e => set("sgk_is_yeri_no", e.target.value)}
              placeholder="Örn: 047-4120-001"
              className={`${inpBase} w-72`}
            />
          </Field>

          <Field label="Sözleşme PDF Eki" error={fieldErr.pdf}>
            <div className="flex items-center gap-3 flex-wrap">
              {canEdit && (
                <>
                  <input
                    type="file"
                    accept=".pdf"
                    ref={pdfRef}
                    className="hidden"
                    onChange={handlePdfSelect}
                  />
                  <button
                    type="button"
                    onClick={() => pdfRef.current?.click()}
                    className="px-4 py-1.5 rounded border border-beton-700 text-sm
                               text-beton-400 hover:text-beton-100 hover:border-emniyet-500
                               transition-colors"
                  >
                    PDF Seç…
                  </button>
                </>
              )}
              {form.pdf_dosya_adi && (
                <span className="text-sm text-beton-400 flex items-center gap-1.5">
                  <span className="text-base">📄</span>
                  {form.pdf_dosya_adi}
                  {form.pdf_dosya_url && (
                    <a
                      href={form.pdf_dosya_url}
                      target="_blank"
                      rel="noreferrer"
                      className="text-emniyet-500 underline ml-1 text-xs"
                    >
                      Görüntüle
                    </a>
                  )}
                </span>
              )}
              {!form.pdf_dosya_adi && !canEdit && (
                <span className="text-sm text-beton-500">PDF eklenmemiş.</span>
              )}
            </div>
          </Field>

        </div>
      </Section>

      {canEdit && (
        <div className="flex justify-end gap-2 pt-2">
          {meta.isLocked && (
            <button
              onClick={() => load()}
              className="px-6 py-2 rounded-md border border-beton-700 text-sm text-beton-400
                         hover:text-beton-100 hover:bg-beton-800 transition-colors"
            >
              Vazgeç
            </button>
          )}
          <button
            onClick={handleSave}
            disabled={saving}
            className="px-6 py-2 rounded-md bg-emniyet-500 hover:bg-emniyet-600 text-beton-950 text-sm font-medium
                       disabled:opacity-50 transition-colors"
          >
            {saving ? "Kaydediliyor…" : "Sözleşmeyi Kaydet ve Kilitle"}
          </button>
        </div>
      )}
      </>
      )}

    </div>
  );
}

// ── Alt bileşenler ────────────────────────────────────────────────────────────

// ContractSummary — kilitli sözleşmenin salt okunur özeti ("view" modu).
// Tüm form yerine sadece kritik alanları gösterir; ayrıntılı değişiklik
// için "Güncelle / Revize Et" ile tekrar edit moduna geçilir.
function ContractSummary({ form, meta }: {
  form: ContractForm;
  meta: { isLocked: boolean; updatedAt?: string; updatedByName?: string };
}) {
  const showBirim = form.sozlesme_turu === "birim_fiyat" || form.sozlesme_turu === "karma";
  const showLump = form.sozlesme_turu === "goturu_bedel" || form.sozlesme_turu === "karma";

  function updatedAtDisplay(): string {
    if (!meta.updatedAt) return "";
    const d = new Date(meta.updatedAt);
    if (isNaN(d.getTime())) return "";
    return d.toLocaleDateString("tr-TR", { day: "2-digit", month: "2-digit", year: "numeric" }) +
      " " + d.toLocaleTimeString("tr-TR", { hour: "2-digit", minute: "2-digit" });
  }

  return (
    <div className="space-y-6">
      <div className="rounded-md bg-beton-800/40 border border-beton-800 px-4 py-2.5 text-xs text-beton-400 flex items-center gap-1.5">
        <span>🔒</span>
        <span>
          Sözleşme kilitli{meta.updatedByName ? ` — son güncelleyen: ${meta.updatedByName}` : ""}
          {updatedAtDisplay() && ` · ${updatedAtDisplay()}`}
        </span>
      </div>

      <Section title="Taraflar">
        <div className="flex flex-col gap-3 text-sm">
          <SummaryRow label="İşveren" value={form.isveren_adi || "—"} />
          <SummaryRow label="Yüklenici Proje Sorumlusu" value={form.yuklenici_proje_sorumlusu || "—"} />
        </div>
      </Section>

      <Section title="Sözleşme Türü ve Fiyat Farkı">
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-x-6 gap-y-3 text-sm">
          <SummaryRow label="Sözleşme Türü" value={TUR_LABEL[form.sozlesme_turu]} />
          <SummaryRow label="Fiyat Farkı" value={form.fiyat_farki_var ? "Var" : "Yok"} />
          {form.fiyat_farki_var && form.fiyat_farki_formulu && (
            <SummaryRow label="Fiyat Farkı Formülü" value={form.fiyat_farki_formulu} full />
          )}
        </div>
      </Section>

      {showLump && (
        <Section title={form.sozlesme_turu === "karma" ? "Götürü Bedel Bölüm Tutarı" : "Sözleşme Bedeli"}>
          <p className="text-lg font-semibold text-beton-100 tabular-nums">
            {form.sozlesme_bedeli ? `${form.sozlesme_bedeli} ${form.sozlesme_para_birimi}` : "—"}
          </p>
        </Section>
      )}

      {showBirim && (
        <Section title={form.sozlesme_turu === "karma" ? "Birim Fiyatlı Bölüm Kalemleri" : "Birim Fiyat Kalemleri"}>
          {form.birim_fiyat_kalemleri.length === 0 ? (
            <p className="text-sm text-beton-500">Kalem girilmemiş.</p>
          ) : (
            <p className="text-sm text-beton-100">{form.birim_fiyat_kalemleri.length} kalem tanımlı.</p>
          )}
        </Section>
      )}

      <Section title="Tarihler ve Süre">
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-x-6 gap-y-3 text-sm">
          <SummaryRow label="Sözleşme Tarihi" value={form.sozlesme_tarihi || "—"} />
          <SummaryRow label="Yer Teslim Tarihi" value={form.yer_teslim_tarihi || "—"} />
          <SummaryRow label="İşin Süresi" value={form.is_suresi_gun ? `${form.is_suresi_gun} takvim günü` : "—"} />
          <SummaryRow label="Kesin Kabul" value={form.gecici_kabul_sonrasi_gun ? `Geçici kabulden ${form.gecici_kabul_sonrasi_gun} gün sonra` : "—"} />
        </div>
      </Section>

      <Section title="İş Artışı / Eksilişi">
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-x-6 gap-y-3 text-sm">
          <SummaryRow label="Azami Artış Oranı" value={form.max_artis_orani ? `+%${form.max_artis_orani}` : "—"} />
          <SummaryRow label="Azami Eksiliş Oranı" value={form.max_eksilis_orani ? `−%${form.max_eksilis_orani}` : "—"} />
        </div>
      </Section>

      <Section title="Diğer Bilgiler">
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-x-6 gap-y-3 text-sm">
          <SummaryRow label="SGK İşyeri Numarası" value={form.sgk_is_yeri_no || "—"} />
          <SummaryRow label="Sözleşme PDF Eki" value={form.pdf_dosya_adi || "—"} />
        </div>
      </Section>
    </div>
  );
}

function SummaryRow({ label, value, full }: { label: string; value: string; full?: boolean }) {
  return (
    <div className={full ? "sm:col-span-2" : undefined}>
      <p className="text-xs text-beton-500 mb-0.5">{label}</p>
      <p className="text-beton-100">{value}</p>
    </div>
  );
}

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div className="rounded-lg border border-beton-800 bg-beton-900 overflow-hidden">
      <div className="px-5 py-3 border-b border-beton-800 bg-beton-900">
        <h2 className="text-sm font-semibold text-beton-100">{title}</h2>
      </div>
      <div className="px-5 py-4">
        {children}
      </div>
    </div>
  );
}

function Field({ label, error, children }: {
  label: string; error?: string; children: React.ReactNode;
}) {
  return (
    <div className="flex flex-col gap-1">
      <label className={labelSm}>{label}</label>
      {children}
      {error && <p className="text-xs text-red-400">{error}</p>}
    </div>
  );
}
