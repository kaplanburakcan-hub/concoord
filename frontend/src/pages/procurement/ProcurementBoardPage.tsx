import { useCallback, useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { api } from "../../api/client";
import { useAuth } from "../../auth/AuthContext";
import { useProjects } from "../ProjectContext";
import type { PR } from "./PurchaseRequestsPage";
import type { PO } from "./PurchaseOrdersPage";

// Faz 11 — Satınalma akış panosu.
//
// Talep ve sipariş sistemde iki ayrı kayıt olsa da SAHADA TEK BİR AKIŞTIR:
// ihtiyaç doğar → onaylanır → sipariş verilir → mal gelir → irsaliye girilir.
// Malzeme onayı (MAR) panosunda olduğu gibi bu akışı da tek bakışta görmek,
// "hangi talep nerede takıldı" sorusunu tablo taramadan yanıtlar.
//
// Kart = SATINALMA İZİ (talep + varsa siparişi). Talep siparişe dönüştüğünde
// aynı kart sipariş sütunlarına geçer; böylece iz kopmaz. Talebi olmayan bir
// sipariş de akışa kendi kartıyla girer — kart numarasında (PO-002 ← PR-002)
// bağ zaten görüldüğü için ayrıca "bağımsız" diye ayrılmaz.

type Card = {
  key: string;
  prNo?: string;
  prId?: string;
  poNo?: string;
  poId?: string;
  title: string;      // tedarikçi ya da talep eden
  subtitle?: string;  // tutar / kalem sayısı
  date?: string;
  overdue?: boolean;
  deliveryCount?: number;
};

type Column = {
  code: string;
  label: string;
  hint: string;
  head: string;   // sütun başlığı rengi
  card: string;   // kart rengi
};

// Akış sırası: soldan sağa ilerleyen yaşam döngüsü.
const COLUMNS: Column[] = [
  {
    code: "Requested", label: "Talep", hint: "onay bekliyor",
    head: "bg-beton-800/60 text-beton-200 border-beton-700",
    card: "border-beton-700 bg-beton-950",
  },
  {
    code: "Approved", label: "Onaylandı", hint: "sipariş bekliyor",
    head: "bg-blue-500/15 text-blue-300 border-blue-500/40",
    card: "border-blue-500/30 bg-blue-500/5",
  },
  {
    code: "Ordered", label: "Sipariş verildi", hint: "mal bekleniyor",
    head: "bg-emniyet-500/15 text-emniyet-500 border-emniyet-500/40",
    card: "border-emniyet-500/30 bg-emniyet-500/5",
  },
  {
    code: "PartiallyDelivered", label: "Kısmi teslimat", hint: "eksik tamamlanacak",
    head: "bg-amber-500/15 text-amber-400 border-amber-500/40",
    card: "border-amber-500/30 bg-amber-500/5",
  },
  {
    code: "Delivered", label: "Teslim alındı", hint: "irsaliye girildi",
    head: "bg-green-500/15 text-green-300 border-green-500/40",
    card: "border-green-500/30 bg-green-500/5",
  },
  {
    code: "Closed", label: "Reddedildi / İptal", hint: "akış dışı",
    head: "bg-red-500/10 text-red-300 border-red-500/30",
    card: "border-red-500/25 bg-red-500/5",
  },
];

export default function ProcurementBoardPage() {
  const { current } = useProjects();
  const { can } = useAuth();
  const pid = current?.id;
  const [prs, setPrs] = useState<PR[]>([]);
  const [pos, setPos] = useState<PO[]>([]);
  const [err, setErr] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  const load = useCallback(async () => {
    if (!pid) return;
    setLoading(true);
    try {
      const [a, b] = await Promise.all([
        api<{ purchase_requests: PR[] }>(`/projects/${pid}/purchase-requests`, { projectId: pid }),
        api<{ purchase_orders: PO[] }>(`/projects/${pid}/purchase-orders`, { projectId: pid }),
      ]);
      setPrs(a.purchase_requests ?? []);
      setPos(b.purchase_orders ?? []);
      setErr(null);
    } catch (e: any) {
      setErr(e?.message ?? "Satınalma akışı yüklenemedi.");
    } finally {
      setLoading(false);
    }
  }, [pid]);

  useEffect(() => { load(); }, [load]);

  if (!current) {
    return <p className="text-sm text-beton-400">Önce bir proje seçin.</p>;
  }

  // --- Kartları akış sütunlarına yerleştir ---
  const buckets: Record<string, Card[]> = {};
  for (const c of COLUMNS) buckets[c.code] = [];

  const money = (v?: number, cur?: string) =>
    v == null ? undefined : `${v.toLocaleString("tr-TR")} ${cur ?? ""}`.trim();

  // 1) Siparişler: talebe bağlı olsun olmasın, akışın sipariş tarafını temsil eder.
  const prIdsWithPO = new Set<string>();
  for (const o of pos) {
    if (o.pr_id) prIdsWithPO.add(o.pr_id);
    const col =
      o.status === "Cancelled" ? "Closed"
      : o.status === "Delivered" ? "Delivered"
      : o.status === "PartiallyDelivered" ? "PartiallyDelivered"
      : "Ordered";
    buckets[col].push({
      key: "po-" + o.id,
      poNo: o.po_no, poId: o.id,
      prNo: o.pr_no, prId: o.pr_id,
      title: o.supplier_name,
      subtitle: money(o.amount, o.currency),
      date: o.expected_date,
      overdue: o.overdue,
      deliveryCount: o.delivery_count,
    });
  }

  // 2) Henüz siparişe dönmemiş talepler.
  for (const p of prs) {
    if (p.po_id || prIdsWithPO.has(p.id)) continue; // sipariş kartı zaten temsil ediyor
    const col =
      p.status === "Rejected" ? "Closed"
      : p.status === "Approved" ? "Approved"
      : "Requested";
    buckets[col].push({
      key: "pr-" + p.id,
      prNo: p.pr_no, prId: p.id,
      title: p.requested_by_name,
      subtitle: `${p.item_count} kalem`,
      date: p.needed_by_date,
      overdue: p.overdue,
    });
  }

  const total = Object.values(buckets).reduce((a, b) => a + b.length, 0);
  // Akış dışı sütunu boşsa gösterilmez: pano sadeleşir.
  const visible = COLUMNS.filter((c) => c.code !== "Closed" || buckets[c.code].length > 0);

  return (
    <div className="space-y-4">
      <div className="flex items-center gap-3 flex-wrap">
        <h1 className="text-lg font-display font-bold text-white">Satınalma</h1>
        <span className="text-xs text-beton-500">{total} kayıt</span>
        <div className="ml-auto flex gap-2">
          <Link to="/satinalma/tedarik-plani"
            className="rounded border border-beton-700 text-beton-300 hover:border-emniyet-500 hover:text-emniyet-500 px-3 py-1.5 text-xs font-medium transition-colors">
            📋 Tedarik Planı
          </Link>
          <button onClick={load}
            className="rounded bg-[var(--group-accent)] hover:brightness-110 text-white-solid px-3 py-1.5 text-xs font-medium">
            Yenile
          </button>
          {/* Akış talepten ya da doğrudan siparişten başlayabilir; iki düğme
              ayrı renkte ki karışmasınlar. */}
          {can("procurement.create_pr") && (
            <Link to="/satinalma/talepler?yeni=1"
              className="rounded bg-emniyet-500 hover:bg-emniyet-600 text-beton-950 font-semibold px-3 py-1.5 text-xs">
              Yeni Talep
            </Link>
          )}
          {can("procurement.manage_po") && (
            <Link to="/satinalma/siparisler?yeni=1"
              className="rounded bg-blue-500 hover:bg-blue-600 text-white-solid font-semibold px-3 py-1.5 text-xs">
              Yeni Sipariş
            </Link>
          )}
        </div>
      </div>
      <p className="text-xs text-beton-500 -mt-2">
        İhtiyaçtan teslimata kadar tek akış. Talep siparişe dönüştüğünde kart sipariş
        sütunlarına geçer; iz kopmaz.
      </p>

      {err && <p className="text-sm text-red-400">{err}</p>}
      {loading && <p className="text-sm text-beton-400">Yükleniyor…</p>}

      {/* Sütun sayısı görünen sütuna göre: "akış dışı" boşken 5 sütun kalır ve
          pano MAR ile aynı oranda genişler. */}
      <div className={"grid gap-3 md:grid-cols-3 " +
        (visible.length === 6 ? "xl:grid-cols-6" : "xl:grid-cols-5")}>
        {visible.map((col) => {
          const cards = buckets[col.code];
          return (
            <div key={col.code} className="rounded-lg border border-beton-800 bg-beton-900 overflow-hidden">
              <div className={"px-3 py-2 text-xs font-semibold rounded-t-lg border-b " + col.head}>
                {col.label} <span className="opacity-70">({cards.length})</span>
                <span className="block text-[10px] font-normal opacity-60 mt-0.5">{col.hint}</span>
              </div>
              <div className="p-2 space-y-2 min-h-[80px]">
                {cards.map((c) => (
                  <Link
                    key={c.key}
                    to={c.poId ? `/satinalma/siparisler/${c.poId}` : `/satinalma/talepler/${c.prId}`}
                    className={"block rounded-md border p-2 transition hover:brightness-110 " + col.card}
                  >
                    <div className="flex items-center gap-1.5 font-mono text-[11px]">
                      {c.poNo && <span className="font-semibold">{c.poNo}</span>}
                      {c.poNo && c.prNo && <span className="opacity-50">←</span>}
                      {c.prNo && <span className="opacity-80">{c.prNo}</span>}
                    </div>
                    <p className="text-sm text-white truncate">{c.title}</p>
                    {c.subtitle && (
                      <p className="text-[11px] opacity-80 tabular-nums">{c.subtitle}</p>
                    )}
                    <div className="mt-1.5 flex items-center gap-2 text-[10px]">
                      {c.date && (
                        <span className={c.overdue ? "text-red-400" : "text-beton-500"}>
                          {c.date}{c.overdue ? " · gecikti" : ""}
                        </span>
                      )}
                      {(c.deliveryCount ?? 0) > 0 && (
                        <span className="text-green-300">{c.deliveryCount} teslimat</span>
                      )}
                    </div>
                  </Link>
                ))}
                {cards.length === 0 && (
                  <p className="px-1 py-3 text-[11px] text-beton-600">Kayıt yok.</p>
                )}
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
}
