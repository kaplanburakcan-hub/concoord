import { useCallback, useEffect, useRef, useState } from "react";
import { Link } from "react-router-dom";
import { api } from "../../api/client";
import { useAuth } from "../../auth/AuthContext";
import { useProjects } from "../../projects/ProjectContext";

// ── Types — backend snapshot modeli ──────────────────────────────────────────

type WRStatus = "Pending" | "Ready" | "Failed";

type WeeklyReport = {
  id: string;
  week_no: number;
  period_start: string;
  period_end: string;
  status: WRStatus;
  error?: string;
  generated_by_name: string;
  has_pdf: boolean;
  created_at: string;
};

type SnapManpower  = { trade?: string; headcount: number; subcontractor_name?: string };
type SnapEquipment = { equipment_name: string; count: number; working_hours?: number };
type SnapWorkEntry = { description: string; location?: string; qty?: number; unit?: string };

type SnapDay = {
  date: string;
  status: string;
  weather_condition?: string;
  temperature_min?: number;
  temperature_max?: number;
  notes?: string;
  manpower_total: number;
  manpower?: SnapManpower[];
  equipment?: SnapEquipment[];
  work_entries?: SnapWorkEntry[];
};

type SnapDeliveryItem = { material_name: string; unit: string; qty: number };
type SnapDelivery = {
  po_no: string; supplier: string; delivery_note_no: string;
  delivered_at: string; items: SnapDeliveryItem[];
};

type SnapStockItem = {
  malzeme_adi: string; kategori: string; birim: string;
  mevcut_miktar: number; min_stok: number;
  week_giris: number; week_cikis: number; below_min: boolean;
};

type SnapPendingPO  = { po_no: string; supplier: string; expected_date?: string; overdue: boolean };
type SnapPayment    = { subcontractor: string; period_no: number; status: string };

type Snapshot = {
  generated_at: string;
  project_name: string; project_code: string;
  week_no: number; period_start: string; period_end: string;
  days: SnapDay[];
  totals: { days_reported: number; manpower_person_days: number; equipment_hours: number; work_entry_count: number };
  deliveries: SnapDelivery[];
  stock: SnapStockItem[];
  pending_payments: SnapPayment[];
  pending_pos: SnapPendingPO[];
  open_tasks: number; tasks_due_this_week: number; pending_mars: number;
};

// ── Helpers ───────────────────────────────────────────────────────────────────

function mondayOf(d: Date): Date {
  const wd = d.getDay() || 7;
  const m = new Date(d);
  m.setDate(d.getDate() - (wd - 1));
  return m;
}
function fmtISO(d: Date) {
  const y = d.getFullYear();
  const m = String(d.getMonth() + 1).padStart(2, "0");
  const day = String(d.getDate()).padStart(2, "0");
  return `${y}-${m}-${day}`;
}
function fmtTR(iso: string) {
  const [y, mo, day] = iso.split("-");
  return `${day}.${mo}.${y}`;
}
function weatherIcon(c?: string) {
  if (!c) return "";
  const l = c.toLowerCase();
  if (l.includes("fırtına"))                       return "⛈️";
  if (l.includes("yağmur") || l.includes("yağış")) return "🌧️";
  if (l.includes("kar"))                           return "❄️";
  if (l.includes("parçalı") || l.includes("bulut"))return "⛅";
  if (l.includes("açık") || l.includes("güneş"))  return "☀️";
  if (l.includes("sis"))                           return "🌫️";
  return "🌡️";
}

const WR_STATUS_LABEL: Record<WRStatus, string> = {
  Pending: "Hazırlanıyor",
  Ready:   "Hazır",
  Failed:  "Hata",
};
const WR_STATUS_STYLE: Record<WRStatus, string> = {
  Pending: "bg-yellow-500/15 text-yellow-300 border-yellow-500/40",
  Ready:   "bg-green-500/15 text-green-300 border-green-500/40",
  Failed:  "bg-red-500/15 text-red-400 border-red-500/40",
};

