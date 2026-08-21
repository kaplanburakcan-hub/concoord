import { useEffect, useState } from "react";
import { api } from "../../api/client";
import { useProjects } from "../ProjectContext";
import { useKesinKabulTarihi } from "../../hooks/useKesinKabulTarihi";

// ── Tipler ────────────────────────────────────────────────────────────────────
interface Meeting {
  id: string;
  project_id: string;
  toplanti_no?: string;
  baslik: string;
  toplanti_turu: string;
  tarih: string;
  baslangic_saati?: string;
  bitis_saati?: string;
  lokasyon?: string;
  katilimcilar: string[];
  gundem?: string;
  kararlar?: string;
  aksiyon_maddeleri: string; // JSON string
  sonraki_toplanti_tarihi?: string;
  durum: string;
}

interface AksiyonMaddesi {
  aksiyon: string;
  sorumlu: string;
  son_tarih?: string;
  durum?: string;
}

const TURU_META: Record<string, { label: string; cls: string }> = {
  kickoff:  { label: "Açılış",    cls: "bg-purple-500/15 text-purple-300 border-purple-500/40" },
  ilerleme: { label: "İlerleme",  cls: "bg-blue-500/15 text-blue-300 border-blue-500/40" },
  saha:     { label: "Saha",      cls: "bg-green-500/15 text-green-300 border-green-500/40" },
  taseron:  { label: "Taşeron",   cls: "bg-orange-500/15 text-orange-300 border-orange-500/40" },
  risk:     { label: "Risk",      cls: "bg-red-500/15 text-red-300 border-red-500/40" },
  teknik:   { label: "Teknik",    cls: "bg-indigo-500/15 text-indigo-300 border-indigo-500/40" },
  acil:     { label: "Acil",      cls: "bg-rose-500/15 text-rose-300 border-rose-500/40" },
};

const DURUM_META: Record<string, { label: string; cls: string }> = {
  planli:       { label: "Planlandı",    cls: "bg-beton-800 text-beton-300 border-beton-700" },
  gerceklesti:  { label: "Gerçekleşti", cls: "bg-green-500/15 text-green-300 border-green-500/40" },
  iptal:        { label: "İptal",        cls: "bg-red-500/15 text-red-300 border-red-500/40" },
};

function fmtDate(s?: string) {
  if (!s) return "—";
  return new Date(s).toLocaleDateString("tr-TR", { day: "2-digit", month: "short", year: "numeric" });
}

function parseAksiyon(raw: string | null | undefined): AksiyonMaddesi[] {
  try { return JSON.parse(raw ?? "[]") ?? []; } catch { return []; }
}

const inpCls =
  "rounded-lg border border-beton-800 bg-beton-900 px-3 py-1.5 text-sm text-beton-100 " +
  "outline-none focus:border-blue-500 transition-colors";

const taCls =
  "rounded-lg border border-beton-800 bg-beton-900 px-3 py-2 text-sm text-beton-100 " +
  "outline-none focus:border-blue-500 transition-colors resize-none w-full";

