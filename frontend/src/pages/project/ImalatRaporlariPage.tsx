import { useState } from "react";
import { Link } from "react-router-dom";
import { useProjects } from "../../projects/ProjectContext";

// İmalat Raporları — Günlük, Haftalık, Aylık ve özel dönem raporları.
// Mevcut aylık rapor (MonthlyReportsPage) ayrı çalışmaya devam eder;
// bu sayfa raporlama modülünün ana girişidir.

type ReportType = {
  id: string;
  label: string;
  desc: string;
  path: string;
  color: string;
  default?: boolean;
};

const REPORT_TYPES: ReportType[] = [
  {
    id: "daily",
    label: "Günlük Rapor",
    desc: "Sahadan günlük imalat verisi, hava durumu, personel ve ekipman bilgisi.",
    path: "/saha-raporlari",
    color: "border-emniyet-500/40 bg-emniyet-500/5",
    default: true,
  },
  {
    id: "weekly",
    label: "Haftalık Rapor",
    desc: "Haftalık imalat özeti, plana göre ilerleme ve kritik kalemler.",
    path: "/saha-raporlari",
    color: "border-blue-500/40 bg-blue-500/5",
    default: true,
  },
  {
    id: "monthly",
    label: "Aylık Rapor",
    desc: "EVM (PV/EV/AC, SPI/CPI) + kesinleşen hakedişler + İSG + tedarik özeti. PDF olarak üretilir.",
    path: "/aylik-raporlar",
    color: "border-green-500/40 bg-green-500/5",
    default: true,
  },
  {
    id: "quarterly",
    label: "Çeyrek Yıl Raporu",
    desc: "3 aylık dönem özeti. İsteğe bağlı oluşturulur.",
    path: "/aylik-raporlar",
    color: "border-amber-500/40 bg-amber-500/5",
  },
  {
    id: "custom",
    label: "Özel Dönem Raporu",
    desc: "Başlangıç ve bitiş tarihi seçilerek oluşturulan dönem raporu.",
    path: "/aylik-raporlar",
    color: "border-violet-500/40 bg-violet-500/5",
  },
];

export default function ImalatRaporlariPage() {
  const { current } = useProjects();
  const [customStart, setCustomStart] = useState("");
  const [customEnd, setCustomEnd] = useState("");

  if (!current) {
    return <p className="text-sm text-beton-400">Önce bir proje seçin.</p>;
  }

  return (
    <div className="space-y-6">
      <div>
        <p className="flex items-center gap-2 text-xs font-medium text-emniyet-500">
          <span className="inline-block w-1.5 h-1.5 rounded-full bg-emniyet-500" />
          İmalat raporları
        </p>
        <h1 className="font-display text-2xl font-medium text-beton-100 mt-1 tracking-tight">
          {current.name}
        </h1>
        <p className="text-sm text-beton-400 mt-1">
          Günlük rapor girişi, haftalık özet ve aylık yönetim raporlarına buradan ulaşın.
          Çeyrek yıl ve özel dönem raporları isteğe bağlı oluşturulabilir.
        </p>
      </div>

      {/* Varsayılan rapor türleri */}
      <div>
        <p className="text-xs uppercase tracking-wider text-beton-500 mb-3">Varsayılan raporlar</p>
        <div className="grid gap-3 sm:grid-cols-3">
          {REPORT_TYPES.filter((r) => r.default).map((r) => (
            <Link
              key={r.id}
              to={r.path}
              className={"rounded-xl border p-4 transition hover:brightness-110 " + r.color}
            >
              <p className="font-medium text-beton-100 text-sm">{r.label}</p>
              <p className="mt-1 text-xs text-beton-400 leading-relaxed">{r.desc}</p>
            </Link>
          ))}
        </div>
      </div>

      {/* İsteğe bağlı raporlar */}
      <div>
        <p className="text-xs uppercase tracking-wider text-beton-500 mb-3">İsteğe bağlı raporlar</p>
        <div className="grid gap-3 sm:grid-cols-2">
          {REPORT_TYPES.filter((r) => !r.default).map((r) => (
            <Link
              key={r.id}
              to={r.id === "custom"
                ? (customStart && customEnd ? `${r.path}?start=${customStart}&end=${customEnd}` : r.path)
                : r.path}
              className={"rounded-xl border p-4 transition hover:brightness-110 " + r.color}
            >
              <p className="font-medium text-beton-100 text-sm">{r.label}</p>
              <p className="mt-1 text-xs text-beton-400 leading-relaxed">{r.desc}</p>
              {r.id === "custom" && (
                <div className="mt-3 flex gap-2" onClick={(e) => e.preventDefault()}>
                  <input
                    type="date"
                    value={customStart}
                    onChange={(e) => setCustomStart(e.target.value)}
                    className="flex-1 rounded-md bg-beton-950 border border-beton-800 px-2 py-1.5 text-xs text-beton-100 outline-none focus:border-emniyet-500"
                  />
                  <span className="text-beton-500 text-xs self-center">–</span>
                  <input
                    type="date"
                    value={customEnd}
                    onChange={(e) => setCustomEnd(e.target.value)}
                    className="flex-1 rounded-md bg-beton-950 border border-beton-800 px-2 py-1.5 text-xs text-beton-100 outline-none focus:border-emniyet-500"
                  />
                </div>
              )}
            </Link>
          ))}
        </div>
      </div>
    </div>
  );
}
