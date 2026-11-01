import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { api, apiFetchBlob } from "../../api/client";
import ApprovalChain from "./ApprovalChain";
import PaymentHero from "./PaymentHero";

// Hakedişler listesinde seçili satırın özeti: koyu özet bandı, hakediş oranları
// (salt okunur) ve onay zinciri. Düzenleme için hakediş detay sayfası açılır.

type Pmt = {
  id: string; period_no: number; status: string; period_start?: string; period_end?: string;
  vat_pct: number; vat_withholding_ratio?: number;
  gross_cum?: number; gross_prev?: number; gross_this?: number; net_payable?: number;
};
type Rates = {
  is_turu: string; contract_found: boolean;
  effective: { vat_pct: number; vat_withholding_ratio: number; retention_pct: number; withholding_pct: number };
};

const STATUS_LABEL: Record<string, string> = {
  Draft: "Taslak", Submitted: "Gönderildi", SiteApproved: "Saha Onaylı",
  Finalized: "Kesinleşti", Rejected: "Reddedildi",
};
const IS_TURU_LABEL: Record<string, string> = { yapim: "Yapım", hizmet: "Hizmet", kiralama: "Kiralama", diger: "Diğer" };
const rateTxt = (v: number) => String(v).replace(".", ",");
const tevkifatTxt = (v: number) => (v === 0 ? "Tevkifat yok" : v === 1 ? "Tam tevkifat" : `${Math.round(v * 10)}/10`);
const fmtDate = (d: string) => {
  const t = new Date(d);
  return isNaN(t.getTime()) ? d : t.toLocaleDateString("tr-TR");
};

export default function PaymentPreview({
  projectId, paymentId, subName, canFin, onChanged,
}: { projectId: string; paymentId: string; subName: string; canFin: boolean; onChanged: () => void }) {
  const [pmt, setPmt] = useState<Pmt | null>(null);
  const [rates, setRates] = useState<Rates | null>(null);
  const [contract, setContract] = useState<number | null>(null);
  const [contractNo, setContractNo] = useState("");
  const [err, setErr] = useState<string | null>(null);

  useEffect(() => {
    let alive = true;
    setPmt(null); setRates(null); setContract(null); setContractNo(""); setErr(null);
    (async () => {
      try {
        const r = await api<{ payment: Pmt & { subcontractor_id: string }; rates?: Rates }>(
          `/projects/${projectId}/payments/${paymentId}`, { projectId });
        if (!alive) return;
        setPmt(r.payment);
        setRates(r.rates ?? null);
        try {
          const cs = await api<{ contracts: { amount?: number | null; contract_no?: string }[] }>(
            `/projects/${projectId}/contracts?subcontractor_id=${r.payment.subcontractor_id}`, { projectId });
          if (!alive) return;
          const total = cs.contracts.reduce((sum, c) => sum + (c.amount ?? 0), 0);
          setContract(total > 0 ? total : null);
          setContractNo(cs.contracts.find((c) => c.contract_no)?.contract_no ?? "");
        } catch { /* sözleşme yoksa bant bedelsiz gösterilir */ }
      } catch { if (alive) setErr("Hakediş özeti yüklenemedi."); }
    })();
    return () => { alive = false; };
  }, [projectId, paymentId]);

  async function viewPdf() {
    const w = window.open("", "_blank");
    let layout = "resmi";
    try {
      const v = localStorage.getItem("hakedis.pdfLayout");
      if (v === "panel" || v === "akis") layout = v;
    } catch { /* varsayılan düzen */ }
    try {
      const url = await apiFetchBlob(`/projects/${projectId}/payments/${paymentId}/summary.pdf?layout=${layout}`);
      if (w) w.location.href = url; else window.location.href = url;
    } catch {
      w?.close();
      setErr("PDF görüntülenemedi (finansal görüntüleme yetkisi gerekli).");
    }
  }

  if (err && !pmt) return <p className="mt-4 text-sm text-red-400">{err}</p>;
  if (!pmt) return <p className="mt-4 text-sm text-beton-400">Hakediş özeti yükleniyor…</p>;

  const period = pmt.period_start && pmt.period_end ? `${fmtDate(pmt.period_start)} – ${fmtDate(pmt.period_end)}` : "";
  const field = (label: string, value: string) => (
    <div>
      <div className="text-[11px] text-beton-400">{label}</div>
      <div className="mt-0.5 rounded border border-beton-800 bg-beton-950 px-2 py-1.5 text-sm text-beton-100 text-right tabular-nums">{value}</div>
    </div>
  );

  return (
    <div className="mt-4 space-y-4">
      <PaymentHero
        title={`Hakediş #${pmt.period_no} · ${subName || "Taşeron"}`}
        subtitle={[contractNo, period].filter(Boolean).join(" · ")}
        status={<span className="rounded px-2.5 py-0.5 text-xs font-bold" style={{ background: "#e0b400", color: "#0b2545" }}>{STATUS_LABEL[pmt.status] || pmt.status}</span>}
        contract={contract} cum={pmt.gross_cum} prev={pmt.gross_prev} thisGross={pmt.gross_this} payable={pmt.net_payable}
        canFin={canFin}
      />

      <div className="grid gap-4 md:grid-cols-[1.1fr_1fr]">
        <div className="rounded-lg border border-beton-800 bg-beton-900 p-3">
          <h2 className="font-display text-sm font-bold uppercase tracking-wide text-beton-100">
            Hakediş oranları
            {rates && <span className="ml-2 normal-case font-normal text-beton-400">· {IS_TURU_LABEL[rates.is_turu] ?? rates.is_turu} sözleşmesi</span>}
          </h2>
          {rates ? (
            <div className="mt-3 grid grid-cols-2 gap-3">
              {field("KDV oranı %", rateTxt(rates.effective.vat_pct))}
              {field("KDV tevkifat oranı", tevkifatTxt(rates.effective.vat_withholding_ratio))}
              {field("Teminat kesintisi %", rateTxt(rates.effective.retention_pct))}
              {field("Stopaj oranı %", rateTxt(rates.effective.withholding_pct))}
            </div>
          ) : (
            <p className="mt-2 text-sm text-beton-400">Oran bilgisi yok.</p>
          )}
          <div className="mt-4 flex flex-wrap gap-2">
            <Link to={`/hakedis/${pmt.id}`}
              className="rounded bg-emniyet-500 hover:bg-emniyet-600 text-beton-950 font-semibold text-sm px-4 py-2">
              {pmt.status === "Draft" ? "Düzenle →" : "Hakedişi aç →"}
            </Link>
            {canFin && (
              <button onClick={viewPdf}
                className="rounded border border-beton-700 text-beton-200 hover:bg-beton-800 text-sm px-4 py-2">
                Özet PDF — Görüntüle
              </button>
            )}
          </div>
          {err && <p className="mt-2 text-sm text-red-400">{err}</p>}
        </div>

        <div className="rounded-lg border border-beton-800 bg-beton-900 p-3">
          <h2 className="font-display text-sm font-bold uppercase tracking-wide text-beton-100">Onay Zinciri</h2>
          <div className="mt-2">
            <ApprovalChain paymentId={pmt.id} onChanged={onChanged} />
          </div>
        </div>
      </div>
    </div>
  );
}
