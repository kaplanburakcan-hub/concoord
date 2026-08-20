import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { api, RequestError } from "../../api/client";
import { useAuth } from "../../auth/AuthContext";
import { useProjects } from "../ProjectContext";

// Sigorta ve Poliçeler — proje bazlı sigorta poliçesi takibi (İnşaat All
// Risk/CAR-EAR, İşveren Mali Sorumluluk, Üçüncü Şahıs Mali Sorumluluk,
// Nakliyat, Diğer). Ana Sözleşme'deki gibi onay süreci yok — doğrudan
// giriş. PDF eki Ana Sözleşme'yle aynı desende (ad/url alanı, gerçek
// yükleme S3 motoruna bağlı değil — bkz. memory: project_fotograflar_s3_bekliyor).

type PoliceTuru = "car_ear" | "isveren_mali_sorumluluk" | "ucuncu_sahis_mali_sorumluluk" | "nakliyat" | "diger";
type Durum = "aktif" | "suresi_doldu" | "iptal";

type Policy = {
  id?: string;
  police_turu: PoliceTuru;
  sigorta_sirketi: string;
  police_no: string;
  baslangic_tarihi?: string;
  bitis_tarihi?: string;
  teminat_bedeli?: number;
  teminat_para_birimi: string;
  prim_tutari?: number;
  prim_para_birimi: string;
  durum: Durum;
  aciklama: string;
  pdf_dosya_url: string;
  pdf_dosya_adi: string;
  created_by_name?: string;
  created_at?: string;
  row_version: number;
};

const TUR_LABEL: Record<PoliceTuru, string> = {
  car_ear: "İnşaat All Risk (CAR/EAR)",
  isveren_mali_sorumluluk: "İşveren Mali Sorumluluk",
  ucuncu_sahis_mali_sorumluluk: "Üçüncü Şahıs Mali Sorumluluk",
  nakliyat: "Nakliyat",
  diger: "Diğer",
};

// Yazışmalar sayfasındaki Durum rozetleriyle aynı renk paleti (bkz.
// CorrespondencePage.tsx DURUM_STYLE) — okunaklılık için.
const DURUM_META: Record<Durum, { label: string; cls: string }> = {
  aktif: { label: "Aktif", cls: "bg-green-500/15 text-green-300 border-green-500/40" },
  suresi_doldu: { label: "Süresi Doldu", cls: "bg-red-500/15 text-red-300 border-red-500/40" },
  iptal: { label: "İptal", cls: "bg-beton-800 text-beton-500 border-beton-700 line-through" },
};

const PARA_BIRIMLERI = ["TRY", "USD", "EUR", "GBP", "CHF"];

const inpBase =
  "rounded bg-beton-950 border border-beton-800 px-2 py-1 text-sm text-beton-200 " +
  "outline-none focus:border-emniyet-500 disabled:opacity-50";

function fmt(n: number): string {
  return n.toLocaleString("tr-TR", { minimumFractionDigits: 2, maximumFractionDigits: 2 });
}
function parseFmt(s: string): number {
  return parseFloat(s.replace(/\./g, "").replace(",", ".")) || 0;
}
function isoToDisplay(iso?: string): string {
  if (!iso) return "";
  const [y, m, d] = iso.split("-");
  return `${d}/${m}/${y}`;
}
function displayToISO(s: string): string {
  const p = s.split("/");
  if (p.length !== 3 || p[2].length !== 4) return "";
  return `${p[2]}-${p[1].padStart(2, "0")}-${p[0].padStart(2, "0")}`;
}
function maskDate(raw: string): string {
  const d = raw.replace(/\D/g, "").slice(0, 8);
  let o = d;
  if (d.length > 2) o = d.slice(0, 2) + "/" + d.slice(2);
  if (d.length > 4) o = o.slice(0, 5) + "/" + o.slice(5);
  return o;
}
function todayISO() {
  return new Date().toISOString().slice(0, 10);
}
function daysUntil(iso?: string): number | null {
  if (!iso) return null;
  const ms = new Date(iso + "T00:00:00").getTime() - new Date(todayISO() + "T00:00:00").getTime();
  return Math.round(ms / 86400000);
}

function emptyPolicy(): Policy {
  return {
    police_turu: "car_ear", sigorta_sirketi: "", police_no: "",
    baslangic_tarihi: "", bitis_tarihi: "",
    teminat_bedeli: undefined, teminat_para_birimi: "TRY",
    prim_tutari: undefined, prim_para_birimi: "TRY",
    durum: "aktif", aciklama: "", pdf_dosya_url: "", pdf_dosya_adi: "",
    row_version: 0,
  };
}

