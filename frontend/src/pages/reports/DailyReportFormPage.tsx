import { useCallback, useEffect, useState } from "react";
import { useNavigate, useParams } from "react-router-dom";
import { api, RequestError } from "../../api/client";
import { useAuth } from "../../auth/AuthContext";
import { useProjects } from "../../projects/ProjectContext";
import { apiWithOfflineFallback } from "../../offline/queue";
import { DR_STATUS_LABEL, DR_STATUS_STYLE, formatDateTR } from "./DailyReportsPage";

// Faz 6 — Günlük rapor formu (mobil öncelikli: tek kolon, büyük dokunma
// hedefleri, bölüm bölüm satır ekleme). Aynı bileşen üç modda çalışır:
//   /saha-raporlari/yeni      → yeni taslak
//   /saha-raporlari/:id       → görüntüle (Submitted) ya da taslak düzenle
// Submitted raporda form salt okunur; "Revizyon aç" yeni taslak açar.
// Çevrimdışıyken kaydet/gönder localStorage kuyruğuna düşer (offline/queue.ts).

type Manpower = { subcontractor_id?: string; trade: string; headcount: number };
type Equipment = { equipment_name: string; count: number; working_hours?: number; idle_reason?: string };
type WorkEntry = { work_item_id?: string; location?: string; description: string; qty?: number; unit?: string };

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
  manpower?: (Manpower & { subcontractor_name?: string })[];
  equipment?: Equipment[];
  work_entries?: (WorkEntry & { work_item_poz?: string })[];
};

type Sub = { id: string; company_name: string };
type WorkItem = { id: string; poz_no: string; description: string; unit: string };

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

  // Form state
  const [reportDate, setReportDate] = useState(todayISO());
  const [condition, setCondition] = useState("");
  const [tempMin, setTempMin] = useState("");
  const [tempMax, setTempMax] = useState("");
  const [notes, setNotes] = useState("");
  const [manpower, setManpower] = useState<Manpower[]>([]);
  const [equipment, setEquipment] = useState<Equipment[]>([]);
  const [entries, setEntries] = useState<WorkEntry[]>([]);

  const [subs, setSubs] = useState<Sub[]>([]);
  const [items, setItems] = useState<WorkItem[]>([]);

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
    } catch {
      setErr("Rapor yüklenemedi ya da erişim yetkiniz yok.");
    }
  }, [pid, id, isNew]);

  useEffect(() => {
    load();
  }, [load]);

  // Referans listeleri (çevrimdışıysa boş kalır; serbest metin girişi yeter).
  useEffect(() => {
    if (!pid) return;
    api<{ subcontractors: Sub[] }>(`/projects/${pid}/subcontractors`, { projectId: pid })
      .then((r) => setSubs(r.subcontractors))
      .catch(() => {});
  }, [pid]);
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

  const input =
    "w-full rounded-md bg-beton-950 border border-beton-800 px-3 py-2 text-sm text-beton-100 outline-none focus:border-emniyet-500 disabled:opacity-60";
  const label = "block text-xs text-beton-400 mb-1";
  const section = "rounded-lg border border-beton-800 bg-beton-900 p-4 space-y-3";
  const addBtn =
    "rounded-md border border-dashed border-beton-700 px-3 py-2 text-xs text-beton-300 hover:border-emniyet-500 w-full";
  const rmBtn = "shrink-0 self-start rounded border border-beton-700 px-2 py-1 text-xs text-beton-400 hover:border-red-400";

  return (
    <div className="max-w-2xl mx-auto space-y-4 pb-24">
      <div className="flex items-center justify-between gap-2 flex-wrap">
        <h1 className="font-display font-extrabold text-xl text-white">
          {isNew ? "Yeni Günlük Rapor" : `Günlük Rapor — ${formatDateTR(reportDate)}`}
          {detail && detail.revision_no > 1 && (
            <span className="ml-2 text-sm text-emniyet-500">rev {detail.revision_no}</span>
          )}
        </h1>
        {detail && (
          <span className={`rounded-full border px-2 py-0.5 text-xs ${DR_STATUS_STYLE[detail.status]}`}>
            {DR_STATUS_LABEL[detail.status]}
          </span>
        )}
      </div>

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
                <input className={input} placeholder="Kule vinç…" value={e.equipment_name} disabled={readOnly || !canEdit}
                  onChange={(ev) => setEquipment(upd(equipment, i, { equipment_name: ev.target.value }))} />
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

      {/* Notlar */}
      <div className={section}>
        <label className={label}>Genel notlar</label>
        <textarea rows={3} className={input} value={notes} disabled={readOnly || !canEdit}
          onChange={(e) => setNotes(e.target.value)} />
      </div>

      {/* Aksiyon çubuğu (mobilde alta sabit) */}
      <div className="fixed bottom-0 inset-x-0 border-t border-beton-800 bg-beton-900/95 backdrop-blur p-3">
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
