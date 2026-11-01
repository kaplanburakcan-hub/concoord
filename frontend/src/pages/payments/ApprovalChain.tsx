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
      <p className="text-[11px] text-beton-400">
        {data.history.filter((h) => h.decision === "Approved").length} / {data.chain.length} adım tamamlandı
      </p>
      {/* Zaman çizelgesi: tamamlanan adım yeşil, sıradaki amber (halkalı), bekleyen gri. */}
      <ol className="mt-2">
        {data.chain.map((s, i) => {
          const rec = decidedBy.get(s.step_no);
          const isNext = data.next_step?.step_no === s.step_no;
          const done = rec?.decision === "Approved";
          const rejected = rec?.decision === "Rejected";
          const last = i === data.chain.length - 1;
          const dot = done ? "bg-green-600" : rejected ? "bg-red-500" : isNext ? "bg-emniyet-500" : "bg-beton-500";
          return (
            <li key={s.step_no} className="relative pl-6 pb-3 text-sm">
              {!last && (
                <span className={"absolute left-[4px] top-3 -bottom-0.5 w-0.5 " + (done ? "bg-green-600" : "bg-beton-800")} />
              )}
              <span
                className={"absolute left-0 top-1 w-2.5 h-2.5 rounded-full " + dot}
                style={isNext && !rec ? { boxShadow: "0 0 0 4px rgb(var(--emniyet-500) / .25)" } : undefined}
              />
              <span className="block font-semibold text-beton-100">{s.label}</span>
              {rec && (
                <span className="block text-[11px] text-beton-400">
                  {rec.actor_name} ·{" "}
                  {new Date(rec.created_at).toLocaleDateString("tr-TR")}
                  {rec.note ? ` · ${rec.note}` : ""}
                </span>
              )}
              {isNext && !rec && <span className="block text-[11px] text-beton-400">bekliyor</span>}
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
