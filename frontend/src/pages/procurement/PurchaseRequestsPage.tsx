import { useCallback, useEffect, useState } from "react";
import { Link, useLocation, useNavigate } from "react-router-dom";
import { api } from "../../api/client";
import { useAuth } from "../../auth/AuthContext";
import { useProjects } from "../ProjectContext";

// Faz 7 — Satınalma talepleri (PR) listesi + yeni talep formu.
// Gecikenler (ihtiyaç tarihi geçmiş, siparişe dönmemiş) kırmızı vurgulanır
// (Plan Faz 7 kabul kriteri).

export type PRItem = {
  id?: string; material_name: string; spec?: string; qty: number; unit: string; note?: string;
};
export type PR = {
  id: string; pr_no: string; status: string; needed_by_date: string; note?: string;
  requested_by_name: string; decided_by_name?: string; decided_at?: string; decision_note?: string;
  po_id?: string; po_no?: string; overdue: boolean; item_count: number;
  items?: PRItem[]; row_version: number; created_at: string;
};

export const PR_STATUS_LABEL: Record<string, string> = {
  Draft: "Taslak", Submitted: "Onay Bekliyor", Approved: "Onaylandı",
  Rejected: "Reddedildi", Converted: "Siparişe Dönüştü",
};
export const PR_STATUS_STYLE: Record<string, string> = {
  Draft: "bg-beton-800 text-beton-200 border-beton-700",
  Submitted: "bg-blue-500/15 text-blue-300 border-blue-500/40",
  Approved: "bg-green-500/15 text-green-300 border-green-500/40",
  Rejected: "bg-red-500/15 text-red-300 border-red-500/40",
  Converted: "bg-emniyet-500/15 text-emniyet-500 border-emniyet-500/40",
};

const EMPTY_ITEM: PRItem = { material_name: "", qty: 1, unit: "" };

