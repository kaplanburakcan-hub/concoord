import { useCallback, useEffect, useRef, useState } from "react";
import { Link, useParams } from "react-router-dom";
import { api, apiDownload, apiUpload } from "../../api/client";
import { useAuth } from "../../auth/AuthContext";
import { useProjects } from "../ProjectContext";
import { MAR, MAR_STATUS_LABEL, MAR_STATUS_STYLE } from "./MaterialApprovalsPage";

// MAR detay: künye, doküman ekleri (Faz 2 polimorfik motor:
// entity_type='material_approval'), inceleme ve karar aksiyonları.
// Karar notu zorunludur — boş notla karar butonları devre dışıdır ve
// backend de ayrıca reddeder.

type Doc = { id: string; title: string; latest_version?: number };
type DocVersion = { id: string; version_no: number; original_name: string; size_bytes: number };

export default function MaterialApprovalDetailPage() {
  const { id } = useParams<{ id: string }>();
  const { current } = useProjects();
  const { can } = useAuth();
  const pid = current?.id;

  const [mar, setMar] = useState<MAR | null>(null);
  const [docs, setDocs] = useState<Doc[]>([]);
  const [err, setErr] = useState<string | null>(null);
  const [note, setNote] = useState("");
  const [busy, setBusy] = useState(false);
  const fileRef = useRef<HTMLInputElement>(null);

  const load = useCallback(async () => {
    if (!pid || !id) return;
    setErr(null);
    try {
      const m = await api<MAR>(`/projects/${pid}/materials/${id}`, { projectId: pid });
      setMar(m);
      const d = await api<{ documents: Doc[] }>(
        `/projects/${pid}/documents?entity_type=material_approval&entity_id=${id}`, { projectId: pid });
      setDocs(d.documents);
    } catch { setErr("MAR yüklenemedi ya da erişim yetkiniz yok."); }
  }, [pid, id]);

  useEffect(() => { load(); }, [load]);

  async function attach() {
    const file = fileRef.current?.files?.[0];
    if (!file || !pid || !mar) return;
    setBusy(true);
    try {
      // 1) MAR'a bağlı doküman kaydı, 2) ilk versiyon dosyası.
      const doc = await api<{ document: { id: string } }>(`/projects/${pid}/documents`, {
        method: "POST", projectId: pid,
        body: { title: `${mar.mar_no} — ${file.name}`, doc_category: "Submittal",
                entity_type: "material_approval", entity_id: mar.id },
      });
      const fd = new FormData();
      fd.append("file", file);
      await apiUpload(`/projects/${pid}/documents/${doc.document.id}/versions`, fd);
      if (fileRef.current) fileRef.current.value = "";
      await load();
    } catch { setErr("Ek yüklenemedi."); }
    finally { setBusy(false); }
  }

  async function download(d: Doc) {
    if (!d.latest_version) return;
    try {
      const det = await api<{ versions: DocVersion[] }>(`/projects/${pid}/documents/${d.id}`, { projectId: pid });
      const v = det.versions[0];
      if (v) await apiDownload(`/projects/${pid}/documents/${d.id}/versions/${v.version_no}/download`, v.original_name);
    } catch { setErr("İndirme başarısız."); }
  }

  async function act(path: string, body?: unknown) {
    setBusy(true); setErr(null);
    try {
      await api(`/projects/${pid}/materials/${id}/${path}`, { method: "POST", projectId: pid, body });
      setNote("");
      await load();
    } catch { setErr("İşlem başarısız (durum uygun olmayabilir)."); }
    finally { setBusy(false); }
  }

  if (!current) return <p className="text-beton-400">Önce üst bardan bir proje seçin.</p>;
  if (err && !mar) return <p className="text-sm text-red-400">{err}</p>;
  if (!mar) return <p className="text-beton-400">Yükleniyor…</p>;

  const decided = ["Approved", "ConditionallyApproved", "Rejected"].includes(mar.status);

  return (
    <div className="space-y-4 max-w-3xl">
      <div className="flex items-center gap-3 flex-wrap">
        <Link to="/malzeme-onaylari" className="text-beton-400 hover:text-beton-200 text-sm">← Pano</Link>
        <h1 className="text-lg font-display font-bold text-white">{mar.mar_no}</h1>
        <span className={"text-xs px-2 py-0.5 rounded border " + (MAR_STATUS_STYLE[mar.status] || "")}>
          {MAR_STATUS_LABEL[mar.status] || mar.status}
        </span>
      </div>
      {err && <p className="text-sm text-red-400">{err}</p>}

      <div className="rounded-lg border border-beton-800 bg-beton-900 p-4 grid gap-2 sm:grid-cols-2 text-sm">
        <Field label="Malzeme" value={mar.material_name} />
        <Field label="Şartname Ref" value={mar.spec_ref || "—"} />
        <Field label="Üretici" value={mar.manufacturer || "—"} />
        <Field label="Taşeron" value={mar.subcontractor_name || "— (iç talep)"} />
        <Field label="Sunan" value={mar.created_by_name} />
        <Field label="Sunum" value={new Date(mar.created_at).toLocaleString("tr-TR")} />
      </div>

      {decided && (
        <div className={"rounded-lg border p-4 text-sm " + (MAR_STATUS_STYLE[mar.status] || "")}>
          <p className="font-semibold mb-1">Karar: {MAR_STATUS_LABEL[mar.status]}</p>
          <p className="whitespace-pre-wrap">{mar.decision_note}</p>
          <p className="text-xs opacity-80 mt-2">
            {mar.decided_by_name} · {mar.decided_at ? new Date(mar.decided_at).toLocaleString("tr-TR") : ""}
          </p>
        </div>
      )}

      {/* Doküman ekleri */}
      <div className="rounded-lg border border-beton-800 bg-beton-900 p-4">
        <p className="text-sm font-semibold text-beton-200 mb-2">Ekler ({docs.length})</p>
        {docs.length === 0 && <p className="text-xs text-beton-500">Henüz ek yok.</p>}
        <ul className="space-y-1">
          {docs.map((d) => (
            <li key={d.id} className="flex items-center gap-2 text-sm text-beton-200">
              <span className="truncate flex-1">{d.title}</span>
              {can("documents.download") && d.latest_version && (
                <button onClick={() => download(d)} className="text-emniyet-500 hover:underline text-xs">indir</button>
              )}
            </li>
          ))}
        </ul>
        {can("documents.upload") && !decided && (
          <div className="mt-3 flex items-center gap-2">
            <input ref={fileRef} type="file"
              className="text-xs text-beton-300 file:mr-2 file:rounded file:border-0 file:bg-beton-800 file:px-2 file:py-1 file:text-beton-200" />
            <button onClick={attach} disabled={busy}
              className="rounded bg-beton-800 hover:bg-beton-700 disabled:opacity-60 text-beton-100 px-3 py-1 text-xs">
              {busy ? "Yükleniyor…" : "Ek yükle"}
            </button>
          </div>
        )}
      </div>

      {/* İş akışı aksiyonları */}
      {mar.status === "Submitted" && can("material_approvals.review") && (
        <button onClick={() => act("review")} disabled={busy}
          className="rounded bg-emniyet-500/80 hover:bg-emniyet-500 disabled:opacity-60 text-beton-950 font-semibold px-4 py-2 text-sm">
          İncelemeye al ve karara sun
        </button>
      )}

      {mar.status === "UnderReview" && can("material_approvals.decide") && (
        <div className="rounded-lg border border-beton-800 bg-beton-900 p-4 space-y-2">
          <p className="text-sm font-semibold text-beton-200">Karar</p>
          <textarea value={note} onChange={(e) => setNote(e.target.value)} rows={3}
            placeholder="Karar notu (zorunlu) — gerekçe, şartlar, referanslar…"
            className="w-full rounded bg-beton-950 border border-beton-800 px-2 py-1.5 text-sm text-beton-100 outline-none focus:border-emniyet-500" />
          <div className="flex flex-wrap gap-2">
            <DecideBtn label="Onayla" cls="bg-green-600 hover:bg-green-500" disabled={busy || !note.trim()}
              onClick={() => act("decide", { decision: "Approved", decision_note: note.trim() })} />
            <DecideBtn label="Şartlı onayla" cls="bg-emniyet-500 hover:bg-emniyet-600 text-beton-950" disabled={busy || !note.trim()}
              onClick={() => act("decide", { decision: "ConditionallyApproved", decision_note: note.trim() })} />
            <DecideBtn label="Reddet" cls="bg-red-600 hover:bg-red-500" disabled={busy || !note.trim()}
              onClick={() => act("decide", { decision: "Rejected", decision_note: note.trim() })} />
          </div>
          {!note.trim() && <p className="text-[11px] text-beton-500">Karar verebilmek için karar notu girin.</p>}
        </div>
      )}
    </div>
  );
}

function Field({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <p className="text-[11px] uppercase tracking-wide text-beton-500">{label}</p>
      <p className="text-beton-100">{value}</p>
    </div>
  );
}

function DecideBtn({ label, cls, disabled, onClick }: {
  label: string; cls: string; disabled: boolean; onClick: () => void;
}) {
  return (
    <button onClick={onClick} disabled={disabled}
      className={"rounded disabled:opacity-50 text-white font-semibold px-4 py-1.5 text-sm " + cls}>
      {label}
    </button>
  );
}
