import { useCallback, useEffect, useRef, useState } from "react";
import { api, apiUpload } from "../../api/client";
import { useAuth } from "../../auth/AuthContext";
import { useProjects } from "../ProjectContext";
import ContractPanel from "./ContractPanel";

type Sub = {
  id: string; company_name: string; tax_no?: string; contact_person?: string;
  phone?: string; email?: string; trade?: string; row_version: number;
};
type WorkItem = {
  id: string; poz_no: string; description: string; unit: string;
  contract_qty: number; unit_price?: number; contract_amount?: number; row_version: number;
};

// Tedarikçi tipi (localStorage'da saklanır)
type Tedarikci = {
  id: string; company_name: string; trade?: string; contact_person?: string;
  phone?: string; email?: string; tax_no?: string;
};

function tedarikcilerKey(pid: string) { return `ipks_tedarikciler_${pid}`; }

function loadTedarikciler(pid: string): Tedarikci[] {
  try { return JSON.parse(localStorage.getItem(tedarikcilerKey(pid)) || "[]"); } catch { return []; }
}

function saveTedarikciler(pid: string, data: Tedarikci[]) {
  localStorage.setItem(tedarikcilerKey(pid), JSON.stringify(data));
}

type ActiveType = "taseron" | "tedarikci";

export default function SubcontractorsPage() {
  const { current } = useProjects();
  const { can } = useAuth();
  const [subs, setSubs] = useState<Sub[]>([]);
  const [tedarikciler, setTedarikciler] = useState<Tedarikci[]>([]);
  const [sel, setSel] = useState<string | null>(null);
  const [activeType, setActiveType] = useState<ActiveType>("taseron");
  const [err, setErr] = useState<string | null>(null);
  const pid = current?.id;

  const load = useCallback(async () => {
    if (!pid) return;
    setErr(null);
    try {
      const r = await api<{ subcontractors: Sub[] }>(`/projects/${pid}/subcontractors`, { projectId: pid });
      setSubs(r.subcontractors);
      if (activeType === "taseron" && !sel && r.subcontractors.length) setSel(r.subcontractors[0].id);
    } catch {
      setErr("Taşeronlar yüklenemedi ya da erişim yetkiniz yok.");
    }
    setTedarikciler(loadTedarikciler(pid));
  }, [pid, sel, activeType]);

  useEffect(() => { load(); }, [load]);

  function handleSelectTaseron(id: string) {
    setActiveType("taseron");
    setSel(id);
  }

  function handleSelectTedarikci(id: string) {
    setActiveType("tedarikci");
    setSel(id);
  }

  function onTedarikciChanged(data: Tedarikci[]) {
    if (!pid) return;
    saveTedarikciler(pid, data);
    setTedarikciler(data);
  }

  if (!current) return <p className="text-beton-400">Önce üst bardan bir proje seçin.</p>;

  const selTedarikci = activeType === "tedarikci" ? tedarikciler.find(t => t.id === sel) : null;

  return (
    <div>
      <h1 className="font-display text-2xl font-extrabold text-white">Taşeronlar & Tedarikçiler</h1>
      <p className="text-sm text-beton-400 mt-1">{current.name} — firma kartları ve sözleşme bilgileri.</p>
      {err && <p className="mt-3 text-sm text-red-400">{err}</p>}

      <div className="mt-4 grid gap-4 md:grid-cols-[300px_1fr]">
        {/* Sol panel — iki grup */}
        <div className="space-y-3">
          {/* Taşeronlar */}
          <SubPanel
            projectId={pid!} subs={subs}
            selected={activeType === "taseron" ? sel : null}
            onSelect={handleSelectTaseron}
            canManage={can("contracts.upload")} onChanged={load}
          />
          {/* Tedarikçiler */}
          <TedarikciPanel
            tedarikciler={tedarikciler}
            selected={activeType === "tedarikci" ? sel : null}
            onSelect={handleSelectTedarikci}
            canManage={can("contracts.upload")}
            onChanged={onTedarikciChanged}
          />
        </div>

        {/* Sağ panel */}
        {activeType === "taseron" && sel ? (
          <div className="space-y-4">
            <ContractPanel
              projectId={pid!} subId={sel}
              canManage={can("contracts.upload")}
              canFin={can("progress_payments.view_financials")}
            />
            <WorkItemPanel
              projectId={pid!} subId={sel}
              canManage={can("contracts.upload")} canDelete={can("contracts.delete")}
              canFin={can("progress_payments.view_financials")}
            />
          </div>
        ) : activeType === "tedarikci" && selTedarikci ? (
          <div className="rounded-lg border border-beton-800 bg-beton-900 p-4 space-y-3">
            <h2 className="font-display font-bold text-white text-lg">{selTedarikci.company_name}</h2>
            {selTedarikci.trade && <p className="text-beton-400 text-sm">Branş: {selTedarikci.trade}</p>}
            {selTedarikci.contact_person && <p className="text-beton-400 text-sm">İletişim: {selTedarikci.contact_person}</p>}
            {selTedarikci.phone && <p className="text-beton-400 text-sm">Tel: {selTedarikci.phone}</p>}
            {selTedarikci.email && <p className="text-beton-400 text-sm">E-posta: {selTedarikci.email}</p>}
            {selTedarikci.tax_no && <p className="text-beton-400 text-sm">Vergi No: {selTedarikci.tax_no}</p>}
            <p className="text-xs text-beton-500 mt-4">Tedarikçi sözleşme ve ekstreler yakında eklenecek.</p>
          </div>
        ) : (
          <div className="rounded-lg border border-beton-800 bg-beton-900 p-4 text-beton-400 text-sm">
            Soldan bir taşeron veya tedarikçi seçin.
          </div>
        )}
      </div>
    </div>
  );
}

