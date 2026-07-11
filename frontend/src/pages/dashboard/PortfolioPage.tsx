import { useEffect, useState } from "react";
import { useProjects } from "../../projects/ProjectContext";
import { api } from "../../api/client";

// Faz 9 — Portföy dashboard'u (Plan §3): projeler arası özet kartlar.
// SPI/CPI ve kümülatif tutar yalnızca ilgili projede finansal rapor izni
// olan kullanıcıya döner (backend süzer); taşeron kapsamlı projeler listelenmez.

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
  net_payable_cum?: number;
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
      <p className="font-mono text-xs tracking-[0.3em] text-emniyet-500 uppercase">
        Faz 9 · Portföy
      </p>
      <h1 className="font-display text-3xl font-extrabold text-white mt-2">
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

      <div className="mt-6 grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        {cards?.map((c) => (
          <button
            key={c.project_id}
            onClick={() => select(c.project_id)}
            className="text-left rounded-lg border border-beton-800 bg-beton-900 p-4 hover:border-emniyet-500 transition"
            title="Bu projeyi aktif yap"
          >
            <div className="flex items-baseline justify-between gap-2">
              <span className="font-mono text-xs text-emniyet-500">{c.code}</span>
              <span className="font-mono text-[11px] text-beton-500">{c.status}</span>
            </div>
            <h2 className="mt-1 text-sm font-semibold text-white">{c.name}</h2>

            <div className="mt-3 h-2 rounded bg-beton-800 overflow-hidden">
              <div
                className="h-full bg-emniyet-500"
                style={{ width: `${Math.min(100, c.progress_pct)}%` }}
              />
            </div>
            <p className="mt-1 font-mono text-[11px] text-beton-400">
              ilerleme %{c.progress_pct.toFixed(1)}
            </p>

            <div className="mt-3 grid grid-cols-2 gap-2 text-xs font-mono">
              <Metric label="SPI" value={fmtIdx(c.spi)} bad={(c.spi ?? 1) > 0 && (c.spi ?? 1) < 0.9} />
              <Metric label="CPI" value={fmtIdx(c.cpi)} bad={(c.cpi ?? 1) > 0 && (c.cpi ?? 1) < 0.9} />
              <Metric label="Açık İSG" value={String(c.open_findings)} bad={c.open_findings > 0} />
              <Metric label="Bekleyen onay" value={String(c.pending_approvals)} />
            </div>
            {c.net_payable_cum !== undefined && (
              <p className="mt-2 font-mono text-[11px] text-beton-400">
                Kümülatif gerçekleşen: {c.net_payable_cum.toLocaleString("tr-TR")} {c.currency}
              </p>
            )}
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
function Metric({ label, value, bad }: { label: string; value: string; bad?: boolean }) {
  return (
    <div className="rounded bg-beton-950 border border-beton-800 px-2 py-1.5">
      <span className="block text-[10px] uppercase tracking-wider text-beton-500">{label}</span>
      <span className={bad ? "text-red-400" : "text-beton-100"}>{value}</span>
    </div>
  );
}
