import type { ReactNode } from "react";
import { useCallback, useEffect, useRef, useState } from "react";
import { Link } from "react-router-dom";
import { api, apiFetchBlob, apiUpload } from "../api/client";
import { useAuth } from "../auth/AuthContext";
import { Can } from "../auth/guards";
import { useProjects } from "../projects/ProjectContext";
import SCurve from "./dashboard/SCurve";
import type { SCurvePoint } from "./dashboard/SCurve";
import SegmentedDonut from "./dashboard/SegmentedDonut";
import type { DonutSegment } from "./dashboard/SegmentedDonut";
import RadialRing from "./dashboard/RadialRing";
import type { ColorStop } from "./dashboard/RadialRing";

// Faz 9 — rol duyarlı proje dashboard'u. Finansal blok (EVM) backend'de izinle
// süzülür: reports.view_financial_reports yoksa `evm` alanı hiç gelmez; taşeron
// temsilcisi yalnızca kendi firmasının sayaçlarını görür (satır seviyesi güvenlik).
//
// Panel v3 — kullanıcının paylaştığı modern referans görsele göre tam
// yeniden tasarım: ayrı "kart" ızgarası yerine TEK bir koyu levha, bölümler
// arasında sadece ince ayraç (hairline) çizgileri var — gölge, gradyan,
// parlama yok ("dolu dolu ve kesintisiz" geri bildirimi). Bu levha kasıtlı
// olarak HER ZAMAN koyu render olur (bkz. index.css --panel-* token'ları,
// [data-theme] bloklarının dışında tanımlı) — açık temada bile Panel koyu
// bir çerçeve içinde kalır. Tüm halka/donut grafikler segmentli/gradyanlı
// tick stilinde (RadialRing/SegmentedDonut), referanstaki düz donut'ların
// yerini alıyor.

const FIZIKSEL_STOPS: ColorStop[] = [{ t: 0, hex: "#22d3ee" }, { t: 0.5, hex: "#2f6fed" }, { t: 1, hex: "#6d5ef8" }];
const TASERON_STOPS: ColorStop[] = [{ t: 0, hex: "#60a5fa" }, { t: 1, hex: "#2f6fed" }];
const MALZEME_STOPS: ColorStop[] = [{ t: 0, hex: "#fbbf24" }, { t: 1, hex: "#f59e0b" }];
const DIGER_STOPS: ColorStop[] = [{ t: 0, hex: "#9ca3af" }, { t: 1, hex: "#6b7280" }];
const ONAYLI_STOPS: ColorStop[] = [{ t: 0, hex: "#60a5fa" }, { t: 1, hex: "#3b82f6" }];
const TAMAMLANAN_STOPS: ColorStop[] = [{ t: 0, hex: "#4ade80" }, { t: 1, hex: "#22c55e" }];
const DEVAM_STOPS: ColorStop[] = [{ t: 0, hex: "#fbbf24" }, { t: 1, hex: "#f5a800" }];

type Milestone = {
  id: string;
  name: string;
  planned_date: string | null;
  actual_date: string | null;
  weight_pct: number | null;
  status: string;
  late: boolean;
};
type Activity = {
  entity: string;
  entity_id: string;
  from_status: string | null;
  to_status: string;
  actor: string | null;
  at: string;
};
type EVM = {
  bac: number;
  pv: number;
  ev: number;
  ac: number;
  spi: number;
  cpi: number;
  eac: number;
  etc: number;
  progress_pct: number;
  plan_source: string;
  s_curve: SCurvePoint[];
  as_of_month: string;
  contract_amount?: number;
  financial_progress_pct: number;
};
type Dash = {
  project: { id: string; code: string; name: string; status: string; currency: string };
  progress_pct: number;
  milestones: Milestone[];
  evm?: EVM;
  open_findings: {
    total: number;
    critical: number;
    major: number;
    minor: number;
    observation: number;
    overdue: number;
  };
  pending: { payments: number; mars: number; prs: number; open_pos: number; delivered_pos: number; overdue_pos: number; open_tasks: number };
  activity?: Activity[];
  cost_breakdown?: { tasaron: number; malzeme: number; diger: number };
  document_status?: { onayli: number; revizyon: number; taslak: number };
  cover_image?: { document_id: string; version: number };
  accident_free_days: { days: number; reference_date: string | null; since_accident: boolean; has_reference: boolean };
  subcontractor_scoped: boolean;
};
type PVEntry = { month: string; planned_pct: number };

