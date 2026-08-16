import { useCallback, useEffect, useState } from "react";
import { api } from "../../api/client";
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

  const load = useCallback(async () => {
    if (!pid) return;
    setErr(null);
    try {
      const r = await api<{ transfers: Transfer[] }>(
        `/projects/${pid}/equipment-transfers?status=pending`, { projectId: pid });
      setTransfers(r.transfers ?? []);
    } catch { setErr("Bekleyen talepler yüklenemedi ya da erişim yetkiniz yok."); }
  }, [pid]);

  useEffect(() => { load(); }, [load]);

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
