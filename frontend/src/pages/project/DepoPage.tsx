import { useEffect, useState } from "react";
import { api } from "../../api/client";
import { useProjects } from "../ProjectContext";
import { useKesinKabulTarihi } from "../../hooks/useKesinKabulTarihi";

// ── Tipler ────────────────────────────────────────────────────────────────────
interface Item {
  id: string;
  project_id: string;
  malzeme_adi: string;
  kategori: string;
  birim: string;
  mevcut_miktar: number;
  min_stok: number;
  aciklama?: string;
  sira: number;
}

interface Movement {
  id: string;
  project_id: string;
  item_id: string;
  malzeme_adi: string;
  hareket_turu: "giris" | "cikis" | "sayim" | "iade";
  miktar: number;
  tarih: string;
  belge_no?: string;
  aciklama?: string;
}

const HAREKET_META: Record<string, { label: string; cls: string; sign: string }> = {
  giris:  { label: "Giriş",  cls: "bg-green-100 text-green-700 dark:bg-green-900/40 dark:text-green-300",   sign: "+" },
  cikis:  { label: "Çıkış",  cls: "bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-300",           sign: "−" },
  sayim:  { label: "Sayım",  cls: "bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-300",       sign: "=" },
  iade:   { label: "İade",   cls: "bg-yellow-100 text-yellow-700 dark:bg-yellow-900/40 dark:text-yellow-300", sign: "↩" },
};

function fmtDate(s?: string) {
  if (!s) return "—";
  return new Date(s).toLocaleDateString("tr-TR", { day: "2-digit", month: "short", year: "numeric" });
}

function fmtNum(n: number, unit: string) {
  return `${n.toLocaleString("tr-TR", { maximumFractionDigits: 2 })} ${unit}`;
}

const inpCls =
  "rounded-lg border border-beton-800 bg-beton-900 px-3 py-1.5 text-sm text-beton-100 " +
  "outline-none focus:border-blue-500 transition-colors";

