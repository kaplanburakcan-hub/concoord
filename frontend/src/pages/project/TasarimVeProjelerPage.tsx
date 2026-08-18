import { useCallback, useEffect, useMemo, useState } from "react";
import { api } from "../../api/client";
import { useProjects } from "../ProjectContext";

const DISIPLINLER = [
  "Mimari",
  "Statik",
  "Mekanik",
  "Elektrik",
  "İç Mimari",
  "Peyzaj",
  "Altyapı",
  "Diğer",
];

// Yazışmalar sayfasındaki Durum rozetleriyle aynı renk paleti (bkz.
// CorrespondencePage.tsx DURUM_STYLE) — okunaklılık için.
const DURUMLAR: { value: string; label: string; cls: string }[] = [
  { value: "taslak",            label: "Taslak",           cls: "bg-beton-800 text-beton-300 border-beton-700" },
  { value: "incelemede",        label: "İncelemede",       cls: "bg-yellow-500/15 text-yellow-300 border-yellow-500/40" },
  { value: "onaylı",            label: "Onaylı",           cls: "bg-green-500/15 text-green-300 border-green-500/40" },
  { value: "revizyon_gerekli",  label: "Rev. Gerekli",     cls: "bg-red-500/15 text-red-300 border-red-500/40" },
  { value: "iptal",             label: "İptal",            cls: "bg-beton-800 text-beton-500 border-beton-700 line-through" },
];

const inpBase =
  "rounded bg-beton-950 border border-beton-800 px-2 py-1 text-sm text-beton-200 " +
  "outline-none focus:border-emniyet-500 disabled:opacity-50";

type Doc = {
  id?: string;
  disiplin: string;
  poz_no: string;
  baslik: string;
  rev_no: string;
  tarih: string;
  durum: string;
  aciklama: string;
  sira: number;
};

function emptyDoc(disiplin: string): Doc {
  return { disiplin, poz_no: "", baslik: "", rev_no: "0", tarih: "", durum: "taslak", aciklama: "", sira: 0 };
}

function durumMeta(val: string) {
  return DURUMLAR.find((d) => d.value === val) ?? DURUMLAR[0];
}

function isoToDisplay(iso: string) {
  if (!iso) return "";
  const [y, m, d] = iso.split("-");
  return `${d}/${m}/${y}`;
}

function displayToISO(s: string) {
  const parts = s.split("/");
  if (parts.length !== 3) return "";
  const [d, m, y] = parts;
  if (!y || !m || !d || y.length !== 4) return "";
  return `${y}-${m.padStart(2, "0")}-${d.padStart(2, "0")}`;
}

function maskDate(raw: string) {
  const digits = raw.replace(/\D/g, "").slice(0, 8);
  let out = digits;
  if (digits.length > 2) out = digits.slice(0, 2) + "/" + digits.slice(2);
  if (digits.length > 4) out = out.slice(0, 5) + "/" + out.slice(5);
  return out;
}

// ─── ViewRow ────────────────────────────────────────────────────────────────
function ViewRow({
  doc,
  onEdit,
  onDelete,
  canEdit,
}: {
  doc: Doc;
  onEdit: () => void;
  onDelete: () => void;
  canEdit: boolean;
}) {
  const dm = durumMeta(doc.durum);
  return (
    <tr className="border-b border-beton-800 hover:bg-beton-900/40 group">
      <td className="py-2 px-3 text-beton-400 text-xs font-mono whitespace-nowrap">{doc.poz_no}</td>
      <td className="py-2 px-3 text-beton-200 text-sm">{doc.baslik}</td>
      <td className="py-2 px-3 text-center">
        <span className="font-mono text-xs text-beton-300">{doc.rev_no}</span>
      </td>
      <td className="py-2 px-3 text-beton-400 text-xs whitespace-nowrap">{isoToDisplay(doc.tarih)}</td>
      <td className="py-2 px-3">
        <span className={`inline-block rounded-full border px-2 py-0.5 text-[10.5px] font-semibold ${dm.cls}`}>
          {dm.label}
        </span>
      </td>
      <td className="py-2 px-3 text-beton-500 text-xs max-w-[180px] truncate">{doc.aciklama}</td>
      {canEdit && (
        <td className="py-2 px-3">
          <div className="flex gap-2 opacity-0 group-hover:opacity-100 transition-opacity">
            <button
              onClick={onEdit}
              className="text-xs text-emniyet-400 hover:text-emniyet-300"
            >
              Düzenle
            </button>
            <button
              onClick={onDelete}
              className="text-xs text-red-500 hover:text-red-400"
            >
              Sil
            </button>
          </div>
        </td>
      )}
    </tr>
  );
}

