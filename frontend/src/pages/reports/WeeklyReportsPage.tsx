import { useCallback, useEffect, useRef, useState } from "react";
import { Link } from "react-router-dom";
import { api, apiDownload, RequestError } from "../../api/client";
import { useAuth } from "../../auth/AuthContext";
import { useProjects } from "../../projects/ProjectContext";

// ── Types ──────────────────────────────────────────────────────────────────

type Weekly = {
  id: string; week_no: number; period_start: string; period_end: string;
  status: "Pending" | "Ready" | "Failed"; error?: string;
  generated_by_name: string; has_pdf: boolean; created_at: string;
};

type DEntry = { ilerleme: string; plan: string };
type EkipRow = { tip: string; klv: number; kira: number };
type TaskRow = { firma: string; disiplin: string; alanlar: string };
type SevkRow = { tarih: string; malzeme: string };
type TalRow  = { termin: string; malzeme: string };
type KasaRow = { tarih: string; aciklama: string; kategori: string; tutar: string; makbuz: string };

type WeekContent = {
  insaat: DEntry; elektrik: DEntry; mekanik: DEntry;
  ekipman: EkipRow[]; taseronlar: TaskRow[];
  sevkler: SevkRow[]; talepler: TalRow[];
  kasa: KasaRow[];
};

const EMPTY: WeekContent = {
  insaat:   { ilerleme: "", plan: "" },
  elektrik: { ilerleme: "", plan: "" },
  mekanik:  { ilerleme: "", plan: "" },
  ekipman: [], taseronlar: [], sevkler: [], talepler: [], kasa: [],
};

// ── Helpers ────────────────────────────────────────────────────────────────

const W_STATUS_LABEL: Record<string, string> = {
  Pending: "Üretiliyor…", Ready: "Hazır", Failed: "Başarısız",
};
const W_STATUS_STYLE: Record<string, string> = {
  Pending: "bg-blue-500/15 text-blue-300 border-blue-500/40",
  Ready:   "bg-green-500/15 text-green-300 border-green-500/40",
  Failed:  "bg-red-500/15 text-red-300 border-red-500/40",
};

const DAY_NAMES = ["Pzt", "Sal", "Çar", "Per", "Cum", "Cts", "Paz"];

function mondayOf(iso: string): string {
  const d = new Date(iso + "T00:00:00");
  const wd = d.getDay() === 0 ? 7 : d.getDay();
  d.setDate(d.getDate() - (wd - 1));
  return d.toISOString().slice(0, 10);
}

function addDays(iso: string, n: number): string {
  const d = new Date(iso + "T00:00:00");
  d.setDate(d.getDate() + n);
  return d.toISOString().slice(0, 10);
}

function fmt(iso: string) {
  const [y, m, d] = iso.split("-");
  return `${d}.${m}.${y}`;
}

function fmtShort(iso: string) {
  const [, m, d] = iso.split("-");
  return `${d}.${m}`;
}

function storageKey(pid: string, weekStart: string) {
  return `ipks.weekly.${pid}.${weekStart}`;
}

function loadContent(pid: string, weekStart: string): WeekContent {
  try {
    const raw = localStorage.getItem(storageKey(pid, weekStart));
    if (!raw) return EMPTY;
    return { ...EMPTY, ...JSON.parse(raw) };
  } catch {
    return EMPTY;
  }
}

function saveContent(pid: string, weekStart: string, c: WeekContent) {
  localStorage.setItem(storageKey(pid, weekStart), JSON.stringify(c));
}

// ── Shared styles ──────────────────────────────────────────────────────────

const inp = "w-full rounded bg-beton-950 border border-beton-800 px-2 py-1.5 text-sm text-beton-100 outline-none focus:border-emniyet-500";
const ta  = inp + " resize-none";
const delBtn = "shrink-0 rounded border border-beton-700 px-2 py-1 text-xs text-beton-400 hover:border-red-400 hover:text-red-400 transition-colors";
const addRow = "w-full mt-2 rounded border border-dashed border-beton-700 px-3 py-1.5 text-xs text-beton-300 hover:border-emniyet-500 transition-colors";

const sectionBase = "rounded-lg border border-beton-800 bg-beton-900 overflow-hidden mb-3";

