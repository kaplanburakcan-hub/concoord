import type { ReactNode } from "react";
import { useCallback, useEffect, useState } from "react";
import { api, apiDownload } from "../../api/client";
import { Can } from "../../auth/guards";
import { useProjects } from "../../projects/ProjectContext";

// Faz 9 — Aylık Yönetim Raporu (EVM + finansal + İSG + tedarik).
// Haftalık raporla aynı akış: snapshot API'de dondurulur, PDF worker'da
// üretilir (Pending → Ready), indirilir. Eşik ayarları da bu ekranda
// (projects.edit iznine bağlı) yönetilir.

type Monthly = {
  id: string;
  year: number;
  month: number;
  period_start: string;
  period_end: string;
  status: "Pending" | "Ready" | "Failed";
  error?: string;
  generated_by_name: string;
  has_pdf: boolean;
  created_at: string;
};
type Settings = { cpi_min: number; spi_min: number; finding_aging_days: number };

export default function MonthlyReportsPage() {
  const { current } = useProjects();
  const [items, setItems] = useState<Monthly[] | null>(null);
  const [month, setMonth] = useState(() => new Date().toISOString().slice(0, 7));
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  const load = useCallback(async () => {
    if (!current) return;
    try {
      const r = await api<{ monthly_reports: Monthly[] }>(
        `/projects/${current.id}/monthly-reports`
      );
      setItems(r.monthly_reports);
    } catch {
      setErr("Aylık raporlar yüklenemedi.");
    }
  }, [current]);

  useEffect(() => {
    setItems(null);
    setErr(null);
    load();
  }, [load]);

  // Pending kayıt varken kısa aralıklı yenile (worker PDF'i üretene dek).
  useEffect(() => {
    if (!items?.some((i) => i.status === "Pending")) return;
    const t = setInterval(load, 4000);
    return () => clearInterval(t);
  }, [items, load]);

  async function generate() {
    if (!current) return;
    setBusy(true);
    setErr(null);
    try {
      await api(`/projects/${current.id}/monthly-reports`, {
        method: "POST",
        body: { month },
      });
      await load();
    } catch {
      setErr("Rapor üretimi başlatılamadı (ay biçimi YYYY-MM olmalı).");
    } finally {
      setBusy(false);
    }
  }

  if (!current)
    return <p className="text-sm text-beton-400">Önce üst bardan bir proje seçin.</p>;

  return (
    <div>
      <p className="font-mono text-xs tracking-[0.3em] text-emniyet-500 uppercase">
        Faz 9 · Aylık Yönetim Raporu
      </p>
      <h1 className="font-display text-3xl font-extrabold text-white mt-2">
        Aylık Raporlar — {current.code}
      </h1>
      <p className="mt-1 text-sm text-beton-400">
        EVM (PV/EV/AC, SPI/CPI, EAC/ETC) + kesinleşen hakedişler ve kesinti
        dökümü + İSG performansı + tedarik özeti. Rakamlar üretim anında
        snapshot olarak dondurulur.
      </p>

      <div className="mt-6 flex items-center gap-2">
        <input
          value={month}
          onChange={(e) => setMonth(e.target.value)}
          placeholder="YYYY-MM"
          className="w-32 rounded-md bg-beton-900 border border-beton-700 px-2 py-1.5 text-sm text-beton-100 font-mono outline-none focus:border-emniyet-500"
        />
        <Can perm="reports.generate_weekly">
          <button
            onClick={generate}
            disabled={busy}
            className="rounded-md bg-emniyet-500 px-4 py-1.5 text-sm font-semibold text-beton-950 hover:brightness-110 transition disabled:opacity-50"
          >
            {busy ? "Üretiliyor…" : "Rapor üret"}
          </button>
        </Can>
      </div>
      {err && <p className="mt-2 text-sm text-red-400">{err}</p>}

      <div className="mt-6 rounded-lg border border-beton-800 bg-beton-900 overflow-hidden">
        <table className="w-full text-sm">
          <thead>
            <tr className="text-left text-xs uppercase tracking-wider text-beton-500 border-b border-beton-800">
              <th className="px-4 py-2">Dönem</th>
              <th className="px-4 py-2">Durum</th>
              <th className="px-4 py-2">Üreten</th>
              <th className="px-4 py-2">Oluşturma</th>
              <th className="px-4 py-2 text-right">PDF</th>
            </tr>
          </thead>
          <tbody>
            {!items && (
              <tr>
                <td colSpan={5} className="px-4 py-6 text-center text-beton-400">
                  Yükleniyor…
                </td>
              </tr>
            )}
            {items?.length === 0 && (
              <tr>
                <td colSpan={5} className="px-4 py-6 text-center text-beton-400">
                  Henüz aylık rapor üretilmedi.
                </td>
              </tr>
            )}
            {items?.map((m) => (
              <tr key={m.id} className="border-b border-beton-800/60 last:border-0">
                <td className="px-4 py-2 font-mono text-beton-100">
                  {m.year}-{String(m.month).padStart(2, "0")}
                </td>
                <td className="px-4 py-2">
                  <StatusBadge status={m.status} />
                  {m.status === "Failed" && m.error && (
                    <span className="ml-2 text-xs text-red-400">{m.error}</span>
                  )}
                </td>
                <td className="px-4 py-2 text-beton-300">{m.generated_by_name}</td>
                <td className="px-4 py-2 font-mono text-xs text-beton-400">
                  {new Date(m.created_at).toLocaleString("tr-TR")}
                </td>
                <td className="px-4 py-2 text-right">
                  {m.status === "Ready" && m.has_pdf ? (
                    <button
                      onClick={() =>
                        apiDownload(
                          `/projects/${current.id}/monthly-reports/${m.id}/download`,
                          `aylik-rapor-${m.year}-${m.month}.pdf`
                        )
                      }
                      className="text-emniyet-500 hover:underline text-sm"
                    >
                      İndir
                    </button>
                  ) : (
                    <span className="text-beton-600 text-xs">—</span>
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <Can perm="projects.edit">
        <ThresholdEditor projectId={current.id} />
      </Can>
    </div>
  );
}

function StatusBadge({ status }: { status: Monthly["status"] }) {
  const cls =
    status === "Ready"
      ? "bg-emniyet-500/15 text-emniyet-500"
      : status === "Failed"
        ? "bg-red-500/15 text-red-400"
        : "bg-beton-800 text-beton-300";
  const tr = status === "Ready" ? "Hazır" : status === "Failed" ? "Başarısız" : "Üretiliyor";
  return <span className={"font-mono text-xs px-2 py-0.5 rounded " + cls}>{tr}</span>;
}

// Eşik tabanlı uyarı parametreleri (Plan §7: kontrol tanımları veri olarak).
function ThresholdEditor({ projectId }: { projectId: string }) {
  const [s, setS] = useState<Settings | null>(null);
  const [msg, setMsg] = useState<string | null>(null);

  useEffect(() => {
    api<{ settings: Settings }>(`/projects/${projectId}/control-settings`)
      .then((r) => setS(r.settings))
      .catch(() => setMsg("Eşikler yüklenemedi."));
  }, [projectId]);

  async function save() {
    if (!s) return;
    setMsg(null);
    try {
      await api(`/projects/${projectId}/control-settings`, { method: "PUT", body: s });
      setMsg("Kaydedildi — worker sonraki taramada yeni eşikleri kullanır.");
    } catch {
      setMsg("Kaydedilemedi (CPI/SPI 0-2, yaşlanma ≥ 1 gün).");
    }
  }

  if (!s) return null;
  return (
    <div className="mt-6 rounded-lg border border-beton-800 bg-beton-900 p-4">
      <h2 className="text-sm font-semibold text-white">Otomatik uyarı eşikleri</h2>
      <p className="mt-1 text-xs text-beton-400">
        İhlalde proje yöneticilerine bildirim gider (aynı uyarı ay içinde bir kez).
      </p>
      <div className="mt-3 flex flex-wrap items-end gap-4">
        <Field label="CPI alt eşiği">
          <input
            type="number"
            step="0.01"
            value={s.cpi_min}
            onChange={(e) => setS({ ...s, cpi_min: Number(e.target.value) })}
            className="w-24 rounded bg-beton-950 border border-beton-800 px-2 py-1 text-sm text-beton-100 font-mono"
          />
        </Field>
        <Field label="SPI alt eşiği">
          <input
            type="number"
            step="0.01"
            value={s.spi_min}
            onChange={(e) => setS({ ...s, spi_min: Number(e.target.value) })}
            className="w-24 rounded bg-beton-950 border border-beton-800 px-2 py-1 text-sm text-beton-100 font-mono"
          />
        </Field>
        <Field label="Bulgu yaşlanma (gün)">
          <input
            type="number"
            value={s.finding_aging_days}
            onChange={(e) => setS({ ...s, finding_aging_days: Number(e.target.value) })}
            className="w-24 rounded bg-beton-950 border border-beton-800 px-2 py-1 text-sm text-beton-100 font-mono"
          />
        </Field>
        <button
          onClick={save}
          className="rounded-md bg-emniyet-500 px-4 py-1.5 text-sm font-semibold text-beton-950 hover:brightness-110 transition"
        >
          Kaydet
        </button>
      </div>
      {msg && <p className="mt-2 text-xs text-beton-300">{msg}</p>}
    </div>
  );
}

function Field({ label, children }: { label: string; children: ReactNode }) {
  return (
    <label className="block">
      <span className="block text-[11px] uppercase tracking-wider text-beton-500 mb-1">
        {label}
      </span>
      {children}
    </label>
  );
}
