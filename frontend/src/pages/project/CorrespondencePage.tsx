import { useCallback, useEffect, useRef, useState } from "react";
import { api, apiDownload, apiUpload, RequestError } from "../../api/client";
import { useAuth } from "../../auth/AuthContext";
import { useProjects } from "../ProjectContext";

// Yazışmalar (Gelen/Giden Evrak) — Türkiye'deki kurumsal evrak defteri
// geleneği ile inşaat sektöründeki RFI/Transmittal log pratiğinin (evrak no,
// ball-in-court, cevap süresi) birleşimi. Tek bileşen, iki route (Gelen/Giden)
// — Makine & Ekipman sayfalarındaki `tip` deseninin aynısı.

type Direction = "gelen" | "giden";

type Correspondence = {
  id: string;
  direction: Direction;
  evrak_no: string;
  karsi_evrak_no?: string;
  tarih: string;
  kayit_tarihi: string;
  kurum_kisi: string;
  konu: string;
  kategori: string;
  durum: string;
  cevap_gerekli: boolean;
  cevap_tarihi?: string;
  ilgili_yazi_id?: string;
  ilgili_evrak_no?: string;
  dagitim?: string;
  notlar?: string;
  teslim_yontemi: string;
  created_by_name: string;
  row_version: number;
  created_at: string;
};

type Doc = { id: string; title: string; latest_version?: number };
type DocVersion = { id: string; version_no: number; original_name: string; size_bytes: number };

const KATEGORILER = ["Genel", "Teknik", "İdari", "Mali", "İSG", "Onay Talebi"];
const DURUMLAR = ["Açık", "Cevaplandı", "Kapalı", "Bilgi Amaçlı"];
const TESLIM_YONTEMLERI = ["eposta", "fiziksel"];
const TESLIM_LABEL: Record<string, string> = { eposta: "E-posta", fiziksel: "Fiziksel (imzalı/barkodlu)" };

const DURUM_STYLE: Record<string, string> = {
  "Açık": "bg-yellow-500/15 text-yellow-300 border-yellow-500/40",
  "Cevaplandı": "bg-green-500/15 text-green-300 border-green-500/40",
  "Kapalı": "bg-beton-800 text-beton-300 border-beton-700",
  "Bilgi Amaçlı": "bg-blue-500/15 text-blue-300 border-blue-500/40",
};
const KATEGORI_STYLE: Record<string, string> = {
  "Genel": "bg-beton-800 text-beton-300 border-beton-700",
  "Teknik": "bg-blue-500/15 text-blue-300 border-blue-500/40",
  "İdari": "bg-purple-500/15 text-purple-300 border-purple-500/40",
  "Mali": "bg-emerald-500/15 text-emerald-300 border-emerald-500/40",
  "İSG": "bg-red-500/15 text-red-300 border-red-500/40",
  "Onay Talebi": "bg-amber-500/15 text-amber-300 border-amber-500/40",
};

// Teslim yöntemi ikonu — e-posta için zarf, fiziksel (imzalı/barkodlu kağıt)
// için damgalı belge simgesi. Liste başlıklarında ve filtre pillerinde kullanılır.
function TeslimIcon({ type, className }: { type: string; className?: string }) {
  if (type === "fiziksel") {
    return (
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} className={className}>
        <title>Fiziksel (imzalı/barkodlu kağıt)</title>
        <rect x="4" y="3" width="13" height="18" rx="1.5" />
        <path d="M8 8h5M8 12h5M8 16h3" />
        <circle cx="18" cy="17" r="4" />
        <path d="M16.3 17l1.1 1.1 2.1-2.1" />
      </svg>
    );
  }
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} className={className}>
      <title>E-posta</title>
      <rect x="3" y="5" width="18" height="14" rx="2" />
      <path d="M3.5 6.5l8.5 6 8.5-6" />
    </svg>
  );
}

function fmtTR(iso?: string) {
  if (!iso) return "—";
  const [y, m, d] = iso.split("-");
  return `${d}.${m}.${y}`;
}
function todayISO() { return new Date().toISOString().slice(0, 10); }

