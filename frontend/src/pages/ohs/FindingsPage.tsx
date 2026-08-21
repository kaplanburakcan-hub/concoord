import { useCallback, useEffect, useRef, useState } from "react";
import { api, apiDownload } from "../../api/client";
import { apiWithOfflineFallback } from "../../offline/queue";
import { useAuth } from "../../auth/AuthContext";
import { useProjects } from "../ProjectContext";
import { useKesinKabulTarihi } from "../../hooks/useKesinKabulTarihi";

// Faz 8 — İSG bulguları: foto (data-URL, offline kuyruk uyumlu) + GPS/lokasyon,
// yaşam döngüsü Open→InProgress→Closed, termin takibi (gecikenler vurgulu).
// Faz 9 — kanban görünüm: bulgular önem derecesine göre sütunlanır, 5. sütun
// (İş Kazası) ayrı ohs_accidents tablosundan gelir (kendi İnceleniyor/Kapandı
// durumu var, findings'in Open/InProgress/Closed zincirinden bağımsız).

type Finding = {
  id: string; inspection_id?: string; subcontractor_id?: string; subcontractor_name?: string;
  severity: string; description: string; location?: string;
  gps_lat?: number; gps_lng?: number; photo_document_id?: string;
  due_date?: string; status: string; overdue: boolean; age_days: number;
  reported_by_name: string; closed_by_name?: string; closed_at?: string; close_note?: string;
  row_version: number; created_at: string;
};
type Accident = {
  id: string; accident_date: string; description: string; status: string;
  created_by_name: string; created_at: string;
  closed_by_name?: string; closed_at?: string; close_note?: string; row_version: number;
};
type FreeDays = { days: number; reference_date: string | null; since_accident: boolean; has_reference: boolean };
type Sub = { id: string; company_name: string };
type DocVersion = { version_no: number; original_name: string };

const SEV_ORDER = ["Observation", "Minor", "Major", "Critical"] as const;
const SEV_LABEL: Record<string, string> = {
  Observation: "Gözlem", Minor: "Küçük", Major: "Büyük", Critical: "Kritik",
};
const SEV_HINT: Record<string, string> = {
  Observation: "bilgilendirme amaçlı", Minor: "düşük risk",
  Major: "orta risk — termin takipli", Critical: "acil müdahale",
};
const SEV_STYLE: Record<string, string> = {
  Observation: "bg-beton-800 text-beton-200 border-beton-700",
  Minor: "bg-emniyet-500/15 text-emniyet-500 border-emniyet-500/40",
  Major: "bg-orange-500/15 text-orange-300 border-orange-500/40",
  Critical: "bg-red-500/15 text-red-300 border-red-500/40",
};
const ST_LABEL: Record<string, string> = { Open: "Açık", InProgress: "Devam Ediyor", Closed: "Kapandı" };
const ST_STYLE: Record<string, string> = {
  Open: "bg-red-500/15 text-red-300 border-red-500/40",
  InProgress: "bg-blue-500/15 text-blue-300 border-blue-500/40",
  Closed: "bg-green-500/15 text-green-300 border-green-500/40",
};
const ACC_ST_LABEL: Record<string, string> = { Investigating: "İnceleniyor", Closed: "Kapandı" };
const ACC_ST_STYLE: Record<string, string> = {
  Investigating: "bg-violet-500/15 text-violet-300 border-violet-500/40",
  Closed: "bg-green-500/15 text-green-300 border-green-500/40",
};

function fileToDataURL(f: File): Promise<string> {
  return new Promise((res, rej) => {
    const r = new FileReader();
    r.onload = () => res(r.result as string);
    r.onerror = () => rej(new Error("dosya okunamadı"));
    r.readAsDataURL(f);
  });
}
function todayISO() { return new Date().toISOString().slice(0, 10); }

