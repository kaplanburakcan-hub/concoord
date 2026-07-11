import { useCallback, useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { api } from "../../api/client";
import { useAuth } from "../../auth/AuthContext";
import { useProjects } from "../../projects/ProjectContext";
import OfflineQueueBanner from "./OfflineQueueBanner";

// Faz 6 — Günlük saha raporları listesi (mobil öncelikli: kart görünümü).
// Tarih başına GEÇERLİ revizyon listelenir; Submitted rapor değişmezdir,
// düzeltme detay sayfasından "Revizyon aç" ile yapılır.

export type DailyReport = {
  id: string;
  report_date: string;
  revision_no: number;
  status: "Draft" | "Submitted";
  temperature_min?: number;
  temperature_max?: number;
  weather?: { condition?: string };
  author_name: string;
  submitted_at?: string;
  notes?: string;
};

export const DR_STATUS_LABEL: Record<string, string> = {
  Draft: "Taslak",
  Submitted: "Gönderildi",
};
export const DR_STATUS_STYLE: Record<string, string> = {
  Draft: "bg-beton-800 text-beton-200 border-beton-700",
  Submitted: "bg-green-500/15 text-green-300 border-green-500/40",
};

export default function DailyReportsPage() {
  const { current } = useProjects();
  const { can } = useAuth();
  const pid = current?.id;
  const [reports, setReports] = useState<DailyReport[]>([]);
  const [err, setErr] = useState<string | null>(null);

  const load = useCallback(async () => {
    if (!pid) return;
    setErr(null);
    try {
      const res = await api<{ daily_reports: DailyReport[] }>(
        `/projects/${pid}/daily-reports`,
        { projectId: pid }
      );
      setReports(res.daily_reports);
    } catch {
      setErr("Günlük raporlar yüklenemedi ya da erişim yetkiniz yok.");
    }
  }, [pid]);

  useEffect(() => {
    load();
  }, [load]);

  if (!pid) {
    return <p className="text-beton-400 text-sm">Önce üst bardan bir proje seçin.</p>;
  }

  return (
    <div className="max-w-3xl mx-auto space-y-4">
      <OfflineQueueBanner onSynced={load} />

      <div className="flex items-center justify-between gap-3 flex-wrap">
        <h1 className="font-display font-extrabold text-xl text-white">Günlük Saha Raporları</h1>
        <div className="flex items-center gap-2">
          {can("reports.view") && (
            <Link
              to="/saha-raporlari/haftalik"
              className="rounded-md border border-beton-700 px-3 py-2 text-sm text-beton-200 hover:border-emniyet-500"
            >
              Haftalık Raporlar
            </Link>
          )}
          {can("reports.create_daily") && (
            <Link
              to="/saha-raporlari/yeni"
              className="rounded-md bg-emniyet-500 px-3 py-2 text-sm font-medium text-beton-950 hover:brightness-110"
            >
              + Yeni Rapor
            </Link>
          )}
        </div>
      </div>

      {err && <p className="text-red-400 text-sm">{err}</p>}
      {!err && reports.length === 0 && (
        <p className="text-beton-400 text-sm">Henüz günlük rapor girilmemiş.</p>
      )}

      <ul className="space-y-2">
        {reports.map((r) => (
          <li key={r.id}>
            <Link
              to={`/saha-raporlari/${r.id}`}
              className="block rounded-lg border border-beton-800 bg-beton-900 p-4 hover:border-emniyet-500"
            >
              <div className="flex items-center justify-between gap-2 flex-wrap">
                <span className="font-medium text-white">
                  {formatDateTR(r.report_date)}
                  {r.revision_no > 1 && (
                    <span className="ml-2 text-xs text-emniyet-500">rev {r.revision_no}</span>
                  )}
                </span>
                <span
                  className={`rounded-full border px-2 py-0.5 text-xs ${DR_STATUS_STYLE[r.status]}`}
                >
                  {DR_STATUS_LABEL[r.status]}
                </span>
              </div>
              <div className="mt-1 text-xs text-beton-400 flex flex-wrap gap-x-4 gap-y-1">
                <span>Yazan: {r.author_name}</span>
                {r.weather?.condition && <span>Hava: {r.weather.condition}</span>}
                {(r.temperature_min != null || r.temperature_max != null) && (
                  <span>
                    Sıcaklık: {r.temperature_min ?? "–"} / {r.temperature_max ?? "–"} °C
                  </span>
                )}
              </div>
            </Link>
          </li>
        ))}
      </ul>
    </div>
  );
}

export function formatDateTR(iso: string): string {
  const [y, m, d] = iso.split("-");
  return `${d}.${m}.${y}`;
}
