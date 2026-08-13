import { useCallback, useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { api } from "../../api/client";
import { useProjects } from "../ProjectContext";

// Nakit Akış Faz F — hakediş/ekstre/PO ödeme planları + idari hakediş
// tahsilatını TEK, filtrelenebilir bir listede toplar. Her satır kendi
// kaynağının detay sayfasına bağlanır — bu sayfa onları taşımaz, üzerine
// bir toplu/özet katman ekler.

type Payment = {
  id: string;
  source_type: "progress_payment_disbursement" | "supplier_payment" | "po_payment" | "idari_hakedis";
  direction: "in" | "out";
  description: string;
  amount: number;
  payment_method?: string | null;
  event_date: string;
  link: string;
  pending_approval: boolean;
};

const SOURCE_LABEL: Record<Payment["source_type"], string> = {
  progress_payment_disbursement: "Hakediş Ödemesi",
  supplier_payment: "Tedarikçi Ekstresi",
  po_payment: "Sipariş Ödemesi",
  idari_hakedis: "İdari Hakediş",
};
const METHOD_LABEL: Record<string, string> = { nakit: "Nakit", havale: "Havale", cek: "Çek" };

function fmt(n: number): string {
  return n.toLocaleString("tr-TR", { minimumFractionDigits: 2, maximumFractionDigits: 2 });
}
function todayISO(offsetDays: number): string {
  const d = new Date();
  d.setDate(d.getDate() + offsetDays);
  return d.toISOString().slice(0, 10);
}

export default function OdemePlanlariPage() {
  const { current } = useProjects();
  const pid = current?.id;

  const [from, setFrom] = useState(todayISO(-30));
  const [to, setTo] = useState(todayISO(90));
  const [kaynak, setKaynak] = useState<Payment["source_type"] | "hepsi">("hepsi");
  const [yon, setYon] = useState<"hepsi" | "in" | "out">("hepsi");
  const [liste, setListe] = useState<Payment[]>([]);
  const [err, setErr] = useState<string | null>(null);

  const load = useCallback(async () => {
    if (!pid) return;
    setErr(null);
    try {
      const r = await api<{ payments: Payment[] }>(
        `/projects/${pid}/payment-plans?from=${from}&to=${to}`, { projectId: pid });
      setListe(r.payments ?? []);
    } catch {
      setErr("Ödeme planları yüklenemedi ya da erişim yetkiniz yok.");
    }
  }, [pid, from, to]);

  useEffect(() => { load(); }, [load]);

  const filtreli = liste.filter((p) => {
    if (kaynak !== "hepsi" && p.source_type !== kaynak) return false;
    if (yon !== "hepsi" && p.direction !== yon) return false;
    return true;
  });
  const toplamGiris = filtreli.filter((p) => p.direction === "in").reduce((s, p) => s + p.amount, 0);
  const toplamCikis = filtreli.filter((p) => p.direction === "out").reduce((s, p) => s + p.amount, 0);

  if (!current) return <p className="text-beton-400 text-sm">Önce üst bardan bir proje seçin.</p>;

  return (
    <div className="max-w-6xl mx-auto space-y-4">
      <div>
        <h1 className="font-display font-extrabold text-xl text-white">Ödeme Planları</h1>
        <p className="text-xs text-beton-400 mt-0.5">
          Hakediş, tedarikçi ekstresi, sipariş ve idari hakediş ödeme planlarının toplu görünümü —
          her satır kendi detay sayfasında girilmeye devam eder, burada yalnızca birlikte görüntülenir.
        </p>
      </div>
      {err && <p className="text-sm text-red-400">{err}</p>}

      <div className="flex flex-wrap items-end gap-3 rounded-lg border border-beton-800 bg-beton-900 p-3">
        <label className="text-xs text-beton-400">
          Başlangıç
          <input type="date" value={from} onChange={(e) => setFrom(e.target.value)}
            className="mt-1 block rounded-md bg-beton-950 border border-beton-800 px-2 py-1.5 text-sm text-beton-100" />
        </label>
        <label className="text-xs text-beton-400">
          Bitiş
          <input type="date" value={to} onChange={(e) => setTo(e.target.value)}
            className="mt-1 block rounded-md bg-beton-950 border border-beton-800 px-2 py-1.5 text-sm text-beton-100" />
        </label>
        <label className="text-xs text-beton-400">
          Kaynak
          <select value={kaynak} onChange={(e) => setKaynak(e.target.value as typeof kaynak)}
            className="mt-1 block rounded-md bg-beton-950 border border-beton-800 px-2 py-1.5 text-sm text-beton-100">
            <option value="hepsi">Tümü</option>
            {Object.entries(SOURCE_LABEL).map(([k, v]) => <option key={k} value={k}>{v}</option>)}
          </select>
        </label>
        <label className="text-xs text-beton-400">
          Yön
          <select value={yon} onChange={(e) => setYon(e.target.value as typeof yon)}
            className="mt-1 block rounded-md bg-beton-950 border border-beton-800 px-2 py-1.5 text-sm text-beton-100">
            <option value="hepsi">Tümü</option>
            <option value="in">Giriş</option>
            <option value="out">Çıkış</option>
          </select>
        </label>
      </div>

      <div className="grid grid-cols-2 gap-3">
        <div className="rounded-lg border border-beton-800 bg-beton-900 p-3">
          <p className="text-xs text-beton-500 mb-1">Toplam Giriş (filtreli)</p>
          <p className="text-lg font-bold text-emniyet-500">{fmt(toplamGiris)} TL</p>
        </div>
        <div className="rounded-lg border border-beton-800 bg-beton-900 p-3">
          <p className="text-xs text-beton-500 mb-1">Toplam Çıkış (filtreli)</p>
          <p className="text-lg font-bold text-red-400">{fmt(toplamCikis)} TL</p>
        </div>
      </div>

      <div className="border border-beton-800 rounded-lg overflow-hidden">
        <table className="w-full text-sm">
          <thead>
            <tr className="bg-beton-900 border-b border-beton-800 text-left text-xs text-beton-500">
              <th className="py-2 px-3">Tarih</th>
              <th className="py-2 px-3">Kaynak</th>
              <th className="py-2 px-3">Açıklama</th>
              <th className="py-2 px-3">Ödeme Şekli</th>
              <th className="py-2 px-3 text-right">Tutar</th>
              <th className="py-2 px-3" />
            </tr>
          </thead>
          <tbody>
            {filtreli.map((p) => (
              <tr key={`${p.source_type}-${p.id}`} className="border-b border-beton-800/50 hover:bg-beton-900/40">
                <td className="py-2 px-3 text-beton-300 font-mono text-xs">{p.event_date}</td>
                <td className="py-2 px-3">
                  <span className="text-xs rounded bg-beton-800 px-1.5 py-0.5 text-beton-300">
                    {SOURCE_LABEL[p.source_type]}
                  </span>
                </td>
                <td className="py-2 px-3 text-beton-100">
                  <Link to={p.link} className="hover:underline hover:text-emniyet-500">{p.description}</Link>
                  {p.pending_approval && (
                    <span className="ml-2 rounded bg-amber-500/15 text-amber-400 text-[10px] px-1.5 py-0.5 align-middle">
                      Onay Bekliyor
                    </span>
                  )}
                </td>
                <td className="py-2 px-3 text-beton-400">
                  {p.payment_method ? (METHOD_LABEL[p.payment_method] || p.payment_method) : "—"}
                </td>
                <td className={`py-2 px-3 text-right font-mono font-semibold ${p.direction === "in" ? "text-emniyet-500" : "text-red-400"}`}>
                  {p.direction === "in" ? "+" : "−"}{fmt(p.amount)}
                </td>
                <td className="py-2 px-3">
                  <Link to={p.link} className="text-xs text-emniyet-500 hover:underline">Aç →</Link>
                </td>
              </tr>
            ))}
            {!filtreli.length && (
              <tr><td colSpan={6} className="py-6 text-center text-beton-500 text-sm">Seçili filtrede ödeme planı yok.</td></tr>
            )}
          </tbody>
        </table>
      </div>
    </div>
  );
}