function SectionHeader({
  color, title, right,
}: { color: string; title: string; right?: React.ReactNode }) {
  return (
    <div className="flex items-center gap-2.5 px-4 py-2.5 border-b border-beton-800">
      <div className="w-[3px] h-3.5 rounded shrink-0" style={{ background: color }} />
      <span className="text-[11px] font-bold uppercase tracking-widest text-beton-100">{title}</span>
      {right && <span className="ml-auto text-[10.5px] text-beton-400">{right}</span>}
    </div>
  );
}

// ── Discipline Section ─────────────────────────────────────────────────────

function DiscSection({
  title, color, value, onChange,
}: {
  title: string; color: string;
  value: DEntry; onChange: (v: DEntry) => void;
}) {
  return (
    <div className={sectionBase}>
      <SectionHeader color={color} title={title} />
      <div className="grid grid-cols-1 sm:grid-cols-2 divide-y sm:divide-y-0 sm:divide-x divide-beton-800">
        <div className="p-3">
          <p className="text-[10px] font-semibold text-beton-400 uppercase tracking-wider mb-2">Haftalık İlerleme</p>
          <textarea
            rows={5} className={ta}
            placeholder={"Gerçekleştirilen iş 1\nGerçekleştirilen iş 2"}
            value={value.ilerleme}
            onChange={e => onChange({ ...value, ilerleme: e.target.value })}
          />
          <p className="text-[10px] text-beton-600 mt-1">Her satır ayrı madde olarak görünür</p>
        </div>
        <div className="p-3">
          <p className="text-[10px] font-semibold text-beton-400 uppercase tracking-wider mb-2">Sonraki Hafta Planlaması</p>
          <textarea
            rows={5} className={ta}
            placeholder={"Planlanan iş 1\nPlanlanan iş 2"}
            value={value.plan}
            onChange={e => onChange({ ...value, plan: e.target.value })}
          />
          <p className="text-[10px] text-beton-600 mt-1">Her satır ayrı madde olarak görünür</p>
        </div>
      </div>
    </div>
  );
}

// ── Main Component ─────────────────────────────────────────────────────────

