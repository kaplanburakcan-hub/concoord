import { useCallback, useEffect, useRef, useState } from "react";
import { Link, useParams } from "react-router-dom";
import { api, apiDownload, apiUpload } from "../../api/client";
import { useAuth } from "../../auth/AuthContext";
import { useProjects } from "../ProjectContext";
import { PO, PO_STATUS_LABEL, PO_STATUS_STYLE } from "./PurchaseOrdersPage";

// Faz 7 — Sipariş detayı: teslimat zinciri + irsaliye fotoğrafı.
// Fotoğraf mobil kameradan (capture) alınır, Faz 2 doküman motoruna
// doc_category='Delivery' ile yüklenir ve teslimat kaydına document_id ile
// bağlanır (Plan §6.6). mark_delivered işaretli teslimat siparişi kapatır.

type DocVersion = { version_no: number; original_name: string };

// Mal kabul durumu etiketleri (Faz 11).
const COND_LABEL: Record<string, string> = {
  Complete: "Tam ve sağlam", Short: "Eksik", Damaged: "Hasarlı",
  Defective: "Şartnameye aykırı", Rejected: "Reddedildi",
};
const COND_CLS: Record<string, string> = {
  Complete: "text-green-300 border-green-500/40",
  Short: "text-amber-400 border-amber-500/40",
  Damaged: "text-red-300 border-red-500/40",
  Defective: "text-red-300 border-red-500/40",
  Rejected: "text-red-300 border-red-500/50",
};
const RECEIPT_LABEL: Record<string, string> = {
  Warehouse: "Depo", Site: "Saha", DirectToSubcontractor: "Taşeron",
};

