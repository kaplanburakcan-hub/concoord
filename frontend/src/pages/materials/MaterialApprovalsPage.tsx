import { useCallback, useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { api, apiDownload } from "../../api/client";
import { useAuth } from "../../auth/AuthContext";
import { useProjects } from "../ProjectContext";

// Faz 5 — MAR durum panosu: statü kolonlu, renk kodlu görünüm.
// Client rolü yalnızca kendisine sunulan (Submitted dışı) kayıtları görür;
// filtre backend'dedir, bu ekran ne dönerse onu gösterir.

export type MAR = {
  id: string; mar_no: string; material_name: string; status: string;
  spec_ref?: string; manufacturer?: string;
  subcontractor_id?: string; subcontractor_name?: string;
  decision_note?: string; decided_by_name?: string; decided_at?: string;
  created_by_name: string; attachment_count: number; created_at: string;
};
type Sub = { id: string; company_name: string };

export const MAR_STATUS_LABEL: Record<string, string> = {
  Submitted: "Sunuldu", UnderReview: "İncelemede", Approved: "Onaylandı",
  ConditionallyApproved: "Şartlı Onay", Rejected: "Reddedildi",
};
// Renk kodu (Plan Faz 5): gri → mavi → yeşil / turuncu / kırmızı
export const MAR_STATUS_STYLE: Record<string, string> = {
  Submitted: "bg-beton-800 text-beton-200 border-beton-700",
  UnderReview: "bg-blue-500/15 text-blue-300 border-blue-500/40",
  Approved: "bg-green-500/15 text-green-300 border-green-500/40",
  ConditionallyApproved: "bg-emniyet-500/15 text-emniyet-500 border-emniyet-500/40",
  Rejected: "bg-red-500/15 text-red-300 border-red-500/40",
};
const COLUMNS = ["Submitted", "UnderReview", "Approved", "ConditionallyApproved", "Rejected"];

export default function MaterialApprovalsPage() {
  const { current } = useProjects();
  const { can } = useAuth();
  const [mars, setMars] = useState<MAR[]>([]);
  const [subs, setSubs] = useState<Sub[]>([]);
  const [err, setErr] = useState<string | null>(null);
  const [showForm, setShowForm] = useState(false);
  const [name, setName] = useState("");
  const [specRef, setSpecRef] = useState("");
  const [manufacturer, setManufacturer] = useState("");
  const [subID, setSubID] = useState("");
  const pid = current?.id;

  const load = useCallback(async () => {
    if (!pid) return;
    setErr(null);
    try {
      const res = await api<{ material_approvals: MAR[] }>(`/projects/${pid}/materials`, { projectId: pid });
      setMars(res.material_approvals);
    } catch { setErr("Malzeme onayları yüklenemedi ya da erişim yetkiniz yok."); }
    try {
      const s = await api<{ subcontractors: Sub[] }>(`/projects/${pid}/subcontractors`, { projectId: pid });
      setSubs(s.subcontractors);
    } catch { /* taşeron listesi yetkisi olmayabilir (ör. Client) */ }
  }, [pid]);

  useEffect(() => { load(); }, [load]);

  async function create() {
    if (!name.trim()) return;
    try {
      await api(`/projects/${pid}/materials`, {
        method: "POST", projectId: pid,
        body: {
          material_name: name.trim(),
          spec_ref: specRef.trim() || undefined,
          manufacturer: manufacturer.trim() || undefined,
          subcontractor_id: subID || undefined,
        },
      });
      setName(""); setSpecRef(""); setManufacturer(""); setSubID(""); setShowForm(false);
      load();
    } catch { setErr("MAR oluşturulamadı."); }
  }

  if (!current) return <p className="text-beton-400">Önce üst bardan bir proje seçin.</p>;

  return (
    <div className="space-y-4">
      <div className="flex items-center gap-3 flex-wrap">
        <h1 className="text-lg font-display font-bold text-white">Malzeme Onayları (MAR)</h1>
        <span className="text-xs text-beton-500">{mars.length} kayıt</span>
        <div className="ml-auto flex gap-2">
          <button
            onClick={() => apiDownload(`/projects/${pid}/materials/register.csv`, "mar-kayit-defteri.csv")}
            className="rounded border border-beton-700 hover:border-emniyet-500 text-beton-200 px-3 py-1.5 text-xs">
            Kayıt defterini indir (CSV)
          </button>
          {can("material_approvals.create") && (
            <button onClick={() => setShowForm((v) => !v)}
              className="rounded bg-emniyet-500 hover:bg-emniyet-600 text-beton-950 font-semibold px-3 py-1.5 text-xs">
              {showForm ? "Vazgeç" : "Yeni MAR"}
            </button>
          )}
        </div>
      </div>

      {err && <p className="text-sm text-red-400">{err}</p>}

      {showForm && (
        <div className="rounded-lg border border-beton-800 bg-beton-900 p-4 grid gap-2 sm:grid-cols-2">
          <input value={name} onChange={(e) => setName(e.target.value)} placeholder="Malzeme adı *"
            className="rounded bg-beton-950 border border-beton-800 px-2 py-1.5 text-sm text-beton-100 outline-none focus:border-emniyet-500" />
          <input value={specRef} onChange={(e) => setSpecRef(e.target.value)} placeholder="Şartname referansı"
            className="rounded bg-beton-950 border border-beton-800 px-2 py-1.5 text-sm text-beton-100 outline-none focus:border-emniyet-500" />
          <input value={manufacturer} onChange={(e) => setManufacturer(e.target.value)} placeholder="Üretici"
            className="rounded bg-beton-950 border border-beton-800 px-2 py-1.5 text-sm text-beton-100 outline-none focus:border-emniyet-500" />
          {subs.length > 0 && (
            <select value={subID} onChange={(e) => setSubID(e.target.value)}
              className="rounded bg-beton-950 border border-beton-800 px-2 py-1.5 text-sm text-beton-100 outline-none focus:border-emniyet-500">
              <option value="">Taşeron (opsiyonel)</option>
              {subs.map((s) => <option key={s.id} value={s.id}>{s.company_name}</option>)}
            </select>
          )}
          <div className="sm:col-span-2">
            <button onClick={create} disabled={!name.trim()}
              className="rounded bg-emniyet-500 hover:bg-emniyet-600 disabled:opacity-60 text-beton-950 font-semibold px-4 py-1.5 text-sm">
              Sun (Submitted)
            </button>
          </div>
        </div>
      )}

      {/* Renk kodlu durum panosu */}
      <div className="grid gap-3 md:grid-cols-3 xl:grid-cols-5">
        {COLUMNS.map((st) => {
          // Not: Client görünümünde "Sunuldu" kolonu doğal olarak boştur
          // (backend filtresi); kolon yine de tutarlılık için gösterilir.
          const items = mars.filter((m) => m.status === st);
          return (
            <div key={st} className="rounded-lg border border-beton-800 bg-beton-900/60">
              <div className={"px-3 py-2 text-xs font-semibold rounded-t-lg border-b " + (MAR_STATUS_STYLE[st] || "")}>
                {MAR_STATUS_LABEL[st]} <span className="opacity-70">({items.length})</span>
              </div>
              <div className="p-2 space-y-2 min-h-[3rem]">
                {items.map((m) => (
                  <Link key={m.id} to={`/malzeme-onaylari/${m.id}`}
                    className={"block rounded-md border p-2 hover:brightness-110 transition " + (MAR_STATUS_STYLE[m.status] || "")}>
                    <div className="flex items-center justify-between gap-2">
                      <span className="font-mono text-[11px]">{m.mar_no}</span>
                      {m.attachment_count > 0 && (
                        <span className="text-[10px] opacity-80">📎 {m.attachment_count}</span>
                      )}
                    </div>
                    <p className="text-sm text-white truncate">{m.material_name}</p>
                    {m.subcontractor_name && (
                      <p className="text-[11px] opacity-80 truncate">{m.subcontractor_name}</p>
                    )}
                  </Link>
                ))}
                {items.length === 0 && <p className="text-[11px] text-beton-600 px-1">—</p>}
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
}