export default function Dashboard() {
  const { user, can } = useAuth();
  const { current, loading: projLoading } = useProjects();
  const [dash, setDash] = useState<Dash | null>(null);
  const [err, setErr] = useState<string | null>(null);
  const [showPV, setShowPV] = useState(false);
  const [addingAccident, setAddingAccident] = useState(false);
  const [accidentDate, setAccidentDate] = useState(new Date().toISOString().slice(0, 10));
  const [accidentDesc, setAccidentDesc] = useState("");
  const [accidentBusy, setAccidentBusy] = useState(false);

  const load = useCallback(async () => {
    if (!current) return;
    setErr(null);
    try {
      const res = await api<{ dashboard: Dash }>(`/projects/${current.id}/dashboard`);
      setDash(res.dashboard);
    } catch {
      setErr("Dashboard verisi yüklenemedi.");
    }
  }, [current]);

  useEffect(() => {
    setDash(null);
    load();
  }, [load]);

  async function saveAccident() {
    if (!current || !accidentDesc.trim()) return;
    setAccidentBusy(true);
    try {
      await api(`/projects/${current.id}/ohs-accidents`, {
        method: "POST", projectId: current.id,
        body: { accident_date: accidentDate, description: accidentDesc.trim() },
      });
      setAddingAccident(false);
      setAccidentDesc("");
      await load();
    } finally {
      setAccidentBusy(false);
    }
  }

  if (projLoading) return <p className="text-beton-400 text-sm">Yükleniyor…</p>;
  if (!current)
    return (
      <div>
        <h1 className="font-display text-2xl font-medium text-beton-100 tracking-tight">
          Hoş geldiniz, {user?.full_name}
        </h1>
        <p className="mt-2 text-sm text-beton-400">
          Henüz bir projeye üye değilsiniz. Yöneticinizden proje ataması isteyin.
        </p>
      </div>
    );

  const cur = dash?.project.currency ?? current.currency;
  const money = (v: number) => v.toLocaleString("tr-TR", { maximumFractionDigits: 2 }) + " " + cur;
  const moneyShort = (v: number) =>
    (v >= 1_000_000 ? (v / 1_000_000).toFixed(2) + "M" : v.toLocaleString("tr-TR", { maximumFractionDigits: 0 })) + " " + cur;

  return (
    <div>
      {err && <p className="text-sm text-red-400">{err}</p>}
      {!dash && !err && <p className="text-sm text-beton-400">Yükleniyor…</p>}

      {dash && (
        <div
          className="rounded-xl overflow-hidden border"
          style={{ background: "rgb(var(--panel-bg))", borderColor: "rgb(var(--panel-hairline))" }}
        >
          {/* ── Head ── */}
          <div
            className="flex items-start justify-between gap-4 flex-wrap px-5 py-4 border-b"
            style={{ borderColor: "rgb(var(--panel-hairline))" }}
          >
            <div>
              <p className="text-[10px] font-bold uppercase tracking-[.1em]" style={{ color: "var(--group-accent)" }}>
                Proje kontrol paneli
              </p>
              <h1 className="font-display text-xl font-extrabold mt-1.5" style={{ color: "rgb(var(--panel-ink))" }}>
                {current.code} — {current.name}
              </h1>
              <p className="mt-1 text-xs" style={{ color: "rgb(var(--panel-ink2))" }}>
                Durum: {dash.project.status} · Üstyapı Projesi
                {dash.subcontractor_scoped && " · yalnızca kendi firmanızın verisi"}
              </p>
            </div>
            <div className="text-right">
              <p className="text-[10.5px] mb-1.5" style={{ color: "rgb(var(--panel-ink3))" }}>
                Son güncelleme: {new Date().toLocaleString("tr-TR")}
              </p>
              <button
                onClick={load}
                className="rounded-md px-3.5 py-1.5 text-xs font-bold transition hover:brightness-110"
                style={{ background: "var(--group-accent)", color: "#141414" }}
              >
                Yenile
              </button>
            </div>
          </div>

          {/* ── KPI şeridi ── */}
          <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-6" style={{ borderBottom: "1px solid rgb(var(--panel-hairline))" }}>
            <PanelKpiRing
              label="Fiziksel İlerleme"
              pct={dash.progress_pct}
              colorStops={FIZIKSEL_STOPS}
              hint={dash.evm && dash.evm.bac > 0
                ? `${dash.progress_pct >= (dash.evm.pv / dash.evm.bac) * 100 ? "▲" : "▼"} Plan %${((dash.evm.pv / dash.evm.bac) * 100).toFixed(1)}`
                : undefined}
              bad={!!dash.evm && dash.evm.bac > 0 && dash.progress_pct < (dash.evm.pv / dash.evm.bac) * 100}
            />
            {dash.evm ? (
              <PanelKpi
                label="Zaman Performansı"
                value={idx(dash.evm.spi)}
                hint={spiHint(dash.evm.spi)}
                bad={dash.evm.spi > 0 && dash.evm.spi < 0.9}
                good={dash.evm.spi >= 1.1}
                icon={<IconTrend />}
              />
            ) : (
              <PanelKpi label="Zaman Performansı" value="—" icon={<IconTrend />} />
            )}
            {dash.evm ? (
              <PanelKpi
                label="Maliyet Performansı"
                value={idx(dash.evm.cpi)}
                hint={cpiHint(dash.evm.cpi)}
                bad={dash.evm.cpi > 0 && dash.evm.cpi < 0.9}
                good={dash.evm.cpi >= 1.1}
                icon={<IconCheck />}
              />
            ) : (
              <PanelKpi label="Maliyet Performansı" value="—" icon={<IconCheck />} />
            )}
            <PanelKpi
              label="Kazasız Gün"
              value={dash.accident_free_days.has_reference ? String(dash.accident_free_days.days) : "—"}
              icon={<IconShield />}
              hint={can("ohs.perform_inspection") ? undefined : "Son kazadan bu yana"}
              action={can("ohs.perform_inspection") && (
                <button
                  onClick={() => setAddingAccident((v) => !v)}
                  className="text-[10px] hover:underline"
                  style={{ color: "var(--group-accent)" }}
                >
                  {addingAccident ? "vazgeç" : "+ kaza kaydı ekle"}
                </button>
              )}
            />
            <PanelKpi
              label="Açık İSG Bulgusu"
              value={String(dash.open_findings.total)}
              icon={<IconWarning />}
              bad={dash.open_findings.critical > 0}
              hint={dash.open_findings.critical > 0 ? `${dash.open_findings.critical} kritik` : undefined}
            />
            {dash.evm ? (
              <PanelKpi
                label="Toplam Harcama"
                value={moneyShort(dash.evm.ac)}
                hint={`Bütçe ${moneyShort(dash.evm.bac)}`}
                icon={<IconReceipt />}
              />
            ) : (
              <PanelKpi label="Toplam Harcama" value="—" icon={<IconReceipt />} />
            )}
          </div>

          {addingAccident && (
            <div className="px-5 py-3 border-b" style={{ borderColor: "rgb(var(--panel-hairline))" }}>
              <div className="flex flex-wrap items-end gap-2">
                <input type="date" value={accidentDate} onChange={(e) => setAccidentDate(e.target.value)}
                  max={new Date().toISOString().slice(0, 10)}
                  className="rounded-md px-2 py-1.5 text-sm outline-none"
                  style={panelInputStyle} />
                <input value={accidentDesc} onChange={(e) => setAccidentDesc(e.target.value)} placeholder="Kısa açıklama"
                  className="flex-1 min-w-[160px] rounded-md px-2 py-1.5 text-sm outline-none"
                  style={panelInputStyle} />
                <button onClick={saveAccident} disabled={accidentBusy || !accidentDesc.trim()}
                  className="rounded-md bg-red-500/90 hover:bg-red-500 disabled:opacity-50 text-white-solid text-xs font-semibold px-3 py-1.5">
                  {accidentBusy ? "Kaydediliyor…" : "Kaza kaydını gir"}
                </button>
              </div>
            </div>
          )}

          {/* ── Row A: S-eğrisi + Proje Görseli ── */}
          <PanelRow cols={dash.evm ? "1.9fr 1fr" : "1fr"}>
            {dash.evm && (
              <PanelCell
                title={`Fiziksel İlerleme Trendi (S-Eğrisi · ${dash.evm.as_of_month})`}
                action={
                  <Can perm="projects.edit">
                    <button onClick={() => setShowPV((s) => !s)} className="text-[11px] font-semibold hover:underline" style={{ color: "var(--group-accent)" }}>
                      {showPV ? "PV planını gizle" : "PV planını düzenle"}
                    </button>
                  </Can>
                }
              >
                <div className="grid grid-cols-2 gap-3 mb-3">
                  <PanelStat label="EAC" value={money(dash.evm.eac)} />
                  <PanelStat label="ETC" value={money(dash.evm.etc)} />
                </div>
                {dash.evm.s_curve.length >= 2 ? (
                  <SCurve points={dash.evm.s_curve} currency={cur} asOf={dash.evm.as_of_month} />
                ) : (
                  <div className="rounded-lg border border-dashed px-4 py-8 text-center" style={{ borderColor: "rgb(var(--panel-hairline))" }}>
                    <p className="text-sm" style={{ color: "rgb(var(--panel-ink2))" }}>S-eğrisi için henüz yeterli veri yok.</p>
                    <p className="mt-1 text-xs" style={{ color: "rgb(var(--panel-ink3))" }}>
                      Grafik, en az iki dönem hakediş/ilerleme kaydı girildiğinde görünür.
                    </p>
                  </div>
                )}
                <p className="mt-2 text-[11px]" style={{ color: "rgb(var(--panel-ink3))" }}>
                  PV kaynağı: {planSourceTR(dash.evm.plan_source)} · BAC {money(dash.evm.bac)}
                </p>
                {showPV && <PVEditor projectId={current.id} onSaved={load} />}
              </PanelCell>
            )}
            <PanelCell title="Proje Görseli" noPad>
              <CoverImageCard projectId={current.id} coverImage={dash.cover_image} canUpload={can("documents.upload")} onChanged={load} />
            </PanelCell>
          </PanelRow>

          {/* ── Row B: Satınalma Özeti + Maliyet Durumu ── */}
          {!dash.subcontractor_scoped && (
            <PanelRow cols={dash.cost_breakdown ? "1fr 1fr" : "1fr"}>
              <PanelCell
                title="Satınalma Özeti"
                action={<Link to="/satinalma" className="text-[11px] font-semibold hover:underline" style={{ color: "var(--group-accent)" }}>Detaylı gör →</Link>}
              >
                <div className="flex items-center gap-5">
                  <SegmentedDonut
                    size={92}
                    centerLabel="AÇIK PO"
                    segments={[
                      { label: "Tamamlanan", value: dash.pending.delivered_pos, colorStops: TAMAMLANAN_STOPS, swatch: "#22c55e" },
                      { label: "Devam Eden", value: dash.pending.open_pos, colorStops: DEVAM_STOPS, swatch: "#f5a800" },
                    ]}
                  />
                  <div className="space-y-2 text-[12.5px] flex-1">
                    <PanelLine label="Açık Talep (PR)" value={dash.pending.prs} />
                    <PanelLine label="Açık Sipariş (PO)" value={dash.pending.open_pos} />
                    <PanelLine label="Geciken Sipariş" value={dash.pending.overdue_pos} bad={dash.pending.overdue_pos > 0} />
                  </div>
                </div>
              </PanelCell>
              {dash.cost_breakdown && (
                <PanelCell title="Maliyet Durumu">
                  <SegmentedDonut
                    size={92}
                    formatValue={(v) => v.toLocaleString("tr-TR")}
                    centerLabel="TOPLAM"
                    segments={[
                      { label: "Taşeron", value: dash.cost_breakdown.tasaron, colorStops: TASERON_STOPS, swatch: "#2f6fed" },
                      { label: "Malzeme", value: dash.cost_breakdown.malzeme, colorStops: MALZEME_STOPS, swatch: "#f59e0b" },
                      { label: "Diğer", value: dash.cost_breakdown.diger, colorStops: DIGER_STOPS, swatch: "#8b93a3" },
                    ] as DonutSegment[]}
                  />
                </PanelCell>
              )}
            </PanelRow>
          )}

          {/* ── Row C: Açık İSG + Doküman Durumu + Bekleyen İşler ── */}
          <PanelRow cols={dash.document_status ? "1fr 1fr 1fr" : "1fr 1fr"}>
            <PanelCell title="Açık İSG Bulguları">
              <div className="flex flex-wrap gap-2">
                <PanelBadge label={`Toplam ${dash.open_findings.total}`} tone="muted" />
                <PanelBadge label={`Kritik ${dash.open_findings.critical}`} tone={dash.open_findings.critical ? "red" : "muted"} />
                <PanelBadge label={`Majör ${dash.open_findings.major}`} tone={dash.open_findings.major ? "amber" : "muted"} />
                <PanelBadge label={`Minör ${dash.open_findings.minor}`} tone="muted" />
                <PanelBadge label={`Gözlem ${dash.open_findings.observation}`} tone="muted" />
                <PanelBadge label={`Termini geçen ${dash.open_findings.overdue}`} tone={dash.open_findings.overdue ? "red" : "muted"} />
              </div>
            </PanelCell>
            {dash.document_status && (
              <PanelCell title="Doküman Durumu">
                <div className="flex flex-wrap gap-2">
                  <PanelBadge label={`Onaylı ${dash.document_status.onayli}`} tone="blue" />
                  <PanelBadge label={`Revizyon ${dash.document_status.revizyon}`} tone={dash.document_status.revizyon ? "amber" : "muted"} />
                  <PanelBadge label={`Taslak ${dash.document_status.taslak}`} tone="muted" />
                </div>
              </PanelCell>
            )}
            <PanelCell title="Bekleyen İşler">
              <ul className="text-[13px] space-y-1.5" style={{ color: "rgb(var(--panel-ink))" }}>
                <li>Bekleyen hakediş: <b>{dash.pending.payments}</b></li>
                {!dash.subcontractor_scoped && (
                  <>
                    <li>Bekleyen MAR: <b>{dash.pending.mars}</b></li>
                    <li>Açık görev: <b>{dash.pending.open_tasks}</b></li>
                  </>
                )}
              </ul>
            </PanelCell>
          </PanelRow>

          {/* ── Row D: Nakit Akışı + Milestone ── */}
          <PanelRow cols="1.9fr 1fr">
            <Can perm="reports.view_financial_reports" fallback={<div />}>
              <NakitAkisCard projectId={current.id} currency={cur} />
            </Can>
            <PanelCell title="Yaklaşan Milestone'lar">
              {dash.milestones.length === 0 ? (
                <p className="text-sm" style={{ color: "rgb(var(--panel-ink2))" }}>Tanımlı milestone yok.</p>
              ) : (
                <ul className="flex flex-col">
                  {dash.milestones.map((m, i) => (
                    <li
                      key={m.id}
                      className="flex items-center gap-3 text-[12.5px] py-2"
                      style={i < dash.milestones.length - 1 ? { borderBottom: "1px solid rgb(var(--panel-hairline))" } : undefined}
                    >
                      <span
                        className="inline-block w-2.5 h-2.5 rounded-full shrink-0"
                        style={{ background: m.status === "Completed" ? "var(--group-accent)" : m.late ? "#f87171" : "rgb(var(--panel-hairline))" }}
                      />
                      <span className="flex-1" style={{ color: "rgb(var(--panel-ink))" }}>{m.name}</span>
                      <span className={"font-mono text-[10.5px] " + (m.late ? "" : "")} style={{ color: m.late ? "#f87171" : "rgb(var(--panel-ink3))" }}>
                        {m.late ? "GECİKMİŞ" : (m.actual_date ?? m.planned_date ?? "—")}
                      </span>
                    </li>
                  ))}
                </ul>
              )}
            </PanelCell>
          </PanelRow>

          {/* ── Aktivite akışı — taşerona dönmez ── */}
          {dash.activity && (
            <PanelRow cols="1fr">
              <PanelCell title="Son Aktivite (İş Akışı Geçişleri)">
                {dash.activity.length === 0 ? (
                  <p className="text-sm" style={{ color: "rgb(var(--panel-ink2))" }}>Henüz aktivite yok.</p>
                ) : (
                  <ul className="space-y-1.5">
                    {dash.activity.map((a, i) => (
                      <li key={i} className="text-xs font-mono" style={{ color: "rgb(var(--panel-ink2))" }}>
                        <span style={{ color: "rgb(var(--panel-ink3))" }}>{new Date(a.at).toLocaleString("tr-TR")}</span>{" "}
                        <span style={{ color: "rgb(var(--panel-ink))" }}>{entityTR(a.entity)}</span>{" "}
                        {a.from_status ? `${a.from_status} → ` : ""}
                        <span style={{ color: "var(--group-accent)" }}>{a.to_status}</span>
                        {a.actor && <span style={{ color: "rgb(var(--panel-ink3))" }}> · {a.actor}</span>}
                      </li>
                    ))}
                  </ul>
                )}
              </PanelCell>
            </PanelRow>
          )}
        </div>
      )}
    </div>
  );
}

