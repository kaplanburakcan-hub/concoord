import type { ReactNode } from "react";

// Hakediş özet bandı — hakediş raporundaki koyu lacivert üst bant: başlık,
// dört ana rakam ve ilerleme çubuğu. Renkler temadan bağımsız sabittir
// (rapor paleti): lacivert zemin, amber vurgu.
const trn = (v: number) => Math.round(v).toLocaleString("tr-TR");
const pct = (v: number) => (Math.round(v * 10) / 10).toLocaleString("tr-TR", { minimumFractionDigits: 1 });

export default function PaymentHero({
  title, subtitle, status, contract, cum, prev, thisGross, payable, canFin,
}: {
  title: string; subtitle?: string; status: ReactNode;
  contract?: number | null; cum?: number; prev?: number; thisGross?: number; payable?: number;
  canFin: boolean;
}) {
  const base = contract && contract > 0 ? contract : null;
  const prevPct = base && prev != null ? Math.max(0, (prev / base) * 100) : 0;
  const thisPct = base && thisGross != null ? Math.max(0, (thisGross / base) * 100) : 0;
  const restPct = Math.max(0, 100 - prevPct - thisPct);
  const kpis: { label: string; value: string; hl?: boolean }[] = [
    { label: "Sözleşme bedeli", value: base ? trn(base) : "—" },
    { label: "Kümülatif", value: cum != null ? trn(cum) : "—" },
    { label: "Bu dönem brüt", value: thisGross != null ? trn(thisGross) : "—" },
    { label: "Ödenecek", value: payable != null ? trn(payable) : "—", hl: true },
  ];

  return (
    <section className="rounded-lg p-4 text-white-solid" style={{ background: "#0b2545" }}>
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="min-w-0">
          <div className="font-display text-base font-bold truncate">{title}</div>
          {subtitle && <div className="mt-0.5 text-[11.5px]" style={{ color: "#c9d4e3" }}>{subtitle}</div>}
        </div>
        {status}
      </div>

      {canFin ? (
        <>
          <div className="mt-3 grid grid-cols-2 md:grid-cols-4 gap-px rounded overflow-hidden" style={{ background: "rgba(255,255,255,.16)" }}>
            {kpis.map((k) => (
              <div key={k.label} className="px-3 py-2 min-w-0"
                style={k.hl ? { background: "#e0b400", color: "#0b2545" } : { background: "#1d3d68" }}>
                <div className="text-[9px] uppercase tracking-wider" style={k.hl ? { opacity: .85 } : { color: "#c9d4e3" }}>
                  {k.label}
                </div>
                <div className="font-display text-sm font-bold tabular-nums truncate">{k.value}</div>
              </div>
            ))}
          </div>

          {base && (
            <div className="mt-3">
              <div className="flex h-2.5 rounded overflow-hidden" style={{ background: "#f3f6fa" }}
                role="img" aria-label={`Önceki dönemler %${pct(prevPct)}, bu dönem %${pct(thisPct)}, kalan %${pct(restPct)}`}>
                <i style={{ width: `${prevPct}%`, background: "#1d3d68" }} />
                <i style={{ width: `${thisPct}%`, background: "#e0b400" }} />
              </div>
              <div className="mt-1.5 flex flex-wrap gap-x-4 gap-y-1 text-[11px]" style={{ color: "#c9d4e3" }}>
                <span className="inline-flex items-center gap-1.5">
                  <em className="inline-block w-2 h-2 rounded-sm" style={{ background: "#1d3d68", border: "1px solid rgba(255,255,255,.3)" }} />
                  Önceki dönemler %{pct(prevPct)}
                </span>
                <span className="inline-flex items-center gap-1.5">
                  <em className="inline-block w-2 h-2 rounded-sm" style={{ background: "#e0b400" }} />
                  Bu dönem %{pct(thisPct)}
                </span>
                <span>Kalan %{pct(restPct)}</span>
              </div>
            </div>
          )}
        </>
      ) : (
        <p className="mt-3 text-xs" style={{ color: "#c9d4e3" }}>Tutarları görmek için finansal görüntüleme yetkisi gerekir.</p>
      )}
    </section>
  );
}
