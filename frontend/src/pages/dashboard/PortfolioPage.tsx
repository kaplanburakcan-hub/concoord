import { useEffect, useState } from "react";
import { useProjects } from "../../projects/ProjectContext";
import { api } from "../../api/client";

// Faz 9 — Portföy dashboard'u (Plan §3): projeler arası özet kartlar.
// SPI/CPI ve kümülatif tutar yalnızca ilgili projede finansal rapor izni
// olan kullanıcıya döner (backend süzer); taşeron kapsamlı projeler listelenmez.
//
// İlerleme üç ayrı eksende gösterilir (tek bir "ilerleme %" hangi anlama
// geldiği belirsizdi — bkz. kullanıcı geri bildirimi):
//   Zamansal — bitişe kalan gün sayısı (künye tarihlerinden, herkese açık)
//   Fiziki   — EV/BAC, ekskavatör işaretçili turuncu bar (herkese açık)
//   Parasal  — AC/sözleşme bedeli, yeşil bar (yalnızca finansal izinle)

type Card = {
  project_id: string;
  code: string;
  name: string;
  status: string;
  currency: string;
  progress_pct: number;
  spi?: number;
  cpi?: number;
  open_findings: number;
  pending_approvals: number;
  start_date?: string;
  end_date?: string;
  elapsed_pct?: number;
  days_to_end?: number;
  parasal_pct?: number;
};

