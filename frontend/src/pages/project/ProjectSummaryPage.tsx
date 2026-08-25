import { useEffect, useState } from "react";
import { api } from "../../api/client";
import { useProjects } from "../ProjectContext";
import type { ColorStop } from "../dashboard/RadialRing";
import SegmentedDonut, { type DonutSegment } from "../dashboard/SegmentedDonut";
import {
  PanelRow, PanelCell, PanelKpi,
  IconTrend, IconCheck, IconFlag,
} from "../dashboard/PanelKit";

// Proje Özet — Panel Dashboard'la (bkz. ../Dashboard.tsx) aynı görsel dil:
// kesintisiz koyu levha, hairline ayraçlar, gradyanlı/segmentli halkalar.
// --panel-* token'ları temadan bağımsız (index.css'te bare :root'ta tanımlı),
// bu yüzden bu sayfa da Panel gibi her zaman koyu render olur.

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
interface Giderler {
  satinalma: number;
  kasa_harcamalari: number;
  tasaron_hakedis: number;
  sabit_giderler: number;
}
interface SozlesmeTakipItem {
  id: string;
  kategori: string;
  poz_no: string;
  tanim: string;
  birim: string;
  miktar: number;
  eslesmeler: unknown[];
}

const STATUS_LABEL: Record<string, string> = {
  Planning: "Planlama", Active: "Aktif", OnHold: "Beklemede",
  Closed: "Tamamlandı", Archived: "Arşiv",
};
const MS_META: Record<string, { label: string; dot: string }> = {
  Planned:    { label: "Bekliyor",     dot: "rgb(var(--panel-hairline))" },
  InProgress: { label: "Devam Ediyor", dot: "#60a5fa" },
  Completed:  { label: "Tamamlandı",   dot: "var(--group-accent)" },
  Delayed:    { label: "Gecikmiş",     dot: "#f87171" },
};

const ZAMAN_STOPS: ColorStop[] = [{ t: 0, hex: "#22d3ee" }, { t: 0.5, hex: "#2f6fed" }, { t: 1, hex: "#6d5ef8" }];
const ADAMSAAT_STOPS: ColorStop[] = [{ t: 0, hex: "#f5a800" }, { t: 1, hex: "#fb923c" }];
const SATINALMA_STOPS: ColorStop[] = [{ t: 0, hex: "#60a5fa" }, { t: 1, hex: "#2f6fed" }];
const KASA_STOPS: ColorStop[] = [{ t: 0, hex: "#fbbf24" }, { t: 1, hex: "#f59e0b" }];
const TASERON_STOPS: ColorStop[] = [{ t: 0, hex: "#34d399" }, { t: 1, hex: "#10b981" }];
const SABIT_STOPS: ColorStop[] = [{ t: 0, hex: "#a78bfa" }, { t: 1, hex: "#8b5cf6" }];

// Adam-Saat Takip — proje genelinde adam-saat verimlilik takibi için henüz
// bir backend/veri kaynağı yok (bkz. manhour database notu). Gerçek veri
// bağlanana kadar temsili sabit değerler kullanılır; hesap mantığı
// (kullanılan/planlanan oranı) gerçek veriyle birebir aynı kalacak.
const ADAM_SAAT_KULLANILAN = 64200;
const ADAM_SAAT_PLANLANAN = 121500;

