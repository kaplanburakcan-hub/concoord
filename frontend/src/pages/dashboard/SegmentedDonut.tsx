// Çok kategorili donut grafik — RadialRing.tsx'in tekli-metrik versiyonunun
// kategorili karşılığı. Düz, sürekli renkli SVG yaylar (kullanıcının
// paylaştığı referans görsele göre) — tick/kesikli görünüm yerine her
// kategori kendi payına orantılı, aralarında ince boşluklu tek parça bir
// yay olarak çizilir. Panel her zaman koyu render olduğundan halka
// arka planı --panel-* sabit tokenlara bağlıdır (temadan bağımsız).

export type ColorStop = { t: number; hex: string };
export type DonutSegment = { label: string; value: number; colorStops: ColorStop[]; swatch: string };

const GAP_DEG = 6;

// Büyük para tutarlarının (ör. ₺13.895.000) halkanın dar iç boşluğuna
// sığması için kısaltılmış gösterim (₺13,9M) — legend'deki tam değerler
// (formatValue) bundan etkilenmez, yalnızca halka merkezindeki toplam.
function compactCenter(full: string, raw: number): string {
  const symbolMatch = full.match(/^[^\d\-]+/);
  const symbol = symbolMatch ? symbolMatch[0] : "";
  const abs = Math.abs(raw);
  let num: string;
  if (abs >= 1_000_000_000) num = (raw / 1_000_000_000).toLocaleString("tr-TR", { maximumFractionDigits: 1 }) + "B";
  else if (abs >= 1_000_000) num = (raw / 1_000_000).toLocaleString("tr-TR", { maximumFractionDigits: 1 }) + "M";
  else if (abs >= 10_000) num = (raw / 1_000).toLocaleString("tr-TR", { maximumFractionDigits: 1 }) + "K";
  else return full;
  return symbol + num;
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
  const centerText = total === 0 ? "—" : compactCenter(fmt(total), total);

  const strokeWidth = size * 0.15;
  const r = (size - strokeWidth) / 2;
  const cx = size / 2;
  const cy = size / 2;
  const circumference = 2 * Math.PI * r;

  let cumulativeDeg = 0;
  const arcs = total > 0
    ? segments
        .filter((s) => s.value > 0)
        .map((s, i) => {
          const frac = Math.max(0, s.value) / total;
          const sweepDeg = Math.max(0, frac * 360 - GAP_DEG);
          const startDeg = cumulativeDeg;
          cumulativeDeg += frac * 360;
          const len = (sweepDeg / 360) * circumference;
          const offset = -((startDeg / 360) * circumference);
          return { key: i, color: s.swatch, dash: `${len} ${Math.max(0, circumference - len)}`, offset };
        })
    : [];

  return (
    <div className="flex items-center gap-5">
      <div className="relative shrink-0" style={{ width: size, height: size }}>
        <svg width={size} height={size} viewBox={`0 0 ${size} ${size}`}>
          <circle cx={cx} cy={cy} r={r} fill="none" stroke="rgb(var(--panel-hairline))" strokeWidth={strokeWidth} />
          {arcs.map((a) => (
            <circle
              key={a.key}
              cx={cx}
              cy={cy}
              r={r}
              fill="none"
              stroke={a.color}
              strokeWidth={strokeWidth}
              strokeLinecap="butt"
              strokeDasharray={a.dash}
              strokeDashoffset={a.offset}
              transform={`rotate(-90 ${cx} ${cy})`}
            />
          ))}
        </svg>
        <div className="absolute inset-0 flex flex-col items-center justify-center">
          <span
            className="font-display font-extrabold leading-none tabular-nums"
            style={{ fontSize: size * 0.155, color: "rgb(var(--panel-ink))" }}
          >
            {centerText}
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
