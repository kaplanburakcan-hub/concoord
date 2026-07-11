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

export default function PurchaseOrderDetailPage() {
  const { id } = useParams();
  const { current } = useProjects();
  const { can } = useAuth();
  const [po, setPo] = useState<PO | null>(null);
  const [err, setErr] = useState<string | null>(null);
  const [noteNo, setNoteNo] = useState("");
  const [dNote, setDNote] = useState("");
  const [markDelivered, setMarkDelivered] = useState(false);
  const [busy, setBusy] = useState(false);
  const fileRef = useRef<HTMLInputElement>(null);
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
      // 1) İrsaliye fotoğrafı varsa önce doküman motoruna yükle (Faz 2).
      let documentId: string | undefined;
      const file = fileRef.current?.files?.[0];
      if (file) {
        const doc = await api<{ id: string }>(`/projects/${pid}/documents`, {
          method: "POST", projectId: pid,
          body: { title: `${po.po_no} — İrsaliye ${noteNo.trim()}`, doc_category: "Delivery" },
        });
        const fd = new FormData();
        fd.append("file", file);
        await apiUpload(`/projects/${pid}/documents/${doc.id}/versions`, fd);
        documentId = doc.id;
      }
      // 2) Teslimat kaydı (kısmi teslim → PartiallyDelivered; kapama → Delivered).
      await api(`/projects/${pid}/purchase-orders/${id}/deliveries`, {
        method: "POST", projectId: pid,
        body: {
          delivery_note_no: noteNo.trim(),
          note: dNote.trim() || undefined,
          document_id: documentId,
          mark_delivered: markDelivered,
        },
      });
      setNoteNo(""); setDNote(""); setMarkDelivered(false);
      if (fileRef.current) fileRef.current.value = "";
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
            <span className="text-xs text-beton-400">
              {new Date(d.delivered_at).toLocaleString("tr-TR")} · {d.received_by_name}
            </span>
            {d.note && <span className="text-xs text-beton-400">— {d.note}</span>}
            {d.document_id && can("documents.download") && (
              <button onClick={() => downloadWaybill(d.document_id!)}
                className="ml-auto text-xs text-emniyet-500 hover:underline">
                İrsaliye fotoğrafı
              </button>
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
            <label className="flex-1 min-w-[160px] text-xs text-beton-300">
              Not
              <input value={dNote} onChange={(e) => setDNote(e.target.value)}
                className="mt-1 block w-full rounded-md bg-beton-950 border border-beton-800 px-2 py-1 text-sm text-beton-100" />
            </label>
          </div>
          <div className="flex flex-wrap items-center gap-4">
            {/* capture: mobil sahada doğrudan kamera açılır (PWA) */}
            <input ref={fileRef} type="file" accept="image/*" capture="environment"
              className="text-xs text-beton-300 file:mr-2 file:rounded file:border-0 file:bg-beton-800 file:px-2 file:py-1 file:text-beton-200" />
            <label className="flex items-center gap-2 text-xs text-beton-300">
              <input type="checkbox" checked={markDelivered}
                onChange={(e) => setMarkDelivered(e.target.checked)} />
              Bu teslimatla sipariş tamamlandı
            </label>
            <button onClick={addDelivery} disabled={busy || !noteNo.trim()}
              className="rounded-md bg-emniyet-500 px-3 py-1.5 text-xs font-semibold text-beton-950 hover:bg-emniyet-400 disabled:opacity-40">
              {busy ? "Kaydediliyor…" : "Kaydet"}
            </button>
          </div>
        </div>
      )}
    </div>
  );
}
