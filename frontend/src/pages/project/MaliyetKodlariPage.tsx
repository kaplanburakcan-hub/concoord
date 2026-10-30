import { useCallback, useEffect, useState } from "react";
import { api, describeLoadError } from "../../api/client";
import { useAuth } from "../../auth/AuthContext";
import { useProjects } from "../../projects/ProjectContext";

// Maliyet kodu tanım listesi (E6, ilk adım). Yalnız tanım: bütçe, PO ve hakediş
// ile bağlantı bu aşamada yoktur.

type CostCode = { id: string; code: string; name: string; parent_id?: string; budget?: number; created_by_name: string; created_at: string };

const inp = "rounded bg-beton-950 border border-beton-800 px-2 py-1.5 text-sm text-beton-100 outline-none focus:border-emniyet-500";

export default function MaliyetKodlariPage() {
  const { current } = useProjects();
  const { can } = useAuth();
  const pid = current?.id;
  const canEdit = can("budget.edit");

  const [codes, setCodes] = useState<CostCode[]>([]);
  const [err, setErr] = useState<string | null>(null);
  const [showForm, setShowForm] = useState(false);
  const [code, setCode] = useState("");
  const [name, setName] = useState("");
  const [parent, setParent] = useState("");
  const [saving, setSaving] = useState(false);
  // Bütçe tutarı girişi (E6, ilk adım): yalnız giriş; EVM ve sapma hesabı yok.
  const [budgetDraft, setBudgetDraft] = useState<Record<string, string>>({});

  async function saveBudget(c: CostCode) {
    if (!pid) return;
    const raw = (budgetDraft[c.id] ?? "").trim();
    const amount = Number(raw);
    if (raw === "" || Number.isNaN(amount) || amount < 0) {
      setErr("Bütçe için 0 veya daha büyük bir tutar girin.");
      return;
    }
    try {
      await api(`/projects/${pid}/cost-codes/${c.id}/budget`, { method: "PUT", projectId: pid, body: { amount } });
      setBudgetDraft((d) => { const n = { ...d }; delete n[c.id]; return n; });
      await load();
    } catch (e) {
      setErr(describeLoadError(e, "Bütçe"));
    }
  }

  const load = useCallback(async () => {
    if (!pid) return;
    setErr(null);
    try {
      const r = await api<{ cost_codes: CostCode[] }>(`/projects/${pid}/cost-codes`, { projectId: pid });
      setCodes(r.cost_codes ?? []);
    } catch (e) {
      setErr(describeLoadError(e, "Maliyet kodları"));
    }
  }, [pid]);

  useEffect(() => { load(); }, [load]);

  async function create() {
    if (!pid || !code.trim() || !name.trim()) return;
    setSaving(true);
    try {
      await api(`/projects/${pid}/cost-codes`, {
        method: "POST", projectId: pid,
        body: { code: code.trim(), name: name.trim(), parent_id: parent || undefined },
      });
      setCode(""); setName(""); setParent(""); setShowForm(false);
      await load();
    } catch (e) {
      setErr(describeLoadError(e, "Maliyet kodu"));
    } finally {
      setSaving(false);
    }
  }

  if (!current) return <p className="text-beton-400">Önce üst bardan bir proje seçin.</p>;

  const nameOf = Object.fromEntries(codes.map((c) => [c.id, `${c.code} ${c.name}`]));

  return (
    <div className="space-y-4">
      <div className="flex items-center gap-3 flex-wrap">
        <div>
          <h1 className="text-lg font-display font-bold text-white">Maliyet Kodları</h1>
          <p className="text-xs text-beton-500 mt-0.5">
            Yalnız tanım listesi. Bütçe, sipariş ve hakediş bağlantısı sonraki aşamada eklenecek.
          </p>
        </div>
        {canEdit && (
          <button onClick={() => setShowForm((v) => !v)}
            className="ml-auto rounded-md bg-emniyet-500 hover:bg-emniyet-600 text-beton-950 text-xs font-semibold px-3 py-1.5">
            {showForm ? "Vazgeç" : "+ Kod ekle"}
          </button>
        )}
      </div>

      {err && <p className="text-sm text-red-400">{err}</p>}

      {showForm && (
        <div className="rounded-lg border border-beton-800 bg-beton-900 p-4 grid gap-3 md:grid-cols-4">
          <input value={code} onChange={(e) => setCode(e.target.value)} placeholder="Kod *" className={inp} />
          <input value={name} onChange={(e) => setName(e.target.value)} placeholder="Ad *" className={`${inp} md:col-span-2`} />
          <select value={parent} onChange={(e) => setParent(e.target.value)} className={inp}>
            <option value="">Üst kod yok</option>
            {codes.map((c) => <option key={c.id} value={c.id}>{c.code} {c.name}</option>)}
          </select>
          <div className="md:col-span-4 flex justify-end">
            <button onClick={create} disabled={saving || !code.trim() || !name.trim()}
              className="rounded-md bg-emniyet-500 px-4 py-1.5 text-sm font-semibold text-beton-950 disabled:opacity-50">
              Kaydet
            </button>
          </div>
        </div>
      )}

      <div className="overflow-x-auto rounded-lg border border-beton-800">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-beton-800 bg-beton-900/60 text-left text-[10px] font-bold uppercase tracking-wider text-beton-400">
              <th className="py-2 px-3">Kod</th>
              <th className="py-2 px-3">Ad</th>
              <th className="py-2 px-3">Üst kod</th>
              <th className="py-2 px-3 text-right">Bütçe</th>
              <th className="py-2 px-3">Ekleyen</th>
            </tr>
          </thead>
          <tbody>
            {codes.length === 0 && (
              <tr><td colSpan={5} className="py-6 text-center text-beton-500">Maliyet kodu tanımlı değil.</td></tr>
            )}
            {codes.map((c) => (
              <tr key={c.id} className="border-b border-beton-800/60">
                <td className="py-2 px-3 font-mono text-beton-200">{c.code}</td>
                <td className="py-2 px-3 text-beton-100">{c.name}</td>
                <td className="py-2 px-3 text-beton-400 text-xs">{c.parent_id ? nameOf[c.parent_id] ?? "—" : "—"}</td>
                <td className="py-2 px-3 text-right">
                  {canEdit ? (
                    <div className="flex items-center justify-end gap-2">
                      <input type="number" min={0} step="0.01"
                        value={budgetDraft[c.id] ?? (c.budget != null ? String(c.budget) : "")}
                        onChange={(e) => setBudgetDraft((d) => ({ ...d, [c.id]: e.target.value }))}
                        className={`${inp} w-32 text-right font-mono`} placeholder="0,00" />
                      {budgetDraft[c.id] !== undefined && (
                        <button onClick={() => saveBudget(c)} className="text-xs text-emniyet-500 hover:underline">Kaydet</button>
                      )}
                    </div>
                  ) : (
                    <span className="font-mono text-beton-300">{c.budget != null ? c.budget.toLocaleString("tr-TR", { minimumFractionDigits: 2 }) : "—"}</span>
                  )}
                </td>
                <td className="py-2 px-3 text-beton-500 text-xs">{c.created_by_name} · {c.created_at}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}