// ─── EditRow ────────────────────────────────────────────────────────────────
function EditRow({
  initial,
  onSave,
  onCancel,
  isNew,
  canEdit,
}: {
  initial: Doc;
  onSave: (d: Doc) => void;
  onCancel: () => void;
  isNew: boolean;
  canEdit: boolean;
}) {
  const [d, setD] = useState<Doc>(initial);

  function set(k: keyof Doc, v: string | number) {
    setD((prev) => ({ ...prev, [k]: v }));
  }

  if (!canEdit) return null;

  return (
    <tr className="border-b border-beton-700 bg-beton-900/60">
      <td className="py-1 px-2">
        <input
          className={`${inpBase} w-24 font-mono`}
          placeholder="A.001"
          value={d.poz_no}
          onChange={(e) => set("poz_no", e.target.value)}
        />
      </td>
      <td className="py-1 px-2">
        <input
          className={`${inpBase} w-full min-w-[180px]`}
          placeholder="Başlık *"
          value={d.baslik}
          onChange={(e) => set("baslik", e.target.value)}
        />
      </td>
      <td className="py-1 px-2">
        <input
          className={`${inpBase} w-14 text-center font-mono`}
          placeholder="Rev"
          value={d.rev_no}
          onChange={(e) => set("rev_no", e.target.value)}
        />
      </td>
      <td className="py-1 px-2">
        <input
          className={`${inpBase} w-28`}
          placeholder="gg/aa/yyyy"
          value={isoToDisplay(d.tarih)}
          onChange={(e) => set("tarih", displayToISO(maskDate(e.target.value)))}
          maxLength={10}
        />
      </td>
      <td className="py-1 px-2">
        <select
          className={`${inpBase}`}
          value={d.durum}
          onChange={(e) => set("durum", e.target.value)}
        >
          {DURUMLAR.map((dm) => (
            <option key={dm.value} value={dm.value}>
              {dm.label}
            </option>
          ))}
        </select>
      </td>
      <td className="py-1 px-2">
        <input
          className={`${inpBase} w-full min-w-[120px]`}
          placeholder="Açıklama"
          value={d.aciklama}
          onChange={(e) => set("aciklama", e.target.value)}
        />
      </td>
      <td className="py-1 px-2">
        <div className="flex gap-1">
          <button
            onClick={() => onSave(d)}
            disabled={!d.baslik.trim()}
            className="text-xs bg-emniyet-600 hover:bg-emniyet-500 text-white px-2 py-1 rounded disabled:opacity-40"
          >
            {isNew ? "Ekle" : "Kaydet"}
          </button>
          <button
            onClick={onCancel}
            className="text-xs text-beton-400 hover:text-beton-200 px-2 py-1"
          >
            İptal
          </button>
        </div>
      </td>
    </tr>
  );
}

// ─── DisciplineSection ───────────────────────────────────────────────────────
function DisciplineSection({
  disiplin,
  docs,
  onSave,
  onDelete,
  canEdit,
}: {
  disiplin: string;
  docs: Doc[];
  onSave: (doc: Doc) => Promise<void>;
  onDelete: (doc: Doc) => void;
  canEdit: boolean;
}) {
  const [open, setOpen] = useState(true);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [addingNew, setAddingNew] = useState(false);

  const total = docs.length;
  const onaylandi = docs.filter((d) => d.durum === "onaylı").length;

  return (
    <div className="mb-4 border border-beton-800 rounded-lg overflow-hidden">
      {/* Header */}
      <button
        onClick={() => setOpen((o) => !o)}
        className="w-full flex items-center justify-between px-4 py-3 bg-beton-900 hover:bg-beton-800/60 transition-colors text-left"
      >
        <div className="flex items-center gap-3">
          <span className="text-beton-400 text-xs">{open ? "▼" : "▶"}</span>
          <span className="font-medium text-beton-100">{disiplin}</span>
          <span className="text-beton-500 text-xs">{total} kalem</span>
        </div>
        <div className="flex items-center gap-4">
          {total > 0 && (
            <span className="text-xs text-beton-400">
              {onaylandi}/{total} onaylı
            </span>
          )}
          {total === 0 && (
            <span className="text-beton-600 text-xs">—</span>
          )}
        </div>
      </button>

      {open && (
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            {total > 0 && (
              <thead>
                <tr className="border-b border-beton-800">
                  <th className="py-2 px-3 text-left text-xs text-beton-500 font-medium w-24">Poz No</th>
                  <th className="py-2 px-3 text-left text-xs text-beton-500 font-medium">Başlık</th>
                  <th className="py-2 px-3 text-center text-xs text-beton-500 font-medium w-16">Rev</th>
                  <th className="py-2 px-3 text-left text-xs text-beton-500 font-medium w-28">Tarih</th>
                  <th className="py-2 px-3 text-left text-xs text-beton-500 font-medium w-36">Durum</th>
                  <th className="py-2 px-3 text-left text-xs text-beton-500 font-medium">Açıklama</th>
                  {canEdit && <th className="w-28" />}
                </tr>
              </thead>
            )}
            <tbody>
              {docs.map((doc) =>
                editingId === doc.id ? (
                  <EditRow
                    key={doc.id}
                    initial={doc}
                    isNew={false}
                    canEdit={canEdit}
                    onSave={async (updated) => {
                      await onSave({ ...updated, id: doc.id });
                      setEditingId(null);
                    }}
                    onCancel={() => setEditingId(null)}
                  />
                ) : (
                  <ViewRow
                    key={doc.id}
                    doc={doc}
                    canEdit={canEdit}
                    onEdit={() => setEditingId(doc.id!)}
                    onDelete={() => onDelete(doc)}
                  />
                )
              )}
              {addingNew && (
                <EditRow
                  initial={emptyDoc(disiplin)}
                  isNew
                  canEdit={canEdit}
                  onSave={async (d) => {
                    await onSave(d);
                    setAddingNew(false);
                  }}
                  onCancel={() => setAddingNew(false)}
                />
              )}
            </tbody>
          </table>

          {canEdit && !addingNew && (
            <div className="px-3 py-2 border-t border-beton-800/50">
              <button
                onClick={() => { setAddingNew(true); setOpen(true); }}
                className="text-xs text-emniyet-400 hover:text-emniyet-300"
              >
                + {disiplin} kalemi ekle
              </button>
            </div>
          )}
        </div>
      )}
    </div>
  );
}

