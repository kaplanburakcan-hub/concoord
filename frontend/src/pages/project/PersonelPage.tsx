import { useEffect, useState } from "react";
import { api } from "../../api/client";
import { useProjects } from "../ProjectContext";

// ── Tipler ────────────────────────────────────────────────────────────────────
interface Personel {
  id: string;
  ad_soyad: string;
  gorev: string;
  firma: string;
  is_aktif: boolean;
  sira: number;
}

const GOREV_META: Record<string, { label: string; cls: string }> = {
  "İşçi":         { label: "İşçi",         cls: "bg-gray-100 text-gray-600 dark:bg-gray-700 dark:text-gray-300" },
  "Formen":       { label: "Formen",       cls: "bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-300" },
  "Teknisyen":    { label: "Teknisyen",    cls: "bg-indigo-100 text-indigo-700 dark:bg-indigo-900/40 dark:text-indigo-300" },
  "Mühendis":     { label: "Mühendis",     cls: "bg-purple-100 text-purple-700 dark:bg-purple-900/40 dark:text-purple-300" },
  "Şef":          { label: "Şef",          cls: "bg-yellow-100 text-yellow-700 dark:bg-yellow-900/40 dark:text-yellow-300" },
  "Yönetici":     { label: "Yönetici",     cls: "bg-orange-100 text-orange-700 dark:bg-orange-900/40 dark:text-orange-300" },
  "Alt Yüklenici":{ label: "Alt Yüklenici",cls: "bg-green-100 text-green-700 dark:bg-green-900/40 dark:text-green-300" },
};

const DEFAULT_CLS = "bg-gray-100 text-gray-600 dark:bg-gray-700 dark:text-gray-300";

function initials(name: string) {
  return name.split(" ").map(w => w[0]).filter(Boolean).slice(0, 2).join("").toUpperCase();
}

function avatarColor(name: string) {
  const colors = [
    "bg-blue-500", "bg-purple-500", "bg-green-500", "bg-yellow-500",
    "bg-orange-500", "bg-pink-500", "bg-teal-500", "bg-indigo-500",
  ];
  let hash = 0;
  for (let i = 0; i < name.length; i++) hash = (hash * 31 + name.charCodeAt(i)) | 0;
  return colors[Math.abs(hash) % colors.length];
}

