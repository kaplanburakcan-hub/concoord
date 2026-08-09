import { useCallback, useEffect, useMemo, useState } from "react";
import { api } from "../../api/client";
import { useProjects } from "../ProjectContext";

// ── Tipler ────────────────────────────────────────────────────────────────────
type Statement = {
  id?: string;
  tedarikci_adi: string;
  ekstre_no: string;
  ekstre_tarihi: string;
  vade_tarihi: string;
  toplam_tutar: number;
  odenen_tutar: number;
  para_birimi: string;
  odeme_durumu: string;
  aciklama: string;
};

// ── Sabitler ─────────────────────────────────────────────────────────────────
const DURUM_META: Record<string, { label: string; cls: string }> = {
  bekliyor:     { label: "Bekliyor",      cls: "bg-yellow-900/60 text-yellow-300" },
  kismi_odendi: { label: "Kısmi Ödendi",  cls: "bg-blue-900/60 text-blue-300" },
  odendi:       { label: "Ödendi",        cls: "bg-green-900/60 text-green-300" },
  iptal:        { label: "İptal",         cls: "bg-beton-800 text-beton-500 line-through" },
};

const inpBase =
  "rounded bg-beton-950 border border-beton-800 px-2 py-1 text-sm text-beton-200 " +
  "outline-none focus:border-emniyet-500 disabled:opacity-50";

function fmt(n: number): string {
  return n.toLocaleString("tr-TR", { minimumFractionDigits: 2, maximumFractionDigits: 2 });
}

function parseFmt(s: string): number {
  return parseFloat(s.replace(/\./g, "").replace(",", ".")) || 0;
}

function isoToDisplay(iso: string): string {
  if (!iso) return "";
  const [y, m, d] = iso.split("-");
  return `${d}/${m}/${y}`;
}

function displayToISO(s: string): string {
  const p = s.split("/");
  if (p.length !== 3 || p[2].length !== 4) return "";
  return `${p[2]}-${p[1].padStart(2,"0")}-${p[0].padStart(2,"0")}`;
}

function maskDate(raw: string): string {
  const d = raw.replace(/\D/g, "").slice(0, 8);
  let o = d;
  if (d.length > 2) o = d.slice(0, 2) + "/" + d.slice(2);
  if (d.length > 4) o = o.slice(0, 5) + "/" + o.slice(5);
  return o;
}

function emptyStatement(): Statement {
  return {
    tedarikci_adi: "", ekstre_no: "", ekstre_tarihi: "", vade_tarihi: "",
    toplam_tutar: 0, odenen_tutar: 0, para_birimi: "TRY",
    odeme_durumu: "bekliyor", aciklama: "",
  };
}

