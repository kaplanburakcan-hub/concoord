import { useCallback, useEffect, useState } from "react";
import { Link, useLocation, useNavigate } from "react-router-dom";
import { api } from "../../api/client";
import { useAuth } from "../../auth/AuthContext";
import { useProjects } from "../ProjectContext";
import { useKesinKabulTarihi } from "../../hooks/useKesinKabulTarihi";

// Faz 7 — Tedarik durum panosu + sipariş (PO) listesi.
// Geciken siparişler (beklenen tarih geçti, kapanmadı) kırmızı vurgulanır
// (Plan Faz 7: "tedarik durum panosu, geciken siparişler vurgulu").

export type DeliveryItem = {
  id: string; material_name: string; unit: string;
  ordered_qty?: number; received_qty: number; accepted_qty: number; rejected_qty: number;
  note?: string;
};
export type Delivery = {
  id: string; delivery_note_no: string; delivered_at: string;
  received_by_name: string; document_id?: string; note?: string;
  // Faz 11 — mal kabul detayı
  receipt_type?: string; location_note?: string;
  condition?: string; discrepancy_note?: string;
  // İki zorunlu kanıt: irsaliye (document_id) ve malzeme fotoğrafı
  photo_document_id?: string; material_photo_document_id?: string;
  items?: DeliveryItem[];
};
export type PO = {
  id: string; po_no: string; pr_id?: string; pr_no?: string;
  supplier_name: string; tedarikci_id?: string; tedarikci_name?: string;
  amount?: number; currency: string; status: string;
  expected_date?: string; note?: string; created_by_name: string;
  overdue: boolean; delivery_count: number; deliveries?: Delivery[];
  row_version: number; created_at: string;
};
type Board = {
  pr_counts: Record<string, number>;
  po_counts: Record<string, number>;
  overdue_prs: { id: string; pr_no: string; needed_by_date: string }[];
  overdue_pos: { id: string; po_no: string; expected_date?: string }[];
};

export const PO_STATUS_LABEL: Record<string, string> = {
  Ordered: "Sipariş Verildi", PartiallyDelivered: "Kısmi Teslim",
  Delivered: "Teslim Edildi", Cancelled: "İptal",
};
export const PO_STATUS_STYLE: Record<string, string> = {
  Ordered: "bg-blue-500/15 text-blue-300 border-blue-500/40",
  PartiallyDelivered: "bg-emniyet-500/15 text-emniyet-500 border-emniyet-500/40",
  Delivered: "bg-green-500/15 text-green-300 border-green-500/40",
  Cancelled: "bg-beton-800 text-beton-400 border-beton-700",
};