// ─── Ana Sayfa ───────────────────────────────────────────────────────────────
export default function TasarimVeProjelerPage() {
  const { current: currentProject } = useProjects();
  const [docs, setDocs] = useState<Doc[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const canEdit = true;

  const pid = currentProject?.id;

  const load = useCallback(async () => {
    if (!pid) return;
    setLoading(true);
    setError(null);
    try {
      const data = await api<{ docs: Doc[] }>(`/projects/${pid}/design-docs`, { projectId: pid });
      setDocs(data.docs ?? []);
    } catch {
      setError("Veriler yüklenemedi.");
    } finally {
      setLoading(false);
    }
  }, [pid]);

  useEffect(() => { load(); }, [load]);

  const grouped = useMemo(() => {
    const m = new Map<string, Doc[]>();
    for (const dis of DISIPLINLER) m.set(dis, []);
    for (const d of docs) {
      if (!m.has(d.disiplin)) m.set(d.disiplin, []);
      m.get(d.disiplin)!.push(d);
    }
    return m;
  }, [docs]);

  async function handleSave(doc: Doc) {
    if (!pid) return;
    if (doc.id) {
      await api(`/projects/${pid}/design-docs/${doc.id}`, { method: "PATCH", body: doc, projectId: pid });
    } else {
      await api(`/projects/${pid}/design-docs`, { method: "POST", body: doc, projectId: pid });
    }
    await load();
  }

  async function handleDelete(doc: Doc) {
    if (!pid || !doc.id) return;
    if (!confirm(`"${doc.baslik}" silinecek. Onaylıyor musunuz?`)) return;
    await api(`/projects/${pid}/design-docs/${doc.id}`, { method: "DELETE", projectId: pid });
    await load();
  }

  // İstatistikler
  const stats = useMemo(() => {
    const total = docs.length;
    const byDurum = Object.fromEntries(DURUMLAR.map((d) => [d.value, 0]));
    for (const d of docs) byDurum[d.durum] = (byDurum[d.durum] ?? 0) + 1;
    return { total, byDurum };
  }, [docs]);

  return (
    <div className="p-6 max-w-6xl mx-auto">
      {/* Başlık */}
      <div className="flex items-start justify-between mb-6">
        <div>
          <h1 className="text-xl font-semibold text-beton-100">Tasarım ve Projeler</h1>
          {currentProject && (
            <p className="text-beton-400 text-sm mt-0.5">{currentProject.name}</p>
          )}
        </div>

        {/* Özet rozetler */}
        {stats.total > 0 && (
          <div className="flex gap-2 flex-wrap justify-end">
            {DURUMLAR.filter((dm) => stats.byDurum[dm.value] > 0).map((dm) => (
              <span
                key={dm.value}
                className={`rounded-full border px-2.5 py-1 text-[11px] font-semibold ${dm.cls}`}
              >
                {dm.label}: {stats.byDurum[dm.value]}
              </span>
            ))}
          </div>
        )}
      </div>

      {loading && (
        <p className="text-beton-500 text-sm">Yükleniyor…</p>
      )}
      {error && (
        <p className="text-red-400 text-sm">{error}</p>
      )}

      {!loading && !error && (
        <>
          {Array.from(grouped.entries()).map(([disiplin, disiplinDocs]) => (
            <DisciplineSection
              key={disiplin}
              disiplin={disiplin}
              docs={disiplinDocs}
              onSave={handleSave}
              onDelete={handleDelete}
              canEdit={canEdit}
            />
          ))}

          {stats.total === 0 && (
            <p className="text-beton-500 text-sm text-center py-8">
              Henüz kayıt yok. Yukarıdaki disiplin bloklarından kalem ekleyebilirsiniz.
            </p>
          )}
        </>
      )}
    </div>
  );
}
