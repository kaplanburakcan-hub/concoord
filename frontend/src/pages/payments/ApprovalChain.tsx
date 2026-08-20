import { useEffect, useState } from "react";
import { api } from "../../api/client";

// Faz 11 — Çok adımlı hakediş onay zinciri.
// Zincir sunucudan gelir (proje bazında tanımlanabilir); burada yalnızca
// görselleştirilir ve sıradaki adım onaylanır/reddedilir.

type Step = { step_no: number; code: string; label: string; permission: string };
type Record = {
  step_no: number;
  step_code: string;
  label?: string;
  decision: "Approved" | "Rejected";
  note?: string | null;
  actor_name: string;
  created_at: string;
};
type ChainResp = {
  chain: Step[];
  history: Record[];
  current_step_no: number;
  next_step: Step | null;
  status: string;
};

export default function ApprovalChain({
  paymentId,
  onChanged,
}: {
  paymentId: string;
  onChanged?: () => void;
}) {
  const [data, setData] = useState<ChainResp | null>(null);
  const [note, setNote] = useState("");
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  async function load() {
    try {
      setData(await api<ChainResp>(`/payments/${paymentId}/approvals`));
    } catch (e: any) {
      setErr(e?.message ?? "Onay zinciri yüklenemedi.");
    }
  }
  useEffect(() => {
    load();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [paymentId]);

  async function decide(decision: "Approved" | "Rejected") {
    if (decision === "Rejected" && !note.trim()) {
      setErr("Ret gerekçesi zorunludur.");
      return;
    }
    setBusy(true);
    setErr(null);
    try {
      await api(`/payments/${paymentId}/approvals`, {
        method: "POST",
        body: { decision, note: note.trim() },
      });
      setNote("");
      await load();
      onChanged?.();
    } catch (e: any) {
      setErr(e?.message ?? "İşlem tamamlanamadı.");
    } finally {
      setBusy(false);
    }
  }

  if (!data) {
    return <p className="text-sm text-beton-400">Onay zinciri yükleniyor…</p>;
  }

  const decidedBy = new Map<number, Record>();
  for (const h of data.history) decidedBy.set(h.step_no, h);

  return (
    <div>
      <ol className="space-y-1.5">
        {data.chain.map((s) => {
          const rec = decidedBy.get(s.step_no);
          const isNext = data.next_step?.step_no === s.step_no;
          const done = rec?.decision === "Approved";
          const rejected = rec?.decision === "Rejected";
          return (
            <li
              key={s.step_no}
              className={
                "flex items-start gap-3 rounded-md border px-3 py-2 text-sm " +
                (isNext
                  ? "border-emniyet-500 bg-emniyet-500/10"
                  : "border-beton-800 bg-beton-950")
              }
            >
              <span
                className={
                  "mt-0.5 flex-none w-5 h-5 rounded-full grid place-items-center text-[11px] font-medium " +
                  (done
                    ? "bg-green-500/20 text-green-300"
                    : rejected
                    ? "bg-red-500/20 text-red-300"
                    : isNext
                    ? "bg-emniyet-500 text-beton-950"
                    : "bg-beton-800 text-beton-400")
                }
              >
                {done ? "✓" : rejected ? "✕" : s.step_no}
              </span>
              <span className="min-w-0 flex-1">
                <span className="text-beton-200">{s.label}</span>
                {rec && (
                  <span className="block text-xs text-beton-500 mt-0.5">
                    {rec.actor_name} ·{" "}
                    {new Date(rec.created_at).toLocaleString("tr-TR", {
                      dateStyle: "short",
                      timeStyle: "short",
                    })}
                    {rec.note ? ` · ${rec.note}` : ""}
                  </span>
                )}
                {isNext && !rec && (
                  <span className="block text-xs text-emniyet-500 mt-0.5">Onayınız bekleniyor</span>
                )}
              </span>
            </li>
          );
        })}
      </ol>

      {data.next_step && (
        <div className="mt-3">
          <input
            value={note}
            onChange={(e) => setNote(e.target.value)}
            placeholder="Not (ret için zorunlu)"
            className="w-full rounded-md bg-beton-950 border border-beton-800 px-3 py-2 text-sm text-beton-100 outline-none focus:border-emniyet-500"
          />
          <div className="mt-2 flex gap-2">
            <button
              disabled={busy}
              onClick={() => decide("Approved")}
              className="rounded-md bg-emniyet-500 hover:bg-emniyet-600 disabled:opacity-60 text-beton-950 text-sm font-medium px-4 py-2"
            >
              {data.next_step.label} — Onayla
            </button>
            <button
              disabled={busy}
              onClick={() => decide("Rejected")}
              className="rounded-md border border-red-500/50 text-red-300 hover:bg-red-500/10 disabled:opacity-60 text-sm px-4 py-2"
            >
              Reddet
            </button>
          </div>
        </div>
      )}

      {data.status === "Finalized" && (
        <p className="mt-3 text-xs text-green-300">
          Zincir tamamlandı — hakediş kesinleşti ve değiştirilemez.
        </p>
      )}
      {data.status === "Rejected" && (
        <p className="mt-3 text-xs text-red-300">Hakediş reddedildi; taslağa geri çekilebilir.</p>
      )}
      {err && <p className="mt-2 text-sm text-red-400">{err}</p>}
    </div>
  );
}
