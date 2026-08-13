// Nakit Akış grafiği — SCurve.tsx deseniyle aynı: bağımsız satır içi SVG,
// dış kütüphane yok. Dönem başına giriş/çıkış çubukları + kümülatif bakiye
// çizgisi.

export type CashFlowPeriod = {
  label: string;
  start: string;
  end: string;
  in: number;
  out: number;
  net: number;
  cumulative_balance: number;
};

export default function CashFlowChart({ periods }: { periods: CashFlowPeriod[] }) {
  if (!periods || periods.length === 0) {
    return <p className="text-sm text-beton-400">Seçili aralıkta veri yok.</p>;
  }

  const W = 1100;
  const H = 300;
  const PAD = { l: 62, r: 62, t: 14, b: 34 };

  const maxBar = Math.max(1, ...periods.map((p) => Math.max(p.in, p.out)));
  const balances = periods.map((p) => p.cumulative_balance);
  const minBal = Math.min(0, ...balances);
  const maxBal = Math.max(1, ...balances);

  const n = periods.length;
  const slot = (W - PAD.l - PAD.r) / n;
  const barW = Math.min(18, slot * 0.32);

  const xCenter = (i: number) => PAD.l + slot * (i + 0.5);
  const yBar = (v: number) => H - PAD.b - (v / maxBar) * (H - PAD.t - PAD.b) * 0.62;
  const barBase = H - PAD.b;
  const yLine = (v: number) =>
    PAD.t + (H - PAD.t - PAD.b) * 0.62 - ((v - minBal) / (maxBal - minBal || 1)) * (H - PAD.t - PAD.b) * 0.62;

  const linePath = periods
    .map((p, i) => `${i === 0 ? "M" : "L"}${xCenter(i).toFixed(1)},${yLine(p.cumulative_balance).toFixed(1)}`)
    .join(" ");

  const fmt = (v: number) =>
    Math.abs(v) >= 1_000_000 ? (v / 1_000_000).toFixed(1) + "M"
    : Math.abs(v) >= 1_000 ? (v / 1_000).toFixed(0) + "K"
    : v.toFixed(0);

  const step = Math.max(1, Math.ceil(n / 10));

  return (
    <div>
      <svg viewBox={`0 0 ${W} ${H}`} className="w-full" role="img" aria-label="Nakit akış grafiği">
        {/* taban çizgisi */}
        <line x1={PAD.l} x2={W - PAD.r} y1={barBase} y2={barBase} className="text-beton-800" stroke="currentColor" strokeWidth={1} />

        {/* çubuklar: giriş (yeşil, yukarı) / çıkış (kırmızı, yukarı — ayrı renk) */}
        {periods.map((p, i) => (
          <g key={"bars" + i}>
            <rect
              x={xCenter(i) - barW - 2} y={yBar(p.in)} width={barW}
              height={Math.max(0, barBase - yBar(p.in))}
              className="text-emniyet-500" fill="currentColor" fillOpacity={0.85} rx={1.5}
            >
              <title>{`${p.label} · Giriş: ${p.in.toLocaleString("tr-TR")}`}</title>
            </rect>
            <rect
              x={xCenter(i) + 2} y={yBar(p.out)} width={barW}
              height={Math.max(0, barBase - yBar(p.out))}
              className="text-red-400" fill="currentColor" fillOpacity={0.85} rx={1.5}
            >
              <title>{`${p.label} · Çıkış: ${p.out.toLocaleString("tr-TR")}`}</title>
            </rect>
          </g>
        ))}

        {/* kümülatif bakiye çizgisi */}
        <path d={linePath} className="text-white" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round" />
        {periods.map((p, i) => (
          <circle key={"pt" + i} cx={xCenter(i)} cy={yLine(p.cumulative_balance)} r={2.6} className="text-white" fill="currentColor">
            <title>{`${p.label} · Kümülatif bakiye: ${p.cumulative_balance.toLocaleString("tr-TR")}`}</title>
          </circle>
        ))}

        {/* sıfır çizgisi (bakiye eksiye düşerse görünür olsun) */}
        {minBal < 0 && (
          <line x1={PAD.l} x2={W - PAD.r} y1={yLine(0)} y2={yLine(0)} className="text-beton-700" stroke="currentColor" strokeWidth={1} strokeDasharray="3 3" />
        )}

        {periods.map((p, i) =>
          i % step === 0 || i === n - 1 ? (
            <text key={"lbl" + i} x={xCenter(i)} y={H - 10} textAnchor="middle" fontSize={10} className="fill-beton-500">
              {p.label}
            </text>
          ) : null
        )}
        <text x={PAD.l - 8} y={barBase + 3.5} textAnchor="end" fontSize={10} className="fill-beton-500">0</text>
        <text x={W - PAD.r + 8} y={yLine(maxBal) + 3.5} textAnchor="start" fontSize={10} className="fill-beton-500">{fmt(maxBal)}</text>
        <text x={W - PAD.r + 8} y={yLine(minBal) + 3.5} textAnchor="start" fontSize={10} className="fill-beton-500">{fmt(minBal)}</text>
      </svg>

      <div className="mt-2 flex gap-5 text-xs flex-wrap text-beton-400">
        <span className="flex items-center gap-2">
          <span className="inline-block w-3 h-3 rounded-sm text-emniyet-500" style={{ background: "currentColor" }} />
          Giriş
        </span>
        <span className="flex items-center gap-2">
          <span className="inline-block w-3 h-3 rounded-sm text-red-400" style={{ background: "currentColor" }} />
          Çıkış
        </span>
        <span className="flex items-center gap-2">
          <span className="inline-block w-3.5 h-0.5 rounded text-white" style={{ background: "currentColor" }} />
          Kümülatif bakiye (sağ eksen)
        </span>
      </div>
    </div>
  );
}