// ── Modal: Hareket Ekle ────────────────────────────────────────────────────────
function MovementModal({
  items, projectId,
  onSave, onClose,
}: {
  items: Item[];
  projectId: string;
  onSave: () => void;
  onClose: () => void;
}) {
  const kesinKabul = useKesinKabulTarihi(projectId);
  const [itemId, setItemId]   = useState(items[0]?.id ?? "");
  const [turu, setTuru]       = useState<"giris" | "cikis" | "sayim" | "iade">("giris");
  const [miktar, setMiktar]   = useState("");
  const [tarih, setTarih]     = useState(new Date().toISOString().slice(0, 10));
  const [belgeNo, setBelgeNo] = useState("");
  const [aciklama, setAciklama] = useState("");
  const [saving, setSaving]   = useState(false);
  const [err, setErr]         = useState("");

  const selectedItem = items.find(i => i.id === itemId);

  async function submit() {
    const m = parseFloat(miktar);
    if (!itemId || isNaN(m) || m <= 0) { setErr("Malzeme ve geçerli miktar zorunludur."); return; }
    setSaving(true); setErr("");
    try {
      await api(`/projects/${projectId}/warehouse-movements`, {
        projectId,
        method: "POST",
        body: { item_id: itemId, hareket_turu: turu, miktar: m, tarih, belge_no: belgeNo || undefined, aciklama: aciklama || undefined },
      });
      onSave();
    } catch {
      setErr("Kaydedilemedi.");
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4">
      <div className="bg-beton-900 border border-beton-800 rounded-2xl p-6 w-full max-w-md space-y-4 shadow-xl">
        <h3 className="font-semibold text-beton-100 text-lg">Stok Hareketi Ekle</h3>
        {err && <div className="text-sm text-red-500">{err}</div>}
        <div className="space-y-3">
          <div>
            <label className="text-xs text-beton-400 mb-1 block">Malzeme</label>
            <select className={inpCls + " w-full"} value={itemId} onChange={e => setItemId(e.target.value)}>
              {items.map(i => <option key={i.id} value={i.id}>{i.malzeme_adi} ({i.birim})</option>)}
            </select>
          </div>
          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className="text-xs text-beton-400 mb-1 block">Hareket Türü</label>
              <select className={inpCls + " w-full"} value={turu} onChange={e => setTuru(e.target.value as typeof turu)}>
                <option value="giris">Giriş</option>
                <option value="cikis">Çıkış</option>
                <option value="sayim">Sayım (Fiili)</option>
                <option value="iade">İade</option>
              </select>
            </div>
            <div>
              <label className="text-xs text-beton-400 mb-1 block">
                Miktar{selectedItem ? ` (${selectedItem.birim})` : ""}
              </label>
              <input
                className={inpCls + " w-full"}
                type="number" min="0.01" step="0.01"
                value={miktar} onChange={e => setMiktar(e.target.value)}
                placeholder="0"
              />
            </div>
          </div>
          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className="text-xs text-beton-400 mb-1 block">Tarih</label>
              <input className={inpCls + " w-full"} type="date" value={tarih} max={kesinKabul} onChange={e => setTarih(e.target.value)} />
            </div>
            <div>
              <label className="text-xs text-beton-400 mb-1 block">Belge No</label>
              <input className={inpCls + " w-full"} type="text" value={belgeNo} onChange={e => setBelgeNo(e.target.value)} placeholder="İrsaliye, fatura…" />
            </div>
          </div>
          <div>
            <label className="text-xs text-beton-400 mb-1 block">Açıklama</label>
            <input className={inpCls + " w-full"} type="text" value={aciklama} onChange={e => setAciklama(e.target.value)} placeholder="Opsiyonel…" />
          </div>
        </div>
        <div className="flex gap-2 pt-1">
          <button onClick={onClose} className="flex-1 py-2 rounded-lg border border-beton-800 text-sm text-beton-400 hover:text-beton-100 transition-colors">
            İptal
          </button>
          <button onClick={submit} disabled={saving} className="flex-1 py-2 rounded-lg bg-emniyet-500 hover:bg-emniyet-600 text-beton-950 text-sm font-medium transition-colors disabled:opacity-50">
            {saving ? "Kaydediliyor…" : "Kaydet"}
          </button>
        </div>
      </div>
    </div>
  );
}

// ── Modal: Malzeme Ekle ───────────────────────────────────────────────────────
function ItemModal({
  projectId,
  onSave, onClose,
}: {
  projectId: string;
  onSave: () => void;
  onClose: () => void;
}) {
  const [malzemeAdi, setMalzemeAdi] = useState("");
  const [kategori, setKategori]     = useState("Genel");
  const [birim, setBirim]           = useState("adet");
  const [minStok, setMinStok]       = useState("0");
  const [saving, setSaving]         = useState(false);
  const [err, setErr]               = useState("");

  async function submit() {
    if (!malzemeAdi.trim()) { setErr("Malzeme adı zorunludur."); return; }
    setSaving(true); setErr("");
    try {
      await api(`/projects/${projectId}/warehouse-items`, {
        projectId,
        method: "POST",
        body: { malzeme_adi: malzemeAdi.trim(), kategori, birim, mevcut_miktar: 0, min_stok: parseFloat(minStok) || 0 },
      });
      onSave();
    } catch {
      setErr("Kaydedilemedi.");
    } finally {
      setSaving(false);
    }
  }

  const BIRIMLER = ["adet", "kg", "ton", "m", "m²", "m³", "lt", "paket", "koli", "takım"];
  const KATEGORILER = ["Genel", "İnşaat Malzemesi", "Demir-Çelik", "Beton", "Ahşap", "Elektrik", "Mekanik", "İSG", "Araç-Gereç"];

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4">
      <div className="bg-beton-900 border border-beton-800 rounded-2xl p-6 w-full max-w-md space-y-4 shadow-xl">
        <h3 className="font-semibold text-beton-100 text-lg">Malzeme Ekle</h3>
        {err && <div className="text-sm text-red-500">{err}</div>}
        <div className="space-y-3">
          <div>
            <label className="text-xs text-beton-400 mb-1 block">Malzeme Adı</label>
            <input className={inpCls + " w-full"} value={malzemeAdi} onChange={e => setMalzemeAdi(e.target.value)} placeholder="ör. Nervürlü Demir Ø10" />
          </div>
          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className="text-xs text-beton-400 mb-1 block">Kategori</label>
              <select className={inpCls + " w-full"} value={kategori} onChange={e => setKategori(e.target.value)}>
                {KATEGORILER.map(k => <option key={k} value={k}>{k}</option>)}
              </select>
            </div>
            <div>
              <label className="text-xs text-beton-400 mb-1 block">Birim</label>
              <select className={inpCls + " w-full"} value={birim} onChange={e => setBirim(e.target.value)}>
                {BIRIMLER.map(b => <option key={b} value={b}>{b}</option>)}
              </select>
            </div>
          </div>
          <div>
            <label className="text-xs text-beton-400 mb-1 block">Minimum Stok</label>
            <input className={inpCls + " w-full"} type="number" min="0" step="0.01" value={minStok} onChange={e => setMinStok(e.target.value)} />
          </div>
        </div>
        <div className="flex gap-2 pt-1">
          <button onClick={onClose} className="flex-1 py-2 rounded-lg border border-beton-800 text-sm text-beton-400 hover:text-beton-100 transition-colors">
            İptal
          </button>
          <button onClick={submit} disabled={saving} className="flex-1 py-2 rounded-lg bg-emniyet-500 hover:bg-emniyet-600 text-beton-950 text-sm font-medium transition-colors disabled:opacity-50">
            {saving ? "Kaydediliyor…" : "Kaydet"}
          </button>
        </div>
      </div>
    </div>
  );
}

