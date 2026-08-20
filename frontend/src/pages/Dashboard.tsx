import type { ReactNode } from "react";
import { useCallback, useEffect, useRef, useState } from "react";
import { Link } from "react-router-dom";
import { api, apiFetchBlob, apiUpload } from "../api/client";
import { useAuth } from "../auth/AuthContext";
import { Can } from "../auth/guards";
import { useProjects } from "../projects/ProjectContext";
import SCurve from "./dashboard/SCurve";
import type { SCurvePoint } from "./dashboard/SCurve";
import MultiDonut from "./dashboard/MultiDonut";
import RadialRing from "./dashboard/RadialRing";
import Gauge from "./dashboard/Gauge";

// Faz 9 — rol duyarlı proje dashboard'u. Finansal blok (EVM) backend'de izinle
// süzülür: reports.view_financial_reports yoksa `evm` alanı hiç gelmez; taşeron
// temsilcisi yalnızca kendi firmasının sayaçlarını görür (satır seviyesi güvenlik).

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

  return (
    <div>
      <div className="flex items-baseline justify-between flex-wrap gap-2">
        <div>
          <p className="flex items-center gap-2 text-xs font-medium text-emniyet-500">
            <span className="inline-block w-1.5 h-1.5 rounded-full bg-emniyet-500" />
            Proje kontrol paneli
          </p>
          <h1 className="font-display text-3xl font-medium text-beton-100 mt-2 tracking-tight">
            {current.code} — {current.name}
          </h1>
          {dash && (
            <p className="mt-1 text-sm text-beton-400">
              Durum: {dash.project.status} · Fiziki ilerleme %{dash.progress_pct.toFixed(1)}
              {dash.subcontractor_scoped && " · yalnızca kendi firmanızın verisi"}
            </p>
          )}
        </div>
        <button
          onClick={load}
          className="rounded-md bg-[var(--group-accent)] hover:brightness-110 px-3 py-1 text-sm text-white-solid font-medium transition"
        >
          Yenile
        </button>
      </div>

      {err && <p className="mt-4 text-sm text-red-400">{err}</p>}
      {!dash && !err && <p className="mt-4 text-sm text-beton-400">Yükleniyor…</p>}

      {dash && (
        <div className="mt-6 grid gap-4">
          {/* İlerleme göstergeleri: Fiziksel/Zamansal/Parasal İlerleme yüzde
              halkaları + altlarında SPI (fiziksel+zamansal ile aynı grupta,
              ikisinin karşılaştırması olduğu için) ve CPI (parasal ile aynı
              grupta, maliyetle ilgili olduğu için) ibre göstergeleri.
              Zamansal İlerleme yeni bir backend alanı GEREKTİRMEZ — mevcut
              EVM verisinden (PV/BAC, "planlanan % tamamlanma") türetilir. */}
          <div className="rounded-xl border border-beton-800 bg-beton-900 p-5" style={{ boxShadow: "var(--shadow)" }}>
            <div className="grid grid-cols-3 gap-3">
              <ProgressRing label="Fiziksel İlerleme" pct={dash.progress_pct} color="#2f6fed" />
              {dash.evm ? (
                <ProgressRing
                  label="Zamansal İlerleme"
                  pct={dash.evm.bac > 0 ? (dash.evm.pv / dash.evm.bac) * 100 : 0}
                  color="#f59e0b"
                  sub="PV/BAC"
                />
              ) : (
                <ProgressRing label="Zamansal İlerleme" pct={0} color="#f59e0b" empty />
              )}
              {dash.evm?.contract_amount ? (
                <ProgressRing label="Parasal İlerleme" pct={dash.evm.financial_progress_pct} color="#22c55e" />
              ) : (
                <ProgressRing label="Parasal İlerleme" pct={0} color="#22c55e" empty />
              )}
            </div>
            {dash.evm && (
              <div className="mt-3 grid grid-cols-3 gap-3">
                <div className="col-span-2 flex flex-col items-center gap-1 pt-2 border-t border-beton-800">
                  <Gauge value={dash.evm.spi} />
                  <p className="text-[11px] font-semibold uppercase tracking-wide text-beton-400">SPI — Zamansal Performans</p>
                </div>
                <div className="flex flex-col items-center gap-1 pt-2 border-t border-beton-800">
                  <Gauge value={dash.evm.cpi} />
                  <p className="text-[11px] font-semibold uppercase tracking-wide text-beton-400">CPI — Maliyet Performansı</p>
                </div>
              </div>
            )}
            {dash.evm && !dash.evm.contract_amount && (
              <p className="mt-3 text-xs text-beton-500">Ana sözleşme bedeli girilmemiş — parasal ilerleme için Ana Sözleşme sayfasından ekleyin.</p>
            )}
          </div>

          <div className="grid grid-cols-2 gap-3">
            <Kpi
              label="Kazasız Gün"
              value={dash.accident_free_days.has_reference ? String(dash.accident_free_days.days) : "—"}
              action={can("ohs.perform_inspection") && (
                <button onClick={() => setAddingAccident((v) => !v)} className="mt-1 text-[10px] text-emniyet-500 hover:underline">
                  {addingAccident ? "vazgeç" : "+ kaza kaydı ekle"}
                </button>
              )}
            />
            <Kpi label="Açık İSG" value={String(dash.open_findings.total)} bad={dash.open_findings.critical > 0} />
          </div>
          {addingAccident && (
            <div className="-mt-2 rounded-xl border border-beton-800 bg-beton-900 p-4" style={{ boxShadow: "var(--shadow)" }}>
              <div className="flex flex-wrap items-end gap-2">
                <input type="date" value={accidentDate} onChange={(e) => setAccidentDate(e.target.value)}
                  max={new Date().toISOString().slice(0, 10)}
                  className="rounded-md bg-beton-950 border border-beton-800 px-2 py-1.5 text-sm text-beton-100" />
                <input value={accidentDesc} onChange={(e) => setAccidentDesc(e.target.value)} placeholder="Kısa açıklama"
                  className="flex-1 min-w-[160px] rounded-md bg-beton-950 border border-beton-800 px-2 py-1.5 text-sm text-beton-100" />
                <button onClick={saveAccident} disabled={accidentBusy || !accidentDesc.trim()}
                  className="rounded-md bg-red-500/90 hover:bg-red-500 disabled:opacity-50 text-white-solid text-xs font-semibold px-3 py-1.5">
                  {accidentBusy ? "Kaydediliyor…" : "Kaza kaydını gir"}
                </button>
              </div>
            </div>
          )}

          {/* Row A: EVM trendi (geniş) + proje görseli (dar sütun, büyütülmüş) */}
          <div className={dash.evm ? "grid gap-4 lg:grid-cols-3" : "grid gap-4"}>
            {dash.evm && (
              <div className="lg:col-span-2">
                <Card
                  title={`EVM (kümülatif · ${dash.evm.as_of_month})`}
                  action={
                    <Can perm="projects.edit">
                      <button
                        onClick={() => setShowPV((s) => !s)}
                        className="text-xs text-emniyet-500 hover:underline"
                      >
                        {showPV ? "PV planını gizle" : "PV planını düzenle"}
                      </button>
                    </Can>
                  }
                >
                  <div className="grid grid-cols-2 gap-3">
                    <Kpi label="EAC" value={money(dash.evm.eac)} />
                    <Kpi label="ETC" value={money(dash.evm.etc)} />
                  </div>
                  <div
                    className="mt-3 grid grid-cols-3 gap-3 text-xs text-beton-400"
                    style={{ fontVariantNumeric: "tabular-nums" }}
                  >
                    <span>PV <span className="text-beton-200">{money(dash.evm.pv)}</span></span>
                    <span>EV <span className="text-beton-200">{money(dash.evm.ev)}</span></span>
                    <span>AC <span className="text-beton-200">{money(dash.evm.ac)}</span></span>
                  </div>
                  <div className="mt-4">
                    {dash.evm.s_curve.length >= 2 ? (
                      <SCurve points={dash.evm.s_curve} currency={cur} asOf={dash.evm.as_of_month} />
                    ) : (
                      <div className="rounded-lg border border-dashed border-beton-700 bg-beton-950 px-4 py-8 text-center">
                        <p className="text-sm text-beton-400">S-eğrisi için henüz yeterli veri yok.</p>
                        <p className="mt-1 text-xs text-beton-500">
                          Grafik, en az iki dönem hakediş/ilerleme kaydı girildiğinde görünür.
                        </p>
                      </div>
                    )}
                  </div>
                  <p className="mt-1 text-[11px] text-beton-500">
                    PV kaynağı: {planSourceTR(dash.evm.plan_source)} · BAC {money(dash.evm.bac)}
                  </p>
                  {showPV && <PVEditor projectId={current.id} onSaved={load} />}
                </Card>
              </div>
            )}
            <CoverImageCard projectId={current.id} coverImage={dash.cover_image} canUpload={can("documents.upload")} onChanged={load} />
          </div>

          {/* Row B: Satınalma özeti (solda liste, sağda tamamlanan/devam eden donut'u)
              + Maliyet Durumu — Row A ile aynı 2:1 grid ritmi (bütünleşik yerleşim). */}
          {!dash.subcontractor_scoped && (
            <div className={dash.cost_breakdown ? "grid gap-4 lg:grid-cols-3" : "grid gap-4"}>
              <div className={dash.cost_breakdown ? "lg:col-span-2" : ""}>
                <Card
                  title="Satınalma Özeti"
                  action={<Link to="/satinalma" className="text-xs text-emniyet-500 hover:underline">Detaylı gör →</Link>}
                >
                  <div className="grid grid-cols-2 gap-4 items-center">
                    <div className="space-y-2.5 text-sm">
                      <div className="flex items-center gap-2">
                        <span className="w-2 h-2 rounded-full shrink-0" style={{ background: "rgb(var(--emniyet-500))" }} />
                        <span className="text-beton-300 flex-1">Açık Talep (PR)</span>
                        <span className="font-mono font-semibold text-beton-100">{dash.pending.prs}</span>
                      </div>
                      <div className="flex items-center gap-2">
                        <span className="w-2 h-2 rounded-full shrink-0" style={{ background: "#f59e0b" }} />
                        <span className="text-beton-300 flex-1">Açık Sipariş (PO)</span>
                        <span className="font-mono font-semibold text-beton-100">{dash.pending.open_pos}</span>
                      </div>
                      <div className="flex items-center gap-2">
                        <span className="w-2 h-2 rounded-full shrink-0 bg-red-500" />
                        <span className="text-beton-300 flex-1">Geciken Sipariş</span>
                        <span className={"font-mono font-semibold " + (dash.pending.overdue_pos ? "text-red-400" : "text-beton-100")}>
                          {dash.pending.overdue_pos}
                        </span>
                      </div>
                    </div>
                    <MultiDonut
                      layout="row"
                      size={64}
                      segments={[
                        { label: "Tamamlanan", value: dash.pending.delivered_pos, color: "#22c55e" },
                        { label: "Devam Eden", value: dash.pending.open_pos, color: "rgb(var(--emniyet-500))" },
                      ]}
                    />
                  </div>
                </Card>
              </div>
              {dash.cost_breakdown && (
                <Card title="Maliyet Durumu">
                  <MultiDonut
                    layout="row"
                    size={54}
                    formatValue={(v) => v.toLocaleString("tr-TR")}
                    segments={[
                      { label: "Taşeron", value: dash.cost_breakdown.tasaron, color: "rgb(var(--emniyet-500))" },
                      { label: "Malzeme", value: dash.cost_breakdown.malzeme, color: "#f59e0b" },
                      { label: "Diğer", value: dash.cost_breakdown.diger, color: "rgb(var(--beton-500))" },
                    ]}
                  />
                </Card>
              )}
            </div>
          )}

          <div className={dash.document_status ? "grid gap-4 md:grid-cols-3" : "grid gap-4 md:grid-cols-2"}>
            {/* Açık İSG bulguları */}
            <Card title="Açık İSG bulguları">
              <div className="flex flex-wrap gap-2">
                <Badge label={`Toplam ${dash.open_findings.total}`} tone="muted" />
                <Badge label={`Kritik ${dash.open_findings.critical}`} tone={dash.open_findings.critical ? "red" : "muted"} />
                <Badge label={`Majör ${dash.open_findings.major}`} tone={dash.open_findings.major ? "amber" : "muted"} />
                <Badge label={`Minör ${dash.open_findings.minor}`} tone="muted" />
                <Badge label={`Gözlem ${dash.open_findings.observation}`} tone="muted" />
                <Badge label={`Termini geçen ${dash.open_findings.overdue}`} tone={dash.open_findings.overdue ? "red" : "muted"} />
              </div>
            </Card>

            {/* Doküman durumu (Dashboard v2) — rozet listesi, donut değil
                (Panel v2 önizlemesinde İSG bulguları ile aynı dile getirildi). */}
            {dash.document_status && (
              <Card title="Doküman Durumu">
                <div className="flex flex-wrap gap-2">
                  <Badge label={`Onaylı ${dash.document_status.onayli}`} tone="blue" />
                  <Badge label={`Revizyon ${dash.document_status.revizyon}`} tone={dash.document_status.revizyon ? "amber" : "muted"} />
                  <Badge label={`Taslak ${dash.document_status.taslak}`} tone="muted" />
                </div>
              </Card>
            )}

            {/* Bekleyen onaylar */}
            <Card title="Bekleyen işler">
              <ul className="text-sm text-beton-200 space-y-1">
                <li>Bekleyen hakediş: <b>{dash.pending.payments}</b></li>
                {!dash.subcontractor_scoped && (
                  <>
                    <li>Bekleyen MAR: <b>{dash.pending.mars}</b></li>
                    <li>Açık görev: <b>{dash.pending.open_tasks}</b></li>
                  </>
                )}
              </ul>
            </Card>
          </div>

          {/* Nakit akış + milestone — Row A/B ile aynı 2:1 grid ritmi (bütünleşik yerleşim) */}
          <div className="grid gap-4 lg:grid-cols-3">
            <Can perm="reports.view_financial_reports">
              <div className="lg:col-span-2">
                <NakitAkisCard projectId={current.id} currency={cur} />
              </div>
            </Can>
            <Card title="Milestone zaman çizelgesi">
              {dash.milestones.length === 0 ? (
                <p className="text-sm text-beton-400">Tanımlı milestone yok.</p>
              ) : (
                <ul className="space-y-2">
                  {dash.milestones.map((m) => (
                    <li key={m.id} className="flex items-center gap-3 text-sm">
                      <span
                        className={
                          "inline-block w-2.5 h-2.5 rounded-full " +
                          (m.status === "Completed"
                            ? "bg-emniyet-500"
                            : m.late
                              ? "bg-red-500"
                              : "bg-beton-600")
                        }
                      />
                      <span className="text-beton-100 flex-1">{m.name}</span>
                      <span className="font-mono text-xs text-beton-400">
                        plan {m.planned_date ?? "—"} · gerçek {m.actual_date ?? "—"}
                      </span>
                      <span className={"font-mono text-xs " + (m.late ? "text-red-400" : "text-beton-300")}>
                        {m.late ? "GECİKMİŞ" : m.status}
                      </span>
                    </li>
                  ))}
                </ul>
              )}
            </Card>
          </div>

          {/* Aktivite akışı — taşerona dönmez */}
          {dash.activity && (
            <Card title="Son aktivite (iş akışı geçişleri)">
              {dash.activity.length === 0 ? (
                <p className="text-sm text-beton-400">Henüz aktivite yok.</p>
              ) : (
                <ul className="space-y-1.5">
                  {dash.activity.map((a, i) => (
                    <li key={i} className="text-xs text-beton-300 font-mono">
                      <span className="text-beton-500">{new Date(a.at).toLocaleString("tr-TR")}</span>{" "}
                      <span className="text-beton-100">{entityTR(a.entity)}</span>{" "}
                      {a.from_status ? `${a.from_status} → ` : ""}
                      <span className="text-emniyet-500">{a.to_status}</span>
                      {a.actor && <span className="text-beton-400"> · {a.actor}</span>}
                    </li>
                  ))}
                </ul>
              )}
            </Card>
          )}
        </div>
      )}
    </div>
  );
}

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
    <div className="mt-4 border-t border-beton-800 pt-3">
      <p className="text-xs text-beton-400 mb-2">
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
              className="w-28 rounded bg-beton-950 border border-beton-800 px-2 py-1 text-xs text-beton-100 font-mono"
            />
            <input
              type="number"
              step="0.001"
              value={e.planned_pct}
              onChange={(ev) =>
                setEntries(entries.map((x, j) => (j === i ? { ...x, planned_pct: Number(ev.target.value) } : x)))
              }
              className="w-24 rounded bg-beton-950 border border-beton-800 px-2 py-1 text-xs text-beton-100 font-mono"
            />
            <span className="text-xs text-beton-500">%</span>
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
          className="text-xs text-emniyet-500 hover:underline"
        >
          + ay ekle
        </button>
        <span className={"font-mono text-xs " + (Math.abs(sum - 100) <= 0.5 || entries.length === 0 ? "text-beton-400" : "text-red-400")}>
          toplam %{sum.toFixed(3)}
        </span>
        <button
          onClick={save}
          className="ml-auto rounded-md bg-emniyet-500 px-3 py-1 text-xs font-semibold text-beton-950 hover:brightness-110 transition"
        >
          Kaydet
        </button>
      </div>
      {msg && <p className="mt-1 text-xs text-beton-300">{msg}</p>}
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
    <Card
      title="Nakit Akış (son 30 gün + gelecek 30 gün)"
      action={<Link to="/nakit-akis" className="text-xs text-emniyet-500 hover:underline">Detaylı gör →</Link>}
    >
      {err && <p className="text-sm text-beton-400">Nakit akış verisi yüklenemedi.</p>}
      {!err && !summary && <p className="text-sm text-beton-400">Yükleniyor…</p>}
      {summary && (
        <div className="grid grid-cols-3 gap-3">
          <div>
            <p className="text-[11px] uppercase tracking-wider text-beton-500">Giriş</p>
            <p className="mt-1 font-display text-lg font-medium text-emniyet-500" style={{ fontVariantNumeric: "tabular-nums" }}>
              {money(summary.total_in)}
            </p>
          </div>
          <div>
            <p className="text-[11px] uppercase tracking-wider text-beton-500">Çıkış</p>
            <p className="mt-1 font-display text-lg font-medium text-red-400" style={{ fontVariantNumeric: "tabular-nums" }}>
              {money(summary.total_out)}
            </p>
          </div>
          <div>
            <p className="text-[11px] uppercase tracking-wider text-beton-500">Net</p>
            <p className={"mt-1 font-display text-lg font-medium " + (summary.net >= 0 ? "text-beton-100" : "text-red-400")} style={{ fontVariantNumeric: "tabular-nums" }}>
              {money(summary.net)}
            </p>
          </div>
        </div>
      )}
    </Card>
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
    <Card
      title="Proje Görseli"
      action={canUpload && (
        <button onClick={() => fileRef.current?.click()} disabled={busy}
          className="text-xs text-emniyet-500 hover:underline disabled:opacity-50">
          {busy ? "Yükleniyor…" : url ? "Değiştir" : "Yükle"}
        </button>
      )}
    >
      <input ref={fileRef} type="file" accept="image/*" className="hidden" onChange={(e) => upload(e.target.files)} />
      {url ? (
        <img src={url} alt="Proje görseli" className="w-full h-full min-h-[280px] object-cover rounded-lg" />
      ) : (
        <div className="h-full min-h-[280px] rounded-lg border border-dashed border-beton-700 bg-beton-950 flex items-center justify-center">
          <p className="text-sm text-beton-500">Henüz proje görseli eklenmedi.</p>
        </div>
      )}
    </Card>
  );
}

