import { useEffect, useState } from "react";
import { api } from "../../api/client";
import { useProjects } from "../ProjectContext";

// ── Tipler ────────────────────────────────────────────────────────────────────
interface Project {
  id: string; code: string; name: string; location?: string;
  client_name?: string; budget_total?: number; currency: string;
  start_date?: string; end_date?: string; status: string;
}
interface Milestone {
  id: string; name: string; planned_date?: string; actual_date?: string;
  weight_pct?: number; status: string; sort_order: number;
}
interface Task { id: string; status: string; priority: string; due_date?: string; }
interface DailyReport { id: string; report_date: string; status: string; }

const STATUS_LABEL: Record<string, string> = {
  Planning: "Planlama", Active: "Aktif", OnHold: "Beklemede",
  Closed: "Tamamlandı", Archived: "Arşiv",
};
const STATUS_CLS: Record<string, string> = {
  Planning: "bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-300",
  Active: "bg-green-100 text-green-700 dark:bg-green-900/40 dark:text-green-300",
  OnHold: "bg-yellow-100 text-yellow-700 dark:bg-yellow-900/40 dark:text-yellow-300",
  Closed: "bg-gray-100 text-gray-600 dark:bg-gray-700 dark:text-gray-300",
  Archived: "bg-gray-100 text-gray-500 dark:bg-gray-800 dark:text-gray-400",
};
const MS_STATUS: Record<string, { label: string; dot: string }> = {
  Planned:    { label: "Bekliyor",     dot: "bg-gray-400" },
  InProgress: { label: "Devam Ediyor", dot: "bg-blue-500" },
  Completed:  { label: "Tamamlandı",   dot: "bg-green-500" },
  Delayed:    { label: "Gecikmiş",     dot: "bg-red-500" },
};

function fmt(n?: number, cur = "TRY") {
  if (n == null) return "—";
  return new Intl.NumberFormat("tr-TR", { style: "currency", currency: cur, maximumFractionDigits: 0 }).format(n);
}
function fmtDate(s?: string) {
  if (!s) return "—";
  return new Date(s).toLocaleDateString("tr-TR", { day: "2-digit", month: "short", year: "numeric" });
}
function diffDays(a: string, b: string) {
  return Math.round((new Date(b).getTime() - new Date(a).getTime()) / 86400000);
}
function progressPct(start?: string, end?: string): number {
  if (!start || !end) return 0;
  const total = diffDays(start, end);
  if (total <= 0) return 100;
  const elapsed = diffDays(start, new Date().toISOString().slice(0, 10));
  return Math.min(100, Math.max(0, Math.round((elapsed / total) * 100)));
}

function KPICard({ label, value, sub, accent }: { label: string; value: string; sub?: string; accent?: string }) {
  return (
    <div className="rounded-xl border border-[var(--border)] bg-[var(--card)] p-4 flex flex-col gap-1">
      <span className="text-xs font-medium text-[var(--text-muted)] uppercase tracking-wide">{label}</span>
      <span className={`text-2xl font-bold ${accent ?? "text-[var(--text)]"}`}>{value}</span>
      {sub && <span className="text-xs text-[var(--text-muted)]">{sub}</span>}
    </div>
  );
}

function MilestoneRow({ ms }: { ms: Milestone }) {
  const meta = MS_STATUS[ms.status] ?? MS_STATUS["Planned"];
  return (
    <div className="flex items-center gap-3 py-2 border-b border-[var(--border)] last:border-0">
      <span className={`w-2.5 h-2.5 rounded-full flex-shrink-0 ${meta.dot}`} />
      <span className="flex-1 text-sm text-[var(--text)] truncate">{ms.name}</span>
      <span className="text-xs text-[var(--text-muted)] w-28 text-right shrink-0">
        {ms.planned_date ? fmtDate(ms.planned_date) : "—"}
      </span>
      <span className={`text-xs px-2 py-0.5 rounded-full font-medium shrink-0 ${
        ms.status === "Completed"  ? "bg-green-100 text-green-700 dark:bg-green-900/40 dark:text-green-300" :
        ms.status === "Delayed"    ? "bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-300" :
        ms.status === "InProgress" ? "bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-300" :
        "bg-gray-100 text-gray-500 dark:bg-gray-700 dark:text-gray-400"
      }`}>{meta.label}</span>
    </div>
  );
}