// ── Ana Sayfa ─────────────────────────────────────────────────────────────────
export default function DepoPage() {
  const { current: proj } = useProjects();
  const [items, setItems]         = useState<Item[]>([]);
  const [movements, setMovements] = useState<Movement[]>([]);
  const [loading, setLoading]     = useState(true);
  const [error, setError]         = useState(false);
  const [tab, setTab]             = useState<"stok" | "hareketler">("stok");
  const [filterKat, setFilterKat] = useState("");
  const [search, setSearch]       = useState("");
  const [showMovModal, setShowMovModal] = useState(false);
  const [showItemModal, setShowItemModal] = useState(false);

  function reload() {
    if (!proj?.id) return;
    const pid = proj.id;
    Promise.all([
      api<{ items: Item[] }>(`/projects/${pid}/warehouse-items`, { projectId: pid }),
      api<{ movements: Movement[] }>(`/projects/${pid}/warehouse-movements`, { projectId: pid }),
    ]).then(([ir, mr]) => {
      setItems(ir.items ?? []);
      setMovements(mr.movements ?? []);
    }).catch(() => setError(true))
      .finally(() => setLoading(false));
  }

  useEffect(() => {
    if (!proj?.id) return;
    setLoading(true); setError(false);
    reload();
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [proj?.id]);

  if (!proj) return <div className="p-8 text-beton-400">Proje seçilmedi.</div>;
  if (loading) return <div className="p-8 text-beton-400">Yükleniyor…</div>;
  if (error) return <div className="p-8 text-red-500">Veriler yüklenemedi.</div>;

  const kategoriler = [...new Set(items.map(i => i.kategori).filter(Boolean))].sort();
  const kritikItems = items.filter(i => i.mevcut_miktar <= i.min_stok && i.min_stok > 0);

  const filteredItems = items.filter(i => {
    if (filterKat && i.kategori !== filterKat) return false;
    if (search) {
      const q = search.toLowerCase();
      return i.malzeme_adi.toLowerCase().includes(q) || i.kategori.toLowerCase().includes(q);
    }
    return true;
  });

  const filteredMovements = movements.filter(m => {
    if (!search) return true;
    const q = search.toLowerCase();
    return m.malzeme_adi.toLowerCase().includes(q) ||
           (m.belge_no ?? "").toLowerCase().includes(q) ||
           (m.aciklama ?? "").toLowerCase().includes(q);
  });

  return (
    <div className="p-6 space-y-5 max-w-5xl mx-auto">
      {/* Başlık */}
      <div className="flex items-start justify-between gap-4 flex-wrap">
        <div>
          <h1 className="text-xl font-bold text-beton-100">Depo Raporları</h1>
          <p className="text-sm text-beton-400 mt-0.5">{proj.name} — Stok takibi ve hareket kayıtları</p>
        </div>
        <div className="flex gap-2">
          <button
            onClick={() => setShowItemModal(true)}
            className="px-3 py-1.5 rounded-lg bg-emniyet-500 hover:bg-emniyet-600 text-beton-950 text-sm font-medium transition-colors"
          >
            + Malzeme
          </button>
          {items.length > 0 && (
            <button
              onClick={() => setShowMovModal(true)}
              className="px-3 py-1.5 rounded-lg bg-emniyet-500 hover:bg-emniyet-600 text-beton-950 text-sm font-medium transition-colors"
            >
              + Hareket Ekle
            </button>
          )}
        </div>
      </div>

      {/* KPI */}
      <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
        <div className="rounded-xl border border-beton-800 bg-beton-900 p-4 text-center">
          <div className="text-2xl font-bold text-beton-100">{items.length}</div>
          <div className="text-xs text-beton-400 mt-0.5">Malzeme Kalemi</div>
        </div>
        <div className="rounded-xl border border-beton-800 bg-beton-900 p-4 text-center">
          <div className={`text-2xl font-bold ${kritikItems.length > 0 ? "text-red-500" : "text-green-600"}`}>
            {kritikItems.length}
          </div>
          <div className="text-xs text-beton-400 mt-0.5">Kritik Stok</div>
        </div>
        <div className="rounded-xl border border-beton-800 bg-beton-900 p-4 text-center">
          <div className="text-2xl font-bold text-beton-100">{movements.length}</div>
          <div className="text-xs text-beton-400 mt-0.5">Toplam Hareket</div>
        </div>
        <div className="rounded-xl border border-beton-800 bg-beton-900 p-4 text-center">
          <div className="text-2xl font-bold text-beton-100">{kategoriler.length}</div>
          <div className="text-xs text-beton-400 mt-0.5">Kategori</div>
        </div>
      </div>

      {/* Kritik Stok Uyarısı */}
      {kritikItems.length > 0 && (
        <div className="rounded-xl border border-red-300 bg-red-50 dark:border-red-800 dark:bg-red-900/20 p-4">
          <div className="text-sm font-semibold text-red-700 dark:text-red-400 mb-2">
            ⚠ Kritik Stok Uyarısı — {kritikItems.length} kalem
          </div>
          <div className="flex flex-wrap gap-2">
            {kritikItems.map(i => (
              <span key={i.id} className="text-xs bg-red-100 dark:bg-red-900/40 text-red-700 dark:text-red-300 px-2 py-0.5 rounded-full">
                {i.malzeme_adi} ({fmtNum(i.mevcut_miktar, i.birim)} / min {fmtNum(i.min_stok, i.birim)})
              </span>
            ))}
          </div>
        </div>
      )}

      {/* Sekme */}
      <div className="flex gap-1 border-b border-beton-800">
        {(["stok", "hareketler"] as const).map(t => (
          <button
            key={t}
            onClick={() => setTab(t)}
            className={`px-4 py-2 text-sm font-medium transition-colors border-b-2 -mb-px ${
              tab === t
                ? "border-blue-500 text-blue-600 dark:text-blue-400"
                : "border-transparent text-beton-400 hover:text-beton-100"
            }`}
          >
            {t === "stok" ? `Stok Durumu (${items.length})` : `Hareketler (${movements.length})`}
          </button>
        ))}
      </div>

      {/* Filtreler */}
      <div className="flex flex-wrap gap-2">
        <input
          className={inpCls + " w-48"}
          placeholder={tab === "stok" ? "Malzeme ara…" : "Ara…"}
          value={search}
          onChange={e => setSearch(e.target.value)}
        />
        {tab === "stok" && (
          <select className={inpCls} value={filterKat} onChange={e => setFilterKat(e.target.value)}>
            <option value="">Tüm Kategoriler</option>
            {kategoriler.map(k => <option key={k} value={k}>{k}</option>)}
          </select>
        )}
        {(search || filterKat) && (
          <button onClick={() => { setSearch(""); setFilterKat(""); }} className="px-3 py-1.5 text-xs rounded-lg border border-beton-800 text-beton-400 hover:text-beton-100 transition-colors">
            Temizle
          </button>
        )}
      </div>

      {/* Stok Tablosu */}
      {tab === "stok" && (
        filteredItems.length === 0 ? (
          <div className="rounded-xl border border-beton-800 bg-beton-900 p-12 text-center text-beton-400">
            {items.length === 0 ? "Henüz malzeme eklenmedi. \"+ Malzeme\" butonuyla başlayın." : "Arama kriterine uyan malzeme yok."}
          </div>
        ) : (
          <div className="rounded-xl border border-beton-800 bg-beton-900 overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-beton-800 bg-[var(--bg-hover)]">
                  <th className="px-4 py-2.5 text-left text-xs font-medium text-beton-400 uppercase tracking-wide">Malzeme</th>
                  <th className="px-4 py-2.5 text-left text-xs font-medium text-beton-400 uppercase tracking-wide hidden sm:table-cell">Kategori</th>
                  <th className="px-4 py-2.5 text-right text-xs font-medium text-beton-400 uppercase tracking-wide">Mevcut</th>
                  <th className="px-4 py-2.5 text-right text-xs font-medium text-beton-400 uppercase tracking-wide hidden md:table-cell">Min. Stok</th>
                  <th className="px-4 py-2.5 text-center text-xs font-medium text-beton-400 uppercase tracking-wide">Durum</th>
                </tr>
              </thead>
              <tbody>
                {filteredItems.map(item => {
                  const kritik = item.min_stok > 0 && item.mevcut_miktar <= item.min_stok;
                  return (
                    <tr key={item.id} className={`border-b border-beton-800 last:border-0 hover:bg-[var(--bg-hover)] transition-colors ${kritik ? "bg-red-50/50 dark:bg-red-900/10" : ""}`}>
                      <td className="px-4 py-3">
                        <div className="font-medium text-beton-100">{item.malzeme_adi}</div>
                        {item.aciklama && <div className="text-xs text-beton-400">{item.aciklama}</div>}
                      </td>
                      <td className="px-4 py-3 text-beton-400 hidden sm:table-cell">{item.kategori}</td>
                      <td className={`px-4 py-3 text-right font-medium ${kritik ? "text-red-500" : "text-beton-100"}`}>
                        {fmtNum(item.mevcut_miktar, item.birim)}
                      </td>
                      <td className="px-4 py-3 text-right text-beton-400 hidden md:table-cell">
                        {item.min_stok > 0 ? fmtNum(item.min_stok, item.birim) : "—"}
                      </td>
                      <td className="px-4 py-3 text-center">
                        <span className={`text-xs px-2 py-0.5 rounded-full font-medium ${
                          kritik
                            ? "bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-300"
                            : "bg-green-100 text-green-700 dark:bg-green-900/40 dark:text-green-300"
                        }`}>
                          {kritik ? "Kritik" : "Normal"}
                        </span>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        )
      )}

      {/* Hareketler Tablosu */}
      {tab === "hareketler" && (
        filteredMovements.length === 0 ? (
          <div className="rounded-xl border border-beton-800 bg-beton-900 p-12 text-center text-beton-400">
            {movements.length === 0 ? "Henüz hareket kaydı yok." : "Arama kriterine uyan kayıt yok."}
          </div>
        ) : (
          <div className="rounded-xl border border-beton-800 bg-beton-900 overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-beton-800 bg-[var(--bg-hover)]">
                  <th className="px-4 py-2.5 text-left text-xs font-medium text-beton-400 uppercase tracking-wide">Tarih</th>
                  <th className="px-4 py-2.5 text-left text-xs font-medium text-beton-400 uppercase tracking-wide">Malzeme</th>
                  <th className="px-4 py-2.5 text-center text-xs font-medium text-beton-400 uppercase tracking-wide">Tür</th>
                  <th className="px-4 py-2.5 text-right text-xs font-medium text-beton-400 uppercase tracking-wide">Miktar</th>
                  <th className="px-4 py-2.5 text-left text-xs font-medium text-beton-400 uppercase tracking-wide hidden md:table-cell">Belge</th>
                </tr>
              </thead>
              <tbody>
                {filteredMovements.map(mv => {
                  const meta = HAREKET_META[mv.hareket_turu] ?? HAREKET_META.giris;
                  return (
                    <tr key={mv.id} className="border-b border-beton-800 last:border-0 hover:bg-[var(--bg-hover)] transition-colors">
                      <td className="px-4 py-3 text-beton-400 text-xs">{fmtDate(mv.tarih)}</td>
                      <td className="px-4 py-3">
                        <div className="font-medium text-beton-100">{mv.malzeme_adi}</div>
                        {mv.aciklama && <div className="text-xs text-beton-400">{mv.aciklama}</div>}
                      </td>
                      <td className="px-4 py-3 text-center">
                        <span className={`text-xs px-2 py-0.5 rounded-full font-medium ${meta.cls}`}>
                          {meta.sign} {meta.label}
                        </span>
                      </td>
                      <td className="px-4 py-3 text-right font-medium text-beton-100">
                        {mv.miktar.toLocaleString("tr-TR", { maximumFractionDigits: 2 })}
                      </td>
                      <td className="px-4 py-3 text-beton-400 text-xs hidden md:table-cell">
                        {mv.belge_no ?? "—"}
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        )
      )}

      {/* Modallar */}
      {showMovModal && (
        <MovementModal
          items={items}
          projectId={proj.id}
          onSave={() => { setShowMovModal(false); reload(); }}
          onClose={() => setShowMovModal(false)}
        />
      )}
      {showItemModal && (
        <ItemModal
          projectId={proj.id}
          onSave={() => { setShowItemModal(false); reload(); }}
          onClose={() => setShowItemModal(false)}
        />
      )}
    </div>
  );
}
