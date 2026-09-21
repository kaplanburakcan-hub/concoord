import { useEffect, useMemo, useState } from "react";
import { api } from "../../api/client";

// Verimlilik Normları — firma çapında, projeden bağımsız referans veri.
// Kullanıcının paylaştığı "Manhour Database.xlsx"ten (migration 000066)
// tek seferlik yüklendi: genel normlar (min~max aralık + ortalama) ve 5
// geçmiş projenin (Adana, Yozgat, Elazığ, İkitelli, Bursa) poz bazlı
// gerçekleşen birim adam-saat kıyaslaması. Salt okunur — yeni proje
// planlarken ya da girilen adam-saatleri değerlendirirken referans içindir.

type Norm = {
  id: string; kategori: string; imalat_aciklamasi: string;
  birim: string | null; araligi: string | null; ortalama_adam_saat: number;
};
type HistoryRow = {
  id: string; kategori: string | null; poz_no: string; imalat: string;
  adana: number | null; yozgat: number | null; elazig: number | null;
  ikitelli: number | null; bursa: number | null; ortalama: number | null;
};

const th = "text-left text-[10px] font-bold uppercase tracking-wider text-beton-400 pb-2 pr-3 whitespace-nowrap";
const td = "py-2 pr-3 text-[12.5px] text-beton-100 border-b border-beton-800/60 align-middle";
const tdNum = "py-2 pr-3 text-[12.5px] text-beton-300 border-b border-beton-800/60 align-middle text-right tabular-nums";

function fmt(n: number | null): string {
  if (n == null) return "—";
  return n.toLocaleString("tr-TR", { minimumFractionDigits: 2, maximumFractionDigits: 2 });
}