function idx(v: number) {
  return v === 0 ? "—" : v.toFixed(3);
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

function Card({ title, action, children }: { title: string; action?: ReactNode; children: ReactNode }) {
  return (
    <div className="h-full flex flex-col rounded-xl border border-beton-800 bg-beton-900 p-5" style={{ boxShadow: "var(--shadow)" }}>
      <div className="flex items-center justify-between">
        <h2 className="text-sm font-medium text-beton-100">{title}</h2>
        {action}
      </div>
      <div className="mt-3 flex-1 min-h-0">{children}</div>
    </div>
  );
}
// ProgressRing — İlerleme Göstergeleri kartındaki tek bir yüzde halkası +
// etiket. `empty` (ör. ana sözleşme bedeli girilmemiş) iken halka soluk
// gösterilir ve "Veri yok" yazar — Kpi bileşenindeki "—" deseniyle tutarlı.
function ProgressRing({ label, pct, color, sub, empty }: {
  label: string; pct: number; color: string; sub?: string; empty?: boolean;
}) {
  return (
    <div className="flex flex-col items-center gap-1.5 text-center">
      <RadialRing pct={pct} color={empty ? "rgb(var(--beton-700))" : color} />
      <p className="text-[11px] font-semibold uppercase tracking-wide text-beton-400">{label}</p>
      {empty ? (
        <p className="text-[10.5px] text-beton-500">Veri yok</p>
      ) : sub ? (
        <p className="text-[10.5px] text-beton-500">{sub}</p>
      ) : null}
    </div>
  );
}
function Kpi({ label, value, bad, action }: { label: string; value: string; bad?: boolean; action?: ReactNode }) {
  // "—" değeri (henüz hesaplanamayan endeks) soluk ve küçük gösterilir ki
  // gerçek rakamlarla görsel olarak karışmasın.
  const empty = value === "—" || value.trim() === "";
  return (
    <div
      className="rounded-xl bg-beton-900 border border-beton-800 px-4 py-3"
      style={{ boxShadow: "var(--shadow)" }}
    >
      <p className="flex items-center gap-2 text-[11px] uppercase tracking-wider text-beton-500">
        <span
          className="inline-block w-1.5 h-1.5 rounded-full"
          style={{ background: empty ? "rgb(var(--beton-600))" : bad ? "#ef4444" : "var(--accent)" }}
        />
        {label}
      </p>
      {empty ? (
        <p className="mt-1.5 text-sm text-beton-500">Veri yok</p>
      ) : (
        <p
          className={"mt-1.5 font-display text-xl font-medium " + (bad ? "text-red-400" : "text-beton-100")}
          style={{ fontVariantNumeric: "tabular-nums" }}
        >
          {value}
        </p>
      )}
      {action}
    </div>
  );
}
function Badge({ label, tone }: { label: string; tone: "red" | "amber" | "blue" | "muted" }) {
  const cls =
    tone === "red"
      ? "bg-red-500/15 text-red-400"
      : tone === "amber"
        ? "bg-emniyet-500/15 text-emniyet-500"
        : tone === "blue"
          ? "bg-blue-500/15 text-blue-400"
          : "bg-beton-800 text-beton-300";
  return <span className={"font-mono text-xs px-2 py-1 rounded " + cls}>{label}</span>;
}
