import { useEffect, useMemo, useState } from "react";
import { api } from "../../api/client";
import { useProjects } from "../ProjectContext";

// PuantajPage — rota /proje/pdks-puantaj. Kullanıcının kararıyla mevcut
// manuel haftalık puantaj sayfasından (/proje/personel-puantaj) BİLİNÇLİ
// olarak AYRI: burası QR/GPS'ten TÜRETİLEN saatleri gösterir, o sayfaya
// dokunulmadı.

type Day = {
  id: string; project_id: string; person_id: string; person_name: string;
  work_date: string; derived_hours: number | null; adjusted_hours: number | null;
  overtime_hours: number; status: string; adjusted_reason?: string; approved_at?: string;
  has_flag: boolean;
};

function pad(n: number) { return String(n).padStart(2, "0"); }
function currentYm() {
  const d = new Date();
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}`;
}
function monthRange(ym: string): [string, string, number] {
  const [y, m] = ym.split("-").map(Number);
  const lastDay = new Date(y, m, 0).getDate();
  return [`${y}-${pad(m)}-01`, `${y}-${pad(m)}-${pad(lastDay)}`, lastDay];
}

export default function PuantajPage() {
  const { current } = useProjects();
  const [ym, setYm] = useState(currentYm());
  const [days, setDays] = useState<Day[]>([]);
  const [loading, setLoading] = useState(true);
  const [editing, setEditing] = useState<Day | null>(null);

  const [from, to, daysInMonth] = monthRange(ym);

  useEffect(() => {
    load();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [current?.id, ym]);

  async function load() {
    if (!current?.id) return;
    setLoading(true);
    try {
      const res = await api<{ days: Day[] }>(
        `/projects/${current.id}/attendance/days?from=${from}&to=${to}`, { projectId: current.id }
      );
      setDays(res.days);
    } finally {
      setLoading(false);
    }
  }

  const grid = useMemo(() => {
    const byPerson = new Map<string, { name: string; cells: Map<string, Day> }>();
    for (const d of days) {
      if (!byPerson.has(d.person_id)) byPerson.set(d.person_id, { name: d.person_name, cells: new Map() });
      byPerson.get(d.person_id)!.cells.set(d.work_date, d);
    }
    return [...byPerson.entries()].sort((a, b) => a[1].name.localeCompare(b[1].name, "tr"));
  }, [days]);

  async function approveMonth() {
    if (!current?.id) return;
    if (!window.confirm(`${ym} dönemini tümüyle onaylamak istiyor musunuz? Onaylanan günler bir daha düzeltilemez.`)) return;
    await api(`/projects/${current.id}/attendance/approve`, { method: "POST", projectId: current.id, body: { from, to } });
    load();
  }

  if (!current) return <div className="p-8 text-beton-400">Proje seçilmedi.</div>;

  return (
    <div>
      <div className="flex items-center gap-3 flex-wrap">
        <h1 className="font-display text-2xl font-extrabold text-white">PDKS Puantaj</h1>
        <input
          type="month" value={ym} onChange={(e) => setYm(e.target.value)}
          className="rounded-md bg-beton-900 border border-beton-700 px-3 py-1.5 text-sm text-beton-200"
        />
        <button
          onClick={approveMonth}
          className="ml-auto rounded-md bg-emniyet-500 hover:bg-emniyet-600 text-beton-950 font-semibold px-3 py-1.5 text-sm transition"
        >
          Dönemi Onayla
        </button>
      </div>
      <p className="mt-1 text-sm text-beton-400 max-w-2xl">
        Saatler QR/GPS giriş-çıkış kayıtlarından otomatik türetilir. Kırmızı işaretli günlerde en az bir kayıt
        şantiye sınırı dışından yapılmış — inceleyip gerekirse düzeltin.
      </p>

      {loading ? (
        <p className="px-4 py-8 text-center text-beton-400 text-sm">Yükleniyor…</p>
      ) : grid.length === 0 ? (
        <p className="px-4 py-8 text-center text-beton-400 text-sm">Bu dönemde puantaj kaydı yok.</p>
      ) : (
        <div className="mt-4 overflow-x-auto rounded-lg border border-beton-800">
          <table className="text-xs border-collapse w-full">
            <thead>
              <tr>
                <th className="sticky left-0 z-10 bg-beton-900 px-3 py-2 text-left text-beton-300 border-b border-beton-800 min-w-[140px]">
                  Personel
                </th>
                {Array.from({ length: daysInMonth }, (_, i) => i + 1).map((d) => (
                  <th key={d} className="px-2 py-2 text-center text-beton-400 border-b border-beton-800 font-mono">{d}</th>
                ))}
              </tr>
            </thead>
            <tbody>
              {grid.map(([personId, { name, cells }]) => (
                <tr key={personId} className="border-b border-beton-800/60">
                  <td className="sticky left-0 z-10 bg-beton-950 px-3 py-2 text-beton-200 font-medium whitespace-nowrap">{name}</td>
                  {Array.from({ length: daysInMonth }, (_, i) => i + 1).map((d) => {
                    const dateStr = `${ym}-${pad(d)}`;
                    const cell = cells.get(dateStr);
                    const hours = cell?.adjusted_hours ?? cell?.derived_hours ?? null;
                    return (
                      <td
                        key={d}
                        onClick={() => cell && setEditing(cell)}
                        className={`px-2 py-2 text-center font-mono tabular-nums ${cell ? "cursor-pointer hover:bg-beton-800/60" : ""} ${
                          cell?.has_flag ? "bg-red-500/10 text-red-300" : "text-beton-300"
                        } ${cell?.status === "approved" ? "opacity-60" : ""}`}
                        title={cell?.has_flag ? "Şantiye sınırı dışından kayıt var" : undefined}
                      >
                        {hours != null ? hours.toFixed(1) : cell ? "—" : ""}
                      </td>
                    );
                  })}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {editing && (
        <AdjustModal day={editing} onClose={() => setEditing(null)} onSaved={() => { setEditing(null); load(); }} />
      )}
    </div>
  );
}

function AdjustModal({ day, onClose, onSaved }: { day: Day; onClose: () => void; onSaved: () => void }) {
  const [hours, setHours] = useState(String(day.adjusted_hours ?? day.derived_hours ?? ""));
  const [reason, setReason] = useState("");
  const [err, setErr] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const locked = day.status === "approved";

  async function save() {
    setErr(null);
    const h = Number(hours);
    if (Number.isNaN(h) || h < 0 || h > 24) {
      setErr("0 ile 24 arasında bir saat girin.");
      return;
    }
    if (!reason.trim()) {
      setErr("Düzeltme gerekçesi zorunlu.");
      return;
    }
    setBusy(true);
    try {
      await api(`/attendance/days/${day.id}`, { method: "PATCH", body: { adjusted_hours: h, adjusted_reason: reason.trim() } });
      onSaved();
    } catch {
      setErr("Kaydedilemedi.");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="fixed inset-0 z-50 bg-black/60 flex items-center justify-center p-4" onClick={onClose}>
      <div className="w-full max-w-sm rounded-xl border border-beton-700 bg-beton-900 p-5" onClick={(e) => e.stopPropagation()}>
        <h2 className="text-lg font-bold text-beton-100">{day.person_name}</h2>
        <p className="text-xs text-beton-400 font-mono">{day.work_date}</p>
        {day.has_flag && (
          <p className="mt-2 text-xs text-red-400">⚠ Bu güne ait en az bir giriş/çıkış kaydı şantiye sınırı dışından yapılmış.</p>
        )}
        {locked ? (
          <p className="mt-3 text-sm text-beton-400">Bu dönem onaylanmış, düzeltilemez.</p>
        ) : (
          <>
            <p className="block mt-4 text-xs text-beton-400">
              Türetilen saat: {day.derived_hours != null ? day.derived_hours.toFixed(2) : "— (hesaplanamadı, giriş/çıkış sırası bozuk olabilir)"}
            </p>
            <label className="block mt-3 text-xs text-beton-400 mb-1">Düzeltilmiş saat</label>
            <input
              type="number" step="0.25" min={0} max={24} value={hours}
              onChange={(e) => setHours(e.target.value)}
              className="w-full rounded-md bg-beton-950 border border-beton-800 px-3 py-1.5 text-sm text-beton-100 outline-none focus:border-emniyet-500"
            />
            <label className="block mt-3 text-xs text-beton-400 mb-1">Düzeltme gerekçesi *</label>
            <textarea
              value={reason} onChange={(e) => setReason(e.target.value)} rows={2}
              className="w-full rounded-md bg-beton-950 border border-beton-800 px-3 py-1.5 text-sm text-beton-100 outline-none focus:border-emniyet-500"
            />
          </>
        )}
        {err && <p className="mt-2 text-sm text-red-400">{err}</p>}
        <div className="mt-4 flex justify-end gap-2">
          <button onClick={onClose} className="rounded-md border border-beton-700 text-beton-300 hover:bg-beton-800 px-3 py-1.5 text-sm transition">
            Kapat
          </button>
          {!locked && (
            <button
              onClick={save} disabled={busy}
              className="rounded-md bg-emniyet-500 hover:bg-emniyet-600 disabled:opacity-60 text-beton-950 font-semibold px-3 py-1.5 text-sm transition"
            >
              {busy ? "Kaydediliyor…" : "Kaydet"}
            </button>
          )}
        </div>
      </div>
    </div>
  );
}
