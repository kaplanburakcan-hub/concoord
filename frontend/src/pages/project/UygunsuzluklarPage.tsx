import { useCallback, useEffect, useState } from "react";
import { api, describeLoadError } from "../../api/client";
import { useAuth } from "../../auth/AuthContext";
import { useProjects } from "../../projects/ProjectContext";

// Uygunsuzluk (NCR) kaydı, ilk sürüm (E4). Kayıt silinmez; kapatılır. Muayene
// ve hold noktaları bu aşamada yoktur.

type NCR = {
  id: string; no: number; title: string; poz_no?: string; description?: string;
  owner_name?: string; status: string; created_by_name: string; created_at: string;
  closed_by_name?: string; closed_at?: string;
};

const inp = "rounded bg-beton-950 border border-beton-800 px-2 py-1.5 text-sm text-beton-100 outline-none focus:border-emniyet-500";

export default function UygunsuzluklarPage() {
  const { current } = useProjects();
  const { can } = useAuth();
  const pid = current?.id;
  const canManage = can("ncr.manage");

  const [ncrs, setNcrs] = useState<NCR[]>([]);
  const [err, setErr] = useState<string | null>(null);
  const [showForm, setShowForm] = useState(false);
  const [title, setTitle] = useState("");
  const [poz, setPoz] = useState("");
  const [description, setDescription] = useState("");
  const [saving, setSaving] = useState(false);
  const [subID, setSubID] = useState("");
  const [subs, setSubs] = useState<{ id: string; company_name: string }[]>([]);
  // Kapatma akışı: önce kanıt belgesi seçilir (E4 kapanış kanıtı).
  const [closingId, setClosingId] = useState<string | null>(null);
  const [closeDoc, setCloseDoc] = useState("");
  const [docs, setDocs] = useState<{ id: string; title: string }[]>([]);

  const load = useCallback(async () => {
    if (!pid) return;
    setErr(null);
    try {
      const r = await api<{ ncrs: NCR[] }>(`/projects/${pid}/ncrs`, { projectId: pid });
      setNcrs(r.ncrs ?? []);
    } catch (e) {
      setErr(describeLoadError(e, "Uygunsuzluk kayıtları"));
    }
  }, [pid]);

  useEffect(() => { load(); }, [load]);
  useEffect(() => {
    if (!pid) return;
    api<{ subcontractors: { id: string; company_name: string }[] }>(`/projects/${pid}/subcontractors`, { projectId: pid })
      .then((r) => setSubs(r.subcontractors ?? []))
      .catch(() => setSubs([]));
  }, [pid]);

  async function create() {
    if (!pid || !title.trim()) return;
    setSaving(true);
    try {
      await api(`/projects/${pid}/ncrs`, {
        method: "POST", projectId: pid,
        body: {
          title: title.trim(), poz_no: poz.trim() || undefined, description: description.trim() || undefined,
          subcontractor_id: subID || undefined,
        },
      });
      setTitle(""); setPoz(""); setDescription(""); setSubID(""); setShowForm(false);
      await load();
    } catch (e) {
      setErr(describeLoadError(e, "Uygunsuzluk kaydı"));
    } finally {
      setSaving(false);
    }
  }

  async function openClose(n: NCR) {
    setClosingId(n.id); setCloseDoc(""); setErr(null);
    if (!pid) return;
    try {
      const r = await api<{ documents: { id: string; title: string }[] }>(`/projects/${pid}/documents`, { projectId: pid });
      setDocs(r.documents ?? []);
    } catch { setDocs([]); }
  }

  async function confirmClose(n: NCR) {
    if (!pid || !closeDoc) return;
    try {
      await api(`/projects/${pid}/ncrs/${n.id}/close`, {
        method: "POST", projectId: pid, body: { document_id: closeDoc },
      });
      setClosingId(null);
      await load();
    } catch (e) {
      setErr(describeLoadError(e, "Kapatma"));
    }
  }

  if (!current) return <p className="text-beton-400">Önce üst bardan bir proje seçin.</p>;

  return (
    <div className="space-y-4">
      <div className="flex items-center gap-3 flex-wrap">
        <div>
          <h1 className="text-lg font-display font-bold text-white">Uygunsuzluklar</h1>
          <p className="text-xs text-beton-500 mt-0.5">Açık uygunsuzluklar listelenir. Kapatılan kayıtlar silinmez.</p>
        </div>
        {canManage && (
          <button onClick={() => setShowForm((v) => !v)}
            className="ml-auto rounded-md bg-emniyet-500 hover:bg-emniyet-600 text-beton-950 text-xs font-semibold px-3 py-1.5">
            {showForm ? "Vazgeç" : "+ Uygunsuzluk aç"}
          </button>
        )}
      </div>

      {err && <p className="text-sm text-red-400">{err}</p>}

      {showForm && (
        <div className="rounded-lg border border-beton-800 bg-beton-900 p-4 grid gap-3 md:grid-cols-4">
          <input value={title} onChange={(e) => setTitle(e.target.value)} placeholder="Başlık *" className={`${inp} md:col-span-2`} />
          <input value={poz} onChange={(e) => setPoz(e.target.value)} placeholder="Poz no (isteğe bağlı)" className={inp} />
          <select value={subID} onChange={(e) => setSubID(e.target.value)} className={inp}>
            <option value="">Taşeron (isteğe bağlı)</option>
            {subs.map((x) => <option key={x.id} value={x.id}>{x.company_name}</option>)}
          </select>
          <textarea value={description} onChange={(e) => setDescription(e.target.value)} rows={2}
            placeholder="Açıklama" className={`${inp} md:col-span-4`} />
          <div className="md:col-span-4 flex justify-end">
            <button onClick={create} disabled={saving || !title.trim()}
              className="rounded-md bg-emniyet-500 px-4 py-1.5 text-sm font-semibold text-beton-950 disabled:opacity-50">
              Kaydet
            </button>
          </div>
        </div>
      )}

      <div className="overflow-x-auto rounded-lg border border-beton-800">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-beton-800 bg-beton-900/60 text-left text-[10px] font-bold uppercase tracking-wider text-beton-400">
              <th className="py-2 px-3">No</th>
              <th className="py-2 px-3">Uygunsuzluk</th>
              <th className="py-2 px-3">Poz</th>
              <th className="py-2 px-3">Sorumlu</th>
              <th className="py-2 px-3">Durum</th>
              <th className="py-2 px-3" />
            </tr>
          </thead>
          <tbody>
            {ncrs.length === 0 && (
              <tr><td colSpan={6} className="py-6 text-center text-beton-500">Uygunsuzluk kaydı yok.</td></tr>
            )}
            {ncrs.map((n) => (
              <tr key={n.id} className="border-b border-beton-800/60 align-top">
                <td className="py-2 px-3 font-mono text-xs text-beton-400">U-{String(n.no).padStart(3, "0")}</td>
                <td className="py-2 px-3">
                  <div className="text-beton-100">{n.title}</div>
                  {n.description && <div className="text-xs text-beton-500 mt-0.5">{n.description}</div>}
                  <div className="text-[11px] text-beton-600 mt-0.5">{n.created_by_name} · {n.created_at}</div>
                </td>
                <td className="py-2 px-3 font-mono text-xs text-beton-300">{n.poz_no ?? "—"}</td>
                <td className="py-2 px-3 text-beton-300">{n.owner_name ?? "—"}</td>
                <td className="py-2 px-3">
                  {n.status === "Açık"
                    ? <span className="rounded border border-amber-500/40 px-1.5 py-0.5 text-xs text-amber-300">Açık</span>
                    : <span className="text-xs text-beton-400">Kapandı · {n.closed_by_name} · {n.closed_at}</span>}
                </td>
                <td className="py-2 px-3 text-right">
                  {canManage && n.status === "Açık" && closingId !== n.id && (
                    <button onClick={() => openClose(n)} className="text-xs text-emniyet-500 hover:underline">Kapat</button>
                  )}
                  {canManage && closingId === n.id && (
                    <div className="flex flex-col items-end gap-1">
                      <select value={closeDoc} onChange={(e) => setCloseDoc(e.target.value)} className={`${inp} text-xs max-w-[220px]`}>
                        <option value="">— kanıt belgesi seç —</option>
                        {docs.map((d) => <option key={d.id} value={d.id}>{d.title}</option>)}
                      </select>
                      <div className="flex gap-2">
                        <button onClick={() => confirmClose(n)} disabled={!closeDoc}
                          className="text-xs text-emniyet-500 hover:underline disabled:opacity-50">Kanıtla kapat</button>
                        <button onClick={() => setClosingId(null)} className="text-xs text-beton-400 hover:underline">Vazgeç</button>
                      </div>
                    </div>
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}
