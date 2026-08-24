import { useCallback, useEffect, useRef, useState } from "react";
import { api, apiUpload } from "../../api/client";
import { useAuth } from "../../auth/AuthContext";
import { useProjects } from "../ProjectContext";

// Tedarik Planı — Satın Alma altında, mevcut PR/PO onay zincirine
// dokunmayan, zorunlu olmayan bir planlama/takip referansı. Excel/CSV'den
// toplu içe aktarılabilir (subcontractors.go'daki work_items importuyla
// aynı desen) ya da tek tek elle eklenebilir; durum elle güncellenir.

type PlanItem = {
  id: string;
  poz_no?: string;
  description: string;
  category?: string;
  quantity?: number;
  unit?: string;
  supplier_name?: string;
  planned_order_date?: string;
  planned_delivery_date?: string;
  criticality?: string;
  status: string;
  note?: string;
  created_by_name: string;
  row_version: number;
};

const STATUS_LABEL: Record<string, string> = {
  Planlandi: "Planlandı", SiparisVerildi: "Sipariş Verildi", Yolda: "Yolda",
  TeslimAlindi: "Teslim Alındı", Gecikti: "Gecikti",
};
const STATUS_STYLE: Record<string, string> = {
  Planlandi: "bg-beton-800 text-beton-300 border-beton-700",
  SiparisVerildi: "bg-blue-500/15 text-blue-300 border-blue-500/40",
  Yolda: "bg-yellow-500/15 text-yellow-300 border-yellow-500/40",
  TeslimAlindi: "bg-green-500/15 text-green-300 border-green-500/40",
  Gecikti: "bg-red-500/15 text-red-300 border-red-500/40",
};
const CRIT_STYLE: Record<string, string> = {
  Kritik: "bg-red-500/15 text-red-300 border-red-500/40",
  Normal: "bg-beton-800 text-beton-300 border-beton-700",
};
const CATEGORIES = ["Malzeme", "Ekipman", "Taşeron Hizmeti"];

const inp = "rounded-md bg-beton-950 border border-beton-800 px-2 py-1.5 text-sm text-beton-100 outline-none focus:border-emniyet-500";

function emptyForm() {
  return {
    poz_no: "", description: "", category: "Malzeme", quantity: "", unit: "",
    supplier_name: "", planned_order_date: "", planned_delivery_date: "",
    criticality: "", note: "",
  };
}