// ── Taşeron Paneli ────────────────────────────────────────────────────
function SubPanel({ projectId, subs, selected, onSelect, canManage, onChanged }: {
  projectId: string; subs: Sub[]; selected: string | null;
  onSelect: (id: string) => void; canManage: boolean; onChanged: () => void;
}) {
  const [name, setName] = useState("");
  const [trade, setTrade] = useState("");
  const [busy, setBusy] = useState(false);

  async function add() {
    if (!name.trim()) return;
    setBusy(true);
    try {
      await api(`/projects/${projectId}/subcontractors`, {
        method: "POST", projectId,
        body: { company_name: name.trim(), trade: trade.trim() || null },
      });
      setName(""); setTrade(""); onChanged();
    } finally { setBusy(false); }
  }

  return (
    <div className="rounded-lg border border-beton-800 bg-beton-900 p-3">
      <h2 className="font-display font-bold text-white text-sm uppercase tracking-wide mb-2">🏗️ Taşeronlar</h2>
      <ul className="space-y-1">
        {subs.map((s) => (
          <li key={s.id}>
            <button
              onClick={() => onSelect(s.id)}
              className={`w-full text-left rounded px-2 py-1.5 text-sm ${
                selected === s.id ? "bg-emniyet-500 text-beton-950 font-semibold" : "text-beton-200 hover:bg-beton-800"
              }`}
            >
              {s.company_name}
              {s.trade && <span className="block text-xs opacity-70">{s.trade}</span>}
            </button>
          </li>
        ))}
        {!subs.length && <li className="text-beton-400 text-sm px-2 py-1">Henüz taşeron yok.</li>}
      </ul>

      {canManage && (
        <div className="mt-3 border-t border-beton-800 pt-3 space-y-2">
          <input
            value={name} onChange={(e) => setName(e.target.value)} placeholder="Firma adı"
            className="w-full rounded bg-beton-950 border border-beton-800 px-2 py-1.5 text-sm text-white"
          />
          <input
            value={trade} onChange={(e) => setTrade(e.target.value)} placeholder="Branş (ör. Kalıp)"
            className="w-full rounded bg-beton-950 border border-beton-800 px-2 py-1.5 text-sm text-white"
          />
          <button
            onClick={add} disabled={busy}
            className="w-full rounded bg-emniyet-500 hover:bg-emniyet-600 text-beton-950 font-semibold text-sm py-1.5 disabled:opacity-50"
          >
            Taşeron Ekle
          </button>
        </div>
      )}
    </div>
  );
}

