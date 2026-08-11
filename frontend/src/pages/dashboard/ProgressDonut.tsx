// Tek değerli halka (donut) grafik — SCurve.tsx ile aynı desen: bağımlılıksız,
// satır içi SVG, currentColor/CSS custom property tabanlı renkler (tema
// değişiminde otomatik uyum sağlar).

export default function ProgressDonut({
  pct,
  color = "var(--accent)",
}: {
  pct: number;
  color?: string;
}) {
  const size = 128;
  const stroke = 14;
  const r = (size - stroke) / 2;
  const c = 2 * Math.PI * r;
  const clamped = Math.max(0, Math.min(100, pct));
  const dash = (clamped / 100) * c;

  return (
    <svg viewBox={`0 0 ${size} ${size}`} width={size} height={size} className="mx-auto">
      <circle
        cx={size / 2} cy={size / 2} r={r} fill="none"
        stroke="rgb(var(--beton-800))" strokeWidth={stroke}
      />
      <circle
        cx={size / 2} cy={size / 2} r={r} fill="none"
        stroke={color} strokeWidth={stroke} strokeLinecap="round"
        strokeDasharray={`${dash.toFixed(1)} ${(c - dash).toFixed(1)}`}
        transform={`rotate(-90 ${size / 2} ${size / 2})`}
        style={{ transition: "stroke-dasharray 0.4s ease" }}
      />
      <text
        x="50%" y="50%" textAnchor="middle" dominantBaseline="central"
        fill="currentColor"
        className="text-beton-100 font-display font-medium"
        style={{ fontSize: 22, fontVariantNumeric: "tabular-nums" }}
      >
        %{clamped.toFixed(1)}
      </text>
    </svg>
  );
}
