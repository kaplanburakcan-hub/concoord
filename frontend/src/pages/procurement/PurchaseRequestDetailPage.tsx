import { useCallback, useEffect, useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { api } from "../../api/client";
import { useAuth } from "../../auth/AuthContext";
import { useProjects } from "../ProjectContext";
import { PR, PR_STATUS_LABEL, PR_STATUS_STYLE } from "./PurchaseRequestsPage";

// Faz 7 — PR detayı: kalemler + iş akışı (onaya gönder → onayla/reddet →
// siparişe dönüştür). Ret gerekçesi zorunlu; dönüşümde PO künyesi istenir.

export default function PurchaseRequestDetailPage() {
  const { id } = useParams();
  const { current } = useProjects();
  const { can } = useAuth();
  const nav = useNavigate();
  const [pr, setPr] = useState<PR | null>(null);
  const [err, setErr] = useState<string | null>(null);
  const [rejectNote, setRejectNote] = useState("");
  const [showReject, setShowReject] = useState(false);
  const [showConvert, setShowConvert] = useState(false);
  const [supplier, setSupplier] = useState("");
  const [amount, setAmount] = useState("");
  const [expected, setExpected] = useState("");
  const pid = current?.id;

  const load = useCallback(async () => {
    if (!pid || !id) return;
    setErr(null);
    try {
      const res = await api<PR>(`/projects/${pid}/purchase-requests/${id}`, { projectId: pid });
      setPr(res);
    } catch { setErr("Talep yüklenemedi."); }
  }, [pid, id]);

  useEffect(() => { load(); }, [load]);

  async function act(path: string, body?: unknown) {
    try {
      await api(`/projects/${pid}/purchase-requests/${id}/${path}`, {
        method: "POST", projectId: pid, body: body ?? {},
      });
      setShowReject(false); setShowConvert(false);
      load();
    } catch (e) {
      setErr(e instanceof Error && e.message ? e.message : "İşlem başarısız.");
    }
  }

  async function remove() {
    if (!confirm("Taslak talep silinsin mi?")) return;
    try {
      await api(`/projects/${pid}/purchase-requests/${id}`, { method: "DELETE", projectId: pid });
      nav("/satinalma");
    } catch { setErr("Silinemedi."); }
  }

  if (!current) return <p className="text-beton-400">Önce üst bardan bir proje seçin.</p>;
  if (!pr) return <p className="text-beton-400">{err ?? "Yükleniyor…"}</p>;

  return (
    <div className="space-y-4 max-w-3xl">
      <div className="flex items-center gap-3">
        <Link to="/satinalma" className="text-xs text-beton-400 hover:text-beton-200">← Talepler</Link>
        <h1 className="text-lg font-display font-bold text-white">{pr.pr_no}</h1>
        <span className={`rounded border px-1.5 py-0.5 text-xs ${PR_STATUS_STYLE[pr.status] ?? ""}`}>
          {PR_STATUS_LABEL[pr.status] ?? pr.status}
        </span>
        {pr.overdue && (
          <span className="rounded border border-red-500/40 bg-red-500/10 px-1.5 py-0.5 text-xs text-red-300">
            ihtiyaç tarihi geçti
          </span>
        )}
      </div>

      {err && <p className="text-sm text-red-400">{err}</p>}

      <div className="rounded-lg border border-beton-800 bg-beton-900 p-4 text-sm text-beton-300 space-y-1">
        <p>İhtiyaç tarihi: <span className="text-beton-100">{pr.needed_by_date}</span></p>
        <p>Talep eden: <span className="text-beton-100">{pr.requested_by_name}</span></p>
        {pr.note && <p>Not: <span className="text-beton-100">{pr.note}</span></p>}
        {pr.decided_by_name && (
          <p>Karar: <span className="text-beton-100">{pr.decided_by_name}</span>
            {pr.decision_note && <> — <span className="text-beton-100">{pr.decision_note}</span></>}
          </p>
        )}
        {pr.po_no && pr.po_id && (
          <p>Bağlı sipariş: <Link to={`/satinalma/siparisler/${pr.po_id}`}
            className="text-emniyet-500 hover:underline">{pr.po_no}</Link></p>
        )}
      </div>

      <div className="overflow-x-auto rounded-lg border border-beton-800">
        <table className="min-w-full text-sm">
          <thead className="bg-beton-900 text-left text-xs text-beton-400">
            <tr>
              <th className="px-3 py-2">Malzeme</th>
              <th className="px-3 py-2">Şartname</th>
              <th className="px-3 py-2 text-right">Miktar</th>
              <th className="px-3 py-2">Birim</th>
              <th className="px-3 py-2">Not</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-beton-800">
            {(pr.items ?? []).map((it) => (
              <tr key={it.id}>
                <td className="px-3 py-2 text-beton-100">{it.material_name}</td>
                <td className="px-3 py-2 text-beton-300">{it.spec ?? "—"}</td>
                <td className="px-3 py-2 text-right text-beton-100">{it.qty}</td>
                <td className="px-3 py-2 text-beton-300">{it.unit}</td>
                <td className="px-3 py-2 text-beton-300">{it.note ?? "—"}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <div className="flex flex-wrap gap-2">
        {pr.status === "Draft" && can("procurement.create_pr") && (
          <>
            <button onClick={() => act("submit")}
              className="rounded-md bg-emniyet-500 px-3 py-1.5 text-xs font-semibold text-beton-950 hover:bg-emniyet-400">
              Onaya Gönder
            </button>
            <button onClick={remove}
              className="rounded-md border border-red-500/40 px-3 py-1.5 text-xs text-red-300 hover:bg-red-500/10">
              Taslağı Sil
            </button>
          </>
        )}
        {pr.status === "Submitted" && can("procurement.approve_pr") && (
          <>
            <button onClick={() => act("approve")}
              className="rounded-md bg-green-600 px-3 py-1.5 text-xs font-semibold text-white hover:bg-green-500">
              Onayla
            </button>
            <button onClick={() => setShowReject((v) => !v)}
              className="rounded-md border border-red-500/40 px-3 py-1.5 text-xs text-red-300 hover:bg-red-500/10">
              Reddet
            </button>
          </>
        )}
        {pr.status === "Approved" && can("procurement.manage_po") && (
          <button onClick={() => setShowConvert((v) => !v)}
            className="rounded-md bg-emniyet-500 px-3 py-1.5 text-xs font-semibold text-beton-950 hover:bg-emniyet-400">
            Siparişe Dönüştür
          </button>
        )}
      </div>

      {showReject && (
        <div className="rounded-lg border border-beton-800 bg-beton-900 p-3 space-y-2">
          <label className="block text-xs text-beton-300">
            Ret gerekçesi (zorunlu)
            <textarea value={rejectNote} onChange={(e) => setRejectNote(e.target.value)} rows={2}
              className="mt-1 block w-full rounded-md bg-beton-950 border border-beton-800 px-2 py-1 text-sm text-beton-100" />
          </label>
          <button disabled={!rejectNote.trim()}
            onClick={() => act("reject", { decision_note: rejectNote.trim() })}
            className="rounded-md bg-red-600 px-3 py-1.5 text-xs font-semibold text-white hover:bg-red-500 disabled:opacity-40">
            Reddi Onayla
          </button>
        </div>
      )}

      {showConvert && (
        <div className="rounded-lg border border-beton-800 bg-beton-900 p-3 space-y-2">
          <div className="flex flex-wrap gap-3">
            <label className="text-xs text-beton-300">
              Tedarikçi
              <input value={supplier} onChange={(e) => setSupplier(e.target.value)}
                className="mt-1 block rounded-md bg-beton-950 border border-beton-800 px-2 py-1 text-sm text-beton-100" />
            </label>
            <label className="text-xs text-beton-300">
              Tutar (opsiyonel)
              <input type="number" min={0} step="any" value={amount} onChange={(e) => setAmount(e.target.value)}
                className="mt-1 block rounded-md bg-beton-950 border border-beton-800 px-2 py-1 text-sm text-beton-100" />
            </label>
            <label className="text-xs text-beton-300">
              Beklenen teslim (opsiyonel)
              <input type="date" value={expected} onChange={(e) => setExpected(e.target.value)}
                className="mt-1 block rounded-md bg-beton-950 border border-beton-800 px-2 py-1 text-sm text-beton-100" />
            </label>
          </div>
          <button disabled={!supplier.trim()}
            onClick={() => act("convert", {
              supplier_name: supplier.trim(),
              amount: amount ? Number(amount) : undefined,
              expected_date: expected || undefined,
            })}
            className="rounded-md bg-emniyet-500 px-3 py-1.5 text-xs font-semibold text-beton-950 hover:bg-emniyet-400 disabled:opacity-40">
            Sipariş Oluştur
          </button>
        </div>
      )}
    </div>
  );
}
