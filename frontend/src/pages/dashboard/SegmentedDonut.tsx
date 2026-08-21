// Çok kategorili segmentli/gradyanlı halka — RadialRing.tsx'in tekli-metrik
// versiyonunun kategorili karşılığı. Her kategori toplam içindeki payına
// orantılı bir tick bloğu alır, o blok kendi renk ailesinde küçük bir
// gradyana sahiptir (kullanıcının onayladığı referans görsele göre).
// Panel her zaman koyu render olduğundan renkler --panel-* sabit
// token'lara bağlıdır (temadan bağımsız).

export type ColorStop = { t: number; hex: string };
export type DonutSegment = { label: string; value: number; colorStops: ColorStop[]; swatch: string };

const TOTAL_TICKS = 36;

function hexToRgb(hex: string): [number, number, number] {
  const h = hex.replace("#", "");
  return [parseInt(h.slice(0, 2), 16), parseInt(h.slice(2, 4), 16), parseInt(h.slice(4, 6), 16)];
}
function lerp(a: number, b: number, t: number) {
  return a + (b - a) * t;
}
function colorAt(stops: ColorStop[], t: number): string {
  for (let i = 0; i < stops.length - 1; i++) {
    if (t >= stops[i].t && t <= stops[i + 1].t) {
      const localT = (t - stops[i].t) / (stops[i + 1].t - stops[i].t || 1);
      const c0 = hexToRgb(stops[i].hex);
      const c1 = hexToRgb(stops[i + 1].hex);
      return `rgb(${Math.round(lerp(c0[0], c1[0], localT))},${Math.round(lerp(c0[1], c1[1], localT))},${Math.round(lerp(c0[2], c1[2], localT))})`;
    }
  }
  return stops[stops.length - 1].hex;
}

export default function SegmentedDonut({
  segments,
  formatValue,
  centerLabel = "TOPLAM",
  size = 100,
}: {
  segments: DonutSegment[];
  formatValue?: (v: number) => string;
  centerLabel?: string;
  size?: number;
}) {
  const total = segments.reduce((s, seg) => s + Math.max(0, seg.value), 0);
  const fmt = formatValue ?? ((v: number) => v.toLocaleString("tr-TR"));
  const radius = size * 0.36;
  const tickW = size * 0.028;
  const tickH = size * 0.11;

  const ticks: { deg: number; color: string }[] = [];
  if (total > 0) {
    let tickIdx = 0;
    for (const seg of segments) {
      const value = Math.max(0, seg.value);
      const catTicks = Math.round((value / total) * TOTAL_TICKS);
      for (let j = 0; j < catTicks && tickIdx < TOTAL_TICKS; j++, tickIdx++) {
        const localT = catTicks > 1 ? j / (catTicks - 1) : 0;
        ticks.push({ deg: (360 / TOTAL_TICKS) * tickIdx, color: colorAt(seg.colorStops, localT) });
      }
    }
  }

  return (
    <div className="flex items-center gap-5">
      <div className="relative shrink-0" style={{ width: size, height: size }}>
        {total === 0
          ? Array.from({ length: TOTAL_TICKS }).map((_, i) => (
              <div
                key={i}
                className="absolute left-1/2 top-1/2 rounded-full"
                style={{
                  width: tickW, height: tickH, marginLeft: -tickW / 2, transformOrigin: "50% 0",
                  transform: `rotate(${(360 / TOTAL_TICKS) * i}deg) translateY(-${radius}px)`,
                  background: "rgb(var(--panel-hairline))",
                }}
              />
            ))
          : ticks.map((tk, i) => (
              <div
                key={i}
                className="absolute left-1/2 top-1/2 rounded-full"
                style={{
                  width: tickW, height: tickH, marginLeft: -tickW / 2, transformOrigin: "50% 0",
                  transform: `rotate(${tk.deg}deg) translateY(-${radius}px)`,
                  background: tk.color,
                }}
              />
            ))}
        <div className="absolute inset-0 flex flex-col items-center justify-center">
          <span
            className="font-display font-extrabold leading-none"
            style={{ fontSize: size * 0.17, color: "rgb(var(--panel-ink))", fontVariantNumeric: "tabular-nums" }}
          >
            {total === 0 ? "—" : fmt(total)}
          </span>
          <span
            className="mt-1 uppercase tracking-wide"
            style={{ fontSize: size * 0.075, color: "rgb(var(--panel-ink3))", letterSpacing: ".05em" }}
          >
            {centerLabel}
          </span>
        </div>
      </div>
      <div className="flex-1 min-w-0 space-y-2 text-[11.5px]">
        {segments.map((s, i) => (
          <div key={i} className="flex items-center gap-2 min-w-0">
            <span className="inline-block w-2 h-2 rounded-sm shrink-0" style={{ background: s.swatch }} />
            <span className="truncate" style={{ color: "rgb(var(--panel-ink2))" }}>{s.label}</span>
            <span
              className="ml-auto pl-2 shrink-0 font-bold"
              style={{ color: "rgb(var(--panel-ink))", fontVariantNumeric: "tabular-nums" }}
            >
              {fmt(s.value)}
            </span>
          </div>
        ))}
        {total === 0 && <p style={{ color: "rgb(var(--panel-ink3))" }}>Henüz veri yok.</p>}
      </div>
    </div>
  );
}
