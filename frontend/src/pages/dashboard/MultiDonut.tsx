// Çok segmentli halka grafik — ProgressDonut.tsx ile aynı desen: bağımlılıksız,
// satır içi SVG. Maliyet Durumu widget'ı (Dashboard v2) için — segment
// sayısı/renkleri/boyutu çağırana bağlı.

export type DonutSegment = { label: string; value: number; color: string };

export default function MultiDonut({
  segments,
  formatValue,
  size = 140,
  layout = "stack",
}: {
  segments: DonutSegment[];
  formatValue?: (v: number) => string;
  size?: number;
  layout?: "stack" | "row";
}) {
  const stroke = Math.max(8, Math.round(size * 0.13));
  const r = (size - stroke) / 2;
  const c = 2 * Math.PI * r;
  const total = segments.reduce((s, seg) => s + Math.max(0, seg.value), 0);
  const fmt = formatValue ?? ((v: number) => v.toLocaleString("tr-TR"));

  let offset = 0;
  const arcs = segments.map((seg) => {
    const value = Math.max(0, seg.value);
    const frac = total > 0 ? value / total : 0;
    const dash = frac * c;
    const arc = { ...seg, dash, gapStart: offset };
    offset += dash;
    return arc;
  });

  const svg = (
    <svg viewBox={`0 0 ${size} ${size}`} width={size} height={size} className={layout === "stack" ? "mx-auto block shrink-0" : "shrink-0"}>
      {total === 0 ? (
        <circle cx={size / 2} cy={size / 2} r={r} fill="none" stroke="rgb(var(--beton-800))" strokeWidth={stroke} />
      ) : (
        arcs.map((a, i) => (
          <circle
            key={i}
            cx={size / 2} cy={size / 2} r={r} fill="none"
            stroke={a.color} strokeWidth={stroke}
            strokeDasharray={`${a.dash.toFixed(1)} ${(c - a.dash).toFixed(1)}`}
            strokeDashoffset={-a.gapStart}
            transform={`rotate(-90 ${size / 2} ${size / 2})`}
          />
        ))
      )}
    </svg>
  );

  const legend = (
    <div className={"text-xs " + (layout === "row" ? "space-y-1 min-w-0 flex-1" : "mt-3 space-y-1.5")}>
      {segments.map((s, i) => (
        <div key={i} className="flex items-center gap-2 min-w-0">
          <span className="inline-block w-2 h-2 rounded-full shrink-0" style={{ background: s.color }} />
          <span className="text-beton-300 truncate">{s.label}</span>
          <span className="ml-auto pl-2 font-mono text-beton-100 shrink-0" style={{ fontVariantNumeric: "tabular-nums" }}>
            {fmt(s.value)}
          </span>
        </div>
      ))}
      {total === 0 && <p className="text-beton-500">Henüz veri yok.</p>}
    </div>
  );

  if (layout === "row") {
    return <div className="flex items-center gap-3">{svg}{legend}</div>;
  }
  return <div>{svg}{legend}</div>;
}
