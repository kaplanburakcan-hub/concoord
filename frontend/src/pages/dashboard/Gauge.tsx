// Yarım daire ibre göstergesi — SPI/CPI için: 0–2 aralığı, 1 tam ortada
// (üstte). Kırmızı/amber/yeşil bantlar evrensel EVM eşiğini yansıtır
// (<0.9 geride/bütçe üstü, 0.9–1.1 planda, >1.1 önde/bütçe altı).

const BANDS = [
  { from: 0, to: 0.9, color: "#ef4444" },
  { from: 0.9, to: 1.1, color: "#f59e0b" },
  { from: 1.1, to: 2.0, color: "#22c55e" },
];

function toAngle(v: number): number {
  // değer-uzayı [0,2] -> açı-uzayı [-90,90] derece (0 solda, 1 üstte, 2 sağda)
  return -90 + (Math.max(0, Math.min(2, v)) / 2) * 180;
}

function tone(v: number): string {
  return v >= 1.1 ? "#22c55e" : v >= 0.9 ? "#f59e0b" : "#ef4444";
}

export default function Gauge({ value, size = 148 }: { value: number; size?: number }) {
  const w = size;
  const h = size * 0.62;
  const cx = w / 2;
  const cy = h - 8;
  const r = h - 24;
  const stroke = Math.round(size * 0.07);

  const polar = (deg: number, radius: number): [number, number] => {
    const rad = ((deg - 90) * Math.PI) / 180;
    return [cx + radius * Math.cos(rad), cy + radius * Math.sin(rad)];
  };
  const arcPath = (a0: number, a1: number, radius: number): string => {
    const [x0, y0] = polar(a0, radius);
    const [x1, y1] = polar(a1, radius);
    const large = a1 - a0 > 180 ? 1 : 0;
    return `M ${x0} ${y0} A ${radius} ${radius} 0 ${large} 1 ${x1} ${y1}`;
  };

  const needleAngle = toAngle(value);
  const [nx, ny] = polar(needleAngle, r - stroke / 2 - 2);

  return (
    <svg viewBox={`0 0 ${w} ${h + 6}`} width={w} height={h + 6} className="shrink-0">
      {BANDS.map((b) => (
        <path
          key={b.color}
          d={arcPath(toAngle(b.from), toAngle(b.to), r)}
          fill="none" stroke={b.color} strokeWidth={stroke} opacity={0.9}
        />
      ))}
      <line x1={cx} y1={cy} x2={nx} y2={ny} className="stroke-beton-100" strokeWidth={3} strokeLinecap="round" />
      <circle cx={cx} cy={cy} r={5} className="fill-beton-100" />
      <text x={polar(-90, r + 14)[0]} y={polar(-90, r + 14)[1]} textAnchor="middle" fontSize={10} className="fill-beton-500">0</text>
      <text x={cx} y={polar(0, r + 16)[1]} textAnchor="middle" fontSize={10} className="fill-beton-500">1</text>
      <text x={polar(90, r + 14)[0]} y={polar(90, r + 14)[1]} textAnchor="middle" fontSize={10} className="fill-beton-500">2</text>
      <text
        x={cx} y={h + 2} textAnchor="middle"
        className="font-display font-bold" fill={tone(value)}
        style={{ fontSize: size * 0.13, fontVariantNumeric: "tabular-nums" }}
      >
        {value.toFixed(3)}
      </text>
    </svg>
  );
}