export default function PortfolioPage() {
  const { select } = useProjects();
  const [cards, setCards] = useState<Card[] | null>(null);
  const [err, setErr] = useState<string | null>(null);

  useEffect(() => {
    api<{ portfolio: Card[] }>("/portfolio")
      .then((r) => setCards(r.portfolio))
      .catch(() => setErr("Portföy verisi yüklenemedi."));
  }, []);

  return (
    <div>
      <p className="flex items-center gap-2 text-xs font-medium text-emniyet-500">
        <span className="inline-block w-1.5 h-1.5 rounded-full bg-emniyet-500" />
        Portföy
      </p>
      <h1 className="font-display text-3xl font-medium text-beton-100 mt-2 tracking-tight">
        Portföy Görünümü
      </h1>
      <p className="mt-1 text-sm text-beton-400">
        Projeler arası ilerleme, EVM endeksleri, açık İSG bulguları ve bekleyen onaylar.
      </p>

      {err && <p className="mt-4 text-sm text-red-400">{err}</p>}
      {!cards && !err && <p className="mt-4 text-sm text-beton-400">Yükleniyor…</p>}
      {cards && cards.length === 0 && (
        <p className="mt-4 text-sm text-beton-400">Görüntülenecek proje yok.</p>
      )}

      {/* shared icon defs */}
      <svg width="0" height="0" className="absolute">
        <symbol id="ico-excavator" viewBox="0 0 100 64">
          <g fill="currentColor">
            <rect x="8" y="46" width="60" height="10" rx="5" />
            <path d="M18 46 L18 30 Q18 24 24 24 L46 24 L54 34 L60 34 L60 46 Z" />
            <path d="M40 26 L64 10" stroke="currentColor" strokeWidth="6" strokeLinecap="round" fill="none" />
            <path d="M64 10 L83 21" stroke="currentColor" strokeWidth="6" strokeLinecap="round" fill="none" />
            <path d="M83 21 L94 18 L91 31 L80 29 Z" />
          </g>
        </symbol>
        <symbol id="ico-cash" viewBox="0 0 100 64">
          <g fill="currentColor">
            <rect x="8" y="12" width="84" height="12" rx="2" />
            <rect x="8" y="26" width="84" height="12" rx="2" />
            <rect x="8" y="40" width="84" height="12" rx="2" />
          </g>
          <rect x="38" y="8" width="16" height="48" rx="2" fill="rgb(var(--beton-900))" opacity=".9" />
        </symbol>
      </svg>

      <div className="mt-6 grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        {cards?.map((c) => (
          <button
            key={c.project_id}
            onClick={() => select(c.project_id)}
            className="text-left rounded-xl border border-beton-800 bg-beton-900 p-5 hover:border-emniyet-500 transition"
            title="Bu projeyi aktif yap"
          >
            <div className="flex items-baseline justify-between gap-2">
              <span className="font-mono text-xs text-emniyet-500">{c.code}</span>
              <span className="font-mono text-[11px] text-beton-500">{c.status}</span>
            </div>
            <h2 className="mt-1 text-sm font-semibold text-white">{c.name}</h2>

            {/* Zamansal İlerleme */}
            <div className="mt-4">
              <div className="flex items-baseline justify-between">
                <span className="text-[9.5px] font-bold uppercase tracking-wider text-beton-500">
                  Zamansal İlerleme
                </span>
                {c.days_to_end !== undefined ? (
                  <b className="font-mono text-xs font-bold text-red-600 dark:text-red-400">
                    {c.days_to_end < 0 ? `${-c.days_to_end} gün gecikti` : `Bitişe ${c.days_to_end} gün`}
                  </b>
                ) : (
                  <b className="font-mono text-xs text-beton-500">—</b>
                )}
              </div>
              {c.elapsed_pct !== undefined ? (
                <>
                  <div
                    className="relative mt-1.5 h-1.5 rounded-full bg-beton-800"
                    style={{
                      backgroundImage:
                        "repeating-linear-gradient(90deg, rgb(var(--beton-700)) 0 3px, transparent 3px 8px)",
                      backgroundPosition: "center",
                      backgroundSize: "100% 2px",
                      backgroundRepeat: "no-repeat",
                    }}
                  >
                    <div
                      className="h-full rounded-full bg-red-600 dark:bg-red-400"
                      style={{ width: `${Math.min(100, c.elapsed_pct)}%` }}
                    />
                  </div>
                  <div className="mt-1 flex justify-between font-mono text-[9.5px] text-beton-500">
                    <span>{fmtDate(c.start_date)}</span>
                    <span>{fmtDate(c.end_date)}</span>
                  </div>
                </>
              ) : (
                <p className="mt-1.5 text-[10.5px] text-beton-500">Proje tarihleri tanımlı değil.</p>
              )}
            </div>

            {/* Fiziki İlerleme */}
            <div className="mt-3.5">
              <div className="flex items-baseline justify-between">
                <span className="text-[9.5px] font-bold uppercase tracking-wider text-beton-500">
                  Fiziki İlerleme
                </span>
                <b className="font-mono text-xs font-bold text-beton-100">{c.progress_pct.toFixed(1)}%</b>
              </div>
              <div className="relative mt-5 h-6">
                <div className="absolute inset-x-0 bottom-0 h-1.5 rounded-full bg-beton-800 overflow-hidden">
                  <div
                    className="h-full rounded-full bg-orange-600 dark:bg-orange-400"
                    style={{ width: `${Math.min(100, c.progress_pct)}%` }}
                  />
                </div>
                <div
                  className="absolute bottom-2 h-4 w-5 -translate-x-1/2 text-orange-600 dark:text-orange-400"
                  style={{ left: `${Math.min(100, c.progress_pct)}%` }}
                >
                  <svg className="h-full w-full"><use href="#ico-excavator" /></svg>
                </div>
              </div>
            </div>

            {/* Parasal İlerleme */}
            <div className="mt-3.5">
              <div className="flex items-baseline justify-between">
                <span className="text-[9.5px] font-bold uppercase tracking-wider text-beton-500">
                  Parasal İlerleme
                </span>
                <b className="font-mono text-xs font-bold text-beton-100">
                  {c.parasal_pct !== undefined ? `${c.parasal_pct.toFixed(1)}%` : c.spi !== undefined ? "—" : "•••"}
                </b>
              </div>
              <div className="relative mt-5 h-6">
                <div className="absolute inset-x-0 bottom-0 h-1.5 rounded-full bg-beton-800 overflow-hidden">
                  {c.parasal_pct !== undefined && (
                    <div
                      className="h-full rounded-full bg-green-600 dark:bg-green-400"
                      style={{ width: `${Math.min(100, c.parasal_pct)}%` }}
                    />
                  )}
                </div>
                {c.parasal_pct !== undefined && (
                  <div
                    className="absolute bottom-2 h-4 w-5 -translate-x-1/2 text-green-600 dark:text-green-400"
                    style={{ left: `${Math.min(100, c.parasal_pct)}%` }}
                  >
                    <svg className="h-full w-full"><use href="#ico-cash" /></svg>
                  </div>
                )}
              </div>
              {c.parasal_pct === undefined && c.spi !== undefined && (
                <p className="mt-1 text-[10.5px] text-beton-500">Ana sözleşme bedeli girilmemiş.</p>
              )}
            </div>

            <div className="mt-4 grid grid-cols-2 gap-2 text-xs font-mono">
              <Metric label="SPI" value={fmtIdx(c.spi)} bad={(c.spi ?? 1) > 0 && (c.spi ?? 1) < 0.9} />
              <Metric label="CPI" value={fmtIdx(c.cpi)} bad={(c.cpi ?? 1) > 0 && (c.cpi ?? 1) < 0.9} />
              <Metric label="Açık İSG" value={String(c.open_findings)} bad={c.open_findings > 0} />
              <Metric label="Bekleyen onay" value={String(c.pending_approvals)} />
            </div>
          </button>
        ))}
      </div>
    </div>
  );
}

function fmtIdx(v?: number) {
  if (v === undefined) return "•••"; // finansal izin yok
  if (v === 0) return "—"; // tanımsız
  return v.toFixed(3);
}
function fmtDate(s?: string) {
  if (!s) return "—";
  return new Date(s).toLocaleDateString("tr-TR", { day: "2-digit", month: "2-digit", year: "numeric" });
}
function Metric({ label, value, bad }: { label: string; value: string; bad?: boolean }) {
  return (
    <div className="rounded bg-beton-950 border border-beton-800 px-2 py-1.5">
      <span className="block text-[10px] uppercase tracking-wider text-beton-500">{label}</span>
      <span className={bad ? "text-red-400" : "text-beton-100"}>{value}</span>
    </div>
  );
}
