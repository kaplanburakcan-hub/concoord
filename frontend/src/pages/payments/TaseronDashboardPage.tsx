import { useEffect, useState } from "react";
import { api } from "../../api/client";
import { useProjects } from "../ProjectContext";

type Sub = {
  id: string; company_name: string; trade?: string; phone?: string; email?: string;
};
type ContractDTO = {
  id: string; subcontractor_id?: string; contract_no: string; type: string;
  amount: number; start_date?: string | null; end_date?: string | null;
  revised_end_date?: string | null; advance_rate_pct: number; retention_pct: number;
};
type Tedarikci = {
  id: string; company_name: string; trade?: string; contact_person?: string;
  phone?: string; email?: string;
};

function loadTedarikciler(pid: string): Tedarikci[] {
  try { return JSON.parse(localStorage.getItem(`ipks_tedarikciler_${pid}`) || "[]"); } catch { return []; }
}

function fmtDate(s?: string | null) {
  if (!s) return "—";
  return new Date(s).toLocaleDateString("tr-TR", { day: "2-digit", month: "short", year: "numeric" });
}
function fmtMoney(n: number) {
  return new Intl.NumberFormat("tr-TR", { style: "currency", currency: "TRY", maximumFractionDigits: 0 }).format(n);
}
function diffDays(a: Date, b: Date) {
  return Math.round((b.getTime() - a.getTime()) / 86400000);
}
function timePct(start?: string | null, end?: string | null, today = new Date()): number | null {
  if (!start || !end) return null;
  const s = new Date(start).getTime(), e = new Date(end).getTime(), n = today.getTime();
  if (e <= s) return 100;
  return Math.min(100, Math.max(0, Math.round(((n - s) / (e - s)) * 100)));
}

