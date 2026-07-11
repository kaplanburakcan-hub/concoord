// S-eğrisi (PV/EV/AC) — bağımlılıksız satır içi SVG çizim.
// Kümülatif noktalar backend'den hazır gelir; burada yalnızca ölçekleme yapılır.

export type SCurvePoint = { month: string; pv: number; ev: number; ac: number };

const SERIES: { key: keyof Omit<SCurvePoint, "month">; label: string; color: string }[] = [
  { key: "pv", label: "PV (plan)", color: "#8a93a3" },
  { key: "ev", label: "EV (kazanılan)", color: "#f5b301" },
  { key: "ac", label: "AC (gerçekleşen)", color: "#e05d5d" },
];

export default function SCurve({ points, currency }: { points: SCurvePoint[]; currency: string }) {
  if (!points || points.length === 0) {
    return <p className="text-sm text-beton-400">S-eğrisi için veri yok.</p>;
  }
  const W = 640;
  const H = 220;
  const PAD = { l: 56, r: 10, t: 10, b: 26 };
  const max = Math.max(1, ...points.flatMap((p) => [p.pv, p.ev, p.ac]));
  const x = (i: number) =>
    PAD.l + (points.length === 1 ? 0 : (i / (points.length - 1)) * (W - PAD.l - PAD.r));
  const y = (v: number) => H - PAD.b - (v / max) * (H - PAD.t - PAD.b);

  const path = (key: "pv" | "ev" | "ac") =>
    points.map((p, i) => `${i === 0 ? "M" : "L"}${x(i).toFixed(1)},${y(p[key]).toFixed(1)}`).join(" ");

  const fmt = (v: number) =>
    v >= 1_000_000 ? (v / 1_000_000).toFixed(1) + "M" : v >= 1_000 ? (v / 1_000).toFixed(0) + "K" : v.toFixed(0);

  // Y ekseni: 0, ½, tam.
  const ticks = [0, max / 2, max];
  // X etiketleri: en fazla ~8 etiket.
  const step = Math.max(1, Math.ceil(points.length / 8));

  return (
    <div>
      <svg viewBox={`0 0 ${W} ${H}`} className="w-full" role="img" aria-label="S-eğrisi">
        {ticks.map((t, i) => (
          <g key={i}>
            <line x1={PAD.l} x2={W - PAD.r} y1={y(t)} y2={y(t)} stroke="#2a2e37" strokeWidth={1} />
            <text x={PAD.l - 6} y={y(t) + 3} textAnchor="end" fontSize={9} fill="#8a93a3">
              {fmt(t)}
            </text>
          </g>
        ))}
        {points.map((p, i) =>
          i % step === 0 || i === points.length - 1 ? (
            <text key={p.month} x={x(i)} y={H - 8} textAnchor="middle" fontSize={9} fill="#8a93a3">
              {p.month}
            </text>
          ) : null
        )}
        {SERIES.map((s) => (
          <path key={s.key} d={path(s.key)} fill="none" stroke={s.color} strokeWidth={2} />
        ))}
        {SERIES.map((s) =>
          points.map((p, i) => (
            <circle key={s.key + p.month} cx={x(i)} cy={y(p[s.key])} r={2.2} fill={s.color}>
              <title>{`${p.month} · ${s.label}: ${p[s.key].toLocaleString("tr-TR")} ${currency}`}</title>
            </circle>
          ))
        )}
      </svg>
      <div className="mt-1 flex gap-4 text-xs">
        {SERIES.map((s) => (
          <span key={s.key} className="flex items-center gap-1.5 text-beton-300">
            <span className="inline-block w-3 h-0.5" style={{ background: s.color }} />
            {s.label}
          </span>
        ))}
      </div>
    </div>
  );
}