const panelInputStyle: React.CSSProperties = {
  background: "rgb(var(--panel-hairline) / .4)",
  border: "1px solid rgb(var(--panel-hairline))",
  color: "rgb(var(--panel-ink))",
};

// PV aylık dağılım editörü (projects.edit) — toplam 100 olmalı.
function PVEditor({ projectId, onSaved }: { projectId: string; onSaved: () => void }) {
  const [entries, setEntries] = useState<PVEntry[]>([]);
  const [msg, setMsg] = useState<string | null>(null);

  useEffect(() => {
    api<{ entries: PVEntry[] }>(`/projects/${projectId}/pv-plan`)
      .then((r) => setEntries(r.entries))
      .catch(() => setMsg("PV planı yüklenemedi."));
  }, [projectId]);

  const sum = entries.reduce((s, e) => s + (Number(e.planned_pct) || 0), 0);

  async function save() {
    setMsg(null);
    try {
      await api(`/projects/${projectId}/pv-plan`, {
        method: "PUT",
        body: { entries: entries.map((e) => ({ month: e.month, planned_pct: Number(e.planned_pct) })) },
      });
      setMsg("Kaydedildi.");
      onSaved();
    } catch {
      setMsg("Kaydedilemedi — dağılım toplamı 100 (±0.5) ve aylar YYYY-MM olmalı.");
    }
  }

  return (
    <div className="mt-4 border-t pt-3" style={{ borderColor: "rgb(var(--panel-hairline))" }}>
      <p className="text-xs mb-2" style={{ color: "rgb(var(--panel-ink2))" }}>
        PV aylık dağılımı (dönemsel %). Boş bırakılırsa S-eğrisi milestone
        ağırlıklarından, o da yoksa doğrusal dağılımdan türetilir.
      </p>
      <div className="space-y-1.5">
        {entries.map((e, i) => (
          <div key={i} className="flex gap-2 items-center">
            <input
              value={e.month}
              onChange={(ev) => setEntries(entries.map((x, j) => (j === i ? { ...x, month: ev.target.value } : x)))}
              placeholder="YYYY-MM"
              className="w-28 rounded px-2 py-1 text-xs font-mono"
              style={panelInputStyle}
            />
            <input
              type="number"
              step="0.001"
              value={e.planned_pct}
              onChange={(ev) =>
                setEntries(entries.map((x, j) => (j === i ? { ...x, planned_pct: Number(ev.target.value) } : x)))
              }
              className="w-24 rounded px-2 py-1 text-xs font-mono"
              style={panelInputStyle}
            />
            <span className="text-xs" style={{ color: "rgb(var(--panel-ink3))" }}>%</span>
            <button
              onClick={() => setEntries(entries.filter((_, j) => j !== i))}
              className="text-xs text-red-400 hover:underline"
            >
              sil
            </button>
          </div>
        ))}
      </div>
      <div className="mt-2 flex items-center gap-3">
        <button
          onClick={() => setEntries([...entries, { month: "", planned_pct: 0 }])}
          className="text-xs hover:underline"
          style={{ color: "var(--group-accent)" }}
        >
          + ay ekle
        </button>
        <span className={"font-mono text-xs " + (Math.abs(sum - 100) <= 0.5 || entries.length === 0 ? "" : "text-red-400")}
          style={Math.abs(sum - 100) <= 0.5 || entries.length === 0 ? { color: "rgb(var(--panel-ink2))" } : undefined}>
          toplam %{sum.toFixed(3)}
        </span>
        <button
          onClick={save}
          className="ml-auto rounded-md px-3 py-1 text-xs font-semibold transition hover:brightness-110"
          style={{ background: "var(--group-accent)", color: "#141414" }}
        >
          Kaydet
        </button>
      </div>
      {msg && <p className="mt-1 text-xs" style={{ color: "rgb(var(--panel-ink2))" }}>{msg}</p>}
    </div>
  );
}