export default function VerimlilikNormlariPage() {
  const [tab, setTab] = useState<"normlar" | "kiyaslama">("normlar");
  const [norms, setNorms] = useState<Norm[]>([]);
  const [history, setHistory] = useState<HistoryRow[]>([]);
  const [loading, setLoading] = useState(true);
  const [err, setErr] = useState<string | null>(null);
  const [search, setSearch] = useState("");

  useEffect(() => {
    setLoading(true);
    setErr(null);
    Promise.all([
      api<{ norms: Norm[] }>("/manhour/norms"),
      api<{ history: HistoryRow[] }>("/manhour/project-history"),
    ])
      .then(([n, h]) => { setNorms(n.norms ?? []); setHistory(h.history ?? []); })
      .catch(() => setErr("Veriler yüklenemedi."))
      .finally(() => setLoading(false));
  }, []);

  const filteredNorms = useMemo(() => {
    const q = search.trim().toLowerCase();
    if (!q) return norms;
    return norms.filter(n =>
      n.imalat_aciklamasi.toLowerCase().includes(q) || n.kategori.toLowerCase().includes(q)
    );
  }, [norms, search]);

  const filteredHistory = useMemo(() => {
    const q = search.trim().toLowerCase();
    if (!q) return history;
    return history.filter(h =>
      h.imalat.toLowerCase().includes(q) || h.poz_no.includes(q) || (h.kategori ?? "").toLowerCase().includes(q)
    );
  }, [history, search]);

  let lastKategoriNorm = "";
  let lastKategoriHist = "";

  return (
    <div className="max-w-5xl mx-auto space-y-4">
      <div>
        <h1 className="font-display font-extrabold text-xl text-white">Verimlilik Normları</h1>
        <p className="text-xs text-beton-500 mt-0.5">
          5 geçmiş projeden (Adana, Yozgat, Elazığ, İkitelli, Bursa) derlenmiş birim adam-saat referans veritabanı — firma çapında, salt okunur.
        </p>
      </div>

      {err && <p className="text-red-400 text-sm">{err}</p>}

      <div className="flex items-center gap-2">
        <button
          onClick={() => setTab("normlar")}
          className={`rounded-full border px-3 py-1.5 text-xs font-semibold transition-colors ${tab === "normlar" ? "bg-emniyet-500 border-emniyet-500 text-beton-950" : "border-beton-700 text-beton-400 hover:border-beton-500"}`}
        >
          Genel Normlar
        </button>
        <button
          onClick={() => setTab("kiyaslama")}
          className={`rounded-full border px-3 py-1.5 text-xs font-semibold transition-colors ${tab === "kiyaslama" ? "bg-emniyet-500 border-emniyet-500 text-beton-950" : "border-beton-700 text-beton-400 hover:border-beton-500"}`}
        >
          Proje Kıyaslaması (İcmal)
        </button>
        <input
          className="ml-auto w-64 rounded-md bg-beton-950 border border-beton-800 px-3 py-1.5 text-sm text-beton-200 outline-none focus:border-emniyet-500"
          placeholder="İmalat veya poz ara…"
          value={search}
          onChange={e => setSearch(e.target.value)}
        />
      </div>

      <div className="rounded-lg border border-beton-800 bg-beton-900 overflow-hidden">
        {loading ? (
          <p className="px-4 py-6 text-sm text-beton-500 text-center">Yükleniyor…</p>
        ) : tab === "normlar" ? (
          filteredNorms.length === 0 ? (
            <p className="px-4 py-6 text-sm text-beton-500 text-center">Eşleşen kayıt yok.</p>
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full min-w-[600px]">
                <thead>
                  <tr>
                    <th className={`${th} pl-4`}>İmalat</th>
                    <th className={th}>Birim</th>
                    <th className={th}>Aralık</th>
                    <th className={`${th} text-right pr-4`}>Ortalama Br. Ad.Sa</th>
                  </tr>
                </thead>
                <tbody>
                  {filteredNorms.map(n => {
                    const showKategori = n.kategori !== lastKategoriNorm;
                    lastKategoriNorm = n.kategori;
                    return (
                      <>
                        {showKategori && (
                          <tr key={`${n.id}-kat`}>
                            <td colSpan={4} className="pl-4 pt-3 pb-1 text-[10px] font-bold uppercase tracking-wider text-emniyet-500">
                              {n.kategori}
                            </td>
                          </tr>
                        )}
                        <tr key={n.id} className="hover:bg-beton-800/30 transition-colors">
                          <td className={`${td} pl-4`}>{n.imalat_aciklamasi}</td>
                          <td className={td}>{n.birim ?? "—"}</td>
                          <td className={td}>{n.araligi ?? "—"}</td>
                          <td className={`${tdNum} pr-4 font-semibold text-beton-100`}>{fmt(n.ortalama_adam_saat)}</td>
                        </tr>
                      </>
                    );
                  })}
                </tbody>
              </table>
            </div>
          )
        ) : filteredHistory.length === 0 ? (
          <p className="px-4 py-6 text-sm text-beton-500 text-center">Eşleşen kayıt yok.</p>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full min-w-[760px]">
              <thead>
                <tr>
                  <th className={`${th} pl-4`}>Poz No</th>
                  <th className={th}>İmalat</th>
                  <th className={`${th} text-right`}>Adana</th>
                  <th className={`${th} text-right`}>Yozgat</th>
                  <th className={`${th} text-right`}>Elazığ</th>
                  <th className={`${th} text-right`}>İkitelli</th>
                  <th className={`${th} text-right`}>Bursa</th>
                  <th className={`${th} text-right pr-4`}>Ortalama</th>
                </tr>
              </thead>
              <tbody>
                {filteredHistory.map(h => {
                  const showKategori = (h.kategori ?? "") !== lastKategoriHist;
                  lastKategoriHist = h.kategori ?? "";
                  return (
                    <>
                      {showKategori && h.kategori && (
                        <tr key={`${h.id}-kat`}>
                          <td colSpan={8} className="pl-4 pt-3 pb-1 text-[10px] font-bold uppercase tracking-wider text-emniyet-500">
                            {h.kategori}
                          </td>
                        </tr>
                      )}
                      <tr key={h.id} className="hover:bg-beton-800/30 transition-colors">
                        <td className={`${td} pl-4 font-mono whitespace-nowrap`}>{h.poz_no}</td>
                        <td className={td}>{h.imalat}</td>
                        <td className={tdNum}>{fmt(h.adana)}</td>
                        <td className={tdNum}>{fmt(h.yozgat)}</td>
                        <td className={tdNum}>{fmt(h.elazig)}</td>
                        <td className={tdNum}>{fmt(h.ikitelli)}</td>
                        <td className={tdNum}>{fmt(h.bursa)}</td>
                        <td className={`${tdNum} pr-4 font-semibold text-beton-100`}>{fmt(h.ortalama)}</td>
                      </tr>
                    </>
                  );
                })}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </div>
  );
}
