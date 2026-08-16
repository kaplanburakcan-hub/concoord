import { useCallback, useEffect, useState } from "react";
import { api, apiFetchBlob, apiUpload } from "../../api/client";
import { useProjects } from "../ProjectContext";

// Makine/Ekipman/Araç Envanteri Faz B — proje-arası transfer talebi onayı.
// Bir başka proje sizin projenizdeki bir makine/ekipman/aracı talep
// ettiğinde burada bekleyen bir talep olarak listelenir.

type Transfer = {
  id: string;
  company_equipment_id: string;
  equipment_ad: string;
  equipment_tip: string;
  to_project_name: string;
  requested_by_name: string;
  status: string;
  created_at: string;
};

const TIP_LABEL: Record<string, string> = {
  arac: "Araç",
  is_makinesi: "İş Makinesi",
  ekipman: "Ekipman",
};

export default function TransferTalepleriPage() {
  const { current } = useProjects();
  const pid = current?.id;
  const [transfers, setTransfers] = useState<Transfer[]>([]);
  const [notes, setNotes] = useState<Record<string, string>>({});
  const [busy, setBusy] = useState<string | null>(null);
  const [err, setErr] = useState<string | null>(null);

  // Nakliye irsaliyesi — Faz C: fiziksel nakliye yapılan talepler için
  // transfer talebine bağlı irsaliye belgesi (entity_type=equipment_transfer_requests).
  const [irsaliyeByTransfer, setIrsaliyeByTransfer] = useState<Record<string, { id: string; title: string; url: string }[]>>({});
  const [irsaliyeUploading, setIrsaliyeUploading] = useState<string | null>(null);

  const load = useCallback(async () => {
    if (!pid) return;
    setErr(null);
    try {
      const r = await api<{ transfers: Transfer[] }>(
        `/projects/${pid}/equipment-transfers?status=pending`, { projectId: pid });
      setTransfers(r.transfers ?? []);
      for (const t of r.transfers ?? []) loadIrsaliye(t.id);
    } catch { setErr("Bekleyen talepler yüklenemedi ya da erişim yetkiniz yok."); }
  }, [pid]);

  useEffect(() => { load(); }, [load]);

  async function loadIrsaliye(transferId: string) {
    if (!pid) return;
    try {
      const d = await api<{ documents: { id: string; title: string; latest_version?: number }[] }>(
        `/projects/${pid}/documents?entity_type=equipment_transfer_requests&entity_id=${transferId}&category=NakliyeIrsaliyesi`,
        { projectId: pid });
      const withUrls = await Promise.all(
        (d.documents ?? []).filter(x => x.latest_version).map(async (doc) => {
          const url = await apiFetchBlob(`/projects/${pid}/documents/${doc.id}/versions/${doc.latest_version}/download`);
          return { id: doc.id, title: doc.title, url };
        })
      );
      setIrsaliyeByTransfer(prev => ({ ...prev, [transferId]: withUrls }));
    } catch { /* sessizce boş listede kalır */ }
  }

  async function uploadIrsaliye(files: FileList | null, transferId: string) {
    if (!files || !pid) return;
    setIrsaliyeUploading(transferId);
    try {
      for (const file of Array.from(files)) {
        if (file.size > 20 * 1024 * 1024) { alert(`${file.name} 20MB sınırını aşıyor.`); continue; }
        const doc = await api<{ document: { id: string } }>(`/projects/${pid}/documents`, {
          method: "POST", projectId: pid,
          body: { title: file.name, doc_category: "NakliyeIrsaliyesi", entity_type: "equipment_transfer_requests", entity_id: transferId },
        });
        const fd = new FormData();
        fd.append("file", file);
        await apiUpload(`/projects/${pid}/documents/${doc.document.id}/versions`, fd);
      }
      await loadIrsaliye(transferId);
    } catch {
      setErr("Nakliye irsaliyesi yüklenemedi.");
    } finally {
      setIrsaliyeUploading(null);
    }
  }

  async function decide(id: string, approve: boolean) {
    if (!pid) return;
    const note = notes[id] ?? "";
    if (!approve && !note.trim()) {
      setErr("Ret gerekçesi zorunludur.");
      return;
    }
    setBusy(id);
    setErr(null);
    try {
      await api(`/projects/${pid}/equipment-transfers/${id}/${approve ? "approve" : "reject"}`, {
        method: "POST", projectId: pid, body: { decision_note: note.trim() },
      });
      setNotes((n) => ({ ...n, [id]: "" }));
      load();
    } catch { setErr("İşlem tamamlanamadı."); } finally { setBusy(null); }
  }

  if (!current) return <p className="text-beton-400">Önce üst bardan bir proje seçin.</p>;

  return (
    <div className="max-w-4xl">
      <h1 className="font-display text-2xl font-extrabold text-white">Transfer Talepleri</h1>
      <p className="mt-1 text-sm text-beton-400">
        Başka projelerin sizin projenizden talep ettiği makine/ekipman/araçlar burada onayınızı bekler.
        Onaylanırsa ekipman hedef projeye taşınır; reddedilirse sizin projenizde kalmaya devam eder.
      </p>
      {err && <p className="mt-3 text-sm text-red-400">{err}</p>}

      <div className="mt-4 space-y-3">
        {transfers.map((t) => (
          <div key={t.id} className="rounded-lg border border-beton-800 bg-beton-900 p-4">
            <div className="flex items-start justify-between gap-3">
              <div>
                <span className="rounded bg-beton-800 px-2 py-0.5 text-xs text-beton-300">
                  {TIP_LABEL[t.equipment_tip] ?? t.equipment_tip}
                </span>
                <p className="mt-1 text-white font-medium">{t.equipment_ad}</p>
                <p className="text-sm text-beton-400 mt-0.5">
                  Talep eden: {t.requested_by_name} ({t.to_project_name}) ·{" "}
                  {new Date(t.created_at).toLocaleString("tr-TR", { dateStyle: "short", timeStyle: "short" })}
                </p>
              </div>
              <div className="text-right flex-none">
                <div className="text-xs text-beton-400">Hedef proje</div>
                <div className="text-sm font-semibold text-amber-400">{t.to_project_name}</div>
              </div>
            </div>
            <div className="mt-3 border-t border-beton-800 pt-2.5">
              <div className="flex items-center justify-between mb-1.5">
                <p className="text-xs text-beton-400 uppercase tracking-wide">
                  Nakliye İrsaliyesi ({(irsaliyeByTransfer[t.id] ?? []).length})
                </p>
                <input id={`irsaliye-${t.id}`} type="file" accept="application/pdf,image/*" multiple className="hidden"
                  onChange={(e) => uploadIrsaliye(e.target.files, t.id)} />
                <button onClick={() => document.getElementById(`irsaliye-${t.id}`)?.click()}
                  disabled={irsaliyeUploading === t.id}
                  className="text-xs rounded border border-beton-700 px-2 py-1 text-beton-300 hover:border-emniyet-500 disabled:opacity-50">
                  {irsaliyeUploading === t.id ? "Yükleniyor…" : "📎 İrsaliye Ekle"}
                </button>
              </div>
              {(irsaliyeByTransfer[t.id] ?? []).length > 0 ? (
                <ul className="space-y-1">
                  {(irsaliyeByTransfer[t.id] ?? []).map((d) => (
                    <li key={d.id}>
                      <a href={d.url} download={d.title} className="text-xs text-emniyet-500 hover:underline">
                        {d.title}
                      </a>
                    </li>
                  ))}
                </ul>
              ) : (
                <p className="text-xs text-beton-500">Henüz irsaliye yüklenmedi (araçlar için gerekmeyebilir).</p>
              )}
            </div>
            <div className="mt-3 flex flex-wrap items-center gap-2">
              <input
                value={notes[t.id] ?? ""}
                onChange={(e) => setNotes((n) => ({ ...n, [t.id]: e.target.value }))}
                placeholder="Not (ret için zorunlu)"
                className="flex-1 min-w-[200px] rounded bg-beton-950 border border-beton-800 px-2 py-1.5 text-sm text-beton-100 outline-none focus:border-emniyet-500"
              />
              <button
                disabled={busy === t.id}
                onClick={() => decide(t.id, true)}
                className="rounded-md bg-emniyet-500 hover:bg-emniyet-600 disabled:opacity-60 text-beton-950 text-sm font-semibold px-4 py-1.5">
                Onayla
              </button>
              <button
                disabled={busy === t.id}
                onClick={() => decide(t.id, false)}
                className="rounded-md border border-red-500/50 text-red-300 hover:bg-red-500/10 disabled:opacity-60 text-sm px-4 py-1.5">
                Reddet
              </button>
            </div>
          </div>
        ))}
        {!transfers.length && !err && (
          <p className="text-beton-500 text-sm text-center py-10">Bekleyen transfer talebi yok.</p>
        )}
      </div>
    </div>
  );
}