export default function TaseronDashboardPage() {
  const { current } = useProjects();
  const pid = current?.id;
  const [subs, setSubs] = useState<Sub[]>([]);
  const [contracts, setContracts] = useState<ContractDTO[]>([]);
  const [tedarikciler, setTedarikciler] = useState<Tedarikci[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    if (!pid) return;
    setLoading(true);
    Promise.all([
      api<{ subcontractors: Sub[] }>(`/projects/${pid}/subcontractors`, { projectId: pid }),
      api<{ contracts: ContractDTO[] }>(`/projects/${pid}/contracts`, { projectId: pid })
        .catch(() => ({ contracts: [] as ContractDTO[] })),
    ]).then(([sr, cr]) => {
      setSubs(sr.subcontractors ?? []);
      setContracts(cr.contracts ?? []);
      setTedarikciler(loadTedarikciler(pid));
    }).finally(() => setLoading(false));
  }, [pid]);

  if (!current) return <div className="p-8 text-beton-400">Önce üst bardan bir proje seçin.</div>;
  if (loading) return <div className="p-8 text-beton-400">Yükleniyor…</div>;

  const today = new Date();

  // Her taşeron için en yüksek bedelti sözleşmeyi birincil kabul et
  const primaryBySub: Record<string, ContractDTO> = {};
  contracts.forEach(c => {
    if (!c.subcontractor_id) return;
    const ex = primaryBySub[c.subcontractor_id];
    if (!ex || (c.amount ?? 0) > (ex.amount ?? 0)) primaryBySub[c.subcontractor_id] = c;
  });

  const totalValue = Object.values(primaryBySub).reduce((s, c) => s + (c.amount ?? 0), 0);
  const activeCount = Object.values(primaryBySub).filter(c => {
    const eff = c.revised_end_date ?? c.end_date;
    return c.start_date && eff && new Date(c.start_date) <= today && new Date(eff) >= today;
  }).length;

  // Gantt ekseni: tüm sözleşme tarihlerinden global min/max bul
  const allDates = subs
    .map(s => primaryBySub[s.id])
    .filter((c): c is ContractDTO => !!c)
    .flatMap(c => [c.start_date, c.revised_end_date ?? c.end_date])
    .filter((d): d is string => !!d)
    .map(d => new Date(d).getTime())
    .filter(t => !isNaN(t));

  const gMin = allDates.length ? Math.min(...allDates) : null;
  const gMax = allDates.length ? Math.max(...allDates) : null;
  const gRange = gMin != null && gMax != null ? gMax - gMin : 0;

  function toX(d: string): number {
    if (!gMin || !gRange) return 0;
    return Math.max(0, Math.min(100, ((new Date(d).getTime() - gMin) / gRange) * 100));
  }
  const todayX = gMin != null && gRange ? Math.max(0, Math.min(100, ((today.getTime() - gMin) / gRange) * 100)) : null;

  return (
    <div className="space-y-6 max-w-5xl mx-auto">
      {/* Başlık */}
      <div>
        <h1 className="font-display font-extrabold text-xl text-white">Taşeron & Tedarikçi Dashboard</h1>
        <p className="text-sm text-beton-400 mt-1">{current.name}</p>
      </div>

      {/* KPI Kartları */}
      <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
        {[
          { label: "Taşeron",       value: String(subs.length),           accent: "text-emniyet-500" },
          { label: "Tedarikçi",     value: String(tedarikciler.length),   accent: "text-blue-400" },
          { label: "Aktif Sözleşme", value: String(activeCount),          accent: "text-green-400" },
          { label: "Toplam Bedel",  value: totalValue > 0 ? fmtMoney(totalValue) : "—", accent: "text-yellow-400" },
        ].map(k => (
          <div key={k.label} className="rounded-xl border border-beton-800 bg-beton-900 p-4">
            <div className="text-xs text-beton-400 uppercase tracking-wide mb-1">{k.label}</div>
            <div className={`text-xl font-bold ${k.accent}`}>{k.value}</div>
          </div>
        ))}
      </div>

      {/* Sözleşme Süresi Çizelgesi (Gantt) */}
      {subs.length > 0 && gMin != null && gMax != null && (
        <div className="rounded-xl border border-beton-800 bg-beton-900 p-4">
          <h2 className="text-sm font-semibold text-white mb-3">Sözleşme Süresi Çizelgesi</h2>
          {/* Eksen başlıkları */}
          <div className="flex justify-between text-[10px] text-beton-500 mb-2 ml-[148px]">
            <span>{fmtDate(new Date(gMin).toISOString())}</span>
            <span>{fmtDate(new Date(gMax).toISOString())}</span>
          </div>
          <div className="space-y-2">
            {subs.map(sub => {
              const c = primaryBySub[sub.id];
              const eff = c?.revised_end_date ?? c?.end_date;
              const sx = c?.start_date ? toX(c.start_date) : null;
              const ex = eff ? toX(eff) : null;
              const pct = timePct(c?.start_date, eff, today);
              const overdue = eff != null && new Date(eff) < today;
              const daysRem = eff ? diffDays(today, new Date(eff)) : null;

              return (
                <div key={sub.id} className="flex items-center gap-2">
                  {/* Firma adı */}
                  <div
                    className="w-36 shrink-0 text-right text-xs text-beton-200 truncate"
                    title={sub.company_name}
                  >
                    {sub.company_name}
                  </div>
                  {/* Bar */}
                  <div className="flex-1 relative h-6 bg-beton-800 rounded overflow-hidden">
                    {sx != null && ex != null ? (
                      <>
                        {/* Arka plan (toplam süre) */}
                        <div
                          className={`absolute top-0 h-full rounded ${overdue ? "bg-red-500/25" : "bg-emniyet-500/20"}`}
                          style={{ left: `${sx}%`, width: `${ex - sx}%` }}
                        />
                        {/* Tamamlanan kısım */}
                        <div
                          className={`absolute top-0 h-full ${overdue ? "bg-red-500/80" : "bg-emniyet-500"}`}
                          style={{
                            left: `${sx}%`,
                            width: `${Math.max(0, Math.min(ex - sx, (todayX ?? 0) - sx))}%`,
                          }}
                        />
                        {pct != null && (
                          <span className="absolute inset-0 flex items-center justify-center text-[11px] font-semibold text-white drop-shadow">
                            {pct}%
                          </span>
                        )}
                      </>
                    ) : (
                      <span className="absolute inset-0 flex items-center px-2 text-xs text-beton-600">Tarih girilmemiş</span>
                    )}
                    {/* Bugün çizgisi */}
                    {todayX != null && (
                      <div
                        className="absolute top-0 bottom-0 w-px bg-white/80"
                        style={{ left: `${todayX}%` }}
                        title="Bugün"
                      />
                    )}
                  </div>
                  {/* Kalan gün */}
                  <div className="w-20 shrink-0 text-xs text-right tabular-nums">
                    {daysRem === null
                      ? <span className="text-beton-600">—</span>
                      : daysRem < 0
                        ? <span className="text-red-400">{Math.abs(daysRem)}g geçti</span>
                        : <span className="text-beton-400">{daysRem}g kaldı</span>
                    }
                  </div>
                </div>
              );
            })}
          </div>
          {/* Legend */}
          <div className="mt-3 flex gap-5 text-[11px] text-beton-500">
            <span className="flex items-center gap-1.5">
              <span className="inline-block w-4 h-2 rounded bg-emniyet-500" />Devam Eden
            </span>
            <span className="flex items-center gap-1.5">
              <span className="inline-block w-4 h-2 rounded bg-red-500/80" />Gecikmiş
            </span>
            <span className="flex items-center gap-1.5">
              <span className="inline-block w-px h-3 bg-white/80" />Bugün
            </span>
          </div>
        </div>
      )}

      {/* Taşeronlar Tablosu */}
      <div className="rounded-xl border border-beton-800 bg-beton-900 overflow-hidden">
        <div className="px-4 py-3 border-b border-beton-800 flex items-center gap-2">
          <span className="text-base">🏗️</span>
          <h2 className="text-sm font-semibold text-white">Taşeronlar</h2>
          <span className="text-xs text-beton-400 ml-auto">{subs.length} firma</span>
        </div>
        {subs.length === 0 ? (
          <p className="px-4 py-3 text-beton-400 text-sm">Taşeron kaydı yok.</p>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead className="text-xs text-beton-400 text-left border-b border-beton-800">
                <tr>
                  <th className="px-4 py-2">Firma</th>
                  <th className="px-4 py-2">Branş</th>
                  <th className="px-4 py-2">Sözleşme No</th>
                  <th className="px-4 py-2 text-right">Bedel</th>
                  <th className="px-4 py-2">Başlangıç</th>
                  <th className="px-4 py-2">Bitiş</th>
                  <th className="px-4 py-2 min-w-[120px]">İlerleme</th>
                </tr>
              </thead>
              <tbody>
                {subs.map(sub => {
                  const c = primaryBySub[sub.id];
                  const eff = c?.revised_end_date ?? c?.end_date;
                  const pct = timePct(c?.start_date, eff, today);
                  const overdue = eff != null && new Date(eff) < today && (pct ?? 0) < 100;
                  return (
                    <tr key={sub.id} className="border-b border-beton-800/50 text-beton-200">
                      <td className="px-4 py-2.5 font-medium">{sub.company_name}</td>
                      <td className="px-4 py-2.5 text-beton-400 text-xs">{sub.trade ?? "—"}</td>
                      <td className="px-4 py-2.5 font-mono text-xs text-beton-400">{c?.contract_no ?? "—"}</td>
                      <td className="px-4 py-2.5 text-right tabular-nums text-xs">
                        {c?.amount ? fmtMoney(c.amount) : "—"}
                      </td>
                      <td className="px-4 py-2.5 text-xs text-beton-400">{fmtDate(c?.start_date)}</td>
                      <td className="px-4 py-2.5 text-xs text-beton-400">
                        {fmtDate(eff)}
                        {c?.revised_end_date && (
                          <span className="ml-1 text-amber-400" title="Süre uzatımı">↻</span>
                        )}
                      </td>
                      <td className="px-4 py-2.5">
                        {pct != null ? (
                          <div className="flex items-center gap-2">
                            <div className="flex-1 h-1.5 bg-beton-800 rounded-full min-w-[60px]">
                              <div
                                className={`h-full rounded-full ${overdue ? "bg-red-500" : "bg-emniyet-500"}`}
                                style={{ width: `${pct}%` }}
                              />
                            </div>
                            <span className={`text-xs tabular-nums shrink-0 ${overdue ? "text-red-400" : "text-beton-400"}`}>
                              {pct}%
                            </span>
                          </div>
                        ) : (
                          <span className="text-beton-600 text-xs">—</span>
                        )}
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {/* Tedarikçiler Kartlar */}
      <div className="rounded-xl border border-beton-800 bg-beton-900 overflow-hidden">
        <div className="px-4 py-3 border-b border-beton-800 flex items-center gap-2">
          <span className="text-base">📦</span>
          <h2 className="text-sm font-semibold text-white">Tedarikçiler</h2>
          <span className="text-xs text-beton-400 ml-auto">{tedarikciler.length} firma</span>
        </div>
        {tedarikciler.length === 0 ? (
          <p className="px-4 py-3 text-beton-400 text-sm">Tedarikçi kaydı yok.</p>
        ) : (
          <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 gap-3 p-4">
            {tedarikciler.map(t => (
              <div key={t.id} className="rounded-lg border border-beton-800 bg-beton-950 p-3 space-y-0.5">
                <div className="font-medium text-white text-sm">{t.company_name}</div>
                {t.trade && <div className="text-xs text-beton-400">{t.trade}</div>}
                {t.contact_person && <div className="text-xs text-beton-500 mt-1">👤 {t.contact_person}</div>}
                {t.phone && <div className="text-xs text-beton-500">📞 {t.phone}</div>}
                {t.email && <div className="text-xs text-beton-500">✉ {t.email}</div>}
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
