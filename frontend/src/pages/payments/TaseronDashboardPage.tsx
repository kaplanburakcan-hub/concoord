import { useEffect, useState } from "react";
import { api } from "../../api/client";
import { useProjects } from "../ProjectContext";

type Sub = {
  id: string; company_name: string; trade?: string; phone?: string; email?: string;
};
type ContractDTO = {
  id: string; subcontractor_id?: string; contract_no: string; type: string;
  amount: number; start_date?: string | null; end_date?: string | null;
  revised_end_date?: string | null;
};
type Payment = {
  id: string; subcontractor_id: string; status: string; gross_this?: number;
};
type Tedarikci = {
  id: string; company_name: string; trade?: string; contact_person?: string;
  phone?: string; email?: string;
};

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
function timePct(start?: string | null, end?: string | null): number | null {
  if (!start || !end) return null;
  const s = new Date(start).getTime(), e = new Date(end).getTime(), n = Date.now();
  if (e <= s) return 100;
  return Math.min(100, Math.max(0, Math.round(((n - s) / (e - s)) * 100)));
}

// ── İkili İlerleme Barı ───────────────────────────────────────────────
function DualBar({ pct1, pct2, label1, label2 }: { pct1: number | null; pct2: number | null; label1: string; label2: string }) {
  return (
    <div className="space-y-1 min-w-[120px]">
      <div className="flex items-center gap-2">
        <div className="flex-1 h-2.5 bg-beton-800 rounded-full overflow-hidden">
          {pct1 != null && (
            <div
              className={`h-full rounded-full ${pct1 >= 100 ? "bg-green-500" : "bg-orange-500"}`}
              style={{ width: `${pct1}%` }}
            />
          )}
        </div>
        <span className="text-xs tabular-nums w-8 text-right text-beton-300">{pct1 != null ? `${pct1}%` : "—"}</span>
      </div>
      <div className="flex items-center gap-2">
        <div className="flex-1 h-2.5 bg-beton-800 rounded-full overflow-hidden">
          {pct2 != null && (
            <div
              className={`h-full rounded-full ${pct2 > 100 ? "bg-red-500" : "bg-sky-500"}`}
              style={{ width: `${Math.min(pct2, 100)}%` }}
            />
          )}
        </div>
        <span className={`text-xs tabular-nums w-8 text-right ${pct2 != null && pct2 > 100 ? "text-red-400" : "text-beton-300"}`}>
          {pct2 != null ? `${Math.min(pct2, 100)}%` : "—"}
        </span>
      </div>
      <div className="flex gap-3 pt-0.5">
        <span className="flex items-center gap-1 text-[10px] text-beton-500">
          <span className="inline-block w-2.5 h-1.5 rounded-sm bg-orange-500" />{label1}
        </span>
        <span className="flex items-center gap-1 text-[10px] text-beton-500">
          <span className="inline-block w-2.5 h-1.5 rounded-sm bg-sky-500" />{label2}
        </span>
      </div>
    </div>
  );
}

