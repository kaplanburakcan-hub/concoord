import { useCallback, useEffect, useRef, useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { api, apiFetchBlob, RequestError } from "../../api/client";
import { useAuth } from "../../auth/AuthContext";
import { useProjects } from "../../projects/ProjectContext";
import { apiWithOfflineFallback } from "../../offline/queue";
import { DR_STATUS_LABEL, DR_STATUS_STYLE, formatDateTR } from "./DailyReportsPage";
import PdfPreviewModal from "../../components/PdfPreviewModal";

// Faz 6 — Günlük rapor formu (mobil öncelikli: tek kolon, büyük dokunma
// hedefleri, bölüm bölüm satır ekleme). Aynı bileşen üç modda çalışır:
//   /saha-raporlari/yeni      → yeni taslak
//   /saha-raporlari/:id       → görüntüle (Submitted) ya da taslak düzenle
// Submitted raporda form salt okunur; "Revizyon aç" yeni taslak açar.
// Çevrimdışıyken kaydet/gönder localStorage kuyruğuna düşer (offline/queue.ts).

type Manpower = { subcontractor_id?: string; trade: string; headcount: number };
type Equipment = { equipment_name: string; count: number; working_hours?: number; idle_reason?: string };
type WorkEntry = { work_item_id?: string; location?: string; description: string; qty?: number; unit?: string };
type CashExpense = { description: string; category: string; amount: number; receipt_no?: string };

const CASH_CATEGORIES = ["Yakıt", "Yemek", "Nakliye", "Küçük Malzeme", "Diğer"];

type Detail = {
  id: string;
  report_date: string;
  revision_no: number;
  status: "Draft" | "Submitted";
  weather?: { condition?: string; wind_kph?: number; precipitation_mm?: number; source?: string };
  temperature_min?: number;
  temperature_max?: number;
  notes?: string;
  author_name: string;
  cover_photo_file_id?: string;
  manpower?: (Manpower & { subcontractor_name?: string })[];
  equipment?: Equipment[];
  work_entries?: (WorkEntry & { work_item_poz?: string })[];
  cash_expenses?: CashExpense[];
};

type Sub = { id: string; company_name: string };
type WorkItem = { id: string; poz_no: string; description: string; unit: string };
type Machine = { id: string; tip: string; ad: string };

const MACHINE_TIP_LABEL: Record<string, string> = {
  arac: "Araç",
  is_makinesi: "İş Makinesi",
  ekipman: "Ekipman",
};

type WarehouseDelta = {
  malzeme_adi: string; kategori: string; birim: string;
  giris: number; cikis: number; net_delta: number;
};
type DailyCtx = {
  warehouse_delta: WarehouseDelta[];
  pending_mars: number;
  pending_pos: number;
  open_tasks: number;
};

const todayISO = () => new Date().toISOString().slice(0, 10);

export default function DailyReportFormPage() {
  const { id } = useParams();
  const isNew = !id;
  const nav = useNavigate();
  const { current } = useProjects();
  const { can } = useAuth();
  const pid = current?.id;

  const [detail, setDetail] = useState<Detail | null>(null);
  const [err, setErr] = useState<string | null>(null);
  const [info, setInfo] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [showPdfPreview, setShowPdfPreview] = useState(false);

  // Form state
  const [reportDate, setReportDate] = useState(todayISO());
  const [condition, setCondition] = useState("");
  const [tempMin, setTempMin] = useState("");
  const [tempMax, setTempMax] = useState("");
  const [notes, setNotes] = useState("");
  const [manpower, setManpower] = useState<Manpower[]>([]);
  const [equipment, setEquipment] = useState<Equipment[]>([]);
  const [entries, setEntries] = useState<WorkEntry[]>([]);
  const [cashExpenses, setCashExpenses] = useState<CashExpense[]>([]);

  const [subs, setSubs]   = useState<Sub[]>([]);
  const [items, setItems] = useState<WorkItem[]>([]);
  const [machines, setMachines] = useState<Machine[]>([]);
  const [ctx,  setCtx]   = useState<DailyCtx | null>(null);

  const readOnly = !!detail && detail.status === "Submitted";
  const canEdit = can("reports.create_daily");

  const load = useCallback(async () => {
    if (!pid || isNew) return;
    setErr(null);
    try {
      const res = await api<{ daily_report: Detail }>(
        `/projects/${pid}/daily-reports/${id}`,
        { projectId: pid }
      );
      const d = res.daily_report;
      setDetail(d);
      setReportDate(d.report_date);
      setCondition(d.weather?.condition ?? "");
      setTempMin(d.temperature_min != null ? String(d.temperature_min) : "");
      setTempMax(d.temperature_max != null ? String(d.temperature_max) : "");
      setNotes(d.notes ?? "");
      setManpower((d.manpower ?? []).map(({ subcontractor_id, trade, headcount }) => ({ subcontractor_id, trade, headcount })));
      setEquipment((d.equipment ?? []).map(({ equipment_name, count, working_hours, idle_reason }) => ({ equipment_name, count, working_hours, idle_reason })));
      setEntries((d.work_entries ?? []).map(({ work_item_id, location, description, qty, unit }) => ({ work_item_id, location, description, qty, unit })));
      setCashExpenses((d.cash_expenses ?? []).map(({ description, category, amount, receipt_no }) => ({ description, category, amount, receipt_no })));
    } catch {
      setErr("Rapor yüklenemedi ya da erişim yetkiniz yok.");
    }
  }, [pid, id, isNew]);

  useEffect(() => {
    load();
  }, [load]);

  // Referans listeleri (çevrimdışıysa boş kalır; serbest metin girişi yeter).
  // active_on=reportDate: yalnızca o tarihte sözleşmesi geçerli olan
  // taşeronlar listelenir (personel dropdown'ının kontrol altyapısı).
  useEffect(() => {
    if (!pid || !reportDate) return;
    api<{ subcontractors: Sub[] }>(
      `/projects/${pid}/subcontractors?active_on=${reportDate}`,
      { projectId: pid }
    )
      .then((r) => setSubs(r.subcontractors))
      .catch(() => {});
  }, [pid, reportDate]);
  // Tanımlı araçlar/iş makineleri/ekipmanlar — ekipman satırları serbest
  // metin yerine bu listeden seçilir.
  useEffect(() => {
    if (!pid) return;
    api<{ machines: Machine[] }>(`/projects/${pid}/machines`, { projectId: pid })
      .then((r) => setMachines(r.machines))
      .catch(() => {});
  }, [pid]);
  // Depo-stok özeti ve bekleyen aktiviteler — tarih değiştiğinde yenile.
  useEffect(() => {
    if (!pid || !reportDate) return;
    api<DailyCtx>(
      `/projects/${pid}/daily-report-context?date=${reportDate}`,
      { projectId: pid }
    ).then(setCtx).catch(() => {});
  }, [pid, reportDate]);

  useEffect(() => {
    if (!pid || subs.length === 0) return;
    // Poz listesi: tüm taşeronların birleşimi (imalat girdisi bağlamak için).
    Promise.all(
      subs.map((s) =>
        api<{ work_items: WorkItem[] }>(
          `/projects/${pid}/subcontractors/${s.id}/work-items`,
          { projectId: pid }
        ).then((r) => r.work_items).catch(() => [] as WorkItem[])
      )
    ).then((all) => setItems(all.flat()));
  }, [pid, subs]);

  function buildBody() {
    return {
      report_date: reportDate,
      weather: condition ? { condition, source: "manual" } : null,
      temperature_min: tempMin === "" ? null : Number(tempMin),
      temperature_max: tempMax === "" ? null : Number(tempMax),
      notes,
      manpower,
      equipment,
      work_entries: entries,
      cash_expenses: cashExpenses,
    };
  }

  async function save() {
    if (!pid) return;
    setBusy(true);
    setErr(null);
    setInfo(null);
    try {
      if (isNew) {
        const res = await apiWithOfflineFallback<{ id: string }>({
          method: "POST",
          path: `/projects/${pid}/daily-reports`,
          projectId: pid,
          body: buildBody(),
          label: `Günlük rapor ${formatDateTR(reportDate)} (oluştur)`,
        });
        if (res.queued) {
          setInfo("Çevrimdışısınız: rapor cihaza kaydedildi, bağlantıda gönderilecek.");
          return;
        }
        nav(`/saha-raporlari/${res.data!.id}`, { replace: true });
      } else {
        const res = await apiWithOfflineFallback({
          method: "PUT",
          path: `/projects/${pid}/daily-reports/${id}`,
          projectId: pid,
          body: buildBody(),
          label: `Günlük rapor ${formatDateTR(reportDate)} (güncelle)`,
        });
        setInfo(res.queued
          ? "Çevrimdışısınız: değişiklik cihaza kaydedildi, bağlantıda gönderilecek."
          : "Taslak kaydedildi.");
        if (!res.queued) load();
      }
    } catch (e) {
      setErr(e instanceof RequestError ? e.message : "Kaydedilemedi.");
    } finally {
      setBusy(false);
    }
  }

  async function submit() {
    if (!pid || !id) return;
    if (!window.confirm("Rapor gönderilecek ve değiştirilemez olacak. Emin misiniz?")) return;
    setBusy(true);
    setErr(null);
    try {
      const res = await apiWithOfflineFallback({
        method: "POST",
        path: `/projects/${pid}/daily-reports/${id}/submit`,
        projectId: pid,
        body: {},
        label: `Günlük rapor ${formatDateTR(reportDate)} (gönder)`,
      });
      if (res.queued) {
        setInfo("Çevrimdışısınız: gönderim bağlantıda tamamlanacak.");
      } else {
        load();
      }
    } catch (e) {
      setErr(e instanceof RequestError ? e.message : "Gönderilemedi.");
    } finally {
      setBusy(false);
    }
  }

  async function revise() {
    if (!pid || !id) return;
    setBusy(true);
    setErr(null);
    try {
      const res = await api<{ id: string }>(
        `/projects/${pid}/daily-reports/${id}/revise`,
        { method: "POST", body: {}, projectId: pid }
      );
      nav(`/saha-raporlari/${res.id}`, { replace: true });
      setDetail(null);
      load();
    } catch (e) {
      setErr(e instanceof RequestError ? e.message : "Revizyon açılamadı.");
    } finally {
      setBusy(false);
    }
  }

  async function prefillWeather() {
    if (!pid) return;
    setErr(null);
    if (!("geolocation" in navigator)) {
      setErr("Cihaz konumu alınamıyor; hava alanlarını elle doldurun.");
      return;
    }
    navigator.geolocation.getCurrentPosition(
      async (pos) => {
        try {
          const r = await api<{ weather: { condition: string; temperature_min?: number; temperature_max?: number } }>(
            `/projects/${pid}/daily-reports/weather?date=${reportDate}&lat=${pos.coords.latitude.toFixed(4)}&lng=${pos.coords.longitude.toFixed(4)}`,
            { projectId: pid }
          );
          setCondition(r.weather.condition);
          if (r.weather.temperature_min != null) setTempMin(String(r.weather.temperature_min));
          if (r.weather.temperature_max != null) setTempMax(String(r.weather.temperature_max));
          setInfo("Hava durumu ön dolduruldu; kaydetmeden önce kontrol edin.");
        } catch (e) {
          setErr(e instanceof RequestError ? e.message : "Hava durumu alınamadı; elle doldurun.");
        }
      },
      () => setErr("Konum izni verilmedi; hava alanlarını elle doldurun.")
    );
  }

  if (!pid) return <p className="text-beton-400 text-sm">Önce üst bardan bir proje seçin.</p>;

  // ── Read-only (Submitted) view ──────────────────────────────────────────
  if (readOnly && detail) {
    return (
      <ReadOnlyView
        detail={detail}
        busy={busy}
        canEdit={canEdit}
        err={err}
        ctx={ctx}
        onRevise={revise}
        project={current}
        pid={pid}
      />
    );
  }

  const input =
    "w-full rounded-md bg-beton-950 border border-beton-800 px-3 py-2 text-sm text-beton-100 outline-none focus:border-emniyet-500 disabled:opacity-60";
  const label = "block text-xs text-beton-400 mb-1";
  const section = "rounded-lg border border-beton-800 bg-beton-900 p-4 space-y-3";
  const addBtn =
    "rounded-md border border-dashed border-beton-700 px-3 py-2 text-xs text-beton-300 hover:border-emniyet-500 w-full";
  const rmBtn = "shrink-0 self-start rounded border border-beton-700 px-2 py-1 text-xs text-beton-400 hover:border-red-400";

  return (
    <div className="max-w-2xl mx-auto space-y-4 pb-24">
      <div className="rounded-lg border border-beton-800 bg-beton-900 p-4">
        <div className="flex gap-3">
          <div className="flex-1 min-w-0">
            <div className="flex items-center justify-between gap-2 flex-wrap">
              <h1 className="font-display font-extrabold text-xl text-white">
                {isNew ? "Yeni Günlük Rapor" : `Günlük Rapor — ${formatDateTR(reportDate)}`}
                {detail && detail.revision_no > 1 && (
                  <span className="ml-2 text-sm text-emniyet-500">rev {detail.revision_no}</span>
                )}
              </h1>
              <div className="flex items-center gap-2">
                {detail && (
                  <button onClick={() => setShowPdfPreview(true)}
                    className="no-print rounded-md border border-beton-700 px-2.5 py-1 text-xs text-beton-300 hover:border-emniyet-500 transition-colors">
                    PDF Rapor Üret
                  </button>
                )}
                {detail && (
                  <span className={`rounded-full border px-2 py-0.5 text-xs ${DR_STATUS_STYLE[detail.status]}`}>
                    {DR_STATUS_LABEL[detail.status]}
                  </span>
                )}
              </div>
            </div>
          </div>
          <CoverPhotoBox pid={pid} reportId={detail?.id} coverPhotoFileId={detail?.cover_photo_file_id}
            canEdit={canEdit} onUploaded={load} />
        </div>
      </div>
      {detail && (
        <PdfPreviewModal
          open={showPdfPreview}
          onClose={() => setShowPdfPreview(false)}
          title={`Günlük Rapor — ${formatDateTR(reportDate)}`}
          fetchPath={`/projects/${pid}/daily-reports/${detail.id}/pdf`}
          downloadName={`gunluk-rapor-${detail.report_date}.pdf`}
        />
      )}

      {err && <p className="text-red-400 text-sm">{err}</p>}
      {info && <p className="text-emniyet-500 text-sm">{info}</p>}
      {readOnly && (
        <p className="text-beton-400 text-xs">
          Gönderilmiş rapor değiştirilemez. Düzeltme gerekiyorsa yeni revizyon açın; eski kayıt izlenebilir kalır.
        </p>
      )}

      {/* Başlık: tarih + hava */}
      <div className={section}>
        <div className="grid grid-cols-2 gap-3">
          <div className="col-span-2 sm:col-span-1">
            <label className={label}>Rapor tarihi</label>
            <input type="date" className={input} value={reportDate} disabled={!isNew}
              onChange={(e) => setReportDate(e.target.value)} />
          </div>
          <div className="col-span-2 sm:col-span-1">
            <label className={label}>Hava durumu</label>
            <input className={input} placeholder="Açık / Yağmurlu…" value={condition}
              disabled={readOnly || !canEdit} onChange={(e) => setCondition(e.target.value)} />
          </div>
          <div>
            <label className={label}>Sıcaklık min (°C)</label>
            <input type="number" className={input} value={tempMin}
              disabled={readOnly || !canEdit} onChange={(e) => setTempMin(e.target.value)} />
          </div>
          <div>
            <label className={label}>Sıcaklık max (°C)</label>
            <input type="number" className={input} value={tempMax}
              disabled={readOnly || !canEdit} onChange={(e) => setTempMax(e.target.value)} />
          </div>
        </div>
        {!readOnly && canEdit && (
          <button onClick={prefillWeather} className="text-xs text-emniyet-500 hover:underline">
            Konumdan hava durumunu doldur
          </button>
        )}
      </div>

      {/* Personel */}
      <div className={section}>
        <h2 className="font-medium text-white text-sm">Personel</h2>
        {manpower.map((m, i) => (
          <div key={i} className="flex gap-2 items-end">
            <div className="flex-1 grid grid-cols-2 gap-2">
              <div className="col-span-2 sm:col-span-1">
                <label className={label}>Taşeron (opsiyonel)</label>
                <select className={input} value={m.subcontractor_id ?? ""} disabled={readOnly || !canEdit}
                  onChange={(e) => setManpower(upd(manpower, i, { subcontractor_id: e.target.value || undefined }))}>
                  <option value="">— Ana yüklenici —</option>
                  {subs.map((s) => <option key={s.id} value={s.id}>{s.company_name}</option>)}
                </select>
              </div>
              <div>
                <label className={label}>Branş</label>
                <input className={input} placeholder="Kalıpçı…" value={m.trade} disabled={readOnly || !canEdit}
                  onChange={(e) => setManpower(upd(manpower, i, { trade: e.target.value }))} />
              </div>
              <div>
                <label className={label}>Kişi</label>
                <input type="number" min={0} className={input} value={m.headcount} disabled={readOnly || !canEdit}
                  onChange={(e) => setManpower(upd(manpower, i, { headcount: Number(e.target.value) }))} />
              </div>
            </div>
            {!readOnly && canEdit && (
              <button className={rmBtn} onClick={() => setManpower(manpower.filter((_, j) => j !== i))}>Sil</button>
            )}
          </div>
        ))}
        {!readOnly && canEdit && (
          <button className={addBtn} onClick={() => setManpower([...manpower, { trade: "", headcount: 0 }])}>
            + Personel satırı ekle
          </button>
        )}
      </div>

      {/* Ekipman */}
      <div className={section}>
        <h2 className="font-medium text-white text-sm">Ekipman</h2>
        {equipment.map((e, i) => (
          <div key={i} className="flex gap-2 items-end">
            <div className="flex-1 grid grid-cols-2 gap-2">
              <div className="col-span-2 sm:col-span-1">
                <label className={label}>Ekipman</label>
                <select className={input} value={e.equipment_name} disabled={readOnly || !canEdit}
                  onChange={(ev) => setEquipment(upd(equipment, i, { equipment_name: ev.target.value }))}>
                  <option value="" disabled>— Seçin —</option>
                  {e.equipment_name && !machines.some((m) => m.ad === e.equipment_name) && (
                    <option value={e.equipment_name}>{e.equipment_name} (tanımsız)</option>
                  )}
                  {["arac", "is_makinesi", "ekipman"].map((tip) => {
                    const group = machines.filter((m) => m.tip === tip);
                    if (group.length === 0) return null;
                    return (
                      <optgroup key={tip} label={MACHINE_TIP_LABEL[tip] ?? tip}>
                        {group.map((m) => <option key={m.id} value={m.ad}>{m.ad}</option>)}
                      </optgroup>
                    );
                  })}
                </select>
                {machines.length === 0 && !readOnly && canEdit && (
                  <p className="mt-1 text-[11px] text-beton-500">
                    Bu projede tanımlı araç/ekipman yok.{" "}
                    <Link to="/makine/ekipmanlar" target="_blank" className="text-emniyet-500 hover:underline">
                      Makine & Ekipman'dan tanımlayın
                    </Link>
                    .
                  </p>
                )}
              </div>
              <div>
                <label className={label}>Adet</label>
                <input type="number" min={0} className={input} value={e.count} disabled={readOnly || !canEdit}
                  onChange={(ev) => setEquipment(upd(equipment, i, { count: Number(ev.target.value) }))} />
              </div>
              <div>
                <label className={label}>Çalışma saati</label>
                <input type="number" min={0} step="0.5" className={input}
                  value={e.working_hours ?? ""} disabled={readOnly || !canEdit}
                  onChange={(ev) => setEquipment(upd(equipment, i, { working_hours: ev.target.value === "" ? undefined : Number(ev.target.value) }))} />
              </div>
              <div className="col-span-2">
                <label className={label}>Bekleme nedeni (varsa)</label>
                <input className={input} value={e.idle_reason ?? ""} disabled={readOnly || !canEdit}
                  onChange={(ev) => setEquipment(upd(equipment, i, { idle_reason: ev.target.value || undefined }))} />
              </div>
            </div>
            {!readOnly && canEdit && (
              <button className={rmBtn} onClick={() => setEquipment(equipment.filter((_, j) => j !== i))}>Sil</button>
            )}
          </div>
        ))}
        {!readOnly && canEdit && (
          <button className={addBtn} onClick={() => setEquipment([...equipment, { equipment_name: "", count: 1 }])}>
            + Ekipman satırı ekle
          </button>
        )}
      </div>

      {/* İmalat girdileri */}
      <div className={section}>
        <h2 className="font-medium text-white text-sm">İmalat Girdileri</h2>
        {entries.map((w, i) => (
          <div key={i} className="flex gap-2 items-end">
            <div className="flex-1 grid grid-cols-2 gap-2">
              <div className="col-span-2">
                <label className={label}>Poz bağlantısı (opsiyonel — hakediş metrajıyla eşleşir)</label>
                <select className={input} value={w.work_item_id ?? ""} disabled={readOnly || !canEdit}
                  onChange={(e) => {
                    const wi = items.find((x) => x.id === e.target.value);
                    setEntries(upd(entries, i, {
                      work_item_id: e.target.value || undefined,
                      unit: wi ? wi.unit : w.unit,
                    }));
                  }}>
                  <option value="">— Serbest imalat —</option>
                  {items.map((x) => (
                    <option key={x.id} value={x.id}>{x.poz_no} — {x.description}</option>
                  ))}
                </select>
              </div>
              <div className="col-span-2">
                <label className={label}>Açıklama</label>
                <input className={input} placeholder="B blok 3. kat perde betonu…" value={w.description}
                  disabled={readOnly || !canEdit}
                  onChange={(e) => setEntries(upd(entries, i, { description: e.target.value }))} />
              </div>
              <div>
                <label className={label}>Lokasyon</label>
                <input className={input} placeholder="Blok/kat/aks" value={w.location ?? ""} disabled={readOnly || !canEdit}
                  onChange={(e) => setEntries(upd(entries, i, { location: e.target.value || undefined }))} />
              </div>
              <div className="grid grid-cols-2 gap-2">
                <div>
                  <label className={label}>Miktar</label>
                  <input type="number" min={0} step="0.001" className={input} value={w.qty ?? ""}
                    disabled={readOnly || !canEdit}
                    onChange={(e) => setEntries(upd(entries, i, { qty: e.target.value === "" ? undefined : Number(e.target.value) }))} />
                </div>
                <div>
                  <label className={label}>Birim</label>
                  <input className={input} placeholder="m³" value={w.unit ?? ""} disabled={readOnly || !canEdit}
                    onChange={(e) => setEntries(upd(entries, i, { unit: e.target.value || undefined }))} />
                </div>
              </div>
            </div>
            {!readOnly && canEdit && (
              <button className={rmBtn} onClick={() => setEntries(entries.filter((_, j) => j !== i))}>Sil</button>
            )}
          </div>
        ))}
        {!readOnly && canEdit && (
          <button className={addBtn} onClick={() => setEntries([...entries, { description: "" }])}>
            + İmalat girdisi ekle
          </button>
        )}
      </div>

      {/* Şantiye Kasa Harcaması */}
      <div className={section}>
        <h2 className="font-medium text-white text-sm">Şantiye Kasa Harcaması</h2>
        {cashExpenses.map((c, i) => (
          <div key={i} className="flex gap-2 items-end">
            <div className="flex-1 grid grid-cols-2 gap-2">
              <div className="col-span-2">
                <label className={label}>Açıklama</label>
                <input className={input} placeholder="Şantiye personeli yemek bedeli…" value={c.description}
                  disabled={readOnly || !canEdit}
                  onChange={(e) => setCashExpenses(upd(cashExpenses, i, { description: e.target.value }))} />
              </div>
              <div>
                <label className={label}>Kategori</label>
                <select className={input} value={c.category} disabled={readOnly || !canEdit}
                  onChange={(e) => setCashExpenses(upd(cashExpenses, i, { category: e.target.value }))}>
                  <option value="">— Seçin —</option>
                  {CASH_CATEGORIES.map((cat) => <option key={cat} value={cat}>{cat}</option>)}
                </select>
              </div>
              <div>
                <label className={label}>Tutar (₺)</label>
                <input type="number" min={0} step="0.01" className={input} value={c.amount || ""}
                  disabled={readOnly || !canEdit}
                  onChange={(e) => setCashExpenses(upd(cashExpenses, i, { amount: Number(e.target.value) }))} />
              </div>
              <div className="col-span-2">
                <label className={label}>Makbuz No (opsiyonel)</label>
                <input className={input} value={c.receipt_no ?? ""} disabled={readOnly || !canEdit}
                  onChange={(e) => setCashExpenses(upd(cashExpenses, i, { receipt_no: e.target.value || undefined }))} />
              </div>
            </div>
            {!readOnly && canEdit && (
              <button className={rmBtn} onClick={() => setCashExpenses(cashExpenses.filter((_, j) => j !== i))}>Sil</button>
            )}
          </div>
        ))}
        {!readOnly && canEdit && (
          <button className={addBtn} onClick={() => setCashExpenses([...cashExpenses, { description: "", category: "Diğer", amount: 0 }])}>
            + Kasa harcaması ekle
          </button>
        )}
      </div>

      {/* Notlar */}
      <div className={section}>
        <label className={label}>Genel notlar</label>
        <textarea rows={3} className={input} value={notes} disabled={readOnly || !canEdit}
          onChange={(e) => setNotes(e.target.value)} />
      </div>

      {/* Depo-Stok Özeti */}
      {ctx && (
        <div className={section}>
          <h2 className="font-medium text-white text-sm">Depo-Stok Özeti</h2>
          <p className="text-xs text-beton-500">{formatDateTR(reportDate)} tarihli depo hareketleri</p>
          {ctx.warehouse_delta.length === 0 ? (
            <p className="text-xs text-beton-500 italic">Bu tarih için depo hareketi yok.</p>
          ) : (
            <div className="space-y-1.5">
              {ctx.warehouse_delta.map((wh, i) => (
                <div key={i} className="flex items-center gap-2 text-sm flex-wrap">
                  <span className="flex-1 min-w-0 truncate text-beton-100">{wh.malzeme_adi}</span>
                  <span className="text-[11px] text-beton-500">{wh.kategori}</span>
                  {wh.giris > 0 && <span className="text-green-400 tabular-nums text-xs">+{wh.giris} {wh.birim}</span>}
                  {wh.cikis > 0 && <span className="text-red-400 tabular-nums text-xs">−{wh.cikis} {wh.birim}</span>}
                  <span className={`font-semibold tabular-nums text-xs ${wh.net_delta >= 0 ? "text-green-300" : "text-red-300"}`}>
                    Net: {wh.net_delta >= 0 ? "+" : ""}{wh.net_delta} {wh.birim}
                  </span>
                </div>
              ))}
            </div>
          )}
        </div>
      )}

      {/* Bekleyen Aktiviteler */}
      {ctx && (ctx.pending_mars > 0 || ctx.pending_pos > 0 || ctx.open_tasks > 0) && (
        <div className={section}>
          <h2 className="font-medium text-white text-sm">Bekleyen Aktiviteler</h2>
          <div className="flex flex-wrap gap-3">
            {ctx.pending_mars > 0 && (
              <div className="rounded-md border border-beton-700 bg-beton-950 px-3 py-2 text-center min-w-[90px]">
                <div className="text-lg font-bold text-beton-100 tabular-nums">{ctx.pending_mars}</div>
                <div className="text-[10px] uppercase tracking-wider text-beton-500 mt-0.5">Bekleyen MAR</div>
              </div>
            )}
            {ctx.pending_pos > 0 && (
              <div className="rounded-md border border-beton-700 bg-beton-950 px-3 py-2 text-center min-w-[90px]">
                <div className="text-lg font-bold text-beton-100 tabular-nums">{ctx.pending_pos}</div>
                <div className="text-[10px] uppercase tracking-wider text-beton-500 mt-0.5">Açık Sipariş</div>
              </div>
            )}
            {ctx.open_tasks > 0 && (
              <div className="rounded-md border border-beton-700 bg-beton-950 px-3 py-2 text-center min-w-[90px]">
                <div className="text-lg font-bold text-beton-100 tabular-nums">{ctx.open_tasks}</div>
                <div className="text-[10px] uppercase tracking-wider text-beton-500 mt-0.5">Açık Görev</div>
              </div>
            )}
          </div>
        </div>
      )}

      {/* Aksiyon çubuğu (mobilde alta sabit) */}
      <div className="no-print fixed bottom-0 inset-x-0 border-t border-beton-800 bg-beton-900/95 backdrop-blur p-3">
        <div className="max-w-2xl mx-auto flex gap-2">
          {!readOnly && canEdit && (
            <button onClick={save} disabled={busy}
              className="flex-1 rounded-md border border-beton-700 px-4 py-3 text-sm text-beton-100 hover:border-emniyet-500 disabled:opacity-50">
              {isNew ? "Taslak oluştur" : "Taslağı kaydet"}
            </button>
          )}
          {!isNew && !readOnly && canEdit && (
            <button onClick={submit} disabled={busy}
              className="flex-1 rounded-md bg-emniyet-500 px-4 py-3 text-sm font-medium text-beton-950 hover:brightness-110 disabled:opacity-50">
              Gönder
            </button>
          )}
          {readOnly && canEdit && (
            <button onClick={revise} disabled={busy}
              className="flex-1 rounded-md bg-emniyet-500 px-4 py-3 text-sm font-medium text-beton-950 hover:brightness-110 disabled:opacity-50">
              Revizyon aç
            </button>
          )}
        </div>
      </div>
    </div>
  );
}

function upd<T>(arr: T[], i: number, patch: Partial<T>): T[] {
  return arr.map((x, j) => (j === i ? { ...x, ...patch } : x));
}

// Kapak fotoğrafı — taslak (kaydedilmiş) ve gönderilmiş raporda aynı kutu.
// reportId yoksa (yeni, henüz kaydedilmemiş taslak) yükleme devre dışıdır.
function CoverPhotoBox({ pid, reportId, coverPhotoFileId, canEdit, onUploaded }: {
  pid?: string;
  reportId?: string;
  coverPhotoFileId?: string;
  canEdit: boolean;
  onUploaded?: () => void;
}) {
  const [photoSrc, setPhotoSrc] = useState<string | null>(null);
  const [photoUploading, setPhotoUploading] = useState(false);
  const [photoErr, setPhotoErr] = useState<string | null>(null);
  const fileRef = useRef<HTMLInputElement>(null);
  const prevFileID = useRef<string | undefined>(undefined);

  useEffect(() => {
    if (!pid || !reportId || !coverPhotoFileId) { setPhotoSrc(null); return; }
    if (coverPhotoFileId === prevFileID.current) return;
    prevFileID.current = coverPhotoFileId;
    apiFetchBlob(`/projects/${pid}/daily-reports/${reportId}/cover-photo`)
      .then(setPhotoSrc)
      .catch(() => setPhotoSrc(null));
  }, [pid, reportId, coverPhotoFileId]);

  async function handlePhotoFile(file: File) {
    if (!pid || !reportId) return;
    if (file.size > 8 * 1024 * 1024) { setPhotoErr("Fotoğraf 8 MB sınırını aşıyor."); return; }
    setPhotoUploading(true);
    setPhotoErr(null);
    try {
      const dataUrl = await new Promise<string>((res, rej) => {
        const r = new FileReader();
        r.onload = () => res(r.result as string);
        r.onerror = rej;
        r.readAsDataURL(file);
      });
      await api(`/projects/${pid}/daily-reports/${reportId}/cover-photo`, {
        method: "POST",
        body: { photo: dataUrl },
        projectId: pid,
      });
      setPhotoSrc(dataUrl);
      onUploaded?.();
    } catch {
      setPhotoErr("Fotoğraf yüklenemedi.");
    } finally {
      setPhotoUploading(false);
    }
  }

  const clickable = canEdit && !!reportId && !photoUploading;

  return (
    <div className="shrink-0">
      <div
        className={`relative rounded-md overflow-hidden border border-dashed border-beton-700 bg-beton-950 flex flex-col items-center justify-center gap-1 transition-colors ${clickable ? "cursor-pointer hover:border-emniyet-500" : ""}`}
        style={{ width: "120px", height: "192px" }}
        onClick={() => clickable && fileRef.current?.click()}
        title={!reportId ? "Fotoğraf eklemeden önce taslağı kaydedin" : canEdit ? "Fotoğraf ekle / değiştir" : "Kapak fotoğrafı"}
      >
        {photoSrc ? (
          <img src={photoSrc} alt="Kapak fotoğrafı" className="w-full h-full object-cover" />
        ) : (
          <>
            <span className="text-2xl text-beton-600">📷</span>
            {canEdit && (
              <span className="text-[10px] text-beton-500 text-center px-1 leading-tight">
                {photoUploading ? "Yükleniyor..." : reportId ? "Fotoğraf ekle" : "Önce kaydedin"}
              </span>
            )}
          </>
        )}
        {canEdit && photoSrc && reportId && (
          <div className="absolute inset-0 bg-black/40 opacity-0 hover:opacity-100 transition-opacity flex items-center justify-center">
            <span className="text-[11px] text-white font-medium">Değiştir</span>
          </div>
        )}
      </div>
      <input
        ref={fileRef}
        type="file"
        accept="image/*"
        className="hidden no-print"
        onChange={(e) => { const f = e.target.files?.[0]; if (f) handlePhotoFile(f); e.target.value = ""; }}
      />
      {photoErr && <p className="mt-1.5 text-xs text-red-400 max-w-[120px]">{photoErr}</p>}
    </div>
  );
}

// ── Read-only card view (Submitted reports) ────────────────────────────────

function weatherIcon(condition?: string): string {
  if (!condition) return "🌡️";
  const c = condition.toLowerCase();
  if (c.includes("fırtına")) return "⛈️";
  if (c.includes("yağmur") || c.includes("yağış")) return "🌧️";
  if (c.includes("kar")) return "❄️";
  if (c.includes("parçalı") || c.includes("bulut")) return "⛅";
  if (c.includes("açık") || c.includes("güneş")) return "☀️";
  if (c.includes("sisli") || c.includes("sis")) return "🌫️";
  return "🌡️";
}

function SecHeader({ color, title, count }: { color: string; title: string; count?: number }) {
  return (
    <div className="flex items-center gap-2.5 px-4 py-2.5 border-b border-beton-800">
      <div className="w-[3px] h-3.5 rounded shrink-0" style={{ background: color }} />
      <span className="text-[11px] font-bold uppercase tracking-widest text-beton-100">{title}</span>
      {count != null && (
        <span className="ml-auto text-[10.5px] text-beton-400">{count} kayıt</span>
      )}
    </div>
  );
}

const roTh = "text-left text-[10px] font-bold uppercase tracking-wider text-beton-400 pb-2 pr-3 whitespace-nowrap";
const roTd = "py-2 pr-3 text-[12.5px] text-beton-100 border-b border-beton-800/60 align-middle";
const roTdM = "py-2 pr-3 text-[12.5px] text-beton-400 border-b border-beton-800/60 align-middle";

type TutanakMin = { id: string; tip: string; baslik: string; tarih: string; tutar?: number; birim?: string };

function daysBetween(a: string, b: string): number {
  return Math.round((new Date(b).getTime() - new Date(a).getTime()) / 86400000);
}

function ReadOnlyView({
  detail, busy, canEdit, err, ctx, onRevise, project, pid,
}: {
  detail: Detail;
  busy: boolean;
  canEdit: boolean;
  err: string | null;
  ctx: DailyCtx | null;
  onRevise: () => void;
  project?: { start_date?: string; end_date?: string } | null;
  pid?: string;
}) {
  const [showPdfPreview, setShowPdfPreview] = useState(false);
  const totalPersonnel = (detail.manpower ?? []).reduce((s, m) => s + m.headcount, 0);

  // Proje günü hesabı
  const dayNo = project?.start_date
    ? daysBetween(project.start_date, detail.report_date) + 1
    : null;
  const remaining = project?.end_date
    ? daysBetween(detail.report_date, project.end_date)
    : null;

  // Tutanaklar (localStorage)
  const tutanaklar: TutanakMin[] = (() => {
    if (!pid) return [];
    try {
      const all: TutanakMin[] = JSON.parse(localStorage.getItem(`ipks_saha_tutanaklar_${pid}`) || "[]");
      return all.filter((t) => t.tarih === detail.report_date);
    } catch { return []; }
  })();

  return (
    <div className="max-w-3xl mx-auto pb-24 space-y-3">

      {/* Header + kapak fotoğrafı yan yana */}
      <div className="rounded-lg border border-beton-800 bg-beton-900 p-4">
        <div className="flex gap-3">
          {/* Sol: başlık bilgileri */}
          <div className="flex-1 min-w-0">
            <div className="flex items-start justify-between gap-2 mb-2">
              <div>
                <h1 className="font-display font-extrabold text-xl text-white leading-tight">
                  Günlük Saha Raporu
                  {detail.revision_no > 1 && (
                    <span className="ml-2 text-sm font-normal text-emniyet-500">rev {detail.revision_no}</span>
                  )}
                </h1>
                <p className="text-sm text-beton-400 mt-0.5">{formatDateTR(detail.report_date)}</p>
              </div>
              <div className="flex items-center gap-2 shrink-0">
                <button onClick={() => setShowPdfPreview(true)}
                  className="no-print rounded-md border border-beton-700 px-2.5 py-1 text-xs text-beton-300 hover:border-emniyet-500 transition-colors">
                  PDF Rapor Üret
                </button>
                <span className={`rounded-full border px-2.5 py-0.5 text-xs font-semibold ${DR_STATUS_STYLE[detail.status]}`}>
                  {DR_STATUS_LABEL[detail.status]}
                </span>
              </div>
            </div>
            <PdfPreviewModal
              open={showPdfPreview}
              onClose={() => setShowPdfPreview(false)}
              title={`Günlük Rapor — ${formatDateTR(detail.report_date)}`}
              fetchPath={`/projects/${pid}/daily-reports/${detail.id}/pdf`}
              downloadName={`gunluk-rapor-${detail.report_date}.pdf`}
            />

            <div className="flex flex-wrap gap-x-5 gap-y-1.5">
              <div>
                <p className="text-[10px] font-bold uppercase tracking-wider text-beton-500 mb-0.5">Hazırlayan</p>
                <p className="text-sm text-beton-100">{detail.author_name}</p>
              </div>
              <div>
                <p className="text-[10px] font-bold uppercase tracking-wider text-beton-500 mb-0.5">Toplam Personel</p>
                <p className="text-sm text-beton-100 tabular-nums">{totalPersonnel} kişi</p>
              </div>
              {dayNo != null && (
                <div>
                  <p className="text-[10px] font-bold uppercase tracking-wider text-beton-500 mb-0.5">Proje Günü</p>
                  <p className="text-sm text-beton-100 tabular-nums">{dayNo}. gün</p>
                </div>
              )}
              {remaining != null && (
                <div>
                  <p className="text-[10px] font-bold uppercase tracking-wider text-beton-500 mb-0.5">Kalan Gün</p>
                  <p className={`text-sm tabular-nums font-semibold ${remaining < 0 ? "text-red-400" : remaining < 14 ? "text-amber-400" : "text-beton-100"}`}>
                    {remaining < 0 ? `${Math.abs(remaining)} gün gecikme` : `${remaining} gün`}
                  </p>
                </div>
              )}
            </div>

            {/* Hava durumu — kompakt satır */}
            {(detail.weather?.condition || detail.temperature_min != null || detail.temperature_max != null) && (
              <div className="mt-2.5 flex items-center gap-2 rounded-md bg-beton-950 border border-beton-800 px-3 py-2">
                {detail.weather?.condition && (
                  <>
                    <span className="text-lg leading-none">{weatherIcon(detail.weather.condition)}</span>
                    <span className="text-sm text-beton-200">{detail.weather.condition}</span>
                  </>
                )}
                {(detail.temperature_min != null || detail.temperature_max != null) && (
                  <span className="ml-auto text-sm text-beton-100 tabular-nums font-medium">
                    {detail.temperature_min != null ? `${detail.temperature_min}°` : "–"}
                    {" / "}
                    {detail.temperature_max != null ? `${detail.temperature_max}°` : "–"}
                    {" C"}
                  </span>
                )}
              </div>
            )}
          </div>

          {/* Sağ: kapak fotoğrafı (5cm × 8cm ≈ w-[120px] h-[192px]) */}
          <CoverPhotoBox pid={pid} reportId={detail.id} coverPhotoFileId={detail.cover_photo_file_id} canEdit={canEdit} />
        </div>
      </div>

      {err && <p className="text-red-400 text-sm">{err}</p>}

      {/* Şantiye Mevcudu */}
      <div className="rounded-lg border border-beton-800 bg-beton-900 overflow-hidden">
        <SecHeader color="#22c55e" title="Şantiye Mevcudu" count={totalPersonnel} />
        {(detail.manpower ?? []).length === 0 ? (
          <p className="px-4 py-3 text-[12px] text-beton-500 italic">Bu tarih için personel girişi yapılmamış.</p>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full min-w-[400px]">
              <thead>
                <tr>
                  <th className={`${roTh} pl-4`}>Taşeron</th>
                  <th className={roTh}>Branş / Meslek</th>
                  <th className={`${roTh} text-right pr-4`}>Kişi</th>
                </tr>
              </thead>
              <tbody>
                {(detail.manpower ?? []).map((m, i) => (
                  <tr key={i} className="hover:bg-beton-800/30 transition-colors">
                    <td className={`${roTd} pl-4`}>{m.subcontractor_name ?? "Ana Yüklenici"}</td>
                    <td className={roTdM}>{m.trade || "—"}</td>
                    <td className={`${roTd} text-right font-semibold pr-4 tabular-nums`}>{m.headcount}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {/* Günlük İmalat */}
      <div className="rounded-lg border border-beton-800 bg-beton-900 overflow-hidden">
        <SecHeader color="#3b7fd4" title="Günlük İmalat" count={(detail.work_entries ?? []).length} />
        {(detail.work_entries ?? []).length === 0 ? (
          <p className="px-4 py-3 text-[12px] text-beton-500 italic">Bu tarih için imalat kalemi girilmemiş.</p>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full min-w-[500px]">
              <thead>
                <tr>
                  <th className={`${roTh} pl-4`}>Poz No</th>
                  <th className={roTh}>Açıklama</th>
                  <th className={roTh}>Konum</th>
                  <th className={`${roTh} text-right`}>Miktar</th>
                  <th className={`${roTh} pr-4`}>Birim</th>
                </tr>
              </thead>
              <tbody>
                {(detail.work_entries ?? []).map((w, i) => (
                  <tr key={i} className="hover:bg-beton-800/30 transition-colors">
                    <td className={`${roTdM} pl-4 tabular-nums`}>{w.work_item_poz ?? "—"}</td>
                    <td className={roTd}>{w.description}</td>
                    <td className={roTdM}>{w.location ?? "—"}</td>
                    <td className={`${roTd} text-right tabular-nums`}>{w.qty != null ? w.qty : "—"}</td>
                    <td className={`${roTdM} pr-4`}>{w.unit ?? "—"}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {/* Makine & Ekipman */}
      <div className="rounded-lg border border-beton-800 bg-beton-900 overflow-hidden">
        <SecHeader color="#f59e0b" title="Makine & Ekipman" count={(detail.equipment ?? []).length} />
        {(detail.equipment ?? []).length === 0 ? (
          <p className="px-4 py-3 text-[12px] text-beton-500 italic">Bu tarih için makine/ekipman girilmemiş.</p>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full min-w-[420px]">
              <thead>
                <tr>
                  <th className={`${roTh} pl-4`}>Ekipman</th>
                  <th className={`${roTh} text-right`}>Adet</th>
                  <th className={`${roTh} text-right`}>Çalışma (sa)</th>
                  <th className={`${roTh} pr-4`}>Bekleme Nedeni</th>
                </tr>
              </thead>
              <tbody>
                {(detail.equipment ?? []).map((e, i) => (
                  <tr key={i} className="hover:bg-beton-800/30 transition-colors">
                    <td className={`${roTd} pl-4`}>{e.equipment_name}</td>
                    <td className={`${roTd} text-right tabular-nums`}>{e.count}</td>
                    <td className={`${roTd} text-right tabular-nums`}>{e.working_hours ?? "—"}</td>
                    <td className={`${roTdM} pr-4`}>{e.idle_reason ?? "—"}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {/* Şantiye Kasa Harcaması */}
      <div className="rounded-lg border border-beton-800 bg-beton-900 overflow-hidden">
        <SecHeader color="#eab308" title="Şantiye Kasa Harcaması" count={(detail.cash_expenses ?? []).length} />
        {(detail.cash_expenses ?? []).length === 0 ? (
          <p className="px-4 py-3 text-[12px] text-beton-500 italic">Bu tarih için kasa harcaması girilmemiş.</p>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full min-w-[460px]">
              <thead>
                <tr>
                  <th className={`${roTh} pl-4`}>Açıklama</th>
                  <th className={roTh}>Kategori</th>
                  <th className={`${roTh} text-right`}>Tutar</th>
                  <th className={`${roTh} pr-4`}>Makbuz No</th>
                </tr>
              </thead>
              <tbody>
                {(detail.cash_expenses ?? []).map((c, i) => (
                  <tr key={i} className="hover:bg-beton-800/30 transition-colors">
                    <td className={`${roTd} pl-4`}>{c.description}</td>
                    <td className={roTdM}>{c.category}</td>
                    <td className={`${roTd} text-right tabular-nums font-semibold`}>
                      {c.amount.toLocaleString("tr-TR", { minimumFractionDigits: 2, maximumFractionDigits: 2 })} ₺
                    </td>
                    <td className={`${roTdM} pr-4`}>{c.receipt_no ?? "—"}</td>
                  </tr>
                ))}
                <tr className="bg-beton-800/20">
                  <td className={`${roTd} pl-4 font-bold text-beton-100`} colSpan={2}>Toplam</td>
                  <td className={`${roTd} text-right tabular-nums font-bold text-white`}>
                    {(detail.cash_expenses ?? []).reduce((s, c) => s + c.amount, 0)
                      .toLocaleString("tr-TR", { minimumFractionDigits: 2, maximumFractionDigits: 2 })} ₺
                  </td>
                  <td className={`${roTdM} pr-4`}></td>
                </tr>
              </tbody>
            </table>
          </div>
        )}
      </div>

      {/* Notlar */}
      {detail.notes && (
        <div className="rounded-lg border border-beton-800 bg-beton-900 overflow-hidden">
          <SecHeader color="#4e6a87" title="Notlar & Açıklamalar" />
          <p className="px-4 py-3 text-sm text-beton-300 leading-relaxed">{detail.notes}</p>
        </div>
      )}

      {/* Tutanaklar (varsa) */}
      {tutanaklar.length > 0 && (
        <div className="rounded-lg border border-beton-800 bg-beton-900 overflow-hidden">
          <SecHeader color="#a855f7" title="Tutanaklar" count={tutanaklar.length} />
          <div className="divide-y divide-beton-800/60">
            {tutanaklar.map((t) => (
              <div key={t.id} className="px-4 py-2.5 flex items-center gap-3">
                <span className="text-[10px] font-bold uppercase tracking-wider text-beton-500 shrink-0 w-20 truncate">{t.tip}</span>
                <span className="flex-1 text-[12.5px] text-beton-100 truncate">{t.baslik}</span>
                {t.tutar != null && (
                  <span className="text-[12px] text-beton-300 tabular-nums shrink-0">
                    {t.tutar.toLocaleString("tr-TR")} {t.birim ?? ""}
                  </span>
                )}
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Depo-Stok Özeti */}
      {ctx && (
        <div className="rounded-lg border border-beton-800 bg-beton-900 overflow-hidden">
          <SecHeader color="#f59e0b" title="Depo-Stok Özeti" />
          {ctx.warehouse_delta.length === 0 ? (
            <p className="px-4 py-3 text-[12px] text-beton-500 italic">Bu tarih için depo hareketi yok.</p>
          ) : (
            <div className="px-4 py-3 space-y-2">
              {ctx.warehouse_delta.map((wh, i) => (
                <div key={i} className="flex items-center gap-2 flex-wrap text-[12.5px]">
                  <span className="flex-1 min-w-0 text-beton-100">{wh.malzeme_adi}</span>
                  <span className="text-beton-500 text-[11px]">{wh.kategori}</span>
                  {wh.giris > 0 && <span className="text-green-400 tabular-nums">+{wh.giris} {wh.birim}</span>}
                  {wh.cikis > 0 && <span className="text-red-400 tabular-nums">−{wh.cikis} {wh.birim}</span>}
                  <span className={`font-semibold tabular-nums ${wh.net_delta >= 0 ? "text-green-300" : "text-red-300"}`}>
                    Net: {wh.net_delta >= 0 ? "+" : ""}{wh.net_delta} {wh.birim}
                  </span>
                </div>
              ))}
            </div>
          )}
        </div>
      )}

      {/* Bekleyen Aktiviteler */}
      {ctx && (ctx.pending_mars > 0 || ctx.pending_pos > 0 || ctx.open_tasks > 0) && (
        <div className="rounded-lg border border-beton-800 bg-beton-900 overflow-hidden">
          <SecHeader color="#4e6a87" title="Bekleyen Aktiviteler" />
          <div className="px-4 py-3 flex flex-wrap gap-3">
            {ctx.pending_mars > 0 && (
              <div className="rounded-md border border-beton-700 bg-beton-950 px-3 py-2 text-center min-w-[90px]">
                <div className="text-lg font-bold text-beton-100 tabular-nums">{ctx.pending_mars}</div>
                <div className="text-[10px] uppercase tracking-wider text-beton-500 mt-0.5">Bekleyen MAR</div>
              </div>
            )}
            {ctx.pending_pos > 0 && (
              <div className="rounded-md border border-beton-700 bg-beton-950 px-3 py-2 text-center min-w-[90px]">
                <div className="text-lg font-bold text-beton-100 tabular-nums">{ctx.pending_pos}</div>
                <div className="text-[10px] uppercase tracking-wider text-beton-500 mt-0.5">Açık Sipariş</div>
              </div>
            )}
            {ctx.open_tasks > 0 && (
              <div className="rounded-md border border-beton-700 bg-beton-950 px-3 py-2 text-center min-w-[90px]">
                <div className="text-lg font-bold text-beton-100 tabular-nums">{ctx.open_tasks}</div>
                <div className="text-[10px] uppercase tracking-wider text-beton-500 mt-0.5">Açık Görev</div>
              </div>
            )}
          </div>
        </div>
      )}

      {/* Action bar */}
      <div className="no-print fixed bottom-0 inset-x-0 border-t border-beton-800 bg-beton-900/95 backdrop-blur p-3">
        <div className="max-w-3xl mx-auto flex gap-2">
          <p className="flex-1 text-xs text-beton-500 self-center">
            Gönderilmiş rapor değiştirilemez. Düzeltme gerekiyorsa yeni revizyon açın.
          </p>
          {canEdit && (
            <button
              onClick={onRevise}
              disabled={busy}
              className="rounded-md bg-emniyet-500 px-5 py-2.5 text-sm font-medium text-beton-950 hover:brightness-110 disabled:opacity-50"
            >
              Revizyon aç
            </button>
          )}
        </div>
      </div>
    </div>
  );
}