// Nakit Akış özet kartı (Faz F) — son 30 gün + gelecek 30 gün toplu
// giriş/çıkış/net. Detaylı görünüm için /nakit-akis'e yönlendirir.
type CashFlowSummary = { total_in: number; total_out: number; net: number };

function NakitAkisCard({ projectId, currency }: { projectId: string; currency: string }) {
  const [summary, setSummary] = useState<CashFlowSummary | null>(null);
  const [err, setErr] = useState(false);

  useEffect(() => {
    const from = new Date(); from.setDate(from.getDate() - 30);
    const to = new Date(); to.setDate(to.getDate() + 30);
    const q = `from=${from.toISOString().slice(0, 10)}&to=${to.toISOString().slice(0, 10)}&group=monthly`;
    api<{ summary: CashFlowSummary }>(`/projects/${projectId}/cash-flow?${q}`)
      .then((r) => setSummary(r.summary))
      .catch(() => setErr(true));
  }, [projectId]);

  const money = (v: number) => v.toLocaleString("tr-TR", { maximumFractionDigits: 0 }) + " " + currency;

  return (
    <PanelCell
      title="Nakit Akışı (son 30 gün + gelecek 30 gün)"
      action={<Link to="/nakit-akis" className="text-[11px] font-semibold hover:underline" style={{ color: "var(--group-accent)" }}>Detaylı gör →</Link>}
    >
      {err && <p className="text-sm" style={{ color: "rgb(var(--panel-ink2))" }}>Nakit akış verisi yüklenemedi.</p>}
      {!err && !summary && <p className="text-sm" style={{ color: "rgb(var(--panel-ink2))" }}>Yükleniyor…</p>}
      {summary && (
        <div className="grid grid-cols-3 gap-3">
          <div>
            <p className="text-[10.5px] uppercase tracking-wider" style={{ color: "rgb(var(--panel-ink3))" }}>Giriş</p>
            <p className="mt-1 font-display text-lg font-bold" style={{ fontVariantNumeric: "tabular-nums", color: "var(--group-accent)" }}>
              {money(summary.total_in)}
            </p>
          </div>
          <div>
            <p className="text-[10.5px] uppercase tracking-wider" style={{ color: "rgb(var(--panel-ink3))" }}>Çıkış</p>
            <p className="mt-1 font-display text-lg font-bold text-red-400" style={{ fontVariantNumeric: "tabular-nums" }}>
              {money(summary.total_out)}
            </p>
          </div>
          <div>
            <p className="text-[10.5px] uppercase tracking-wider" style={{ color: "rgb(var(--panel-ink3))" }}>Net</p>
            <p
              className={"mt-1 font-display text-lg font-bold " + (summary.net >= 0 ? "" : "text-red-400")}
              style={{ fontVariantNumeric: "tabular-nums", color: summary.net >= 0 ? "rgb(var(--panel-ink))" : undefined }}
            >
              {money(summary.net)}
            </p>
          </div>
        </div>
      )}
    </PanelCell>
  );
}