export default function TaseronDashboardPage() {
  const { current } = useProjects();
  const pid = current?.id;
  const [subs, setSubs] = useState<Sub[]>([]);
  const [contracts, setContracts] = useState<ContractDTO[]>([]);
  const [payments, setPayments] = useState<Payment[]>([]);
  const [tedarikciler, setTedarikciler] = useState<Tedarikci[]>([]);
  const [loading, setLoading] = useState(true);
  const [hideCompleted, setHideCompleted] = useState(false);

  useEffect(() => {
    if (!pid) return;
    setLoading(true);
    Promise.all([
      api<{ subcontractors: Sub[] }>(`/projects/${pid}/subcontractors`, { projectId: pid }),
      api<{ contracts: ContractDTO[] }>(`/projects/${pid}/contracts`, { projectId: pid })
        .catch(() => ({ contracts: [] as ContractDTO[] })),
      api<{ payments: Payment[] }>(`/projects/${pid}/payments`, { projectId: pid })
        .catch(() => ({ payments: [] as Payment[] })),
      api<{ tedarikciler: Tedarikci[] }>(`/projects/${pid}/tedarikciler`, { projectId: pid })
        .catch(() => ({ tedarikciler: [] as Tedarikci[] })),
    ]).then(([sr, cr, pr, tr]) => {
      setSubs(sr.subcontractors ?? []);
      setContracts(cr.contracts ?? []);
      setPayments(pr.payments ?? []);
      setTedarikciler(tr.tedarikciler ?? []);
    }).finally(() => setLoading(false));
  }, [pid]);

  if (!current) return <div className="p-8 text-beton-400">Önce üst bardan bir proje seçin.</div>;
  if (loading) return <div className="p-8 text-beton-400">Yükleniyor…</div>;

  const today = new Date();

  // Birincil sözleşme (en yüksek bedel) per taşeron
  const primaryBySub: Record<string, ContractDTO> = {};
  contracts.forEach(c => {
    if (!c.subcontractor_id) return;
    const ex = primaryBySub[c.subcontractor_id];
    if (!ex || (c.amount ?? 0) > (ex.amount ?? 0)) primaryBySub[c.subcontractor_id] = c;
  });

  // Kesinleşmiş (Finalized) hakedişlerin taşeron bazlı toplamı
  const certifiedBySub: Record<string, number> = {};
  payments.forEach(p => {
    if (p.status === "Finalized" && p.gross_this) {
      certifiedBySub[p.subcontractor_id] = (certifiedBySub[p.subcontractor_id] ?? 0) + p.gross_this;
    }
  });

  // KPI hesaplamaları
  const totalValue = Object.values(primaryBySub).reduce((s, c) => s + (c.amount ?? 0), 0);
  const activeCount = Object.values(primaryBySub).filter(c => {
    const eff = c.revised_end_date ?? c.end_date;
    return c.start_date && eff && new Date(c.start_date) <= today && new Date(eff) >= today;
  }).length;
  const totalCertified = Object.values(certifiedBySub).reduce((s, v) => s + v, 0);

  // Taşeron satır verisi
  type SubRow = {
    sub: Sub; contract: ContractDTO | undefined;
    isilerleme: number | null; sureilerleme: number | null;
    certified: number; kalanIs: number | null; kalanSure: number | null;
  };
  const subRows: SubRow[] = subs.map(sub => {
    const c = primaryBySub[sub.id];
    const eff = c?.revised_end_date ?? c?.end_date;
    const certified = certifiedBySub[sub.id] ?? 0;
    const isilerleme = c?.amount ? Math.round((certified / c.amount) * 100) : null;
    const sureilerleme = timePct(c?.start_date, eff);
    const kalanIs = c?.amount != null ? Math.max(0, c.amount - certified) : null;
    const kalanSure = eff ? diffDays(today, new Date(eff)) : null;
    return { sub, contract: c, isilerleme, sureilerleme, certified, kalanIs, kalanSure };
  });

  const visibleTedarikciler = hideCompleted ? tedarikciler : tedarikciler;

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
          { label: "Taşeron",           value: String(subs.length),               accent: "text-emniyet-500" },
          { label: "Tedarikçi",         value: String(tedarikciler.length),        accent: "text-blue-400" },
          { label: "Aktif Sözleşme",    value: String(activeCount),                accent: "text-green-400" },
          { label: "Kesinleşen Hakediş", value: totalCertified > 0 ? fmtMoney(totalCertified) : fmtMoney(0), accent: "text-orange-400" },
        ].map(k => (
          <div key={k.label} className="rounded-xl border border-beton-800 bg-beton-900 p-4">
            <div className="text-xs text-beton-400 uppercase tracking-wide mb-1">{k.label}</div>
            <div className={`text-lg font-bold ${k.accent}`}>{k.value}</div>
          </div>
        ))}
      </div>

      {/* ── Taşeronlar Tablosu ──────────────────────────────────────────── */}
      <div className="rounded-xl border border-beton-800 bg-beton-900 overflow-hidden">
        <div className="px-4 py-3 border-b border-beton-800 flex items-center gap-2">
          <span>🏗️</span>
          <h2 className="text-sm font-semibold text-white">Taşeronlar</h2>
          <span className="text-xs text-beton-500 ml-1">({subs.length} firma)</span>
          <div className="ml-auto flex items-center gap-3 text-xs text-beton-400">
            <span className="flex items-center gap-1">
              <span className="w-2.5 h-1.5 rounded-sm bg-orange-500 inline-block" />İş ilerlemesi
            </span>
            <span className="flex items-center gap-1">
              <span className="w-2.5 h-1.5 rounded-sm bg-sky-500 inline-block" />Süre ilerlemesi
            </span>
          </div>
        </div>

        {subRows.length === 0 ? (
          <p className="px-4 py-4 text-beton-400 text-sm">Taşeron kaydı yok.</p>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-sm border-collapse">
              <thead className="text-[11px] text-beton-400 uppercase tracking-wide border-b border-beton-800">
                <tr>
                  <th className="px-4 py-2 text-left font-medium">Taşeron</th>
                  <th className="px-4 py-2 text-left font-medium">Faaliyet Alanı</th>
                  <th className="px-4 py-2 text-right font-medium">Sözleşme</th>
                  <th className="px-4 py-2 text-left font-medium min-w-[180px]">İlerleme</th>
                  <th className="px-4 py-2 text-right font-medium">Kalan</th>
                </tr>
              </thead>
              <tbody>
                {subRows.map(({ sub, contract: c, isilerleme, sureilerleme, kalanIs, kalanSure }) => {
                  const eff = c?.revised_end_date ?? c?.end_date;
                  const isOverdue = eff && new Date(eff) < today;
                  return (
                    <tr key={sub.id} className="border-b border-beton-800/50 hover:bg-beton-800/20 transition-colors">
                      {/* Taşeron adı */}
                      <td className="px-4 py-3 font-medium text-white align-top">
                        {sub.company_name}
                        {sub.phone && <div className="text-xs text-beton-500 font-normal mt-0.5">{sub.phone}</div>}
                      </td>
                      {/* Branş */}
                      <td className="px-4 py-3 text-beton-300 text-xs align-top">
                        {sub.trade ?? <span className="text-beton-600">—</span>}
                      </td>
                      {/* Sözleşme bedeli + tarihler */}
                      <td className="px-4 py-3 text-right align-top">
                        {c ? (
                          <>
                            <div className="font-semibold text-white tabular-nums text-sm">
                              {fmtMoney(c.amount)}
                            </div>
                            <div className="text-[11px] text-beton-500 mt-0.5 whitespace-nowrap">
                              {fmtDate(c.start_date)} – {fmtDate(eff)}
                              {c.revised_end_date && <span className="ml-1 text-amber-400">↻</span>}
                            </div>
                            <div className="text-[11px] text-beton-500">{c.contract_no}</div>
                          </>
                        ) : (
                          <span className="text-beton-600 text-xs">Sözleşme yok</span>
                        )}
                      </td>
                      {/* Çift bar */}
                      <td className="px-4 py-3 align-top">
                        <DualBar
                          pct1={isilerleme}
                          pct2={sureilerleme}
                          label1="İş"
                          label2="Süre"
                        />
                      </td>
                      {/* Kalan */}
                      <td className="px-4 py-3 text-right align-top text-xs space-y-1">
                        {kalanIs != null && (
                          <div>
                            <div className="text-beton-400">Kalan İş</div>
                            <div className="text-white tabular-nums font-medium">{fmtMoney(kalanIs)}</div>
                          </div>
                        )}
                        {kalanSure != null && (
                          <div className="mt-1">
                            <div className="text-beton-400">Kalan Süre</div>
                            <div className={`tabular-nums font-medium ${isOverdue ? "text-red-400" : "text-white"}`}>
                              {kalanSure < 0 ? `${Math.abs(kalanSure)} gün geçti` : `${kalanSure} gün`}
                            </div>
                          </div>
                        )}
                        {kalanIs == null && kalanSure == null && (
                          <span className="text-beton-600">—</span>
                        )}
                      </td>
                    </tr>
                  );
                })}
              </tbody>
              {totalValue > 0 && (
                <tfoot className="border-t border-beton-700 bg-beton-800/30">
                  <tr>
                    <td colSpan={2} className="px-4 py-2 text-xs text-beton-400">Toplam</td>
                    <td className="px-4 py-2 text-right font-semibold text-white tabular-nums text-sm">
                      {fmtMoney(totalValue)}
                    </td>
                    <td />
                    <td className="px-4 py-2 text-right text-xs text-beton-400">
                      {totalCertified > 0 && (
                        <span>Kesinleşen: <span className="text-white font-medium">{fmtMoney(totalCertified)}</span></span>
                      )}
                    </td>
                  </tr>
                </tfoot>
              )}
            </table>
          </div>
        )}
      </div>

      {/* ── Tedarikçiler Tablosu ─────────────────────────────────────────── */}
      <div className="rounded-xl border border-beton-800 bg-beton-900 overflow-hidden">
        <div className="px-4 py-3 border-b border-beton-800 flex items-center gap-2 flex-wrap">
          <span>📦</span>
          <h2 className="text-sm font-semibold text-white">Tedarikçiler</h2>
          <span className="text-xs text-beton-500 ml-1">({tedarikciler.length} firma)</span>
          <label className="ml-auto flex items-center gap-1.5 text-xs text-beton-400 cursor-pointer select-none">
            <input
              type="checkbox"
              checked={hideCompleted}
              onChange={e => setHideCompleted(e.target.checked)}
              className="accent-emniyet-500"
            />
            Tamamlananları gizle
          </label>
        </div>

        {tedarikciler.length === 0 ? (
          <p className="px-4 py-4 text-beton-400 text-sm">Tedarikçi kaydı yok.</p>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-sm border-collapse">
              <thead className="text-[11px] text-beton-400 uppercase tracking-wide border-b border-beton-800">
                <tr>
                  <th className="px-4 py-2 text-left font-medium">Tedarikçi</th>
                  <th className="px-4 py-2 text-left font-medium">Ürün / Hizmet</th>
                  <th className="px-4 py-2 text-left font-medium">İletişim</th>
                </tr>
              </thead>
              <tbody>
                {visibleTedarikciler.map(t => (
                  <tr key={t.id} className="border-b border-beton-800/50 hover:bg-beton-800/20 transition-colors">
                    <td className="px-4 py-3 font-medium text-white align-top">{t.company_name}</td>
                    <td className="px-4 py-3 text-beton-300 text-xs align-top">
                      {t.trade ?? <span className="text-beton-600">—</span>}
                    </td>
                    <td className="px-4 py-3 text-xs text-beton-400 align-top space-y-0.5">
                      {t.contact_person && <div>{t.contact_person}</div>}
                      {t.phone && <div>{t.phone}</div>}
                      {t.email && <div>{t.email}</div>}
                      {!t.contact_person && !t.phone && !t.email && <span className="text-beton-600">—</span>}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}

        {tedarikciler.length > 0 && (
          <div className="px-4 py-2 border-t border-beton-800/50">
            <p className="text-[11px] text-beton-600">
              Tedarikçi sözleşme bedeli ve sipariş ilerlemesi, tedarikçi ekstreler modülüne entegre edildiğinde burada otomatik görünecek.
            </p>
          </div>
        )}
      </div>
    </div>
  );
}