// ── Detay Paneli ─────────────────────────────────────────────────────────────
function MeetingDetail({ m, onClose }: { m: Meeting; onClose: () => void }) {
  const aksiyonlar = parseAksiyon(m.aksiyon_maddeleri);
  const turuMeta   = TURU_META[m.toplanti_turu] ?? TURU_META.ilerleme;
  const durumMeta  = DURUM_META[m.durum] ?? DURUM_META.planli;

  return (
    <div className="fixed inset-0 z-50 flex items-start justify-center bg-black/50 p-4 overflow-y-auto">
      <div className="bg-beton-900 border border-beton-800 rounded-2xl p-6 w-full max-w-2xl space-y-5 shadow-xl mt-8 mb-8">
        <div className="flex items-start justify-between gap-4">
          <div className="flex-1 min-w-0">
            <div className="flex flex-wrap gap-1.5 mb-1.5">
              {m.toplanti_no && (
                <span className="text-xs font-mono bg-[var(--bg-hover)] text-beton-400 px-2 py-0.5 rounded">
                  {m.toplanti_no}
                </span>
              )}
              <span className={`rounded-full border px-2 py-0.5 text-[10.5px] font-semibold ${turuMeta.cls}`}>{turuMeta.label}</span>
              <span className={`rounded-full border px-2 py-0.5 text-[10.5px] font-semibold ${durumMeta.cls}`}>{durumMeta.label}</span>
            </div>
            <h2 className="text-lg font-bold text-beton-100">{m.baslik}</h2>
            <p className="text-sm text-beton-400 mt-0.5">
              {fmtDate(m.tarih)}
              {m.baslangic_saati && ` · ${m.baslangic_saati}`}
              {m.bitis_saati && ` – ${m.bitis_saati}`}
              {m.lokasyon && ` · 📍 ${m.lokasyon}`}
            </p>
          </div>
          <button onClick={onClose} className="text-beton-400 hover:text-beton-100 text-xl leading-none p-1">×</button>
        </div>

        {m.katilimcilar?.length > 0 && (
          <div>
            <div className="text-xs font-semibold text-beton-400 uppercase tracking-wide mb-2">Katılımcılar</div>
            <div className="flex flex-wrap gap-1.5">
              {m.katilimcilar.map((k, i) => (
                <span key={i} className="text-xs bg-[var(--bg-hover)] text-beton-100 px-2 py-0.5 rounded-full border border-beton-800">
                  {k}
                </span>
              ))}
            </div>
          </div>
        )}

        {m.gundem && (
          <div>
            <div className="text-xs font-semibold text-beton-400 uppercase tracking-wide mb-2">Gündem</div>
            <div className="text-sm text-beton-100 bg-[var(--bg-hover)] rounded-lg p-3 whitespace-pre-wrap">{m.gundem}</div>
          </div>
        )}

        {m.kararlar && (
          <div>
            <div className="text-xs font-semibold text-beton-400 uppercase tracking-wide mb-2">Alınan Kararlar</div>
            <div className="text-sm text-beton-100 bg-[var(--bg-hover)] rounded-lg p-3 whitespace-pre-wrap">{m.kararlar}</div>
          </div>
        )}

        {aksiyonlar.length > 0 && (
          <div>
            <div className="text-xs font-semibold text-beton-400 uppercase tracking-wide mb-2">
              Aksiyon Maddeleri ({aksiyonlar.length})
            </div>
            <div className="space-y-2">
              {aksiyonlar.map((a, i) => (
                <div key={i} className="flex items-start gap-3 bg-[var(--bg-hover)] rounded-lg p-3">
                  <span className="text-xs font-bold text-beton-400 w-5 mt-0.5 shrink-0">{i + 1}.</span>
                  <div className="flex-1 min-w-0">
                    <div className="text-sm text-beton-100">{a.aksiyon}</div>
                    <div className="flex flex-wrap gap-2 mt-1">
                      {a.sorumlu && (
                        <span className="text-xs text-beton-400">👤 {a.sorumlu}</span>
                      )}
                      {a.son_tarih && (
                        <span className="text-xs text-beton-400">📅 {fmtDate(a.son_tarih)}</span>
                      )}
                      {a.durum && (
                        <span className={`text-xs px-1.5 py-0.5 rounded-full ${
                          a.durum === "tamamlandi" ? "bg-green-100 text-green-700 dark:bg-green-900/40 dark:text-green-300" :
                          a.durum === "devam"      ? "bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-300" :
                          "bg-gray-100 text-gray-500 dark:bg-gray-700 dark:text-gray-400"
                        }`}>
                          {a.durum === "tamamlandi" ? "Tamamlandı" : a.durum === "devam" ? "Devam Ediyor" : a.durum}
                        </span>
                      )}
                    </div>
                  </div>
                </div>
              ))}
            </div>
          </div>
        )}

        {m.sonraki_toplanti_tarihi && (
          <div className="rounded-lg border border-beton-800 p-3 flex items-center gap-2">
            <span className="text-sm text-beton-400">Sonraki Toplantı:</span>
            <span className="text-sm font-medium text-beton-100">{fmtDate(m.sonraki_toplanti_tarihi)}</span>
          </div>
        )}
      </div>
    </div>
  );
}

