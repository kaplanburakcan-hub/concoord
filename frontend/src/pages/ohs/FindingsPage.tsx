import { useCallback, useEffect, useRef, useState } from "react";
import { api, apiDownload } from "../../api/client";
import { apiWithOfflineFallback } from "../../offline/queue";
import { useAuth } from "../../auth/AuthContext";
import { useProjects } from "../ProjectContext";

// Faz 8 — İSG bulguları: foto (data-URL, offline kuyruk uyumlu) + GPS/lokasyon,
// yaşam döngüsü Open→InProgress→Closed, termin takibi (gecikenler vurgulu).

type Finding = {
  id: string; inspection_id?: string; subcontractor_id?: string; subcontractor_name?: string;
  severity: string; description: string; location?: string;
  gps_lat?: number; gps_lng?: number; photo_document_id?: string;
  due_date?: string; status: string; overdue: boolean; age_days: number;
  reported_by_name: string; closed_by_name?: string; closed_at?: string; close_note?: string;
  row_version: number; created_at: string;
};
type Sub = { id: string; company_name: string };
type DocVersion = { version_no: number; original_name: string };

const SEV_LABEL: Record<string, string> = {
  Observation: "Gözlem", Minor: "Küçük", Major: "Büyük", Critical: "Kritik",
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

function fileToDataURL(f: File): Promise<string> {
  return new Promise((res, rej) => {
    const r = new FileReader();
    r.onload = () => res(r.result as string);
    r.onerror = () => rej(new Error("dosya okunamadı"));
    r.readAsDataURL(f);
  });
}

export default function FindingsPage() {
  const { current } = useProjects();
  const { can } = useAuth();
  const pid = current?.id;
  const canInspect = can("ohs.perform_inspection");

  const [findings, setFindings] = useState<Finding[]>([]);
  const [subs, setSubs] = useState<Sub[]>([]);
  const [statusFilter, setStatusFilter] = useState("");
  const [showForm, setShowForm] = useState(false);
  const [err, setErr] = useState<string | null>(null);
  const [msg, setMsg] = useState<string | null>(null);

  // form
  const [severity, setSeverity] = useState("Minor");
  const [desc, setDesc] = useState("");
  const [loc, setLoc] = useState("");
  const [subID, setSubID] = useState("");
  const [due, setDue] = useState("");
  const [gps, setGps] = useState<{ lat: number; lng: number } | null>(null);
  const fileRef = useRef<HTMLInputElement>(null);
  const [busy, setBusy] = useState(false);

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

  return (
    <div className="space-y-4 max-w-3xl">
      <div className="flex items-center gap-3 flex-wrap">
        {/* Modül içi geçiş kenar çubuğu alt başlıklarında (İSG → Bulgular /
            Denetimler / Cezalar); başlık yanındaki soluk bağlantılar kaldırıldı. */}
        <h1 className="text-lg font-display font-medium text-beton-100">İSG Bulguları</h1>
        {overdue > 0 && (
          <span className="rounded border border-red-500/40 bg-red-500/10 px-1.5 py-0.5 text-xs text-red-300">
            {overdue} bulgunun termini geçti
          </span>
        )}
        <select value={statusFilter} onChange={(e) => setStatusFilter(e.target.value)}
          className="ml-auto rounded-md bg-beton-900 border border-beton-700 px-2 py-1 text-xs text-beton-200">
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
      {err && <p className="text-sm text-red-400">{err}</p>}
      {msg && <p className="text-sm text-emniyet-500">{msg}</p>}

      {showForm && (
        <div className="rounded-lg border border-beton-800 bg-beton-900 p-4 space-y-3">
          <div className="flex flex-wrap gap-3">
            <label className="text-xs text-beton-300">
              Önem
              <select value={severity} onChange={(e) => setSeverity(e.target.value)}
                className="mt-1 block rounded-md bg-beton-950 border border-beton-800 px-2 py-1.5 text-sm text-beton-100">
                {Object.entries(SEV_LABEL).map(([k, v]) => <option key={k} value={k}>{v}</option>)}
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
              <input type="date" value={due} onChange={(e) => setDue(e.target.value)}
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

      <div className="rounded-lg border border-beton-800 divide-y divide-beton-800">
        {findings.map((f) => (
          <div key={f.id} className="px-3 py-2 text-sm space-y-1">
            <div className="flex items-center gap-2 flex-wrap">
              <span className={`rounded border px-1.5 py-0.5 text-xs ${SEV_STYLE[f.severity]}`}>
                {SEV_LABEL[f.severity]}
              </span>
              <span className={`rounded border px-1.5 py-0.5 text-xs ${ST_STYLE[f.status]}`}>
                {ST_LABEL[f.status]}
              </span>
              {f.overdue && (
                <span className="rounded border border-red-500/40 bg-red-500/10 px-1.5 py-0.5 text-xs text-red-300">
                  termin geçti{f.due_date ? ` (${f.due_date})` : ""}
                </span>
              )}
              {f.status !== "Closed" && (
                <span className="text-xs text-beton-500">{f.age_days} gündür açık</span>
              )}
              <span className="ml-auto text-xs text-beton-400">
                {new Date(f.created_at).toLocaleDateString("tr-TR")} · {f.reported_by_name}
              </span>
            </div>
            <p className="text-beton-100">{f.description}</p>
            <div className="flex items-center gap-3 text-xs text-beton-400 flex-wrap">
              {f.subcontractor_name && <span>Taşeron: {f.subcontractor_name}</span>}
              {f.location && <span>{f.location}</span>}
              {f.gps_lat != null && f.gps_lng != null && (
                <span>GPS: {f.gps_lat.toFixed(5)}, {f.gps_lng.toFixed(5)}</span>
              )}
              {f.photo_document_id && can("documents.download") && (
                <button onClick={() => downloadPhoto(f.photo_document_id!)}
                  className="text-emniyet-500 hover:underline">Fotoğraf</button>
              )}
              {f.status === "Closed" && f.close_note && (
                <span className="text-beton-500">Kapatma: {f.close_note} ({f.closed_by_name})</span>
              )}
              <span className="ml-auto flex gap-3">
                {f.status === "Open" && (
                  <button onClick={() => transition(f, "start")}
                    className="text-blue-300 hover:underline">Ele Al</button>
                )}
                {f.status !== "Closed" && canInspect && (
                  <button onClick={() => transition(f, "close")}
                    className="text-green-300 hover:underline">Kapat</button>
                )}
              </span>
            </div>
          </div>
        ))}
        {!findings.length && <p className="px-3 py-4 text-sm text-beton-500">Bulgu yok.</p>}
      </div>
    </div>
  );
}
