import { useCallback, useEffect, useMemo, useState } from "react";
import { api } from "../../api/client";
import { useProjects } from "../ProjectContext";

// ── Tipler ────────────────────────────────────────────────────────────────────
type Personel = {
  id?: string;
  ad_soyad: string;
  gorev: string;
  firma: string;
  is_aktif: boolean;
  sira: number;
};

type PuantajEntry = {
  id?: string;
  personel_id: string;
  tarih: string;
  durum: string;
  mesai_saat: number;
  not_metin: string;
};

// ── Sabitler ─────────────────────────────────────────────────────────────────
const GOREVLER = ["İşçi", "Formen", "Teknisyen", "Mühendis", "Şef", "Yönetici", "Alt Yüklenici"];

const DURUM_META: Record<string, { label: string; short: string; cls: string; cellCls: string }> = {
  mevcut:   { label: "Mevcut",   short: "M", cls: "bg-green-900/60 text-green-300",  cellCls: "bg-green-900/40 text-green-300 font-bold" },
  devamsiz: { label: "Devamsız", short: "D", cls: "bg-red-900/60 text-red-300",      cellCls: "bg-red-900/40 text-red-300 font-bold" },
  izinli:   { label: "İzinli",   short: "İ", cls: "bg-yellow-900/60 text-yellow-300",cellCls: "bg-yellow-900/40 text-yellow-300 font-bold" },
  raporlu:  { label: "Raporlu",  short: "R", cls: "bg-blue-900/60 text-blue-300",    cellCls: "bg-blue-900/40 text-blue-300 font-bold" },
};
const DURUM_SIRA = ["mevcut", "devamsiz", "izinli", "raporlu"];

const inpBase =
  "rounded bg-beton-950 border border-beton-800 px-2 py-1 text-sm text-beton-200 " +
  "outline-none focus:border-emniyet-500 disabled:opacity-50";

// ── Hafta yardımcıları ────────────────────────────────────────────────────────
function weekStart(d: Date): Date {
  const day = d.getDay(); // 0=Sun
  const diff = day === 0 ? -6 : 1 - day;
  const mon = new Date(d);
  mon.setDate(d.getDate() + diff);
  mon.setHours(0, 0, 0, 0);
  return mon;
}

function addDays(d: Date, n: number): Date {
  const r = new Date(d);
  r.setDate(r.getDate() + n);
  return r;
}

function toISO(d: Date): string {
  return d.toISOString().slice(0, 10);
}

function weekDays(mon: Date): Date[] {
  return Array.from({ length: 7 }, (_, i) => addDays(mon, i));
}

const TR_DAYS = ["Pzt", "Sal", "Çar", "Per", "Cum", "Cmt", "Paz"];
const TR_MONTHS = ["Oca", "Şub", "Mar", "Nis", "May", "Haz", "Tem", "Ağu", "Eyl", "Eki", "Kas", "Ara"];

function fmtDay(d: Date): string {
  return `${d.getDate()} ${TR_MONTHS[d.getMonth()]}`;
}

function fmtWeekRange(mon: Date): string {
  const sun = addDays(mon, 6);
  return `${fmtDay(mon)} – ${fmtDay(sun)} ${sun.getFullYear()}`;
}