export default function PurchaseOrdersPage() {
  const { current } = useProjects();
  const { can } = useAuth();
  const [pos, setPos] = useState<PO[]>([]);
  const [board, setBoard] = useState<Board | null>(null);
  const [err, setErr] = useState<string | null>(null);
  const [showForm, setShowForm] = useState(false);
  const [supplier, setSupplier] = useState("");
  const [tedarikciId, setTedarikciId] = useState("");
  const [tedarikciler, setTedarikciler] = useState<{ id: string; company_name: string }[]>([]);
  const [amount, setAmount] = useState("");
  const [expected, setExpected] = useState("");
  const [note, setNote] = useState("");
  const pid = current?.id;
  const kesinKabul = useKesinKabulTarihi(pid);

  const load = useCallback(async () => {
    if (!pid) return;
    setErr(null);
    try {
      const res = await api<{ purchase_orders: PO[] }>(
        `/projects/${pid}/purchase-orders`, { projectId: pid });
      setPos(res.purchase_orders);
      const b = await api<Board>(`/projects/${pid}/procurement/board`, { projectId: pid });
      setBoard(b);
    } catch { setErr("Siparişler yüklenemedi ya da erişim yetkiniz yok."); }
  }, [pid]);

  useEffect(() => { load(); }, [load]);

  useEffect(() => {
    if (!pid) return;
    api<{ tedarikciler: { id: string; company_name: string }[] }>(`/projects/${pid}/tedarikciler`, { projectId: pid })
      .then((r) => setTedarikciler(r.tedarikciler ?? []))
      .catch(() => setTedarikciler([]));
  }, [pid]);

  // Akış panosundaki "Yeni ..." düğmesi buraya ?yeni=1 ile gelir ve formu açar.
  // Parametre açıldıktan HEMEN SONRA temizlenir; iki sebeple:
  //   1. Sayfa yenilendiğinde form kendiliğinden açılmasın.
  //   2. Aynı bağlantıya tekrar tıklandığında adres gerçekten değişsin
  //      (aksi hâlde React Router aynı adrese gidişi yok sayar ve form açılmaz).
  const location = useLocation();
  const navigate = useNavigate();
  useEffect(() => {
    if (new URLSearchParams(location.search).has("yeni")) {
      setShowForm(true);
      navigate(location.pathname, { replace: true });
    }
  }, [location.search, location.pathname, navigate]);

  async function create() {
    if (!supplier.trim()) return;
    try {
      await api(`/projects/${pid}/purchase-orders`, {
        method: "POST", projectId: pid,
        body: {
          supplier_name: supplier.trim(),
          tedarikci_id: tedarikciId || undefined,
          amount: amount ? Number(amount) : undefined,
          expected_date: expected || undefined,
          note: note.trim() || undefined,
        },
      });
      setSupplier(""); setTedarikciId(""); setAmount(""); setExpected(""); setNote(""); setShowForm(false);
      load();
    } catch { setErr("Sipariş oluşturulamadı."); }
  }

  if (!current) return <p className="text-beton-400">Önce üst bardan bir proje seçin.</p>;

  const fmtAmt = (o: PO) =>
    o.amount != null ? `${o.amount.toLocaleString("tr-TR")} ${o.currency}` : "—";

  return (
    <div className="space-y-4">
      <div className="flex items-center gap-3">
        <h1 className="text-lg font-display font-medium text-beton-100">Satınalma Siparişleri</h1>
        {can("procurement.manage_po") && (
          <button onClick={() => setShowForm((v) => !v)}
            className="ml-auto rounded-md bg-emniyet-500 px-3 py-1.5 text-xs font-semibold text-beton-950 hover:bg-emniyet-400">
            {showForm ? "Vazgeç" : "Yeni Sipariş"}
          </button>
        )}
      </div>

      {board && (
        <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
          <Stat label="Onay Bekleyen PR" value={board.pr_counts["Submitted"] ?? 0} tone="blue" />
          <Stat label="Açık Sipariş"
            value={(board.po_counts["Ordered"] ?? 0) + (board.po_counts["PartiallyDelivered"] ?? 0)} tone="amber" />
          <Stat label="Geciken PR" value={board.overdue_prs.length} tone="red" />
          <Stat label="Geciken Sipariş" value={board.overdue_pos.length} tone="red" />
        </div>
      )}
      {err && <p className="text-sm text-red-400">{err}</p>}

      {showForm && (
        <div className="rounded-lg border border-beton-800 bg-beton-900 p-4 flex flex-wrap items-end gap-3">
          <label className="text-xs text-beton-300">
            Tedarikçi
            <input value={supplier} onChange={(e) => setSupplier(e.target.value)}
              className="mt-1 block rounded-md bg-beton-950 border border-beton-800 px-2 py-1 text-sm text-beton-100" />
          </label>
          <label className="text-xs text-beton-300">
            Kayıtlı tedarikçi (ödeme planı için)
            <select value={tedarikciId}
              onChange={(e) => {
                setTedarikciId(e.target.value);
                const t = tedarikciler.find((x) => x.id === e.target.value);
                if (t && !supplier.trim()) setSupplier(t.company_name);
              }}
              className="mt-1 block rounded-md bg-beton-950 border border-beton-800 px-2 py-1 text-sm text-beton-100">
              <option value="">— bağlama —</option>
              {tedarikciler.map((t) => (
                <option key={t.id} value={t.id}>{t.company_name}</option>
              ))}
            </select>
          </label>
          <label className="text-xs text-beton-300">
            Tutar
            <input type="number" min={0} step="any" value={amount} onChange={(e) => setAmount(e.target.value)}
              className="mt-1 block w-28 rounded-md bg-beton-950 border border-beton-800 px-2 py-1 text-sm text-beton-100" />
          </label>
          <label className="text-xs text-beton-300">
            Beklenen teslim
            <input type="date" value={expected} max={kesinKabul} onChange={(e) => setExpected(e.target.value)}
              className="mt-1 block rounded-md bg-beton-950 border border-beton-800 px-2 py-1 text-sm text-beton-100" />
          </label>
          <label className="flex-1 min-w-[160px] text-xs text-beton-300">
            Not
            <input value={note} onChange={(e) => setNote(e.target.value)}
              className="mt-1 block w-full rounded-md bg-beton-950 border border-beton-800 px-2 py-1 text-sm text-beton-100" />
          </label>
          <button onClick={create} disabled={!supplier.trim()}
            className="rounded-md bg-emniyet-500 px-3 py-1.5 text-xs font-semibold text-beton-950 hover:bg-emniyet-400 disabled:opacity-40">
            Oluştur
          </button>
        </div>
      )}

      <div className="overflow-x-auto rounded-lg border border-beton-800">
        <table className="min-w-full text-sm">
          <thead className="bg-beton-900 text-left text-xs text-beton-400">
            <tr>
              <th className="px-3 py-2">No</th>
              <th className="px-3 py-2">Durum</th>
              <th className="px-3 py-2">Tedarikçi</th>
              <th className="px-3 py-2">Tutar</th>
              <th className="px-3 py-2">Beklenen Teslim</th>
              <th className="px-3 py-2">Teslimat</th>
              <th className="px-3 py-2">PR</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-beton-800">
            {pos.map((o) => (
              <tr key={o.id} className={o.overdue ? "bg-red-500/5" : ""}>
                <td className="px-3 py-2">
                  <Link to={`/satinalma/siparisler/${o.id}`} className="text-emniyet-500 hover:underline">
                    {o.po_no}
                  </Link>
                </td>
                <td className="px-3 py-2">
                  <span className={`rounded border px-1.5 py-0.5 text-xs ${PO_STATUS_STYLE[o.status] ?? ""}`}>
                    {PO_STATUS_LABEL[o.status] ?? o.status}
                  </span>
                </td>
                <td className="px-3 py-2 text-beton-100">{o.supplier_name}</td>
                <td className="px-3 py-2 text-beton-300">{fmtAmt(o)}</td>
                <td className="px-3 py-2">
                  {o.expected_date ?? "—"}
                  {o.overdue && <span className="ml-2 text-xs text-red-400">gecikti</span>}
                </td>
                <td className="px-3 py-2 text-beton-300">{o.delivery_count}</td>
                <td className="px-3 py-2 text-beton-300">{o.pr_no ?? "—"}</td>
              </tr>
            ))}
            {pos.length === 0 && (
              <tr><td colSpan={7} className="px-3 py-6 text-center text-beton-500">Kayıt yok.</td></tr>
            )}
          </tbody>
        </table>
      </div>
    </div>
  );
}

// Statü rengiyle anında ayırt edilsin diye — ProcurementBoardPage'in akış
// sütunlarıyla aynı ton sözlüğü (blue=onay bekliyor, amber=akışta, red=gecikti).
// Değer 0'ken nötr kalır: boş bir kart renkle "sorun var" izlenimi vermesin.
const STAT_TONE = {
  neutral: { box: "border-beton-800 bg-beton-900", text: "text-white" },
  blue:    { box: "border-blue-500/40 bg-blue-500/10", text: "text-blue-300" },
  amber:   { box: "border-amber-500/40 bg-amber-500/10", text: "text-amber-400" },
  red:     { box: "border-red-500/40 bg-red-500/10", text: "text-red-300" },
} as const;

function Stat({ label, value, tone = "neutral" }: { label: string; value: number; tone?: keyof typeof STAT_TONE }) {
  const t = STAT_TONE[value > 0 ? tone : "neutral"];
  return (
    <div className={`rounded-lg border p-3 ${t.box}`}>
      <div className={`text-2xl font-display font-bold ${t.text}`}>
        {value}
      </div>
      <div className="text-xs text-beton-400">{label}</div>
    </div>
  );
}