const input =
  "w-full rounded-md bg-beton-950 border border-beton-800 px-3 py-2 text-sm text-beton-100 outline-none focus:border-emniyet-500 disabled:opacity-60";
const label = "block text-xs text-beton-400 mb-1";
const th = "text-left text-[10px] font-bold uppercase tracking-wider text-beton-400 pb-2 pr-3 whitespace-nowrap";
const td = "py-2.5 pr-3 text-[12.5px] text-beton-100 border-b border-beton-800/60 align-middle";
const tdM = "py-2.5 pr-3 text-[12.5px] text-beton-400 border-b border-beton-800/60 align-middle";

type FormState = {
  id?: string;
  karsi_evrak_no: string;
  tarih: string;
  kurum_kisi: string;
  konu: string;
  kategori: string;
  durum: string;
  cevap_gerekli: boolean;
  cevap_tarihi: string;
  ilgili_yazi_id: string;
  dagitim: string;
  notlar: string;
  teslim_yontemi: string;
  row_version: number;
};

function emptyForm(): FormState {
  return {
    karsi_evrak_no: "", tarih: todayISO(), kurum_kisi: "", konu: "",
    kategori: "Genel", durum: "Açık", cevap_gerekli: false, cevap_tarihi: "",
    ilgili_yazi_id: "", dagitim: "", notlar: "", teslim_yontemi: "eposta", row_version: 0,
  };
}