// ── Form ─────────────────────────────────────────────────────────────────────
function StatementForm({
  initial, onSave, onCancel,
}: {
  initial: Statement;
  onSave: (s: Statement) => void;
  onCancel: () => void;
}) {
  const [s, setS] = useState<Statement>(initial);

  function set<K extends keyof Statement>(k: K, v: Statement[K]) {
    setS((prev) => ({ ...prev, [k]: v }));
  }

  const bakiye = s.toplam_tutar - s.odenen_tutar;

  return (
    <div className="p-4 border border-beton-700 rounded-lg bg-beton-900/60 mb-4">
      <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-4">
        <div className="col-span-2">
          <label className="text-xs text-beton-500 block mb-1">Tedarikçi Adı *</label>
          <input className={`${inpBase} w-full`} value={s.tedarikci_adi}
            onChange={(e) => set("tedarikci_adi", e.target.value)} />
        </div>
        <div>
          <label className="text-xs text-beton-500 block mb-1">Ekstre No *</label>
          <input className={`${inpBase} w-full`} placeholder="EKS-2024-001" value={s.ekstre_no}
            onChange={(e) => set("ekstre_no", e.target.value)} />
        </div>
        <div>
          <label className="text-xs text-beton-500 block mb-1">Ekstre Tarihi *</label>
          <input className={`${inpBase} w-full`} placeholder="gg/aa/yyyy" maxLength={10}
            value={isoToDisplay(s.ekstre_tarihi)}
            onChange={(e) => set("ekstre_tarihi", displayToISO(maskDate(e.target.value)))} />
        </div>
        <div>
          <label className="text-xs text-beton-500 block mb-1">Vade Tarihi</label>
          <input className={`${inpBase} w-full`} placeholder="gg/aa/yyyy" maxLength={10}
            value={isoToDisplay(s.vade_tarihi)}
            onChange={(e) => set("vade_tarihi", displayToISO(maskDate(e.target.value)))} />
        </div>
        <div>
          <label className="text-xs text-beton-500 block mb-1">Toplam Tutar</label>
          <input className={`${inpBase} w-full text-right`}
            value={s.toplam_tutar === 0 ? "" : fmt(s.toplam_tutar)}
            onChange={(e) => set("toplam_tutar", parseFmt(e.target.value))} />
        </div>
        <div>
          <label className="text-xs text-beton-500 block mb-1">Ödenen Tutar</label>
          <input className={`${inpBase} w-full text-right`}
            value={s.odenen_tutar === 0 ? "" : fmt(s.odenen_tutar)}
            onChange={(e) => set("odenen_tutar", parseFmt(e.target.value))} />
        </div>
        <div>
          <label className="text-xs text-beton-500 block mb-1">Bakiye</label>
          <div className={`${inpBase} w-full text-right bg-beton-900/50 pointer-events-none ${bakiye > 0 ? "text-yellow-400" : "text-green-400"}`}>
            {fmt(bakiye)}
          </div>
        </div>
        <div>
          <label className="text-xs text-beton-500 block mb-1">Para Birimi</label>
          <select className={`${inpBase} w-full`} value={s.para_birimi}
            onChange={(e) => set("para_birimi", e.target.value)}>
            {["TRY", "USD", "EUR"].map((c) => <option key={c}>{c}</option>)}
          </select>
        </div>
        <div>
          <label className="text-xs text-beton-500 block mb-1">Ödeme Durumu</label>
          <select className={`${inpBase} w-full`} value={s.odeme_durumu}
            onChange={(e) => set("odeme_durumu", e.target.value)}>
            {Object.entries(DURUM_META).map(([k, v]) => (
              <option key={k} value={k}>{v.label}</option>
            ))}
          </select>
        </div>
        <div className="col-span-2 lg:col-span-2">
          <label className="text-xs text-beton-500 block mb-1">Açıklama</label>
          <input className={`${inpBase} w-full`} value={s.aciklama}
            onChange={(e) => set("aciklama", e.target.value)} />
        </div>
      </div>
      <div className="flex gap-2 mt-3">
        <button
          onClick={() => onSave(s)}
          disabled={!s.tedarikci_adi || !s.ekstre_no || !s.ekstre_tarihi}
          className="px-3 py-1.5 bg-emniyet-600 hover:bg-emniyet-500 text-white text-sm rounded-md disabled:opacity-40"
        >
          Kaydet
        </button>
        <button onClick={onCancel} className="px-3 py-1.5 text-beton-400 hover:text-beton-200 text-sm">
          İptal
        </button>
      </div>
    </div>
  );
}

