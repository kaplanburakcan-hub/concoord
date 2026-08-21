// Segmentli/gradyanlı yüzde halkası — İlerleme Göstergeleri kartında
// (Dashboard.tsx) Fiziksel/Zamansal/Parasal İlerleme için kullanılır.
// Kullanıcının paylaştığı referans görsele göre: tek düz renkli yay yerine
// küçük yuvarlak segmentler + segmentler boyunca renk geçişi (colorStops).
// Tamamlanmamış segmentler beton-800 ile gösterilir (eski RadialRing'in
// arka plan halkasıyla aynı, iki temada da çalışır).

export type ColorStop = { t: number; hex: string };

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

export default function RadialRing({
  pct,
  colorStops,
  size = 132,
  empty,
}: {
  pct: number;
  colorStops: ColorStop[];
  size?: number;
  empty?: boolean;
}) {
  const radius = size * 0.44;
  const tickW = size * 0.03;
  const tickH = size * 0.098;
  const clamped = Math.max(0, Math.min(100, pct));
  const activeTicks = empty ? 0 : Math.round((clamped / 100) * TOTAL_TICKS);

  return (
    <div className="relative shrink-0" style={{ width: size, height: size }}>
      {Array.from({ length: TOTAL_TICKS }).map((_, i) => {
        const deg = (360 / TOTAL_TICKS) * i;
        const active = i < activeTicks;
        const t = TOTAL_TICKS > 1 ? i / (TOTAL_TICKS - 1) : 0;
        return (
          <div
            key={i}
            className="absolute left-1/2 top-1/2 rounded-full"
            style={{
              width: tickW,
              height: tickH,
              marginLeft: -tickW / 2,
              transformOrigin: "50% 0",
              transform: `rotate(${deg}deg) translateY(-${radius}px)`,
              background: active ? colorAt(colorStops, t) : "rgb(var(--panel-hairline))",
            }}
          />
        );
      })}
      <div className="absolute inset-0 flex items-center justify-center">
        <span
          className="font-display font-extrabold"
          style={{ fontSize: size * 0.167, fontVariantNumeric: "tabular-nums", color: "rgb(var(--panel-ink))" }}
        >
          {empty ? "—" : `%${pct.toFixed(1)}`}
        </span>
      </div>
    </div>
  );
}