export default function ProjectSummaryPage() {
  const { current: proj } = useProjects();
  const [project, setProject]     = useState<Project | null>(null);
  const [milestones, setMilestones] = useState<Milestone[]>([]);
  const [tasks, setTasks]           = useState<Task[]>([]);
  const [reports, setReports]       = useState<DailyReport[]>([]);
  const [loading, setLoading]       = useState(true);
  const [error, setError]           = useState(false);

  useEffect(() => {
    if (!proj?.id) return;
    setLoading(true); setError(false);
    const pid = proj.id;
    Promise.all([
      api<{ project: Project }>(`/projects/${pid}`, { projectId: pid }),
      api<{ milestones: Milestone[] }>(`/projects/${pid}/milestones`, { projectId: pid }),
      api<{ tasks: Task[] }>(`/projects/${pid}/tasks`, { projectId: pid }),
      api<{ reports: DailyReport[] }>(`/projects/${pid}/daily-reports`, { projectId: pid })
        .catch(() => ({ reports: [] })),
    ]).then(([pd, md, td, rd]) => {
      setProject(pd.project);
      setMilestones(md.milestones ?? []);
      setTasks(td.tasks ?? []);
      setReports((rd.reports ?? []).slice(0, 5));
    }).catch(() => setError(true))
      .finally(() => setLoading(false));
  }, [proj?.id]);

  if (!proj) return <div className="p-8 text-[var(--text-muted)]">Proje seçilmedi.</div>;
  if (loading) return <div className="p-8 text-[var(--text-muted)]">Yükleniyor…</div>;
  if (error || !project) return <div className="p-8 text-red-500">Veriler yüklenemedi.</div>;

  const timePct = progressPct(project.start_date?.slice(0, 10), project.end_date?.slice(0, 10));
  const msCompleted = milestones.filter(m => m.status === "Completed").length;
  const msTotal     = milestones.length;
  const msPct       = msTotal > 0 ? Math.round((msCompleted / msTotal) * 100) : 0;

  const taskByStatus: Record<string, number> = {};
  tasks.forEach(t => { taskByStatus[t.status] = (taskByStatus[t.status] ?? 0) + 1; });
  const taskOverdue = tasks.filter(t =>
    t.due_date && t.status !== "Done" && t.status !== "Backlog" &&
    new Date(t.due_date) < new Date()
  ).length;
  const msDelayed = milestones.filter(m => m.status === "Delayed").length;

  const spiNum = timePct > 0 ? msPct / timePct : null;
  const spi    = spiNum != null ? spiNum.toFixed(2) : "—";

  const daysRemain = project.end_date
    ? diffDays(new Date().toISOString().slice(0, 10), project.end_date.slice(0, 10))
    : null;

  return (
    <div className="p-6 space-y-6 max-w-5xl mx-auto">
      {/* Başlık */}
      <div className="flex items-start justify-between gap-4 flex-wrap">
        <div>
          <div className="flex items-center gap-2 mb-1">
            <span className="text-xs font-mono bg-[var(--bg-hover)] text-[var(--text-muted)] px-2 py-0.5 rounded">
              {project.code}
            </span>
            <span className={`text-xs font-medium px-2 py-0.5 rounded-full ${STATUS_CLS[project.status] ?? ""}`}>
              {STATUS_LABEL[project.status] ?? project.status}
            </span>
          </div>
          <h1 className="text-xl font-bold text-[var(--text)]">{project.name}</h1>
          {project.client_name && (
            <p className="text-sm text-[var(--text-muted)] mt-0.5">{project.client_name}</p>
          )}
          {project.location && (
            <p className="text-xs text-[var(--text-muted)]">📍 {project.location}</p>
          )}
        </div>
        <div className="text-right text-sm text-[var(--text-muted)]">
          <div>{fmtDate(project.start_date?.slice(0, 10))} – {fmtDate(project.end_date?.slice(0, 10))}</div>
          {daysRemain != null && (
            <div className={`font-medium mt-0.5 ${
              daysRemain < 0 ? "text-red-500" :
              daysRemain < 30 ? "text-yellow-600" :
              "text-[var(--text)]"
            }`}>
              {daysRemain < 0 ? `${Math.abs(daysRemain)} gün geçti` : `${daysRemain} gün kaldı`}
            </div>
          )}
        </div>
      </div>

      {/* Zaman İlerlemesi */}
      <div className="rounded-xl border border-[var(--border)] bg-[var(--card)] p-4 space-y-2">
        <div className="flex justify-between text-xs text-[var(--text-muted)]">
          <span>Proje Zaman İlerlemesi</span>
          <span className="font-medium">{timePct}%</span>
        </div>
        <div className="h-2 bg-[var(--bg-hover)] rounded-full overflow-hidden">
          <div className="h-full bg-blue-500 rounded-full transition-all" style={{ width: `${timePct}%` }} />
        </div>
        <div className="flex justify-between text-[10px] text-[var(--text-muted)]">
          <span>{fmtDate(project.start_date?.slice(0, 10))}</span>
          <span>{fmtDate(project.end_date?.slice(0, 10))}</span>
        </div>
      </div>

      {/* KPI Grid */}
      <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
        <KPICard
          label="SPI (Tahmini)"
          value={spi}
          sub="Takvim Performans İndeksi"
          accent={
            spiNum == null ? undefined :
            spiNum >= 0.95 ? "text-green-600" :
            spiNum >= 0.80 ? "text-yellow-600" :
            "text-red-600"
          }
        />
        <KPICard
          label="Toplam Bütçe"
          value={fmt(project.budget_total, project.currency)}
          sub={project.currency}
        />
        <KPICard
          label="Kilometre Taşı"
          value={`${msCompleted}/${msTotal}`}
          sub={`${msPct}% tamamlandı`}
          accent={msDelayed > 0 ? "text-red-600" : "text-[var(--text)]"}
        />
        <KPICard
          label="Açık Görevler"
          value={String((taskByStatus["Todo"] ?? 0) + (taskByStatus["InProgress"] ?? 0))}
          sub={taskOverdue > 0 ? `${taskOverdue} gecikmiş` : "Gecikme yok"}
          accent={taskOverdue > 0 ? "text-red-600" : "text-[var(--text)]"}
        />
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        {/* Kilometre Taşları */}
        <div className="rounded-xl border border-[var(--border)] bg-[var(--card)] p-4">
          <h2 className="text-sm font-semibold text-[var(--text)] mb-3 flex items-center justify-between">
            Kilometre Taşları
            <span className="text-xs font-normal text-[var(--text-muted)]">{msCompleted}/{msTotal} tamamlandı</span>
          </h2>
          {milestones.length === 0 ? (
            <p className="text-sm text-[var(--text-muted)]">Henüz kilometre taşı tanımlanmamış.</p>
          ) : (
            <div>
              {milestones.slice(0, 8).map(ms => <MilestoneRow key={ms.id} ms={ms} />)}
            </div>
          )}
        </div>

        <div className="space-y-4">
          {/* Görev Dağılımı */}
          <div className="rounded-xl border border-[var(--border)] bg-[var(--card)] p-4">
            <h2 className="text-sm font-semibold text-[var(--text)] mb-3">Görev Dağılımı</h2>
            {tasks.length === 0 ? (
              <p className="text-sm text-[var(--text-muted)]">Görev bulunamadı.</p>
            ) : (
              <div className="space-y-2">
                {[
                  { key: "Todo",       label: "Yapılacak",  cls: "bg-gray-400" },
                  { key: "InProgress", label: "Devam Eden", cls: "bg-blue-500" },
                  { key: "Review",     label: "İncelemede", cls: "bg-yellow-500" },
                  { key: "Done",       label: "Tamamlandı", cls: "bg-green-500" },
                ].map(({ key, label, cls }) => {
                  const count = taskByStatus[key] ?? 0;
                  const pct   = Math.round((count / tasks.length) * 100);
                  return (
                    <div key={key} className="flex items-center gap-2">
                      <span className="text-xs text-[var(--text-muted)] w-24 shrink-0">{label}</span>
                      <div className="flex-1 h-1.5 bg-[var(--bg-hover)] rounded-full">
                        <div className={`h-full rounded-full ${cls}`} style={{ width: `${pct}%` }} />
                      </div>
                      <span className="text-xs font-medium text-[var(--text)] w-6 text-right">{count}</span>
                    </div>
                  );
                })}
                {taskOverdue > 0 && (
                  <div className="mt-2 text-xs text-red-500 font-medium">
                    ⚠ {taskOverdue} görev vadesi geçmiş
                  </div>
                )}
              </div>
            )}
          </div>

          {/* Son Saha Raporları */}
          <div className="rounded-xl border border-[var(--border)] bg-[var(--card)] p-4">
            <h2 className="text-sm font-semibold text-[var(--text)] mb-3">Son Saha Raporları</h2>
            {reports.length === 0 ? (
              <p className="text-sm text-[var(--text-muted)]">Rapor bulunamadı.</p>
            ) : (
              <div className="space-y-2">
                {reports.map(rp => (
                  <div key={rp.id} className="flex items-center justify-between text-sm">
                    <span className="text-[var(--text)]">{fmtDate(rp.report_date)}</span>
                    <span className={`text-xs px-2 py-0.5 rounded-full ${
                      rp.status === "submitted" ? "bg-green-100 text-green-700 dark:bg-green-900/40 dark:text-green-300" :
                      rp.status === "draft"     ? "bg-gray-100 text-gray-500 dark:bg-gray-700 dark:text-gray-400" :
                      "bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-300"
                    }`}>
                      {rp.status === "submitted" ? "Onaylandı" :
                       rp.status === "draft"     ? "Taslak" : rp.status}
                    </span>
                  </div>
                ))}
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}
