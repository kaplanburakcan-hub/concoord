import { useEffect, useMemo, useState } from "react";
import { api } from "../../api/client";
import { useProjects } from "../ProjectContext";

// Sözleşme Takip — Proje Keşfi kalemlerinin karşısında, o imalat için
// sözleşme yapılmış taşeron(lar)ı ve sözleşme bilgilerini gösterir.
// Eşleştirme poz_no üzerinden yapılır: Proje Keşfi kalemi ile taşeronun
// "İş Kalemleri" (work_items) tablosundaki poz_no birebir aynıysa
// eşleşme sayılır — hiçbir yeni veri girişi/yazma yok, salt okunur
// çapraz rapor (bkz. backend internal/payments/sozlesme_takip.go).

type Eslesme = {
  taseron_adi: string;
  sozlesme_no?: string;
  sozlesme_turu?: string;
  sozlesme_tarihi?: string;
};
type Item = {
  id: string;
  kategori: string;
  poz_no: string;
  tanim: string;
  birim: string;
  miktar: number;
  eslesmeler: Eslesme[];
};

const TUR_LABEL: Record<string, string> = { Main: "Ana Sözleşme", Sub: "Taşeron Sözleşmesi", Addendum: "Zeyilname" };

function fmtDate(s?: string) {
  if (!s) return null;
  const [y, m, d] = s.split("-");
  return `${d}.${m}.${y}`;
}
function fmtMiktar(n: number) {
  return n.toLocaleString("tr-TR", { maximumFractionDigits: 2 });
}