// ── Sub-components ────────────────────────────────────────────────────────────

function SecHeader({ color, title }: { color: string; title: string }) {
  return (
    <div className="flex items-center gap-2.5 px-4 py-2.5 border-b border-beton-800">
      <div className="w-[3px] h-3.5 rounded shrink-0" style={{ background: color }} />
      <span className="text-[11px] font-bold uppercase tracking-widest text-beton-100">{title}</span>
    </div>
  );
}

const th  = "text-left text-[10px] font-bold uppercase tracking-wider text-beton-400 pb-2 pr-3 whitespace-nowrap";
const td  = "py-2 pr-3 text-[12.5px] text-beton-100 border-b border-beton-800/60 align-middle";
const tdM = "py-2 pr-3 text-[12.5px] text-beton-400 border-b border-beton-800/60 align-middle";

function SnapshotView({ sn, reportId, pid, hasPdf }: {
  sn: Snapshot; reportId: string; pid: string; hasPdf: boolean;
}) {
  const pendingCount =
    sn.pending_payments.length +
    sn.pending_pos.length +
    (sn.pending_mars > 0 ? 1 : 0) +
    (sn.open_tasks > 0 ? 1 : 0);

  return (
    <div className="space-y-3 pt-1">

      {/* Genel özet kartları */}
      <div className="grid grid-cols-2 sm:grid-cols-4 gap-2">
        {[
          { label: "Günlük Rapor",   value: sn.totals.days_reported },
          { label: "Adam-Gün",       value: sn.totals.manpower_person_days },
          { label: "Ekipman (sa)",   value: sn.totals.equipment_hours.toFixed(1) },
          { label: "İmalat Kalemi",  value: sn.totals.work_entry_count },
        ].map(({ label, value }) => (
          <div key={label} className="rounded-lg border border-beton-800 bg-beton-950 px-3 py-2.5 text-center">
            <div className="text-lg font-bold text-white tabular-nums">{value}</div>
            <div className="text-[10px] uppercase tracking-wider text-beton-500 mt-0.5">{label}</div>
          </div>
        ))}
      </div>

      {/* Günlük özet tablosu */}
      {sn.days.length > 0 && (
        <div className="rounded-lg border border-beton-800 bg-beton-900 overflow-hidden">
          <SecHeader color="#3b7fd4" title="Günlük Saha Özeti" />
          <div className="overflow-x-auto">
            <table className="w-full min-w-[540px]">
              <thead>
                <tr>
                  <th className={`${th} pl-4`}>Tarih</th>
                  <th className={th}>Hava</th>
                  <th className={`${th} text-right`}>Personel</th>
                  <th className={`${th} text-right`}>İmalat</th>
                  <th className={`${th} text-right`}>Ekipman (sa)</th>
                  <th className={`${th} pr-4`}>Not</th>
                </tr>
              </thead>
              <tbody>
                {sn.days.map((d) => {
                  const eqHours = (d.equipment ?? []).reduce(
                    (s, e) => s + (e.working_hours ?? 0), 0
                  );
                  return (
                    <tr key={d.date} className="hover:bg-beton-800/30 transition-colors">
                      <td className={`${td} pl-4 tabular-nums whitespace-nowrap`}>{fmtTR(d.date)}</td>
                      <td className={tdM}>
                        {weatherIcon(d.weather_condition)}{" "}
                        <span className="text-[11px]">{d.weather_condition ?? "—"}</span>
                        {d.temperature_min != null && (
                          <span className="ml-1 text-[10px] text-beton-500 tabular-nums">
                            {d.temperature_min}°/{d.temperature_max}°
                          </span>
                        )}
                      </td>
                      <td className={`${td} text-right tabular-nums`}>{d.manpower_total}</td>
                      <td className={`${td} text-right tabular-nums`}>{(d.work_entries ?? []).length}</td>
                      <td className={`${td} text-right tabular-nums`}>{eqHours > 0 ? eqHours.toFixed(1) : "—"}</td>
                      <td className={`${tdM} pr-4 max-w-[180px] truncate`}>{d.notes || "—"}</td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {/* Teslimatlar */}
      {sn.deliveries.length > 0 && (
        <div className="rounded-lg border border-beton-800 bg-beton-900 overflow-hidden">
          <SecHeader color="#22c55e" title={`Gelen Malzeme & Teslimat (${sn.deliveries.length} irsaliye)`} />
          <div className="divide-y divide-beton-800/60">
            {sn.deliveries.map((d, i) => (
              <div key={i} className="px-4 py-2.5">
                <div className="flex items-center gap-2 flex-wrap mb-1">
                  <span className="text-xs font-semibold text-beton-100">{d.supplier}</span>
                  <span className="text-[10px] text-beton-500">PO: {d.po_no}</span>
                  <span className="text-[10px] text-beton-500">İrsaliye: {d.delivery_note_no}</span>
                  <span className="text-[10px] text-beton-500 ml-auto">{fmtTR(d.delivered_at)}</span>
                </div>
                <div className="flex flex-wrap gap-x-4 gap-y-0.5">
                  {(d.items ?? []).map((it, j) => (
                    <span key={j} className="text-[11.5px] text-beton-300 tabular-nums">
                      {it.material_name} — <strong>{it.qty}</strong> {it.unit}
                    </span>
                  ))}
                </div>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Depo-Stok Durumu */}
      {sn.stock.length > 0 && (
        <div className="rounded-lg border border-beton-800 bg-beton-900 overflow-hidden">
          <SecHeader color="#f59e0b" title="Depo-Stok Durumu" />
          <div className="overflow-x-auto">
            <table className="w-full min-w-[500px]">
              <thead>
                <tr>
                  <th className={`${th} pl-4`}>Malzeme</th>
                  <th className={th}>Kategori</th>
                  <th className={`${th} text-right`}>Haftalık Giriş</th>
                  <th className={`${th} text-right`}>Haftalık Çıkış</th>
                  <th className={`${th} text-right`}>Mevcut Stok</th>
                  <th className={`${th} pr-4`}>Birim</th>
                </tr>
              </thead>
              <tbody>
                {sn.stock.map((s, i) => (
                  <tr
                    key={i}
                    className={`hover:bg-beton-800/30 transition-colors ${s.below_min ? "bg-red-500/5" : ""}`}
                  >
                    <td className={`${td} pl-4`}>
                      {s.below_min && <span className="text-red-400 mr-1 text-xs">⚠</span>}
                      {s.malzeme_adi}
                    </td>
                    <td className={tdM}>{s.kategori}</td>
                    <td className={`${td} text-right tabular-nums text-green-400`}>
                      {s.week_giris > 0 ? `+${s.week_giris}` : "—"}
                    </td>
                    <td className={`${td} text-right tabular-nums text-red-400`}>
                      {s.week_cikis > 0 ? `-${s.week_cikis}` : "—"}
                    </td>
                    <td className={`${td} text-right tabular-nums font-semibold ${s.below_min ? "text-red-400" : ""}`}>
                      {s.mevcut_miktar}
                    </td>
                    <td className={`${tdM} pr-4`}>{s.birim}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          {sn.stock.some((s) => s.below_min) && (
            <p className="px-4 py-2 text-[11px] text-red-400">
              ⚠ Kırmızı satırlar minimum stok seviyesinin altında.
            </p>
          )}
        </div>
      )}

      {/* Bekleyen Aktiviteler */}
      {pendingCount > 0 && (
        <div className="rounded-lg border border-beton-800 bg-beton-900 overflow-hidden">
          <SecHeader color="#4e6a87" title="Bekleyen Aktiviteler" />
          <div className="px-4 py-3 space-y-4">

            {sn.pending_payments.length > 0 && (
              <div>
                <p className="text-[10.5px] font-bold uppercase tracking-wider text-beton-400 mb-1.5">
                  Hakediş Onay Süreci ({sn.pending_payments.length})
                </p>
                <div className="space-y-1">
                  {sn.pending_payments.map((p, i) => (
                    <div key={i} className="flex items-center gap-2 text-[12.5px]">
                      <span className="text-beton-100">{p.subcontractor}</span>
                      <span className="text-beton-500">·</span>
                      <span className="text-beton-400">Dönem {p.period_no}</span>
                      <span className="ml-auto rounded border border-beton-700 bg-beton-800 px-1.5 py-0.5 text-[10px] text-beton-300">
                        {p.status}
                      </span>
                    </div>
                  ))}
                </div>
              </div>
            )}

            {sn.pending_pos.length > 0 && (
              <div>
                <p className="text-[10.5px] font-bold uppercase tracking-wider text-beton-400 mb-1.5">
                  Açık Siparişler ({sn.pending_pos.length})
                </p>
                <div className="space-y-1">
                  {sn.pending_pos.map((p, i) => (
                    <div key={i} className="flex items-center gap-2 text-[12.5px]">
                      <span className={p.overdue ? "text-red-400 font-medium" : "text-beton-100"}>
                        {p.overdue && "⚠ "}{p.supplier} — {p.po_no}
                      </span>
                      {p.expected_date && (
                        <span className={`ml-auto text-[10.5px] ${p.overdue ? "text-red-400" : "text-beton-400"}`}>
                          Termin: {fmtTR(p.expected_date)}
                        </span>
                      )}
                    </div>
                  ))}
                </div>
              </div>
            )}

            <div className="flex flex-wrap gap-3">
              {sn.pending_mars > 0 && (
                <div className="rounded-md border border-beton-800 bg-beton-950 px-3 py-2 text-center min-w-[100px]">
                  <div className="text-lg font-bold text-beton-100 tabular-nums">{sn.pending_mars}</div>
                  <div className="text-[10px] uppercase tracking-wider text-beton-500 mt-0.5">Bekleyen MAR</div>
                </div>
              )}
              {sn.open_tasks > 0 && (
                <div className="rounded-md border border-beton-800 bg-beton-950 px-3 py-2 text-center min-w-[100px]">
                  <div className="text-lg font-bold text-beton-100 tabular-nums">{sn.open_tasks}</div>
                  <div className="text-[10px] uppercase tracking-wider text-beton-500 mt-0.5">Açık Görev</div>
                </div>
              )}
              {sn.tasks_due_this_week > 0 && (
                <div className="rounded-md border border-yellow-500/30 bg-yellow-500/10 px-3 py-2 text-center min-w-[100px]">
                  <div className="text-lg font-bold text-yellow-300 tabular-nums">{sn.tasks_due_this_week}</div>
                  <div className="text-[10px] uppercase tracking-wider text-yellow-600 mt-0.5">Bu Hafta Termin</div>
                </div>
              )}
            </div>
          </div>
        </div>
      )}

      {/* PDF indir */}
      {hasPdf && (
        <a
          href={`/api/projects/${pid}/weekly-reports/${reportId}/download`}
          target="_blank"
          rel="noreferrer"
          className="flex items-center justify-center gap-2 rounded-lg border border-beton-700 bg-beton-900 px-4 py-2.5 text-sm text-beton-200 hover:border-emniyet-500 hover:text-white transition-colors"
        >
          PDF İndir
        </a>
      )}
    </div>
  );
}

// ── Ana sayfa ─────────────────────────────────────────────────────────────────

export default function WeeklyReportsPage() {
  const { current } = useProjects();
  const { can }     = useAuth();
  const pid         = current?.id;

  const [reports,     setReports]     = useState<WeeklyReport[]>([]);
  const [err,         setErr]         = useState<string | null>(null);
  const [expanded,    setExpanded]    = useState<string | null>(null);
  const [snapshots,   setSnapshots]   = useState<Record<string, Snapshot>>({});
  const [loadingSnap, setLoadingSnap] = useState<string | null>(null);
  const [generating,  setGenerating]  = useState(false);
  const [genWeek,     setGenWeek]     = useState(() => fmtISO(mondayOf(new Date())));
  const pollerRef = useRef<ReturnType<typeof setInterval> | null>(null);

  const load = useCallback(async () => {
    if (!pid) return;
    try {
      const res = await api<{ weekly_reports: WeeklyReport[] }>(
        `/projects/${pid}/weekly-reports`,
        { projectId: pid }
      );
      setReports(res.weekly_reports ?? []);
    } catch {
      setErr("Haftalık raporlar yüklenemedi.");
    }
  }, [pid]);

  useEffect(() => { load(); }, [load]);

  const toggle = useCallback(async (id: string) => {
    if (expanded === id) { setExpanded(null); return; }
    setExpanded(id);
    if (snapshots[id]) return;
    setLoadingSnap(id);
    try {
      const res = await api<{ snapshot: Snapshot }>(
        `/projects/${pid}/weekly-reports/${id}`,
        { projectId: pid! }
      );
      setSnapshots((prev) => ({ ...prev, [id]: res.snapshot }));
    } catch {
      // sessiz hata — "Veri yüklenemedi." gösterilir
    } finally {
      setLoadingSnap(null);
    }
  }, [expanded, pid, snapshots]);

  const generate = useCallback(async () => {
    if (!pid || generating) return;
    setGenerating(true);
    setErr(null);
    try {
      const res = await api<{ id: string }>(
        `/projects/${pid}/weekly-reports`,
        { method: "POST", projectId: pid, body: { period_start: genWeek } }
      );
      await load();
      setExpanded(res.id);
      if (pollerRef.current) clearInterval(pollerRef.current);
      pollerRef.current = setInterval(async () => {
        await load();
        setReports((prev) => {
          const r = prev.find((x) => x.id === res.id);
          if (r && r.status !== "Pending") {
            clearInterval(pollerRef.current!);
            pollerRef.current = null;
          }
          return prev;
        });
      }, 5000);
    } catch {
      setErr("Rapor oluşturulamadı.");
    } finally {
      setGenerating(false);
    }
  }, [pid, generating, genWeek, load]);

  useEffect(() => () => { if (pollerRef.current) clearInterval(pollerRef.current); }, []);

  if (!pid) {
    return <p className="text-beton-400 text-sm">Önce üst bardan bir proje seçin.</p>;
  }

  return (
    <div className="max-w-3xl mx-auto space-y-4">

      <div className="flex items-center justify-between gap-3 flex-wrap">
        <div>
          <h1 className="font-display font-extrabold text-xl text-white">Haftalık İlerleme Raporları</h1>
          <p className="text-xs text-beton-500 mt-0.5">
            Günlük raporlar, teslimatlar ve depo verilerinden otomatik derlenir.
          </p>
        </div>
        <Link
          to="/saha-raporlari"
          className="rounded-md border border-beton-700 px-3 py-2 text-sm text-beton-200 hover:border-emniyet-500"
        >
          Günlük Raporlar
        </Link>
      </div>

      {err && <p className="text-red-400 text-sm">{err}</p>}

      {can("reports.generate_weekly") && (
        <div className="rounded-lg border border-beton-800 bg-beton-900 p-4">
          <p className="text-sm font-medium text-beton-100 mb-3">Yeni Haftalık Rapor Oluştur</p>
          <div className="flex gap-2 flex-wrap">
            <div className="flex-1 min-w-[180px]">
              <label className="block text-[10.5px] uppercase tracking-wider text-beton-500 mb-1">
                Hafta Başlangıcı (Pazartesi)
              </label>
              <input
                type="date"
                value={genWeek}
                onChange={(e) => {
                  const d = new Date(e.target.value);
                  if (!isNaN(d.getTime())) setGenWeek(fmtISO(mondayOf(d)));
                }}
                className="w-full rounded-md bg-beton-950 border border-beton-800 px-3 py-2 text-sm text-beton-100 outline-none focus:border-emniyet-500"
              />
            </div>
            <div className="flex items-end">
              <button
                onClick={generate}
                disabled={generating}
                className="rounded-md bg-emniyet-500 px-5 py-2 text-sm font-medium text-beton-950 hover:brightness-110 disabled:opacity-50"
              >
                {generating ? "Oluşturuluyor…" : "Oluştur"}
              </button>
            </div>
          </div>
          <p className="text-[10.5px] text-beton-500 mt-2">
            Seçilen haftanın günlük raporları, teslimatları ve depo hareketleri otomatik derlenir.
          </p>
        </div>
      )}

      {reports.length === 0 ? (
        <div className="rounded-lg border border-beton-800 bg-beton-900 p-8 text-center">
          <p className="text-beton-400 text-sm">Henüz haftalık rapor oluşturulmamış.</p>
        </div>
      ) : (
        <div className="space-y-2">
          {reports.map((r) => {
            const isOpen = expanded === r.id;
            const sn     = snapshots[r.id];
            return (
              <div key={r.id} className="rounded-lg border border-beton-800 bg-beton-900 overflow-hidden">
                <button
                  onClick={() => toggle(r.id)}
                  className="w-full flex items-center gap-3 px-4 py-3 hover:bg-beton-800/40 transition-colors text-left"
                >
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2 flex-wrap">
                      <span className="font-semibold text-beton-100 text-sm">
                        Hafta {r.week_no}
                      </span>
                      <span className="text-[11.5px] text-beton-400">
                        {fmtTR(r.period_start)} – {fmtTR(r.period_end)}
                      </span>
                      <span className={`rounded-full border px-2 py-0.5 text-[10.5px] font-semibold ${WR_STATUS_STYLE[r.status]}`}>
                        {WR_STATUS_LABEL[r.status]}
                      </span>
                    </div>
                    <p className="text-[11px] text-beton-500 mt-0.5">{r.generated_by_name}</p>
                  </div>
                  <svg
                    className={`w-4 h-4 text-beton-500 shrink-0 transition-transform ${isOpen ? "rotate-180" : ""}`}
                    fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}
                  >
                    <path strokeLinecap="round" strokeLinejoin="round" d="M19 9l-7 7-7-7" />
                  </svg>
                </button>

                {r.status === "Failed" && r.error && (
                  <p className="px-4 pb-3 text-xs text-red-400">{r.error}</p>
                )}

                {isOpen && r.status !== "Failed" && (
                  <div className="px-4 pb-4 space-y-2">
                    {r.status === "Pending" && (
                      <div className="flex items-center gap-2 rounded-lg border border-yellow-500/30 bg-yellow-500/10 px-4 py-3">
                        <svg className="w-4 h-4 animate-spin text-yellow-400 shrink-0" fill="none" viewBox="0 0 24 24">
                          <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
                          <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v8z" />
                        </svg>
                        <span className="text-sm text-yellow-300">Rapor verisi derleniyor…</span>
                      </div>
                    )}
                    {r.status === "Ready" && !r.has_pdf && (
                      <p className="text-[11px] text-beton-500">PDF hazırlanıyor — veriler aşağıda görüntülenebilir.</p>
                    )}
                    {loadingSnap === r.id ? (
                      <p className="text-sm text-beton-500 py-3">Yükleniyor…</p>
                    ) : sn ? (
                      <SnapshotView sn={sn} reportId={r.id} pid={pid} hasPdf={r.has_pdf} />
                    ) : (
                      <p className="text-sm text-beton-500 py-3">Veri yüklenemedi.</p>
                    )}
                  </div>
                )}
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}
