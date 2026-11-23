import { useState } from "react";
import { apiDownload, apiFetchBlob, describeLoadError } from "../../api/client";
import { useProjects } from "../ProjectContext";

// Malzeme - Envanter Raporları: Depo-Stok ve Makine-Ekipman raporlarını PDF olarak
// üretir. Veriler sunucudaki canlı tablolardan okunur; dönem seçimi iki rapora da uygulanır.

type ReportKey = "depo-stok" | "makine-ekipman";

const REPORTS: { key: ReportKey; title: string; desc: string }[] = [
  {
    key: "depo-stok",
    title: "Depo-Stok Raporu",
    desc: "Stok bakiyesi, minimum stok uyumu, kapsama günü, dönem hareketleri, açık zimmetler ve hareket dökümü.",
  },
  {
    key: "makine-ekipman",
    title: "Makine-Ekipman Raporu",
    desc: "Envanter durumu, bakım takvimi, kullanım oranı, kiralık ekipmanın boşta maliyeti ve transfer talepleri.",
  },
];

const inputCls = "rounded-md border border-beton-700 bg-beton-950 px-2 py-1.5 text-sm text-beton-100";
const btnPrimary = "rounded-md bg-emniyet-500 px-3 py-1.5 text-xs font-medium text-white-solid hover:brightness-110 disabled:opacity-50";
const btnGhost = "rounded-md border border-beton-700 px-3 py-1.5 text-xs text-beton-200 hover:border-beton-500 disabled:opacity-50";

export default function MalzemeEnvanterPage() {
  const { current } = useProjects();
  const pid = current?.id;

  const bugun = new Date().toISOString().slice(0, 10);
  const [from, setFrom] = useState(`${bugun.slice(0, 8)}01`);
  const [to, setTo] = useState(bugun);
  const [busy, setBusy] = useState<string | null>(null); // "<rapor>:<onizle|indir>"
  const [err, setErr] = useState<string | null>(null);

  if (!current) return <p className="text-beton-400">Önce üst bardan bir proje seçin.</p>;

  const aralikHatali = from > to;
  const query = `from=${from}&to=${to}`;

  async function onizle(key: ReportKey) {
    setErr(null);
    setBusy(`${key}:onizle`);
    try {
      const url = await apiFetchBlob(`/projects/${pid}/inventory/${key}.pdf?${query}`);
      window.open(url, "_blank", "noopener");
    } catch (e) {
      setErr(describeLoadError(e, "Rapor"));
    } finally {
      setBusy(null);
    }
  }

  async function indir(key: ReportKey) {
    setErr(null);
    setBusy(`${key}:indir`);
    try {
      await apiDownload(`/projects/${pid}/inventory/${key}.pdf?${query}&download=1`, `${key}.pdf`);
    } catch (e) {
      setErr(describeLoadError(e, "Rapor"));
    } finally {
      setBusy(null);
    }
  }

  return (
    <div className="space-y-5 max-w-4xl">
      <div>
        <p className="text-xs text-beton-500">Proje Raporları › Malzeme - Envanter</p>
        <h1 className="font-display text-2xl font-bold tracking-tight text-beton-100">Malzeme - Envanter Raporları</h1>
        <p className="text-sm text-beton-400 mt-1">
          Depo, stok, zimmet ve makine-ekipman verilerinden PDF raporlar üretir.
        </p>
      </div>

      <div className="flex flex-wrap items-end gap-3 rounded-lg border border-beton-800 bg-beton-900 p-4">
        <label className="flex flex-col gap-1 text-xs text-beton-400">
          Başlangıç
          <input type="date" value={from} onChange={(e) => setFrom(e.target.value)} className={inputCls} />
        </label>
        <label className="flex flex-col gap-1 text-xs text-beton-400">
          Bitiş
          <input type="date" value={to} onChange={(e) => setTo(e.target.value)} className={inputCls} />
        </label>
        {aralikHatali && <p className="text-xs text-red-400">Başlangıç tarihi bitişten sonra olamaz.</p>}
      </div>

      {err && <p className="text-sm text-red-400">{err}</p>}

      <div className="grid gap-4 md:grid-cols-2">
        {REPORTS.map((r) => (
          <section key={r.key} className="rounded-lg border border-beton-800 bg-beton-900 p-5 flex flex-col gap-3">
            <h2 className="text-base font-semibold text-beton-100">{r.title}</h2>
            <p className="text-sm text-beton-400 leading-relaxed">{r.desc}</p>
            <div className="mt-auto flex flex-wrap gap-2 pt-2">
              <button
                onClick={() => onizle(r.key)}
                disabled={aralikHatali || busy !== null}
                className={btnPrimary}
              >
                {busy === `${r.key}:onizle` ? "Hazırlanıyor…" : "Raporu üret"}
              </button>
              <button
                onClick={() => indir(r.key)}
                disabled={aralikHatali || busy !== null}
                className={btnGhost}
              >
                {busy === `${r.key}:indir` ? "İndiriliyor…" : "PDF indir"}
              </button>
            </div>
          </section>
        ))}
      </div>
    </div>
  );
}
