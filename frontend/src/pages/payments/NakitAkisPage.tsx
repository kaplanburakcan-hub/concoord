import { useCallback, useEffect, useState } from "react";
import { api } from "../../api/client";
import { useProjects } from "../ProjectContext";
import CashFlowChart, { type CashFlowPeriod } from "./CashFlowChart";

// Nakit Akış Faz F — gerçek cash_events (hakediş/ekstre/PO ödeme planları,
// kasa fişi, idari hakediş tahsilatı) ile Faz E'nin sanal sabit gider
// satırlarını birleştirip gün/hafta/ay gruplu, kümülatif bakiyeli bir rapor
// gösterir. Panel önerisindeki mockup'ın gerçek veriyle beslenen hali.

type Group = "daily" | "weekly" | "monthly";
type Resp = {
  periods: CashFlowPeriod[];
  summary: { total_in: number; total_out: number; net: number };
  from: string; to: string; group: Group;
};

const GROUP_LABEL: Record<Group, string> = { daily: "Günlük", weekly: "Haftalık", monthly: "Aylık" };

function fmt(n: number): string {
  return n.toLocaleString("tr-TR", { minimumFractionDigits: 2, maximumFractionDigits: 2 });
}
function todayISO(offsetDays: number): string {
  const d = new Date();
  d.setDate(d.getDate() + offsetDays);
  return d.toISOString().slice(0, 10);
}

export default function NakitAkisPage() {
  const { current } = useProjects();
  const pid = current?.id;

  const [from, setFrom] = useState(todayISO(-30));
  const [to, setTo] = useState(todayISO(60));
  const [group, setGroup] = useState<Group>("monthly");
  const [data, setData] = useState<Resp | null>(null);
  const [err, setErr] = useState<string | null>(null);

  const load = useCallback(async () => {
    if (!pid) return;
    setErr(null);
    try {
      const r = await api<Resp>(
        `/projects/${pid}/cash-flow?from=${from}&to=${to}&group=${group}`, { projectId: pid });
      setData(r);
    } catch {
      setErr("Nakit akış raporu yüklenemedi ya da erişim yetkiniz yok.");
    }
  }, [pid, from, to, group]);

  useEffect(() => { load(); }, [load]);

  if (!current) return <p className="text-beton-400 text-sm">Önce üst bardan bir proje seçin.</p>;

  return (
    <div className="max-w-6xl mx-auto space-y-4">
      <div>
        <h1 className="font-display font-extrabold text-xl text-white">Nakit Akış</h1>
        <p className="text-xs text-beton-400 mt-0.5">
          Hakediş/ekstre/sipariş ödeme planları, kasa fişi ve idari hakediş tahsilatı (gerçek) +
          sabit giderler (öngörülen) — dönem bazlı giriş/çıkış ve kümülatif bakiye.
        </p>
      </div>
      {err && <p className="text-sm text-red-400">{err}</p>}

      <div className="flex flex-wrap items-end gap-3 rounded-lg border border-beton-800 bg-beton-900 p-3">
        <label className="text-xs text-beton-400">
          Başlangıç
          <input type="date" value={from} onChange={(e) => setFrom(e.target.value)}
            className="mt-1 block rounded-md bg-beton-950 border border-beton-800 px-2 py-1.5 text-sm text-beton-100" />
        </label>
        <label className="text-xs text-beton-400">
          Bitiş
          <input type="date" value={to} onChange={(e) => setTo(e.target.value)}
            className="mt-1 block rounded-md bg-beton-950 border border-beton-800 px-2 py-1.5 text-sm text-beton-100" />
        </label>
        <div className="flex gap-1">
          {(Object.keys(GROUP_LABEL) as Group[]).map((g) => (
            <button key={g} onClick={() => setGroup(g)}
              className={`rounded-md px-3 py-1.5 text-xs font-medium border transition ${
                group === g ? "bg-emniyet-500 text-beton-950 border-emniyet-500"
                : "border-beton-700 text-beton-300 hover:border-beton-500"
              }`}>
              {GROUP_LABEL[g]}
            </button>
          ))}
        </div>
      </div>

      {data && (
        <>
          <div className="grid grid-cols-3 gap-3">
            <div className="rounded-lg border border-beton-800 bg-beton-900 p-3">
              <p className="text-xs text-beton-500 mb-1">Toplam Giriş</p>
              <p className="text-lg font-bold text-emniyet-500">{fmt(data.summary.total_in)} TL</p>
            </div>
            <div className="rounded-lg border border-beton-800 bg-beton-900 p-3">
              <p className="text-xs text-beton-500 mb-1">Toplam Çıkış</p>
              <p className="text-lg font-bold text-red-400">{fmt(data.summary.total_out)} TL</p>
            </div>
            <div className="rounded-lg border border-beton-800 bg-beton-900 p-3">
              <p className="text-xs text-beton-500 mb-1">Net</p>
              <p className={`text-lg font-bold ${data.summary.net >= 0 ? "text-white" : "text-red-400"}`}>
                {fmt(data.summary.net)} TL
              </p>
            </div>
          </div>

          <div className="rounded-lg border border-beton-800 bg-beton-900 p-4">
            <CashFlowChart periods={data.periods} />
          </div>

          <div className="border border-beton-800 rounded-lg overflow-hidden">
            <table className="w-full text-sm">
              <thead>
                <tr className="bg-beton-900 border-b border-beton-800 text-left text-xs text-beton-500">
                  <th className="py-2 px-3">Dönem</th>
                  <th className="py-2 px-3 text-right">Giriş</th>
                  <th className="py-2 px-3 text-right">Çıkış</th>
                  <th className="py-2 px-3 text-right">Net</th>
                  <th className="py-2 px-3 text-right">Kümülatif Bakiye</th>
                </tr>
              </thead>
              <tbody>
                {data.periods.map((p) => (
                  <tr key={p.label} className="border-b border-beton-800/50">
                    <td className="py-1.5 px-3 text-beton-200 font-mono text-xs">{p.label}</td>
                    <td className="py-1.5 px-3 text-right text-emniyet-500 font-mono">{p.in ? fmt(p.in) : "—"}</td>
                    <td className="py-1.5 px-3 text-right text-red-400 font-mono">{p.out ? fmt(p.out) : "—"}</td>
                    <td className={`py-1.5 px-3 text-right font-mono ${p.net >= 0 ? "text-beton-200" : "text-red-400"}`}>{fmt(p.net)}</td>
                    <td className={`py-1.5 px-3 text-right font-mono font-semibold ${p.cumulative_balance >= 0 ? "text-white" : "text-red-400"}`}>
                      {fmt(p.cumulative_balance)}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </>
      )}
    </div>
  );
}
