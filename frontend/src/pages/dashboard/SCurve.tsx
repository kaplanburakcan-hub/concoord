// S-eğrisi (PV/EV/AC) — bağımlılıksız satır içi SVG çizim.
// Faz 10 arayüz yenileme: renkler token sınıfları + currentColor ile verilir,
// böylece light/dark ve palet değişimine otomatik uyum sağlar. EV altına düz
// %opaklık dolgu (kazanılan değer vurgusu).

export type SCurvePoint = { month: string; pv: number; ev: number; ac: number };

const SERIES: { key: keyof Omit<SCurvePoint, "month">; label: string; cls: string; dash?: boolean }[] = [
  { key: "pv", label: "PV · planlanan", cls: "text-beton-500", dash: true },
  { key: "ev", label: "EV · kazanılan", cls: "text-emniyet-500" },
  { key: "ac", label: "AC · gerçekleşen", cls: "text-red-400" },
];

export default function SCurve({ points, currency }: { points: SCurvePoint[]; currency: string }) {
  if (!points || points.length === 0) {
    return <p className="text-sm text-beton-400">S-eğrisi için veri yok.</p>;
  }
  // Geniş viewBox oranı: tam genişlikte render edildiğinde grafik dikeyde
  // makul yükseklikte kalır (720x260 oranı ~390px'e uzuyordu).
  const W = 1100;
  const H = 260;
  const PAD = { l: 52, r: 14, t: 14, b: 28 };
  const max = Math.max(1, ...points.flatMap((p) => [p.pv, p.ev, p.ac]));
  const x = (i: number) =>
    PAD.l + (points.length === 1 ? 0 : (i / (points.length - 1)) * (W - PAD.l - PAD.r));
  const y = (v: number) => H - PAD.b - (v / max) * (H - PAD.t - PAD.b);

  const path = (key: "pv" | "ev" | "ac") =>
    points.map((p, i) => `${i === 0 ? "M" : "L"}${x(i).toFixed(1)},${y(p[key]).toFixed(1)}`).join(" ");
  const area = (key: "pv" | "ev" | "ac") =>
    `M${x(0).toFixed(1)},${y(0).toFixed(1)} ` +
    points.map((p, i) => `L${x(i).toFixed(1)},${y(p[key]).toFixed(1)}`).join(" ") +
    ` L${x(points.length - 1).toFixed(1)},${y(0).toFixed(1)} Z`;

  const fmt = (v: number) =>
    v >= 1_000_000 ? (v / 1_000_000).toFixed(1) + "M" : v >= 1_000 ? (v / 1_000).toFixed(0) + "K" : v.toFixed(0);

  const ticks = [0, max / 2, max];
  const step = Math.max(1, Math.ceil(points.length / 8));

  return (
    <div>
      <svg viewBox={`0 0 ${W} ${H}`} className="w-full" role="img" aria-label="S-eğrisi">
        {ticks.map((t, i) => (
          <g key={i} className="text-beton-800">
            <line x1={PAD.l} x2={W - PAD.r} y1={y(t)} y2={y(t)} stroke="currentColor" strokeWidth={1} />
          </g>
        ))}
        {ticks.map((t, i) => (
          <text key={"tl" + i} x={PAD.l - 8} y={y(t) + 3.5} textAnchor="end" fontSize={10} className="fill-beton-500">
            {fmt(t)}
          </text>
        ))}
        {points.map((p, i) =>
          i % step === 0 || i === points.length - 1 ? (
            <text key={p.month} x={x(i)} y={H - 8} textAnchor="middle" fontSize={10} className="fill-beton-500">
              {p.month}
            </text>
          ) : null
        )}

        {/* EV altı düz dolgu (kazanılan değer vurgusu) */}
        <path d={area("ev")} className="text-emniyet-500" fill="currentColor" fillOpacity={0.12} stroke="none" />

        {SERIES.map((s) => (
          <path
            key={s.key}
            d={path(s.key)}
            className={s.cls}
            fill="none"
            stroke="currentColor"
            strokeWidth={s.key === "ev" ? 2.8 : 2.2}
            strokeLinecap="round"
            strokeLinejoin="round"
            strokeDasharray={s.dash ? "5 4" : undefined}
          />
        ))}

        {points.map((p, i) => (
          <circle key={"ev" + p.month} cx={x(i)} cy={y(p.ev)} r={2.8} className="text-emniyet-500" fill="currentColor">
            <title>{`${p.month} · EV: ${p.ev.toLocaleString("tr-TR")} ${currency}`}</title>
          </circle>
        ))}
      </svg>

      <div className="mt-2 flex gap-5 text-xs flex-wrap">
        {SERIES.map((s) => (
          <span key={s.key} className="flex items-center gap-2 text-beton-400">
            <span className={"inline-block w-3.5 h-0.5 rounded " + s.cls} style={{ background: "currentColor" }} />
            {s.label}
          </span>
        ))}
      </div>
    </div>
  );
}
