import { useCallback, useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { api } from "../../api/client";
import { useAuth } from "../../auth/AuthContext";
import { useProjects } from "../../projects/ProjectContext";
import PdfPreviewModal from "../../components/PdfPreviewModal";

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

type SnapDisciplineSection = {
  discipline: string;
  this_week: string[];
  next_week_plan?: string[];
};

type SnapNextWeekPlan = { discipline: string; plans: string[] };

type SnapDay = {
  date: string;
  revision_no?: number;
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

type SnapSubcontractor = {
  company_name: string; trade?: string; contact_person?: string; phone?: string;
  contract_no?: string; contract_amount?: number;
};
type SnapPurchaseOrder = {
  po_no: string; supplier: string; amount?: number; currency: string;
  status: string; expected_date?: string; overdue: boolean;
};
type SnapCashExpense = {
  date: string; description: string; category: string; amount: number; receipt_no?: string;
};

const PO_STATUS_LABEL: Record<string, string> = {
  Ordered: "Sipariş Verildi",
  PartiallyDelivered: "Kısmi Teslim",
  Delivered: "Teslim Edildi",
  Cancelled: "İptal",
};

type Snapshot = {
  generated_at: string;
  project_name: string; project_code: string;
  week_no: number; period_start: string; period_end: string;
  days: SnapDay[];
  totals: { days_reported: number; manpower_person_days: number; equipment_hours: number; work_entry_count: number };
  discipline_sections?: SnapDisciplineSection[];
  deliveries: SnapDelivery[];
  stock: SnapStockItem[];
  subcontractors?: SnapSubcontractor[];
  purchase_orders?: SnapPurchaseOrder[];
  cash_expenses?: SnapCashExpense[];
  cash_total?: number;
  pending_payments: SnapPayment[];
  pending_pos: SnapPendingPO[];
  open_tasks: number; tasks_due_this_week: number; pending_mars: number;
  ohs_note?: string;
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

const DISC_COLORS: Record<string, string> = {
  "İnşaat":  "#3b82f6",
  "Elektrik":"#f59e0b",
  "Mekanik": "#22c55e",
  "İSG":     "#f87171",
};
function discColor(d: string) { return DISC_COLORS[d] ?? "#6b7280"; }

function SnapshotView({ sn, reportId, pid, status, nextWeekPlans }: {
  sn: Snapshot; reportId: string; pid: string; status: WRStatus;
  nextWeekPlans: SnapNextWeekPlan[];
}) {
  const [showPdf, setShowPdf] = useState(false);
  const days            = sn.days            ?? [];
  const deliveries      = sn.deliveries      ?? [];
  const stock           = sn.stock           ?? [];
  const pendingPayments = sn.pending_payments ?? [];
  const pendingPOs      = sn.pending_pos      ?? [];

  const pendingCount =
    pendingPayments.length +
    pendingPOs.length +
    (sn.pending_mars > 0 ? 1 : 0) +
    (sn.open_tasks > 0 ? 1 : 0);

  // ── Haftalık aggregation ──────────────────────────────────────────────────
  const manpowerAgg = new Map<string, number>();
  days.forEach((d) =>
    (d.manpower ?? []).forEach((m) => {
      const key = m.trade ?? "Genel";
      manpowerAgg.set(key, (manpowerAgg.get(key) ?? 0) + m.headcount);
    })
  );

  const equipAgg = new Map<string, { hours: number; days: number }>();
  days.forEach((d) =>
    (d.equipment ?? []).forEach((e) => {
      const prev = equipAgg.get(e.equipment_name) ?? { hours: 0, days: 0 };
      equipAgg.set(e.equipment_name, {
        hours: prev.hours + (e.working_hours ?? 0),
        days:  prev.days  + 1,
      });
    })
  );

  // work entries: group by description, sum qty (assume same unit per description)
  const workAgg = new Map<string, { qty: number; unit: string; locations: string[] }>();
  days.forEach((d) =>
    (d.work_entries ?? []).forEach((w) => {
      const prev = workAgg.get(w.description);
      if (prev) {
        prev.qty += w.qty ?? 0;
        if (w.location && !prev.locations.includes(w.location))
          prev.locations.push(w.location);
      } else {
        workAgg.set(w.description, {
          qty: w.qty ?? 0,
          unit: w.unit ?? "",
          locations: w.location ? [w.location] : [],
        });
      }
    })
  );

  return (
    <div className="space-y-3 pt-1">

      {/* Durum + PDF aksiyonları — sağ üst köşe (günlük rapor ile aynı desen) */}
      <div className="no-print flex items-center justify-end gap-2">
        <span className={`rounded-full border px-2 py-0.5 text-[10.5px] font-semibold ${WR_STATUS_STYLE[status]}`}>
          {WR_STATUS_LABEL[status]}
        </span>
        {status === "Ready" && (
          <button
            onClick={() => setShowPdf(true)}
            className="rounded-md border border-beton-700 px-2.5 py-1 text-xs text-beton-300 hover:border-emniyet-500 transition-colors"
          >
            PDF Rapor Üret
          </button>
        )}
      </div>
      <PdfPreviewModal
        open={showPdf}
        onClose={() => setShowPdf(false)}
        title={`Haftalık Rapor — ${sn.project_name} (Hafta ${sn.week_no})`}
        fetchPath={`/projects/${pid}/weekly-reports/${reportId}/download`}
        downloadName={`haftalik-rapor-${sn.project_code}-hafta${sn.week_no}.pdf`}
      />

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

      {/* Günlük personel bar */}
      {days.length > 0 && (
        <div className="rounded-lg border border-beton-800 bg-beton-900 overflow-hidden">
          <div className="overflow-x-auto">
            <div className="flex min-w-max divide-x divide-beton-800">
              {days.map((d) => {
                const dayNames: Record<string, string> = {
                  "1": "PZT", "2": "SAL", "3": "ÇAR", "4": "PER",
                  "5": "CUM", "6": "CTS", "0": "PAZ",
                };
                const dayKey = String(new Date(d.date).getDay());
                const dayLabel = dayNames[dayKey] ?? "";
                const isToday = d.date === new Date().toISOString().slice(0, 10);
                const hasData = d.manpower_total > 0;
                return (
                  <div
                    key={d.date}
                    className={`flex-1 min-w-[80px] px-3 py-3 text-center transition-colors ${
                      isToday
                        ? "bg-emniyet-500/10 border-b-2 border-b-emniyet-500"
                        : "hover:bg-beton-800/30"
                    }`}
                  >
                    <div className={`text-[10px] font-bold uppercase tracking-widest mb-0.5 ${
                      isToday ? "text-emniyet-400" : "text-beton-500"
                    }`}>{dayLabel}</div>
                    <div className="text-[11px] text-beton-500 tabular-nums mb-2">
                      {fmtTR(d.date).slice(0, 5)}
                    </div>
                    <div className={`text-2xl font-bold tabular-nums leading-none ${
                      !hasData ? "text-beton-700" : isToday ? "text-white" : "text-beton-100"
                    }`}>
                      {hasData ? d.manpower_total : "—"}
                    </div>
                    <div className="text-[9px] uppercase tracking-wider text-beton-600 mt-1">kişi</div>
                  </div>
                );
              })}
            </div>
          </div>
        </div>
      )}

      {/* Günlük özet tablosu */}
      {days.length > 0 && (
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
                {days.map((d) => {
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

      {/* Personel Dökümü */}
      {manpowerAgg.size > 0 && (
        <div className="rounded-lg border border-beton-800 bg-beton-900 overflow-hidden">
          <SecHeader color="#a78bfa" title="Personel Dökümü (Haftalık)" />
          <div className="overflow-x-auto">
            <table className="w-full">
              <thead>
                <tr>
                  <th className={`${th} pl-4`}>Meslek / Unvan</th>
                  <th className={`${th} text-right pr-4`}>Adam-Gün</th>
                </tr>
              </thead>
              <tbody>
                {[...manpowerAgg.entries()].map(([trade, total]) => (
                  <tr key={trade} className="hover:bg-beton-800/30 transition-colors">
                    <td className={`${td} pl-4`}>{trade}</td>
                    <td className={`${td} text-right tabular-nums font-semibold pr-4`}>{total}</td>
                  </tr>
                ))}
                <tr className="bg-beton-800/20">
                  <td className={`${td} pl-4 font-bold text-beton-100`}>Toplam</td>
                  <td className={`${td} text-right tabular-nums font-bold text-white pr-4`}>
                    {sn.totals.manpower_person_days}
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
      )}

      {/* İmalat Kalemleri */}
      {workAgg.size > 0 && (
        <div className="rounded-lg border border-beton-800 bg-beton-900 overflow-hidden">
          <SecHeader color="#34d399" title="Haftalık İmalat Kalemleri" />
          <div className="overflow-x-auto">
            <table className="w-full min-w-[460px]">
              <thead>
                <tr>
                  <th className={`${th} pl-4`}>İş Kalemi</th>
                  <th className={th}>Konum</th>
                  <th className={`${th} text-right`}>Miktar</th>
                  <th className={`${th} pr-4`}>Birim</th>
                </tr>
              </thead>
              <tbody>
                {[...workAgg.entries()].map(([desc, { qty, unit, locations }]) => (
                  <tr key={desc} className="hover:bg-beton-800/30 transition-colors">
                    <td className={`${td} pl-4`}>{desc}</td>
                    <td className={tdM}>{locations.join(", ") || "—"}</td>
                    <td className={`${td} text-right tabular-nums font-semibold`}>
                      {qty > 0 ? qty.toLocaleString("tr-TR", { maximumFractionDigits: 3 }) : "—"}
                    </td>
                    <td className={`${tdM} pr-4`}>{unit || "—"}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {/* Ekipman Kullanımı */}
      {equipAgg.size > 0 && (
        <div className="rounded-lg border border-beton-800 bg-beton-900 overflow-hidden">
          <SecHeader color="#fb923c" title="Ekipman Kullanımı (Haftalık)" />
          <div className="overflow-x-auto">
            <table className="w-full">
              <thead>
                <tr>
                  <th className={`${th} pl-4`}>Ekipman</th>
                  <th className={`${th} text-right`}>Gün</th>
                  <th className={`${th} text-right pr-4`}>Toplam (sa)</th>
                </tr>
              </thead>
              <tbody>
                {[...equipAgg.entries()].map(([name, { hours, days: dCnt }]) => (
                  <tr key={name} className="hover:bg-beton-800/30 transition-colors">
                    <td className={`${td} pl-4`}>{name}</td>
                    <td className={`${td} text-right tabular-nums`}>{dCnt}</td>
                    <td className={`${td} text-right tabular-nums font-semibold pr-4`}>
                      {hours.toFixed(1)}
                    </td>
                  </tr>
                ))}
                <tr className="bg-beton-800/20">
                  <td className={`${td} pl-4 font-bold text-beton-100`} colSpan={2}>Toplam</td>
                  <td className={`${td} text-right tabular-nums font-bold text-white pr-4`}>
                    {sn.totals.equipment_hours.toFixed(1)} sa
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
      )}

      {/* Disiplin İlerleme Bölümleri */}
      {(() => {
        // snapshot'taki discipline_sections ile next_week_plans'ı birleştir
        const sections = sn.discipline_sections ?? [];
        const planMap = new Map(nextWeekPlans.map((p) => [p.discipline, p.plans]));
        if (sections.length === 0) return null;
        return (
          <>
            {sections.map((sec) => {
              const plans = planMap.get(sec.discipline) ?? sec.next_week_plan ?? [];
              return (
                <div key={sec.discipline} className="rounded-lg border border-beton-800 bg-beton-900 overflow-hidden">
                  <SecHeader color={discColor(sec.discipline)} title={`${sec.discipline} İlerleme`} />
                  <div className="grid sm:grid-cols-2 divide-y sm:divide-y-0 sm:divide-x divide-beton-800">
                    {/* Bu hafta */}
                    <div className="px-4 py-3">
                      <p className="text-[10px] font-bold uppercase tracking-widest text-beton-500 mb-2">
                        Haftalık İlerleme
                      </p>
                      <ul className="space-y-1.5">
                        {sec.this_week.map((item, i) => (
                          <li key={i} className="flex items-start gap-2 text-[12.5px] text-beton-200">
                            <span className="mt-1.5 w-1 h-1 rounded-full shrink-0" style={{ background: discColor(sec.discipline) }} />
                            {item}
                          </li>
                        ))}
                      </ul>
                    </div>
                    {/* Sonraki hafta */}
                    <div className="px-4 py-3">
                      <p className="text-[10px] font-bold uppercase tracking-widest text-beton-500 mb-2">
                        Sonraki Hafta Planlaması
                      </p>
                      {plans.length > 0 ? (
                        <ul className="space-y-1.5">
                          {plans.map((item, i) => (
                            <li key={i} className="flex items-start gap-2 text-[12.5px] text-beton-400">
                              <span className="mt-1.5 w-1 h-1 rounded-full bg-beton-600 shrink-0" />
                              {item}
                            </li>
                          ))}
                        </ul>
                      ) : (
                        <p className="text-[11px] text-beton-600 italic">Plan girilmemiş.</p>
                      )}
                    </div>
                  </div>
                </div>
              );
            })}
          </>
        );
      })()}

      {/* Aktif Taşeron Listesi — proje taşeronu olsun olmasın her zaman gösterilir */}
      <div className="rounded-lg border border-beton-800 bg-beton-900 overflow-hidden">
        <SecHeader color="#a78bfa" title="Aktif Taşeron Listesi" />
        {(sn.subcontractors ?? []).length === 0 ? (
          <p className="px-4 py-3 text-[12px] text-beton-500 italic">Bu projede kayıtlı aktif taşeron yok.</p>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full min-w-[560px]">
              <thead>
                <tr>
                  <th className={`${th} pl-4`}>Firma</th>
                  <th className={th}>Branş</th>
                  <th className={th}>Yetkili</th>
                  <th className={th}>Telefon</th>
                  <th className={th}>Sözleşme No</th>
                  <th className={`${th} text-right pr-4`}>Sözleşme Tutarı</th>
                </tr>
              </thead>
              <tbody>
                {(sn.subcontractors ?? []).map((s, i) => (
                  <tr key={i} className="hover:bg-beton-800/30 transition-colors">
                    <td className={`${td} pl-4`}>{s.company_name}</td>
                    <td className={tdM}>{s.trade || "—"}</td>
                    <td className={tdM}>{s.contact_person || "—"}</td>
                    <td className={tdM}>{s.phone || "—"}</td>
                    <td className={tdM}>{s.contract_no || "—"}</td>
                    <td className={`${td} text-right tabular-nums pr-4`}>
                      {s.contract_amount != null
                        ? s.contract_amount.toLocaleString("tr-TR", { maximumFractionDigits: 0 }) + " ₺"
                        : "—"}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {/* Satın Alma — açık sipariş/talep olsun olmasın her zaman gösterilir */}
      <div className="rounded-lg border border-beton-800 bg-beton-900 overflow-hidden">
        <SecHeader color="#fb923c" title="Satın Alma" />
        {(sn.purchase_orders ?? []).length === 0 ? (
          <p className="px-4 py-3 text-[12px] text-beton-500 italic">Bekleyen satın alma siparişi ya da talebi yok.</p>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full min-w-[560px]">
              <thead>
                <tr>
                  <th className={`${th} pl-4`}>PO No</th>
                  <th className={th}>Tedarikçi</th>
                  <th className={`${th} text-right`}>Tutar</th>
                  <th className={th}>Durum</th>
                  <th className={`${th} pr-4`}>Beklenen Tarih</th>
                </tr>
              </thead>
              <tbody>
                {(sn.purchase_orders ?? []).map((p, i) => (
                  <tr key={i} className="hover:bg-beton-800/30 transition-colors">
                    <td className={`${td} pl-4`}>{p.po_no}</td>
                    <td className={tdM}>{p.supplier}</td>
                    <td className={`${td} text-right tabular-nums font-semibold`}>
                      {p.amount != null
                        ? p.amount.toLocaleString("tr-TR", { maximumFractionDigits: 0 }) + " " + (p.currency || "TRY")
                        : "—"}
                    </td>
                    <td className={tdM}>
                      <span className={p.overdue ? "text-red-400 font-medium" : ""}>
                        {p.overdue && "⚠ "}{PO_STATUS_LABEL[p.status] ?? p.status}
                      </span>
                    </td>
                    <td className={`${tdM} pr-4`}>{p.expected_date ? fmtTR(p.expected_date) : "—"}</td>
                  </tr>
                ))}
                <tr className="bg-beton-800/20">
                  <td className={`${td} pl-4 font-bold text-beton-100`} colSpan={2}>Toplam</td>
                  <td className={`${td} text-right tabular-nums font-bold text-white`}>
                    {(sn.purchase_orders ?? []).reduce((s, p) => s + (p.amount ?? 0), 0)
                      .toLocaleString("tr-TR", { maximumFractionDigits: 0 })} TRY
                  </td>
                  <td className={tdM} colSpan={2}></td>
                </tr>
              </tbody>
            </table>
          </div>
        )}
      </div>

      {/* Şantiye Kasa Harcaması — girilmemiş olsun olmasın her zaman gösterilir */}
      <div className="rounded-lg border border-beton-800 bg-beton-900 overflow-hidden">
        <SecHeader color="#eab308" title="Şantiye Kasa Harcaması" />
        {(sn.cash_expenses ?? []).length === 0 ? (
          <p className="px-4 py-3 text-[12px] text-beton-500 italic">Bu hafta şantiye kasa harcaması girilmemiş.</p>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full min-w-[500px]">
              <thead>
                <tr>
                  <th className={`${th} pl-4`}>Tarih</th>
                  <th className={th}>Açıklama</th>
                  <th className={th}>Kategori</th>
                  <th className={`${th} text-right`}>Tutar</th>
                  <th className={`${th} pr-4`}>Makbuz No</th>
                </tr>
              </thead>
              <tbody>
                {(sn.cash_expenses ?? []).map((c, i) => (
                  <tr key={i} className="hover:bg-beton-800/30 transition-colors">
                    <td className={`${td} pl-4 tabular-nums whitespace-nowrap`}>{fmtTR(c.date)}</td>
                    <td className={td}>{c.description}</td>
                    <td className={tdM}>{c.category}</td>
                    <td className={`${td} text-right tabular-nums font-semibold`}>
                      {c.amount.toLocaleString("tr-TR", { minimumFractionDigits: 2, maximumFractionDigits: 2 })} ₺
                    </td>
                    <td className={`${tdM} pr-4`}>{c.receipt_no ?? "—"}</td>
                  </tr>
                ))}
                <tr className="bg-beton-800/20">
                  <td className={`${td} pl-4 font-bold text-beton-100`} colSpan={3}>Toplam</td>
                  <td className={`${td} text-right tabular-nums font-bold text-white`}>
                    {(sn.cash_total ?? 0).toLocaleString("tr-TR", { minimumFractionDigits: 2, maximumFractionDigits: 2 })} ₺
                  </td>
                  <td className={tdM}></td>
                </tr>
              </tbody>
            </table>
          </div>
        )}
      </div>

      {/* İSG Notu */}
      {sn.ohs_note && (
        <div className="rounded-lg border border-beton-800 bg-beton-900 overflow-hidden">
          <SecHeader color="#f87171" title="İSG / Güvenlik Notu" />
          <p className="px-4 py-3 text-[12.5px] text-beton-300 leading-relaxed">{sn.ohs_note}</p>
        </div>
      )}

      {/* Teslimatlar */}
      {deliveries.length > 0 && (
        <div className="rounded-lg border border-beton-800 bg-beton-900 overflow-hidden">
          <SecHeader color="#22c55e" title={`Gelen Malzeme & Teslimat (${deliveries.length} irsaliye)`} />
          <div className="divide-y divide-beton-800/60">
            {deliveries.map((d, i) => (
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

      {/* Depo-Stok Durumu — depo kalemi tanımlı olsun olmasın her zaman gösterilir */}
      <div className="rounded-lg border border-beton-800 bg-beton-900 overflow-hidden">
        <SecHeader color="#f59e0b" title="Depo-Stok Durumu" />
        {stock.length === 0 ? (
          <p className="px-4 py-3 text-[12px] text-beton-500 italic">Bu projede tanımlı depo kalemi yok.</p>
        ) : (
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
                {stock.map((s, i) => (
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
        )}
        {stock.some((s) => s.below_min) && (
          <p className="px-4 py-2 text-[11px] text-red-400">
            ⚠ Kırmızı satırlar minimum stok seviyesinin altında.
          </p>
        )}
      </div>

      {/* Bekleyen Aktiviteler */}
      {pendingCount > 0 && (
        <div className="rounded-lg border border-beton-800 bg-beton-900 overflow-hidden">
          <SecHeader color="#4e6a87" title="Bekleyen Aktiviteler" />
          <div className="px-4 py-3 space-y-4">

            {pendingPayments.length > 0 && (
              <div>
                <p className="text-[10.5px] font-bold uppercase tracking-wider text-beton-400 mb-1.5">
                  Hakediş Onay Süreci ({pendingPayments.length})
                </p>
                <div className="space-y-1">
                  {pendingPayments.map((p, i) => (
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

            {pendingPOs.length > 0 && (
              <div>
                <p className="text-[10.5px] font-bold uppercase tracking-wider text-beton-400 mb-1.5">
                  Açık Siparişler ({pendingPOs.length})
                </p>
                <div className="space-y-1">
                  {pendingPOs.map((p, i) => (
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

    </div>
  );
}

// ── Ana sayfa ─────────────────────────────────────────────────────────────────

export default function WeeklyReportsPage() {
  const { current } = useProjects();
  const { can }     = useAuth();
  const pid         = current?.id;

  const [reports,      setReports]      = useState<WeeklyReport[]>([]);
  const [err,          setErr]          = useState<string | null>(null);
  const [expanded,     setExpanded]     = useState<string | null>(null);
  const [snapshots,    setSnapshots]    = useState<Record<string, Snapshot>>({});
  const [nwPlans,      setNwPlans]      = useState<Record<string, SnapNextWeekPlan[]>>({});
  const [loadingSnap,  setLoadingSnap]  = useState<string | null>(null);
  const [generating,  setGenerating]  = useState(false);
  const [genWeek,     setGenWeek]     = useState(() => fmtISO(mondayOf(new Date())));
  const [showGen,     setShowGen]     = useState(false);

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
      const res = await api<{ snapshot: Snapshot; next_week_plans: SnapNextWeekPlan[] }>(
        `/projects/${pid}/weekly-reports/${id}`,
        { projectId: pid! }
      );
      setSnapshots((prev) => ({ ...prev, [id]: res.snapshot }));
      setNwPlans((prev) => ({ ...prev, [id]: res.next_week_plans ?? [] }));
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
      setShowGen(false);
      try {
        const snap = await api<{ snapshot: Snapshot; next_week_plans: SnapNextWeekPlan[] }>(
          `/projects/${pid}/weekly-reports/${res.id}`,
          { projectId: pid }
        );
        setSnapshots((prev) => ({ ...prev, [res.id]: snap.snapshot }));
        setNwPlans((prev) => ({ ...prev, [res.id]: snap.next_week_plans ?? [] }));
      } catch { /* sessiz — "Veri yüklenemedi." görünür */ }
    } catch {
      setErr("Rapor oluşturulamadı.");
    } finally {
      setGenerating(false);
    }
  }, [pid, generating, genWeek, load]);

  if (!pid) {
    return <p className="text-beton-400 text-sm">Önce üst bardan bir proje seçin.</p>;
  }

  return (
    <div className="max-w-3xl mx-auto space-y-4">

      <div className="flex items-center justify-between gap-3 flex-wrap">
        <div>
          <h1 className="font-display font-extrabold text-xl text-white">Haftalık İlerleme Raporları</h1>
          <p className="text-xs text-beton-500 mt-0.5">
            Günlük raporlar, teslimatlar ve depo verilerinden otomatik derlenen haftalık özetler.
          </p>
        </div>
        <div className="no-print flex items-center gap-2">
          {can("reports.generate_weekly") && (
            <button
              onClick={() => setShowGen((v) => !v)}
              className="rounded-md bg-emniyet-500 hover:bg-emniyet-600 px-3 py-2 text-sm font-medium text-beton-950 transition-colors"
            >
              + Rapor Üret
            </button>
          )}
          <Link
            to="/saha-raporlari"
            className="rounded-md border border-beton-700 px-3 py-2 text-sm text-beton-200 hover:border-beton-500 transition-colors"
          >
            Günlük Raporlar
          </Link>
        </div>
      </div>

      {err && <p className="text-red-400 text-sm">{err}</p>}

      {showGen && can("reports.generate_weekly") && (
        <div className="rounded-lg border border-beton-800 bg-beton-900/60 px-4 py-3 flex items-center gap-3 flex-wrap">
          <label className="text-xs text-beton-400 shrink-0">Hafta başlangıcı (Pazartesi):</label>
          <input
            type="date"
            value={genWeek}
            onChange={(e) => {
              const d = new Date(e.target.value);
              if (!isNaN(d.getTime())) setGenWeek(fmtISO(mondayOf(d)));
            }}
            className="rounded-md bg-beton-950 border border-beton-800 px-3 py-1.5 text-sm text-beton-100 outline-none focus:border-emniyet-500"
          />
          <button
            onClick={generate}
            disabled={generating}
            className="rounded-md bg-emniyet-500 px-4 py-1.5 text-sm font-medium text-beton-950 hover:brightness-110 disabled:opacity-50"
          >
            {generating ? "Oluşturuluyor…" : "Oluştur"}
          </button>
          <button
            onClick={() => setShowGen(false)}
            className="text-xs text-beton-500 hover:text-beton-300 transition-colors ml-auto"
          >
            İptal
          </button>
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
                    {loadingSnap === r.id ? (
                      <p className="text-sm text-beton-500 py-3">Yükleniyor…</p>
                    ) : sn ? (
                      <SnapshotView sn={sn} reportId={r.id} pid={pid} status={r.status}
                        nextWeekPlans={nwPlans[r.id] ?? []} />
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
