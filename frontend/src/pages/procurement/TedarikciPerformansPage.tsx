import { useCallback, useEffect, useState } from "react";
import { api, describeLoadError } from "../../api/client";
import { useProjects } from "../../projects/ProjectContext";

// Tedarikçi teslim performansı (E11). Salt okunur. Tedarikçi, PO'daki tedarikçi adına
// göre gruplanır; oran yalnız kapanmış (teslim edilmiş) siparişlerle hesaplanır.

type Row = {
  supplier: string; total: number; on_time: number; late: number; open_pending: number;
  on_time_rate?: number;
};

export default function TedarikciPerformansPage() {
  const { current } = useProjects();
  const pid = current?.id;
  const [rows, setRows] = useState<Row[]>([]);
  const [err, setErr] = useState<string | null>(null);

  const load = useCallback(async () => {
    if (!pid) return;
    setErr(null);
    try {
      const r = await api<{ suppliers: Row[] }>(`/projects/${pid}/procurement/supplier-performance`, { projectId: pid });
      setRows(r.suppliers ?? []);
    } catch (e) {
      setErr(describeLoadError(e, "Tedarikçi performansı"));
    }
  }, [pid]);

  useEffect(() => { load(); }, [load]);

  if (!current) return <p className="text-beton-400">Önce üst bardan bir proje seçin.</p>;

  return (
    <div className="space-y-4">
      <div>
        <h1 className="text-lg font-display font-bold text-white">Tedarikçi Performansı</h1>
        <p className="text-xs text-beton-500 mt-0.5">
          Yalnızca beklenen teslim tarihi girilmiş siparişler. Oran = zamanında / (zamanında + geç), kapanmış siparişlerle.
        </p>
      </div>

      {err && <p className="text-sm text-red-400">{err}</p>}

      <div className="overflow-x-auto rounded-lg border border-beton-800">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-beton-800 bg-beton-900/60 text-left text-[10px] font-bold uppercase tracking-wider text-beton-400">
              <th className="py-2 px-3">Tedarikçi</th>
              <th className="py-2 px-3 text-right">Sipariş</th>
              <th className="py-2 px-3 text-right">Zamanında</th>
              <th className="py-2 px-3 text-right">Geç</th>
              <th className="py-2 px-3 text-right">Açık (bekleyen)</th>
              <th className="py-2 px-3 text-right">Zamanında oranı</th>
            </tr>
          </thead>
          <tbody>
            {rows.length === 0 && (
              <tr><td colSpan={6} className="py-6 text-center text-beton-500">Beklenen teslim tarihi girilmiş sipariş yok.</td></tr>
            )}
            {rows.map((r) => (
              <tr key={r.supplier} className="border-b border-beton-800/60">
                <td className="py-2 px-3 text-beton-100">{r.supplier}</td>
                <td className="py-2 px-3 text-right font-mono text-beton-300">{r.total}</td>
                <td className="py-2 px-3 text-right font-mono text-beton-300">{r.on_time}</td>
                <td className={`py-2 px-3 text-right font-mono ${r.late > 0 ? "text-red-300" : "text-beton-300"}`}>{r.late}</td>
                <td className="py-2 px-3 text-right font-mono text-beton-300">{r.open_pending}</td>
                <td className="py-2 px-3 text-right font-mono text-beton-100">
                  {r.on_time_rate != null ? `%${(r.on_time_rate * 100).toFixed(0)}` : "—"}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}
