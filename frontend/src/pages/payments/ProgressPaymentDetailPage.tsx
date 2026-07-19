import { useCallback, useEffect, useMemo, useState } from "react";
import type { ReactNode } from "react";
import { useParams, Link } from "react-router-dom";
import { api, apiDownload } from "../../api/client";
import ApprovalChain from "./ApprovalChain";
import { useAuth } from "../../auth/AuthContext";
import { useProjects } from "../ProjectContext";

type Payment = {
  id: string; subcontractor_id: string; period_no: number; status: string;
  period_start?: string; period_end?: string; vat_pct: number; row_version: number;
  gross_cum?: number; gross_prev?: number; gross_this?: number;
  total_deductions?: number; net_payable?: number;
  withholding_applied?: boolean | null;
  vat_amount?: number; vat_withheld?: number; vat_collected?: number;
  payable_gross?: number; actual_cost?: number;
  vat_withholding_ratio?: number; vat_exemption_code?: string | null;
};
type ItemView = {
  work_item_id: string; poz_no: string; description: string; unit: string;
  unit_price?: number; prev_cum_qty: number; this_period_qty: number; cum_qty: number;
  cum_amount?: number; this_amount?: number;
};
type DeductionView = {
  type: string; description: string; rate_pct?: number; amount?: number;
  nature?: string; reduces_cost?: boolean;
  source_entity?: string; source_id?: string;
};
type PenaltySuggestion = {
  penalty_id: string; penalty_no: string; violation_type: string;
  issued_at: string; amount: number; already_added: boolean;
};
type WorkItem = { id: string; poz_no: string; description: string; unit: string; unit_price?: number };

const STATUS_LABEL: Record<string, string> = {
  Draft: "Taslak", Submitted: "Gönderildi", SiteApproved: "Saha Onaylı",
  Finalized: "Kesinleşti", Rejected: "Reddedildi",
};
const DED_LABEL: Record<string, string> = {
  AdvanceOffset: "Avans mahsubu", Retention: "Teminat kesintisi",
  Withholding: "Stopaj (yıllara sari)", Tax: "Vergi", OHSPenalty: "İSG ceza", Other: "Diğer",
};

// Kesinti niteliği (Faz 11): Mahsup / Geçici (iade edilecek) / Kâti.
// Geçici kesintiler kabulde taşerona iade edilir; bu ayrım hem hakediş
// takibinde hem EVM maliyet hesabında belirleyicidir.
const NATURE_LABEL: Record<string, string> = {
  Offset: "Mahsup",
  Temporary: "Geçici · iade edilecek",
  Permanent: "Kâti",
};
const NATURE_CLS: Record<string, string> = {
  Offset: "text-blue-300 border-blue-500/40",
  Temporary: "text-amber-400 border-amber-500/40",
  Permanent: "text-beton-400 border-beton-700",
};

