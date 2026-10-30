import { useCallback, useEffect, useState } from "react";
import { api, describeLoadError } from "../../api/client";
import { useAuth } from "../../auth/AuthContext";
import { useProjects } from "../../projects/ProjectContext";

// Risk kaydı (E3, ilk sürüm). Kayıt silinmez; durum değişir. Skor = olasılık × etki.

type Risk = {
  id: string; no: number; title: string; description?: string;
  probability: number; impact: number; score: number;
  owner_name?: string; status: string; row_version: number;
};

const STATUSES = ["Açık", "İzleniyor", "Kapandı"] as const;
const inp = "rounded bg-beton-950 border border-beton-800 px-2 py-1.5 text-sm text-beton-100 outline-none focus:border-emniyet-500";

function scoreStyle(score: number, status: string): string {
  if (status === "Kapandı") return "bg-beton-800 text-beton-400 border-beton-700";
  if (score >= 15) return "bg-red-500/15 text-red-300 border-red-500/40";
  if (score >= 8) return "bg-amber-500/15 text-amber-300 border-amber-500/40";
  return "bg-green-500/15 text-green-300 border-green-500/40";
}

export default function RisklerPage() {
  const { current } = useProjects();
  const { can } = useAuth();
  const pid = current?.id;
  const canManage = can("risks.manage");

  const [risks, setRisks] = useState<Risk[]>([]);
  const [err, setErr] = useState<string | null>(null);
  const [showForm, setShowForm] = useState(false);
  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");
  const [probability, setProbability] = useState(3);
  const [impact, setImpact] = useState(3);
  const [saving, setSaving] = useState(false);

  const load = useCallback(async () => {
    if (!pid) return;
    setErr(null);
    try {
      const r = await api<{ risks: Risk[] }>(`/projects/${pid}/risks`, { projectId: pid });
      setRisks(r.risks ?? []);
    } catch (e) {
      setErr(describeLoadError(e, "Risk kayıtları"));
    }
  }, [pid]);

  useEffect(() => { load(); }, [load]);

  async function create() {
    if (!pid || !title.trim()) return;
    setSaving(true);
    try {
      await api(`/projects/${pid}/risks`, {
        method: "POST", projectId: pid,
        body: { title: title.trim(), description: description.trim() || undefined, probability, impact },
      });
      setTitle(""); setDescription(""); setProbability(3); setImpact(3); setShowForm(false);
      await load();
    } catch (e) {
      setErr(describeLoadError(e, "Risk kaydı"));
    } finally {
      setSaving(false);
    }
  }

  async function setStatus(r: Risk, status: string) {
    if (!pid) return;
    try {
      await api(`/projects/${pid}/risks/${r.id}`, { method: "PATCH", projectId: pid, body: { status } });
      await load();
    } catch (e) {
      setErr(describeLoadError(e, "Risk durumu"));
    }
  }

  if (!current) return <p className="text-beton-400">Önce üst bardan bir proje seçin.</p>;

  return (
    <div className="space-y-4">
      <div className="flex items-center gap-3 flex-wrap">
        <div>
          <h1 className="text-lg font-display font-bold text-white">Risk Kaydı</h1>
          <p className="text-xs text-beton-500 mt-0.5">
            Skor = olasılık × etki (1–25). Kayıtlar silinmez; durum değişir.
          </p>
        </div>
        {canManage && (
          <button onClick={() => setShowForm((v) => !v)}
            className="ml-auto rounded-md bg-emniyet-500 hover:bg-emniyet-600 text-beton-950 text-xs font-semibold px-3 py-1.5">
            {showForm ? "Vazgeç" : "+ Risk ekle"}
          </button>
        )}
      </div>

      {err && <p className="text-sm text-red-400">{err}</p>}

      {showForm && (
        <div className="rounded-lg border border-beton-800 bg-beton-900 p-4 grid gap-3 md:grid-cols-4">
          <input value={title} onChange={(e) => setTitle(e.target.value)} placeholder="Risk başlığı *"
            className={`${inp} md:col-span-2`} />
          <label className="text-xs text-beton-400">Olasılık (1–5)
            <select value={probability} onChange={(e) => setProbability(Number(e.target.value))} className={`${inp} w-full mt-1`}>
              {[1, 2, 3, 4, 5].map((n) => <option key={n} value={n}>{n}</option>)}
            </select>
          </label>
          <label className="text-xs text-beton-400">Etki (1–5)
            <select value={impact} onChange={(e) => setImpact(Number(e.target.value))} className={`${inp} w-full mt-1`}>
              {[1, 2, 3, 4, 5].map((n) => <option key={n} value={n}>{n}</option>)}
            </select>
          </label>
          <textarea value={description} onChange={(e) => setDescription(e.target.value)} rows={2}
            placeholder="Açıklama (isteğe bağlı)" className={`${inp} md:col-span-4`} />
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
              <th className="py-2 px-3">Risk</th>
              <th className="py-2 px-3 text-center">O × E</th>
              <th className="py-2 px-3 text-center">Skor</th>
              <th className="py-2 px-3">Durum</th>
            </tr>
          </thead>
          <tbody>
            {risks.length === 0 && (
              <tr><td colSpan={5} className="py-6 text-center text-beton-500">Henüz risk kaydı yok.</td></tr>
            )}
            {risks.map((r) => (
              <tr key={r.id} className="border-b border-beton-800/60 align-top">
                <td className="py-2 px-3 font-mono text-xs text-beton-400">R-{String(r.no).padStart(3, "0")}</td>
                <td className="py-2 px-3">
                  <div className="text-beton-100">{r.title}</div>
                  {r.description && <div className="text-xs text-beton-500 mt-0.5">{r.description}</div>}
                </td>
                <td className="py-2 px-3 text-center font-mono text-beton-300">{r.probability} × {r.impact}</td>
                <td className="py-2 px-3 text-center">
                  <span className={`inline-block rounded border px-2 py-0.5 text-xs font-bold ${scoreStyle(r.score, r.status)}`}>
                    {r.score}
                  </span>
                </td>
                <td className="py-2 px-3">
                  {canManage ? (
                    <select value={r.status} onChange={(e) => setStatus(r, e.target.value)} className={inp}>
                      {STATUSES.map((s) => <option key={s} value={s}>{s}</option>)}
                    </select>
                  ) : (
                    <span className="text-beton-300">{r.status}</span>
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