export default function CorrespondencePage({ direction, title }: { direction: Direction; title: string }) {
  const { current } = useProjects();
  const { can } = useAuth();
  const pid = current?.id;

  const [list, setList] = useState<Correspondence[]>([]);
  const [err, setErr] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const [filterDurum, setFilterDurum] = useState("");
  const [filterKategori, setFilterKategori] = useState("");
  const [filterTeslim, setFilterTeslim] = useState("");
  const [search, setSearch] = useState("");

  const [formOpen, setFormOpen] = useState(false);
  const [form, setForm] = useState<FormState>(emptyForm());
  const [saveError, setSaveError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [confirmDeleteId, setConfirmDeleteId] = useState<string | null>(null);

  const [expandedId, setExpandedId] = useState<string | null>(null);
  const [docs, setDocs] = useState<Doc[]>([]);
  const [docBusy, setDocBusy] = useState(false);
  const fileRef = useRef<HTMLInputElement>(null);

  const canManage = can("correspondence.create");

  const load = useCallback(async () => {
    if (!pid) return;
    setBusy(true);
    setErr(null);
    try {
      const q = new URLSearchParams({ direction });
      if (filterDurum) q.set("durum", filterDurum);
      if (filterKategori) q.set("kategori", filterKategori);
      if (filterTeslim) q.set("teslim_yontemi", filterTeslim);
      if (search.trim()) q.set("q", search.trim());
      const res = await api<{ correspondences: Correspondence[] }>(
        `/projects/${pid}/correspondences?${q.toString()}`, { projectId: pid }
      );
      setList(res.correspondences ?? []);
    } catch {
      setErr("Yazışmalar yüklenemedi ya da erişim yetkiniz yok.");
    } finally {
      setBusy(false);
    }
  }, [pid, direction, filterDurum, filterKategori, filterTeslim, search]);

  useEffect(() => { load(); }, [load]);

  const today = todayISO();
  const stats = {
    toplam: list.length,
    acik: list.filter((c) => c.durum === "Açık").length,
    suresiGecen: list.filter((c) => c.cevap_gerekli && c.cevap_tarihi && c.cevap_tarihi < today && c.durum !== "Cevaplandı" && c.durum !== "Kapalı").length,
    buAy: list.filter((c) => c.tarih.slice(0, 7) === today.slice(0, 7)).length,
  };

  function openAdd() {
    setForm(emptyForm());
    setSaveError(null);
    setFormOpen(true);
  }

  function openEdit(c: Correspondence) {
    setForm({
      id: c.id,
      karsi_evrak_no: c.karsi_evrak_no ?? "",
      tarih: c.tarih,
      kurum_kisi: c.kurum_kisi,
      konu: c.konu,
      kategori: c.kategori,
      durum: c.durum,
      cevap_gerekli: c.cevap_gerekli,
      cevap_tarihi: c.cevap_tarihi ?? "",
      ilgili_yazi_id: c.ilgili_yazi_id ?? "",
      dagitim: c.dagitim ?? "",
      notlar: c.notlar ?? "",
      teslim_yontemi: c.teslim_yontemi || "eposta",
      row_version: c.row_version,
    });
    setSaveError(null);
    setFormOpen(true);
  }

  async function saveForm() {
    if (!pid) return;
    setSaving(true);
    setSaveError(null);
    const body = {
      direction,
      karsi_evrak_no: form.karsi_evrak_no || null,
      tarih: form.tarih,
      kurum_kisi: form.kurum_kisi,
      konu: form.konu,
      kategori: form.kategori,
      durum: form.durum,
      cevap_gerekli: form.cevap_gerekli,
      cevap_tarihi: form.cevap_gerekli ? (form.cevap_tarihi || null) : null,
      ilgili_yazi_id: form.ilgili_yazi_id || null,
      dagitim: form.dagitim || null,
      notlar: form.notlar || null,
      teslim_yontemi: form.teslim_yontemi,
      row_version: form.row_version,
    };
    try {
      if (form.id) {
        await api(`/projects/${pid}/correspondences/${form.id}`, { method: "PATCH", projectId: pid, body });
      } else {
        await api(`/projects/${pid}/correspondences`, { method: "POST", projectId: pid, body });
      }
      setFormOpen(false);
      await load();
    } catch (e) {
      setSaveError(e instanceof RequestError ? e.message : "Kaydedilemedi.");
    } finally {
      setSaving(false);
    }
  }

  async function doDelete(id: string) {
    if (!pid) return;
    setBusy(true);
    try {
      await api(`/projects/${pid}/correspondences/${id}`, { method: "DELETE", projectId: pid });
      setConfirmDeleteId(null);
      if (expandedId === id) setExpandedId(null);
      await load();
    } catch {
      setErr("Silinemedi.");
    } finally {
      setBusy(false);
    }
  }

  async function toggleExpand(c: Correspondence) {
    if (expandedId === c.id) { setExpandedId(null); return; }
    setExpandedId(c.id);
    if (!pid) return;
    try {
      const d = await api<{ documents: Doc[] }>(
        `/projects/${pid}/documents?entity_type=correspondence&entity_id=${c.id}`, { projectId: pid }
      );
      setDocs(d.documents ?? []);
    } catch { setDocs([]); }
  }

  async function attach(c: Correspondence) {
    const file = fileRef.current?.files?.[0];
    if (!file || !pid) return;
    setDocBusy(true);
    try {
      const doc = await api<{ document: { id: string } }>(`/projects/${pid}/documents`, {
        method: "POST", projectId: pid,
        body: { title: `${c.evrak_no} — ${file.name}`, doc_category: "Other",
                entity_type: "correspondence", entity_id: c.id },
      });
      const fd = new FormData();
      fd.append("file", file);
      await apiUpload(`/projects/${pid}/documents/${doc.document.id}/versions`, fd);
      if (fileRef.current) fileRef.current.value = "";
      const d = await api<{ documents: Doc[] }>(
        `/projects/${pid}/documents?entity_type=correspondence&entity_id=${c.id}`, { projectId: pid }
      );
      setDocs(d.documents ?? []);
    } catch {
      setErr("Ek yüklenemedi.");
    } finally {
      setDocBusy(false);
    }
  }

  async function downloadDoc(d: Doc) {
    if (!d.latest_version || !pid) return;
    try {
      const det = await api<{ versions: DocVersion[] }>(`/projects/${pid}/documents/${d.id}`, { projectId: pid });
      const v = det.versions[0];
      if (v) await apiDownload(`/projects/${pid}/documents/${d.id}/versions/${v.version_no}/download`, v.original_name);
    } catch {
      setErr("İndirme başarısız.");
    }
  }

  if (!pid) {
    return <p className="text-beton-400 text-sm">Önce üst bardan bir proje seçin.</p>;
  }

  return (
    <div className="max-w-5xl mx-auto space-y-4">
      <div className="flex items-center justify-between gap-3 flex-wrap">
        <div>
          <h1 className="font-display font-extrabold text-xl text-white">{title}</h1>
          <p className="text-xs text-beton-500 mt-0.5">
            Projeye ait {direction === "gelen" ? "gelen" : "giden"} yazışma ve evrak kayıtları.
          </p>
        </div>
        {canManage && (
          <button onClick={openAdd}
            className="rounded-md bg-emniyet-500 px-3 py-2 text-sm font-medium text-beton-950 hover:brightness-110">
            + Yeni Kayıt
          </button>
        )}
      </div>

      {err && <p className="text-red-400 text-sm">{err}</p>}

      {/* Özet kartları */}
      <div className="grid grid-cols-2 sm:grid-cols-4 gap-2">
        {[
          { label: "Toplam", value: stats.toplam },
          { label: "Açık", value: stats.acik },
          { label: "Süresi Geçen", value: stats.suresiGecen, warn: stats.suresiGecen > 0 },
          { label: "Bu Ay", value: stats.buAy },
        ].map(({ label: l, value, warn }) => (
          <div key={l} className={`rounded-lg border px-3 py-2.5 text-center ${warn ? "border-red-500/40 bg-red-500/5" : "border-beton-800 bg-beton-950"}`}>
            <div className={`text-lg font-bold tabular-nums ${warn ? "text-red-400" : "text-white"}`}>{value}</div>
            <div className="text-[10px] uppercase tracking-wider text-beton-500 mt-0.5">{l}</div>
          </div>
        ))}
      </div>

      {/* Filtreler */}
      <div className="rounded-lg border border-beton-800 bg-beton-900 p-3 space-y-2.5">
        <input
          className={`${input} max-w-sm`}
          placeholder="Konu, kurum, evrak no ara…"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
        />
        <div className="flex flex-wrap items-center gap-1.5">
          <span className="text-[10px] font-bold uppercase tracking-wider text-beton-500 mr-1">Durum</span>
          <button onClick={() => setFilterDurum("")}
            className={`rounded-full border px-2.5 py-1 text-[11px] font-semibold transition-colors ${filterDurum === "" ? "bg-emniyet-500 border-emniyet-500 text-beton-950" : "border-beton-700 text-beton-400 hover:border-beton-500"}`}>
            Tümü
          </button>
          {DURUMLAR.map((d) => (
            <button key={d} onClick={() => setFilterDurum(filterDurum === d ? "" : d)}
              className={`rounded-full border px-2.5 py-1 text-[11px] font-semibold transition-colors ${filterDurum === d ? DURUM_STYLE[d] : "border-beton-700 text-beton-500 hover:border-beton-500"}`}>
              {d}
            </button>
          ))}
        </div>
        <div className="flex flex-wrap items-center gap-1.5">
          <span className="text-[10px] font-bold uppercase tracking-wider text-beton-500 mr-1">Kategori</span>
          <button onClick={() => setFilterKategori("")}
            className={`rounded-full border px-2.5 py-1 text-[11px] font-semibold transition-colors ${filterKategori === "" ? "bg-emniyet-500 border-emniyet-500 text-beton-950" : "border-beton-700 text-beton-400 hover:border-beton-500"}`}>
            Tümü
          </button>
          {KATEGORILER.map((k) => (
            <button key={k} onClick={() => setFilterKategori(filterKategori === k ? "" : k)}
              className={`rounded-full border px-2.5 py-1 text-[11px] font-semibold transition-colors ${filterKategori === k ? KATEGORI_STYLE[k] : "border-beton-700 text-beton-500 hover:border-beton-500"}`}>
              {k}
            </button>
          ))}
        </div>
        <div className="flex flex-wrap items-center gap-1.5">
          <span className="text-[10px] font-bold uppercase tracking-wider text-beton-500 mr-1">Teslim Yöntemi</span>
          <button onClick={() => setFilterTeslim("")}
            className={`rounded-full border px-2.5 py-1 text-[11px] font-semibold transition-colors ${filterTeslim === "" ? "bg-emniyet-500 border-emniyet-500 text-beton-950" : "border-beton-700 text-beton-400 hover:border-beton-500"}`}>
            Tümü
          </button>
          {TESLIM_YONTEMLERI.map((t) => (
            <button key={t} onClick={() => setFilterTeslim(filterTeslim === t ? "" : t)}
              className={`flex items-center gap-1 rounded-full border px-2.5 py-1 text-[11px] font-semibold transition-colors ${filterTeslim === t ? "bg-emniyet-500 border-emniyet-500 text-beton-950" : "border-beton-700 text-beton-500 hover:border-beton-500"}`}>
              <TeslimIcon type={t} className="w-3 h-3" />
              {TESLIM_LABEL[t]}
            </button>
          ))}
        </div>
      </div>

      {/* Liste */}
      <div className="rounded-lg border border-beton-800 bg-beton-900 overflow-hidden">
        {busy ? (
          <p className="px-4 py-6 text-sm text-beton-500 text-center">Yükleniyor…</p>
        ) : list.length === 0 ? (
          <p className="px-4 py-6 text-sm text-beton-500 text-center">
            Henüz {direction === "gelen" ? "gelen" : "giden"} evrak kaydı yok.
          </p>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full min-w-[720px]">
              <thead>
                <tr>
                  <th className={`${th} pl-4`}>Evrak No</th>
                  <th className={th}>Tarih</th>
                  <th className={th}>{direction === "gelen" ? "Kimden" : "Kime"}</th>
                  <th className={th}>Konu</th>
                  <th className={th}>Kategori</th>
                  <th className={th}>Durum</th>
                  <th className={`${th} pr-4`}></th>
                </tr>
              </thead>
              <tbody>
                {list.map((c) => {
                  const overdue = c.cevap_gerekli && c.cevap_tarihi && c.cevap_tarihi < today && c.durum !== "Cevaplandı" && c.durum !== "Kapalı";
                  const isOpen = expandedId === c.id;
                  return (
                    <>
                      <tr key={c.id} className="hover:bg-beton-800/30 transition-colors cursor-pointer" onClick={() => toggleExpand(c)}>
                        <td className={`${td} pl-4 font-medium whitespace-nowrap`}>{c.evrak_no}</td>
                        <td className={`${tdM} whitespace-nowrap tabular-nums`}>{fmtTR(c.tarih)}</td>
                        <td className={td}>{c.kurum_kisi}</td>
                        <td className={`${td} max-w-[260px]`}>
                          <span className="flex items-center gap-1.5 truncate">
                            <TeslimIcon type={c.teslim_yontemi} className="w-3.5 h-3.5 shrink-0 text-beton-500" />
                            <span className="truncate">{c.konu}</span>
                          </span>
                        </td>
                        <td className={tdM}>{c.kategori}</td>
                        <td className={td}>
                          <span className={`rounded-full border px-2 py-0.5 text-[10.5px] font-semibold ${DURUM_STYLE[c.durum] ?? ""}`}>
                            {c.durum}
                          </span>
                          {overdue && (
                            <span className="ml-1.5 rounded-full border border-red-500/40 bg-red-500/15 px-2 py-0.5 text-[10.5px] font-semibold text-red-400">
                              ⚠ Süresi Geçti
                            </span>
                          )}
                        </td>
                        <td className={`${td} pr-4 text-right whitespace-nowrap`}>
                          {canManage && (
                            <>
                              <button onClick={(e) => { e.stopPropagation(); openEdit(c); }}
                                className="text-xs text-emniyet-500 hover:underline mr-3">Düzenle</button>
                              <button onClick={(e) => { e.stopPropagation(); setConfirmDeleteId(c.id); }}
                                className="text-xs text-red-400 hover:underline">Sil</button>
                            </>
                          )}
                        </td>
                      </tr>
                      {isOpen && (
                        <tr key={`${c.id}-detail`}>
                          <td colSpan={7} className="bg-beton-950/60 px-4 py-3 border-b border-beton-800/60">
                            <div className="grid sm:grid-cols-2 gap-4 text-[12.5px]">
                              <div className="space-y-1.5">
                                {c.karsi_evrak_no && (
                                  <p><span className="text-beton-500">Karşı Kurum Evrak No: </span><span className="text-beton-100">{c.karsi_evrak_no}</span></p>
                                )}
                                <p><span className="text-beton-500">Kayıt Tarihi: </span><span className="text-beton-100">{fmtTR(c.kayit_tarihi)}</span></p>
                                {c.cevap_gerekli && (
                                  <p>
                                    <span className="text-beton-500">Cevap Süresi: </span>
                                    <span className={overdue ? "text-red-400 font-semibold" : "text-beton-100"}>{fmtTR(c.cevap_tarihi)}</span>
                                  </p>
                                )}
                                {c.ilgili_evrak_no && (
                                  <p><span className="text-beton-500">İlgili Yazı: </span><span className="text-beton-100">{c.ilgili_evrak_no}</span></p>
                                )}
                                {c.dagitim && (
                                  <p><span className="text-beton-500">Dağıtım: </span><span className="text-beton-100">{c.dagitim}</span></p>
                                )}
                                {c.notlar && (
                                  <p><span className="text-beton-500">Notlar: </span><span className="text-beton-100">{c.notlar}</span></p>
                                )}
                                <p><span className="text-beton-500">Kaydeden: </span><span className="text-beton-100">{c.created_by_name}</span></p>
                              </div>
                              <div>
                                <p className="text-[10px] font-bold uppercase tracking-wider text-beton-500 mb-1.5">Ekler</p>
                                {docs.length === 0 ? (
                                  <p className="text-beton-600 italic text-[11.5px] mb-2">Henüz ek dosya yok.</p>
                                ) : (
                                  <ul className="space-y-1 mb-2">
                                    {docs.map((d) => (
                                      <li key={d.id}>
                                        <button onClick={() => downloadDoc(d)} className="text-emniyet-500 hover:underline text-left">
                                          📎 {d.title}
                                        </button>
                                      </li>
                                    ))}
                                  </ul>
                                )}
                                {canManage && can("documents.upload") && (
                                  <div className="flex items-center gap-2">
                                    <input ref={fileRef} type="file" className="text-[11px] text-beton-400 flex-1" />
                                    <button onClick={() => attach(c)} disabled={docBusy}
                                      className="rounded-md border border-beton-700 px-2.5 py-1 text-[11px] text-beton-200 hover:border-emniyet-500 disabled:opacity-50 shrink-0">
                                      {docBusy ? "Yükleniyor…" : "Ekle"}
                                    </button>
                                  </div>
                                )}
                              </div>
                            </div>
                          </td>
                        </tr>
                      )}
                    </>
                  );
                })}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {/* Ekle/Düzenle modal */}
      {formOpen && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4">
          <div className="bg-beton-900 border border-beton-700 rounded-xl shadow-2xl p-6 w-full max-w-lg max-h-[90vh] overflow-y-auto space-y-4">
            <h2 className="text-lg font-bold text-beton-100">
              {form.id ? "Yazışmayı Düzenle" : `Yeni ${direction === "gelen" ? "Gelen" : "Giden"} Evrak`}
            </h2>
            {saveError && <p className="text-red-400 text-sm">{saveError}</p>}
            <div className="grid grid-cols-2 gap-3">
              <div>
                <label className={label}>Tarih *</label>
                <input type="date" className={input} value={form.tarih}
                  onChange={(e) => setForm((f) => ({ ...f, tarih: e.target.value }))} />
              </div>
              <div>
                <label className={label}>Karşı Kurum Evrak No</label>
                <input className={input} value={form.karsi_evrak_no}
                  onChange={(e) => setForm((f) => ({ ...f, karsi_evrak_no: e.target.value }))} />
              </div>
              <div className="col-span-2">
                <label className={label}>{direction === "gelen" ? "Kimden" : "Kime"} *</label>
                <input className={input} placeholder="Firma / kurum / kişi adı" value={form.kurum_kisi}
                  onChange={(e) => setForm((f) => ({ ...f, kurum_kisi: e.target.value }))} />
              </div>
              <div className="col-span-2">
                <label className={label}>Konu *</label>
                <input className={input} value={form.konu}
                  onChange={(e) => setForm((f) => ({ ...f, konu: e.target.value }))} />
              </div>
              <div>
                <label className={label}>Kategori</label>
                <select className={input} value={form.kategori}
                  onChange={(e) => setForm((f) => ({ ...f, kategori: e.target.value }))}>
                  {KATEGORILER.map((k) => <option key={k} value={k}>{k}</option>)}
                </select>
              </div>
              <div>
                <label className={label}>Durum</label>
                <select className={input} value={form.durum}
                  onChange={(e) => setForm((f) => ({ ...f, durum: e.target.value }))}>
                  {DURUMLAR.map((d) => <option key={d} value={d}>{d}</option>)}
                </select>
              </div>
              <div className="col-span-2">
                <label className={label}>Teslim Yöntemi</label>
                <select className={input} value={form.teslim_yontemi}
                  onChange={(e) => setForm((f) => ({ ...f, teslim_yontemi: e.target.value }))}>
                  {TESLIM_YONTEMLERI.map((t) => <option key={t} value={t}>{TESLIM_LABEL[t]}</option>)}
                </select>
              </div>
              <div className="col-span-2 flex items-center gap-2">
                <input type="checkbox" id="cevap_gerekli" checked={form.cevap_gerekli}
                  onChange={(e) => setForm((f) => ({ ...f, cevap_gerekli: e.target.checked }))} />
                <label htmlFor="cevap_gerekli" className="text-sm text-beton-200">Cevap gerekli</label>
              </div>
              {form.cevap_gerekli && (
                <div className="col-span-2">
                  <label className={label}>Cevap Süresi (son tarih) *</label>
                  <input type="date" className={input} value={form.cevap_tarihi}
                    onChange={(e) => setForm((f) => ({ ...f, cevap_tarihi: e.target.value }))} />
                </div>
              )}
              <div className="col-span-2">
                <label className={label}>İlgili Yazı (opsiyonel)</label>
                <select className={input} value={form.ilgili_yazi_id}
                  onChange={(e) => setForm((f) => ({ ...f, ilgili_yazi_id: e.target.value }))}>
                  <option value="">— Yok —</option>
                  {list.filter((c) => c.id !== form.id).map((c) => (
                    <option key={c.id} value={c.id}>{c.evrak_no} — {c.konu}</option>
                  ))}
                </select>
              </div>
              <div className="col-span-2">
                <label className={label}>Dağıtım (opsiyonel)</label>
                <input className={input} placeholder="CC: ..." value={form.dagitim}
                  onChange={(e) => setForm((f) => ({ ...f, dagitim: e.target.value }))} />
              </div>
              <div className="col-span-2">
                <label className={label}>Notlar</label>
                <textarea rows={2} className={input} value={form.notlar}
                  onChange={(e) => setForm((f) => ({ ...f, notlar: e.target.value }))} />
              </div>
            </div>
            <div className="flex justify-end gap-2 pt-2">
              <button onClick={() => setFormOpen(false)}
                className="rounded-md border border-beton-700 px-4 py-2 text-sm text-beton-300 hover:border-beton-500">
                İptal
              </button>
              <button onClick={saveForm} disabled={saving || !form.tarih || !form.kurum_kisi.trim() || !form.konu.trim()}
                className="rounded-md bg-emniyet-500 px-4 py-2 text-sm font-medium text-beton-950 hover:brightness-110 disabled:opacity-50">
                {saving ? "Kaydediliyor…" : "Kaydet"}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Silme onayı */}
      {confirmDeleteId && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4">
          <div className="bg-beton-900 border border-beton-700 rounded-xl shadow-2xl p-6 w-full max-w-sm space-y-4">
            <p className="text-sm text-beton-200">Bu yazışma kaydı silinsin mi? Bu işlem geri alınamaz.</p>
            <div className="flex justify-end gap-2">
              <button onClick={() => setConfirmDeleteId(null)}
                className="rounded-md border border-beton-700 px-4 py-2 text-sm text-beton-300 hover:border-beton-500">
                Vazgeç
              </button>
              <button onClick={() => doDelete(confirmDeleteId)}
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
