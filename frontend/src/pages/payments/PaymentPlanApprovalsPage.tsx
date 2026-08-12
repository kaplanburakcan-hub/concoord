import { useCallback, useEffect, useState } from "react";
import { api } from "../../api/client";
import { useProjects } from "../ProjectContext";

// Nakit Akış Faz C — ödeme planı değişikliği onay akışı. Hakediş/ekstre/PO
// ödemesi sözleşme/tedarikçi varsayılan ödeme şeklinden farklı girildiğinde
// burada bekleyen bir talep olarak listelenir.

type Change = {
  id: string;
  source_entity: string;
  description: string;
  amount: number;
  default_method: string;
  requested_method: string;
  requested_by_name: string;
  status: string;
  created_at: string;
};

const SOURCE_LABEL: Record<string, string> = {
  progress_payment_disbursement: "Hakediş Ödemesi",
  supplier_payment: "Tedarikçi Ekstre Ödemesi",
  po_payment: "Sipariş Ödemesi",
};
const METHOD_LABEL: Record<string, string> = { nakit: "Nakit", havale: "Havale", cek: "Çek" };

export default function PaymentPlanApprovalsPage() {
  const { current } = useProjects();
  const pid = current?.id;
  const [changes, setChanges] = useState<Change[]>([]);
  const [notes, setNotes] = useState<Record<string, string>>({});
  const [busy, setBusy] = useState<string | null>(null);
  const [err, setErr] = useState<string | null>(null);

  const load = useCallback(async () => {
    if (!pid) return;
    setErr(null);
    try {
      const r = await api<{ changes: Change[] }>(
        `/projects/${pid}/payment-plan-changes?status=pending`, { projectId: pid });
      setChanges(r.changes ?? []);
    } catch { setErr("Bekleyen talepler yüklenemedi ya da erişim yetkiniz yok."); }
  }, [pid]);

  useEffect(() => { load(); }, [load]);

  async function decide(id: string, approve: boolean) {
    if (!pid) return;
    const note = notes[id] ?? "";
    if (!approve && !note.trim()) {
      setErr("Ret gerekçesi zorunludur.");
      return;
    }
    setBusy(id);
    setErr(null);
    try {
      await api(`/projects/${pid}/payment-plan-changes/${id}/${approve ? "approve" : "reject"}`, {
        method: "POST", projectId: pid, body: { decision_note: note.trim() },
      });
      setNotes((n) => ({ ...n, [id]: "" }));
      load();
    } catch { setErr("İşlem tamamlanamadı."); } finally { setBusy(null); }
  }

  if (!current) return <p className="text-beton-400">Önce üst bardan bir proje seçin.</p>;

  return (
    <div className="max-w-4xl">
      <h1 className="font-display text-2xl font-extrabold text-white">Ödeme Planı Onayları</h1>
      <p className="mt-1 text-sm text-beton-400">
        Sözleşme/tedarikçi varsayılan ödeme şeklinden farklı girilen ödemeler burada onayınızı bekler.
        Onaylanırsa istenen şekille, reddedilirse varsayılan şekille nakit akışına yansır.
      </p>
      {err && <p className="mt-3 text-sm text-red-400">{err}</p>}

      <div className="mt-4 space-y-3">
        {changes.map((c) => (
          <div key={c.id} className="rounded-lg border border-beton-800 bg-beton-900 p-4">
            <div className="flex items-start justify-between gap-3">
              <div>
                <span className="rounded bg-beton-800 px-2 py-0.5 text-xs text-beton-300">
                  {SOURCE_LABEL[c.source_entity] ?? c.source_entity}
                </span>
                <p className="mt-1 text-white font-medium">{c.description}</p>
                <p className="text-sm text-beton-400 mt-0.5">
                  Talep eden: {c.requested_by_name} ·{" "}
                  {new Date(c.created_at).toLocaleString("tr-TR", { dateStyle: "short", timeStyle: "short" })}
                </p>
              </div>
              <div className="text-right flex-none">
                <div className="text-lg font-bold text-white tabular-nums">{c.amount.toLocaleString("tr-TR")}</div>
                <div className="text-xs text-beton-400 mt-1">
                  Varsayılan: <span className="text-beton-200">{METHOD_LABEL[c.default_method] ?? c.default_method}</span>
                  {" · "}İstenen: <span className="text-amber-400 font-medium">{METHOD_LABEL[c.requested_method] ?? c.requested_method}</span>
                </div>
              </div>
            </div>
            <div className="mt-3 flex flex-wrap items-center gap-2">
              <input
                value={notes[c.id] ?? ""}
                onChange={(e) => setNotes((n) => ({ ...n, [c.id]: e.target.value }))}
                placeholder="Not (ret için zorunlu)"
                className="flex-1 min-w-[200px] rounded bg-beton-950 border border-beton-800 px-2 py-1.5 text-sm text-beton-100 outline-none focus:border-emniyet-500"
              />
              <button
                disabled={busy === c.id}
                onClick={() => decide(c.id, true)}
                className="rounded-md bg-emniyet-500 hover:bg-emniyet-600 disabled:opacity-60 text-beton-950 text-sm font-semibold px-4 py-1.5">
                Onayla
              </button>
              <button
                disabled={busy === c.id}
                onClick={() => decide(c.id, false)}
                className="rounded-md border border-red-500/50 text-red-300 hover:bg-red-500/10 disabled:opacity-60 text-sm px-4 py-1.5">
                Reddet
              </button>
            </div>
          </div>
        ))}
        {!changes.length && !err && (
          <p className="text-beton-500 text-sm text-center py-10">Bekleyen ödeme planı değişikliği yok.</p>
        )}
      </div>
    </div>
  );
}