// Proje görseli (Dashboard v2) — mevcut polimorfik documents motoru
// (doc_category="ProjeGorseli") üzerinden; FotograflarPage.tsx'teki
// iki adımlı yükleme deseniyle aynı (create → multipart version upload).
function CoverImageCard({ projectId, coverImage, canUpload, onChanged }: {
  projectId: string;
  coverImage?: { document_id: string; version: number };
  canUpload: boolean;
  onChanged: () => void;
}) {
  const [url, setUrl] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const fileRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (!coverImage) { setUrl(null); return; }
    apiFetchBlob(`/projects/${projectId}/documents/${coverImage.document_id}/versions/${coverImage.version}/download`)
      .then(setUrl)
      .catch(() => setUrl(null));
  }, [projectId, coverImage]);

  async function upload(files: FileList | null) {
    const file = files?.[0];
    if (!file) return;
    setBusy(true);
    try {
      const doc = await api<{ document: { id: string } }>(`/projects/${projectId}/documents`, {
        method: "POST", projectId,
        body: { title: "Proje Görseli", doc_category: "ProjeGorseli", entity_type: "project", entity_id: projectId },
      });
      const fd = new FormData();
      fd.append("file", file);
      await apiUpload(`/projects/${projectId}/documents/${doc.document.id}/versions`, fd);
      onChanged();
    } finally {
      setBusy(false);
      if (fileRef.current) fileRef.current.value = "";
    }
  }

  return (
    <div className="relative h-full min-h-[220px]">
      <input ref={fileRef} type="file" accept="image/*" className="hidden" onChange={(e) => upload(e.target.files)} />
      {url ? (
        <img src={url} alt="Proje görseli" className="w-full h-full min-h-[220px] object-cover" />
      ) : (
        <div className="h-full min-h-[220px] flex items-center justify-center" style={{ background: "rgb(var(--panel-hairline) / .3)" }}>
          <p className="text-sm" style={{ color: "rgb(var(--panel-ink3))" }}>Henüz proje görseli eklenmedi.</p>
        </div>
      )}
      {canUpload && (
        <button onClick={() => fileRef.current?.click()} disabled={busy}
          className="absolute top-2 right-2 rounded-md bg-black/50 px-2.5 py-1 text-[11px] font-semibold text-white-solid hover:bg-black/70 disabled:opacity-50 backdrop-blur-sm">
          {busy ? "Yükleniyor…" : url ? "Değiştir" : "Yükle"}
        </button>
      )}
    </div>
  );
}