// ── Tedarikçi Paneli ──────────────────────────────────────────────────
function TedarikciPanel({ tedarikciler, selected, onSelect, canManage, onChanged }: {
  tedarikciler: Tedarikci[]; selected: string | null;
  onSelect: (id: string) => void; canManage: boolean;
  onChanged: (data: Tedarikci[]) => void;
}) {
  const [name, setName] = useState("");
  const [trade, setTrade] = useState("");

  function add() {
    if (!name.trim()) return;
    const newT: Tedarikci = {
      id: Math.random().toString(36).slice(2, 10),
      company_name: name.trim(),
      trade: trade.trim() || undefined,
    };
    onChanged([...tedarikciler, newT]);
    setName(""); setTrade("");
  }

  function sil(id: string) {
    if (!confirm("Bu tedarikçiyi silmek istediğinize emin misiniz?")) return;
    onChanged(tedarikciler.filter(t => t.id !== id));
  }

  return (
    <div className="rounded-lg border border-beton-800 bg-beton-900 p-3">
      <h2 className="font-display font-bold text-white text-sm uppercase tracking-wide mb-2">📦 Tedarikçiler</h2>
      <ul className="space-y-1">
        {tedarikciler.map((t) => (
          <li key={t.id} className="flex items-center gap-1">
            <button
              onClick={() => onSelect(t.id)}
              className={`flex-1 text-left rounded px-2 py-1.5 text-sm ${
                selected === t.id ? "bg-emniyet-500 text-beton-950 font-semibold" : "text-beton-200 hover:bg-beton-800"
              }`}
            >
              {t.company_name}
              {t.trade && <span className="block text-xs opacity-70">{t.trade}</span>}
            </button>
            {canManage && (
              <button onClick={() => sil(t.id)} className="text-red-400 hover:text-red-300 text-xs px-1">✕</button>
            )}
          </li>
        ))}
        {!tedarikciler.length && <li className="text-beton-400 text-sm px-2 py-1">Henüz tedarikçi yok.</li>}
      </ul>

      {canManage && (
        <div className="mt-3 border-t border-beton-800 pt-3 space-y-2">
          <input
            value={name} onChange={(e) => setName(e.target.value)} placeholder="Firma adı"
            className="w-full rounded bg-beton-950 border border-beton-800 px-2 py-1.5 text-sm text-white"
          />
          <input
            value={trade} onChange={(e) => setTrade(e.target.value)} placeholder="Branş / Ürün grubu"
            className="w-full rounded bg-beton-950 border border-beton-800 px-2 py-1.5 text-sm text-white"
          />
          <button
            onClick={add}
            className="w-full rounded bg-emniyet-500 hover:bg-emniyet-600 text-beton-950 font-semibold text-sm py-1.5"
          >
            Tedarikçi Ekle
          </button>
        </div>
      )}
    </div>
  );
}