export default function PurchaseOrderDetailPage() {
  const { id } = useParams();
  const { current } = useProjects();
  const { can } = useAuth();
  const [po, setPo] = useState<PO | null>(null);
  const [err, setErr] = useState<string | null>(null);
  const [noteNo, setNoteNo] = useState("");
  const [dNote, setDNote] = useState("");
  const [markDelivered, setMarkDelivered] = useState(false);
  // Faz 11 — mal kabul: nereye girdi, tam mı geldi, kalem bazında miktarlar
  const [receiptType, setReceiptType] = useState("Warehouse");
  const [locationNote, setLocationNote] = useState("");
  const [condition, setCondition] = useState("Complete");
  const [discrepancy, setDiscrepancy] = useState("");
  const [items, setItems] = useState<
    { material_name: string; unit: string; received_qty: string; accepted_qty: string; rejected_qty: string }[]
  >([]);
  const [busy, setBusy] = useState(false);
  const fileRef = useRef<HTMLInputElement>(null);        // irsaliye fotoğrafı (zorunlu)
  const matFileRef = useRef<HTMLInputElement>(null);     // malzeme fotoğrafı (zorunlu)
  const [hasNoteFile, setHasNoteFile] = useState(false);
  const [hasMatFile, setHasMatFile] = useState(false);
  const pid = current?.id;

  const load = useCallback(async () => {
    if (!pid || !id) return;
    setErr(null);
    try {
      const res = await api<PO>(`/projects/${pid}/purchase-orders/${id}`, { projectId: pid });
      setPo(res);
    } catch { setErr("Sipariş yüklenemedi."); }
  }, [pid, id]);

  useEffect(() => { load(); }, [load]);

  const open = po?.status === "Ordered" || po?.status === "PartiallyDelivered";

  async function addDelivery() {
    if (!noteNo.trim() || !pid || !po) return;
    setBusy(true);
    setErr(null);
    try {
      // 1) İki zorunlu fotoğraf doküman motoruna yüklenir (Faz 2):
      //    irsaliye = tedarikçinin beyanı, malzeme = sahanın tespiti.
      const uploadPhoto = async (file: File, title: string) => {
        const doc = await api<{ id: string }>(`/projects/${pid}/documents`, {
          method: "POST", projectId: pid,
          body: { title, doc_category: "Delivery" },
        });
        const fd = new FormData();
        fd.append("file", file);
        await apiUpload(`/projects/${pid}/documents/${doc.id}/versions`, fd);
        return doc.id;
      };

      const noteFile = fileRef.current?.files?.[0];
      const matFile = matFileRef.current?.files?.[0];
      if (!noteFile || !matFile) {
        setErr("İrsaliye ve malzeme fotoğraflarının ikisi de zorunludur.");
        setBusy(false);
        return;
      }
      const documentId = await uploadPhoto(noteFile, `${po.po_no} — İrsaliye ${noteNo.trim()}`);
      const materialPhotoId = await uploadPhoto(matFile, `${po.po_no} — Malzeme ${noteNo.trim()}`);
      // 2) Teslimat kaydı (kısmi teslim → PartiallyDelivered; kapama → Delivered).
      await api(`/projects/${pid}/purchase-orders/${id}/deliveries`, {
        method: "POST", projectId: pid,
        body: {
          delivery_note_no: noteNo.trim(),
          note: dNote.trim() || undefined,
          document_id: documentId,
          material_photo_document_id: materialPhotoId,
          mark_delivered: markDelivered,
          receipt_type: receiptType,
          location_note: locationNote.trim() || undefined,
          condition,
          discrepancy_note: condition === "Complete" ? undefined : discrepancy.trim(),
          items: items
            .filter((it) => it.material_name.trim() && Number(it.received_qty) > 0)
            .map((it) => ({
              material_name: it.material_name.trim(),
              unit: it.unit.trim() || "ad",
              received_qty: Number(it.received_qty) || 0,
              accepted_qty: Number(it.accepted_qty) || 0,
              rejected_qty: Number(it.rejected_qty) || 0,
            })),
        },
      });
      setNoteNo(""); setDNote(""); setMarkDelivered(false);
      setReceiptType("Warehouse"); setLocationNote("");
      setCondition("Complete"); setDiscrepancy(""); setItems([]);
      if (fileRef.current) fileRef.current.value = "";
      if (matFileRef.current) matFileRef.current.value = "";
      setHasNoteFile(false); setHasMatFile(false);
      load();
    } catch (e) {
      setErr(e instanceof Error && e.message ? e.message : "Teslimat kaydedilemedi.");
    } finally {
      setBusy(false);
    }
  }

  async function cancel() {
    if (!confirm("Sipariş iptal edilsin mi?")) return;
    try {
      await api(`/projects/${pid}/purchase-orders/${id}/cancel`, { method: "POST", projectId: pid, body: {} });
      load();
    } catch { setErr("İptal edilemedi."); }
  }

  async function downloadWaybill(documentId: string) {
    if (!pid) return;
    try {
      const det = await api<{ versions: DocVersion[] }>(
        `/projects/${pid}/documents/${documentId}`, { projectId: pid });
      const v = det.versions?.[det.versions.length - 1];
      if (v) {
        await apiDownload(
          `/projects/${pid}/documents/${documentId}/versions/${v.version_no}/download`,
          v.original_name);
      }
    } catch { setErr("İrsaliye indirilemedi."); }
  }

  if (!current) return <p className="text-beton-400">Önce üst bardan bir proje seçin.</p>;
  if (!po) return <p className="text-beton-400">{err ?? "Yükleniyor…"}</p>;

  return (
    <div className="space-y-4 max-w-3xl">
      <div className="flex items-center gap-3">
        <Link to="/satinalma/siparisler" className="text-xs text-beton-400 hover:text-beton-200">← Siparişler</Link>
        <h1 className="text-lg font-display font-bold text-white">{po.po_no}</h1>
        <span className={`rounded border px-1.5 py-0.5 text-xs ${PO_STATUS_STYLE[po.status] ?? ""}`}>
          {PO_STATUS_LABEL[po.status] ?? po.status}
        </span>
        {po.overdue && (
          <span className="rounded border border-red-500/40 bg-red-500/10 px-1.5 py-0.5 text-xs text-red-300">
            teslim tarihi geçti
          </span>
        )}
        {open && can("procurement.manage_po") && (
          <button onClick={cancel}
            className="ml-auto rounded-md border border-red-500/40 px-3 py-1.5 text-xs text-red-300 hover:bg-red-500/10">
            İptal Et
          </button>
        )}
      </div>

      {err && <p className="text-sm text-red-400">{err}</p>}

      <div className="rounded-lg border border-beton-800 bg-beton-900 p-4 text-sm text-beton-300 space-y-1">
        <p>Tedarikçi: <span className="text-beton-100">{po.supplier_name}</span></p>
        <p>Tutar: <span className="text-beton-100">
          {po.amount != null ? `${po.amount.toLocaleString("tr-TR")} ${po.currency}` : "—"}</span></p>
        <p>Beklenen teslim: <span className="text-beton-100">{po.expected_date ?? "—"}</span></p>
        <p>Sipariş veren: <span className="text-beton-100">{po.created_by_name}</span></p>
        {po.pr_no && po.pr_id && (
          <p>Kaynak talep: <Link to={`/satinalma/talepler/${po.pr_id}`}
            className="text-emniyet-500 hover:underline">{po.pr_no}</Link></p>
        )}
        {po.note && <p>Not: <span className="text-beton-100">{po.note}</span></p>}
      </div>

      <h2 className="text-sm font-semibold text-white">Teslimatlar</h2>
      <div className="rounded-lg border border-beton-800 divide-y divide-beton-800">
        {(po.deliveries ?? []).map((d) => (
          <div key={d.id} className="flex items-center gap-3 px-3 py-2 text-sm">
            <span className="text-beton-100">İrsaliye {d.delivery_note_no}</span>
            {d.receipt_type && (
              <span className="text-[10px] px-1.5 py-0.5 rounded border border-beton-700 text-beton-400">
                {RECEIPT_LABEL[d.receipt_type] ?? d.receipt_type}
                {d.location_note ? ` · ${d.location_note}` : ""}
              </span>
            )}
            {d.condition && (
              <span
                title={d.discrepancy_note ?? undefined}
                className={"text-[10px] px-1.5 py-0.5 rounded border " + (COND_CLS[d.condition] ?? "text-beton-400 border-beton-700")}>
                {COND_LABEL[d.condition] ?? d.condition}
              </span>
            )}
            {(d.items?.length ?? 0) > 0 && (
              <span className="text-[10px] text-beton-500">
                {d.items!.length} kalem · kabul{" "}
                {d.items!.reduce((a, x) => a + (x.accepted_qty ?? 0), 0)}
                {d.items!.some((x) => (x.rejected_qty ?? 0) > 0) && (
                  <span className="text-red-300">
                    {" "}· red {d.items!.reduce((a, x) => a + (x.rejected_qty ?? 0), 0)}
                  </span>
                )}
              </span>
            )}
            <span className="text-xs text-beton-400">
              {new Date(d.delivered_at).toLocaleString("tr-TR")} · {d.received_by_name}
            </span>
            {d.note && <span className="text-xs text-beton-400">— {d.note}</span>}
            {can("documents.download") && (
              <span className="ml-auto flex items-center gap-3">
                {d.document_id && (
                  <button onClick={() => downloadWaybill(d.document_id!)}
                    className="text-xs text-emniyet-500 hover:underline">
                    İrsaliye
                  </button>
                )}
                {d.material_photo_document_id && (
                  <button onClick={() => downloadWaybill(d.material_photo_document_id!)}
                    className="text-xs text-emniyet-500 hover:underline">
                    Malzeme
                  </button>
                )}
              </span>
            )}
          </div>
        ))}
        {(po.deliveries ?? []).length === 0 && (
          <p className="px-3 py-4 text-sm text-beton-500">Henüz teslimat yok.</p>
        )}
      </div>

      {open && can("procurement.upload_delivery") && (
        <div className="rounded-lg border border-beton-800 bg-beton-900 p-4 space-y-3">
          <h3 className="text-xs font-semibold text-beton-300">Teslimat Kaydet</h3>
          <div className="flex flex-wrap items-end gap-3">
            <label className="text-xs text-beton-300">
              İrsaliye no
              <input value={noteNo} onChange={(e) => setNoteNo(e.target.value)}
                className="mt-1 block rounded-md bg-beton-950 border border-beton-800 px-2 py-1 text-sm text-beton-100" />
            </label>
            <label className="text-xs text-beton-300">
              Teslim yeri
              <select value={receiptType} onChange={(e) => setReceiptType(e.target.value)}
                className="mt-1 block rounded-md bg-beton-950 border border-beton-800 px-2 py-1 text-sm text-beton-100">
                <option value="Warehouse">Depo girişi</option>
                <option value="Site">Doğrudan sahaya</option>
                <option value="DirectToSubcontractor">Taşerona teslim</option>
              </select>
            </label>
            <label className="flex-1 min-w-[140px] text-xs text-beton-300">
              Yer detayı (depo adı / blok-kat / firma)
              <input value={locationNote} onChange={(e) => setLocationNote(e.target.value)}
                className="mt-1 block w-full rounded-md bg-beton-950 border border-beton-800 px-2 py-1 text-sm text-beton-100" />
            </label>
          </div>

          {/* Teslim durumu — uygunsuzlukta açıklama zorunlu */}
          <div className="flex flex-wrap items-end gap-3">
            <label className="text-xs text-beton-300">
              Teslim durumu
              <select value={condition} onChange={(e) => setCondition(e.target.value)}
                className="mt-1 block rounded-md bg-beton-950 border border-beton-800 px-2 py-1 text-sm text-beton-100">
                <option value="Complete">Tam ve sağlam</option>
                <option value="Short">Eksik geldi</option>
                <option value="Damaged">Hasarlı geldi</option>
                <option value="Defective">Şartnameye aykırı</option>
                <option value="Rejected">Tümü reddedildi</option>
              </select>
            </label>
            <label className="flex-1 min-w-[200px] text-xs text-beton-300">
              Not
              <input value={dNote} onChange={(e) => setDNote(e.target.value)}
                className="mt-1 block w-full rounded-md bg-beton-950 border border-beton-800 px-2 py-1 text-sm text-beton-100" />
            </label>
          </div>

          {condition !== "Complete" && (
            <label className="block text-xs text-red-300">
              Uygunsuzluk açıklaması (zorunlu)
              <textarea value={discrepancy} onChange={(e) => setDiscrepancy(e.target.value)}
                rows={2}
                placeholder="Ne eksik/hatalı geldi, tedarikçiye ne bildirildi?"
                className="mt-1 block w-full rounded-md bg-beton-950 border border-red-500/40 px-2 py-1 text-sm text-beton-100" />
            </label>
          )}

          {/* Kalem bazında teslim alma: gelen / kabul / red */}
          <div className="border-t border-beton-800 pt-3">
            <p className="text-xs font-semibold text-beton-300 mb-2">Teslim alınan kalemler</p>
            {items.map((it, i) => (
              <div key={i} className="grid grid-cols-[1fr_60px_80px_80px_80px_auto] gap-2 mb-2 items-center">
                <input value={it.material_name} placeholder="Malzeme"
                  onChange={(e) => setItems(items.map((x, j) => j === i ? { ...x, material_name: e.target.value } : x))}
                  className="rounded bg-beton-950 border border-beton-800 px-2 py-1 text-sm text-beton-100" />
                <input value={it.unit} placeholder="Birim"
                  onChange={(e) => setItems(items.map((x, j) => j === i ? { ...x, unit: e.target.value } : x))}
                  className="rounded bg-beton-950 border border-beton-800 px-2 py-1 text-sm text-beton-100" />
                <input value={it.received_qty} placeholder="Gelen" inputMode="decimal"
                  onChange={(e) => setItems(items.map((x, j) => j === i ? { ...x, received_qty: e.target.value } : x))}
                  className="rounded bg-beton-950 border border-beton-800 px-2 py-1 text-sm text-beton-100 text-right" />
                <input value={it.accepted_qty} placeholder="Kabul" inputMode="decimal"
                  onChange={(e) => setItems(items.map((x, j) => j === i ? { ...x, accepted_qty: e.target.value } : x))}
                  className="rounded bg-beton-950 border border-green-500/30 px-2 py-1 text-sm text-beton-100 text-right" />
                <input value={it.rejected_qty} placeholder="Red" inputMode="decimal"
                  onChange={(e) => setItems(items.map((x, j) => j === i ? { ...x, rejected_qty: e.target.value } : x))}
                  className="rounded bg-beton-950 border border-red-500/30 px-2 py-1 text-sm text-beton-100 text-right" />
                <button onClick={() => setItems(items.filter((_, j) => j !== i))}
                  className="text-red-400 hover:text-red-300 text-xs px-1">Sil</button>
              </div>
            ))}
            <button
              onClick={() => setItems([...items, { material_name: "", unit: "ad", received_qty: "", accepted_qty: "", rejected_qty: "0" }])}
              className="text-emniyet-500 hover:underline text-xs">+ Kalem ekle</button>
            <p className="text-[10px] text-beton-500 mt-1">
              Kabul + red toplamı, gelen miktarı aşamaz. Reddedilen miktar iade/eksik tamamlama takibine esas olur.
            </p>
          </div>
          {/* İki zorunlu fotoğraf: irsaliye (tedarikçi beyanı) + malzeme (saha tespiti).
              capture="environment": mobilde doğrudan arka kamera açılır (PWA). */}
          <div className="border-t border-beton-800 pt-3 grid gap-3 sm:grid-cols-2">
            <label className="text-xs text-beton-300">
              İrsaliye fotoğrafı <span className="text-red-400">*</span>
              <input ref={fileRef} type="file" accept="image/*" capture="environment"
                onChange={(e) => setHasNoteFile(!!e.target.files?.length)}
                className={"mt-1 block w-full text-xs file:mr-2 file:rounded file:border-0 file:px-2 file:py-1 " +
                  (hasNoteFile ? "text-green-300 file:bg-green-500/20 file:text-green-200"
                               : "text-beton-400 file:bg-beton-800 file:text-beton-200")} />
            </label>
            <label className="text-xs text-beton-300">
              Malzeme fotoğrafı <span className="text-red-400">*</span>
              <input ref={matFileRef} type="file" accept="image/*" capture="environment"
                onChange={(e) => setHasMatFile(!!e.target.files?.length)}
                className={"mt-1 block w-full text-xs file:mr-2 file:rounded file:border-0 file:px-2 file:py-1 " +
                  (hasMatFile ? "text-green-300 file:bg-green-500/20 file:text-green-200"
                              : "text-beton-400 file:bg-beton-800 file:text-beton-200")} />
            </label>
          </div>
          <p className="text-[10px] text-beton-500">
            Her iki fotoğraf da zorunludur: irsaliye tedarikçinin beyanını, malzeme fotoğrafı
            fiilen gelenin durumunu belgeler. Sonradan çıkacak eksik/hasar tartışmasında ikisi karşılaştırılır.
          </p>

          <div className="flex flex-wrap items-center gap-4">
            <label className="flex items-center gap-2 text-xs text-beton-300">
              <input type="checkbox" checked={markDelivered}
                onChange={(e) => setMarkDelivered(e.target.checked)} />
              Bu teslimatla sipariş tamamlandı
            </label>
            <button onClick={addDelivery}
              disabled={busy || !noteNo.trim() || !hasNoteFile || !hasMatFile ||
                        (condition !== "Complete" && !discrepancy.trim())}
              className="rounded-md bg-emniyet-500 px-3 py-1.5 text-xs font-semibold text-beton-950 hover:bg-emniyet-400 disabled:opacity-40">
              {busy ? "Kaydediliyor…" : "Kaydet"}
            </button>
            {(!hasNoteFile || !hasMatFile) && (
              <span className="text-[11px] text-amber-400">
                Kaydetmek için iki fotoğrafı da ekleyin.
              </span>
            )}
          </div>
        </div>
      )}
    </div>
  );
}