function idx(v: number) {
  return v === 0 ? "—" : v.toFixed(3);
}
function spiHint(v: number): string {
  if (v >= 1.1) return "Planlanandan önde";
  if (v >= 0.9) return "Planda";
  return "Planlanandan geride";
}
function cpiHint(v: number): string {
  if (v >= 1.1) return "Bütçe altında";
  if (v >= 0.9) return "Bütçede";
  return "Bütçe üzerinde";
}
function planSourceTR(s: string) {
  if (s === "manual") return "aylık dağılım girişi";
  if (s === "milestones") return "milestone ağırlıkları";
  return "doğrusal dağılım";
}
function entityTR(e: string) {
  const map: Record<string, string> = {
    progress_payments: "Hakediş",
    material_approvals: "MAR",
    purchase_requests: "PR",
    purchase_orders: "PO",
    ohs_findings: "İSG bulgusu",
    daily_reports: "Günlük rapor",
  };
  return map[e] ?? e;
}

// ── Yerleşim ilkelleri (kesintisiz levha: gölge/gradyan yok, sadece hairline) ──

function PanelRow({ cols, children }: { cols: string; children: ReactNode }) {
  return (
    <div
      className="grid border-b last:border-b-0"
      style={{ gridTemplateColumns: cols, borderColor: "rgb(var(--panel-hairline))" }}
    >
      {children}
    </div>
  );
}

