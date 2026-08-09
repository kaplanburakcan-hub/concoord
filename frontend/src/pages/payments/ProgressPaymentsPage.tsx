import { useCallback, useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { api } from "../../api/client";
import { useAuth } from "../../auth/AuthContext";
import { useProjects } from "../ProjectContext";
import RetentionPanel from "./RetentionPanel";

type Sub = { id: string; company_name: string };
type Payment = {
  id: string; subcontractor_id: string; period_no: number; status: string;
  period_start?: string; period_end?: string;
  gross_this?: number; net_payable?: number; vat_pct: number; created_at: string;
};

const STATUS_LABEL: Record<string, string> = {
  Draft: "Taslak", Submitted: "Gönderildi", SiteApproved: "Saha Onaylı",
  Finalized: "Kesinleşti", Rejected: "Reddedildi",
};
const STATUS_STYLE: Record<string, string> = {
  Draft: "bg-beton-800 text-beton-200", Submitted: "bg-blue-500/20 text-blue-300",
  SiteApproved: "bg-emniyet-500/20 text-emniyet-500", Finalized: "bg-green-500/20 text-green-300",
  Rejected: "bg-red-500/20 text-red-300",
};

export default function ProgressPaymentsPage() {
  const { current } = useProjects();
  const { can } = useAuth();
  const [subs, setSubs] = useState<Sub[]>([]);
  const [payments, setPayments] = useState<Payment[]>([]);
  const [subFilter, setSubFilter] = useState<string>("");
  const [newSub, setNewSub] = useState<string>("");
  const [err, setErr] = useState<string | null>(null);
  const pid = current?.id;
  const canFin = can("progress_payments.view_financials");

  const subName = (id: string) => subs.find((s) => s.id === id)?.company_name || "—";

  const load = useCallback(async () => {
    if (!pid) return;
    setErr(null);
    try {
      const s = await api<{ subcontractors: Sub[] }>(`/projects/${pid}/subcontractors`, { projectId: pid });
      setSubs(s.subcontractors);
      const q = subFilter ? `?subcontractor_id=${subFilter}` : "";
      const p = await api<{ payments: Payment[] }>(`/projects/${pid}/payments${q}`, { projectId: pid });
      setPayments(p.payments);
    } catch { setErr("Hakedişler yüklenemedi ya da erişim yetkiniz yok."); }
  }, [pid, subFilter]);

  useEffect(() => { load(); }, [load]);

  async function createDraft() {
    if (!newSub) return;
    try {
      await api(`/projects/${pid}/payments`, { method: "POST", projectId: pid, body: { subcontractor_id: newSub } });
      setNewSub(""); load();
    } catch { setErr("Taslak oluşturulamadı."); }
  }

  if (!current) return <p className="text-beton-400">Önce üst bardan bir proje seçin.</p>;

  return (
    <div>
      <h1 className="font-display text-2xl font-medium text-beton-100 tracking-tight">Hakedişler</h1>
      <p className="text-sm text-beton-400 mt-1">{current.name} — kümülatif hakediş iş akışı ve kesinti yönetimi.</p>
      {err && <p className="mt-3 text-sm text-red-400">{err}</p>}

      <div className="mt-4 rounded-lg border border-beton-800 bg-beton-900 p-3 flex flex-wrap items-center gap-3">
        <select
          value={subFilter} onChange={(e) => setSubFilter(e.target.value)}
          className="rounded bg-beton-950 border border-beton-800 px-2 py-1.5 text-sm text-white"
        >
          <option value="">Tüm taşeronlar</option>
          {subs.map((s) => <option key={s.id} value={s.id}>{s.company_name}</option>)}
        </select>

        {can("progress_payments.create_draft") && (
          <div className="flex items-center gap-2 ml-auto">
            <select
              value={newSub} onChange={(e) => setNewSub(e.target.value)}
              className="rounded bg-beton-950 border border-beton-800 px-2 py-1.5 text-sm text-white"
            >
              <option value="">Taşeron seç…</option>
              {subs.map((s) => <option key={s.id} value={s.id}>{s.company_name}</option>)}
            </select>
            <button
              onClick={createDraft} disabled={!newSub}
              className="rounded bg-emniyet-500 hover:bg-emniyet-600 text-beton-950 font-semibold text-sm px-3 py-1.5 disabled:opacity-50"
            >
              Yeni Taslak Hakediş
            </button>
          </div>
        )}
      </div>

      <table className="mt-4 w-full text-sm">
        <thead>
          <tr className="text-beton-400 text-left border-b border-beton-800">
            <th className="py-2 pr-2">Taşeron</th>
            <th className="py-2 pr-2">Dönem</th>
            <th className="py-2 pr-2">Durum</th>
            {canFin && <th className="py-2 pr-2 text-right">Bu Dönem Brüt</th>}
            {canFin && <th className="py-2 pr-2 text-right">Net Ödenecek</th>}
            <th></th>
          </tr>
        </thead>
        <tbody>
          {payments.map((p) => (
            <tr key={p.id} className="border-b border-beton-800/50 text-beton-200">
              <td className="py-2 pr-2">{subName(p.subcontractor_id)}</td>
              <td className="py-2 pr-2">#{p.period_no}</td>
              <td className="py-2 pr-2">
                <span className={`rounded px-2 py-0.5 text-xs font-semibold ${STATUS_STYLE[p.status] || ""}`}>
                  {STATUS_LABEL[p.status] || p.status}
                </span>
              </td>
              {canFin && <td className="py-2 pr-2 text-right tabular-nums">{p.gross_this?.toLocaleString("tr-TR")}</td>}
              {canFin && <td className="py-2 pr-2 text-right tabular-nums">{p.net_payable?.toLocaleString("tr-TR")}</td>}
              <td className="py-2 text-right">
                <Link to={`/hakedis/${p.id}`} className="text-emniyet-500 hover:underline text-xs font-semibold">Aç →</Link>
              </td>
            </tr>
          ))}
          {!payments.length && (
            <tr><td colSpan={6} className="py-4 text-beton-400 text-center">Hakediş yok.</td></tr>
          )}
        </tbody>
      </table>

      {/* Faz 11 — teminat (geçici kesinti) bakiyesi ve iade akışı.
          Hakediş listesinin altında: iade kararı, hakediş geçmişiyle
          birlikte değerlendirilir. */}
      {can("progress_payments.view_financials") && (
        <div className="mt-6">
          <RetentionPanel projectId={pid!} subFilter={subFilter || null} canRefund={can("progress_payments.finalize")} />
        </div>
      )}
    </div>
  );
}