// ── Ana Bileşen ───────────────────────────────────────────────────────────────
export default function PersonelPuantajPage() {
  const { current: currentProject } = useProjects();
  const pid = currentProject?.id;

  const [tab, setTab] = useState<"puantaj" | "personel">("puantaj");
  const [weekMon, setWeekMon] = useState<Date>(() => weekStart(new Date()));

  // Personel
  const [personelList, setPersonelList] = useState<Personel[]>([]);
  const [personelLoading, setPersonelLoading] = useState(false);

  // Puantaj
  const [puantajMap, setPuantajMap] = useState<Map<string, PuantajEntry>>(new Map());
  const [puantajLoading, setPuantajLoading] = useState(false);
  const [saving, setSaving] = useState<string | null>(null);
  const [puantajErr, setPuantajErr] = useState<string | null>(null);
  const [submittedDates, setSubmittedDates] = useState<Set<string>>(new Set());

  // Personel form
  const [addingPersonel, setAddingPersonel] = useState(false);
  const [editingPersonel, setEditingPersonel] = useState<Personel | null>(null);
  const [personelForm, setPersonelForm] = useState<Personel>({ ad_soyad: "", gorev: "İşçi", firma: "", is_aktif: true, sira: 0 });

  const days = useMemo(() => weekDays(weekMon), [weekMon]);
  const fromISO = toISO(days[0]);
  const toISO2 = toISO(days[6]);

  // Düzenleme penceresi: sadece bugün ve dün (backend'deki kuralla aynı).
  // toISO(), UTC dönüşümü yaptığından pozitif UTC ofsetli dilimlerde (ör.
  // Türkiye) yerel gece yarısını bir gün geriye kaydırır — days[] dizisiyle
  // aynı sapmayı taşımak için "bugün"ü de weekStart ile aynı yöntemle
  // (yerel saat sıfırlanmış bir Date) hesaplıyoruz, ham `new Date()` değil.
  const startOfToday = useMemo(() => { const d = new Date(); d.setHours(0, 0, 0, 0); return d; }, []);
  const todayISO = toISO(startOfToday);
  const yesterdayISO = toISO(addDays(startOfToday, -1));
  const isLocked = useCallback(
    (tarih: string) => (tarih !== todayISO && tarih !== yesterdayISO) || submittedDates.has(tarih),
    [todayISO, yesterdayISO, submittedDates]
  );
  const lockReason = useCallback(
    (tarih: string) => {
      if (submittedDates.has(tarih)) {
        return "İlgili günlük rapor basıldı, geçmişe dönük veri değişikliği yapmak için yöneticinize başvurun.";
      }
      if (tarih !== todayISO && tarih !== yesterdayISO) {
        return "Sadece bugün ve dün için puantaj girişi yapılabilir.";
      }
      return "";
    },
    [todayISO, yesterdayISO, submittedDates]
  );

  // ── Yükle ──────────────────────────────────────────────────────────────────
  const loadPersonel = useCallback(async () => {
    if (!pid) return;
    setPersonelLoading(true);
    try {
      const data = await api<{ personnel: Personel[] }>(`/projects/${pid}/personnel`, { projectId: pid });
      setPersonelList(data.personnel ?? []);
    } finally {
      setPersonelLoading(false);
    }
  }, [pid]);

  const loadPuantaj = useCallback(async () => {
    if (!pid) return;
    setPuantajLoading(true);
    try {
      const data = await api<{ entries: PuantajEntry[] }>(
        `/projects/${pid}/puantaj?from=${fromISO}&to=${toISO2}`,
        { projectId: pid }
      );
      const m = new Map<string, PuantajEntry>();
      for (const e of data.entries ?? []) {
        m.set(`${e.personel_id}|${e.tarih}`, e);
      }
      setPuantajMap(m);
    } finally {
      setPuantajLoading(false);
    }
  }, [pid, fromISO, toISO2]);

  const loadSubmittedDates = useCallback(async () => {
    if (!pid) return;
    try {
      const data = await api<{ daily_reports: { report_date: string; status: string }[] }>(
        `/projects/${pid}/daily-reports?from=${fromISO}&to=${toISO2}`,
        { projectId: pid }
      );
      const s = new Set<string>();
      for (const d of data.daily_reports ?? []) {
        if (d.status === "Submitted") s.add(d.report_date);
      }
      setSubmittedDates(s);
    } catch {
      // sessiz — kilit sadece UI'da öncelikli görünürlük sağlar, gerçek
      // kaynak doğrusu backend'dir (UpsertPuantaj kendi kontrolünü yapar).
    }
  }, [pid, fromISO, toISO2]);

  useEffect(() => { loadPersonel(); }, [loadPersonel]);
  useEffect(() => { loadPuantaj(); }, [loadPuantaj]);
  useEffect(() => { loadSubmittedDates(); }, [loadSubmittedDates]);

  // ── Puantaj kaydı ──────────────────────────────────────────────────────────
  async function toggleDurum(personelId: string, tarih: string) {
    if (!pid || isLocked(tarih)) return;
    const key = `${personelId}|${tarih}`;
    const prevEntry = puantajMap.get(key);
    const curDurum = prevEntry?.durum ?? "mevcut";
    const nextIdx = (DURUM_SIRA.indexOf(curDurum) + 1) % DURUM_SIRA.length;
    const nextDurum = DURUM_SIRA[nextIdx];

    const entry: PuantajEntry = {
      personel_id: personelId,
      tarih,
      durum: nextDurum,
      mesai_saat: prevEntry?.mesai_saat ?? 0,
      not_metin: prevEntry?.not_metin ?? "",
    };

    // Optimistic update
    setPuantajErr(null);
    setPuantajMap((prev) => {
      const next = new Map(prev);
      next.set(key, entry);
      return next;
    });

    setSaving(key);
    try {
      await api(`/projects/${pid}/puantaj`, { method: "POST", body: entry, projectId: pid });
    } catch (e) {
      // Geri al ve backend'in Türkçe hata mesajını göster (kilit backend'de
      // de doğrulanır — UI kilidi atlatılsa bile burada reddedilir).
      setPuantajMap((prev) => {
        const next = new Map(prev);
        if (prevEntry) next.set(key, prevEntry); else next.delete(key);
        return next;
      });
      setPuantajErr(e instanceof Error ? e.message : "Kaydedilemedi.");
    } finally {
      setSaving(null);
    }
  }

  async function setMesai(personelId: string, tarih: string, mesai: number) {
    if (!pid || isLocked(tarih)) return;
    const key = `${personelId}|${tarih}`;
    const prevEntry = puantajMap.get(key);
    const entry: PuantajEntry = {
      personel_id: personelId,
      tarih,
      durum: prevEntry?.durum ?? "mevcut",
      mesai_saat: mesai,
      not_metin: prevEntry?.not_metin ?? "",
    };
    setPuantajErr(null);
    setPuantajMap((prev) => {
      const next = new Map(prev);
      next.set(key, entry);
      return next;
    });
    setSaving(key);
    try {
      await api(`/projects/${pid}/puantaj`, { method: "POST", body: entry, projectId: pid });
    } catch (e) {
      setPuantajMap((prev) => {
        const next = new Map(prev);
        if (prevEntry) next.set(key, prevEntry); else next.delete(key);
        return next;
      });
      setPuantajErr(e instanceof Error ? e.message : "Kaydedilemedi.");
    } finally {
      setSaving(null);
    }
  }

  // ── Personel CRUD ──────────────────────────────────────────────────────────
  function startAddPersonel() {
    setPersonelForm({ ad_soyad: "", gorev: "İşçi", firma: "", is_aktif: true, sira: personelList.length * 10 });
    setEditingPersonel(null);
    setAddingPersonel(true);
  }

  function startEditPersonel(p: Personel) {
    setPersonelForm({ ...p });
    setEditingPersonel(p);
    setAddingPersonel(false);
  }

  async function savePersonel() {
    if (!pid || !personelForm.ad_soyad) return;
    if (editingPersonel?.id) {
      await api(`/projects/${pid}/personnel/${editingPersonel.id}`, { method: "PATCH", body: personelForm, projectId: pid });
    } else {
      await api(`/projects/${pid}/personnel`, { method: "POST", body: personelForm, projectId: pid });
    }
    setAddingPersonel(false);
    setEditingPersonel(null);
    await loadPersonel();
  }

  async function deletePersonel(p: Personel) {
    if (!pid || !p.id) return;
    if (!confirm(`"${p.ad_soyad}" silinecek. Onaylıyor musunuz?`)) return;
    await api(`/projects/${pid}/personnel/${p.id}`, { method: "DELETE", projectId: pid });
    await loadPersonel();
  }

  // ── Hesaplamalar ───────────────────────────────────────────────────────────
  const aktifPersonel = personelList.filter((p) => p.is_aktif);

  function getEntry(personelId: string, tarih: string): PuantajEntry | undefined {
    return puantajMap.get(`${personelId}|${tarih}`);
  }

  function dayTotal(tarih: string) {
    return aktifPersonel.filter((p) => {
      const e = getEntry(p.id!, tarih);
      return !e || e.durum === "mevcut";
    }).length;
  }

  function personelWeekTotal(personelId: string) {
    return days.filter((d) => {
      const e = getEntry(personelId, toISO(d));
      return !e || e.durum === "mevcut";
    }).length;
  }

  // ── Render ─────────────────────────────────────────────────────────────────
  return (
    <div className="p-6 max-w-7xl mx-auto">
      {/* Başlık + Tabs */}
      <div className="flex items-center justify-between mb-5">
        <div>
          <h1 className="text-xl font-semibold text-beton-100">Personel & Puantaj</h1>
          {currentProject && <p className="text-beton-400 text-sm mt-0.5">{currentProject.name}</p>}
        </div>
        <div className="flex gap-1 bg-beton-900 rounded-lg p-1">
          {(["puantaj", "personel"] as const).map((t) => (
            <button
              key={t}
              onClick={() => setTab(t)}
              className={`px-4 py-1.5 rounded-md text-sm font-medium transition-colors ${
                tab === t ? "bg-emniyet-600 text-white" : "text-beton-400 hover:text-beton-200"
              }`}
            >
              {t === "puantaj" ? "Puantaj Cetveli" : "Personel Listesi"}
            </button>
          ))}
        </div>
      </div>

      {/* ── PUANTAJ TAB ── */}
      {tab === "puantaj" && (
        <div>
          {/* Hafta seçici */}
          <div className="flex items-center gap-3 mb-4">
            <button
              onClick={() => setWeekMon((m) => addDays(m, -7))}
              className="p-1.5 rounded-md bg-beton-900 hover:bg-beton-800 text-beton-300 text-sm"
            >
              ‹
            </button>
            <span className="text-beton-200 font-medium text-sm min-w-[200px] text-center">
              {fmtWeekRange(weekMon)}
            </span>
            <button
              onClick={() => setWeekMon((m) => addDays(m, 7))}
              className="p-1.5 rounded-md bg-beton-900 hover:bg-beton-800 text-beton-300 text-sm"
            >
              ›
            </button>
            <button
              onClick={() => setWeekMon(weekStart(new Date()))}
              className="ml-2 px-3 py-1 rounded-md bg-beton-900 hover:bg-beton-800 text-beton-400 text-xs"
            >
              Bu Hafta
            </button>
            {puantajLoading && <span className="text-beton-500 text-xs">Yükleniyor…</span>}
          </div>

          {puantajErr && (
            <p className="mb-3 text-sm text-red-400 rounded-md border border-red-500/30 bg-red-500/10 px-3 py-2">
              {puantajErr}
            </p>
          )}

          {/* Grid */}
          {aktifPersonel.length === 0 ? (
            <p className="text-beton-500 text-sm text-center py-10">
              Aktif personel yok. "Personel Listesi" sekmesinden ekleyebilirsiniz.
            </p>
          ) : (
            <div className="overflow-x-auto rounded-lg border border-beton-800">
              <table className="w-full text-sm border-collapse">
                <thead>
                  <tr className="bg-beton-900 border-b border-beton-800">
                    <th className="py-2 px-3 text-left text-xs text-beton-400 font-medium min-w-[160px]">Ad Soyad</th>
                    <th className="py-2 px-3 text-left text-xs text-beton-500 font-medium w-20">Görev</th>
                    {days.map((d, i) => (
                      <th key={i} className="py-2 px-2 text-center text-xs font-medium w-16">
                        <span className="text-beton-300">{TR_DAYS[i]}</span>
                        <br />
                        <span className="text-beton-500 font-normal">{fmtDay(d)}</span>
                      </th>
                    ))}
                    <th className="py-2 px-3 text-center text-xs text-beton-400 font-medium w-14">Hafta</th>
                  </tr>
                </thead>
                <tbody>
                  {aktifPersonel.map((p) => (
                    <tr key={p.id} className="border-b border-beton-800/50 hover:bg-beton-900/30">
                      <td className="py-2 px-3 text-beton-200 text-xs font-medium">{p.ad_soyad}</td>
                      <td className="py-2 px-3 text-beton-500 text-xs">{p.gorev}</td>
                      {days.map((d, i) => {
                        const iso = toISO(d);
                        const entry = getEntry(p.id!, iso);
                        const durum = entry?.durum ?? "mevcut";
                        const dm = DURUM_META[durum];
                        const key = `${p.id}|${iso}`;
                        const isSaving = saving === key;
                        const locked = isLocked(iso);
                        return (
                          <td key={i} className="py-1 px-1 text-center">
                            <button
                              onClick={() => toggleDurum(p.id!, iso)}
                              disabled={isSaving || locked}
                              title={locked ? lockReason(iso) : dm.label}
                              className={`w-10 h-8 rounded text-xs font-bold transition-colors disabled:opacity-50 ${dm.cellCls} ${locked ? "cursor-not-allowed" : ""}`}
                            >
                              {isSaving ? "…" : locked ? (
                                <svg className="w-3.5 h-3.5 mx-auto opacity-60" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2}>
                                  <rect x="5" y="11" width="14" height="9" rx="1.5" />
                                  <path d="M8 11V7a4 4 0 018 0v4" />
                                </svg>
                              ) : dm.short}
                            </button>
                          </td>
                        );
                      })}
                      <td className="py-2 px-3 text-center">
                        <span className="text-beton-300 text-xs font-medium">
                          {personelWeekTotal(p.id!)}g
                        </span>
                      </td>
                    </tr>
                  ))}
                </tbody>
                <tfoot>
                  <tr className="border-t border-beton-700 bg-beton-900/60">
                    <td className="py-2 px-3 text-xs text-beton-500 font-medium" colSpan={2}>Günlük Mevcut</td>
                    {days.map((d, i) => (
                      <td key={i} className="py-2 px-2 text-center text-xs font-bold text-beton-300">
                        {dayTotal(toISO(d))}
                      </td>
                    ))}
                    <td className="py-2 px-3 text-center text-xs text-beton-400">
                      {aktifPersonel.reduce((s, p) => s + personelWeekTotal(p.id!), 0)}
                    </td>
                  </tr>
                </tfoot>
              </table>
            </div>
          )}

          {/* Durum açıklaması */}
          <div className="flex gap-3 mt-3 flex-wrap">
            {Object.entries(DURUM_META).map(([k, v]) => (
              <span key={k} className={`text-xs px-2 py-0.5 rounded-full ${v.cls}`}>
                {v.short} = {v.label}
              </span>
            ))}
            <span className="text-beton-600 text-xs">· Hücreye tıklayarak döngüsel değiştirin</span>
          </div>
        </div>
      )}

      {/* ── PERSONEL TAB ── */}
      {tab === "personel" && (
        <div>
          <div className="flex items-center justify-between mb-4">
            <p className="text-beton-400 text-sm">
              {personelList.length} personel · {aktifPersonel.length} aktif
            </p>
            <button
              onClick={startAddPersonel}
              className="px-3 py-1.5 bg-emniyet-600 hover:bg-emniyet-500 text-white text-sm rounded-md"
            >
              + Personel Ekle
            </button>
          </div>

          {/* Form */}
          {(addingPersonel || editingPersonel) && (
            <div className="mb-4 p-4 border border-beton-700 rounded-lg bg-beton-900/60">
              <p className="text-beton-300 text-sm font-medium mb-3">
                {editingPersonel ? "Personel Düzenle" : "Yeni Personel"}
              </p>
              <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
                <div className="col-span-2">
                  <label className="text-xs text-beton-500 block mb-1">Ad Soyad *</label>
                  <input
                    className={`${inpBase} w-full`}
                    value={personelForm.ad_soyad}
                    onChange={(e) => setPersonelForm((f) => ({ ...f, ad_soyad: e.target.value }))}
                  />
                </div>
                <div>
                  <label className="text-xs text-beton-500 block mb-1">Görev</label>
                  <select
                    className={`${inpBase} w-full`}
                    value={personelForm.gorev}
                    onChange={(e) => setPersonelForm((f) => ({ ...f, gorev: e.target.value }))}
                  >
                    {GOREVLER.map((g) => <option key={g}>{g}</option>)}
                  </select>
                </div>
                <div>
                  <label className="text-xs text-beton-500 block mb-1">Firma</label>
                  <input
                    className={`${inpBase} w-full`}
                    placeholder="Ana yüklenici"
                    value={personelForm.firma}
                    onChange={(e) => setPersonelForm((f) => ({ ...f, firma: e.target.value }))}
                  />
                </div>
              </div>
              <div className="flex items-center gap-4 mt-3">
                <label className="flex items-center gap-2 text-sm text-beton-300 cursor-pointer">
                  <input
                    type="checkbox"
                    checked={personelForm.is_aktif}
                    onChange={(e) => setPersonelForm((f) => ({ ...f, is_aktif: e.target.checked }))}
                    className="accent-emniyet-500"
                  />
                  Aktif
                </label>
                <button
                  onClick={savePersonel}
                  disabled={!personelForm.ad_soyad}
                  className="px-3 py-1 bg-emniyet-600 hover:bg-emniyet-500 text-white text-sm rounded-md disabled:opacity-40"
                >
                  Kaydet
                </button>
                <button
                  onClick={() => { setAddingPersonel(false); setEditingPersonel(null); }}
                  className="px-3 py-1 text-beton-400 hover:text-beton-200 text-sm"
                >
                  İptal
                </button>
              </div>
            </div>
          )}

          {/* Liste */}
          {personelLoading ? (
            <p className="text-beton-500 text-sm">Yükleniyor…</p>
          ) : personelList.length === 0 ? (
            <p className="text-beton-500 text-sm text-center py-10">Henüz personel eklenmemiş.</p>
          ) : (
            <div className="border border-beton-800 rounded-lg overflow-hidden">
              <table className="w-full text-sm">
                <thead>
                  <tr className="bg-beton-900 border-b border-beton-800">
                    <th className="py-2 px-3 text-left text-xs text-beton-500 font-medium">Ad Soyad</th>
                    <th className="py-2 px-3 text-left text-xs text-beton-500 font-medium w-28">Görev</th>
                    <th className="py-2 px-3 text-left text-xs text-beton-500 font-medium">Firma</th>
                    <th className="py-2 px-3 text-center text-xs text-beton-500 font-medium w-16">Durum</th>
                    <th className="w-24" />
                  </tr>
                </thead>
                <tbody>
                  {personelList.map((p) => (
                    <tr key={p.id} className="border-b border-beton-800/50 hover:bg-beton-900/30 group">
                      <td className="py-2 px-3 text-beton-200 text-sm">{p.ad_soyad}</td>
                      <td className="py-2 px-3 text-beton-400 text-xs">{p.gorev}</td>
                      <td className="py-2 px-3 text-beton-500 text-xs">{p.firma || "—"}</td>
                      <td className="py-2 px-3 text-center">
                        <span className={`text-xs px-1.5 py-0.5 rounded-full ${p.is_aktif ? "bg-green-900/50 text-green-300" : "bg-beton-800 text-beton-500"}`}>
                          {p.is_aktif ? "Aktif" : "Pasif"}
                        </span>
                      </td>
                      <td className="py-2 px-3">
                        <div className="flex gap-2 opacity-0 group-hover:opacity-100 transition-opacity">
                          <button
                            onClick={() => startEditPersonel(p)}
                            className="text-xs text-emniyet-400 hover:text-emniyet-300"
                          >
                            Düzenle
                          </button>
                          <button
                            onClick={() => deletePersonel(p)}
                            className="text-xs text-red-500 hover:text-red-400"
                          >
                            Sil
                          </button>
                        </div>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>
      )}
    </div>
  );
}