export default function PurchaseRequestsPage() {
  const { current } = useProjects();
  const { can } = useAuth();
  const [prs, setPrs] = useState<PR[]>([]);
  const [err, setErr] = useState<string | null>(null);
  const [showForm, setShowForm] = useState(false);
  const [neededBy, setNeededBy] = useState("");
  const [note, setNote] = useState("");
  const [items, setItems] = useState<PRItem[]>([{ ...EMPTY_ITEM }]);
  const pid = current?.id;

  const load = useCallback(async () => {
    if (!pid) return;
    setErr(null);
    try {
      const res = await api<{ purchase_requests: PR[] }>(
        `/projects/${pid}/purchase-requests`, { projectId: pid });
      setPrs(res.purchase_requests);
    } catch { setErr("Satınalma talepleri yüklenemedi ya da erişim yetkiniz yok."); }
  }, [pid]);

  useEffect(() => { load(); }, [load]);

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

  function setItem(i: number, patch: Partial<PRItem>) {
    setItems((xs) => xs.map((x, j) => (j === i ? { ...x, ...patch } : x)));
  }

  async function create() {
    if (!neededBy) { setErr("İhtiyaç tarihi girin."); return; }
    try {
      await api(`/projects/${pid}/purchase-requests`, {
        method: "POST", projectId: pid,
        body: {
          needed_by_date: neededBy,
          note: note.trim() || undefined,
          items: items
            .filter((it) => it.material_name.trim())
            .map((it) => ({
              material_name: it.material_name.trim(),
              spec: it.spec?.trim() || undefined,
              qty: Number(it.qty),
              unit: it.unit.trim(),
              note: it.note?.trim() || undefined,
            })),
        },
      });
      setNeededBy(""); setNote(""); setItems([{ ...EMPTY_ITEM }]); setShowForm(false);
      load();
    } catch (e) {
      setErr(e instanceof Error && e.message ? e.message : "PR oluşturulamadı.");
    }
  }

  if (!current) return <p className="text-beton-400">Önce üst bardan bir proje seçin.</p>;

  const overdueCount = prs.filter((p) => p.overdue).length;

  return (
    <div className="space-y-4">
      <div className="flex items-center gap-3">
        {/* Modül içi geçiş artık kenar çubuğundaki alt başlıklarda
            (Satınalma → Talepler / Siparişler); başlık yanına iliştirilmiş
            zayıf bağlantı kaldırıldı. */}
        <h1 className="text-lg font-display font-medium text-beton-100">Satınalma Talepleri</h1>
        {can("procurement.create_pr") && (
          <button
            onClick={() => setShowForm((v) => !v)}
            className="ml-auto rounded-md bg-emniyet-500 px-3 py-1.5 text-xs font-semibold text-beton-950 hover:bg-emniyet-400"
          >
            {showForm ? "Vazgeç" : "Yeni Talep"}
          </button>
        )}
      </div>

      {overdueCount > 0 && (
        <div className="rounded-md border border-red-500/40 bg-red-500/10 px-3 py-2 text-xs text-red-300">
          ⚠ {overdueCount} talebin ihtiyaç tarihi geçti ve henüz siparişe dönüşmedi.
        </div>
      )}
      {err && <p className="text-sm text-red-400">{err}</p>}

      {showForm && (
        <div className="rounded-lg border border-beton-800 bg-beton-900 p-4 space-y-3">
          <div className="flex flex-wrap gap-3">
            <label className="text-xs text-beton-300">
              İhtiyaç tarihi
              <input type="date" value={neededBy} onChange={(e) => setNeededBy(e.target.value)}
                className="mt-1 block rounded-md bg-beton-950 border border-beton-800 px-2 py-1 text-sm text-beton-100" />
            </label>
            <label className="flex-1 min-w-[220px] text-xs text-beton-300">
              Not (opsiyonel)
              <input value={note} onChange={(e) => setNote(e.target.value)}
                className="mt-1 block w-full rounded-md bg-beton-950 border border-beton-800 px-2 py-1 text-sm text-beton-100" />
            </label>
          </div>
          <div className="space-y-2">
            {items.map((it, i) => (
              <div key={i} className="grid grid-cols-12 gap-2">
                <input placeholder="Malzeme adı" value={it.material_name}
                  onChange={(e) => setItem(i, { material_name: e.target.value })}
                  className="col-span-4 rounded-md bg-beton-950 border border-beton-800 px-2 py-1 text-sm text-beton-100" />
                <input placeholder="Şartname/spec" value={it.spec ?? ""}
                  onChange={(e) => setItem(i, { spec: e.target.value })}
                  className="col-span-3 rounded-md bg-beton-950 border border-beton-800 px-2 py-1 text-sm text-beton-100" />
                <input type="number" min={0} step="any" placeholder="Miktar" value={it.qty}
                  onChange={(e) => setItem(i, { qty: Number(e.target.value) })}
                  className="col-span-2 rounded-md bg-beton-950 border border-beton-800 px-2 py-1 text-sm text-beton-100" />
                <input placeholder="Birim" value={it.unit}
                  onChange={(e) => setItem(i, { unit: e.target.value })}
                  className="col-span-2 rounded-md bg-beton-950 border border-beton-800 px-2 py-1 text-sm text-beton-100" />
                <button onClick={() => setItems((xs) => xs.filter((_, j) => j !== i))}
                  disabled={items.length === 1}
                  className="col-span-1 rounded-md border border-beton-800 text-xs text-beton-400 hover:text-red-300 disabled:opacity-30">
                  Sil
                </button>
              </div>
            ))}
            <button onClick={() => setItems((xs) => [...xs, { ...EMPTY_ITEM }])}
              className="text-xs text-emniyet-500 hover:underline">+ Kalem ekle</button>
          </div>
          <button onClick={create}
            className="rounded-md bg-emniyet-500 px-3 py-1.5 text-xs font-semibold text-beton-950 hover:bg-emniyet-400">
            Taslak Oluştur
          </button>
        </div>
      )}

      <div className="overflow-x-auto rounded-lg border border-beton-800">
        <table className="min-w-full text-sm">
          <thead className="bg-beton-900 text-left text-xs text-beton-400">
            <tr>
              <th className="px-3 py-2">No</th>
              <th className="px-3 py-2">Durum</th>
              <th className="px-3 py-2">İhtiyaç Tarihi</th>
              <th className="px-3 py-2">Kalem</th>
              <th className="px-3 py-2">Talep Eden</th>
              <th className="px-3 py-2">Sipariş</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-beton-800">
            {prs.map((p) => (
              <tr key={p.id} className={p.overdue ? "bg-red-500/5" : ""}>
                <td className="px-3 py-2">
                  <Link to={`/satinalma/talepler/${p.id}`} className="text-emniyet-500 hover:underline">
                    {p.pr_no}
                  </Link>
                </td>
                <td className="px-3 py-2">
                  <span className={`rounded border px-1.5 py-0.5 text-xs ${PR_STATUS_STYLE[p.status] ?? ""}`}>
                    {PR_STATUS_LABEL[p.status] ?? p.status}
                  </span>
                </td>
                <td className="px-3 py-2">
                  {p.needed_by_date}
                  {p.overdue && <span className="ml-2 text-xs text-red-400">gecikti</span>}
                </td>
                <td className="px-3 py-2 text-beton-300">{p.item_count}</td>
                <td className="px-3 py-2 text-beton-300">{p.requested_by_name}</td>
                <td className="px-3 py-2 text-beton-300">{p.po_no ?? "—"}</td>
              </tr>
            ))}
            {prs.length === 0 && (
              <tr><td colSpan={6} className="px-3 py-6 text-center text-beton-500">Kayıt yok.</td></tr>
            )}
          </tbody>
        </table>
      </div>
    </div>
  );
}