// ── Ana Bileşen ───────────────────────────────────────────────────────────────
export default function TedarikciEkstrerlerPage() {
  const { current: currentProject } = useProjects();
  const pid = currentProject?.id;

  const [statements, setStatements] = useState<Statement[]>([]);
  const [loading, setLoading] = useState(true);
  const [adding, setAdding] = useState(false);
  const [editing, setEditing] = useState<Statement | null>(null);
  const [filterTedarikci, setFilterTedarikci] = useState("");
  const [filterDurum, setFilterDurum] = useState("");

  const load = useCallback(async () => {
    if (!pid) return;
    setLoading(true);
    try {
      const data = await api<{ statements: Statement[] }>(`/projects/${pid}/supplier-statements`, { projectId: pid });
      setStatements(data.statements ?? []);
    } finally {
      setLoading(false);
    }
  }, [pid]);

  useEffect(() => { load(); }, [load]);

  async function handleSave(s: Statement) {
    if (!pid) return;
    if (s.id) {
      await api(`/projects/${pid}/supplier-statements/${s.id}`, { method: "PATCH", body: s, projectId: pid });
    } else {
      await api(`/projects/${pid}/supplier-statements`, { method: "POST", body: s, projectId: pid });
    }
    setAdding(false);
    setEditing(null);
    await load();
  }

  async function handleDelete(s: Statement) {
    if (!pid || !s.id) return;
    if (!confirm(`"${s.ekstre_no}" silinecek. Onaylıyor musunuz?`)) return;
    await api(`/projects/${pid}/supplier-statements/${s.id}`, { method: "DELETE", projectId: pid });
    await load();
  }

  // Filtrele + grupla tedarikçiye göre
  const filtered = useMemo(() => {
    return statements.filter((s) => {
      if (filterTedarikci && !s.tedarikci_adi.toLowerCase().includes(filterTedarikci.toLowerCase())) return false;
      if (filterDurum && s.odeme_durumu !== filterDurum) return false;
      return true;
    });
  }, [statements, filterTedarikci, filterDurum]);

  const tedarikciListesi = useMemo(() => {
    return [...new Set(statements.map((s) => s.tedarikci_adi))].sort();
  }, [statements]);

  // Özet istatistikleri
  const stats = useMemo(() => {
    const toplam = statements.reduce((s, r) => s + r.toplam_tutar, 0);
    const odenen = statements.reduce((s, r) => s + r.odenen_tutar, 0);
    const bekleyen = toplam - odenen;
    const byDurum = Object.fromEntries(Object.keys(DURUM_META).map((k) => [k, 0]));
    for (const r of statements) byDurum[r.odeme_durumu] = (byDurum[r.odeme_durumu] ?? 0) + 1;
    return { toplam, odenen, bekleyen, byDurum };
  }, [statements]);

  return (
    <div className="p-6 max-w-6xl mx-auto">
      {/* Başlık */}
      <div className="flex items-start justify-between mb-5">
        <div>
          <h1 className="text-xl font-semibold text-beton-100">Tedarikçi Ekstreler</h1>
          {currentProject && <p className="text-beton-400 text-sm mt-0.5">{currentProject.name}</p>}
        </div>
        <button
          onClick={() => { setAdding(true); setEditing(null); }}
          className="px-3 py-1.5 bg-emniyet-600 hover:bg-emniyet-500 text-white text-sm rounded-md"
        >
          + Ekstre Ekle
        </button>
      </div>

      {/* Özet kartları — ekstre olsun olmasın her zaman gösterilir */}
      <div className="grid grid-cols-3 gap-3 mb-5">
        <div className="bg-beton-900 rounded-lg p-3 border border-beton-800">
          <p className="text-xs text-beton-500 mb-1">Toplam Ekstre Tutarı</p>
          <p className="text-lg font-bold text-beton-100">{fmt(stats.toplam)} TRY</p>
        </div>
        <div className="bg-beton-900 rounded-lg p-3 border border-beton-800">
          <p className="text-xs text-beton-500 mb-1">Ödenen</p>
          <p className="text-lg font-bold text-green-400">{fmt(stats.odenen)} TRY</p>
        </div>
        <div className="bg-beton-900 rounded-lg p-3 border border-beton-800">
          <p className="text-xs text-beton-500 mb-1">Bekleyen Bakiye</p>
          <p className={`text-lg font-bold ${stats.bekleyen > 0 ? "text-yellow-400" : "text-beton-400"}`}>
            {fmt(stats.bekleyen)} TRY
          </p>
        </div>
      </div>

      {/* Form */}
      {adding && (
        <StatementForm
          initial={emptyStatement()}
          onSave={handleSave}
          onCancel={() => setAdding(false)}
        />
      )}
      {editing && (
        <StatementForm
          initial={editing}
          onSave={handleSave}
          onCancel={() => setEditing(null)}
        />
      )}

      {/* Filtreler */}
      {statements.length > 0 && (
        <div className="rounded-lg border border-beton-800 bg-beton-900 p-3 mb-3 flex gap-2 flex-wrap">
          <input
            className={`${inpBase} w-48`}
            placeholder="Tedarikçi ara…"
            value={filterTedarikci}
            onChange={(e) => setFilterTedarikci(e.target.value)}
          />
          <select
            className={`${inpBase}`}
            value={filterDurum}
            onChange={(e) => setFilterDurum(e.target.value)}
          >
            <option value="">Tüm Durumlar</option>
            {Object.entries(DURUM_META).map(([k, v]) => (
              <option key={k} value={k}>{v.label}</option>
            ))}
          </select>
          {(filterTedarikci || filterDurum) && (
            <button
              onClick={() => { setFilterTedarikci(""); setFilterDurum(""); }}
              className="text-xs text-beton-500 hover:text-beton-300 px-2"
            >
              Temizle
            </button>
          )}
        </div>
      )}

      {/* Liste */}
      {loading ? (
        <p className="text-beton-500 text-sm">Yükleniyor…</p>
      ) : filtered.length === 0 ? (
        <p className="text-beton-500 text-sm text-center py-10">
          {statements.length === 0 ? "Henüz ekstre eklenmemiş." : "Filtreyle eşleşen kayıt yok."}
        </p>
      ) : (
        <div className="border border-beton-800 rounded-lg overflow-hidden">
          <table className="w-full text-sm">
            <thead>
              <tr className="bg-beton-900 border-b border-beton-800">
                <th className="py-2 px-3 text-left text-xs text-beton-500 font-medium">Tedarikçi</th>
                <th className="py-2 px-3 text-left text-xs text-beton-500 font-medium w-32">Ekstre No</th>
                <th className="py-2 px-3 text-left text-xs text-beton-500 font-medium w-24">Tarih</th>
                <th className="py-2 px-3 text-left text-xs text-beton-500 font-medium w-24">Vade</th>
                <th className="py-2 px-3 text-right text-xs text-beton-500 font-medium w-36">Toplam</th>
                <th className="py-2 px-3 text-right text-xs text-beton-500 font-medium w-36">Ödenen</th>
                <th className="py-2 px-3 text-right text-xs text-beton-500 font-medium w-36">Bakiye</th>
                <th className="py-2 px-3 text-left text-xs text-beton-500 font-medium w-32">Durum</th>
                <th className="w-24" />
              </tr>
            </thead>
            <tbody>
              {filtered.map((s) => {
                const dm = DURUM_META[s.odeme_durumu] ?? DURUM_META.bekliyor;
                const bakiye = s.toplam_tutar - s.odenen_tutar;
                return (
                  <tr key={s.id} className="border-b border-beton-800/50 hover:bg-beton-900/30 group">
                    <td className="py-2 px-3 text-beton-200 text-sm font-medium">{s.tedarikci_adi}</td>
                    <td className="py-2 px-3 text-beton-400 text-xs font-mono">{s.ekstre_no}</td>
                    <td className="py-2 px-3 text-beton-400 text-xs">{isoToDisplay(s.ekstre_tarihi)}</td>
                    <td className="py-2 px-3 text-beton-500 text-xs">{isoToDisplay(s.vade_tarihi) || "—"}</td>
                    <td className="py-2 px-3 text-right text-beton-200 text-xs font-mono">{fmt(s.toplam_tutar)}</td>
                    <td className="py-2 px-3 text-right text-green-400 text-xs font-mono">{fmt(s.odenen_tutar)}</td>
                    <td className={`py-2 px-3 text-right text-xs font-mono font-bold ${bakiye > 0 ? "text-yellow-400" : "text-beton-500"}`}>
                      {fmt(bakiye)}
                    </td>
                    <td className="py-2 px-3">
                      <span className={`text-xs px-2 py-0.5 rounded-full font-medium ${dm.cls}`}>
                        {dm.label}
                      </span>
                    </td>
                    <td className="py-2 px-3">
                      <div className="flex gap-2 opacity-0 group-hover:opacity-100 transition-opacity">
                        <button
                          onClick={() => { setEditing(s); setAdding(false); }}
                          className="text-xs text-emniyet-400 hover:text-emniyet-300"
                        >
                          Düzenle
                        </button>
                        <button
                          onClick={() => handleDelete(s)}
                          className="text-xs text-red-500 hover:text-red-400"
                        >
                          Sil
                        </button>
                      </div>
                    </td>
                  </tr>
                );
              })}
            </tbody>
            <tfoot>
              <tr className="border-t border-beton-700 bg-beton-900/60">
                <td colSpan={4} className="py-2 px-3 text-xs text-beton-500 font-medium">
                  {filtered.length} kayıt
                </td>
                <td className="py-2 px-3 text-right text-xs font-bold text-beton-200 font-mono">
                  {fmt(filtered.reduce((s, r) => s + r.toplam_tutar, 0))}
                </td>
                <td className="py-2 px-3 text-right text-xs font-bold text-green-400 font-mono">
                  {fmt(filtered.reduce((s, r) => s + r.odenen_tutar, 0))}
                </td>
                <td className="py-2 px-3 text-right text-xs font-bold text-yellow-400 font-mono">
                  {fmt(filtered.reduce((s, r) => s + (r.toplam_tutar - r.odenen_tutar), 0))}
                </td>
                <td colSpan={2} />
              </tr>
            </tfoot>
          </table>
        </div>
      )}
    </div>
  );
}
