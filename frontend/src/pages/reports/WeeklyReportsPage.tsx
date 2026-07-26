import { useCallback, useEffect, useRef, useState } from "react";
import { Link } from "react-router-dom";
import { api, apiDownload, RequestError } from "../../api/client";
import { useAuth } from "../../auth/AuthContext";
import { useProjects } from "../../projects/ProjectContext";

// Faz 6 – Haftalık İlerleme Raporları.
// "Tek tıkla PDF": kullanıcı haftayı seçer, API snapshot'ı dondurur ve işi
// kuyruğa atar; worker PDF'i üretince satır Ready olur ve indirilebilir.
// Pending kayıtlar kısa aralıkla yoklanır (worker genelde saniyeler içinde biter).

type Weekly = {
  id: string;
  week_no: number;
  period_start: string;
  period_end: string;
  status: "Pending" | "Ready" | "Failed";
  error?: string;
  generated_by_name: string;
  has_pdf: boolean;
  created_at: string;
};

const W_STATUS_LABEL: Record<string, string> = {
  Pending: "Üretiliyor…",
  Ready: "Hazır",
  Failed: "Başarısız",
};
const W_STATUS_STYLE: Record<string, string> = {
  Pending: "bg-blue-500/15 text-blue-300 border-blue-500/40",
  Ready: "bg-green-500/15 text-green-300 border-green-500/40",
  Failed: "bg-red-500/15 text-red-300 border-red-500/40",
};

function mondayOf(iso: string): string {
  const d = new Date(iso + "T00:00:00");
  const wd = d.getDay() === 0 ? 7 : d.getDay();
  d.setDate(d.getDate() - (wd - 1));
  return d.toISOString().slice(0, 10);
}

export default function WeeklyReportsPage() {
  const { current } = useProjects();
  const { can } = useAuth();
  const pid = current?.id;

  const [list, setList] = useState<Weekly[]>([]);
  const [err, setErr] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [pickDate, setPickDate] = useState(mondayOf(new Date().toISOString().slice(0, 10)));
  const pollRef = useRef<number | null>(null);

  const load = useCallback(async () => {
    if (!pid) return;
    try {
      const res = await api<{ weekly_reports: Weekly[] }>(
        `/projects/${pid}/weekly-reports`,
        { projectId: pid }
      );
      setList(res.weekly_reports);
      return res.weekly_reports;
    } catch {
      setErr("Haftalık raporlar yüklenemedi ya da erişim yetkiniz yok.");
      return [];
    }
  }, [pid]);

  useEffect(() => {
    load();
  }, [load]);

  // Pending varken kısa yoklama.
  useEffect(() => {
    if (pollRef.current) window.clearInterval(pollRef.current);
    if (list.some((w) => w.status === "Pending")) {
      pollRef.current = window.setInterval(async () => {
        const fresh = await load();
        if (fresh && !fresh.some((w) => w.status === "Pending") && pollRef.current) {
          window.clearInterval(pollRef.current);
          pollRef.current = null;
        }
      }, 4000);
    }
    return () => {
      if (pollRef.current) window.clearInterval(pollRef.current);
    };
  }, [list, load]);

  async function generate() {
    if (!pid) return;
    setBusy(true);
    setErr(null);
    try {
      await api(`/projects/${pid}/weekly-reports`, {
        method: "POST",
        body: { period_start: pickDate },
        projectId: pid,
      });
      load();
    } catch (e) {
      setErr(e instanceof RequestError ? e.message : "Rapor üretimi başlatılamadı.");
    } finally {
      setBusy(false);
    }
  }

  if (!pid) return <p className="text-beton-400 text-sm">Önce üst bardan bir proje seçin.</p>;

  return (
    <div className="max-w-3xl mx-auto space-y-4">
      <div className="flex items-center justify-between gap-3 flex-wrap">
        <h1 className="font-display font-extrabold text-xl text-white">Haftalık İlerleme Raporları</h1>
        {can("reports.view") && (
          <Link
            to="/saha-raporlari"
            className="rounded-md border border-beton-700 px-3 py-2 text-sm text-beton-200 hover:border-emniyet-500"
          >
            ← Günlük Raporlar
          </Link>
        )}
      </div>

      {can("reports.generate_weekly") && (
        <div className="rounded-lg border border-beton-800 bg-beton-900 p-4 flex items-end gap-3 flex-wrap">
          <div>
            <label className="block text-xs text-beton-400 mb-1">Hafta (herhangi bir günü seçin)</label>
            <input
              type="date"
              value={pickDate}
              onChange={(e) => setPickDate(e.target.value)}
              className="rounded-md bg-beton-950 border border-beton-800 px-3 py-2 text-sm text-beton-100 outline-none focus:border-emniyet-500"
            />
          </div>
          <button
            onClick={generate}
            disabled={busy}
            className="rounded-md bg-emniyet-500 px-4 py-2 text-sm font-medium text-beton-950 hover:brightness-110 disabled:opacity-50"
          >
            {busy ? "Başlatılıyor…" : "PDF Üret"}
          </button>
          <p className="text-xs text-beton-500 basis-full">
            Veriler üretim anında dondurulur (snapshot); sonradan yapılan günlük rapor revizyonları
            üretilen PDF'i değiştirmez.
          </p>
        </div>
      )}

      {err && <p className="text-red-400 text-sm">{err}</p>}
      {list.length === 0 && !err && (
        <p className="text-beton-400 text-sm">Henüz haftalık rapor üretilmemiş.</p>
      )}

      <ul className="space-y-2">
        {list.map((w) => (
          <li
            key={w.id}
            className="rounded-lg border border-beton-800 bg-beton-900 p-4 flex items-center justify-between gap-3 flex-wrap"
          >
            <div>
              <div className="font-medium text-white">
                Hafta {w.week_no}{" "}
                <span className="text-beton-400 text-sm">
                  ({fmt(w.period_start)} – {fmt(w.period_end)})
                </span>
              </div>
              <div className="text-xs text-beton-400 mt-1">
                Üreten: {w.generated_by_name}
                {w.status === "Failed" && w.error && (
                  <span className="text-red-400"> – {w.error}</span>
                )}
              </div>
            </div>
            <div className="flex items-center gap-2">
              <span className={`rounded-full border px-2 py-0.5 text-xs ${W_STATUS_STYLE[w.status]}`}>
                {W_STATUS_LABEL[w.status]}
              </span>
              {w.status === "Ready" && w.has_pdf && (
                <button
                  onClick={() =>
                    apiDownload(
                      `/projects/${pid}/weekly-reports/${w.id}/download`,
                      `haftalik-rapor-H${w.week_no}.pdf`
                    ).catch(() => setErr("PDF indirilemedi."))
                  }
                  className="rounded-md border border-beton-700 px-3 py-1.5 text-xs text-beton-100 hover:border-emniyet-500"
                >
                  PDF indir
                </button>
              )}
            </div>
          </li>
        ))}
      </ul>
    </div>
  );
}

function fmt(iso: string) {
  const [y, m, d] = iso.split("-");
  return `${d}.${m}.${y}`;
}