export default function SozlesmeTakipPage() {
  const { current } = useProjects();
  const pid = current?.id;

  const [items, setItems] = useState<Item[]>([]);
  const [loading, setLoading] = useState(true);
  const [err, setErr] = useState<string | null>(null);
  const [q, setQ] = useState("");
  const [sadeceEslesmeyenler, setSadeceEslesmeyenler] = useState(false);

  useEffect(() => {
    if (!pid) return;
    setLoading(true);
    setErr(null);
    api<{ items: Item[] }>(`/projects/${pid}/sozlesme-takip`, { projectId: pid })
      .then((r) => setItems(r.items ?? []))
      .catch(() => setErr("Sözleşme takip verisi yüklenemedi ya da erişim yetkiniz yok."))
      .finally(() => setLoading(false));
  }, [pid]);

  const filtered = useMemo(() => {
    const term = q.trim().toLowerCase();
    return items.filter((it) => {
      if (sadeceEslesmeyenler && it.eslesmeler.length > 0) return false;
      if (!term) return true;
      return (
        it.tanim.toLowerCase().includes(term) ||
        it.poz_no.toLowerCase().includes(term) ||
        it.kategori.toLowerCase().includes(term) ||
        it.eslesmeler.some((e) => e.taseron_adi.toLowerCase().includes(term))
      );
    });
  }, [items, q, sadeceEslesmeyenler]);

  const stats = useMemo(() => {
    const toplam = items.length;
    const eslesen = items.filter((i) => i.eslesmeler.length > 0).length;
    return { toplam, eslesen, eslesmeyen: toplam - eslesen };
  }, [items]);

  if (!current) return <p className="text-beton-400 text-sm">Önce üst bardan bir proje seçin.</p>;

  return (
    <div className="max-w-6xl mx-auto space-y-4">
      <div>
        <h1 className="font-display font-extrabold text-xl text-white">Sözleşme Takip</h1>
        <p className="text-xs text-beton-400 mt-0.5">
          Proje Keşfi kalemleri × poz no eşleşmesiyle bulunan taşeron ve sözleşme bilgileri — {current.name}
        </p>
      </div>

      {err && <p className="text-sm text-red-400">{err}</p>}

      {!loading && items.length > 0 && (
        <div className="grid grid-cols-3 gap-3">
          <div className="rounded-lg border border-beton-800 bg-beton-900 p-3">
            <p className="text-xs text-beton-500 mb-1">Toplam Keşif Kalemi</p>
            <p className="text-lg font-bold text-white">{stats.toplam}</p>
          </div>
          <div className="rounded-lg border border-beton-800 bg-beton-900 p-3">
            <p className="text-xs text-beton-500 mb-1">Taşerona Bağlı</p>
            <p className="text-lg font-bold text-green-400">{stats.eslesen}</p>
          </div>
          <div className="rounded-lg border border-beton-800 bg-beton-900 p-3">
            <p className="text-xs text-beton-500 mb-1">Sözleşmesi Girilmemiş</p>
            <p className={`text-lg font-bold ${stats.eslesmeyen > 0 ? "text-amber-400" : "text-beton-400"}`}>
              {stats.eslesmeyen}
            </p>
          </div>
        </div>
      )}

      {items.length > 0 && (
        <div className="rounded-lg border border-beton-800 bg-beton-900 p-3 flex flex-wrap items-center gap-3">
          <input
            className="flex-1 min-w-[200px] rounded-md bg-beton-950 border border-beton-800 px-3 py-2 text-sm text-beton-100 outline-none focus:border-emniyet-500"
            placeholder="Poz no, imalat adı, kategori ya da taşeron ara…"
            value={q}
            onChange={(e) => setQ(e.target.value)}
          />
          <label className="flex items-center gap-2 text-xs text-beton-300 cursor-pointer">
            <input type="checkbox" checked={sadeceEslesmeyenler}
              onChange={(e) => setSadeceEslesmeyenler(e.target.checked)}
              className="accent-emniyet-500" />
            Sadece sözleşmesi girilmemiş kalemler
          </label>
        </div>
      )}

      {loading ? (
        <p className="text-beton-500 text-sm">Yükleniyor…</p>
      ) : items.length === 0 ? (
        <p className="text-beton-500 text-sm text-center py-10">
          Bu projede henüz Proje Keşfi kalemi girilmemiş.
        </p>
      ) : filtered.length === 0 ? (
        <p className="text-beton-500 text-sm text-center py-10">Filtreyle eşleşen kayıt yok.</p>
      ) : (
        <div className="border border-beton-800 rounded-lg overflow-hidden overflow-x-auto">
          <table className="w-full text-sm min-w-[860px]">
            <thead>
              <tr className="bg-beton-900 border-b border-beton-800">
                <th className="py-2 px-3 text-left text-xs text-beton-500 font-medium w-24">Poz No</th>
                <th className="py-2 px-3 text-left text-xs text-beton-500 font-medium">İmalat</th>
                <th className="py-2 px-3 text-right text-xs text-beton-500 font-medium w-32">Metraj</th>
                <th className="py-2 px-3 text-left text-xs text-beton-500 font-medium w-72">Taşeron / Sözleşme</th>
              </tr>
            </thead>
            <tbody>
              {filtered.map((it, i) => {
                const prevKategori = i > 0 ? filtered[i - 1].kategori : null;
                return (
                  <>
                    {it.kategori !== prevKategori && (
                      <tr key={`kat-${it.id}`} className="bg-beton-950/60">
                        <td colSpan={4} className="py-1.5 px-3 text-[11px] font-bold uppercase tracking-wider text-beton-500">
                          {it.kategori}
                        </td>
                      </tr>
                    )}
                    <tr key={it.id} className="border-b border-beton-800/50 hover:bg-beton-900/30">
                      <td className="py-2 px-3 text-beton-400 text-xs font-mono">{it.poz_no || "—"}</td>
                      <td className="py-2 px-3 text-beton-200">{it.tanim}</td>
                      <td className="py-2 px-3 text-right text-beton-300 text-xs font-mono">
                        {fmtMiktar(it.miktar)} {it.birim}
                      </td>
                      <td className="py-2 px-3">
                        {it.eslesmeler.length === 0 ? (
                          <span className="text-xs text-beton-600 italic">Sözleşme girilmemiş</span>
                        ) : (
                          <div className="flex flex-col gap-1">
                            {it.eslesmeler.map((e, idx) => (
                              <div key={idx} className="flex flex-wrap items-center gap-1.5">
                                <span className="text-xs font-medium text-beton-100">{e.taseron_adi}</span>
                                {e.sozlesme_no ? (
                                  <span className="rounded-full border border-emniyet-500/40 bg-emniyet-500/10 px-2 py-0.5 text-[10.5px] text-emniyet-400">
                                    {e.sozlesme_no}
                                    {e.sozlesme_tarihi && ` · ${fmtDate(e.sozlesme_tarihi)}`}
                                  </span>
                                ) : (
                                  <span className="text-[10.5px] text-beton-500">sözleşme no girilmemiş</span>
                                )}
                                {e.sozlesme_turu && e.sozlesme_turu !== "Sub" && (
                                  <span className="text-[10px] text-beton-500">({TUR_LABEL[e.sozlesme_turu] ?? e.sozlesme_turu})</span>
                                )}
                              </div>
                            ))}
                          </div>
                        )}
                      </td>
                    </tr>
                  </>
                );
              })}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
