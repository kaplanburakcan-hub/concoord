import { useCallback, useEffect, useState } from "react";
import { api, apiUpload } from "../../api/client";

// Faz 11 — Teminat bakiyesi ve iade paneli.
//
// Geçici nitelikli kesintiler (teminat) hakedişlerde birikir ve kabul
// aşamalarında taşerona İADE EDİLİR. Bakiye sunucuda türetilir
// (kesinleşmiş hakedişlerdeki geçici kesintiler − iadeler), burada yalnızca
// gösterilir; böylece iki kaynak arasında tutarsızlık oluşamaz.

type Balance = {
  subcontractor_id: string;
  company_name: string;
  total_withheld: number;
  total_refunded: number;
  balance: number;
};
type Refund = {
  id: string; description: string; amount: number; stage: string;
  document_id?: string; created_by_name: string; created_at: string;
};

const STAGE_LABEL: Record<string, string> = {
  ProvisionalAcceptance: "Geçici kabul",
  FinalAcceptance: "Kesin kabul",
  ClearanceCertificate: "İlişiksiz belgesi",
  WarrantyEnd: "Garanti süresi sonu",
  Other: "Diğer",
};

export default function RetentionPanel({
  projectId,
  subFilter,
  canRefund,
}: {
  projectId: string;
  subFilter?: string | null;
  canRefund: boolean;
}) {
  const [balances, setBalances] = useState<Balance[]>([]);
  const [sel, setSel] = useState<string | null>(null);
  const [refunds, setRefunds] = useState<Refund[]>([]);
  const [err, setErr] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [f, setF] = useState({ description: "", amount: "", stage: "ProvisionalAcceptance", note: "" });
  const [file, setFile] = useState<File | null>(null);

  const loadBalances = useCallback(async (filter?: string | null) => {
    try {
      const q = filter ? `?subcontractor_id=${filter}` : "";
      const r = await api<{ balances: Balance[] }>(`/projects/${projectId}/retention${q}`, { projectId });
      setBalances(r.balances ?? []);
    } catch {
      setErr("Teminat bakiyeleri yüklenemedi ya da finansal görüntüleme yetkiniz yok.");
    }
  }, [projectId]);

  const loadRefunds = useCallback(async (subId: string) => {
    try {
      const r = await api<{ refunds: Refund[] }>(
        `/projects/${projectId}/subcontractors/${subId}/refunds`, { projectId });
      setRefunds(r.refunds ?? []);
    } catch {
      setRefunds([]);
    }
  }, [projectId]);

  useEffect(() => { loadBalances(subFilter); }, [loadBalances, subFilter]);
  useEffect(() => { if (sel) loadRefunds(sel); }, [sel, loadRefunds]);

  const stageNeedsDoc = f.stage === "ProvisionalAcceptance" || f.stage === "FinalAcceptance";
  const current = balances.find((b) => b.subcontractor_id === sel);

  async function submit() {
    if (!sel) return;
    setBusy(true);
    setErr(null);
    try {
      let documentId: string | undefined;
      if (file) {
        const doc = await api<{ id: string }>(`/projects/${projectId}/documents`, {
          method: "POST", projectId,
          body: { title: `Teminat iadesi — ${STAGE_LABEL[f.stage]}`, doc_category: "Contract" },
        });
        const fd = new FormData();
        fd.append("file", file);
        await apiUpload(`/projects/${projectId}/documents/${doc.id}/versions`, fd);
        documentId = doc.id;
      }
      await api(`/projects/${projectId}/subcontractors/${sel}/refunds`, {
        method: "POST", projectId,
        body: {
          description: f.description.trim(),
          amount: Number(f.amount) || 0,
          stage: f.stage,
          note: f.note.trim() || undefined,
          document_id: documentId ?? "",
        },
      });
      setF({ description: "", amount: "", stage: "ProvisionalAcceptance", note: "" });
      setFile(null);
      await loadBalances(subFilter);
      await loadRefunds(sel);
    } catch (e: any) {
      setErr(e?.message ?? "İade kaydedilemedi.");
    } finally {
      setBusy(false);
    }
  }

  const money = (v: number) => v.toLocaleString("tr-TR", { minimumFractionDigits: 2 });
  const inputCls =
    "w-full rounded-md bg-beton-950 border border-beton-800 px-2.5 py-1.5 text-sm text-beton-100 outline-none focus:border-emniyet-500";

  return (
    <div className="rounded-xl border border-beton-800 bg-beton-900 p-4" style={{ boxShadow: "var(--shadow)" }}>
      <h2 className="font-display font-medium text-beton-100 text-sm uppercase tracking-wide">
        Teminat bakiyesi ve iadeler
      </h2>
      <p className="text-xs text-beton-500 mt-1">
        Geçici kesintiler kabul aşamalarında iade edilir. Bakiye, kesinleşmiş hakedişlerdeki
        teminat kesintilerinden iadeler düşülerek hesaplanır.
      </p>

      {err && <p className="mt-2 text-sm text-red-400">{err}</p>}

      <table className="mt-3 w-full text-sm">
        <thead className="text-beton-400 text-xs">
          <tr className="text-left">
            <th className="py-1 pr-2">Taşeron</th>
            <th className="py-1 pr-2 text-right">Kesilen</th>
            <th className="py-1 pr-2 text-right">İade edilen</th>
            <th className="py-1 pr-2 text-right">Bakiye</th>
          </tr>
        </thead>
        <tbody>
          {balances.map((b) => (
            <tr
              key={b.subcontractor_id}
              onClick={() => setSel(b.subcontractor_id === sel ? null : b.subcontractor_id)}
              className={
                "border-t border-beton-800 cursor-pointer transition " +
                (b.subcontractor_id === sel ? "bg-emniyet-500/10" : "hover:bg-beton-800/40")
              }
            >
              <td className="py-1.5 pr-2 text-beton-200">{b.company_name}</td>
              <td className="py-1.5 pr-2 text-right tabular-nums text-beton-400">{money(b.total_withheld)}</td>
              <td className="py-1.5 pr-2 text-right tabular-nums text-beton-400">{money(b.total_refunded)}</td>
              <td className={"py-1.5 pr-2 text-right tabular-nums font-medium " +
                (b.balance > 0 ? "text-amber-400" : "text-beton-500")}>
                {money(b.balance)}
              </td>
            </tr>
          ))}
          {!balances.length && (
            <tr><td colSpan={4} className="py-3 text-beton-500 text-sm">Teminat kesintisi bulunmuyor.</td></tr>
          )}
        </tbody>
      </table>

      {current && (
        <div className="mt-4 border-t border-beton-800 pt-3">
          <p className="text-xs text-beton-300">
            {current.company_name} — iade geçmişi
            <span className="ml-2 text-amber-400">bakiye {money(current.balance)}</span>
          </p>

          <ul className="mt-2 space-y-1 text-sm">
            {refunds.map((rf) => (
              <li key={rf.id} className="flex items-center gap-2 text-beton-300">
                <span className="text-[10px] px-1.5 py-0.5 rounded border border-beton-700 text-beton-400">
                  {STAGE_LABEL[rf.stage] ?? rf.stage}
                </span>
                <span className="truncate">{rf.description}</span>
                <span className="ml-auto tabular-nums text-green-300">+{money(rf.amount)}</span>
                <span className="text-[11px] text-beton-500">{rf.created_at}</span>
              </li>
            ))}
            {!refunds.length && <li className="text-beton-500">Henüz iade yapılmamış.</li>}
          </ul>

          {canRefund && current.balance > 0 && (
            <div className="mt-3 grid gap-2 sm:grid-cols-2">
              <div>
                <label className="block text-[11px] text-beton-400 mb-1">İade aşaması</label>
                <select className={inputCls} value={f.stage}
                  onChange={(e) => setF({ ...f, stage: e.target.value })}>
                  <option value="ProvisionalAcceptance">Geçici kabul</option>
                  <option value="FinalAcceptance">Kesin kabul</option>
                  <option value="ClearanceCertificate">İlişiksiz belgesi</option>
                  <option value="WarrantyEnd">Garanti süresi sonu</option>
                  <option value="Other">Diğer</option>
                </select>
              </div>
              <div>
                <label className="block text-[11px] text-beton-400 mb-1">
                  İade tutarı (en fazla {money(current.balance)})
                </label>
                <input className={inputCls} inputMode="decimal" value={f.amount}
                  onChange={(e) => setF({ ...f, amount: e.target.value })} />
              </div>
              <div className="sm:col-span-2">
                <label className="block text-[11px] text-beton-400 mb-1">Açıklama</label>
                <input className={inputCls} value={f.description}
                  onChange={(e) => setF({ ...f, description: e.target.value })}
                  placeholder="Ör. Geçici kabul sonrası teminatın %50'sinin iadesi" />
              </div>
              <div className="sm:col-span-2">
                <label className="block text-[11px] text-beton-400 mb-1">
                  Dayanak belge {stageNeedsDoc && <span className="text-red-400">* (kabul tutanağı zorunlu)</span>}
                </label>
                <input type="file" accept="image/*,application/pdf"
                  onChange={(e) => setFile(e.target.files?.[0] ?? null)}
                  className="block w-full text-xs text-beton-400 file:mr-2 file:rounded file:border-0 file:bg-beton-800 file:px-2 file:py-1 file:text-beton-200" />
              </div>
              <div className="sm:col-span-2">
                <button
                  disabled={busy || !f.description.trim() || !(Number(f.amount) > 0) ||
                            (stageNeedsDoc && !file)}
                  onClick={submit}
                  className="rounded-md bg-emniyet-500 hover:bg-emniyet-600 disabled:opacity-50 text-beton-950 text-sm font-medium px-4 py-2">
                  {busy ? "Kaydediliyor…" : "İadeyi Kaydet"}
                </button>
                {stageNeedsDoc && !file && (
                  <span className="ml-3 text-[11px] text-amber-400">
                    Kabul aşamasındaki iadelerde tutanak eklemek zorunludur.
                  </span>
                )}
              </div>
            </div>
          )}
        </div>
      )}
    </div>
  );
}