export default function FindingsPage() {
  const { current } = useProjects();
  const { can } = useAuth();
  const pid = current?.id;
  const kesinKabul = useKesinKabulTarihi(pid);
  const canInspect = can("ohs.perform_inspection");

  const [findings, setFindings] = useState<Finding[]>([]);
  const [accidents, setAccidents] = useState<Accident[]>([]);
  const [freeDays, setFreeDays] = useState<FreeDays | null>(null);
  const [subs, setSubs] = useState<Sub[]>([]);
  const [statusFilter, setStatusFilter] = useState("");
  const [showForm, setShowForm] = useState(false);
  const [err, setErr] = useState<string | null>(null);
  const [msg, setMsg] = useState<string | null>(null);

  // bulgu formu
  const [severity, setSeverity] = useState("Minor");
  const [desc, setDesc] = useState("");
  const [loc, setLoc] = useState("");
  const [subID, setSubID] = useState("");
  const [due, setDue] = useState("");
  const [gps, setGps] = useState<{ lat: number; lng: number } | null>(null);
  const fileRef = useRef<HTMLInputElement>(null);
  const [busy, setBusy] = useState(false);

  // kaza kaydı formu
  const [showAccidentForm, setShowAccidentForm] = useState(false);
  const [accDate, setAccDate] = useState(todayISO());
  const [accDesc, setAccDesc] = useState("");
  const [accBusy, setAccBusy] = useState(false);

  const load = useCallback(async () => {
    if (!pid) return;
    setErr(null);
    try {
      const r = await api<{ findings: Finding[] }>(
        `/projects/${pid}/ohs/findings${statusFilter ? `?status=${statusFilter}` : ""}`, { projectId: pid });
      setFindings(r.findings);
    } catch { setErr("Bulgular yüklenemedi ya da erişim yetkiniz yok."); }
    try {
      const s = await api<{ subcontractors: Sub[] }>(`/projects/${pid}/subcontractors`, { projectId: pid });
      setSubs(s.subcontractors);
    } catch { /* taşeron listesi yetkisi yoksa form taşeronsuz çalışır */ }
    try {
      const a = await api<{ accidents: Accident[] }>(`/projects/${pid}/ohs-accidents`, { projectId: pid });
      setAccidents(a.accidents);
    } catch { /* kaza kaydı yetkisi yoksa sütun boş kalır */ }
    try {
      const fd = await api<FreeDays>(`/projects/${pid}/ohs-accidents/free-days`, { projectId: pid });
      setFreeDays(fd);
    } catch { /* sayaç yetkisi yoksa gösterilmez */ }
  }, [pid, statusFilter]);

  useEffect(() => { load(); }, [load]);

  function takeGps() {
    navigator.geolocation.getCurrentPosition(
      (p) => setGps({ lat: p.coords.latitude, lng: p.coords.longitude }),
      () => setMsg("Konum alınamadı."),
      { enableHighAccuracy: true, timeout: 10_000 }
    );
  }

  async function create() {
    if (!pid || !desc.trim()) return;
    setBusy(true); setErr(null); setMsg(null);
    try {
      // Foto data-URL olarak gövdeye gömülür: tek JSON istek → offline
      // kuyruklanabilir (backend doküman motoruna yazar).
      let photoB64: string | undefined;
      let photoName: string | undefined;
      const f = fileRef.current?.files?.[0];
      if (f) {
        if (f.size > 8 * 1024 * 1024) { setErr("Fotoğraf 8 MB sınırını aşıyor."); setBusy(false); return; }
        photoB64 = await fileToDataURL(f);
        photoName = f.name;
      }
      const res = await apiWithOfflineFallback({
        method: "POST", path: `/projects/${pid}/ohs/findings`, projectId: pid,
        label: `İSG bulgusu — ${SEV_LABEL[severity]}`,
        body: {
          severity, description: desc.trim(),
          location: loc.trim() || undefined,
          subcontractor_id: subID || undefined,
          due_date: due || undefined,
          gps_lat: gps?.lat, gps_lng: gps?.lng,
          photo_base64: photoB64, photo_name: photoName,
        },
      });
      if (res.queued) setMsg("Bağlantı yok: bulgu cihazda sıraya alındı (foto dahil).");
      setDesc(""); setLoc(""); setSubID(""); setDue(""); setGps(null); setShowForm(false);
      if (fileRef.current) fileRef.current.value = "";
      load();
    } catch (e) {
      setErr(e instanceof Error && e.message ? e.message : "Bulgu kaydedilemedi.");
    } finally { setBusy(false); }
  }

  async function transition(f: Finding, action: "start" | "close") {
    let note = "";
    if (action === "close") {
      note = prompt("Kapatma notu (zorunlu):")?.trim() ?? "";
      if (!note) return;
    }
    try {
      await api(`/projects/${pid}/ohs/findings/${f.id}/${action}`, {
        method: "POST", projectId: pid, body: { note, row_version: f.row_version },
      });
      load();
    } catch { setErr("Durum geçişi yapılamadı (yetki ya da durum uygun değil)."); }
  }

  async function createAccident() {
    if (!pid || !accDesc.trim()) return;
    setAccBusy(true); setErr(null);
    try {
      await api(`/projects/${pid}/ohs-accidents`, {
        method: "POST", projectId: pid,
        body: { accident_date: accDate, description: accDesc.trim() },
      });
      setAccDesc(""); setAccDate(todayISO()); setShowAccidentForm(false);
      load();
    } catch (e) {
      setErr(e instanceof Error && e.message ? e.message : "Kaza kaydı oluşturulamadı.");
    } finally { setAccBusy(false); }
  }

  async function closeAccident(a: Accident) {
    const note = prompt("Kapatma notu (zorunlu):")?.trim() ?? "";
    if (!note) return;
    try {
      await api(`/projects/${pid}/ohs-accidents/${a.id}/close`, {
        method: "POST", projectId: pid, body: { note, row_version: a.row_version },
      });
      load();
    } catch { setErr("Kaza kaydı kapatılamadı (yetki ya da durum uygun değil)."); }
  }

  async function downloadPhoto(documentId: string) {
    try {
      const det = await api<{ versions: DocVersion[] }>(
        `/projects/${pid}/documents/${documentId}`, { projectId: pid });
      const v = det.versions?.[det.versions.length - 1];
      if (v) {
        await apiDownload(
          `/projects/${pid}/documents/${documentId}/versions/${v.version_no}/download`, v.original_name);
      }
    } catch { setErr("Fotoğraf indirilemedi."); }
  }

  if (!current) return <p className="text-beton-400">Önce üst bardan bir proje seçin.</p>;

  const overdue = findings.filter((f) => f.overdue).length;

  function accidentVisible(a: Accident) {
    if (!statusFilter) return true;
    if (statusFilter === "Closed") return a.status === "Closed";
    if (statusFilter === "InProgress") return a.status === "Investigating";
    return false; // "Open" bulgu durumunun kaza kaydında karşılığı yok
  }
  const visibleAccidents = accidents.filter(accidentVisible);

  return (
    <div className="space-y-4">
      <div className="flex items-center gap-3 flex-wrap">
        <h1 className="text-lg font-display font-medium text-beton-100">İSG Bulguları</h1>
        {overdue > 0 && (
          <span className="rounded border border-red-500/40 bg-red-500/10 px-1.5 py-0.5 text-xs text-red-300">
            {overdue} bulgunun termini geçti
          </span>
        )}

        <div className="ml-auto flex items-center gap-3 flex-wrap">
          <div className="flex items-center gap-2 rounded-xl border border-beton-800 bg-beton-900 pl-2.5 pr-3 py-1.5"
            style={{ boxShadow: "var(--shadow)" }}>
            <div className="w-7 h-7 rounded-lg grid place-items-center text-sm bg-[var(--group-accent)] text-white-solid">🛡</div>
            <div>
              <p className="text-base font-extrabold leading-none tabular-nums text-beton-100">
                {freeDays?.has_reference ? freeDays.days : "—"}
              </p>
              <p className="text-[9.5px] uppercase tracking-wide text-beton-500 font-semibold mt-0.5">Kazasız Gün</p>
            </div>
            {canInspect && (
              <button onClick={() => setShowAccidentForm((v) => !v)}
                className="ml-1.5 text-[10px] text-emniyet-500 hover:underline whitespace-nowrap">
                {showAccidentForm ? "vazgeç" : "+ kaza kaydı ekle"}
              </button>
            )}
          </div>

          <select value={statusFilter} onChange={(e) => setStatusFilter(e.target.value)}
            className="rounded-md bg-beton-900 border border-beton-700 px-2 py-1 text-xs text-beton-200">
            <option value="">Tümü</option>
            <option value="Open">Açık</option>
            <option value="InProgress">Devam Ediyor</option>
            <option value="Closed">Kapandı</option>
          </select>
          {canInspect && (
            <button onClick={() => setShowForm((v) => !v)}
              className="rounded-md bg-emniyet-500 px-3 py-1.5 text-xs font-semibold text-beton-950 hover:bg-emniyet-400">
              Yeni Bulgu
            </button>
          )}
        </div>
      </div>
      {err && <p className="text-sm text-red-400">{err}</p>}
      {msg && <p className="text-sm text-emniyet-500">{msg}</p>}

      {showAccidentForm && (
        <div className="rounded-xl border border-beton-800 bg-beton-900 p-4" style={{ boxShadow: "var(--shadow)" }}>
          <div className="flex flex-wrap items-end gap-2">
            <input type="date" value={accDate} onChange={(e) => setAccDate(e.target.value)}
              max={todayISO()}
              className="rounded-md bg-beton-950 border border-beton-800 px-2 py-1.5 text-sm text-beton-100" />
            <input value={accDesc} onChange={(e) => setAccDesc(e.target.value)} placeholder="Kısa açıklama"
              className="flex-1 min-w-[160px] rounded-md bg-beton-950 border border-beton-800 px-2 py-1.5 text-sm text-beton-100" />
            <button onClick={createAccident} disabled={accBusy || !accDesc.trim()}
              className="rounded-md bg-red-500/90 hover:bg-red-500 disabled:opacity-50 text-white-solid text-xs font-semibold px-3 py-1.5">
              {accBusy ? "Kaydediliyor…" : "Kaza kaydını gir"}
            </button>
          </div>
        </div>
      )}

      {showForm && (
        <div className="rounded-lg border border-beton-800 bg-beton-900 p-4 space-y-3">
          <div className="flex flex-wrap gap-3">
            <label className="text-xs text-beton-300">
              Önem
              <select value={severity} onChange={(e) => setSeverity(e.target.value)}
                className="mt-1 block rounded-md bg-beton-950 border border-beton-800 px-2 py-1.5 text-sm text-beton-100">
                {SEV_ORDER.map((k) => <option key={k} value={k}>{SEV_LABEL[k]}</option>)}
              </select>
            </label>
            <label className="text-xs text-beton-300">
              Taşeron (varsa)
              <select value={subID} onChange={(e) => setSubID(e.target.value)}
                className="mt-1 block rounded-md bg-beton-950 border border-beton-800 px-2 py-1.5 text-sm text-beton-100">
                <option value="">—</option>
                {subs.map((s) => <option key={s.id} value={s.id}>{s.company_name}</option>)}
              </select>
            </label>
            <label className="text-xs text-beton-300">
              Termin
              <input type="date" value={due} max={kesinKabul} onChange={(e) => setDue(e.target.value)}
                className="mt-1 block rounded-md bg-beton-950 border border-beton-800 px-2 py-1.5 text-sm text-beton-100" />
            </label>
          </div>
          <label className="block text-xs text-beton-300">
            Açıklama
            <textarea value={desc} onChange={(e) => setDesc(e.target.value)} rows={2}
              className="mt-1 block w-full rounded-md bg-beton-950 border border-beton-800 px-2 py-1.5 text-sm text-beton-100" />
          </label>
          <div className="flex flex-wrap items-center gap-3">
            <input value={loc} onChange={(e) => setLoc(e.target.value)} placeholder="Lokasyon (B blok 3. kat)"
              className="rounded-md bg-beton-950 border border-beton-800 px-2 py-1.5 text-sm text-beton-100" />
            <button onClick={takeGps}
              className="rounded-md border border-beton-700 px-3 py-1.5 text-xs text-beton-300 hover:bg-beton-800">
              {gps ? "GPS ✓" : "GPS al"}
            </button>
            {/* capture: mobil sahada doğrudan kamera (PWA) */}
            <input ref={fileRef} type="file" accept="image/*" capture="environment"
              className="text-xs text-beton-300 file:mr-2 file:rounded file:border-0 file:bg-beton-800 file:px-2 file:py-1 file:text-beton-200" />
            <button onClick={create} disabled={busy || !desc.trim()}
              className="rounded-md bg-emniyet-500 px-3 py-1.5 text-xs font-semibold text-beton-950 hover:bg-emniyet-400 disabled:opacity-40">
              {busy ? "Kaydediliyor…" : "Kaydet"}
            </button>
          </div>
        </div>
      )}

      <div className="grid gap-3 grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-5">
        {SEV_ORDER.map((sev) => {
          const items = findings.filter((f) => f.severity === sev);
          return (
            <div key={sev} className="rounded-lg border border-beton-800 bg-beton-900 overflow-hidden flex flex-col">
              <div className={`px-3 py-2 border-b border-beton-800 ${SEV_STYLE[sev]}`}>
                <div className="flex items-center justify-between">
                  <span className="text-xs font-bold">{SEV_LABEL[sev]}</span>
                  <span className="text-[11px] font-bold opacity-80">{items.length}</span>
                </div>
                <p className="text-[10px] opacity-70 mt-0.5">{SEV_HINT[sev]}</p>
              </div>
              <div className="p-2 space-y-2 flex-1 min-h-[90px]">
                {items.length === 0 && <p className="text-[11px] text-beton-500 px-1 py-2">Kayıt yok.</p>}
                {items.map((f) => (
                  <div key={f.id} className="rounded-md border border-beton-800 bg-beton-950/40 p-2 text-xs space-y-1">
                    <p className="font-semibold text-beton-100 leading-snug"
                      style={{ display: "-webkit-box", WebkitLineClamp: 2, WebkitBoxOrient: "vertical", overflow: "hidden" }}>
                      {f.description}
                    </p>
                    <div className="flex flex-wrap gap-x-2 gap-y-0.5 text-[10.5px] text-beton-400">
                      {f.subcontractor_name && <span>{f.subcontractor_name}</span>}
                      {f.location && <span>📍 {f.location}</span>}
                      {f.photo_document_id && can("documents.download") && (
                        <button onClick={() => downloadPhoto(f.photo_document_id!)}
                          className="text-emniyet-500 hover:underline">Fotoğraf</button>
                      )}
                    </div>
                    <div className="flex items-center justify-between pt-0.5">
                      <span className={f.overdue ? "text-[10px] text-red-400 font-semibold" : "text-[10px] text-beton-500"}>
                        {f.overdue
                          ? `gecikti${f.due_date ? " · " + f.due_date : ""}`
                          : new Date(f.created_at).toLocaleDateString("tr-TR")}
                      </span>
                      <span className={`rounded-full border px-1.5 py-0.5 text-[9.5px] font-semibold ${ST_STYLE[f.status]}`}>
                        {ST_LABEL[f.status]}
                      </span>
                    </div>
                    {f.status !== "Closed" && (
                      <div className="flex gap-2 pt-0.5">
                        {f.status === "Open" && (
                          <button onClick={() => transition(f, "start")}
                            className="text-[10.5px] text-blue-300 hover:underline">Ele Al</button>
                        )}
                        {canInspect && (
                          <button onClick={() => transition(f, "close")}
                            className="text-[10.5px] text-green-300 hover:underline">Kapat</button>
                        )}
                      </div>
                    )}
                    {f.status === "Closed" && f.close_note && (
                      <p className="text-[10px] text-beton-500 pt-0.5">Kapatma: {f.close_note}</p>
                    )}
                  </div>
                ))}
              </div>
            </div>
          );
        })}

        <div className="rounded-lg border border-beton-800 bg-beton-900 overflow-hidden flex flex-col">
          <div className="px-3 py-2 border-b border-beton-800 bg-violet-500/15 text-violet-300">
            <div className="flex items-center justify-between">
              <span className="text-xs font-bold">İş Kazası</span>
              <span className="text-[11px] font-bold opacity-80">{visibleAccidents.length}</span>
            </div>
            <p className="text-[10px] opacity-70 mt-0.5">ohs_accidents kaydı</p>
          </div>
          <div className="p-2 space-y-2 flex-1 min-h-[90px]">
            {visibleAccidents.length === 0 && <p className="text-[11px] text-beton-500 px-1 py-2">Kayıt yok.</p>}
            {visibleAccidents.map((a) => (
              <div key={a.id} className="rounded-md border border-beton-800 bg-beton-950/40 p-2 text-xs space-y-1">
                <p className="font-semibold text-beton-100 leading-snug"
                  style={{ display: "-webkit-box", WebkitLineClamp: 2, WebkitBoxOrient: "vertical", overflow: "hidden" }}>
                  {a.description}
                </p>
                <div className="flex items-center justify-between pt-0.5">
                  <span className="text-[10px] text-beton-500">
                    {new Date(a.accident_date).toLocaleDateString("tr-TR")} · {a.created_by_name}
                  </span>
                  <span className={`rounded-full border px-1.5 py-0.5 text-[9.5px] font-semibold ${ACC_ST_STYLE[a.status]}`}>
                    {ACC_ST_LABEL[a.status]}
                  </span>
                </div>
                {a.status === "Investigating" && canInspect && (
                  <div className="pt-0.5">
                    <button onClick={() => closeAccident(a)}
                      className="text-[10.5px] text-green-300 hover:underline">Kapat</button>
                  </div>
                )}
                {a.status === "Closed" && a.close_note && (
                  <p className="text-[10px] text-beton-500 pt-0.5">Kapatma: {a.close_note} ({a.closed_by_name})</p>
                )}
              </div>
            ))}
          </div>
        </div>
      </div>
    </div>
  );
}