export default function PersonelPage() {
  const { current: proj } = useProjects();
  const [personeller, setPersoneller] = useState<Personel[]>([]);
  const [loading, setLoading]         = useState(true);
  const [error, setError]             = useState(false);
  const [search, setSearch]           = useState("");
  const [filterGorev, setFilterGorev] = useState("");
  const [filterFirma, setFilterFirma] = useState("");
  const [filterAktif, setFilterAktif] = useState<"all" | "aktif" | "pasif">("aktif");

  useEffect(() => {
    if (!proj?.id) return;
    setLoading(true); setError(false);
    api<{ personnel: Personel[] }>(`/projects/${proj.id}/personnel`, { projectId: proj.id })
      .then(r => setPersoneller(r.personnel ?? []))
      .catch(() => setError(true))
      .finally(() => setLoading(false));
  }, [proj?.id]);

  if (!proj) return <div className="p-8 text-beton-400">Proje seçilmedi.</div>;
  if (loading) return <div className="p-8 text-beton-400">Yükleniyor…</div>;
  if (error) return <div className="p-8 text-red-500">Veriler yüklenemedi.</div>;

  const firmalar = [...new Set(personeller.map(p => p.firma).filter(Boolean))].sort();
  const gorevler = [...new Set(personeller.map(p => p.gorev).filter(Boolean))].sort();

  const filtered = personeller.filter(p => {
    if (filterAktif === "aktif"  && !p.is_aktif) return false;
    if (filterAktif === "pasif"  &&  p.is_aktif) return false;
    if (filterGorev && p.gorev !== filterGorev)  return false;
    if (filterFirma && p.firma !== filterFirma)  return false;
    if (search) {
      const q = search.toLowerCase();
      return p.ad_soyad.toLowerCase().includes(q) ||
             p.gorev.toLowerCase().includes(q) ||
             p.firma.toLowerCase().includes(q);
    }
    return true;
  });

  const aktifSayisi = personeller.filter(p => p.is_aktif).length;
  const pasifSayisi = personeller.length - aktifSayisi;

  const gorevDagilim: Record<string, number> = {};
  personeller.filter(p => p.is_aktif).forEach(p => {
    gorevDagilim[p.gorev] = (gorevDagilim[p.gorev] ?? 0) + 1;
  });

  const inpCls =
    "rounded-lg border border-beton-800 bg-beton-900 px-3 py-1.5 text-sm text-beton-100 " +
    "outline-none focus:border-blue-500 transition-colors";

  return (
    <div className="p-6 space-y-5 max-w-5xl mx-auto">
      {/* Başlık */}
      <div>
        <h1 className="text-xl font-bold text-beton-100">Personel Yönetimi</h1>
        <p className="text-sm text-beton-400 mt-0.5">
          {proj.name} — Kayıtlı personel: {personeller.length} kişi
        </p>
      </div>

      {/* Özet kartlar */}
      <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
        <div className="rounded-xl border border-beton-800 bg-beton-900 p-4 text-center">
          <div className="text-2xl font-bold text-green-600">{aktifSayisi}</div>
          <div className="text-xs text-beton-400 mt-0.5">Aktif Personel</div>
        </div>
        <div className="rounded-xl border border-beton-800 bg-beton-900 p-4 text-center">
          <div className="text-2xl font-bold text-gray-500">{pasifSayisi}</div>
          <div className="text-xs text-beton-400 mt-0.5">Pasif / Ayrılmış</div>
        </div>
        <div className="rounded-xl border border-beton-800 bg-beton-900 p-4 text-center">
          <div className="text-2xl font-bold text-beton-100">{firmalar.length}</div>
          <div className="text-xs text-beton-400 mt-0.5">Firma</div>
        </div>
        <div className="rounded-xl border border-beton-800 bg-beton-900 p-4 text-center">
          <div className="text-2xl font-bold text-beton-100">{gorevler.length}</div>
          <div className="text-xs text-beton-400 mt-0.5">Farklı Görev</div>
        </div>
      </div>

      {/* Görev dağılımı */}
      {Object.keys(gorevDagilim).length > 0 && (
        <div className="rounded-xl border border-beton-800 bg-beton-900 p-4">
          <h2 className="text-sm font-semibold text-beton-100 mb-3">Aktif Personel — Görev Dağılımı</h2>
          <div className="flex flex-wrap gap-2">
            {Object.entries(gorevDagilim)
              .sort((a, b) => b[1] - a[1])
              .map(([gorev, count]) => (
                <button
                  key={gorev}
                  onClick={() => setFilterGorev(filterGorev === gorev ? "" : gorev)}
                  className={`flex items-center gap-1.5 px-3 py-1 rounded-full text-xs font-medium border transition-all ${
                    filterGorev === gorev
                      ? "bg-blue-500 text-white-solid border-blue-500"
                      : "border-beton-800 text-beton-400 hover:border-blue-400"
                  }`}
                >
                  {gorev}
                  <span className="font-bold">{count}</span>
                </button>
              ))
            }
          </div>
        </div>
      )}

      {/* Filtreler */}
      <div className="flex flex-wrap gap-2">
        <input
          className={inpCls + " w-48"}
          placeholder="Ad, görev, firma ara…"
          value={search}
          onChange={e => setSearch(e.target.value)}
        />
        <select className={inpCls} value={filterAktif} onChange={e => setFilterAktif(e.target.value as "all" | "aktif" | "pasif")}>
          <option value="aktif">Aktif</option>
          <option value="pasif">Pasif</option>
          <option value="all">Tümü</option>
        </select>
        <select className={inpCls} value={filterGorev} onChange={e => setFilterGorev(e.target.value)}>
          <option value="">Tüm Görevler</option>
          {gorevler.map(g => <option key={g} value={g}>{g}</option>)}
        </select>
        <select className={inpCls} value={filterFirma} onChange={e => setFilterFirma(e.target.value)}>
          <option value="">Tüm Firmalar</option>
          {firmalar.map(f => <option key={f} value={f}>{f}</option>)}
        </select>
        {(search || filterGorev || filterFirma || filterAktif !== "aktif") && (
          <button
            onClick={() => { setSearch(""); setFilterGorev(""); setFilterFirma(""); setFilterAktif("aktif"); }}
            className="px-3 py-1.5 text-xs rounded-lg border border-beton-800 text-beton-400 hover:text-beton-100 transition-colors"
          >
            Temizle
          </button>
        )}
        <span className="ml-auto text-xs text-beton-400 self-center">
          {filtered.length} kayıt
        </span>
      </div>

      {/* Liste */}
      {filtered.length === 0 ? (
        <div className="rounded-xl border border-beton-800 bg-beton-900 p-12 text-center text-beton-400">
          {personeller.length === 0 ? "Personel kaydı bulunamadı." : "Arama kriterine uyan personel yok."}
        </div>
      ) : (
        <div className="rounded-xl border border-beton-800 bg-beton-900 overflow-hidden">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-beton-800 bg-[var(--bg-hover)]">
                <th className="px-4 py-2.5 text-left text-xs font-medium text-beton-400 uppercase tracking-wide">Personel</th>
                <th className="px-4 py-2.5 text-left text-xs font-medium text-beton-400 uppercase tracking-wide hidden sm:table-cell">Görev</th>
                <th className="px-4 py-2.5 text-left text-xs font-medium text-beton-400 uppercase tracking-wide hidden md:table-cell">Firma</th>
                <th className="px-4 py-2.5 text-center text-xs font-medium text-beton-400 uppercase tracking-wide">Durum</th>
              </tr>
            </thead>
            <tbody>
              {filtered.map(p => {
                const gorevMeta = GOREV_META[p.gorev] ?? { label: p.gorev, cls: DEFAULT_CLS };
                return (
                  <tr key={p.id} className="border-b border-beton-800 last:border-0 hover:bg-[var(--bg-hover)] transition-colors">
                    <td className="px-4 py-3">
                      <div className="flex items-center gap-3">
                        <div className={`w-8 h-8 rounded-full flex items-center justify-center text-xs font-bold text-white-solid shrink-0 ${avatarColor(p.ad_soyad)}`}>
                          {initials(p.ad_soyad)}
                        </div>
                        <div>
                          <div className="font-medium text-beton-100">{p.ad_soyad}</div>
                          <div className="text-xs text-beton-400 sm:hidden">{p.gorev} · {p.firma}</div>
                        </div>
                      </div>
                    </td>
                    <td className="px-4 py-3 hidden sm:table-cell">
                      <span className={`text-xs px-2 py-0.5 rounded-full font-medium ${gorevMeta.cls}`}>
                        {gorevMeta.label}
                      </span>
                    </td>
                    <td className="px-4 py-3 text-beton-400 hidden md:table-cell">{p.firma || "—"}</td>
                    <td className="px-4 py-3 text-center">
                      <span className={`text-xs px-2 py-0.5 rounded-full font-medium ${
                        p.is_aktif
                          ? "bg-green-100 text-green-700 dark:bg-green-900/40 dark:text-green-300"
                          : "bg-gray-100 text-gray-500 dark:bg-gray-700 dark:text-gray-400"
                      }`}>
                        {p.is_aktif ? "Aktif" : "Pasif"}
                      </span>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