export default function ProcurementPlanPage() {
  const { current } = useProjects();
  const { can } = useAuth();
  const pid = current?.id;
  const canManage = can("procurement.manage_po");

  const [items, setItems] = useState<PlanItem[]>([]);
  const [err, setErr] = useState<string | null>(null);
  const [msg, setMsg] = useState<string | null>(null);
  const [showForm, setShowForm] = useState(false);
  const [form, setForm] = useState(emptyForm());
  const [saving, setSaving] = useState(false);
  const [confirmDeleteId, setConfirmDeleteId] = useState<string | null>(null);
  const fileRef = useRef<HTMLInputElement>(null);

  const load = useCallback(async () => {
    if (!pid) return;
    setErr(null);
    try {
      const r = await api<{ items: PlanItem[] }>(`/projects/${pid}/procurement/plan`, { projectId: pid });
      setItems(r.items ?? []);
    } catch {
      setErr("Tedarik planı yüklenemedi ya da erişim yetkiniz yok.");
    }
  }, [pid]);

  useEffect(() => { load(); }, [load]);

  async function create() {
    if (!pid || !form.description.trim()) return;
    setSaving(true); setErr(null);
    try {
      await api(`/projects/${pid}/procurement/plan`, {
        method: "POST", projectId: pid,
        body: {
          poz_no: form.poz_no.trim() || undefined,
          description: form.description.trim(),
          category: form.category || undefined,
          quantity: form.quantity ? Number(form.quantity) : undefined,
          unit: form.unit.trim() || undefined,
          supplier_name: form.supplier_name.trim() || undefined,
          planned_order_date: form.planned_order_date || undefined,
          planned_delivery_date: form.planned_delivery_date || undefined,
          criticality: form.criticality || undefined,
          note: form.note.trim() || undefined,
        },
      });
      setForm(emptyForm());
      setShowForm(false);
      load();
    } catch {
      setErr("Kalem eklenemedi.");
    } finally {
      setSaving(false);
    }
  }

  async function updateStatus(it: PlanItem, status: string) {
    if (!pid) return;
    try {
      await api(`/projects/${pid}/procurement/plan/${it.id}`, {
        method: "PATCH", projectId: pid,
        body: {
          poz_no: it.poz_no, description: it.description, category: it.category,
          quantity: it.quantity, unit: it.unit, supplier_name: it.supplier_name,
          planned_order_date: it.planned_order_date, planned_delivery_date: it.planned_delivery_date,
          criticality: it.criticality, note: it.note, status, row_version: it.row_version,
        },
      });
      load();
    } catch {
      setErr("Durum güncellenemedi.");
    }
  }

  async function doDelete(id: string) {
    if (!pid) return;
    await api(`/projects/${pid}/procurement/plan/${id}`, { method: "DELETE", projectId: pid });
    setConfirmDeleteId(null);
    load();
  }

  async function doImport(file: File) {
    if (!pid) return;
    setMsg(null); setErr(null);
    try {
      const fd = new FormData();
      fd.append("file", file);
      const r = await apiUpload<{ processed: number; skipped: number; total: number }>(
        `/projects/${pid}/procurement/plan/import`, fd);
      setMsg(`${r.processed} satır işlendi, ${r.skipped} atlandı (toplam ${r.total}).`);
      load();
    } catch {
      setErr("İçe aktarma başarısız. .xlsx/.csv ve sütun düzenini kontrol edin (A=poz no, B=tanım*, C=kategori, D=miktar, E=birim, F=tedarikçi, G=planlanan sipariş tarihi, H=planlanan teslim tarihi, I=kritiklik, J=not).");
    }
  }

  if (!current) return <p className="text-beton-400">Önce üst bardan bir proje seçin.</p>;

  return (
    <div className="space-y-4">
      <div className="flex items-center gap-3 flex-wrap">
        <div>
          <h1 className="text-lg font-display font-medium text-beton-100">Tedarik Planı</h1>
          <p className="text-xs text-beton-500 mt-0.5">
            Satın alma sürecinden bağımsız, zorunlu olmayan planlama/takip referansı.
          </p>
        </div>
        {canManage && (
          <div className="ml-auto flex gap-2">
            <input ref={fileRef} type="file" accept=".xlsx,.csv" className="hidden"
              onChange={(e) => { const f = e.target.files?.[0]; if (f) doImport(f); e.currentTarget.value = ""; }} />
            <button onClick={() => fileRef.current?.click()}
              className="rounded-md border border-emniyet-500 text-emniyet-500 hover:bg-emniyet-500 hover:text-beton-950 text-xs font-semibold px-3 py-1.5 transition-colors">
              Excel/CSV İçe Aktar
            </button>
            <button onClick={() => setShowForm((v) => !v)}
              className="rounded-md bg-emniyet-500 hover:bg-emniyet-600 text-beton-950 text-xs font-semibold px-3 py-1.5">
              {showForm ? "Vazgeç" : "+ Kalem Ekle"}
            </button>
          </div>
        )}
      </div>
      {err && <p className="text-sm text-red-400">{err}</p>}
      {msg && <p className="text-sm text-emniyet-500">{msg}</p>}

      {showForm && (
        <div className="rounded-lg border border-beton-800 bg-beton-900 p-4 space-y-3">
          <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
            <label className="text-xs text-beton-400">Poz No
              <input value={form.poz_no} onChange={(e) => setForm({ ...form, poz_no: e.target.value })}
                className={`${inp} w-full mt-1`} />
            </label>
            <label className="text-xs text-beton-400 md:col-span-2">Malzeme / İş Kalemi Tanımı *
              <input value={form.description} onChange={(e) => setForm({ ...form, description: e.target.value })}
                className={`${inp} w-full mt-1`} />
            </label>
            <label className="text-xs text-beton-400">Kategori
              <select value={form.category} onChange={(e) => setForm({ ...form, category: e.target.value })}
                className={`${inp} w-full mt-1`}>
                {CATEGORIES.map((c) => <option key={c} value={c}>{c}</option>)}
              </select>
            </label>
            <label className="text-xs text-beton-400">Miktar
              <input type="number" value={form.quantity} onChange={(e) => setForm({ ...form, quantity: e.target.value })}
                className={`${inp} w-full mt-1`} />
            </label>
            <label className="text-xs text-beton-400">Birim
              <input value={form.unit} onChange={(e) => setForm({ ...form, unit: e.target.value })}
                className={`${inp} w-full mt-1`} />
            </label>
            <label className="text-xs text-beton-400">Tedarikçi / Taşeron
              <input value={form.supplier_name} onChange={(e) => setForm({ ...form, supplier_name: e.target.value })}
                className={`${inp} w-full mt-1`} />
            </label>
            <label className="text-xs text-beton-400">Kritiklik
              <select value={form.criticality} onChange={(e) => setForm({ ...form, criticality: e.target.value })}
                className={`${inp} w-full mt-1`}>
                <option value="">—</option>
                <option value="Kritik">Kritik</option>
                <option value="Normal">Normal</option>
              </select>
            </label>
            <label className="text-xs text-beton-400">Planlanan Sipariş Tarihi
              <input type="date" value={form.planned_order_date} onChange={(e) => setForm({ ...form, planned_order_date: e.target.value })}
                className={`${inp} w-full mt-1`} />
            </label>
            <label className="text-xs text-beton-400">Planlanan Teslim Tarihi
              <input type="date" value={form.planned_delivery_date} onChange={(e) => setForm({ ...form, planned_delivery_date: e.target.value })}
                className={`${inp} w-full mt-1`} />
            </label>
            <label className="text-xs text-beton-400 md:col-span-2">Not
              <input value={form.note} onChange={(e) => setForm({ ...form, note: e.target.value })}
                className={`${inp} w-full mt-1`} />
            </label>
          </div>
          <button onClick={create} disabled={saving || !form.description.trim()}
            className="rounded-md bg-emniyet-500 hover:bg-emniyet-600 text-beton-950 text-sm font-semibold px-4 py-1.5 disabled:opacity-40">
            {saving ? "Kaydediliyor…" : "Kaydet"}
          </button>
        </div>
      )}

      <div className="overflow-x-auto rounded-lg border border-beton-800">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-beton-800 bg-beton-900/60">
              <th className="text-left text-[10px] font-bold uppercase tracking-wider text-beton-400 py-2 px-3">Poz No</th>
              <th className="text-left text-[10px] font-bold uppercase tracking-wider text-beton-400 py-2 px-3">Tanım</th>
              <th className="text-left text-[10px] font-bold uppercase tracking-wider text-beton-400 py-2 px-3">Kategori</th>
              <th className="text-left text-[10px] font-bold uppercase tracking-wider text-beton-400 py-2 px-3">Miktar</th>
              <th className="text-left text-[10px] font-bold uppercase tracking-wider text-beton-400 py-2 px-3">Tedarikçi</th>
              <th className="text-left text-[10px] font-bold uppercase tracking-wider text-beton-400 py-2 px-3">Plan. Sipariş</th>
              <th className="text-left text-[10px] font-bold uppercase tracking-wider text-beton-400 py-2 px-3">Plan. Teslim</th>
              <th className="text-left text-[10px] font-bold uppercase tracking-wider text-beton-400 py-2 px-3">Kritiklik</th>
              <th className="text-left text-[10px] font-bold uppercase tracking-wider text-beton-400 py-2 px-3">Durum</th>
              {canManage && <th className="py-2 px-3" />}
            </tr>
          </thead>
          <tbody>
            {items.map((it) => (
              <tr key={it.id} className="border-b border-beton-800/60 hover:bg-beton-900/30">
                <td className="py-2 px-3 text-beton-400 text-xs font-mono">{it.poz_no ?? "—"}</td>
                <td className="py-2 px-3 text-beton-100">
                  {it.description}
                  {it.note && <p className="text-[11px] text-beton-500 mt-0.5">{it.note}</p>}
                </td>
                <td className="py-2 px-3 text-beton-300 text-xs">{it.category ?? "—"}</td>
                <td className="py-2 px-3 text-beton-300 text-xs">{it.quantity != null ? `${it.quantity} ${it.unit ?? ""}` : "—"}</td>
                <td className="py-2 px-3 text-beton-300 text-xs">{it.supplier_name ?? "—"}</td>
                <td className="py-2 px-3 text-beton-400 text-xs">{it.planned_order_date ?? "—"}</td>
                <td className="py-2 px-3 text-beton-400 text-xs">{it.planned_delivery_date ?? "—"}</td>
                <td className="py-2 px-3">
                  {it.criticality ? (
                    <span className={`rounded-full border px-2 py-0.5 text-[10.5px] font-semibold ${CRIT_STYLE[it.criticality]}`}>{it.criticality}</span>
                  ) : "—"}
                </td>
                <td className="py-2 px-3">
                  {canManage ? (
                    <select value={it.status} onChange={(e) => updateStatus(it, e.target.value)}
                      className={`rounded-full border px-2 py-0.5 text-[10.5px] font-semibold bg-transparent outline-none ${STATUS_STYLE[it.status]}`}>
                      {Object.entries(STATUS_LABEL).map(([v, l]) => <option key={v} value={v} className="bg-beton-900 text-beton-100">{l}</option>)}
                    </select>
                  ) : (
                    <span className={`rounded-full border px-2 py-0.5 text-[10.5px] font-semibold ${STATUS_STYLE[it.status]}`}>{STATUS_LABEL[it.status]}</span>
                  )}
                </td>
                {canManage && (
                  <td className="py-2 px-3 text-right">
                    <button onClick={() => setConfirmDeleteId(it.id)} className="text-xs text-red-400 hover:underline">Sil</button>
                  </td>
                )}
              </tr>
            ))}
            {items.length === 0 && (
              <tr><td colSpan={canManage ? 10 : 9} className="py-6 px-3 text-center text-beton-500 text-sm">
                Henüz kalem yok. {canManage && "Elle ekleyin ya da Excel/CSV'den içe aktarın."}
              </td></tr>
            )}
          </tbody>
        </table>
      </div>

      {confirmDeleteId && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60" onClick={() => setConfirmDeleteId(null)}>
          <div className="bg-beton-900 border border-beton-700 rounded-xl w-full max-w-sm mx-4 p-6 space-y-4" onClick={(e) => e.stopPropagation()}>
            <h2 className="font-display font-bold text-white text-lg">Kalemi Sil</h2>
            <p className="text-sm text-beton-300">Bu tedarik planı kalemini silmek istediğinize emin misiniz?</p>
            <div className="flex gap-3">
              <button onClick={() => setConfirmDeleteId(null)} className="flex-1 rounded-md border border-beton-700 px-4 py-2 text-sm text-beton-300 hover:bg-beton-800">Vazgeç</button>
              <button onClick={() => doDelete(confirmDeleteId)} className="flex-1 rounded-md bg-red-500 px-4 py-2 text-sm font-medium text-white-solid hover:brightness-110">Sil</button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