export default function WeeklyReportsPage() {
  const { current } = useProjects();
  const { can } = useAuth();
  const pid = current?.id;

  const today = new Date().toISOString().slice(0, 10);
  const [weekStart, setWeekStart] = useState(mondayOf(today));
  const [list, setList] = useState<Weekly[]>([]);
  const [err, setErr]   = useState<string | null>(null);
  const [pdfBusy, setPdfBusy] = useState(false);
  const [content, setContent] = useState<WeekContent>(EMPTY);
  const pollRef = useRef<number | null>(null);

  // Load content when project or week changes
  useEffect(() => {
    if (!pid) return;
    setContent(loadContent(pid, weekStart));
  }, [pid, weekStart]);

  // Auto-save on content change
  useEffect(() => {
    if (!pid) return;
    saveContent(pid, weekStart, content);
  }, [pid, weekStart, content]);

  const loadList = useCallback(async () => {
    if (!pid) return [];
    try {
      const res = await api<{ weekly_reports: Weekly[] }>(
        `/projects/${pid}/weekly-reports`,
        { projectId: pid }
      );
      setList(res.weekly_reports);
      return res.weekly_reports;
    } catch {
      setErr("Haftalık raporlar yüklenemedi.");
      return [];
    }
  }, [pid]);

  useEffect(() => { loadList(); }, [loadList]);

  // Poll while any report is pending
  useEffect(() => {
    if (pollRef.current) window.clearInterval(pollRef.current);
    if (list.some(w => w.status === "Pending")) {
      pollRef.current = window.setInterval(async () => {
        const fresh = await loadList();
        if (fresh && !fresh.some(w => w.status === "Pending") && pollRef.current) {
          window.clearInterval(pollRef.current);
          pollRef.current = null;
        }
      }, 4000);
    }
    return () => { if (pollRef.current) window.clearInterval(pollRef.current); };
  }, [list, loadList]);

  async function generatePdf() {
    if (!pid) return;
    setPdfBusy(true);
    setErr(null);
    try {
      await api(`/projects/${pid}/weekly-reports`, {
        method: "POST", body: { period_start: weekStart }, projectId: pid,
      });
      loadList();
    } catch (e) {
      setErr(e instanceof RequestError ? e.message : "Rapor üretimi başlatılamadı.");
    } finally {
      setPdfBusy(false);
    }
  }

  function upd<K extends keyof WeekContent>(key: K, val: WeekContent[K]) {
    setContent(c => ({ ...c, [key]: val }));
  }

  const kasaTotal = content.kasa.reduce((s, r) => s + (parseFloat(r.tutar) || 0), 0);

  if (!pid) return <p className="text-beton-400 text-sm">Önce üst bardan bir proje seçin.</p>;

  const weekEnd  = addDays(weekStart, 6);
  const weekDays = Array.from({ length: 7 }, (_, i) => addDays(weekStart, i));

  return (
    <div className="max-w-4xl mx-auto pb-10 space-y-0">

      {/* ── Top bar ── */}
      <div className="flex items-center justify-between gap-3 flex-wrap mb-5">
        <h1 className="font-display font-extrabold text-xl text-white">Haftalık İlerleme Raporu</h1>
        <Link
          to="/saha-raporlari"
          className="rounded border border-beton-700 px-3 py-1.5 text-sm text-beton-200 hover:border-emniyet-500 transition-colors"
        >
          ← Günlük Raporlar
        </Link>
      </div>

      {/* ── Week header ── */}
      <div className="rounded-lg border border-beton-800 bg-beton-900 p-4 mb-3">
        <div className="flex items-end gap-4 flex-wrap mb-3">
          <div>
            <label className="block text-[10px] text-beton-400 font-semibold uppercase tracking-wider mb-1">
              Hafta Seçimi
            </label>
            <input
              type="date"
              value={weekStart}
              onChange={e => setWeekStart(mondayOf(e.target.value))}
              className="rounded bg-beton-950 border border-beton-800 px-3 py-2 text-sm text-beton-100 outline-none focus:border-emniyet-500"
            />
          </div>
          <span className="text-sm text-beton-300 font-medium pb-2">
            {fmt(weekStart)} — {fmt(weekEnd)}
          </span>
        </div>
        {/* Day strip */}
        <div className="grid grid-cols-7 gap-1.5">
          {weekDays.map((d, i) => {
            const isToday = d === today;
            return (
              <div
                key={d}
                className={`rounded border text-center py-2 px-1 ${
                  isToday
                    ? "border-emniyet-500 bg-emniyet-500/10"
                    : "border-beton-800 bg-beton-950"
                }`}
              >
                <div className="text-[9px] font-semibold uppercase tracking-wider text-beton-400">
                  {DAY_NAMES[i]}
                </div>
                <div className={`text-[10.5px] font-medium mt-0.5 ${isToday ? "text-emniyet-400" : "text-beton-400"}`}>
                  {fmtShort(d)}
                </div>
              </div>
            );
          })}
        </div>
      </div>

      {/* ── İlerleme Raporu ── */}
      <p className="text-[10.5px] font-bold text-beton-500 uppercase tracking-widest mb-2 mt-4">İlerleme Raporu</p>
      <DiscSection title="İnşaat — Mimari" color="#3b7fd4"
        value={content.insaat}   onChange={v => upd("insaat", v)} />
      <DiscSection title="Elektrik" color="#f59e0b"
        value={content.elektrik} onChange={v => upd("elektrik", v)} />
      <DiscSection title="Mekanik" color="#14b8a6"
        value={content.mekanik}  onChange={v => upd("mekanik", v)} />

      {/* ── Makine & Ekipman ── */}
      <div className={sectionBase}>
        <SectionHeader color="#f59e0b" title="Makine & Ekipman"
          right={<>KLV = Kendi &nbsp;·&nbsp; KİRA = Kiralık</>} />
        <div className="p-3">
          <div className="overflow-x-auto">
            <table className="w-full text-sm min-w-[400px]">
              <thead>
                <tr>
                  <th className="text-left text-[10px] font-bold uppercase tracking-wider text-beton-400 pb-2 pr-3">Ekipman Tipi</th>
                  <th className="text-center text-[10px] font-bold uppercase tracking-wider text-beton-400 pb-2 pr-3 w-24">KLV</th>
                  <th className="text-center text-[10px] font-bold uppercase tracking-wider text-beton-400 pb-2 pr-3 w-24">KİRA</th>
                  <th className="w-8" />
                </tr>
              </thead>
              <tbody>
                {content.ekipman.map((row, i) => (
                  <tr key={i} className="border-t border-beton-800">
                    <td className="py-1.5 pr-3">
                      <input className={inp} value={row.tip} placeholder="Kule Vinç…"
                        onChange={e => { const eq = [...content.ekipman]; eq[i] = { ...eq[i], tip: e.target.value }; upd("ekipman", eq); }} />
                    </td>
                    <td className="py-1.5 pr-3">
                      <input type="number" min={0} className={inp + " text-center"} value={row.klv}
                        onChange={e => { const eq = [...content.ekipman]; eq[i] = { ...eq[i], klv: Number(e.target.value) }; upd("ekipman", eq); }} />
                    </td>
                    <td className="py-1.5 pr-3">
                      <input type="number" min={0} className={inp + " text-center"} value={row.kira}
                        onChange={e => { const eq = [...content.ekipman]; eq[i] = { ...eq[i], kira: Number(e.target.value) }; upd("ekipman", eq); }} />
                    </td>
                    <td className="py-1.5">
                      <button className={delBtn} onClick={() => upd("ekipman", content.ekipman.filter((_, j) => j !== i))}>✕</button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          <button className={addRow}
            onClick={() => upd("ekipman", [...content.ekipman, { tip: "", klv: 0, kira: 0 }])}>
            + Ekipman satırı ekle
          </button>
        </div>
      </div>

      {/* ── Taşeron Listesi ── */}
      <div className={sectionBase}>
        <SectionHeader color="#22c55e" title="Taşeron Listesi" />
        <div className="p-3">
          <div className="overflow-x-auto">
            <table className="w-full text-sm min-w-[480px]">
              <thead>
                <tr>
                  <th className="text-left text-[10px] font-bold uppercase tracking-wider text-beton-400 pb-2 pr-3">Firma</th>
                  <th className="text-left text-[10px] font-bold uppercase tracking-wider text-beton-400 pb-2 pr-3 w-36">Disiplin</th>
                  <th className="text-left text-[10px] font-bold uppercase tracking-wider text-beton-400 pb-2 pr-3">Çalışma Alanları</th>
                  <th className="w-8" />
                </tr>
              </thead>
              <tbody>
                {content.taseronlar.map((row, i) => (
                  <tr key={i} className="border-t border-beton-800">
                    <td className="py-1.5 pr-3">
                      <input className={inp} value={row.firma} placeholder="Firma adı…"
                        onChange={e => { const t = [...content.taseronlar]; t[i] = { ...t[i], firma: e.target.value }; upd("taseronlar", t); }} />
                    </td>
                    <td className="py-1.5 pr-3">
                      <input className={inp} value={row.disiplin} placeholder="İnşaat…"
                        onChange={e => { const t = [...content.taseronlar]; t[i] = { ...t[i], disiplin: e.target.value }; upd("taseronlar", t); }} />
                    </td>
                    <td className="py-1.5 pr-3">
                      <input className={inp} value={row.alanlar} placeholder="A, B Blok…"
                        onChange={e => { const t = [...content.taseronlar]; t[i] = { ...t[i], alanlar: e.target.value }; upd("taseronlar", t); }} />
                    </td>
                    <td className="py-1.5">
                      <button className={delBtn} onClick={() => upd("taseronlar", content.taseronlar.filter((_, j) => j !== i))}>✕</button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          <button className={addRow}
            onClick={() => upd("taseronlar", [...content.taseronlar, { firma: "", disiplin: "", alanlar: "" }])}>
            + Taşeron ekle
          </button>
        </div>
      </div>

      {/* ── Satın Alma ── */}
      <div className={sectionBase}>
        <SectionHeader color="#8b7ad4" title="Satın Alma" />
        <div className="grid grid-cols-1 sm:grid-cols-2 divide-y sm:divide-y-0 sm:divide-x divide-beton-800">
          <div className="p-3">
            <p className="text-[10px] font-semibold text-beton-400 uppercase tracking-wider mb-2">Bu Hafta Şantiyeye Sevk Edilen</p>
            {content.sevkler.map((row, i) => (
              <div key={i} className="flex gap-1.5 mb-1.5 items-center">
                <input type="date" className={inp + " w-36 shrink-0"} value={row.tarih}
                  onChange={e => { const s = [...content.sevkler]; s[i] = { ...s[i], tarih: e.target.value }; upd("sevkler", s); }} />
                <input className={inp} value={row.malzeme} placeholder="Malzeme…"
                  onChange={e => { const s = [...content.sevkler]; s[i] = { ...s[i], malzeme: e.target.value }; upd("sevkler", s); }} />
                <button className={delBtn} onClick={() => upd("sevkler", content.sevkler.filter((_, j) => j !== i))}>✕</button>
              </div>
            ))}
            <button className={addRow}
              onClick={() => upd("sevkler", [...content.sevkler, { tarih: today, malzeme: "" }])}>
              + Ekle
            </button>
          </div>
          <div className="p-3">
            <p className="text-[10px] font-semibold text-beton-400 uppercase tracking-wider mb-2">Sonraki Hafta Olası Satın Alma Talepleri</p>
            {content.talepler.map((row, i) => (
              <div key={i} className="flex gap-1.5 mb-1.5 items-center">
                <input type="date" className={inp + " w-36 shrink-0"} value={row.termin}
                  onChange={e => { const t = [...content.talepler]; t[i] = { ...t[i], termin: e.target.value }; upd("talepler", t); }} />
                <input className={inp} value={row.malzeme} placeholder="Malzeme…"
                  onChange={e => { const t = [...content.talepler]; t[i] = { ...t[i], malzeme: e.target.value }; upd("talepler", t); }} />
                <button className={delBtn} onClick={() => upd("talepler", content.talepler.filter((_, j) => j !== i))}>✕</button>
              </div>
            ))}
            <button className={addRow}
              onClick={() => upd("talepler", [...content.talepler, { termin: addDays(weekEnd, 3), malzeme: "" }])}>
              + Ekle
            </button>
          </div>
        </div>
      </div>

      {/* ── Şantiye Kasa Harcaması ── */}
      <div className={sectionBase}>
        <SectionHeader
          color="#f43f5e"
          title="Şantiye Kasa Harcaması"
          right={
            content.kasa.length > 0
              ? <>Haftalık Toplam: <strong className="text-beton-100 tabular-nums">
                  {kasaTotal.toLocaleString("tr-TR", { minimumFractionDigits: 2 })} ₺
                </strong></>
              : undefined
          }
        />
        <div className="p-3">
          <div className="overflow-x-auto">
            <table className="w-full text-sm min-w-[600px]">
              <thead>
                <tr>
                  <th className="text-left text-[10px] font-bold uppercase tracking-wider text-beton-400 pb-2 pr-2 w-32">Tarih</th>
                  <th className="text-left text-[10px] font-bold uppercase tracking-wider text-beton-400 pb-2 pr-2">Açıklama</th>
                  <th className="text-left text-[10px] font-bold uppercase tracking-wider text-beton-400 pb-2 pr-2 w-32">Kategori</th>
                  <th className="text-right text-[10px] font-bold uppercase tracking-wider text-beton-400 pb-2 pr-2 w-32">Tutar (₺)</th>
                  <th className="text-left text-[10px] font-bold uppercase tracking-wider text-beton-400 pb-2 pr-2 w-28">Makbuz No</th>
                  <th className="w-8" />
                </tr>
              </thead>
              <tbody>
                {content.kasa.map((row, i) => (
                  <tr key={i} className="border-t border-beton-800">
                    <td className="py-1.5 pr-2">
                      <input type="date" className={inp}
                        value={row.tarih}
                        onChange={e => { const k = [...content.kasa]; k[i] = { ...k[i], tarih: e.target.value }; upd("kasa", k); }} />
                    </td>
                    <td className="py-1.5 pr-2">
                      <input className={inp} value={row.aciklama} placeholder="Harcama açıklaması…"
                        onChange={e => { const k = [...content.kasa]; k[i] = { ...k[i], aciklama: e.target.value }; upd("kasa", k); }} />
                    </td>
                    <td className="py-1.5 pr-2">
                      <select className={inp} value={row.kategori}
                        onChange={e => { const k = [...content.kasa]; k[i] = { ...k[i], kategori: e.target.value }; upd("kasa", k); }}>
                        <option value="">— Seçin —</option>
                        <option>Malzeme</option>
                        <option>Yemek</option>
                        <option>Ulaşım</option>
                        <option>İşçilik</option>
                        <option>Diğer</option>
                      </select>
                    </td>
                    <td className="py-1.5 pr-2">
                      <input type="number" min={0} step="0.01" className={inp + " text-right tabular-nums"}
                        value={row.tutar} placeholder="0,00"
                        onChange={e => { const k = [...content.kasa]; k[i] = { ...k[i], tutar: e.target.value }; upd("kasa", k); }} />
                    </td>
                    <td className="py-1.5 pr-2">
                      <input className={inp} value={row.makbuz} placeholder="MKB-…"
                        onChange={e => { const k = [...content.kasa]; k[i] = { ...k[i], makbuz: e.target.value }; upd("kasa", k); }} />
                    </td>
                    <td className="py-1.5">
                      <button className={delBtn} onClick={() => upd("kasa", content.kasa.filter((_, j) => j !== i))}>✕</button>
                    </td>
                  </tr>
                ))}
                {content.kasa.length > 0 && (
                  <tr className="border-t-2 border-beton-700">
                    <td colSpan={3} className="py-2 text-[11px] font-bold text-beton-300 uppercase tracking-wider">
                      Haftalık Toplam
                    </td>
                    <td className="py-2 text-right font-bold text-white tabular-nums pr-2">
                      {kasaTotal.toLocaleString("tr-TR", { minimumFractionDigits: 2 })} ₺
                    </td>
                    <td colSpan={2} />
                  </tr>
                )}
              </tbody>
            </table>
          </div>
          <button className={addRow}
            onClick={() => upd("kasa", [...content.kasa, { tarih: today, aciklama: "", kategori: "", tutar: "", makbuz: "" }])}>
            + Harcama satırı ekle
          </button>
        </div>
      </div>

      {/* ── PDF Üretim ── */}
      {can("reports.generate_weekly") && (
        <div className="rounded-lg border border-beton-800 bg-beton-900 p-4 mb-3">
          <p className="text-[10.5px] font-bold text-beton-500 uppercase tracking-widest mb-3">PDF Raporu</p>
          <div className="flex items-center gap-3 flex-wrap">
            <button
              onClick={generatePdf}
              disabled={pdfBusy}
              className="rounded bg-emniyet-500 px-4 py-2 text-sm font-medium text-beton-950 hover:brightness-110 disabled:opacity-50 transition-all"
            >
              {pdfBusy ? "Başlatılıyor…" : "PDF Üret"}
            </button>
            <p className="text-xs text-beton-500">
              Veriler üretim anında dondurulur; sonraki revizyonlar PDF'i değiştirmez.
            </p>
          </div>
          {err && <p className="text-red-400 text-xs mt-2">{err}</p>}
          {list.length > 0 && (
            <ul className="mt-3 space-y-2">
              {list.map(w => (
                <li
                  key={w.id}
                  className="flex items-center justify-between gap-3 flex-wrap rounded border border-beton-800 bg-beton-950 px-3 py-2"
                >
                  <div>
                    <span className="text-sm text-white">Hafta {w.week_no}</span>
                    <span className="text-xs text-beton-400 ml-2">({fmt(w.period_start)} – {fmt(w.period_end)})</span>
                    <span className="text-xs text-beton-500 ml-2">· {w.generated_by_name}</span>
                    {w.status === "Failed" && w.error && (
                      <span className="text-xs text-red-400 ml-2">— {w.error}</span>
                    )}
                  </div>
                  <div className="flex items-center gap-2">
                    <span className={`rounded-full border px-2 py-0.5 text-xs ${W_STATUS_STYLE[w.status]}`}>
                      {W_STATUS_LABEL[w.status]}
                    </span>
                    {w.status === "Ready" && w.has_pdf && (
                      <button
                        onClick={() =>
                          apiDownload(
                            `/projects/${pid}/weekly-reports/${w.id}/download`,
                            `haftalik-rapor-H${w.week_no}.pdf`
                          ).catch(() => setErr("PDF indirilemedi."))
                        }
                        className="rounded border border-beton-700 px-3 py-1 text-xs text-beton-100 hover:border-emniyet-500 transition-colors"
                      >
                        İndir
                      </button>
                    )}
                  </div>
                </li>
              ))}
            </ul>
          )}
        </div>
      )}

      <p className="text-[10.5px] text-beton-600 text-center mt-2">
        İçerik tarayıcıya otomatik kaydedilir · Her hafta için ayrı kayıt tutulur
      </p>
    </div>
  );
}