// ── Form ─────────────────────────────────────────────────────────────────────
function PolicyForm({
  initial, onSave, onCancel,
}: {
  initial: Policy;
  onSave: (p: Policy) => void;
  onCancel: () => void;
}) {
  const [p, setP] = useState<Policy>(initial);
  const [saving, setSaving] = useState(false);
  const [err, setErr] = useState<string | null>(null);
  const pdfRef = useRef<HTMLInputElement>(null);

  function set<K extends keyof Policy>(k: K, v: Policy[K]) {
    setP((prev) => ({ ...prev, [k]: v }));
  }

  async function submit() {
    setSaving(true);
    setErr(null);
    try {
      await onSave(p);
    } catch (e) {
      setErr(e instanceof RequestError ? e.message : "Kaydedilemedi.");
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="p-4 border border-beton-700 rounded-lg bg-beton-900/60 mb-4">
      <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-4">
        <div>
          <label className="text-xs text-beton-500 block mb-1">Poliçe Türü *</label>
          <select className={`${inpBase} w-full`} value={p.police_turu}
            onChange={(e) => set("police_turu", e.target.value as PoliceTuru)}>
            {(Object.keys(TUR_LABEL) as PoliceTuru[]).map((t) => (
              <option key={t} value={t}>{TUR_LABEL[t]}</option>
            ))}
          </select>
        </div>
        <div>
          <label className="text-xs text-beton-500 block mb-1">Sigorta Şirketi *</label>
          <input className={`${inpBase} w-full`} value={p.sigorta_sirketi}
            onChange={(e) => set("sigorta_sirketi", e.target.value)} />
        </div>
        <div>
          <label className="text-xs text-beton-500 block mb-1">Poliçe No *</label>
          <input className={`${inpBase} w-full`} value={p.police_no}
            onChange={(e) => set("police_no", e.target.value)} />
        </div>
        <div>
          <label className="text-xs text-beton-500 block mb-1">Durum</label>
          <select className={`${inpBase} w-full`} value={p.durum}
            onChange={(e) => set("durum", e.target.value as Durum)}>
            {(Object.keys(DURUM_META) as Durum[]).map((d) => (
              <option key={d} value={d}>{DURUM_META[d].label}</option>
            ))}
          </select>
        </div>
        <div>
          <label className="text-xs text-beton-500 block mb-1">Başlangıç Tarihi</label>
          <input className={`${inpBase} w-full`} placeholder="gg/aa/yyyy" maxLength={10}
            value={isoToDisplay(p.baslangic_tarihi)}
            onChange={(e) => set("baslangic_tarihi", displayToISO(maskDate(e.target.value)))} />
        </div>
        <div>
          <label className="text-xs text-beton-500 block mb-1">Bitiş Tarihi</label>
          <input className={`${inpBase} w-full`} placeholder="gg/aa/yyyy" maxLength={10}
            value={isoToDisplay(p.bitis_tarihi)}
            onChange={(e) => set("bitis_tarihi", displayToISO(maskDate(e.target.value)))} />
        </div>
        <div>
          <label className="text-xs text-beton-500 block mb-1">Teminat Bedeli</label>
          <div className="flex gap-1">
            <input className={`${inpBase} w-full text-right`}
              value={p.teminat_bedeli ? fmt(p.teminat_bedeli) : ""}
              onChange={(e) => set("teminat_bedeli", parseFmt(e.target.value))} />
            <select className={`${inpBase} w-20`} value={p.teminat_para_birimi}
              onChange={(e) => set("teminat_para_birimi", e.target.value)}>
              {PARA_BIRIMLERI.map((c) => <option key={c}>{c}</option>)}
            </select>
          </div>
        </div>
        <div>
          <label className="text-xs text-beton-500 block mb-1">Prim Tutarı</label>
          <div className="flex gap-1">
            <input className={`${inpBase} w-full text-right`}
              value={p.prim_tutari ? fmt(p.prim_tutari) : ""}
              onChange={(e) => set("prim_tutari", parseFmt(e.target.value))} />
            <select className={`${inpBase} w-20`} value={p.prim_para_birimi}
              onChange={(e) => set("prim_para_birimi", e.target.value)}>
              {PARA_BIRIMLERI.map((c) => <option key={c}>{c}</option>)}
            </select>
          </div>
        </div>
        <div className="col-span-2 lg:col-span-2">
          <label className="text-xs text-beton-500 block mb-1">Açıklama</label>
          <input className={`${inpBase} w-full`} value={p.aciklama}
            onChange={(e) => set("aciklama", e.target.value)} />
        </div>
        <div className="col-span-2">
          <label className="text-xs text-beton-500 block mb-1">Poliçe PDF Eki</label>
          <div className="flex items-center gap-2">
            <input type="file" accept=".pdf" ref={pdfRef} className="hidden"
              onChange={(e) => {
                const f = e.target.files?.[0];
                if (f) { set("pdf_dosya_adi", f.name); set("pdf_dosya_url", ""); }
              }} />
            <button type="button" onClick={() => pdfRef.current?.click()}
              className="px-3 py-1.5 rounded border border-beton-700 text-xs text-beton-300 hover:border-emniyet-500">
              PDF Seç…
            </button>
            {p.pdf_dosya_adi && <span className="text-xs text-beton-400">📄 {p.pdf_dosya_adi}</span>}
          </div>
        </div>
      </div>
      {err && <p className="text-xs text-red-400 mt-2">{err}</p>}
      <div className="flex gap-2 mt-3">
        <button
          onClick={submit}
          disabled={saving || !p.sigorta_sirketi.trim() || !p.police_no.trim()}
          className="px-3 py-1.5 bg-emniyet-500 hover:bg-emniyet-600 text-beton-950 text-sm rounded-md disabled:opacity-40"
        >
          {saving ? "Kaydediliyor…" : "Kaydet"}
        </button>
        <button onClick={onCancel} className="px-3 py-1.5 text-beton-400 hover:text-beton-200 text-sm">
          İptal
        </button>
      </div>
    </div>
  );
}

// ── Ana Bileşen ───────────────────────────────────────────────────────────────
export default function SigortaPolicelerPage() {
  const { current } = useProjects();
  const { can } = useAuth();
  const pid = current?.id;
  const canEdit = can("projects.edit");

  const [policies, setPolicies] = useState<Policy[]>([]);
  const [loading, setLoading] = useState(true);
  const [adding, setAdding] = useState(false);
  const [editing, setEditing] = useState<Policy | null>(null);
  const [confirmDeleteId, setConfirmDeleteId] = useState<string | null>(null);
  const [filterDurum, setFilterDurum] = useState("");
  const [err, setErr] = useState<string | null>(null);

  const load = useCallback(async () => {
    if (!pid) return;
    setLoading(true);
    try {
      const data = await api<{ policies: Policy[] }>(`/projects/${pid}/insurance-policies`, { projectId: pid });
      setPolicies(data.policies ?? []);
    } catch {
      setErr("Poliçeler yüklenemedi.");
    } finally {
      setLoading(false);
    }
  }, [pid]);

  useEffect(() => { load(); }, [load]);

  async function handleSave(p: Policy) {
    if (!pid) return;
    if (p.id) {
      await api(`/projects/${pid}/insurance-policies/${p.id}`, { method: "PATCH", body: p, projectId: pid });
    } else {
      await api(`/projects/${pid}/insurance-policies`, { method: "POST", body: p, projectId: pid });
    }
    setAdding(false);
    setEditing(null);
    await load();
  }

  async function handleDelete(id: string) {
    if (!pid) return;
    await api(`/projects/${pid}/insurance-policies/${id}`, { method: "DELETE", projectId: pid });
    setConfirmDeleteId(null);
    await load();
  }

  const filtered = useMemo(() => {
    return policies.filter((p) => !filterDurum || p.durum === filterDurum);
  }, [policies, filterDurum]);

  const stats = useMemo(() => {
    const toplam = policies.length;
    const aktif = policies.filter((p) => p.durum === "aktif").length;
    const suresiYaklasan = policies.filter((p) => {
      if (p.durum !== "aktif") return false;
      const d = daysUntil(p.bitis_tarihi);
      return d !== null && d >= 0 && d <= 30;
    }).length;
    const suresiGecmis = policies.filter((p) => {
      const d = daysUntil(p.bitis_tarihi);
      return p.durum === "aktif" && d !== null && d < 0;
    }).length;
    return { toplam, aktif, suresiYaklasan, suresiGecmis };
  }, [policies]);

  if (!current) {
    return <p className="p-6 text-beton-400 text-sm">Önce üst bardan bir proje seçin.</p>;
  }

  return (
    <div className="p-6 max-w-6xl mx-auto">
      <div className="flex items-start justify-between mb-5">
        <div>
          <h1 className="text-xl font-semibold text-beton-100">Sigorta ve Poliçeler</h1>
          <p className="text-beton-400 text-sm mt-0.5">{current.name}</p>
        </div>
        {canEdit && (
          <button
            onClick={() => { setAdding(true); setEditing(null); }}
            className="px-3 py-1.5 bg-emniyet-500 hover:bg-emniyet-600 text-beton-950 text-sm rounded-md"
          >
            + Poliçe Ekle
          </button>
        )}
      </div>

      {err && <p className="text-red-400 text-sm mb-3">{err}</p>}

      {/* Özet kartları */}
      <div className="grid grid-cols-2 sm:grid-cols-4 gap-2 mb-5">
        <div className="rounded-lg border border-beton-800 bg-beton-950 px-3 py-2.5 text-center">
          <div className="text-lg font-bold tabular-nums text-beton-100">{stats.toplam}</div>
          <div className="text-[10px] uppercase tracking-wider text-beton-500 mt-0.5">Toplam</div>
        </div>
        <div className="rounded-lg border border-beton-800 bg-beton-950 px-3 py-2.5 text-center">
          <div className="text-lg font-bold tabular-nums text-green-400">{stats.aktif}</div>
          <div className="text-[10px] uppercase tracking-wider text-beton-500 mt-0.5">Aktif</div>
        </div>
        <div className={`rounded-lg border px-3 py-2.5 text-center ${stats.suresiYaklasan > 0 ? "border-yellow-500/40 bg-yellow-500/5" : "border-beton-800 bg-beton-950"}`}>
          <div className={`text-lg font-bold tabular-nums ${stats.suresiYaklasan > 0 ? "text-yellow-400" : "text-beton-100"}`}>{stats.suresiYaklasan}</div>
          <div className="text-[10px] uppercase tracking-wider text-beton-500 mt-0.5">30 Gün İçinde Dolacak</div>
        </div>
        <div className={`rounded-lg border px-3 py-2.5 text-center ${stats.suresiGecmis > 0 ? "border-red-500/40 bg-red-500/5" : "border-beton-800 bg-beton-950"}`}>
          <div className={`text-lg font-bold tabular-nums ${stats.suresiGecmis > 0 ? "text-red-400" : "text-beton-100"}`}>{stats.suresiGecmis}</div>
          <div className="text-[10px] uppercase tracking-wider text-beton-500 mt-0.5">Süresi Geçmiş</div>
        </div>
      </div>

      {adding && <PolicyForm initial={emptyPolicy()} onSave={handleSave} onCancel={() => setAdding(false)} />}
      {editing && <PolicyForm initial={editing} onSave={handleSave} onCancel={() => setEditing(null)} />}

      {policies.length > 0 && (
        <div className="rounded-lg border border-beton-800 bg-beton-900 p-3 mb-3 flex gap-2 flex-wrap items-center">
          <span className="text-[10px] font-bold uppercase tracking-wider text-beton-500 mr-1">Durum</span>
          <button onClick={() => setFilterDurum("")}
            className={`rounded-full border px-2.5 py-1 text-[11px] font-semibold transition-colors ${filterDurum === "" ? "bg-emniyet-500 border-emniyet-500 text-beton-950" : "border-beton-700 text-beton-400 hover:border-beton-500"}`}>
            Tümü
          </button>
          {(Object.keys(DURUM_META) as Durum[]).map((d) => (
            <button key={d} onClick={() => setFilterDurum(filterDurum === d ? "" : d)}
              className={`rounded-full border px-2.5 py-1 text-[11px] font-semibold transition-colors ${filterDurum === d ? DURUM_META[d].cls : "border-beton-700 text-beton-500 hover:border-beton-500"}`}>
              {DURUM_META[d].label}
            </button>
          ))}
        </div>
      )}

      {loading ? (
        <p className="text-beton-500 text-sm">Yükleniyor…</p>
      ) : filtered.length === 0 ? (
        <p className="text-beton-500 text-sm text-center py-10">
          {policies.length === 0 ? "Henüz poliçe eklenmemiş." : "Filtreyle eşleşen kayıt yok."}
        </p>
      ) : (
        <div className="border border-beton-800 rounded-lg overflow-hidden overflow-x-auto">
          <table className="w-full text-sm min-w-[860px]">
            <thead>
              <tr className="bg-beton-900 border-b border-beton-800">
                <th className="py-2 px-3 text-left text-xs text-beton-500 font-medium">Poliçe Türü</th>
                <th className="py-2 px-3 text-left text-xs text-beton-500 font-medium">Sigorta Şirketi</th>
                <th className="py-2 px-3 text-left text-xs text-beton-500 font-medium">Poliçe No</th>
                <th className="py-2 px-3 text-left text-xs text-beton-500 font-medium">Başlangıç</th>
                <th className="py-2 px-3 text-left text-xs text-beton-500 font-medium">Bitiş</th>
                <th className="py-2 px-3 text-right text-xs text-beton-500 font-medium">Teminat</th>
                <th className="py-2 px-3 text-right text-xs text-beton-500 font-medium">Prim</th>
                <th className="py-2 px-3 text-left text-xs text-beton-500 font-medium">Durum</th>
                <th className="w-24" />
              </tr>
            </thead>
            <tbody>
              {filtered.map((p) => {
                const d = daysUntil(p.bitis_tarihi);
                const yaklasiyor = p.durum === "aktif" && d !== null && d >= 0 && d <= 30;
                const gecmis = p.durum === "aktif" && d !== null && d < 0;
                return (
                  <tr key={p.id} className="border-b border-beton-800/50 hover:bg-beton-900/30 group">
                    <td className="py-2 px-3 text-beton-200 text-sm">{TUR_LABEL[p.police_turu]}</td>
                    <td className="py-2 px-3 text-beton-200 text-sm">{p.sigorta_sirketi}</td>
                    <td className="py-2 px-3 text-beton-400 text-xs font-mono">{p.police_no}</td>
                    <td className="py-2 px-3 text-beton-400 text-xs">{isoToDisplay(p.baslangic_tarihi) || "—"}</td>
                    <td className="py-2 px-3 text-xs">
                      <span className={gecmis ? "text-red-400 font-semibold" : yaklasiyor ? "text-yellow-400 font-semibold" : "text-beton-400"}>
                        {isoToDisplay(p.bitis_tarihi) || "—"}
                      </span>
                    </td>
                    <td className="py-2 px-3 text-right text-beton-200 text-xs font-mono">
                      {p.teminat_bedeli ? `${fmt(p.teminat_bedeli)} ${p.teminat_para_birimi}` : "—"}
                    </td>
                    <td className="py-2 px-3 text-right text-beton-400 text-xs font-mono">
                      {p.prim_tutari ? `${fmt(p.prim_tutari)} ${p.prim_para_birimi}` : "—"}
                    </td>
                    <td className="py-2 px-3">
                      <span className={`inline-block rounded-full border px-2 py-0.5 text-[10.5px] font-semibold ${DURUM_META[p.durum].cls}`}>
                        {DURUM_META[p.durum].label}
                      </span>
                      {gecmis && (
                        <span className="ml-1.5 rounded-full border border-red-500/40 bg-red-500/15 px-2 py-0.5 text-[10.5px] font-semibold text-red-400">
                          ⚠ Süresi Geçti
                        </span>
                      )}
                    </td>
                    <td className="py-2 px-3">
                      {canEdit && (
                        <div className="flex gap-2 opacity-0 group-hover:opacity-100 transition-opacity">
                          <button onClick={() => { setEditing(p); setAdding(false); }}
                            className="text-xs text-emniyet-400 hover:text-emniyet-300">
                            Düzenle
                          </button>
                          <button onClick={() => p.id && setConfirmDeleteId(p.id)}
                            className="text-xs text-red-500 hover:text-red-400">
                            Sil
                          </button>
                        </div>
                      )}
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}

      {confirmDeleteId && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4">
          <div className="bg-beton-900 border border-beton-700 rounded-xl shadow-2xl p-6 w-full max-w-sm space-y-4">
            <p className="text-sm text-beton-200">Bu poliçe kaydı silinsin mi? Bu işlem geri alınamaz.</p>
            <div className="flex justify-end gap-2">
              <button onClick={() => setConfirmDeleteId(null)}
                className="rounded-md border border-beton-700 px-4 py-2 text-sm text-beton-300 hover:border-beton-500">
                Vazgeç
              </button>
              <button onClick={() => handleDelete(confirmDeleteId)}
                className="rounded-md bg-red-500 px-4 py-2 text-sm font-medium text-white-solid hover:brightness-110">
                Sil
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
