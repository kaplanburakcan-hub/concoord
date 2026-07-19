// S-eğrisi (PV/EV/AC) — bağımlılıksız satır içi SVG çizim.
//
// Faz 10 arayüz yenileme: renkler token sınıfları + currentColor ile verilir,
// böylece light/dark ve palet değişimine otomatik uyum sağlar.
//
// ÖNEMLİ (gerçekleşen çizgilerin bitişi): backend kümülatif EV/AC değerlerini
// proje takviminin TÜM aylarına taşır (veri olmayan gelecek aylar dahil). Bunu
// olduğu gibi çizmek yanıltıcıdır — proje duraklamış gibi görünür. Bu yüzden
// EV/AC yalnızca `asOf` ayına kadar çizilir; PV (plan) sona kadar devam eder.

export type SCurvePoint = { month: string; pv: number; ev: number; ac: number };

export default function SCurve({
  points,
  currency,
  asOf,
}: {
  points: SCurvePoint[];
  currency: string;
  asOf?: string;
}) {
  if (!points || points.length === 0) {
    return <p className="text-sm text-beton-400">S-eğrisi için veri yok.</p>;
  }

  // Geniş viewBox oranı: tam genişlikte render edildiğinde grafik dikeyde
  // makul yükseklikte kalır.
  const W = 1100;
  const H = 260;
  const PAD = { l: 62, r: 14, t: 14, b: 28 };

  // Gerçekleşen serilerin son anlamlı indeksi: asOf ayı (yoksa son nokta).
  const asOfIdx = (() => {
    if (!asOf) return points.length - 1;
    const i = points.findIndex((p) => p.month === asOf);
    return i >= 0 ? i : points.length - 1;
  })();
  const actual = points.slice(0, asOfIdx + 1);

  const max = Math.max(1, ...points.map((p) => p.pv), ...actual.flatMap((p) => [p.ev, p.ac]));
  const x = (i: number) =>
    PAD.l + (points.length === 1 ? 0 : (i / (points.length - 1)) * (W - PAD.l - PAD.r));
  const y = (v: number) => H - PAD.b - (v / max) * (H - PAD.t - PAD.b);

  const line = (arr: SCurvePoint[], key: "pv" | "ev" | "ac") =>
    arr.map((p, i) => `${i === 0 ? "M" : "L"}${x(i).toFixed(1)},${y(p[key]).toFixed(1)}`).join(" ");

  const areaEV =
    actual.length > 1
      ? `M${x(0).toFixed(1)},${y(0).toFixed(1)} ` +
        actual.map((p, i) => `L${x(i).toFixed(1)},${y(p.ev).toFixed(1)}`).join(" ") +
        ` L${x(actual.length - 1).toFixed(1)},${y(0).toFixed(1)} Z`
      : "";

  const fmt = (v: number) =>
    v >= 1_000_000 ? (v / 1_000_000).toFixed(1) + "M" : v >= 1_000 ? (v / 1_000).toFixed(0) + "K" : v.toFixed(0);

  const ticks = [0, max / 2, max];
  const step = Math.max(1, Math.ceil(points.length / 8));
  const lastActual = actual[actual.length - 1];

  return (
    <div>
      <svg viewBox={`0 0 ${W} ${H}`} className="w-full" role="img" aria-label="S-eğrisi">
        {ticks.map((t, i) => (
          <line
            key={"g" + i}
            x1={PAD.l}
            x2={W - PAD.r}
            y1={y(t)}
            y2={y(t)}
            className="text-beton-800"
            stroke="currentColor"
            strokeWidth={1}
          />
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

        {/* Bugün çizgisi — gerçekleşenin bittiği yer */}
        {asOfIdx < points.length - 1 && (
          <line
            x1={x(asOfIdx)}
            x2={x(asOfIdx)}
            y1={PAD.t}
            y2={H - PAD.b}
            className="text-beton-700"
            stroke="currentColor"
            strokeWidth={1}
            strokeDasharray="3 3"
          />
        )}

        {/* EV altı dolgu (yalnızca gerçekleşen aralık) */}
        {areaEV && (
          <path d={areaEV} className="text-emniyet-500" fill="currentColor" fillOpacity={0.12} stroke="none" />
        )}

        {/* PV — plan, sona kadar */}
        <path
          d={line(points, "pv")}
          className="text-beton-500"
          fill="none"
          stroke="currentColor"
          strokeWidth={2}
          strokeDasharray="5 4"
          strokeLinecap="round"
        />
        {/* AC — yalnızca gerçekleşen aralık */}
        <path
          d={line(actual, "ac")}
          className="text-red-400"
          fill="none"
          stroke="currentColor"
          strokeWidth={2.2}
          strokeLinecap="round"
          strokeLinejoin="round"
        />
        {/* EV — yalnızca gerçekleşen aralık */}
        <path
          d={line(actual, "ev")}
          className="text-emniyet-500"
          fill="none"
          stroke="currentColor"
          strokeWidth={2.8}
          strokeLinecap="round"
          strokeLinejoin="round"
        />

        {actual.map((p, i) => (
          <circle key={"ev" + p.month} cx={x(i)} cy={y(p.ev)} r={2.8} className="text-emniyet-500" fill="currentColor">
            <title>{`${p.month} · EV: ${p.ev.toLocaleString("tr-TR")} ${currency}`}</title>
          </circle>
        ))}
        {/* Son gerçekleşen nokta vurgusu */}
        {lastActual && (
          <circle
            cx={x(asOfIdx)}
            cy={y(lastActual.ev)}
            r={5}
            className="text-emniyet-500"
            fill="none"
            stroke="currentColor"
            strokeWidth={2}
          />
        )}
      </svg>

      <div className="mt-2 flex gap-5 text-xs flex-wrap text-beton-400">
        <span className="flex items-center gap-2">
          <span className="inline-block w-3.5 h-0.5 rounded text-beton-500" style={{ background: "currentColor" }} />
          PV · planlanan
        </span>
        <span className="flex items-center gap-2">
          <span className="inline-block w-3.5 h-0.5 rounded text-emniyet-500" style={{ background: "currentColor" }} />
          EV · kazanılan
        </span>
        <span className="flex items-center gap-2">
          <span className="inline-block w-3.5 h-0.5 rounded text-red-400" style={{ background: "currentColor" }} />
          AC · gerçekleşen
        </span>
        {asOf && <span className="text-beton-500">· gerçekleşen seriler {asOf} itibarıyladır</span>}
      </div>
    </div>
  );
}
