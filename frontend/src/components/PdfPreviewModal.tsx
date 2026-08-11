import { useEffect, useState } from "react";
import { apiDownload, apiFetchBlob } from "../api/client";

// Haftalık/günlük rapor PDF'lerini önizlemek için ortak modal: blob getirip
// <iframe> içinde gösterir, ayrı bir "Cihaza İndir" aksiyonu sunar.
export default function PdfPreviewModal({
  open, onClose, title, fetchPath, downloadPath, downloadName,
}: {
  open: boolean;
  onClose: () => void;
  title: string;
  fetchPath: string;
  downloadPath?: string;
  downloadName: string;
}) {
  const [url, setUrl] = useState<string | null>(null);
  const [err, setErr] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const [downloading, setDownloading] = useState(false);

  useEffect(() => {
    if (!open) return;
    let objectUrl: string | null = null;
    setErr(null);
    setUrl(null);
    setLoading(true);
    apiFetchBlob(fetchPath)
      .then((u) => { objectUrl = u; setUrl(u); })
      .catch(() => setErr("PDF yüklenemedi."))
      .finally(() => setLoading(false));
    return () => { if (objectUrl) URL.revokeObjectURL(objectUrl); };
  }, [open, fetchPath]);

  if (!open) return null;

  async function download() {
    setDownloading(true);
    try {
      await apiDownload(downloadPath ?? fetchPath, downloadName);
    } catch {
      setErr("İndirme başarısız.");
    } finally {
      setDownloading(false);
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4">
      <div className="bg-beton-900 border border-beton-700 rounded-xl shadow-2xl w-full max-w-3xl h-[85vh] flex flex-col overflow-hidden">
        <div className="flex items-center justify-between gap-3 px-4 py-3 border-b border-beton-800 shrink-0">
          <h2 className="text-sm font-bold text-beton-100 truncate">{title}</h2>
          <div className="flex items-center gap-2 shrink-0">
            <button
              onClick={download}
              disabled={downloading || !url}
              className="rounded-md bg-emniyet-500 hover:bg-emniyet-600 text-beton-950 disabled:opacity-50 px-3 py-1.5 text-xs font-medium transition-colors"
            >
              {downloading ? "İndiriliyor…" : "Cihaza İndir"}
            </button>
            <button
              onClick={onClose}
              className="rounded-md border border-beton-700 px-3 py-1.5 text-xs text-beton-300 hover:border-emniyet-500 transition-colors"
            >
              Kapat
            </button>
          </div>
        </div>
        <div className="flex-1 bg-beton-950 min-h-0">
          {loading && <p className="text-sm text-beton-500 p-6">PDF hazırlanıyor…</p>}
          {err && <p className="text-sm text-red-400 p-6">{err}</p>}
          {url && !loading && !err && (
            <iframe src={url} title={title} className="w-full h-full border-0" />
          )}
        </div>
      </div>
    </div>
  );
}
