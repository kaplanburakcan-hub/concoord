import { useCallback, useEffect, useState } from "react";
import { api, describeLoadError } from "../../api/client";
import { useAuth } from "../../auth/AuthContext";
import { apiWithOfflineFallback } from "../../offline/queue";
import { useProjects } from "../../projects/ProjectContext";

// Karar günlüğü (E3). Kararlar değiştirilmez; bir karar yalnız yeni bir kayıtla
// geçersiz kılınabilir. Aktif kararlar varsayılan görünümdür.

type Decision = {
  id: string; text: string; decided_on: string; decided_by_name?: string;
  supersedes_id?: string; superseded_by?: string; active: boolean;
  created_by_name: string; created_at: string;
};

const inp = "rounded bg-beton-950 border border-beton-800 px-2 py-1.5 text-sm text-beton-100 outline-none focus:border-emniyet-500";

export default function KararlarPage() {
  const { current } = useProjects();
  const { can } = useAuth();
  const pid = current?.id;
  const canRecord = can("decisions.record");

  const [decisions, setDecisions] = useState<Decision[]>([]);
  const [err, setErr] = useState<string | null>(null);
  const [showAll, setShowAll] = useState(false);
  const [showForm, setShowForm] = useState(false);
  const [text, setText] = useState("");
  const [decidedOn, setDecidedOn] = useState(() => new Date().toISOString().slice(0, 10));
  const [decidedBy, setDecidedBy] = useState("");
  const [supersedes, setSupersedes] = useState("");
  const [saving, setSaving] = useState(false);

  const load = useCallback(async () => {
    if (!pid) return;
    setErr(null);
    try {
      const r = await api<{ decisions: Decision[] }>(`/projects/${pid}/decisions`, { projectId: pid });
      setDecisions(r.decisions ?? []);
    } catch (e) {
      setErr(describeLoadError(e, "Kararlar"));
    }
  }, [pid]);

  useEffect(() => { load(); }, [load]);

  async function create() {
    if (!pid || !text.trim()) return;
    setSaving(true);
    try {
      const res = await apiWithOfflineFallback({
        method: "POST", path: `/projects/${pid}/decisions`, projectId: pid,
        body: {
          text: text.trim(), decided_on: decidedOn,
          decided_by: decidedBy || undefined,
          supersedes_id: supersedes || undefined,
        },
        label: `Karar: ${text.trim().slice(0, 40)}`,
      });
      setText(""); setDecidedBy(""); setSupersedes(""); setShowForm(false);
      if (res.queued) { alert("Çevrimdışısınız. Karar kuyruğa alındı; bağlantı gelince gönderilir."); return; }
      await load();
    } catch (e) {
      setErr(describeLoadError(e, "Karar kaydı"));
    } finally {
      setSaving(false);
    }
  }

  if (!current) return <p className="text-beton-400">Önce üst bardan bir proje seçin.</p>;

  const visible = showAll ? decisions : decisions.filter((d) => d.active);
  const activeOnes = decisions.filter((d) => d.active);

  return (
    <div className="space-y-4">
      <div className="flex items-center gap-3 flex-wrap">
        <div>
          <h1 className="text-lg font-display font-bold text-white">Karar Günlüğü</h1>
          <p className="text-xs text-beton-500 mt-0.5">
            Kararlar değiştirilmez. Bir kararı değiştirmek için yeni kayıt, eskisini geçersiz kılar.
          </p>
        </div>
        {canRecord && (
          <button onClick={() => setShowForm((v) => !v)}
            className="ml-auto rounded-md bg-emniyet-500 hover:bg-emniyet-600 text-beton-950 text-xs font-semibold px-3 py-1.5">
            {showForm ? "Vazgeç" : "+ Karar ekle"}
          </button>
        )}
      </div>

      {err && <p className="text-sm text-red-400">{err}</p>}

      {showForm && (
        <div className="rounded-lg border border-beton-800 bg-beton-900 p-4 grid gap-3 md:grid-cols-4">
          <textarea value={text} onChange={(e) => setText(e.target.value)} rows={3}
            placeholder="Karar metni *" className={`${inp} md:col-span-4`} />
          <label className="text-xs text-beton-400">Karar tarihi
            <input type="date" value={decidedOn} onChange={(e) => setDecidedOn(e.target.value)} className={`${inp} w-full mt-1`} />
          </label>
          <label className="text-xs text-beton-400 md:col-span-2">Karar veren (isteğe bağlı)
            <input value={decidedBy} onChange={(e) => setDecidedBy(e.target.value)} placeholder="Ad Soyad" className={`${inp} w-full mt-1`} />
          </label>
          <label className="text-xs text-beton-400">Geçersiz kılınan karar (isteğe bağlı)
            <select value={supersedes} onChange={(e) => setSupersedes(e.target.value)} className={`${inp} w-full mt-1`}>
              <option value="">— yok —</option>
              {activeOnes.map((d) => <option key={d.id} value={d.id}>{d.decided_on} · {d.text.slice(0, 60)}</option>)}
            </select>
          </label>
          <div className="md:col-span-4 flex justify-end">
            <button onClick={create} disabled={saving || !text.trim()}
              className="rounded-md bg-emniyet-500 px-4 py-1.5 text-sm font-semibold text-beton-950 disabled:opacity-50">
              Kaydet
            </button>
          </div>
        </div>
      )}

      <label className="flex items-center gap-2 text-xs text-beton-400">
        <input type="checkbox" checked={showAll} onChange={(e) => setShowAll(e.target.checked)} />
        Geçersiz kılınmış kararları da göster
      </label>

      <ul className="space-y-2">
        {visible.length === 0 && <li className="text-sm text-beton-500">Karar kaydı yok.</li>}
        {visible.map((d) => (
          <li key={d.id}
            className={`rounded-lg border p-3 ${d.active ? "border-beton-800 bg-beton-900/60" : "border-beton-800/60 bg-beton-950/60 opacity-70"}`}>
            <div className="flex items-center gap-2 text-xs text-beton-500">
              <span className="font-mono">{d.decided_on}</span>
              {d.decided_by_name && <span>· {d.decided_by_name}</span>}
              {!d.active && <span className="rounded border border-amber-500/40 px-1.5 py-0.5 text-amber-300">Geçersiz kılındı</span>}
            </div>
            <p className="mt-1 text-sm text-beton-100 whitespace-pre-line">{d.text}</p>
            <div className="mt-1 text-[11px] text-beton-600">Kaydeden: {d.created_by_name} · {d.created_at}</div>
          </li>
        ))}
      </ul>
    </div>
  );
}
