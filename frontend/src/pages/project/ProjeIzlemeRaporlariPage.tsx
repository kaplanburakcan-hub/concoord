import { useState } from "react";
import { useProjects } from "../../projects/ProjectContext";

// Proje İzleme Raporları — finansal ve operasyonel izleme raporları.
// Nakit Akışı, Maliyet Takip, Malzeme/Stok varsayılan olarak gelir;
// kullanıcı özel rapor türleri ekleyebilir.

type ReportCard = {
  id: string;
  label: string;
  desc: string;
  color: string;
  default: boolean;
  custom?: boolean;
};

const DEFAULT_REPORTS: ReportCard[] = [
  {
    id: "cashflow",
    label: "Nakit Akış Raporu",
    desc: "Tahsilat, ödeme ve nakit pozisyonunun dönemsel görünümü. Hakediş ve avans ödemeleriyle ilişkilendirilir.",
    color: "border-emniyet-500/40 bg-emniyet-500/5",
    default: true,
  },
  {
    id: "cost",
    label: "Maliyet Takip Raporu",
    desc: "Bütçe vs gerçekleşen maliyet karşılaştırması. EVM verileriyle (AC/EV/CPI) desteklenir.",
    color: "border-green-500/40 bg-green-500/5",
    default: true,
  },
  {
    id: "material",
    label: "Malzeme ve Stok Takip Raporu",
    desc: "Depo giriş/çıkışları, mevcut stok durumu ve bekleyen siparişlerin özeti.",
    color: "border-amber-500/40 bg-amber-500/5",
    default: true,
  },
];

export default function ProjeIzlemeRaporlariPage() {
  const { current } = useProjects();
  const [custom, setCustom] = useState<ReportCard[]>([]);
  const [adding, setAdding] = useState(false);
  const [newLabel, setNewLabel] = useState("");
  const [newDesc, setNewDesc] = useState("");

  if (!current) {
    return <p className="text-sm text-beton-400">Önce bir proje seçin.</p>;
  }

  const COLORS = [
    "border-violet-500/40 bg-violet-500/5",
    "border-sky-500/40 bg-sky-500/5",
    "border-rose-500/40 bg-rose-500/5",
    "border-teal-500/40 bg-teal-500/5",
  ];

  function addCustom() {
    if (!newLabel.trim()) return;
    setCustom([...custom, {
      id: "custom-" + Date.now(),
      label: newLabel.trim(),
      desc: newDesc.trim() || "Özel izleme raporu.",
      color: COLORS[custom.length % COLORS.length],
      default: false,
      custom: true,
    }]);
    setNewLabel("");
    setNewDesc("");
    setAdding(false);
  }

  function removeCustom(id: string) {
    setCustom(custom.filter((c) => c.id !== id));
  }

  const allReports = [...DEFAULT_REPORTS, ...custom];

  return (
    <div className="space-y-6">
      <div>
        <p className="flex items-center gap-2 text-xs font-medium text-emniyet-500">
          <span className="inline-block w-1.5 h-1.5 rounded-full bg-emniyet-500" />
          Proje izleme
        </p>
        <h1 className="font-display text-2xl font-medium text-beton-100 mt-1 tracking-tight">
          Proje İzleme Raporları
        </h1>
        <p className="text-sm text-beton-400 mt-1">
          Finansal ve operasyonel izleme raporları. Varsayılan raporlara ek olarak
          projeye özel rapor türleri tanımlayabilirsiniz.
        </p>
      </div>

      {/* Varsayılan + özel raporlar */}
      <div className="grid gap-3 sm:grid-cols-3">
        {allReports.map((r) => (
          <div key={r.id}
            className={"relative rounded-xl border p-4 " + r.color +
              (r.custom ? " cursor-default" : " cursor-pointer hover:brightness-110 transition")}>
            {r.custom && (
              <button
                onClick={() => removeCustom(r.id)}
                className="absolute top-3 right-3 text-beton-500 hover:text-red-400 text-xs"
                title="Kaldır">✕</button>
            )}
            <p className="font-medium text-beton-100 text-sm pr-4">{r.label}</p>
            <p className="mt-1 text-xs text-beton-400 leading-relaxed">{r.desc}</p>
            <span className="mt-3 inline-flex items-center gap-1 text-[10px] text-beton-500">
              {r.default ? "● Varsayılan" : "● Özel"}
            </span>
          </div>
        ))}

        {/* Yeni rapor ekle kartı */}
        {!adding ? (
          <button
            onClick={() => setAdding(true)}
            className="rounded-xl border border-dashed border-beton-700 p-4 text-beton-500
                       hover:border-emniyet-500 hover:text-emniyet-500 transition flex flex-col
                       items-center justify-center gap-2 min-h-[120px]">
            <span className="text-2xl">+</span>
            <span className="text-xs">Rapor türü ekle</span>
          </button>
        ) : (
          <div className="rounded-xl border border-emniyet-500/40 bg-emniyet-500/5 p-4 space-y-2">
            <input
              autoFocus
              value={newLabel}
              onChange={(e) => setNewLabel(e.target.value)}
              placeholder="Rapor adı"
              className="w-full rounded-md bg-beton-950 border border-beton-800 px-2.5 py-1.5
                         text-sm text-beton-100 outline-none focus:border-emniyet-500"
            />
            <textarea
              value={newDesc}
              onChange={(e) => setNewDesc(e.target.value)}
              placeholder="Açıklama (opsiyonel)"
              rows={2}
              className="w-full rounded-md bg-beton-950 border border-beton-800 px-2.5 py-1.5
                         text-xs text-beton-100 outline-none focus:border-emniyet-500 resize-none"
            />
            <div className="flex gap-2">
              <button onClick={addCustom}
                disabled={!newLabel.trim()}
                className="rounded-md bg-emniyet-500 hover:bg-emniyet-600 disabled:opacity-50
                           text-beton-950 text-xs font-medium px-3 py-1.5">
                Ekle
              </button>
              <button onClick={() => { setAdding(false); setNewLabel(""); setNewDesc(""); }}
                className="rounded-md border border-beton-700 text-beton-300 text-xs px-3 py-1.5">
                Vazgeç
              </button>
            </div>
          </div>
        )}
      </div>

      <p className="text-xs text-beton-500">
        Özel rapor türleri bu oturumda görünür. İleride sunucu tarafında kayıt özelliği eklenecek.
      </p>
    </div>
  );
}
