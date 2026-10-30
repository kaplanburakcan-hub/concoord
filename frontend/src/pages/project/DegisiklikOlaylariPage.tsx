import { useCallback, useEffect, useState } from "react";
import { api, describeLoadError } from "../../api/client";
import { useAuth } from "../../auth/AuthContext";
import { useProjects } from "../../projects/ProjectContext";

// Değişiklik olayı taslakları (E1, ilk adım). Yalnız kayıt: onay akışı, sözleşme
// bedeli ve bütçe etkisi bu aşamada yoktur.

type ChangeEvent = {
  id: string; no: number; title: string; source: string; description?: string;
  estimated_amount?: number; estimated_days?: number; status: string;
  created_by_name: string; created_at: string;
};

const SOURCES: [string, string][] = [
  ["Sahip", "Sahip talebi"], ["Tasarim", "Tasarım değişikliği"], ["SahaKosulu", "Saha koşulu"],
  ["RFI", "Bilgi talebi (RFI)"], ["HakTalebi", "Hak talebi"], ["Diger", "Diğer"],
];
const SOURCE_LABEL = Object.fromEntries(SOURCES);
const inp = "rounded bg-beton-950 border border-beton-800 px-2 py-1.5 text-sm text-beton-100 outline-none focus:border-emniyet-500";

export default function DegisiklikOlaylariPage() {
  const { current } = useProjects();
  const { can } = useAuth();
  const pid = current?.id;
  const canCreate = can("changes.create");

  const [events, setEvents] = useState<ChangeEvent[]>([]);
  const [err, setErr] = useState<string | null>(null);
  const [showForm, setShowForm] = useState(false);
  const [title, setTitle] = useState("");
  const [source, setSource] = useState("Sahip");
  const [description, setDescription] = useState("");
  const [amount, setAmount] = useState("");
  const [days, setDays] = useState("");
  const [saving, setSaving] = useState(false);

  const load = useCallback(async () => {
    if (!pid) return;
    setErr(null);
    try {
      const r = await api<{ events: ChangeEvent[] }>(`/projects/${pid}/change-events`, { projectId: pid });
      setEvents(r.events ?? []);
    } catch (e) {
      setErr(describeLoadError(e, "Değişiklik olayları"));
    }
  }, [pid]);

  useEffect(() => { load(); }, [load]);

  async function create() {
    if (!pid || !title.trim()) return;
    setSaving(true);
    try {
      await api(`/projects/${pid}/change-events`, {
        method: "POST", projectId: pid,
        body: {
          title: title.trim(), source,
          description: description.trim() || undefined,
          estimated_amount: amount === "" ? undefined : Number(amount),
          estimated_days: days === "" ? undefined : Number(days),
        },
      });
      setTitle(""); setSource("Sahip"); setDescription(""); setAmount(""); setDays(""); setShowForm(false);
      await load();
    } catch (e) {
      setErr(describeLoadError(e, "Değişiklik olayı"));
    } finally {
      setSaving(false);
    }
  }

  if (!current) return <p className="text-beton-400">Önce üst bardan bir proje seçin.</p>;

  return (
    <div className="space-y-4">
      <div className="flex items-center gap-3 flex-wrap">
        <div>
          <h1 className="text-lg font-display font-bold text-white">Değişiklik Olayları</h1>
          <p className="text-xs text-beton-500 mt-0.5">
            Taslak kayıt. Onay, sözleşme bedeli ve bütçe etkisi sonraki aşamada eklenecek.
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
          <input value={title} onChange={(e) => setTitle(e.target.value)} placeholder="Başlık *"
            className={`${inp} md:col-span-2`} />
          <select value={source} onChange={(e) => setSource(e.target.value)} className={inp}>
            {SOURCES.map(([v, t]) => <option key={v} value={v}>{t}</option>)}
          </select>
          <div className="grid grid-cols-2 gap-2">
            <input type="number" min={0} step="0.01" value={amount} onChange={(e) => setAmount(e.target.value)}
              placeholder="Tahmini tutar" className={inp} />
            <input type="number" min={0} value={days} onChange={(e) => setDays(e.target.value)}
              placeholder="Süre etkisi (gün)" className={inp} />
          </div>
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
              <th className="py-2 px-3">Başlık</th>
              <th className="py-2 px-3">Kaynak</th>
              <th className="py-2 px-3 text-right">Tahmini tutar</th>
              <th className="py-2 px-3 text-right">Süre (gün)</th>
              <th className="py-2 px-3">Durum</th>
            </tr>
          </thead>
          <tbody>
            {events.length === 0 && (
              <tr><td colSpan={6} className="py-6 text-center text-beton-500">Henüz değişiklik olayı yok.</td></tr>
            )}
            {events.map((e) => (
              <tr key={e.id} className="border-b border-beton-800/60 align-top">
                <td className="py-2 px-3 font-mono text-xs text-beton-400">D-{String(e.no).padStart(3, "0")}</td>
                <td className="py-2 px-3">
                  <div className="text-beton-100">{e.title}</div>
                  {e.description && <div className="text-xs text-beton-500 mt-0.5">{e.description}</div>}
                  <div className="text-[11px] text-beton-600 mt-0.5">{e.created_by_name} · {e.created_at}</div>
                </td>
                <td className="py-2 px-3 text-beton-300">{SOURCE_LABEL[e.source] ?? e.source}</td>
                <td className="py-2 px-3 text-right font-mono text-beton-300">
                  {e.estimated_amount != null ? e.estimated_amount.toLocaleString("tr-TR", { minimumFractionDigits: 2 }) : "—"}
                </td>
                <td className="py-2 px-3 text-right font-mono text-beton-300">{e.estimated_days ?? "—"}</td>
                <td className="py-2 px-3 text-xs text-beton-400">{e.status}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}
