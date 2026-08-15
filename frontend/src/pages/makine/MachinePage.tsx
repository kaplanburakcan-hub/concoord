import { useEffect, useMemo, useState } from "react";
import { api } from "../../api/client";
import { useProjects } from "../ProjectContext";
import { AD_KATALOG, AD_DIGER, photoUrl, trLower, type Tip } from "./ekipmanKatalog";

// ── Types ─────────────────────────────────────────────────────────────────────

type Machine = {
  id: string;
  project_id: string;
  tip: string;
  ad: string;
  plaka?: string;
  marka?: string;
  model?: string;
  seri_no?: string;
  uretim_yili?: number;
  sahiplik: string;
  tedarikci?: string;
  gunluk_ucret?: number;
  durum: string;
  son_bakim_tarihi?: string;
  sonraki_bakim_tarihi?: string;
  aciklama?: string;
};

type MachineLog = {
  id: string;
  machine_id: string;
  tarih: string;
  calisma_miktari: number;
  calisma_birimi: string;
  operator?: string;
  aciklama?: string;
};

interface Props {
  tip: Tip;
  tipLabel: string;
  showPlaka?: boolean;
  showSeriNo?: boolean;
  showBakim?: boolean;
  logBirimi?: "saat" | "km" | "her-ikisi";
}

// ── Helpers ───────────────────────────────────────────────────────────────────

const DURUM_MAP: Record<string, { label: string; cls: string }> = {
  aktif:       { label: "Aktif",       cls: "bg-green-100 text-green-700 dark:bg-green-900/40 dark:text-green-300" },
  bakim:       { label: "Bakımda",     cls: "bg-yellow-100 text-yellow-700 dark:bg-yellow-900/40 dark:text-yellow-300" },
  devre_disi:  { label: "Devre Dışı", cls: "bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-300" },
};

const SAHIPLIK_MAP: Record<string, string> = {
  ozmal:   "Öz mal",
  kiralik: "Kiralık",
};

function fmtDate(s?: string) {
  if (!s) return "—";
  return new Date(s).toLocaleDateString("tr-TR", { day: "2-digit", month: "short", year: "numeric" });
}

function fmtCur(n?: number) {
  if (n == null) return "—";
  return new Intl.NumberFormat("tr-TR", { style: "currency", currency: "TRY", maximumFractionDigits: 0 }).format(n);
}

// Grup başlığı fotoğrafı — statik olarak public/ekipman-foto/ altında
// paketlenir (S3/R2 bağımlılığı yok). Kaynak bulunamazsa (404) ya da
// "Diğer" grubu için jenerik bir yer tutucuya düşer.
function GroupPhoto({ src, label }: { src?: string; label: string }) {
  const [failed, setFailed] = useState(false);
  if (!src || failed) {
    return (
      <div className="w-20 h-16 rounded-lg bg-beton-800 border border-beton-700 flex items-center justify-center shrink-0 text-beton-500 text-xs font-medium">
        {label.slice(0, 2).toUpperCase()}
      </div>
    );
  }
  return (
    <img
      src={src}
      alt={label}
      onError={() => setFailed(true)}
      className="w-20 h-16 rounded-lg object-contain bg-beton-950 shrink-0 border border-beton-700 p-1"
    />
  );
}

// ── Empty form state ──────────────────────────────────────────────────────────

const ANA_YUKLENICI = "Ana Yüklenici";

function emptyMachine(tip: Tip): Partial<Machine> {
  return {
    tip,
    sahiplik: "ozmal",
    tedarikci: ANA_YUKLENICI,
    durum: "aktif",
  };
}

function emptyLog(): Partial<MachineLog> {
  return {
    tarih: new Date().toISOString().slice(0, 10),
    calisma_miktari: 0,
    calisma_birimi: "saat",
  };
}

// ── Main Component ────────────────────────────────────────────────────────────

