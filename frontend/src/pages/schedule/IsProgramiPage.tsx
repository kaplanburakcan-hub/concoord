import { useEffect, useMemo, useRef, useState } from "react";
import Gantt from "frappe-gantt";
import "frappe-gantt/dist/frappe-gantt.css";
import {
  ResponsiveContainer, LineChart, Line, XAxis, YAxis, CartesianGrid, Tooltip, Legend,
} from "recharts";
import { api } from "../../api/client";
import { useProjects } from "../ProjectContext";
import { useAuth } from "../../auth/AuthContext";

// IsProgramiPage — rota /proje/is-programi. Blok 2 Aşama 1 (WBS + Gantt).
// Fiziksel ilerleme elle girilmez (progress_source='derived' kalemlerde):
// onaylı hakedişten türetilir, backend'de istek anında hesaplanır — burada
// yalnızca sunucudan gelen `progress` alanı gösterilir.

type ScheduleItem = {
  id: string; project_id: string; parent_id: string | null;
  wbs_code: string; name: string; sort_order: number; is_milestone: boolean;
  baseline_start: string | null; baseline_finish: string | null;
  actual_start: string | null; actual_finish: string | null;
  progress_source: "manual" | "derived"; manual_progress: number | null;
  weight: number; progress: number; row_version: number;
};
type Dependency = { predecessor_id: string; successor_id: string; dep_type: string; lag_days: number };
type Baseline = { id: string; project_id: string; revision_no: number; frozen_at: string; frozen_by: string; note: string | null };
type BaselineDetail = Baseline & { snapshot: { id: string; progress: number; baseline_start: string | null; baseline_finish: string | null }[] };
type AvailablePoz = { id: string; poz_no: string; description: string; unit: string; contract_qty: number; subcontractor_adi: string };
type SCurvePoint = { date: string; planned_physical: number; actual_physical: number; planned_cash: number; actual_cash: number };

function isDelayed(it: ScheduleItem): boolean {
  return !it.actual_finish && !!it.baseline_finish && new Date(it.baseline_finish) < new Date() && it.progress < 100;
}
function isDone(it: ScheduleItem): boolean {
  return it.progress >= 100;
}
function fmtPct(v: number) { return `%${v.toFixed(0)}`; }