// ── İş Kalemi Paneli ──────────────────────────────────────────────────
function WorkItemPanel({ projectId, subId, canManage, canDelete, canFin }: {
  projectId: string; subId: string; canManage: boolean; canDelete: boolean; canFin: boolean;
}) {
  const [items, setItems] = useState<WorkItem[]>([]);
  const [err, setErr] = useState<string | null>(null);
  const [msg, setMsg] = useState<string | null>(null);
  const fileRef = useRef<HTMLInputElement>(null);
  const [f, setF] = useState({ poz_no: "", description: "", unit: "ad", contract_qty: "", unit_price: "" });

  const load = useCallback(async () => {
    setErr(null);
    try {
      const r = await api<{ work_items: WorkItem[] }>(
        `/projects/${projectId}/subcontractors/${subId}/work-items`, { projectId });
      setItems(r.work_items);
    } catch { setErr("Birim fiyat cetveli yüklenemedi."); }
  }, [projectId, subId]);

  useEffect(() => { load(); }, [load]);

  async function add() {
    if (!f.poz_no.trim() || !f.description.trim()) return;
    await api(`/projects/${projectId}/subcontractors/${subId}/work-items`, {
      method: "POST", projectId,
      body: {
        poz_no: f.poz_no.trim(), description: f.description.trim(), unit: f.unit || "ad",
        contract_qty: Number(f.contract_qty) || 0, unit_price: Number(f.unit_price) || 0,
      },
    });
    setF({ poz_no: "", description: "", unit: "ad", contract_qty: "", unit_price: "" });
    load();
  }

  async function del(id: string) {
    await api(`/projects/${projectId}/subcontractors/${subId}/work-items/${id}`, { method: "DELETE", projectId });
    load();
  }

  async function doImport(file: File) {
    setMsg(null); setErr(null);
    try {
      const fd = new FormData();
      fd.append("file", file);
      const r = await apiUpload<{ processed: number; skipped: number; total: number }>(
        `/projects/${projectId}/subcontractors/${subId}/work-items/import`, fd);
      setMsg(`${r.processed} satır işlendi, ${r.skipped} atlandı (toplam ${r.total}).`);
      load();
    } catch { setErr("İçe aktarma başarısız. .xlsx/.csv ve sütun düzenini kontrol edin."); }
  }

  return (
    <div className="rounded-lg border border-beton-800 bg-beton-900 p-3">
      <div className="flex items-center justify-between">
        <h2 className="font-display font-bold text-white text-sm uppercase tracking-wide">Birim Fiyat Cetveli</h2>
        {canManage && (
          <div className="flex gap-2">
            <input
              ref={fileRef} type="file" accept=".xlsx,.csv" className="hidden"
              onChange={(e) => { const file = e.target.files?.[0]; if (file) doImport(file); e.currentTarget.value = ""; }}
            />
            <button
              onClick={() => fileRef.current?.click()}
              className="rounded border border-emniyet-500 text-emniyet-500 hover:bg-emniyet-500 hover:text-beton-950 text-xs font-semibold px-2 py-1"
            >
              Excel/CSV İçe Aktar
            </button>
          </div>
        )}
      </div>
      {err && <p className="mt-2 text-sm text-red-400">{err}</p>}
      {msg && <p className="mt-2 text-sm text-emniyet-500">{msg}</p>}

      <table className="mt-3 w-full text-sm">
        <thead>
          <tr className="text-beton-400 text-left border-b border-beton-800">
            <th className="py-1 pr-2">Poz</th>
            <th className="py-1 pr-2">Açıklama</th>
            <th className="py-1 pr-2">Birim</th>
            <th className="py-1 pr-2 text-right">Söz. Mik.</th>
            {canFin && <th className="py-1 pr-2 text-right">B. Fiyat</th>}
            {canFin && <th className="py-1 pr-2 text-right">Tutar</th>}
            {canDelete && <th></th>}
          </tr>
        </thead>
        <tbody>
          {items.map((wi) => (
            <tr key={wi.id} className="border-b border-beton-800/50 text-beton-200">
              <td className="py-1 pr-2 font-mono">{wi.poz_no}</td>
              <td className="py-1 pr-2">{wi.description}</td>
              <td className="py-1 pr-2">{wi.unit}</td>
              <td className="py-1 pr-2 text-right tabular-nums">{wi.contract_qty}</td>
              {canFin && <td className="py-1 pr-2 text-right tabular-nums">{wi.unit_price?.toLocaleString("tr-TR")}</td>}
              {canFin && <td className="py-1 pr-2 text-right tabular-nums">{wi.contract_amount?.toLocaleString("tr-TR")}</td>}
              {canDelete && (
                <td className="py-1 text-right">
                  <button onClick={() => del(wi.id)} className="text-red-400 hover:text-red-300 text-xs">Sil</button>
                </td>
              )}
            </tr>
          ))}
          {!items.length && (
            <tr><td colSpan={7} className="py-3 text-beton-400 text-center">Poz yok. Elle ekleyin veya içe aktarın.</td></tr>
          )}
        </tbody>
      </table>

      {canManage && (
        <div className="mt-3 grid grid-cols-2 md:grid-cols-6 gap-2 border-t border-beton-800 pt-3">
          <input value={f.poz_no} onChange={(e) => setF({ ...f, poz_no: e.target.value })} placeholder="Poz no"
            className="rounded bg-beton-950 border border-beton-800 px-2 py-1.5 text-sm text-white font-mono" />
          <input value={f.description} onChange={(e) => setF({ ...f, description: e.target.value })} placeholder="Açıklama"
            className="col-span-2 rounded bg-beton-950 border border-beton-800 px-2 py-1.5 text-sm text-white" />
          <input value={f.unit} onChange={(e) => setF({ ...f, unit: e.target.value })} placeholder="Birim"
            className="rounded bg-beton-950 border border-beton-800 px-2 py-1.5 text-sm text-white" />
          <input value={f.contract_qty} onChange={(e) => setF({ ...f, contract_qty: e.target.value })} placeholder="Miktar" inputMode="decimal"
            className="rounded bg-beton-950 border border-beton-800 px-2 py-1.5 text-sm text-white text-right" />
          <input value={f.unit_price} onChange={(e) => setF({ ...f, unit_price: e.target.value })} placeholder="B. Fiyat" inputMode="decimal"
            className="rounded bg-beton-950 border border-beton-800 px-2 py-1.5 text-sm text-white text-right" />
          <button onClick={add}
            className="col-span-2 md:col-span-6 rounded bg-emniyet-500 hover:bg-emniyet-600 text-beton-950 font-semibold text-sm py-1.5">
            Poz Ekle
          </button>
        </div>
      )}
    </div>
  );
}