export default function MachinePage({
  tip,
  tipLabel,
  showPlaka = false,
  showSeriNo = false,
  showBakim = false,
  logBirimi = "saat",
}: Props) {
  const { current } = useProjects();
  const pid = current?.id;

  const [machines, setMachines] = useState<Machine[]>([]);
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  // Form state
  const [formOpen, setFormOpen] = useState(false);
  const [editId, setEditId] = useState<string | null>(null);
  const [form, setForm] = useState<Partial<Machine>>(emptyMachine(tip));
  const [saving, setSaving] = useState(false);
  const [adCustom, setAdCustom] = useState(false);

  // Expanded machine (logs panel)
  const [expandedId, setExpandedId] = useState<string | null>(null);
  const [logs, setLogs] = useState<MachineLog[]>([]);
  const [logsBusy, setLogsBusy] = useState(false);
  const [logForm, setLogForm] = useState<Partial<MachineLog>>(emptyLog());
  const [logSaving, setLogSaving] = useState(false);

  // Filter
  const [filterDurum, setFilterDurum] = useState<string>("all");
  const [search, setSearch] = useState("");
  const [saveError, setSaveError] = useState<string | null>(null);
  const [confirmDeleteId, setConfirmDeleteId] = useState<string | null>(null);
  const [logError, setLogError] = useState<string | null>(null);

  useEffect(() => {
    if (!pid) return;
    loadMachines();
  }, [pid]);

  function loadMachines() {
    if (!pid) return;
    setBusy(true);
    api<{ machines: Machine[] }>(`/projects/${pid}/machines?tip=${tip}`, { projectId: pid })
      .then(r => setMachines(r.machines ?? []))
      .catch(() => setErr("Veriler yüklenemedi."))
      .finally(() => setBusy(false));
  }

  function openAdd(presetAd?: string) {
    setEditId(null);
    setForm(presetAd ? { ...emptyMachine(tip), ad: presetAd } : emptyMachine(tip));
    setAdCustom(false);
    setFormOpen(true);
  }

  function openEdit(m: Machine) {
    setEditId(m.id);
    setForm({ ...m });
    setAdCustom(!!m.ad && !AD_KATALOG[tip].includes(m.ad));
    setFormOpen(true);
  }

  async function saveMachine() {
    if (!pid) return;
    if (!form.ad?.trim()) return;
    setSaving(true);
    setSaveError(null);
    try {
      if (editId) {
        const r = await api<{ machine: Machine }>(`/projects/${pid}/machines/${editId}`, {
          method: "PATCH", body: form, projectId: pid,
        });
        setMachines(prev => prev.map(m => m.id === editId ? r.machine : m));
      } else {
        const r = await api<{ machine: Machine }>(`/projects/${pid}/machines`, {
          method: "POST", body: { ...form, tip }, projectId: pid,
        });
        setMachines(prev => [...prev, r.machine]);
      }
      setFormOpen(false);
    } catch {
      setSaveError("Kaydedilemedi. Lütfen tekrar deneyin.");
    } finally {
      setSaving(false);
    }
  }

  async function deleteMachine(id: string) {
    if (!pid) return;
    await api(`/projects/${pid}/machines/${id}`, { method: "DELETE", projectId: pid });
    setMachines(prev => prev.filter(m => m.id !== id));
    setConfirmDeleteId(null);
    if (expandedId === id) setExpandedId(null);
  }

  function toggleExpand(id: string) {
    if (expandedId === id) {
      setExpandedId(null);
      setLogError(null);
      return;
    }
    setExpandedId(id);
    setLogForm(emptyLog());
    setLogError(null);
    loadLogs(id);
  }

  function loadLogs(machineId: string) {
    if (!pid) return;
    setLogsBusy(true);
    api<{ logs: MachineLog[] }>(`/projects/${pid}/machines/${machineId}/logs`, { projectId: pid })
      .then(r => setLogs(r.logs ?? []))
      .catch(() => {})
      .finally(() => setLogsBusy(false));
  }

  async function addLog() {
    if (!pid || !expandedId) return;
    if ((logForm.calisma_miktari ?? 0) <= 0) return;
    setLogSaving(true);
    try {
      const r = await api<{ log: MachineLog }>(`/projects/${pid}/machines/${expandedId}/logs`, {
        method: "POST", body: logForm, projectId: pid,
      });
      setLogs(prev => [r.log, ...prev]);
      setLogForm(emptyLog());
      setLogError(null);
    } catch {
      setLogError("Puantaj eklenemedi.");
    } finally {
      setLogSaving(false);
    }
  }

  async function deleteLog(logId: string) {
    if (!pid || !expandedId) return;
    await api(`/projects/${pid}/machines/${expandedId}/logs/${logId}`, { method: "DELETE", projectId: pid });
    setLogs(prev => prev.filter(l => l.id !== logId));
  }

  // Filtered list
  const filtered = machines.filter(m => {
    if (filterDurum !== "all" && m.durum !== filterDurum) return false;
    if (search) {
      const q = search.toLowerCase();
      if (
        !m.ad.toLowerCase().includes(q) &&
        !m.marka?.toLowerCase().includes(q) &&
        !m.model?.toLowerCase().includes(q) &&
        !m.plaka?.toLowerCase().includes(q) &&
        !m.seri_no?.toLowerCase().includes(q)
      ) return false;
    }
    return true;
  });

  // KPIs
  const kpiTotal   = machines.length;
  const kpiAktif   = machines.filter(m => m.durum === "aktif").length;
  const kpiBakim   = machines.filter(m => m.durum === "bakim").length;
  const kpiDevre   = machines.filter(m => m.durum === "devre_disi").length;
  const kpiKiralik = machines.filter(m => m.sahiplik === "kiralik").length;

  // Katalog tipine göre gruplama — inşaatta en sık kullanılan tipler
  // (AD_KATALOG) her biri temsili bir fotoğrafla ayrı bir bölüm olarak
  // gösterilir, altında o tipten KAYITLI makineler listelenir. Arama/filtre
  // aktifken boş gruplar gizlenir (arama "bul" amaçlıdır); aksi halde tüm
  // katalog görünür kalır (platform-thinking: boş durumları gizleme).
  const hasActiveFilter = search.trim() !== "" || filterDurum !== "all";
  const groups = useMemo(() => {
    const byAd = new Map<string, Machine[]>();
    for (const m of filtered) {
      const arr = byAd.get(m.ad) ?? [];
      arr.push(m);
      byAd.set(m.ad, arr);
    }
    const catalogGroups = AD_KATALOG[tip]
      .map(ad => ({ key: ad, label: ad, photo: photoUrl(tip, ad), machines: byAd.get(ad) ?? [] }))
      .filter(g => !hasActiveFilter || g.machines.length > 0);
    for (const ad of AD_KATALOG[tip]) byAd.delete(ad);
    const digerMachines = Array.from(byAd.values()).flat();
    const out = [...catalogGroups];
    if (digerMachines.length > 0) {
      out.push({ key: AD_DIGER, label: "Diğer", photo: undefined, machines: digerMachines });
    }
    return out;
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [filtered, tip, hasActiveFilter]);

  if (!pid) return <div className="p-8 text-beton-400">Proje seçilmedi.</div>;
  if (busy)  return <div className="p-8 text-beton-400">Yükleniyor…</div>;
  if (err)   return <div className="p-8 text-red-500">{err}</div>;

  return (
    <div className="p-6 space-y-5 max-w-6xl mx-auto">

      {/* Header */}
      <div className="flex items-center justify-between gap-4 flex-wrap">
        <h1 className="text-xl font-bold text-beton-100">{tipLabel}</h1>
        <button
          onClick={() => openAdd()}
          className="px-4 py-1.5 rounded-lg bg-emniyet-500 hover:bg-emniyet-600 text-beton-950 text-sm font-medium"
        >
          + {tipLabel === "Ekipmanlar" ? "Ekipman" : tipLabel === "İş Makineleri" ? "Makine" : "Araç"} Ekle
        </button>
      </div>

      {/* KPI bar */}
      <div className="grid grid-cols-2 sm:grid-cols-5 gap-3">
        {[
          { label: "Toplam", value: kpiTotal, cls: "text-beton-100" },
          { label: "Aktif", value: kpiAktif, cls: "text-green-600" },
          { label: "Bakımda", value: kpiBakim, cls: "text-yellow-600" },
          { label: "Devre Dışı", value: kpiDevre, cls: "text-red-600" },
          { label: "Kiralık", value: kpiKiralik, cls: "text-blue-600" },
        ].map(k => (
          <div key={k.label} className="rounded-xl border border-beton-800 bg-beton-900 p-3 flex flex-col gap-0.5">
            <span className="text-xs text-beton-400 uppercase tracking-wide">{k.label}</span>
            <span className={`text-2xl font-bold ${k.cls}`}>{k.value}</span>
          </div>
        ))}
      </div>

      {/* Filters */}
      <div className="rounded-lg border border-beton-800 bg-beton-900 p-3 flex gap-2 flex-wrap items-center">
        <input
          placeholder="Ara…"
          value={search}
          onChange={e => setSearch(e.target.value)}
          className="border border-beton-800 bg-beton-950 rounded-lg px-3 py-1.5 text-sm text-beton-100 w-48"
        />
        <select
          value={filterDurum}
          onChange={e => setFilterDurum(e.target.value)}
          className="border border-beton-800 bg-beton-950 rounded-lg px-3 py-1.5 text-sm text-beton-100"
        >
          <option value="all">Tüm Durumlar</option>
          <option value="aktif">Aktif</option>
          <option value="bakim">Bakımda</option>
          <option value="devre_disi">Devre Dışı</option>
        </select>
        <span className="text-xs text-beton-400 ml-1">{filtered.length} kayıt</span>
      </div>

      {/* Machine List — katalog tipine göre gruplanmış (fotoğraf + mevcut kayıtlar) */}
      {groups.length === 0 ? (
        <div className="rounded-xl border border-dashed border-beton-800 p-12 text-center text-beton-400 text-sm">
          {hasActiveFilter
            ? "Aramanızla eşleşen kayıt bulunamadı."
            : `Henüz ${trLower(tipLabel)} tanımlanmamış. "Ekle" butonuyla başlayın.`}
        </div>
      ) : (
        <div className="columns-1 lg:columns-2 gap-3">
          {groups.map(g => (
            <div key={g.key} className="break-inside-avoid mb-3 rounded-xl border border-beton-800 bg-beton-900/60 p-2.5">
              {/* Grup başlığı: fotoğraf + tip adı + kayıt sayısı + hızlı ekle — kompakt tek satır */}
              <div className="flex items-center gap-3">
                <GroupPhoto src={g.photo} label={g.label} />
                <div className="flex-1 min-w-0">
                  <div className="text-sm font-semibold text-beton-100 truncate">{g.label}</div>
                  <div className="text-xs text-beton-500">{g.machines.length} kayıt</div>
                </div>
                {g.key !== AD_DIGER && (
                  <button
                    onClick={() => openAdd(g.label)}
                    className="px-2.5 py-1 rounded-md border border-beton-700 text-xs text-beton-300 hover:border-emniyet-500 hover:text-emniyet-500 shrink-0 self-start"
                  >
                    + Ekle
                  </button>
                )}
              </div>

              {g.machines.length > 0 && (
              <div className="space-y-2 mt-2.5">
              {g.machines.map(m => {
            const durum = DURUM_MAP[m.durum] ?? DURUM_MAP["aktif"];
            const isExpanded = expandedId === m.id;
            return (
              <div key={m.id} className="rounded-xl border border-beton-800 bg-beton-900 overflow-hidden">
                {/* Machine row */}
                <div
                  className="flex items-center gap-3 px-4 py-3 cursor-pointer hover:bg-[var(--bg-hover)] transition-colors"
                  onClick={() => toggleExpand(m.id)}
                >
                  {/* Expand indicator */}
                  <span className="text-beton-400 text-xs select-none">{isExpanded ? "▼" : "▶"}</span>

                  {/* Name + subtitle */}
                  <div className="flex-1 min-w-0">
                    <div className="font-semibold text-sm text-beton-100 truncate">{m.ad}</div>
                    <div className="text-xs text-beton-400 truncate">
                      {[m.marka, m.model].filter(Boolean).join(" ")}
                      {showPlaka && m.plaka ? ` · ${m.plaka}` : ""}
                      {showSeriNo && m.seri_no ? ` · S/N: ${m.seri_no}` : ""}
                      {m.uretim_yili ? ` · ${m.uretim_yili}` : ""}
                    </div>
                  </div>

                  {/* Sahiplik */}
                  <span className="text-xs text-beton-400 hidden sm:block w-20 shrink-0 text-right">
                    {SAHIPLIK_MAP[m.sahiplik] ?? m.sahiplik}
                    {m.gunluk_ucret ? <><br />{fmtCur(m.gunluk_ucret)}/gün</> : ""}
                  </span>

                  {/* Bakım dates */}
                  {showBakim && (
                    <div className="hidden md:block text-xs text-beton-400 w-32 text-right shrink-0">
                      <div>Son: {fmtDate(m.son_bakim_tarihi)}</div>
                      <div>Sıradaki: {fmtDate(m.sonraki_bakim_tarihi)}</div>
                    </div>
                  )}

                  {/* Status badge */}
                  <span className={`text-xs px-2 py-0.5 rounded-full font-medium shrink-0 ${durum.cls}`}>
                    {durum.label}
                  </span>

                  {/* Actions */}
                  <div className="flex gap-1 shrink-0 items-center" onClick={e => e.stopPropagation()}>
                    {confirmDeleteId === m.id ? (
                      <>
                        <span className="text-xs text-red-600 font-medium mr-1">Emin misiniz?</span>
                        <button
                          onClick={() => deleteMachine(m.id)}
                          className="px-2 py-0.5 rounded bg-red-600 hover:bg-red-700 text-white text-xs font-medium"
                        >
                          Sil
                        </button>
                        <button
                          onClick={() => setConfirmDeleteId(null)}
                          className="px-2 py-0.5 rounded border border-beton-800 text-xs text-beton-100 hover:bg-[var(--bg-hover)]"
                        >
                          İptal
                        </button>
                      </>
                    ) : (
                      <>
                        <button
                          onClick={() => openEdit(m)}
                          className="p-1.5 rounded hover:bg-[var(--bg-hover)] text-beton-400 hover:text-beton-100"
                          title="Düzenle"
                        >
                          <svg className="w-3.5 h-3.5" fill="none" stroke="currentColor" strokeWidth="2" viewBox="0 0 24 24">
                            <path d="M11 4H4a2 2 0 00-2 2v14a2 2 0 002 2h14a2 2 0 002-2v-7"/>
                            <path d="M18.5 2.5a2.121 2.121 0 013 3L12 15l-4 1 1-4 9.5-9.5z"/>
                          </svg>
                        </button>
                        <button
                          onClick={() => setConfirmDeleteId(m.id)}
                          className="p-1.5 rounded hover:bg-red-100 dark:hover:bg-red-900/30 text-beton-400 hover:text-red-600"
                          title="Sil"
                        >
                          <svg className="w-3.5 h-3.5" fill="none" stroke="currentColor" strokeWidth="2" viewBox="0 0 24 24">
                            <polyline points="3 6 5 6 21 6"/><path d="M19 6l-1 14a2 2 0 01-2 2H8a2 2 0 01-2-2L5 6"/>
                            <path d="M10 11v6M14 11v6"/><path d="M9 6V4a1 1 0 011-1h4a1 1 0 011 1v2"/>
                          </svg>
                        </button>
                      </>
                    )}
                  </div>
                </div>

                {/* Expanded: Log panel */}
                {isExpanded && (
                  <div className="border-t border-beton-800 bg-beton-950 px-4 py-4 space-y-4">
                    <h3 className="text-sm font-semibold text-beton-100">Günlük Puantaj — {m.ad}</h3>

                    {/* Add log form */}
                    <div className="flex flex-wrap gap-2 items-end">
                      <div className="flex flex-col gap-1">
                        <label className="text-xs text-beton-400">Tarih</label>
                        <input
                          type="date"
                          value={logForm.tarih ?? ""}
                          onChange={e => setLogForm(f => ({ ...f, tarih: e.target.value }))}
                          className="border border-beton-800 bg-beton-900 rounded px-2 py-1 text-sm text-beton-100 w-36"
                        />
                      </div>
                      <div className="flex flex-col gap-1">
                        <label className="text-xs text-beton-400">Miktar</label>
                        <input
                          type="number"
                          min="0"
                          step="0.5"
                          value={logForm.calisma_miktari ?? 0}
                          onChange={e => setLogForm(f => ({ ...f, calisma_miktari: parseFloat(e.target.value) }))}
                          className="border border-beton-800 bg-beton-900 rounded px-2 py-1 text-sm text-beton-100 w-24"
                        />
                      </div>
                      {logBirimi === "her-ikisi" ? (
                        <div className="flex flex-col gap-1">
                          <label className="text-xs text-beton-400">Birim</label>
                          <select
                            value={logForm.calisma_birimi ?? "saat"}
                            onChange={e => setLogForm(f => ({ ...f, calisma_birimi: e.target.value }))}
                            className="border border-beton-800 bg-beton-900 rounded px-2 py-1 text-sm text-beton-100"
                          >
                            <option value="saat">Saat</option>
                            <option value="km">Km</option>
                          </select>
                        </div>
                      ) : (
                        <div className="flex flex-col gap-1">
                          <label className="text-xs text-beton-400">Birim</label>
                          <span className="border border-beton-800 bg-beton-900 rounded px-2 py-1 text-sm text-beton-400">
                            {logBirimi === "saat" ? "Saat" : "Km"}
                          </span>
                        </div>
                      )}
                      <div className="flex flex-col gap-1">
                        <label className="text-xs text-beton-400">Operatör</label>
                        <input
                          placeholder="Operatör adı"
                          value={logForm.operator ?? ""}
                          onChange={e => setLogForm(f => ({ ...f, operator: e.target.value }))}
                          className="border border-beton-800 bg-beton-900 rounded px-2 py-1 text-sm text-beton-100 w-36"
                        />
                      </div>
                      <div className="flex flex-col gap-1">
                        <label className="text-xs text-beton-400">Açıklama</label>
                        <input
                          placeholder="İsteğe bağlı"
                          value={logForm.aciklama ?? ""}
                          onChange={e => setLogForm(f => ({ ...f, aciklama: e.target.value }))}
                          className="border border-beton-800 bg-beton-900 rounded px-2 py-1 text-sm text-beton-100 w-48"
                        />
                      </div>
                      <button
                        onClick={addLog}
                        disabled={logSaving || (logForm.calisma_miktari ?? 0) <= 0}
                        className="px-3 py-1.5 rounded bg-emniyet-500 hover:bg-emniyet-600 text-beton-950 text-sm font-medium disabled:opacity-50"
                      >
                        {logSaving ? "…" : "Ekle"}
                      </button>
                    </div>
                    {logError && (
                      <p className="text-xs text-red-500">{logError}</p>
                    )}

                    {/* Log list */}
                    {logsBusy ? (
                      <div className="text-sm text-beton-400">Yükleniyor…</div>
                    ) : logs.length === 0 ? (
                      <div className="text-sm text-beton-400">Henüz puantaj kaydı yok.</div>
                    ) : (
                      <div className="overflow-x-auto">
                        <table className="w-full text-sm min-w-[520px]">
                          <thead>
                            <tr className="text-xs text-beton-400 border-b border-beton-800">
                              <th className="text-left py-1.5 pr-3">Tarih</th>
                              <th className="text-right py-1.5 pr-3">Miktar</th>
                              <th className="text-left py-1.5 pr-3">Operatör</th>
                              <th className="text-left py-1.5">Açıklama</th>
                              <th className="w-8" />
                            </tr>
                          </thead>
                          <tbody>
                            {logs.map(lg => (
                              <tr key={lg.id} className="border-b border-beton-800 last:border-0 hover:bg-[var(--bg-hover)]">
                                <td className="py-1.5 pr-3 text-beton-100">{fmtDate(lg.tarih)}</td>
                                <td className="py-1.5 pr-3 text-right font-medium text-beton-100">
                                  {lg.calisma_miktari} {lg.calisma_birimi}
                                </td>
                                <td className="py-1.5 pr-3 text-beton-400">{lg.operator ?? "—"}</td>
                                <td className="py-1.5 text-beton-400 max-w-xs truncate">{lg.aciklama ?? "—"}</td>
                                <td className="py-1.5">
                                  <button
                                    onClick={() => deleteLog(lg.id)}
                                    className="text-beton-400 hover:text-red-500 p-1 rounded"
                                  >
                                    <svg className="w-3.5 h-3.5" fill="none" stroke="currentColor" strokeWidth="2" viewBox="0 0 24 24">
                                      <polyline points="3 6 5 6 21 6"/>
                                      <path d="M19 6l-1 14a2 2 0 01-2 2H8a2 2 0 01-2-2L5 6"/>
                                    </svg>
                                  </button>
                                </td>
                              </tr>
                            ))}
                          </tbody>
                        </table>
                      </div>
                    )}

                    {/* Description */}
                    {m.aciklama && (
                      <p className="text-xs text-beton-400 border-t border-beton-800 pt-2">
                        {m.aciklama}
                      </p>
                    )}
                  </div>
                )}
              </div>
            );
              })}
              </div>
              )}
            </div>
          ))}
        </div>
      )}

      {/* Machine form modal */}
      {formOpen && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60">
          <div className="bg-beton-900 border border-beton-700 rounded-xl shadow-2xl p-6 w-full max-w-lg mx-4 max-h-[90vh] overflow-y-auto space-y-4">
            <h2 className="text-lg font-bold text-beton-100">
              {editId ? "Makineyi Düzenle" : `${tipLabel === "Ekipmanlar" ? "Ekipman" : tipLabel === "İş Makineleri" ? "Makine" : "Araç"} Ekle`}
            </h2>

            <div className="grid grid-cols-2 gap-3">
              {/* Ad */}
              <div className="col-span-2 flex flex-col gap-1">
                <label className="text-xs font-medium text-beton-400">Ad / İsim *</label>
                <select
                  value={adCustom ? AD_DIGER : (form.ad ?? "")}
                  onChange={e => {
                    const v = e.target.value;
                    if (v === AD_DIGER) { setAdCustom(true); setForm(f => ({ ...f, ad: "" })); return; }
                    setAdCustom(false);
                    setForm(f => ({ ...f, ad: v }));
                  }}
                  className="rounded-md bg-beton-950 border border-beton-800 px-3 py-2 text-sm text-beton-100 outline-none focus:border-emniyet-500"
                >
                  <option value="" disabled>— Seçin —</option>
                  {AD_KATALOG[tip].map(a => <option key={a} value={a}>{a}</option>)}
                  <option value={AD_DIGER}>Diğer (elle yaz)</option>
                </select>
                {adCustom && (
                  <input
                    placeholder="Örn: Kamyon 1"
                    value={form.ad ?? ""}
                    onChange={e => setForm(f => ({ ...f, ad: e.target.value }))}
                    className="mt-1 rounded-md bg-beton-950 border border-beton-800 px-3 py-2 text-sm text-beton-100 outline-none focus:border-emniyet-500"
                  />
                )}
              </div>

              {/* Marka */}
              <div className="flex flex-col gap-1">
                <label className="text-xs font-medium text-beton-400">Marka</label>
                <input
                  placeholder="Caterpillar, Volvo…"
                  value={form.marka ?? ""}
                  onChange={e => setForm(f => ({ ...f, marka: e.target.value }))}
                  className="rounded-md bg-beton-950 border border-beton-800 px-3 py-2 text-sm text-beton-100 outline-none focus:border-emniyet-500"
                />
              </div>

              {/* Model */}
              <div className="flex flex-col gap-1">
                <label className="text-xs font-medium text-beton-400">Model</label>
                <input
                  placeholder="320D, FMX 460…"
                  value={form.model ?? ""}
                  onChange={e => setForm(f => ({ ...f, model: e.target.value }))}
                  className="rounded-md bg-beton-950 border border-beton-800 px-3 py-2 text-sm text-beton-100 outline-none focus:border-emniyet-500"
                />
              </div>

              {/* Plaka (araç) */}
              {showPlaka && (
                <div className="flex flex-col gap-1">
                  <label className="text-xs font-medium text-beton-400">Plaka</label>
                  <input
                    placeholder="34 AB 1234"
                    value={form.plaka ?? ""}
                    onChange={e => setForm(f => ({ ...f, plaka: e.target.value }))}
                    className="rounded-md bg-beton-950 border border-beton-800 px-3 py-2 text-sm text-beton-100 outline-none focus:border-emniyet-500"
                  />
                </div>
              )}

              {/* Seri No (ekipman) */}
              {showSeriNo && (
                <div className="flex flex-col gap-1">
                  <label className="text-xs font-medium text-beton-400">Seri No</label>
                  <input
                    placeholder="SN-123456"
                    value={form.seri_no ?? ""}
                    onChange={e => setForm(f => ({ ...f, seri_no: e.target.value }))}
                    className="rounded-md bg-beton-950 border border-beton-800 px-3 py-2 text-sm text-beton-100 outline-none focus:border-emniyet-500"
                  />
                </div>
              )}

              {/* Üretim Yılı */}
              <div className="flex flex-col gap-1">
                <label className="text-xs font-medium text-beton-400">Üretim Yılı</label>
                <input
                  type="number"
                  placeholder="2020"
                  min="1990"
                  max="2030"
                  value={form.uretim_yili ?? ""}
                  onChange={e => setForm(f => ({ ...f, uretim_yili: e.target.value ? parseInt(e.target.value) : undefined }))}
                  className="rounded-md bg-beton-950 border border-beton-800 px-3 py-2 text-sm text-beton-100 outline-none focus:border-emniyet-500"
                />
              </div>

              {/* Sahiplik */}
              <div className="flex flex-col gap-1">
                <label className="text-xs font-medium text-beton-400">Sahiplik</label>
                <select
                  value={form.sahiplik ?? "ozmal"}
                  onChange={e => {
                    const sahiplik = e.target.value;
                    setForm(f => ({
                      ...f,
                      sahiplik,
                      tedarikci: sahiplik === "ozmal"
                        ? ANA_YUKLENICI
                        : (f.tedarikci === ANA_YUKLENICI ? "" : f.tedarikci),
                    }));
                  }}
                  className="rounded-md bg-beton-950 border border-beton-800 px-3 py-2 text-sm text-beton-100 outline-none focus:border-emniyet-500"
                >
                  <option value="ozmal">Öz mal</option>
                  <option value="kiralik">Kiralık</option>
                </select>
              </div>

              {/* Tedarikçi */}
              <div className={`flex flex-col gap-1${!showPlaka && !showSeriNo ? " col-span-2" : ""}`}>
                <label className="text-xs font-medium text-beton-400">Tedarikçi / Kiralık Firma</label>
                <input
                  placeholder="Firma adı"
                  value={form.sahiplik === "ozmal" ? ANA_YUKLENICI : (form.tedarikci ?? "")}
                  disabled={form.sahiplik === "ozmal"}
                  onChange={e => setForm(f => ({ ...f, tedarikci: e.target.value }))}
                  className="rounded-md bg-beton-950 border border-beton-800 px-3 py-2 text-sm text-beton-100 outline-none focus:border-emniyet-500 disabled:opacity-60 disabled:cursor-not-allowed"
                />
              </div>

              {/* Günlük Ücret */}
              <div className="flex flex-col gap-1">
                <label className="text-xs font-medium text-beton-400">Günlük Ücret (₺)</label>
                <input
                  type="number"
                  placeholder="0"
                  min="0"
                  value={form.gunluk_ucret ?? ""}
                  onChange={e => setForm(f => ({ ...f, gunluk_ucret: e.target.value ? parseFloat(e.target.value) : undefined }))}
                  className="rounded-md bg-beton-950 border border-beton-800 px-3 py-2 text-sm text-beton-100 outline-none focus:border-emniyet-500"
                />
              </div>

              {/* Durum */}
              <div className="flex flex-col gap-1">
                <label className="text-xs font-medium text-beton-400">Durum</label>
                <select
                  value={form.durum ?? "aktif"}
                  onChange={e => setForm(f => ({ ...f, durum: e.target.value }))}
                  className="rounded-md bg-beton-950 border border-beton-800 px-3 py-2 text-sm text-beton-100 outline-none focus:border-emniyet-500"
                >
                  <option value="aktif">Aktif</option>
                  <option value="bakim">Bakımda</option>
                  <option value="devre_disi">Devre Dışı</option>
                </select>
              </div>

              {/* Bakım tarihleri — col-span-2 wrapper ile her zaman yanyana */}
              {showBakim && (
                <div className="col-span-2 grid grid-cols-2 gap-3">
                  <div className="flex flex-col gap-1">
                    <label className="text-xs font-medium text-beton-400">Son Bakım Tarihi</label>
                    <input
                      type="date"
                      value={form.son_bakim_tarihi ?? ""}
                      onChange={e => setForm(f => ({ ...f, son_bakim_tarihi: e.target.value || undefined }))}
                      className="rounded-md bg-beton-950 border border-beton-800 px-3 py-2 text-sm text-beton-100 outline-none focus:border-emniyet-500"
                    />
                  </div>
                  <div className="flex flex-col gap-1">
                    <label className="text-xs font-medium text-beton-400">Sıradaki Bakım Tarihi</label>
                    <input
                      type="date"
                      value={form.sonraki_bakim_tarihi ?? ""}
                      onChange={e => setForm(f => ({ ...f, sonraki_bakim_tarihi: e.target.value || undefined }))}
                      className="rounded-md bg-beton-950 border border-beton-800 px-3 py-2 text-sm text-beton-100 outline-none focus:border-emniyet-500"
                    />
                  </div>
                </div>
              )}

              {/* Açıklama */}
              <div className="col-span-2 flex flex-col gap-1">
                <label className="text-xs font-medium text-beton-400">Açıklama</label>
                <textarea
                  rows={2}
                  placeholder="İsteğe bağlı notlar…"
                  value={form.aciklama ?? ""}
                  onChange={e => setForm(f => ({ ...f, aciklama: e.target.value }))}
                  className="rounded-md bg-beton-950 border border-beton-800 px-3 py-2 text-sm text-beton-100 outline-none focus:border-emniyet-500 resize-none"
                />
              </div>
            </div>

            {saveError && (
              <p className="text-sm text-red-400">{saveError}</p>
            )}
            <div className="flex justify-end gap-2 pt-1">
              <button
                onClick={() => { setFormOpen(false); setSaveError(null); }}
                className="px-4 py-1.5 rounded-lg border border-beton-700 text-sm text-beton-300 hover:bg-beton-800"
              >
                İptal
              </button>
              <button
                onClick={saveMachine}
                disabled={saving || !form.ad?.trim()}
                className="px-4 py-1.5 rounded-lg bg-emniyet-500 hover:bg-emniyet-600 text-beton-950 text-sm font-medium disabled:opacity-50"
              >
                {saving ? "Kaydediliyor…" : "Kaydet"}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
