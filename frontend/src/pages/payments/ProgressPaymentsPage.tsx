import { useCallback, useEffect, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { api, describeLoadError } from "../../api/client";
import { useAuth } from "../../auth/AuthContext";
import { exportXlsx } from "../../lib/exportXlsx";
import { useProjects } from "../ProjectContext";
import RetentionPanel from "./RetentionPanel";
import PaymentPreview from "./PaymentPreview";

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
  const nav = useNavigate();
  const [subs, setSubs] = useState<Sub[]>([]);
  const [payments, setPayments] = useState<Payment[]>([]);
  const [subFilter, setSubFilter] = useState<string>("");
  const [newSub, setNewSub] = useState<string>("");
  const [creating, setCreating] = useState(false);
  const [err, setErr] = useState<string | null>(null);
  // Listede seçili hakediş: altında özet bandı, oranlar ve onay zinciri görünür.
  const [selectedId, setSelectedId] = useState<string | null>(null);
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
    } catch (e) { setErr(describeLoadError(e, "Hakedişler")); }
  }, [pid, subFilter]);

  useEffect(() => { load(); }, [load]);

  // Seçim listede yoksa (ilk yükleme, filtre değişimi) ilk satıra döner.
  useEffect(() => {
    if (!payments.length) { setSelectedId(null); return; }
    if (!selectedId || !payments.some((p) => p.id === selectedId)) setSelectedId(payments[0].id);
  }, [payments, selectedId]);
  const selected = payments.find((p) => p.id === selectedId);

  async function createDraft() {
    if (!newSub) return;
    setErr(null);
    setCreating(true);
    try {
      const res = await api<{ payment: { id: string } }>(
        `/projects/${pid}/payments`, { method: "POST", projectId: pid, body: { subcontractor_id: newSub } }
      );
      setNewSub("");
      // Taslak oluşur oluşmaz doğrudan giriş ekranına geç — sadece listeye
      // dönmek kullanıcıya "hiçbir şey olmadı" hissi veriyordu.
      nav(`/hakedis/${res.payment.id}`);
    } catch {
      setErr("Taslak oluşturulamadı.");
    } finally {
      setCreating(false);
    }
  }

  if (!current) return <p className="text-beton-400">Önce üst bardan bir proje seçin.</p>;

  return (
    <div>
      <h1 className="font-display text-2xl font-bold tracking-tight text-beton-100">Hakedişler</h1>
      <p className="mt-1 text-sm text-beton-400">{current.name} — kümülatif hakediş iş akışı ve kesinti yönetimi.</p>
      <button onClick={() => exportXlsx("hakedisler", "Hakedişler", [
          { header: "Dönem", value: (p: Payment) => p.period_no },
          { header: "Durum", value: (p: Payment) => STATUS_LABEL[p.status] ?? p.status },
          { header: "Dönem başı", value: (p: Payment) => p.period_start ?? "" },
          { header: "Dönem sonu", value: (p: Payment) => p.period_end ?? "" },
          { header: "Brüt", value: (p: Payment) => p.gross_this ?? "" },
          { header: "Net ödenecek", value: (p: Payment) => p.net_payable ?? "" },
        ], payments)}
        disabled={payments.length === 0}
        className="mt-2 rounded-md border border-beton-700 px-3 py-1.5 text-xs text-beton-200 hover:border-emniyet-500 disabled:opacity-50">
        Excel indir
      </button>
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
              onClick={createDraft} disabled={!newSub || creating}
              className="rounded bg-emniyet-500 hover:bg-emniyet-600 text-beton-950 font-semibold text-sm px-3 py-1.5 disabled:opacity-50"
            >
              {creating ? "Oluşturuluyor…" : "Yeni Taslak Hakediş"}
            </button>
          </div>
        )}
      </div>

      <div className="mt-4 rounded-lg border border-beton-800 bg-beton-900 overflow-x-auto">
        <table className="w-full text-sm">
          <thead>
            <tr className="text-beton-400 text-left border-b border-beton-800 text-[11px] uppercase tracking-wider">
              <th className="py-2 px-3">Taşeron</th>
              <th className="py-2 px-3">Dönem</th>
              <th className="py-2 px-3">Durum</th>
              {canFin && <th className="py-2 px-3 text-right">Bu Dönem Brüt</th>}
              {canFin && <th className="py-2 px-3 text-right">Net Ödenecek</th>}
              <th></th>
            </tr>
          </thead>
          <tbody>
            {payments.map((p) => (
              <tr key={p.id} onClick={() => setSelectedId(p.id)}
                aria-selected={p.id === selectedId}
                className={"border-b border-beton-800/50 text-beton-200 cursor-pointer " +
                  (p.id === selectedId ? "bg-emniyet-500/10" : "hover:bg-beton-800/40")}>
                <td className="py-2 px-3">{subName(p.subcontractor_id)}</td>
                <td className="py-2 px-3">#{p.period_no}</td>
                <td className="py-2 px-3">
                  <span className={`rounded px-2 py-0.5 text-xs font-semibold ${STATUS_STYLE[p.status] || ""}`}>
                    {STATUS_LABEL[p.status] || p.status}
                  </span>
                </td>
                {canFin && <td className="py-2 px-3 text-right tabular-nums">{p.gross_this?.toLocaleString("tr-TR")}</td>}
                {canFin && <td className="py-2 px-3 text-right tabular-nums">{p.net_payable?.toLocaleString("tr-TR")}</td>}
                <td className="py-2 px-3 text-right">
                  <Link to={`/hakedis/${p.id}`} onClick={(e) => e.stopPropagation()}
                    className="text-emniyet-500 hover:underline text-xs font-semibold">Aç →</Link>
                </td>
              </tr>
            ))}
            {!payments.length && (
              <tr><td colSpan={6} className="py-4 text-beton-400 text-center">Hakediş yok.</td></tr>
            )}
          </tbody>
        </table>
      </div>

      {selected && (
        <PaymentPreview
          key={selected.id}
          projectId={pid!} paymentId={selected.id}
          subName={subName(selected.subcontractor_id)}
          canFin={canFin} onChanged={load}
        />
      )}

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
