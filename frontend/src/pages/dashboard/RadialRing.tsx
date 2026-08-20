// Tek segmentli yüzde halkası — MultiDonut.tsx ile aynı desen (bağımlılıksız,
// satır içi SVG), tek bir % değeri için. İlerleme Göstergeleri kartında
// (Dashboard.tsx) Fiziksel/Zamansal/Parasal İlerleme için kullanılır.

export default function RadialRing({
  pct,
  color,
  size = 104,
}: {
  pct: number;
  color: string;
  size?: number;
}) {
  const stroke = Math.round(size * 0.11);
  const r = (size - stroke) / 2;
  const c = 2 * Math.PI * r;
  const clamped = Math.max(0, Math.min(100, pct));
  const dash = (clamped / 100) * c;

  return (
    <svg viewBox={`0 0 ${size} ${size}`} width={size} height={size} className="shrink-0">
      <circle cx={size / 2} cy={size / 2} r={r} fill="none" stroke="rgb(var(--beton-800))" strokeWidth={stroke} />
      <circle
        cx={size / 2} cy={size / 2} r={r} fill="none"
        stroke={color} strokeWidth={stroke} strokeLinecap="round"
        strokeDasharray={`${dash.toFixed(1)} ${(c - dash).toFixed(1)}`}
        transform={`rotate(-90 ${size / 2} ${size / 2})`}
      />
      <text
        x="50%" y="50%" textAnchor="middle" dominantBaseline="central"
        className="font-display font-bold fill-beton-100"
        style={{ fontSize: size * 0.185, fontVariantNumeric: "tabular-nums" }}
      >
        %{pct.toFixed(1)}
      </text>
    </svg>
  );
}