function PanelCell({ title, action, children, noPad }: { title: string; action?: ReactNode; children: ReactNode; noPad?: boolean }) {
  return (
    <div
      className={"flex flex-col border-r last:border-r-0 " + (noPad ? "" : "p-4")}
      style={{ borderColor: "rgb(var(--panel-hairline))" }}
    >
      <div className={"flex items-center justify-between mb-3" + (noPad ? " px-4 pt-4" : "")}>
        <h2 className="text-[11px] font-extrabold uppercase tracking-wide" style={{ color: "rgb(var(--panel-ink2))" }}>{title}</h2>
        {action}
      </div>
      <div className="flex-1 min-h-0">{children}</div>
    </div>
  );
}

function PanelStat({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <p className="text-[10px] uppercase tracking-wider" style={{ color: "rgb(var(--panel-ink3))" }}>{label}</p>
      <p className="mt-0.5 font-display text-base font-bold" style={{ color: "rgb(var(--panel-ink))", fontVariantNumeric: "tabular-nums" }}>{value}</p>
    </div>
  );
}

function PanelLine({ label, value, bad }: { label: string; value: number; bad?: boolean }) {
  return (
    <div className="flex items-center gap-2">
      <span className="flex-1" style={{ color: "rgb(var(--panel-ink2))" }}>{label}</span>
      <span className="font-mono font-bold" style={{ color: bad ? "#f87171" : "rgb(var(--panel-ink))" }}>{value}</span>
    </div>
  );
}