function fmt(n?: number, cur = "TRY") {
  if (n == null) return "—";
  return new Intl.NumberFormat("tr-TR", { style: "currency", currency: cur, maximumFractionDigits: 0 }).format(n);
}
function fmtDate(s?: string) {
  if (!s) return "—";
  return new Date(s).toLocaleDateString("tr-TR", { day: "2-digit", month: "short", year: "numeric" });
}
function fmtShort(s?: string | null) {
  if (!s) return "—";
  const [y, m, d] = s.slice(0, 10).split("-");
  if (!y || !m || !d) return "—";
  return `${d}.${m}.${y}`;
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

export default function ProjectSummaryPage() {
  const { current: proj } = useProjects();
  const [project, setProject]     = useState<Project | null>(null);
  const [milestones, setMilestones] = useState<Milestone[]>([]);
  const [tasks, setTasks]           = useState<Task[]>([]);
  const [reports, setReports]       = useState<DailyReport[]>([]);
  const [giderler, setGiderler]     = useState<Giderler | null>(null);
  const [kalanIsler, setKalanIsler] = useState<SozlesmeTakipItem[]>([]);
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
      api<{ dashboard: { giderler?: Giderler } }>(`/projects/${pid}/dashboard`, { projectId: pid })
        .catch(() => ({ dashboard: { giderler: undefined } })),
      api<{ items: SozlesmeTakipItem[] }>(`/projects/${pid}/sozlesme-takip`, { projectId: pid })
        .catch(() => ({ items: [] })),
    ]).then(([pd, md, td, rd, dashd, std]) => {
      setProject(pd.project);
      setMilestones(md.milestones ?? []);
      setTasks(td.tasks ?? []);
      setReports((rd.reports ?? []).slice(0, 5));
      setGiderler(dashd.dashboard?.giderler ?? null);
      setKalanIsler((std.items ?? []).filter(it => (it.eslesmeler ?? []).length === 0));
    }).catch(() => setError(true))
      .finally(() => setLoading(false));
  }, [proj?.id]);

  if (!proj) return <div className="p-8 text-beton-400">Proje seçilmedi.</div>;
  if (loading) return <div className="p-8 text-beton-400">Yükleniyor…</div>;
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
  const openTasks = (taskByStatus["Todo"] ?? 0) + (taskByStatus["InProgress"] ?? 0);

  const spiNum = timePct > 0 ? msPct / timePct : null;

  const daysRemain = project.end_date
    ? diffDays(new Date().toISOString().slice(0, 10), project.end_date.slice(0, 10))
    : null;

  const giderSegments: DonutSegment[] = giderler ? [
    { label: "Satınalmalar", value: giderler.satinalma, colorStops: SATINALMA_STOPS, swatch: "#60a5fa" },
    { label: "Kasa Harcamaları", value: giderler.kasa_harcamalari, colorStops: KASA_STOPS, swatch: "#f59e0b" },
    { label: "Taşeron Hakedişler", value: giderler.tasaron_hakedis, colorStops: TASERON_STOPS, swatch: "#10b981" },
    { label: "Sabit Giderler", value: giderler.sabit_giderler, colorStops: SABIT_STOPS, swatch: "#8b5cf6" },
  ] : [];
  const giderTotal = giderler ? giderler.satinalma + giderler.kasa_harcamalari + giderler.tasaron_hakedis + giderler.sabit_giderler : 0;

  const adamSaatPct = Math.round((ADAM_SAAT_KULLANILAN / ADAM_SAAT_PLANLANAN) * 100);

  return (
    <div className="relative">
      {/* Bulanık mor/eflatun ambiyans — yalnızca panelin ARKASINDA/ÇEVRESİNDE
          (bu wrapper'ın kendisi kadar geniş, hafifçe taşan). Panel'in kendisi
          aşağıda tamamen opak (--panel-bg) olduğundan üzerindeki hiçbir
          yazı/tablo bu katmandan etkilenmez — okunabilirlik yapısal olarak
          garanti. */}
      <div
        aria-hidden
        className="pointer-events-none absolute -z-10"
        style={{
          inset: "-70px -48px",
          background: `
            radial-gradient(ellipse 640px 480px at 6% 0%, rgba(168,85,247,.30), transparent 60%),
            radial-gradient(ellipse 560px 460px at 96% 6%, rgba(217,70,239,.24), transparent 62%),
            radial-gradient(ellipse 520px 560px at 100% 100%, rgba(99,60,224,.26), transparent 60%),
            radial-gradient(ellipse 440px 380px at 2% 100%, rgba(190,50,180,.2), transparent 60%)
          `,
          filter: "blur(70px)",
        }}
      />
      <div
        className="relative rounded-xl overflow-hidden border"
        style={{ background: "rgb(var(--panel-bg))", borderColor: "rgb(var(--panel-hairline))" }}
      >
      {/* ── Head ── */}
      <div
        className="flex items-start justify-between gap-4 flex-wrap px-5 py-4 border-b"
        style={{ borderColor: "rgb(var(--panel-hairline))" }}
      >
        <div>
          <p className="text-[10px] font-bold uppercase tracking-[.1em]" style={{ color: "var(--group-accent)" }}>
            Proje özeti
          </p>
          <h1 className="font-display text-xl font-extrabold mt-1.5 flex items-center gap-2 flex-wrap" style={{ color: "rgb(var(--panel-ink))" }}>
            {project.code} — {project.name}
            <span
              className="text-[11px] font-semibold px-2 py-0.5 rounded-full"
              style={{ background: "rgba(74,222,128,.14)", color: "#4ade80", border: "1px solid rgba(74,222,128,.3)" }}
            >
              {STATUS_LABEL[project.status] ?? project.status}
            </span>
          </h1>
          <p className="mt-1 text-xs" style={{ color: "rgb(var(--panel-ink2))" }}>
            {project.client_name}
            {project.location && <> · 📍 {project.location}</>}
          </p>
        </div>
        <div className="text-right text-xs" style={{ color: "rgb(var(--panel-ink2))" }}>
          <div>
            <b style={{ color: "rgb(var(--panel-ink))" }}>{fmtDate(project.start_date?.slice(0, 10))}</b>
            {" – "}
            <b style={{ color: "rgb(var(--panel-ink))" }}>{fmtDate(project.end_date?.slice(0, 10))}</b>
          </div>
          {daysRemain != null && (
            <div className="mt-1 font-medium" style={{ color: daysRemain < 0 ? "#f87171" : "rgb(var(--panel-ink2))" }}>
              {daysRemain < 0 ? `${Math.abs(daysRemain)} gün geçti` : `${daysRemain} gün kaldı`}
            </div>
          )}
        </div>
      </div>

      {/* ── Hero satırı: Zaman İlerlemesi + Adam-Saat Takip + Giderler ── */}
      <PanelRow cols="1fr 1fr 1.3fr">
        <PanelCell title="Zaman İlerlemesi">
          <div className="flex items-center gap-5">
            <SolidRing pct={timePct} colorStops={ZAMAN_STOPS} size={116} gradId="ring-zaman" />
            <div className="flex flex-col gap-1.5 min-w-0 flex-1">
              <MetaRow label="Başlangıç" value={fmtDate(project.start_date?.slice(0, 10))} />
              <MetaRow label="Bitiş" value={fmtDate(project.end_date?.slice(0, 10))} />
              {daysRemain != null && (
                <MetaRow
                  label="Kalan"
                  value={daysRemain < 0 ? `${Math.abs(daysRemain)} gün geçti` : `${daysRemain} gün`}
                  bad={daysRemain < 0}
                />
              )}
              {spiNum != null && (
                <p
                  className="text-[11px] mt-1 font-medium"
                  style={{ color: spiNum < 0.8 ? "#f87171" : spiNum >= 0.95 ? "#4ade80" : "var(--group-accent)" }}
                >
                  SPI {spiNum.toFixed(2)} — {spiNum >= 0.95 ? "Plana uygun" : spiNum >= 0.8 ? "Hafif gecikme" : "Ciddi gecikme"}
                </p>
              )}
            </div>
          </div>
        </PanelCell>
        <PanelCell title="Adam-Saat Takip" action={
          <span className="text-[10.5px]" style={{ color: "rgb(var(--panel-ink3))" }}>kullanılan / planlanan</span>
        }>
          <div className="flex items-center gap-5">
            <SolidRing pct={adamSaatPct} colorStops={ADAMSAAT_STOPS} size={116} gradId="ring-adamsaat" />
            <div className="flex flex-col gap-1.5 min-w-0 flex-1">
              <MetaRow label="Kullanılan" value={`${ADAM_SAAT_KULLANILAN.toLocaleString("tr-TR")} sa.`} />
              <MetaRow label="Planlanan" value={`${ADAM_SAAT_PLANLANAN.toLocaleString("tr-TR")} sa.`} />
              <MetaRow label="Kalan" value={`${(ADAM_SAAT_PLANLANAN - ADAM_SAAT_KULLANILAN).toLocaleString("tr-TR")} sa.`} />
              <p
                className="text-[11px] mt-1 font-medium"
                style={{ color: adamSaatPct < timePct ? "var(--group-accent)" : "#4ade80" }}
              >
                {adamSaatPct < timePct ? "Zaman ilerlemesinin gerisinde" : "Zaman ilerlemesiyle uyumlu"}
              </p>
            </div>
          </div>
        </PanelCell>
        <PanelCell title="Giderler" action={
          giderTotal > 0 ? <span className="text-[10.5px]" style={{ color: "rgb(var(--panel-ink3))" }}>toplam</span> : undefined
        }>
          {giderTotal > 0 ? (
            <div className="h-full flex items-center">
              <SegmentedDonut size={132} centerLabel="TOPLAM" formatValue={(v) => fmt(v, project.currency)} segments={giderSegments} />
            </div>
          ) : (
            <p className="text-sm" style={{ color: "rgb(var(--panel-ink2))" }}>Henüz gider kaydı yok.</p>
          )}
        </PanelCell>
      </PanelRow>

      {/* ── İkincil KPI şeridi ── */}
      <div className="grid grid-cols-2 sm:grid-cols-4" style={{ borderBottom: "1px solid rgb(var(--panel-hairline))" }}>
        <PanelKpi
          label="SPI (Tahmini)"
          value={spiNum != null ? spiNum.toFixed(2) : "—"}
          hint={spiNum == null ? undefined : spiNum >= 0.95 ? "Plana uygun" : spiNum >= 0.8 ? "Hafif gecikme" : "Ciddi gecikme"}
          bad={spiNum != null && spiNum < 0.8}
          good={spiNum != null && spiNum >= 0.95}
          icon={<IconTrend />}
        />
        <PanelKpi
          label="Toplam Bütçe"
          value={fmt(project.budget_total, project.currency)}
          hint={project.currency}
          icon={<IconReceiptMini />}
        />
        <PanelKpi
          label="Kilometre Taşı"
          value={`${msCompleted} / ${msTotal}`}
          hint={`%${msPct} tamamlandı`}
          bad={msDelayed > 0}
          icon={<IconFlag />}
        />
        <PanelKpi
          label="Açık Görevler"
          value={String(openTasks)}
          hint={taskOverdue > 0 ? `${taskOverdue} gecikmiş` : "Gecikme yok"}
          bad={taskOverdue > 0}
          good={taskOverdue === 0}
          icon={<IconCheck />}
        />
      </div>

      {/* ── Row: Kalan İşler ── */}
      <PanelRow cols="1fr">
        <PanelCell title="Kalan İşler — Anlaşılacak İmalatlar" action={
          kalanIsler.length > 0
            ? <span className="text-[11px]" style={{ color: "rgb(var(--panel-ink3))" }}>{kalanIsler.length} kalem</span>
            : undefined
        }>
          {kalanIsler.length === 0 ? (
            <p className="text-sm" style={{ color: "rgb(var(--panel-ink2))" }}>Tüm imalat kalemleri bir taşeron sözleşmesiyle ilişkilendirilmiş.</p>
          ) : (
            <>
              <p className="text-[11px] mb-2" style={{ color: "rgb(var(--panel-ink3))" }}>
                Proje keşfinde tanımlı, henüz taşeron sözleşmesiyle (poz no) eşleşmemiş kalemler.
              </p>
              <div className="overflow-x-auto">
                <table className="w-full text-[12px]">
                  <thead>
                    <tr style={{ borderBottom: "1px solid rgb(var(--panel-hairline))" }}>
                      <th className="py-1.5 pr-3 text-left font-semibold" style={{ color: "rgb(var(--panel-ink3))" }}>Poz No</th>
                      <th className="py-1.5 pr-3 text-left font-semibold" style={{ color: "rgb(var(--panel-ink3))" }}>Tanım</th>
                      <th className="py-1.5 pr-3 text-left font-semibold" style={{ color: "rgb(var(--panel-ink3))" }}>Kategori</th>
                      <th className="py-1.5 pr-3 text-right font-semibold" style={{ color: "rgb(var(--panel-ink3))" }}>Miktar</th>
                    </tr>
                  </thead>
                  <tbody>
                    {kalanIsler.slice(0, 6).map(it => (
                      <tr key={it.id} style={{ borderBottom: "1px solid rgb(var(--panel-hairline) / .5)" }}>
                        <td className="py-1.5 pr-3 font-mono text-[11px]" style={{ color: "rgb(var(--panel-ink3))" }}>{it.poz_no || "—"}</td>
                        <td className="py-1.5 pr-3" style={{ color: "rgb(var(--panel-ink))" }}>{it.tanim}</td>
                        <td className="py-1.5 pr-3 text-[11px]" style={{ color: "rgb(var(--panel-ink2))" }}>{it.kategori}</td>
                        <td className="py-1.5 pr-3 text-right tabular-nums" style={{ color: "rgb(var(--panel-ink2))" }}>
                          {it.miktar.toLocaleString("tr-TR", { maximumFractionDigits: 2 })} {it.birim}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
                {kalanIsler.length > 6 && (
                  <p className="text-[10.5px] mt-2" style={{ color: "rgb(var(--panel-ink3))" }}>
                    + {kalanIsler.length - 6} kalem daha — tümü için Sözleşme Takip sayfasına bakın.
                  </p>
                )}
              </div>
            </>
          )}
        </PanelCell>
      </PanelRow>

      {/* ── Row: Kilometre Taşları + Görev Dağılımı ── */}
      <PanelRow cols="1fr 1fr">
        <PanelCell title="Kilometre Taşları" action={
          <span className="text-[11px]" style={{ color: "rgb(var(--panel-ink3))" }}>{msCompleted}/{msTotal} tamamlandı</span>
        }>
          {milestones.length === 0 ? (
            <p className="text-sm" style={{ color: "rgb(var(--panel-ink2))" }}>Henüz kilometre taşı tanımlanmamış.</p>
          ) : (
            <ul className="flex flex-col">
              {milestones.slice(0, 8).map((ms, i) => {
                const meta = MS_META[ms.status] ?? MS_META.Planned;
                return (
                  <li
                    key={ms.id}
                    className="flex items-center gap-3 text-[12.5px] py-2"
                    style={i < Math.min(milestones.length, 8) - 1 ? { borderBottom: "1px solid rgb(var(--panel-hairline))" } : undefined}
                  >
                    <span className="inline-block w-2.5 h-2.5 rounded-full shrink-0" style={{ background: meta.dot }} />
                    <span className="flex-1 truncate" style={{ color: "rgb(var(--panel-ink))" }}>{ms.name}</span>
                    <span
                      className="font-mono text-[10.5px] shrink-0"
                      style={{ color: ms.status === "Delayed" ? "#f87171" : "rgb(var(--panel-ink3))" }}
                    >
                      {ms.status === "Delayed" ? "GECİKMİŞ" : fmtShort(ms.actual_date ?? ms.planned_date)}
                    </span>
                  </li>
                );
              })}
            </ul>
          )}
        </PanelCell>
        <PanelCell title="Görev Dağılımı">
          {tasks.length === 0 ? (
            <p className="text-sm" style={{ color: "rgb(var(--panel-ink2))" }}>Görev bulunamadı.</p>
          ) : (
            <>
              {[
                { key: "Todo",       label: "Yapılacak",  color: "#8b93a3" },
                { key: "InProgress", label: "Devam Eden", color: "#2f6fed" },
                { key: "Review",     label: "İncelemede", color: "var(--group-accent)" },
                { key: "Done",       label: "Tamamlandı", color: "#22c55e" },
              ].map(({ key, label, color }) => {
                const count = taskByStatus[key] ?? 0;
                const pct   = Math.round((count / tasks.length) * 100);
                return (
                  <div key={key} className="flex items-center gap-2 mb-2 last:mb-0">
                    <span className="text-[11px] w-20 shrink-0" style={{ color: "rgb(var(--panel-ink3))" }}>{label}</span>
                    <div className="flex-1 h-1.5 rounded-full" style={{ background: "rgb(var(--panel-hairline))" }}>
                      <div className="h-full rounded-full" style={{ width: `${pct}%`, background: color }} />
                    </div>
                    <span className="text-[11.5px] font-bold w-5 text-right tabular-nums" style={{ color: "rgb(var(--panel-ink))" }}>{count}</span>
                  </div>
                );
              })}
              {taskOverdue > 0 && (
                <div className="mt-2 text-[11px] font-medium" style={{ color: "#f87171" }}>
                  ⚠ {taskOverdue} görev vadesi geçmiş
                </div>
              )}
            </>
          )}
        </PanelCell>
      </PanelRow>

      {/* ── Row: Son Saha Raporları ── */}
      <PanelRow cols="1fr">
        <PanelCell title="Son Saha Raporları">
          {reports.length === 0 ? (
            <p className="text-sm" style={{ color: "rgb(var(--panel-ink2))" }}>Rapor bulunamadı.</p>
          ) : (
            <div className="flex flex-wrap gap-x-6 gap-y-2">
              {reports.map(rp => {
                const tone = rp.status === "submitted" ? { bg: "rgba(74,222,128,.14)", fg: "#4ade80" }
                  : rp.status === "draft" ? { bg: "rgb(var(--panel-hairline) / .6)", fg: "rgb(var(--panel-ink2))" }
                  : { bg: "rgba(96,165,250,.15)", fg: "#60a5fa" };
                return (
                  <div key={rp.id} className="flex items-center gap-2 text-[12.5px]">
                    <span style={{ color: "rgb(var(--panel-ink))" }}>{fmtDate(rp.report_date)}</span>
                    <span className="text-[10.5px] font-semibold px-2 py-0.5 rounded-full" style={{ background: tone.bg, color: tone.fg }}>
                      {rp.status === "submitted" ? "Onaylandı" : rp.status === "draft" ? "Taslak" : rp.status}
                    </span>
                  </div>
                );
              })}
            </div>
          )}
        </PanelCell>
      </PanelRow>
      </div>
    </div>
  );
}

function MetaRow({ label, value, bad }: { label: string; value: string; bad?: boolean }) {
  return (
    <div className="flex items-center justify-between gap-3 text-[11.5px]">
      <span style={{ color: "rgb(var(--panel-ink3))" }}>{label}</span>
      <span className="font-bold tabular-nums" style={{ color: bad ? "#f87171" : "rgb(var(--panel-ink))" }}>{value}</span>
    </div>
  );
}

// Tek metrikli, DÜZ (kesiksiz/dolgulu) yay halkası — RadialRing'in tick'li
// (kesikli) stiline alternatif. Şimdilik yalnızca bu sayfada kullanılıyor;
// gradId, aynı sayfada birden çok halka olduğunda SVG <linearGradient>
// id çakışmasını önlemek için benzersiz olmalı.
function SolidRing({ pct, colorStops, size = 116, gradId }: { pct: number; colorStops: ColorStop[]; size?: number; gradId: string }) {
  const strokeWidth = size * 0.13;
  const r = (size - strokeWidth) / 2;
  const cx = size / 2;
  const cy = size / 2;
  const circumference = 2 * Math.PI * r;
  const clamped = Math.max(0, Math.min(100, pct));
  const len = (clamped / 100) * circumference;

  return (
    <div className="relative shrink-0" style={{ width: size, height: size }}>
      <svg width={size} height={size} viewBox={`0 0 ${size} ${size}`}>
        <defs>
          <linearGradient id={gradId} x1="0%" y1="0%" x2="100%" y2="100%">
            {colorStops.map((s, i) => (
              <stop key={i} offset={`${s.t * 100}%`} stopColor={s.hex} />
            ))}
          </linearGradient>
        </defs>
        <circle cx={cx} cy={cy} r={r} fill="none" stroke="rgb(var(--panel-hairline))" strokeWidth={strokeWidth} />
        <circle
          cx={cx}
          cy={cy}
          r={r}
          fill="none"
          stroke={`url(#${gradId})`}
          strokeWidth={strokeWidth}
          strokeLinecap="round"
          strokeDasharray={`${len} ${Math.max(0, circumference - len)}`}
          transform={`rotate(-90 ${cx} ${cy})`}
        />
      </svg>
      <div className="absolute inset-0 flex items-center justify-center">
        <span
          className="font-display font-extrabold"
          style={{ fontSize: size * 0.167, fontVariantNumeric: "tabular-nums", color: "rgb(var(--panel-ink))" }}
        >
          %{clamped.toFixed(1)}
        </span>
      </div>
    </div>
  );
}

function IconReceiptMini() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round" width="100%" height="100%">
      <rect x="3" y="7" width="18" height="13" rx="2" /><path d="M8 7V5a2 2 0 012-2h4a2 2 0 012 2v2" />
    </svg>
  );
}