export default function IsProgramiPage() {
  const { current } = useProjects();
  const { can } = useAuth();
  const canEdit = can("schedule.edit");
  const canFreeze = can("schedule.freeze_baseline");

  const [tab, setTab] = useState<"gantt" | "tablo" | "s-egrisi">("tablo");
  const [items, setItems] = useState<ScheduleItem[]>([]);
  const [deps, setDeps] = useState<Dependency[]>([]);
  const [baselines, setBaselines] = useState<Baseline[]>([]);
  const [selectedBaselineId, setSelectedBaselineId] = useState<string>("");
  const [baselineDetail, setBaselineDetail] = useState<BaselineDetail | null>(null);
  const [loading, setLoading] = useState(true);

  const [editingItem, setEditingItem] = useState<ScheduleItem | "new" | null>(null);
  const [newItemParent, setNewItemParent] = useState<string | null>(null);
  const [pozModalItem, setPozModalItem] = useState<ScheduleItem | null>(null);
  const [depPanelOpen, setDepPanelOpen] = useState(false);
  const [freezing, setFreezing] = useState(false);

  useEffect(() => {
    load();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [current?.id]);

  useEffect(() => {
    if (!selectedBaselineId) { setBaselineDetail(null); return; }
    api<{ baseline: BaselineDetail }>(`/schedule/baselines/${selectedBaselineId}`)
      .then((r) => setBaselineDetail(r.baseline))
      .catch(() => setBaselineDetail(null));
  }, [selectedBaselineId]);

  async function load() {
    if (!current?.id) return;
    setLoading(true);
    try {
      const [s, d, b] = await Promise.all([
        api<{ items: ScheduleItem[] }>(`/projects/${current.id}/schedule`, { projectId: current.id }),
        api<{ dependencies: Dependency[] }>(`/projects/${current.id}/schedule/dependencies`, { projectId: current.id }),
        api<{ baselines: Baseline[] }>(`/projects/${current.id}/schedule/baselines`, { projectId: current.id }),
      ]);
      setItems(s.items);
      setDeps(d.dependencies);
      setBaselines(b.baselines);
    } finally {
      setLoading(false);
    }
  }

  async function freezeBaseline() {
    if (!current?.id) return;
    const note = window.prompt("Bu revizyona kısa bir not eklemek ister misiniz? (isteğe bağlı)") ?? undefined;
    setFreezing(true);
    try {
      await api(`/projects/${current.id}/schedule/baseline`, { method: "POST", projectId: current.id, body: { note: note || null } });
      await load();
    } finally {
      setFreezing(false);
    }
  }

  const byParent = useMemo(() => {
    const m = new Map<string, ScheduleItem[]>();
    for (const it of items) {
      const key = it.parent_id ?? "";
      if (!m.has(key)) m.set(key, []);
      m.get(key)!.push(it);
    }
    for (const arr of m.values()) arr.sort((a, b) => a.wbs_code.localeCompare(b.wbs_code, "tr", { numeric: true }));
    return m;
  }, [items]);

  if (!current) return <div className="p-8 text-beton-400">Proje seçilmedi.</div>;

  return (
    <div>
      <div className="flex items-center gap-3 flex-wrap">
        <h1 className="font-display text-2xl font-extrabold text-white">İş Programı</h1>
        <div className="ml-auto flex items-center gap-2 flex-wrap">
          <select
            value={selectedBaselineId}
            onChange={(e) => setSelectedBaselineId(e.target.value)}
            className="rounded-md bg-beton-900 border border-beton-700 px-2 py-1.5 text-xs text-beton-200"
          >
            <option value="">Baseline karşılaştırma yok</option>
            {baselines.map((b) => (
              <option key={b.id} value={b.id}>
                Rev {b.revision_no} — {new Date(b.frozen_at).toLocaleDateString("tr-TR")}{b.note ? ` (${b.note})` : ""}
              </option>
            ))}
          </select>
          {canFreeze && (
            <button
              onClick={freezeBaseline} disabled={freezing}
              className="rounded-md border border-beton-700 text-beton-200 hover:bg-beton-800 disabled:opacity-60 px-3 py-1.5 text-xs font-semibold transition"
            >
              {freezing ? "Dondruluyor…" : "Revizyonu Dondur"}
            </button>
          )}
          {canEdit && (
            <button
              onClick={() => { setNewItemParent(null); setEditingItem("new"); }}
              className="rounded-md bg-emniyet-500 hover:bg-emniyet-600 text-beton-950 font-semibold px-3 py-1.5 text-xs transition"
            >
              + Kök Kalem
            </button>
          )}
        </div>
      </div>

      <div className="mt-4 flex gap-1 border-b border-beton-800">
        {(["tablo", "gantt", "s-egrisi"] as const).map((t) => (
          <button
            key={t}
            onClick={() => setTab(t)}
            className={`px-4 py-2 text-sm font-medium border-b-2 -mb-px transition ${
              tab === t ? "border-emniyet-500 text-white" : "border-transparent text-beton-400 hover:text-beton-200"
            }`}
          >
            {t === "tablo" ? "Tablo (WBS)" : t === "gantt" ? "Gantt" : "S-Eğrisi"}
          </button>
        ))}
      </div>

      {loading ? (
        <p className="px-4 py-8 text-center text-beton-400 text-sm">Yükleniyor…</p>
      ) : items.length === 0 ? (
        <p className="px-4 py-8 text-center text-beton-400 text-sm">Henüz bir WBS kalemi yok.</p>
      ) : tab === "tablo" ? (
        <TableView
          items={items} byParent={byParent} canEdit={canEdit}
          baselineDetail={baselineDetail}
          onEdit={(it) => setEditingItem(it)}
          onAddChild={(parentId) => { setNewItemParent(parentId); setEditingItem("new"); }}
          onLinkPoz={(it) => setPozModalItem(it)}
          onDelete={async (it) => {
            if (!window.confirm(`"${it.name}" kalemini silmek istiyor musunuz?`)) return;
            try {
              await api(`/schedule/items/${it.id}`, { method: "DELETE" });
              load();
            } catch (e: any) {
              window.alert(e?.api?.message || "Silinemedi.");
            }
          }}
        />
      ) : tab === "gantt" ? (
        <GanttView items={items} deps={deps} onItemClick={(it) => setEditingItem(it)} />
      ) : (
        <SCurveView projectId={current.id} />
      )}

      {canEdit && (
        <div className="mt-6">
          <button
            onClick={() => setDepPanelOpen((v) => !v)}
            className="text-xs text-beton-400 hover:text-beton-200 underline"
          >
            {depPanelOpen ? "Bağımlılıkları gizle" : "Bağımlılıkları yönet"}
          </button>
          {depPanelOpen && (
            <DependencyPanel projectId={current.id} items={items} deps={deps} onChanged={load} />
          )}
        </div>
      )}

      {editingItem && (
        <ItemModal
          item={editingItem === "new" ? null : editingItem}
          parentId={editingItem === "new" ? newItemParent : editingItem.parent_id}
          projectId={current.id}
          items={items}
          onClose={() => setEditingItem(null)}
          onSaved={() => { setEditingItem(null); load(); }}
        />
      )}
      {pozModalItem && (
        <PozLinkModal
          item={pozModalItem} projectId={current.id}
          onClose={() => setPozModalItem(null)}
          onChanged={() => { setPozModalItem(null); load(); }}
        />
      )}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Tablo görünümü
// ---------------------------------------------------------------------------

function TableView({
  items, byParent, canEdit, baselineDetail, onEdit, onAddChild, onLinkPoz, onDelete,
}: {
  items: ScheduleItem[]; byParent: Map<string, ScheduleItem[]>; canEdit: boolean;
  baselineDetail: BaselineDetail | null;
  onEdit: (it: ScheduleItem) => void; onAddChild: (parentId: string) => void;
  onLinkPoz: (it: ScheduleItem) => void; onDelete: (it: ScheduleItem) => void;
}) {
  const baselineById = useMemo(() => {
    const m = new Map<string, BaselineDetail["snapshot"][number]>();
    for (const s of baselineDetail?.snapshot ?? []) m.set(s.id, s);
    return m;
  }, [baselineDetail]);

  const rows: { item: ScheduleItem; depth: number }[] = [];
  function walk(parentKey: string, depth: number) {
    for (const it of byParent.get(parentKey) ?? []) {
      rows.push({ item: it, depth });
      walk(it.id, depth + 1);
    }
  }
  walk("", 0);

  return (
    <div className="mt-4 overflow-x-auto rounded-lg border border-beton-800">
      <table className="text-xs border-collapse w-full">
        <thead>
          <tr className="bg-beton-900">
            <th className="px-3 py-2 text-left text-beton-300 border-b border-beton-800">WBS</th>
            <th className="px-3 py-2 text-left text-beton-300 border-b border-beton-800 min-w-[220px]">Ad</th>
            <th className="px-3 py-2 text-left text-beton-300 border-b border-beton-800">Başlangıç</th>
            <th className="px-3 py-2 text-left text-beton-300 border-b border-beton-800">Bitiş</th>
            <th className="px-3 py-2 text-left text-beton-300 border-b border-beton-800 min-w-[140px]">İlerleme</th>
            {baselineDetail && <th className="px-3 py-2 text-left text-beton-300 border-b border-beton-800">Baseline'a Göre</th>}
            <th className="px-3 py-2 text-left text-beton-300 border-b border-beton-800">Kaynak</th>
            {canEdit && <th className="px-3 py-2 text-left text-beton-300 border-b border-beton-800">Aksiyonlar</th>}
          </tr>
        </thead>
        <tbody>
          {rows.map(({ item: it, depth }) => {
            const delayed = isDelayed(it);
            const done = isDone(it);
            const base = baselineById.get(it.id);
            return (
              <tr key={it.id} className="border-b border-beton-800/60 hover:bg-beton-900/40">
                <td className="px-3 py-2 font-mono text-beton-400 whitespace-nowrap">{it.wbs_code}</td>
                <td className="px-3 py-2 text-beton-200" style={{ paddingLeft: `${12 + depth * 18}px` }}>
                  {it.is_milestone && <span className="mr-1" title="Kilometre taşı">◆</span>}
                  {it.name}
                </td>
                <td className="px-3 py-2 font-mono text-beton-400 whitespace-nowrap">{it.baseline_start ?? "—"}</td>
                <td className={`px-3 py-2 font-mono whitespace-nowrap ${delayed ? "text-red-400" : "text-beton-400"}`}>
                  {it.baseline_finish ?? "—"}
                </td>
                <td className="px-3 py-2">
                  <div className="flex items-center gap-2">
                    <div className="w-20 h-1.5 rounded-full bg-beton-800 overflow-hidden">
                      <div
                        className={`h-full ${done ? "bg-green-500" : delayed ? "bg-red-500" : "bg-emniyet-500"}`}
                        style={{ width: `${Math.min(100, Math.max(0, it.progress))}%` }}
                      />
                    </div>
                    <span className={`font-mono tabular-nums ${done ? "text-green-400" : delayed ? "text-red-400" : "text-beton-300"}`}>
                      {fmtPct(it.progress)}
                    </span>
                  </div>
                </td>
                {baselineDetail && (
                  <td className="px-3 py-2 font-mono tabular-nums text-beton-400">
                    {base ? (it.progress - base.progress >= 0 ? "+" : "") + (it.progress - base.progress).toFixed(0) + " puan" : "—"}
                  </td>
                )}
                <td className="px-3 py-2">
                  <span className={`rounded px-1.5 py-0.5 text-[10px] font-semibold ${
                    it.progress_source === "derived" ? "bg-emniyet-500/15 text-emniyet-400" : "bg-beton-800 text-beton-300"
                  }`}>
                    {it.progress_source === "derived" ? "Hakedişten" : "Elle"}
                  </span>
                </td>
                {canEdit && (
                  <td className="px-3 py-2 whitespace-nowrap">
                    <div className="flex gap-2 text-[11px]">
                      <button onClick={() => onAddChild(it.id)} className="text-beton-400 hover:text-white">+ Alt</button>
                      <button onClick={() => onEdit(it)} className="text-beton-400 hover:text-white">Düzenle</button>
                      <button onClick={() => onLinkPoz(it)} className="text-beton-400 hover:text-white">Poz</button>
                      <button onClick={() => onDelete(it)} className="text-red-400 hover:text-red-300">Sil</button>
                    </div>
                  </td>
                )}
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Gantt görünümü (frappe-gantt) — kendi Gantt bileşenimizi yazmıyoruz.
// ---------------------------------------------------------------------------

function GanttView({ items, deps, onItemClick }: { items: ScheduleItem[]; deps: Dependency[]; onItemClick: (it: ScheduleItem) => void }) {
  const ref = useRef<HTMLDivElement>(null);
  const withDates = items.filter((it) => (it.baseline_start ?? it.actual_start) && (it.baseline_finish ?? it.actual_finish));
  const withoutDates = items.length - withDates.length;

  useEffect(() => {
    if (!ref.current || withDates.length === 0) return;
    const depsBySuccessor = new Map<string, string[]>();
    for (const d of deps) {
      if (!depsBySuccessor.has(d.successor_id)) depsBySuccessor.set(d.successor_id, []);
      depsBySuccessor.get(d.successor_id)!.push(d.predecessor_id);
    }
    const byId = new Map(items.map((it) => [it.id, it]));
    const tasks = withDates.map((it) => {
      let start = (it.baseline_start ?? it.actual_start)!;
      let end = (it.baseline_finish ?? it.actual_finish)!;
      if (start === end) {
        const d = new Date(end);
        d.setDate(d.getDate() + 1);
        end = d.toISOString().slice(0, 10);
      }
      return {
        id: it.id,
        name: `${it.wbs_code} ${it.name}`,
        start, end,
        progress: Math.round(it.progress),
        dependencies: (depsBySuccessor.get(it.id) ?? []).filter((p) => byId.has(p)).join(","),
        custom_class: isDelayed(it) ? "gantt-bar-delayed" : isDone(it) ? "gantt-bar-done" : undefined,
      };
    });
    ref.current.innerHTML = "";
    try {
      new Gantt(ref.current, tasks, {
        view_mode: "Week",
        on_click: (task) => { const it = byId.get(task.id); if (it) onItemClick(it); },
      });
    } catch {
      /* boş/geçersiz tarih kombinasyonu — sessizce atla */
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [items, deps]);

  return (
    <div className="mt-4">
      <style>{`
        .gantt-bar-delayed .bar { fill: #ef4444 !important; }
        .gantt-bar-delayed .bar-progress { fill: #b91c1c !important; }
        .gantt-bar-done .bar { fill: #22c55e !important; }
        .gantt-bar-done .bar-progress { fill: #16a34a !important; }
      `}</style>
      {withoutDates > 0 && (
        <p className="mb-2 text-xs text-beton-400">
          {withoutDates} kalemde başlangıç/bitiş tarihi girilmediği için Gantt'ta gösterilmiyor — Tablo'dan düzenleyin.
        </p>
      )}
      {withDates.length === 0 ? (
        <p className="px-4 py-8 text-center text-beton-400 text-sm">Gantt için hiçbir kalemde tarih girilmemiş.</p>
      ) : (
        <div ref={ref} className="rounded-lg border border-beton-800 bg-beton-950 p-2 overflow-x-auto" />
      )}
    </div>
  );
}

// ---------------------------------------------------------------------------
// S-Eğrisi görünümü (recharts)
// ---------------------------------------------------------------------------

function SCurveView({ projectId }: { projectId: string }) {
  const [bucket, setBucket] = useState<"week" | "month">("month");
  const [points, setPoints] = useState<SCurvePoint[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    setLoading(true);
    api<{ points: SCurvePoint[] }>(`/projects/${projectId}/schedule/s-curve?bucket=${bucket}`, { projectId })
      .then((r) => setPoints(r.points))
      .finally(() => setLoading(false));
  }, [projectId, bucket]);

  return (
    <div className="mt-4">
      <div className="flex items-center gap-2 mb-3">
        <span className="text-xs text-beton-400">Kova:</span>
        {(["month", "week"] as const).map((b) => (
          <button
            key={b} onClick={() => setBucket(b)}
            className={`rounded px-2 py-1 text-xs ${bucket === b ? "bg-emniyet-500 text-beton-950 font-semibold" : "bg-beton-900 text-beton-300"}`}
          >
            {b === "month" ? "Aylık" : "Haftalık"}
          </button>
        ))}
      </div>
      {loading ? (
        <p className="px-4 py-8 text-center text-beton-400 text-sm">Yükleniyor…</p>
      ) : (
        <>
          <div className="rounded-lg border border-beton-800 bg-beton-950 p-4" style={{ height: 320 }}>
            <p className="text-xs text-beton-400 mb-2">Fiziksel İlerleme (%)</p>
            <ResponsiveContainer width="100%" height="90%">
              <LineChart data={points}>
                <CartesianGrid strokeDasharray="3 3" stroke="#333" />
                <XAxis dataKey="date" tick={{ fontSize: 10, fill: "#999" }} />
                <YAxis domain={[0, 100]} tick={{ fontSize: 10, fill: "#999" }} />
                <Tooltip contentStyle={{ background: "#1a1a1a", border: "1px solid #333", fontSize: 12 }} />
                <Legend wrapperStyle={{ fontSize: 11 }} />
                <Line type="monotone" dataKey="planned_physical" name="Planlanan" stroke="#6b7280" strokeDasharray="4 3" dot={false} />
                <Line type="monotone" dataKey="actual_physical" name="Gerçekleşen" stroke="#10b981" strokeWidth={2} dot={false} />
              </LineChart>
            </ResponsiveContainer>
          </div>
          <div className="mt-4 rounded-lg border border-beton-800 bg-beton-950 p-4" style={{ height: 320 }}>
            <p className="text-xs text-beton-400 mb-2">Nakit Akış (Tutar)</p>
            <ResponsiveContainer width="100%" height="90%">
              <LineChart data={points}>
                <CartesianGrid strokeDasharray="3 3" stroke="#333" />
                <XAxis dataKey="date" tick={{ fontSize: 10, fill: "#999" }} />
                <YAxis tick={{ fontSize: 10, fill: "#999" }} tickFormatter={(v) => v.toLocaleString("tr-TR")} />
                <Tooltip contentStyle={{ background: "#1a1a1a", border: "1px solid #333", fontSize: 12 }} formatter={(v) => Number(v).toLocaleString("tr-TR")} />
                <Legend wrapperStyle={{ fontSize: 11 }} />
                <Line type="monotone" dataKey="planned_cash" name="Planlanan" stroke="#6b7280" strokeDasharray="4 3" dot={false} />
                <Line type="monotone" dataKey="actual_cash" name="Gerçekleşen" stroke="#10b981" strokeWidth={2} dot={false} />
              </LineChart>
            </ResponsiveContainer>
          </div>
        </>
      )}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Kalem oluştur/düzenle modalı
// ---------------------------------------------------------------------------

function ItemModal({
  item, parentId, projectId, items, onClose, onSaved,
}: { item: ScheduleItem | null; parentId: string | null; projectId: string; items: ScheduleItem[]; onClose: () => void; onSaved: () => void }) {
  const [wbsCode, setWbsCode] = useState(item?.wbs_code ?? "");
  const [name, setName] = useState(item?.name ?? "");
  const [isMilestone, setIsMilestone] = useState(item?.is_milestone ?? false);
  const [baselineStart, setBaselineStart] = useState(item?.baseline_start ?? "");
  const [baselineFinish, setBaselineFinish] = useState(item?.baseline_finish ?? "");
  const [actualStart, setActualStart] = useState(item?.actual_start ?? "");
  const [actualFinish, setActualFinish] = useState(item?.actual_finish ?? "");
  const [progressSource, setProgressSource] = useState<"manual" | "derived">(item?.progress_source ?? "manual");
  const [manualProgress, setManualProgress] = useState(item?.manual_progress?.toString() ?? "");
  const [weight, setWeight] = useState(item?.weight?.toString() ?? "0");
  const [busy, setBusy] = useState(false);
  const [errs, setErrs] = useState<Record<string, string>>({});

  const parentName = parentId ? items.find((i) => i.id === parentId)?.name : null;

  async function save() {
    setBusy(true);
    setErrs({});
    const body = {
      parent_id: item ? item.parent_id : parentId,
      wbs_code: wbsCode, name, is_milestone: isMilestone,
      baseline_start: baselineStart || null, baseline_finish: baselineFinish || null,
      actual_start: actualStart || null, actual_finish: actualFinish || null,
      progress_source: progressSource,
      manual_progress: progressSource === "manual" && manualProgress !== "" ? Number(manualProgress) : null,
      weight: Number(weight) || 0,
    };
    try {
      if (item) {
        await api(`/schedule/items/${item.id}`, { method: "PATCH", body: { ...body, row_version: item.row_version } });
      } else {
        await api(`/projects/${projectId}/schedule/items`, { method: "POST", projectId, body });
      }
      onSaved();
    } catch (e: any) {
      if (e?.api?.details) setErrs(e.api.details);
      else window.alert(e?.api?.message || "Kaydedilemedi.");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="fixed inset-0 z-50 bg-black/60 flex items-center justify-center p-4" onClick={onClose}>
      <div className="w-full max-w-md rounded-xl border border-beton-700 bg-beton-900 p-5 max-h-[90vh] overflow-y-auto" onClick={(e) => e.stopPropagation()}>
        <h2 className="text-lg font-bold text-beton-100">{item ? "Kalemi Düzenle" : "Yeni WBS Kalemi"}</h2>
        {parentName && <p className="text-xs text-beton-400 mt-1">Üst kalem: {parentName}</p>}

        <label className="block mt-4 text-xs text-beton-400 mb-1">WBS Kodu *</label>
        <input value={wbsCode} onChange={(e) => setWbsCode(e.target.value)} placeholder="1.2.3"
          className="w-full rounded-md bg-beton-950 border border-beton-800 px-3 py-1.5 text-sm text-beton-100 outline-none focus:border-emniyet-500" />
        {errs.wbs_code && <p className="text-xs text-red-400 mt-0.5">{errs.wbs_code}</p>}

        <label className="block mt-3 text-xs text-beton-400 mb-1">Ad *</label>
        <input value={name} onChange={(e) => setName(e.target.value)}
          className="w-full rounded-md bg-beton-950 border border-beton-800 px-3 py-1.5 text-sm text-beton-100 outline-none focus:border-emniyet-500" />
        {errs.name && <p className="text-xs text-red-400 mt-0.5">{errs.name}</p>}

        <label className="mt-3 flex items-center gap-2 text-xs text-beton-300">
          <input type="checkbox" checked={isMilestone} onChange={(e) => setIsMilestone(e.target.checked)} />
          Kilometre taşı
        </label>

        <div className="grid grid-cols-2 gap-2 mt-3">
          <div>
            <label className="block text-xs text-beton-400 mb-1">Baseline Başlangıç</label>
            <input type="date" value={baselineStart} onChange={(e) => setBaselineStart(e.target.value)}
              className="w-full rounded-md bg-beton-950 border border-beton-800 px-2 py-1.5 text-sm text-beton-100" />
          </div>
          <div>
            <label className="block text-xs text-beton-400 mb-1">Baseline Bitiş</label>
            <input type="date" value={baselineFinish} onChange={(e) => setBaselineFinish(e.target.value)}
              className="w-full rounded-md bg-beton-950 border border-beton-800 px-2 py-1.5 text-sm text-beton-100" />
          </div>
          <div>
            <label className="block text-xs text-beton-400 mb-1">Fiili Başlangıç</label>
            <input type="date" value={actualStart} onChange={(e) => setActualStart(e.target.value)}
              className="w-full rounded-md bg-beton-950 border border-beton-800 px-2 py-1.5 text-sm text-beton-100" />
          </div>
          <div>
            <label className="block text-xs text-beton-400 mb-1">Fiili Bitiş</label>
            <input type="date" value={actualFinish} onChange={(e) => setActualFinish(e.target.value)}
              className="w-full rounded-md bg-beton-950 border border-beton-800 px-2 py-1.5 text-sm text-beton-100" />
          </div>
        </div>

        <label className="block mt-3 text-xs text-beton-400 mb-1">İlerleme Kaynağı</label>
        <select value={progressSource} onChange={(e) => setProgressSource(e.target.value as any)}
          className="w-full rounded-md bg-beton-950 border border-beton-800 px-2 py-1.5 text-sm text-beton-100">
          <option value="manual">Elle girilir</option>
          <option value="derived">Hakedişten türetilir (bağlı pozlar)</option>
        </select>
        {progressSource === "manual" ? (
          <>
            <label className="block mt-3 text-xs text-beton-400 mb-1">İlerleme (%)</label>
            <input type="number" min={0} max={100} value={manualProgress} onChange={(e) => setManualProgress(e.target.value)}
              className="w-full rounded-md bg-beton-950 border border-beton-800 px-2 py-1.5 text-sm text-beton-100" />
          </>
        ) : (
          <p className="mt-2 text-xs text-beton-500">
            Bu kalem bir üst kalemse veya poz bağlanmamışsa ilerleme 0 görünür — kaydettikten sonra "Poz" ile hakedişe bağlayın.
          </p>
        )}

        <label className="block mt-3 text-xs text-beton-400 mb-1">Ağırlık (üst kalemde ağırlıklı ortalama için)</label>
        <input type="number" step="0.01" value={weight} onChange={(e) => setWeight(e.target.value)}
          className="w-full rounded-md bg-beton-950 border border-beton-800 px-2 py-1.5 text-sm text-beton-100" />

        <div className="mt-5 flex justify-end gap-2">
          <button onClick={onClose} className="rounded-md border border-beton-700 text-beton-300 hover:bg-beton-800 px-3 py-1.5 text-sm transition">
            İptal
          </button>
          <button onClick={save} disabled={busy || !wbsCode.trim() || !name.trim()}
            className="rounded-md bg-emniyet-500 hover:bg-emniyet-600 disabled:opacity-60 text-beton-950 font-semibold px-3 py-1.5 text-sm transition">
            {busy ? "Kaydediliyor…" : "Kaydet"}
          </button>
        </div>
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Poz bağlama modalı
// ---------------------------------------------------------------------------

function PozLinkModal({ item, projectId, onClose, onChanged }: { item: ScheduleItem; projectId: string; onClose: () => void; onChanged: () => void }) {
  const [pozlar, setPozlar] = useState<AvailablePoz[]>([]);
  const [selected, setSelected] = useState("");
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    api<{ pozlar: AvailablePoz[] }>(`/projects/${projectId}/schedule/available-pozlar`, { projectId }).then((r) => setPozlar(r.pozlar));
  }, [projectId]);

  async function link() {
    if (!selected) return;
    setBusy(true);
    try {
      await api(`/schedule/items/${item.id}/pozlar`, { method: "POST", body: { poz_id: selected, action: "link" } });
      onChanged();
    } catch (e: any) {
      window.alert(e?.api?.message || "Bağlanamadı.");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="fixed inset-0 z-50 bg-black/60 flex items-center justify-center p-4" onClick={onClose}>
      <div className="w-full max-w-md rounded-xl border border-beton-700 bg-beton-900 p-5" onClick={(e) => e.stopPropagation()}>
        <h2 className="text-lg font-bold text-beton-100">Poz Bağla — {item.name}</h2>
        <p className="text-xs text-beton-400 mt-1">
          Bu WBS kalemini bir sözleşme pozuna bağlayın; ilerlemesi onaylı hakedişten türetilecekse "İlerleme Kaynağı: Hakedişten
          türetilir" seçili olmalı.
        </p>
        <select value={selected} onChange={(e) => setSelected(e.target.value)}
          className="w-full mt-3 rounded-md bg-beton-950 border border-beton-800 px-2 py-1.5 text-sm text-beton-100">
          <option value="">Poz seçin…</option>
          {pozlar.map((p) => (
            <option key={p.id} value={p.id}>
              {p.poz_no} — {p.description} ({p.subcontractor_adi}, {p.contract_qty} {p.unit})
            </option>
          ))}
        </select>
        {pozlar.length === 0 && <p className="mt-2 text-xs text-beton-500">Bu projede henüz tanımlı poz (iş kalemi) yok.</p>}
        <div className="mt-5 flex justify-end gap-2">
          <button onClick={onClose} className="rounded-md border border-beton-700 text-beton-300 hover:bg-beton-800 px-3 py-1.5 text-sm transition">
            Kapat
          </button>
          <button onClick={link} disabled={busy || !selected}
            className="rounded-md bg-emniyet-500 hover:bg-emniyet-600 disabled:opacity-60 text-beton-950 font-semibold px-3 py-1.5 text-sm transition">
            {busy ? "Bağlanıyor…" : "Bağla"}
          </button>
        </div>
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Bağımlılık yönetim paneli
// ---------------------------------------------------------------------------

function DependencyPanel({ projectId, items, deps, onChanged }: { projectId: string; items: ScheduleItem[]; deps: Dependency[]; onChanged: () => void }) {
  const [pred, setPred] = useState("");
  const [succ, setSucc] = useState("");
  const [depType, setDepType] = useState("FS");
  const [lagDays, setLagDays] = useState("0");
  const [err, setErr] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const nameOf = (id: string) => items.find((i) => i.id === id)?.wbs_code ?? id.slice(0, 8);

  async function add() {
    if (!pred || !succ) return;
    setErr(null);
    setBusy(true);
    try {
      await api(`/projects/${projectId}/schedule/dependencies`, {
        method: "POST", projectId,
        body: { predecessor_id: pred, successor_id: succ, dep_type: depType, lag_days: Number(lagDays) || 0 },
      });
      setPred(""); setSucc("");
      onChanged();
    } catch (e: any) {
      setErr(e?.api?.message || "Eklenemedi.");
    } finally {
      setBusy(false);
    }
  }

  async function remove(d: Dependency) {
    await api(`/schedule/dependencies/${d.predecessor_id}/${d.successor_id}`, { method: "DELETE" });
    onChanged();
  }

  return (
    <div className="mt-2 rounded-lg border border-beton-800 bg-beton-950 p-4 max-w-2xl">
      <div className="flex flex-wrap items-end gap-2">
        <div>
          <label className="block text-xs text-beton-400 mb-1">Önce (predecessor)</label>
          <select value={pred} onChange={(e) => setPred(e.target.value)} className="rounded-md bg-beton-900 border border-beton-800 px-2 py-1.5 text-xs text-beton-100">
            <option value="">Seçin…</option>
            {items.map((i) => <option key={i.id} value={i.id}>{i.wbs_code} — {i.name}</option>)}
          </select>
        </div>
        <div>
          <label className="block text-xs text-beton-400 mb-1">Sonra (successor)</label>
          <select value={succ} onChange={(e) => setSucc(e.target.value)} className="rounded-md bg-beton-900 border border-beton-800 px-2 py-1.5 text-xs text-beton-100">
            <option value="">Seçin…</option>
            {items.map((i) => <option key={i.id} value={i.id}>{i.wbs_code} — {i.name}</option>)}
          </select>
        </div>
        <div>
          <label className="block text-xs text-beton-400 mb-1">Tip</label>
          <select value={depType} onChange={(e) => setDepType(e.target.value)} className="rounded-md bg-beton-900 border border-beton-800 px-2 py-1.5 text-xs text-beton-100">
            <option value="FS">FS</option><option value="SS">SS</option><option value="FF">FF</option><option value="SF">SF</option>
          </select>
        </div>
        <div>
          <label className="block text-xs text-beton-400 mb-1">Gecikme (gün)</label>
          <input type="number" value={lagDays} onChange={(e) => setLagDays(e.target.value)} className="w-20 rounded-md bg-beton-900 border border-beton-800 px-2 py-1.5 text-xs text-beton-100" />
        </div>
        <button onClick={add} disabled={busy || !pred || !succ}
          className="rounded-md bg-emniyet-500 hover:bg-emniyet-600 disabled:opacity-60 text-beton-950 font-semibold px-3 py-1.5 text-xs transition">
          Ekle
        </button>
      </div>
      {err && <p className="mt-2 text-xs text-red-400">{err}</p>}

      <div className="mt-4 space-y-1">
        {deps.length === 0 ? (
          <p className="text-xs text-beton-500">Henüz bağımlılık tanımlanmamış.</p>
        ) : deps.map((d) => (
          <div key={`${d.predecessor_id}-${d.successor_id}`} className="flex items-center gap-2 text-xs text-beton-300">
            <span className="font-mono">{nameOf(d.predecessor_id)} → {nameOf(d.successor_id)}</span>
            <span className="text-beton-500">({d.dep_type}{d.lag_days ? `, +${d.lag_days}g` : ""})</span>
            <button onClick={() => remove(d)} className="ml-auto text-red-400 hover:text-red-300">Sil</button>
          </div>
        ))}
      </div>
    </div>
  );
}