function PanelBadge({ label, tone }: { label: string; tone: "red" | "amber" | "blue" | "muted" }) {
  const style =
    tone === "red" ? { background: "rgba(248,113,113,.15)", color: "#f87171" }
    : tone === "amber" ? { background: "rgba(245,168,0,.15)", color: "var(--group-accent)" }
    : tone === "blue" ? { background: "rgba(96,165,250,.15)", color: "#60a5fa" }
    : { background: "rgb(var(--panel-hairline) / .6)", color: "rgb(var(--panel-ink2))" };
  return <span className="font-mono text-xs px-2 py-1 rounded" style={style}>{label}</span>;
}

function PanelKpi({ label, value, hint, bad, good, icon, action }: {
  label: string; value: string; hint?: string; bad?: boolean; good?: boolean; icon: ReactNode; action?: ReactNode;
}) {
  const iconTone = bad ? "#f87171" : good ? "#4ade80" : "rgb(180,186,196)";
  return (
    <div className="flex items-center gap-3 p-3.5 border-r last:border-r-0" style={{ borderColor: "rgb(var(--panel-hairline))" }}>
      <div className="w-9 h-9 rounded-lg grid place-items-center shrink-0" style={{ background: `${iconTone}24` }}>
        <span style={{ color: iconTone, width: 17, height: 17, display: "block" }}>{icon}</span>
      </div>
      <div className="min-w-0 flex-1">
        <p className="text-[9px] uppercase tracking-wide font-bold truncate" style={{ color: "rgb(var(--panel-ink3))" }}>{label}</p>
        <p className="text-[17px] font-extrabold leading-tight" style={{ color: "rgb(var(--panel-ink))", fontVariantNumeric: "tabular-nums" }}>{value}</p>
        {hint && <p className="text-[10px] mt-0.5" style={{ color: bad ? "#f87171" : good ? "#4ade80" : "rgb(var(--panel-ink2))" }}>{hint}</p>}
        {action}
      </div>
    </div>
  );
}

function PanelKpiRing({ label, pct, colorStops, hint, bad }: {
  label: string; pct: number; colorStops: ColorStop[]; hint?: string; bad?: boolean;
}) {
  return (
    <div className="flex items-center gap-3 p-3.5 border-r last:border-r-0" style={{ borderColor: "rgb(var(--panel-hairline))" }}>
      <RadialRing pct={pct} colorStops={colorStops} size={44} />
      <div className="min-w-0 flex-1">
        <p className="text-[9px] uppercase tracking-wide font-bold truncate" style={{ color: "rgb(var(--panel-ink3))" }}>{label}</p>
        <p className="text-[16px] font-extrabold leading-tight" style={{ color: "rgb(var(--panel-ink))", fontVariantNumeric: "tabular-nums" }}>%{pct.toFixed(1)}</p>
        {hint && <p className="text-[10px] mt-0.5" style={{ color: bad ? "#f87171" : "rgb(var(--panel-ink2))" }}>{hint}</p>}
      </div>
    </div>
  );
}

// ── İkonlar (satır ikonları, emoji değil) ──
function IconTrend() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round" width="100%" height="100%">
      <path d="M3 17l6-6 4 4 8-9" /><path d="M15 6h6v6" />
    </svg>
  );
}
function IconCheck() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round" width="100%" height="100%">
      <circle cx="12" cy="12" r="9" /><path d="M9 12l2 2 4-4" />
    </svg>
  );
}
function IconShield() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round" width="100%" height="100%">
      <path d="M12 3l7 3v6c0 4.5-3 7.5-7 9-4-1.5-7-4.5-7-9V6l7-3z" />
    </svg>
  );
}
function IconWarning() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round" width="100%" height="100%">
      <path d="M12 3L2 20h20L12 3z" /><path d="M12 10v4" /><circle cx="12" cy="17" r=".6" fill="currentColor" />
    </svg>
  );
}
function IconReceipt() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round" width="100%" height="100%">
      <rect x="3" y="7" width="18" height="13" rx="2" /><path d="M8 7V5a2 2 0 012-2h4a2 2 0 012 2v2" />
    </svg>
  );
}