export default function ProgressPaymentDetailPage() {
  const { id } = useParams();
  const { current } = useProjects();
  const { can } = useAuth();
  const pid = current?.id;
  const canFin = can("progress_payments.view_financials");
  const canEdit = can("progress_payments.edit_draft");

  const [pmt, setPmt] = useState<Payment | null>(null);
  const [items, setItems] = useState<ItemView[]>([]);
  const [deductions, setDeductions] = useState<DeductionView[]>([]);
  const [workItems, setWorkItems] = useState<WorkItem[]>([]);
  const [qty, setQty] = useState<Record<string, string>>({});
  const [extras, setExtras] = useState<{
    type: string; description: string; amount: string;
    source_entity?: string; source_id?: string;
  }[]>([]);
  const [suggestions, setSuggestions] = useState<PenaltySuggestion[]>([]);
  const [err, setErr] = useState<string | null>(null);
  const [msg, setMsg] = useState<string | null>(null);

  const load = useCallback(async () => {
    if (!pid || !id) return;
    setErr(null);
    try {
      const r = await api<{
        payment: Payment; items: ItemView[]; deductions: DeductionView[];
        ohs_penalty_suggestions?: PenaltySuggestion[];
      }>(`/projects/${pid}/payments/${id}`, { projectId: pid });
      setSuggestions(r.ohs_penalty_suggestions ?? []);
      setPmt(r.payment);
      setItems(r.items);
      setDeductions(r.deductions);
      const q: Record<string, string> = {};
      r.items.forEach((it) => { q[it.work_item_id] = String(it.cum_qty); });
      setQty(q);
      setExtras(r.deductions
        .filter((d) => d.type !== "AdvanceOffset" && d.type !== "Retention")
        .map((d) => ({
          type: d.type, description: d.description, amount: String(d.amount ?? 0),
          source_entity: d.source_entity, source_id: d.source_id,
        })));
      const wi = await api<{ work_items: WorkItem[] }>(
        `/projects/${pid}/subcontractors/${r.payment.subcontractor_id}/work-items`, { projectId: pid });
      setWorkItems(wi.work_items);
    } catch { setErr("Hakediş yüklenemedi ya da erişim yetkiniz yok."); }
  }, [pid, id]);

  useEffect(() => { load(); }, [load]);

  const editable = pmt?.status === "Draft" && canEdit;

  async function save() {
    if (!pmt) return;
    setMsg(null); setErr(null);
    const body = {
      row_version: pmt.row_version,
      withholding_applied: pmt.withholding_applied ?? null,
      vat_withholding_ratio: pmt.vat_withholding_ratio ?? null,
      items: workItems
        .filter((w) => qty[w.id] !== undefined && qty[w.id] !== "")
        .map((w) => ({ work_item_id: w.id, cum_qty: Number(qty[w.id]) || 0 })),
      deductions: extras
        .filter((e) => e.type && Number(e.amount))
        .map((e) => ({
          type: e.type, description: e.description, amount: Number(e.amount),
          source_entity: e.source_entity, source_id: e.source_id,
        })),
    };
    try {
      await api(`/projects/${pid}/payments/${id}`, { method: "PATCH", projectId: pid, body });
      setMsg("Hesaplandı ve kaydedildi.");
      load();
    } catch { setErr("Kaydedilemedi. Durum taslak olmalı ve satır sürümü güncel olmalı."); }
  }

  async function doTransition(action: string, label: string) {
    if (!pmt) return;
    setMsg(null); setErr(null);
    try {
      await api(`/projects/${pid}/payments/${id}/${action}`, {
        method: "POST", projectId: pid, body: { row_version: pmt.row_version },
      });
      setMsg(`${label} tamamlandı.`);
      load();
    } catch { setErr(`${label} başarısız (durum geçişi veya yetki uygun değil).`); }
  }

  async function downloadPdf() {
    try {
      await apiDownload(`/projects/${pid}/payments/${id}/summary.pdf`, `hakedis-${id}.pdf`);
    } catch { setErr("PDF indirilemedi (finansal görüntüleme yetkisi gerekli)."); }
  }

  const vatAmount = useMemo(() => {
    // Faz 11: KDV artık sunucuda dönem brütü üzerinden hesaplanıyor.
    return pmt?.vat_amount ?? 0;
  }, [pmt]);

  if (!current) return <p className="text-beton-400">Önce bir proje seçin.</p>;
  if (!pmt) return <p className="text-beton-400">{err || "Yükleniyor…"}</p>;

  return (
    <div>
      <div className="flex items-center gap-3">
        <Link to="/hakedis" className="text-beton-400 hover:text-white text-sm">← Hakedişler</Link>
      </div>
      <h1 className="font-display text-2xl font-extrabold text-white mt-1">
        Hakediş #{pmt.period_no}
        <span className="ml-3 rounded bg-beton-800 px-2 py-0.5 text-sm text-beton-200 align-middle">
          {STATUS_LABEL[pmt.status] || pmt.status}
        </span>
      </h1>
      {err && <p className="mt-3 text-sm text-red-400">{err}</p>}
      {msg && <p className="mt-3 text-sm text-emniyet-500">{msg}</p>}

      {/* Metraj tablosu */}
      <div className="mt-4 rounded-lg border border-beton-800 bg-beton-900 p-3">
        <h2 className="font-display font-bold text-white text-sm uppercase tracking-wide">İmalat Kalemleri (Kümülatif Metraj)</h2>
        <table className="mt-2 w-full text-sm">
          <thead>
            <tr className="text-beton-400 text-left border-b border-beton-800">
              <th className="py-1 pr-2">Poz</th>
              <th className="py-1 pr-2">Açıklama</th>
              <th className="py-1 pr-2">Birim</th>
              {canFin && <th className="py-1 pr-2 text-right">B. Fiyat</th>}
              <th className="py-1 pr-2 text-right">Önceki Küm.</th>
              <th className="py-1 pr-2 text-right">Kümülatif</th>
              <th className="py-1 pr-2 text-right">Bu Dönem</th>
              {canFin && <th className="py-1 pr-2 text-right">Bu Dönem Tutar</th>}
            </tr>
          </thead>
          <tbody>
            {workItems.map((w) => {
              const iv = items.find((it) => it.work_item_id === w.id);
              const prev = iv?.prev_cum_qty ?? 0;
              const cur = Number(qty[w.id] ?? iv?.cum_qty ?? 0);
              const thisQty = cur - prev;
              return (
                <tr key={w.id} className="border-b border-beton-800/50 text-beton-200">
                  <td className="py-1 pr-2 font-mono">{w.poz_no}</td>
                  <td className="py-1 pr-2">{w.description}</td>
                  <td className="py-1 pr-2">{w.unit}</td>
                  {canFin && <td className="py-1 pr-2 text-right tabular-nums">{w.unit_price?.toLocaleString("tr-TR")}</td>}
                  <td className="py-1 pr-2 text-right tabular-nums">{prev}</td>
                  <td className="py-1 pr-2 text-right">
                    {editable ? (
                      <input
                        value={qty[w.id] ?? ""} inputMode="decimal"
                        onChange={(e) => setQty({ ...qty, [w.id]: e.target.value })}
                        className="w-24 rounded bg-beton-950 border border-beton-800 px-2 py-1 text-right text-white"
                      />
                    ) : (
                      <span className="tabular-nums">{iv?.cum_qty ?? 0}</span>
                    )}
                  </td>
                  <td className="py-1 pr-2 text-right tabular-nums">{thisQty}</td>
                  {canFin && <td className="py-1 pr-2 text-right tabular-nums">{iv?.this_amount?.toLocaleString("tr-TR") ?? "—"}</td>}
                </tr>
              );
            })}
            {!workItems.length && (
              <tr><td colSpan={8} className="py-3 text-beton-400 text-center">Bu taşeronun birim fiyat cetveli boş.</td></tr>
            )}
          </tbody>
        </table>
      </div>

      {/* Kesintiler + özet */}
      <div className="mt-4 grid gap-4 md:grid-cols-2">
        <div className="rounded-lg border border-beton-800 bg-beton-900 p-3">
          <h2 className="font-display font-bold text-white text-sm uppercase tracking-wide">Kesintiler</h2>
          <p className="text-xs text-beton-400 mt-1">Avans ve teminat otomatik hesaplanır. Aşağıya vergi/İSG/diğer ekleyin.</p>

          {/* Faz 11 — Stopaj (GVK Md. 42-44, yıllara sari inşaat işleri).
              Aynı takvim yılında başlayıp biten işlerde kesilmez; sonraki yıla
              sarkan işlerde kesilir. Varsayılan sözleşmenin "yıllara sari"
              işaretinden gelir, burada hakediş bazında geçersiz kılınabilir. */}
          {editable && (
            <label className="mt-3 flex items-start gap-2 rounded-md border border-beton-800 bg-beton-950 p-2 cursor-pointer">
              <input
                type="checkbox"
                className="mt-0.5 accent-emniyet-500"
                checked={!!pmt.withholding_applied}
                onChange={(e) =>
                  setPmt({ ...pmt, withholding_applied: e.target.checked })
                }
              />
              <span className="text-xs">
                <span className="text-beton-200">Stopaj kesintisi uygula</span>
                <span className="block text-beton-500 mt-0.5">
                  Yıllara sari işlerde zorunlu. Kaydettiğinizde hesaba dahil edilir.
                </span>
              </span>
            </label>
          )}
          <ul className="mt-2 space-y-1 text-sm">
            {deductions.map((d, i) => (
              <li key={i} className="flex items-center justify-between gap-2 text-beton-200">
                <span className="flex items-center gap-2 min-w-0">
                  <span className="truncate">
                    {DED_LABEL[d.type] || d.type}{d.description ? ` — ${d.description}` : ""}
                  </span>
                  {d.nature && (
                    <span
                      className={
                        "flex-none text-[10px] px-1.5 py-0.5 rounded border " +
                        (NATURE_CLS[d.nature] || "text-beton-400 border-beton-700")
                      }
                      title={
                        d.reduces_cost
                          ? "Maliyeti azaltır (mal/hizmet bedeli) — EVM AC hesabından düşülür"
                          : "Maliyeti azaltmaz (finansman/vergi kalemi)"
                      }
                    >
                      {NATURE_LABEL[d.nature] || d.nature}
                    </span>
                  )}
                </span>
                {canFin && <span className="tabular-nums flex-none">{d.amount?.toLocaleString("tr-TR")}</span>}
              </li>
            ))}
            {!deductions.length && <li className="text-beton-400">Kesinti yok.</li>}
          </ul>

          {/* Faz 8: bekleyen İSG para cezaları → kesinti önerisi (PY onayıyla
              taslağa eklenir; source_id ile tutanağa izlenebilir bağ kurulur). */}
          {editable && suggestions.length > 0 && (
            <div className="mt-3 rounded-md border border-red-500/30 bg-red-500/5 p-2 space-y-1">
              <p className="text-xs font-semibold text-red-300">
                Bekleyen İSG ceza kesintisi önerileri
              </p>
              {suggestions.map((s) => {
                const added = s.already_added ||
                  extras.some((e) => e.source_id === s.penalty_id);
                return (
                  <div key={s.penalty_id} className="flex items-center gap-2 text-xs text-beton-200">
                    <span>{s.penalty_no} — {s.violation_type} ({s.issued_at})</span>
                    <span className="tabular-nums">{s.amount.toLocaleString("tr-TR")} TL</span>
                    {added ? (
                      <span className="ml-auto text-green-300">eklendi</span>
                    ) : (
                      <button
                        onClick={() => setExtras([...extras, {
                          type: "OHSPenalty",
                          description: `${s.penalty_no} — ${s.violation_type}`,
                          amount: String(s.amount),
                          source_entity: "ohs_penalties",
                          source_id: s.penalty_id,
                        }])}
                        className="ml-auto text-emniyet-500 hover:underline">
                        Kesintiye ekle
                      </button>
                    )}
                  </div>
                );
              })}
              <p className="text-[10px] text-beton-500">
                Ekledikten sonra “Hesapla ve Kaydet”; hakediş kesinleşince tutanak otomatik
                “Hakedişe Uygulandı” olur.
              </p>
            </div>
          )}

          {editable && (
            <div className="mt-3 border-t border-beton-800 pt-3 space-y-2">
              {extras.map((e, i) => (
                <div key={i} className="grid grid-cols-[110px_1fr_100px_auto] gap-2">
                  <select value={e.type}
                    onChange={(ev) => setExtras(extras.map((x, j) => j === i ? { ...x, type: ev.target.value } : x))}
                    className="rounded bg-beton-950 border border-beton-800 px-2 py-1 text-sm text-white">
                    <option value="Tax">Vergi</option>
                    <option value="OHSPenalty">İSG ceza</option>
                    <option value="Other">Diğer</option>
                  </select>
                  <input value={e.description} placeholder="Açıklama"
                    onChange={(ev) => setExtras(extras.map((x, j) => j === i ? { ...x, description: ev.target.value } : x))}
                    className="rounded bg-beton-950 border border-beton-800 px-2 py-1 text-sm text-white" />
                  <input value={e.amount} placeholder="Tutar" inputMode="decimal"
                    onChange={(ev) => setExtras(extras.map((x, j) => j === i ? { ...x, amount: ev.target.value } : x))}
                    className="rounded bg-beton-950 border border-beton-800 px-2 py-1 text-sm text-white text-right" />
                  <button onClick={() => setExtras(extras.filter((_, j) => j !== i))}
                    className="text-red-400 hover:text-red-300 text-xs px-1">Sil</button>
                </div>
              ))}
              <button onClick={() => setExtras([...extras, { type: "Tax", description: "", amount: "" }])}
                className="text-emniyet-500 hover:underline text-xs">+ Kesinti satırı ekle</button>
            </div>
          )}
        </div>

        <div className="rounded-lg border border-beton-800 bg-beton-900 p-3">
          {/* Faz 11 — çok adımlı onay zinciri (teknik ofis → ... → finansal kontrol) */}
          <h2 className="font-display font-bold text-white text-sm uppercase tracking-wide">Onay Zinciri</h2>
          <div className="mt-2">
            {id && <ApprovalChain paymentId={id} onChanged={load} />}
          </div>
        </div>

        <div className="rounded-lg border border-beton-800 bg-beton-900 p-3">
          <h2 className="font-display font-bold text-white text-sm uppercase tracking-wide">Özet</h2>
          {canFin ? (
            <dl className="mt-2 space-y-1 text-sm">
              <Row label="A) Kümülatif imalat" v={pmt.gross_cum} />
              <Row label="B) Önceki dönem" v={pmt.gross_prev} />
              <Row label="C) Bu dönem brüt" v={pmt.gross_this} bold />
              <Row label="Toplam kesinti" v={pmt.total_deductions} />
              {/* Faz 11 — gerçek hakediş akışı:
                  brüt → KDV ekle (tevkifat düş) → kesintileri KDV dahil düş */}
              <Row label={`KDV (%${pmt.vat_pct})`} v={pmt.vat_amount} />
              {(pmt.vat_withheld ?? 0) > 0 && (
                <Row label="Tevkif edilen KDV (vergi dairesine)" v={pmt.vat_withheld} />
              )}
              <Row label="Tahsil edilen KDV" v={pmt.vat_collected} />
              <Row label="Ödenebilir toplam (KDV dahil)" v={pmt.payable_gross} bold />
              <Row label="Kesintiler toplamı (KDV dahil)" v={pmt.total_deductions} />
              <Row label="Yükleniciye ödenecek" v={pmt.net_payable} bold />
              {(pmt.actual_cost ?? 0) > 0 && (
                <Row label="Gerçekleşen maliyet (EVM · KDV hariç)" v={pmt.actual_cost} />
              )}
            </dl>
          ) : (
            <p className="mt-2 text-sm text-beton-400">Finansal değerleri görüntüleme yetkiniz yok (yalnızca metraj).</p>
          )}
        </div>
      </div>

      {/* Aksiyonlar */}
      <div className="mt-4 flex flex-wrap gap-2">
        {editable && (
          <button onClick={save}
            className="rounded bg-emniyet-500 hover:bg-emniyet-600 text-beton-950 font-semibold text-sm px-4 py-2">
            Kaydet & Hesapla
          </button>
        )}
        {pmt.status === "Draft" && can("progress_payments.submit") && (
          <ActionBtn onClick={() => doTransition("submit", "Gönderim")}>Saha Onayına Gönder</ActionBtn>
        )}
        {pmt.status === "Submitted" && can("progress_payments.approve") && (
          <>
            <ActionBtn onClick={() => doTransition("approve", "Saha onayı")}>Saha Onayı Ver</ActionBtn>
            <ActionBtn danger onClick={() => doTransition("reject", "Reddetme")}>Reddet</ActionBtn>
          </>
        )}
        {pmt.status === "SiteApproved" && can("progress_payments.finalize") && (
          <ActionBtn onClick={() => doTransition("finalize", "Kesinleştirme")}>Kesinleştir (Kilitle)</ActionBtn>
        )}
        {canFin && (
          <button onClick={downloadPdf}
            className="rounded border border-beton-700 text-beton-200 hover:bg-beton-800 text-sm px-4 py-2">
            Özet PDF
          </button>
        )}
      </div>
    </div>
  );
}

function Row({ label, v, bold }: { label: string; v?: number; bold?: boolean }) {
  return (
    <div className={`flex justify-between ${bold ? "text-white font-semibold" : "text-beton-200"}`}>
      <dt>{label}</dt>
      <dd className="tabular-nums">{(v ?? 0).toLocaleString("tr-TR", { minimumFractionDigits: 2 })}</dd>
    </div>
  );
}

function ActionBtn({ children, onClick, danger }: { children: ReactNode; onClick: () => void; danger?: boolean }) {
  return (
    <button onClick={onClick}
      className={`rounded text-sm font-semibold px-4 py-2 ${
        danger ? "bg-red-500/20 text-red-300 hover:bg-red-500/30" : "bg-beton-800 text-beton-100 hover:bg-beton-700"
      }`}>
      {children}
    </button>
  );
}
