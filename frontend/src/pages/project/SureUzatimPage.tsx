import { useCallback, useEffect, useState } from "react";
import { api, describeLoadError } from "../../api/client";
import { useAuth } from "../../auth/AuthContext";
import { useProjects } from "../../projects/ProjectContext";

// Süre uzatımı (EOT) talep taslakları (E2, ilk adım). Yalnız kayıt: onay, sözleşme
// bitiş tarihi ve iş programı etkisi bu aşamada yoktur.

type Draft = {
  id: string; no: number; title: string; reason_code: string; requested_days: number;
  basis?: string; status: string; created_by_name: string; created_at: string;
};

const REASONS: [string, string][] = [
  ["Sahip", "Sahip / işveren kaynaklı"], ["Hava", "Hava koşulları"], ["Tasarim", "Tasarım değişikliği"],
  ["MucbirSebep", "Mücbir sebep"], ["Diger", "Diğer"],
];
const REASON_LABEL = Object.fromEntries(REASONS);
const inp = "rounded bg-beton-950 border border-beton-800 px-2 py-1.5 text-sm text-beton-100 outline-none focus:border-emniyet-500";

export default function SureUzatimPage() {
  const { current } = useProjects();
  const { can } = useAuth();
  const pid = current?.id;
  const canCreate = can("claims.create");

  const [drafts, setDrafts] = useState<Draft[]>([]);
  const [err, setErr] = useState<string | null>(null);
  const [showForm, setShowForm] = useState(false);
  const [title, setTitle] = useState("");
  const [reason, setReason] = useState("Sahip");
  const [days, setDays] = useState("");
  const [basis, setBasis] = useState("");
  const [saving, setSaving] = useState(false);

  const load = useCallback(async () => {
    if (!pid) return;
    setErr(null);
    try {
      const r = await api<{ drafts: Draft[] }>(`/projects/${pid}/time-extensions`, { projectId: pid });
      setDrafts(r.drafts ?? []);
    } catch (e) {
      setErr(describeLoadError(e, "Süre uzatımı taslakları"));
    }
  }, [pid]);

  useEffect(() => { load(); }, [load]);

  async function create() {
    if (!pid || !title.trim() || !days) return;
    setSaving(true);
    try {
      await api(`/projects/${pid}/time-extensions`, {
        method: "POST", projectId: pid,
        body: { title: title.trim(), reason_code: reason, requested_days: Number(days), basis: basis.trim() || undefined },
      });
      setTitle(""); setReason("Sahip"); setDays(""); setBasis(""); setShowForm(false);
      await load();
    } catch (e) {
      setErr(describeLoadError(e, "Taslak"));
    } finally {
      setSaving(false);
    }
  }

  if (!current) return <p className="text-beton-400">Önce üst bardan bir proje seçin.</p>;

  return (
    <div className="space-y-4">
      <div className="flex items-center gap-3 flex-wrap">
        <div>
          <h1 className="text-lg font-display font-bold text-white">Süre Uzatımı Talepleri</h1>
          <p className="text-xs text-beton-500 mt-0.5">
            Taslak kayıt. Onay, sözleşme tarihi ve iş programı etkisi sonraki aşamada eklenecek.
          </p>
        </div>
        {canCreate && (
          <button onClick={() => setShowForm((v) => !v)}
            className="ml-auto rounded-md bg-emniyet-500 hover:bg-emniyet-600 text-beton-950 text-xs font-semibold px-3 py-1.5">
            {showForm ? "Vazgeç" : "+ Taslak ekle"}
          </button>
        )}
      </div>

      {err && <p className="text-sm text-red-400">{err}</p>}

      {showForm && (
        <div className="rounded-lg border border-beton-800 bg-beton-900 p-4 grid gap-3 md:grid-cols-4">
          <input value={title} onChange={(e) => setTitle(e.target.value)} placeholder="Başlık *" className={`${inp} md:col-span-2`} />
          <select value={reason} onChange={(e) => setReason(e.target.value)} className={inp}>
            {REASONS.map(([v, t]) => <option key={v} value={v}>{t}</option>)}
          </select>
          <input type="number" min={1} max={3650} value={days} onChange={(e) => setDays(e.target.value)}
            placeholder="Talep edilen gün *" className={inp} />
          <textarea value={basis} onChange={(e) => setBasis(e.target.value)} rows={2}
            placeholder="Dayanak açıklaması" className={`${inp} md:col-span-4`} />
          <div className="md:col-span-4 flex justify-end">
            <button onClick={create} disabled={saving || !title.trim() || !days}
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
              <th className="py-2 px-3">Talep</th>
              <th className="py-2 px-3">Neden</th>
              <th className="py-2 px-3 text-right">Gün</th>
              <th className="py-2 px-3">Durum</th>
            </tr>
          </thead>
          <tbody>
            {drafts.length === 0 && (
              <tr><td colSpan={5} className="py-6 text-center text-beton-500">Henüz taslak yok.</td></tr>
            )}
            {drafts.map((d) => (
              <tr key={d.id} className="border-b border-beton-800/60 align-top">
                <td className="py-2 px-3 font-mono text-xs text-beton-400">S-{String(d.no).padStart(3, "0")}</td>
                <td className="py-2 px-3">
                  <div className="text-beton-100">{d.title}</div>
                  {d.basis && <div className="text-xs text-beton-500 mt-0.5">{d.basis}</div>}
                  <div className="text-[11px] text-beton-600 mt-0.5">{d.created_by_name} · {d.created_at}</div>
                </td>
                <td className="py-2 px-3 text-beton-300">{REASON_LABEL[d.reason_code] ?? d.reason_code}</td>
                <td className="py-2 px-3 text-right font-mono text-beton-300">{d.requested_days}</td>
                <td className="py-2 px-3 text-xs text-beton-400">{d.status}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}