// ── Yeni Toplantı Modalı ─────────────────────────────────────────────────────
function CreateModal({ projectId, onSave, onClose }: { projectId: string; onSave: () => void; onClose: () => void }) {
  const kesinKabul = useKesinKabulTarihi(projectId);
  const [baslik, setBaslik]           = useState("");
  const [turu, setTuru]               = useState("ilerleme");
  const [tarih, setTarih]             = useState(new Date().toISOString().slice(0, 10));
  const [baslangic, setBaslangic]     = useState("");
  const [bitis, setBitis]             = useState("");
  const [lokasyon, setLokasyon]       = useState("");
  const [katilimcilar, setKatilimcilar] = useState("");
  const [gundem, setGundem]           = useState("");
  const [kararlar, setKararlar]       = useState("");
  const [durum, setDurum]             = useState("planli");
  const [toplatiNo, setToplantiNo]    = useState("");
  const [saving, setSaving]           = useState(false);
  const [err, setErr]                 = useState("");

  async function submit() {
    if (!baslik.trim()) { setErr("Toplantı başlığı zorunludur."); return; }
    setSaving(true); setErr("");
    const katList = katilimcilar.split("\n").map(s => s.trim()).filter(Boolean);
    try {
      await api(`/projects/${projectId}/meetings`, {
        projectId,
        method: "POST",
        body: {
          toplanti_no: toplatiNo.trim() || undefined,
          baslik: baslik.trim(),
          toplanti_turu: turu,
          tarih,
          baslangic_saati: baslangic || undefined,
          bitis_saati: bitis || undefined,
          lokasyon: lokasyon.trim() || undefined,
          katilimcilar: katList,
          gundem: gundem.trim() || undefined,
          kararlar: kararlar.trim() || undefined,
          durum,
        },
      });
      onSave();
    } catch {
      setErr("Kaydedilemedi.");
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-start justify-center bg-black/50 p-4 overflow-y-auto">
      <div className="bg-beton-900 border border-beton-800 rounded-2xl p-6 w-full max-w-lg space-y-4 shadow-xl mt-8 mb-8">
        <h3 className="font-semibold text-beton-100 text-lg">Yeni Toplantı Tutanağı</h3>
        {err && <div className="text-sm text-red-500">{err}</div>}
        <div className="space-y-3">
          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className="text-xs text-beton-400 mb-1 block">Toplantı No</label>
              <input className={inpCls + " w-full"} value={toplatiNo} onChange={e => setToplantiNo(e.target.value)} placeholder="TT-001" />
            </div>
            <div>
              <label className="text-xs text-beton-400 mb-1 block">Tür</label>
              <select className={inpCls + " w-full"} value={turu} onChange={e => setTuru(e.target.value)}>
                {Object.entries(TURU_META).map(([k, v]) => <option key={k} value={k}>{v.label}</option>)}
              </select>
            </div>
          </div>
          <div>
            <label className="text-xs text-beton-400 mb-1 block">Başlık *</label>
            <input className={inpCls + " w-full"} value={baslik} onChange={e => setBaslik(e.target.value)} placeholder="ör. Haftalık İlerleme Toplantısı" />
          </div>
          <div className="grid grid-cols-3 gap-3">
            <div>
              <label className="text-xs text-beton-400 mb-1 block">Tarih</label>
              <input className={inpCls + " w-full"} type="date" value={tarih} max={kesinKabul} onChange={e => setTarih(e.target.value)} />
            </div>
            <div>
              <label className="text-xs text-beton-400 mb-1 block">Başlangıç</label>
              <input className={inpCls + " w-full"} type="time" value={baslangic} onChange={e => setBaslangic(e.target.value)} />
            </div>
            <div>
              <label className="text-xs text-beton-400 mb-1 block">Bitiş</label>
              <input className={inpCls + " w-full"} type="time" value={bitis} onChange={e => setBitis(e.target.value)} />
            </div>
          </div>
          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className="text-xs text-beton-400 mb-1 block">Lokasyon</label>
              <input className={inpCls + " w-full"} value={lokasyon} onChange={e => setLokasyon(e.target.value)} placeholder="Şantiye, ofis…" />
            </div>
            <div>
              <label className="text-xs text-beton-400 mb-1 block">Durum</label>
              <select className={inpCls + " w-full"} value={durum} onChange={e => setDurum(e.target.value)}>
                <option value="planli">Planlandı</option>
                <option value="gerceklesti">Gerçekleşti</option>
                <option value="iptal">İptal</option>
              </select>
            </div>
          </div>
          <div>
            <label className="text-xs text-beton-400 mb-1 block">Katılımcılar (her satıra bir isim)</label>
            <textarea className={taCls} rows={3} value={katilimcilar} onChange={e => setKatilimcilar(e.target.value)} placeholder={"Ahmet Yılmaz\nAyşe Kaya\n…"} />
          </div>
          <div>
            <label className="text-xs text-beton-400 mb-1 block">Gündem</label>
            <textarea className={taCls} rows={3} value={gundem} onChange={e => setGundem(e.target.value)} placeholder="Toplantı gündem maddeleri…" />
          </div>
          <div>
            <label className="text-xs text-beton-400 mb-1 block">Alınan Kararlar</label>
            <textarea className={taCls} rows={3} value={kararlar} onChange={e => setKararlar(e.target.value)} placeholder="Toplantıda alınan kararlar…" />
          </div>
        </div>
        <div className="flex gap-2 pt-1">
          <button onClick={onClose} className="flex-1 py-2 rounded-lg border border-beton-800 text-sm text-beton-400 hover:text-beton-100 transition-colors">
            İptal
          </button>
          <button onClick={submit} disabled={saving} className="flex-1 py-2 rounded-lg bg-emniyet-500 hover:bg-emniyet-600 text-beton-950 text-sm font-medium transition-colors disabled:opacity-50">
            {saving ? "Kaydediliyor…" : "Kaydet"}
          </button>
        </div>
      </div>
    </div>
  );
}

// ── Ana Sayfa ─────────────────────────────────────────────────────────────────
export default function ToplantiPage() {
  const { current: proj } = useProjects();
  const [meetings, setMeetings]       = useState<Meeting[]>([]);
  const [loading, setLoading]         = useState(true);
  const [error, setError]             = useState(false);
  const [selected, setSelected]       = useState<Meeting | null>(null);
  const [showCreate, setShowCreate]   = useState(false);
  const [filterTur, setFilterTur]     = useState("");
  const [filterDurum, setFilterDurum] = useState("");
  const [search, setSearch]           = useState("");

  function reload() {
    if (!proj?.id) return;
    api<{ meetings: Meeting[] }>(`/projects/${proj.id}/meetings`, { projectId: proj.id })
      .then(r => setMeetings(r.meetings ?? []))
      .catch(() => setError(true))
      .finally(() => setLoading(false));
  }

  useEffect(() => {
    if (!proj?.id) return;
    setLoading(true); setError(false);
    reload();
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [proj?.id]);

  if (!proj) return <div className="p-8 text-beton-400">Proje seçilmedi.</div>;
  if (loading) return <div className="p-8 text-beton-400">Yükleniyor…</div>;
  if (error) return <div className="p-8 text-red-500">Veriler yüklenemedi.</div>;

  const filtered = meetings.filter(m => {
    if (filterTur   && m.toplanti_turu !== filterTur)   return false;
    if (filterDurum && m.durum         !== filterDurum) return false;
    if (search) {
      const q = search.toLowerCase();
      return m.baslik.toLowerCase().includes(q) ||
             (m.lokasyon ?? "").toLowerCase().includes(q) ||
             (m.gundem ?? "").toLowerCase().includes(q) ||
             (m.kararlar ?? "").toLowerCase().includes(q) ||
             m.katilimcilar.some(k => k.toLowerCase().includes(q));
    }
    return true;
  });

  const byDurum: Record<string, number> = {};
  meetings.forEach(m => { byDurum[m.durum] = (byDurum[m.durum] ?? 0) + 1; });

  const totalAksiyon = meetings.reduce((sum, m) => sum + parseAksiyon(m.aksiyon_maddeleri).length, 0);

  return (
    <div className="p-6 space-y-5 max-w-5xl mx-auto">
      {/* Başlık */}
      <div className="flex items-start justify-between gap-4 flex-wrap">
        <div>
          <h1 className="text-xl font-bold text-beton-100">Toplantı Tutanakları</h1>
          <p className="text-sm text-beton-400 mt-0.5">{proj.name} — PMP İletişim Yönetimi</p>
        </div>
        <button
          onClick={() => setShowCreate(true)}
          className="px-3 py-1.5 rounded-lg bg-emniyet-500 hover:bg-emniyet-600 text-beton-950 text-sm font-medium transition-colors"
        >
          + Yeni Tutanak
        </button>
      </div>

      {/* KPI */}
      <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
        <div className="rounded-xl border border-beton-800 bg-beton-900 p-4 text-center">
          <div className="text-2xl font-bold text-beton-100">{meetings.length}</div>
          <div className="text-xs text-beton-400 mt-0.5">Toplam Toplantı</div>
        </div>
        <div className="rounded-xl border border-beton-800 bg-beton-900 p-4 text-center">
          <div className="text-2xl font-bold text-green-600">{byDurum["gerceklesti"] ?? 0}</div>
          <div className="text-xs text-beton-400 mt-0.5">Gerçekleşti</div>
        </div>
        <div className="rounded-xl border border-beton-800 bg-beton-900 p-4 text-center">
          <div className="text-2xl font-bold text-blue-600">{byDurum["planli"] ?? 0}</div>
          <div className="text-xs text-beton-400 mt-0.5">Planlandı</div>
        </div>
        <div className="rounded-xl border border-beton-800 bg-beton-900 p-4 text-center">
          <div className="text-2xl font-bold text-beton-100">{totalAksiyon}</div>
          <div className="text-xs text-beton-400 mt-0.5">Aksiyon Maddesi</div>
        </div>
      </div>

      {/* Filtreler */}
      <div className="flex flex-wrap gap-2">
        <input
          className={inpCls + " w-48"}
          placeholder="Başlık, gündem, karar ara…"
          value={search}
          onChange={e => setSearch(e.target.value)}
        />
        <select className={inpCls} value={filterTur} onChange={e => setFilterTur(e.target.value)}>
          <option value="">Tüm Türler</option>
          {Object.entries(TURU_META).map(([k, v]) => <option key={k} value={k}>{v.label}</option>)}
        </select>
        <select className={inpCls} value={filterDurum} onChange={e => setFilterDurum(e.target.value)}>
          <option value="">Tüm Durumlar</option>
          {Object.entries(DURUM_META).map(([k, v]) => <option key={k} value={k}>{v.label}</option>)}
        </select>
        {(search || filterTur || filterDurum) && (
          <button
            onClick={() => { setSearch(""); setFilterTur(""); setFilterDurum(""); }}
            className="px-3 py-1.5 text-xs rounded-lg border border-beton-800 text-beton-400 hover:text-beton-100 transition-colors"
          >
            Temizle
          </button>
        )}
        <span className="ml-auto text-xs text-beton-400 self-center">{filtered.length} kayıt</span>
      </div>

      {/* Liste */}
      {filtered.length === 0 ? (
        <div className="rounded-xl border border-beton-800 bg-beton-900 p-12 text-center text-beton-400">
          {meetings.length === 0
            ? "Henüz tutanak eklenmedi. \"+ Yeni Tutanak\" butonuyla başlayın."
            : "Arama kriterine uyan tutanak yok."}
        </div>
      ) : (
        <div className="space-y-2">
          {filtered.map(m => {
            const turuMeta  = TURU_META[m.toplanti_turu]  ?? TURU_META.ilerleme;
            const durumMeta = DURUM_META[m.durum]         ?? DURUM_META.planli;
            const akSayisi  = parseAksiyon(m.aksiyon_maddeleri).length;
            return (
              <button
                key={m.id}
                onClick={() => setSelected(m)}
                className="w-full text-left rounded-xl border border-beton-800 bg-beton-900 p-4 hover:bg-[var(--bg-hover)] transition-colors"
              >
                <div className="flex items-start justify-between gap-3">
                  <div className="flex-1 min-w-0">
                    <div className="flex flex-wrap items-center gap-1.5 mb-1">
                      {m.toplanti_no && (
                        <span className="text-xs font-mono bg-[var(--bg-hover)] text-beton-400 px-1.5 py-0.5 rounded">
                          {m.toplanti_no}
                        </span>
                      )}
                      <span className={`rounded-full border px-2 py-0.5 text-[10.5px] font-semibold ${turuMeta.cls}`}>{turuMeta.label}</span>
                      <span className={`rounded-full border px-2 py-0.5 text-[10.5px] font-semibold ${durumMeta.cls}`}>{durumMeta.label}</span>
                    </div>
                    <div className="font-medium text-beton-100 truncate">{m.baslik}</div>
                    <div className="text-xs text-beton-400 mt-0.5 flex flex-wrap gap-x-3 gap-y-0.5">
                      <span>{fmtDate(m.tarih)}</span>
                      {m.lokasyon && <span>📍 {m.lokasyon}</span>}
                      {m.katilimcilar?.length > 0 && <span>👥 {m.katilimcilar.length} katılımcı</span>}
                      {akSayisi > 0 && <span>✅ {akSayisi} aksiyon</span>}
                    </div>
                  </div>
                  <span className="text-beton-400 text-lg shrink-0">›</span>
                </div>
              </button>
            );
          })}
        </div>
      )}

      {/* Detay Modal */}
      {selected && <MeetingDetail m={selected} onClose={() => setSelected(null)} />}

      {/* Oluşturma Modal */}
      {showCreate && (
        <CreateModal
          projectId={proj.id}
          onSave={() => { setShowCreate(false); reload(); }}
          onClose={() => setShowCreate(false)}
        />
      )}
    </div>
  );
}
